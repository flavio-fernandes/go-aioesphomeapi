// Package simulator provides an in-process ESPHome Native API device.
// It is deterministic, opens no network ports, and is intended for unit tests,
// examples, and MGMT integration tests.
package simulator

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	aioesphomeapi "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/internal/wire"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

// DefaultTestEncryptionKey is public test data shared with the conveyor MCL
// example. It must never be reused by a real device.
const DefaultTestEncryptionKey = "kJ7hc0lJ0Zw9N3DcJzXn1kJ7hc0lJ0Zw9N3DcJzXn1k="

var (
	defaultTestKey, _ = base64.StdEncoding.DecodeString(DefaultTestEncryptionKey)
	// ErrNonLoopbackOnly is returned before accepting a simulator listener
	// that could expose the public test key outside the local host.
	ErrNonLoopbackOnly = errors.New("simulator listener must use a loopback TCP address")
)

// Scenario is the complete initial state advertised by a simulated device.
type Scenario struct {
	Name string
	// Seed controls only explicitly randomized actions. Zero is valid while a
	// scenario contains no randomized actions.
	Seed     uint64
	Entities []proto.Message
	States   []proto.Message
	Logs     []*pb.SubscribeLogsResponse
	Faults   []Fault
}

type config struct {
	plaintext bool
	key       []byte
}
type Option func(*config)

// WithPlaintext enables the intentionally insecure test transport.
func WithPlaintext() Option { return func(c *config) { c.plaintext = true } }

// WithEncryptionKey replaces the public test-only Noise key.
func WithEncryptionKey(base64Key string) Option {
	return func(c *config) {
		if decoded, err := base64.StdEncoding.DecodeString(base64Key); err == nil && len(decoded) == 32 {
			c.key = append([]byte(nil), decoded...)
		}
	}
}

// Device accepts injected net.Pipe connections and records received commands.
type Device struct {
	scenario        Scenario
	validationErr   error
	config          config
	commands        chan proto.Message
	done            chan struct{}
	closeOnce       sync.Once
	mu              sync.Mutex
	accepted        uint64
	droppedCommands uint64
	connections     map[net.Conn]struct{}
	listeners       map[net.Listener]struct{}
	wg              sync.WaitGroup
}

// New creates a stopped-port, in-process device. The default is Noise-encrypted
// with a deliberately public test key.
func New(scenario Scenario, options ...Option) *Device {
	if scenario.Name == "" {
		scenario.Name = "simulated-esphome"
	}
	validationErr := scenario.Validate()
	if validationErr == nil {
		scenario = cloneScenario(scenario)
	} else {
		// Invalid payloads are never served, so retain only the fields needed to
		// build safe client options while the typed error is deferred.
		scenario = Scenario{Name: scenario.Name, Seed: scenario.Seed}
	}
	cfg := config{key: append([]byte(nil), defaultTestKey...)}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	return &Device{
		scenario:      scenario,
		validationErr: validationErr,
		config:        cfg,
		commands:      make(chan proto.Message, 64),
		done:          make(chan struct{}),
		connections:   make(map[net.Conn]struct{}),
		listeners:     make(map[net.Listener]struct{}),
	}
}

// DialContext is passed to aioesphomeapi.WithDialContext.
func (d *Device) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if d.validationErr != nil {
		return nil, d.validationErr
	}
	select {
	case <-d.done:
		return nil, errors.New("simulator closed")
	default:
	}
	client, server := net.Pipe()
	if !d.startConnection(server) {
		_ = client.Close()
		_ = server.Close()
		return nil, errors.New("simulator closed")
	}
	// Like net.Dialer.DialContext, ctx bounds connection establishment only.
	// The returned connection's lifetime belongs to the client and Device.Close.
	return client, nil
}

// Serve accepts encrypted Native API sessions from a caller-owned TCP
// listener. It rejects wildcard and non-loopback addresses so the test-only
// key cannot accidentally expose the simulator to a network. Close stops the
// listener and all accepted sessions.
func (d *Device) Serve(listener net.Listener) error {
	if d.validationErr != nil {
		return d.validationErr
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.IP == nil || !address.IP.IsLoopback() {
		return ErrNonLoopbackOnly
	}
	d.mu.Lock()
	select {
	case <-d.done:
		d.mu.Unlock()
		return errors.New("simulator closed")
	default:
	}
	d.listeners[listener] = struct{}{}
	d.mu.Unlock()
	defer func() {
		_ = listener.Close()
		d.mu.Lock()
		delete(d.listeners, listener)
		d.mu.Unlock()
	}()

	for {
		connection, err := listener.Accept()
		if err != nil {
			select {
			case <-d.done:
				return nil
			default:
				return fmt.Errorf("simulator accept failed: %w", err)
			}
		}
		if !d.startConnection(connection) {
			_ = connection.Close()
			return nil
		}
	}
}

func (d *Device) startConnection(connection net.Conn) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	select {
	case <-d.done:
		return false
	default:
	}
	d.accepted++
	d.connections[connection] = struct{}{}
	d.wg.Add(1)
	go d.serve(connection)
	return true
}

// ClientOptions returns all options required to connect to this Device.
func (d *Device) ClientOptions() []aioesphomeapi.Option {
	options := []aioesphomeapi.Option{aioesphomeapi.WithDialContext(d.DialContext), aioesphomeapi.WithExpectedName(d.scenario.Name)}
	if d.config.plaintext {
		return append(options, aioesphomeapi.WithInsecurePlaintext())
	}
	return append(options, aioesphomeapi.WithEncryptionKey(base64.StdEncoding.EncodeToString(d.config.key)))
}

// Commands yields defensive copies of commands received by the device.
func (d *Device) Commands() <-chan proto.Message { return d.commands }

// DeviceStats is a point-in-time connection snapshot. It contains no network
// addresses, device identifiers, or credential material.
type DeviceStats struct {
	AcceptedConnections uint64
	ActiveConnections   int
	DroppedCommands     uint64
}

// Stats reports deterministic connection counts for cleanup, polling, and
// reconnect assertions.
func (d *Device) Stats() DeviceStats {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DeviceStats{AcceptedConnections: d.accepted, ActiveConnections: len(d.connections), DroppedCommands: d.droppedCommands}
}

// Close terminates every active simulated connection.
func (d *Device) Close() error {
	d.closeOnce.Do(func() {
		close(d.done)
		d.mu.Lock()
		for listener := range d.listeners {
			_ = listener.Close()
		}
		for connection := range d.connections {
			_ = connection.Close()
		}
		d.mu.Unlock()
		d.wg.Wait()
		close(d.commands)
	})
	return nil
}

func (d *Device) serve(connection net.Conn) {
	defer d.wg.Done()
	defer connection.Close()
	defer func() { d.mu.Lock(); delete(d.connections, connection); d.mu.Unlock() }()
	var framer wire.Framer
	var err error
	if d.config.plaintext {
		framer = wire.NewPlainFramer(connection, wire.DefaultMaxFrameSize)
	} else {
		framer, err = wire.NewNoiseServerFramer(connection, d.config.key, d.scenario.Name, 5*time.Second, wire.DefaultMaxFrameSize)
	}
	if err != nil {
		return
	}
	id, payload, err := framer.ReadFrame()
	if err != nil || id != 1 {
		return
	}
	if _, err = wire.Decode(id, payload); err != nil {
		return
	}
	if send(framer, &pb.HelloResponse{ApiVersionMajor: 1, ApiVersionMinor: 10, ServerInfo: "go-aioesphomeapi simulator", Name: d.scenario.Name}) != nil {
		return
	}
	if d.triggerFault(framer, FaultAfterHello) {
		return
	}

	states := make(map[uint32]proto.Message)
	for _, state := range d.scenario.States {
		if key, ok := stateKey(state); ok {
			states[key] = proto.Clone(state)
		}
	}
	for {
		id, payload, err = framer.ReadFrame()
		if err != nil {
			return
		}
		message, decodeErr := wire.Decode(id, payload)
		if decodeErr != nil {
			return
		}
		switch m := message.(type) {
		case *pb.ListEntitiesRequest:
			for _, entity := range d.scenario.Entities {
				if send(framer, proto.Clone(entity)) != nil {
					return
				}
			}
			if d.triggerFault(framer, FaultBeforeEntitiesDone) {
				return
			}
			if send(framer, &pb.ListEntitiesDoneResponse{}) != nil {
				return
			}
		case *pb.SubscribeStatesRequest:
			for _, state := range d.scenario.States {
				if send(framer, proto.Clone(state)) != nil {
					return
				}
			}
			if d.triggerFault(framer, FaultAfterInitialStates) {
				return
			}
		case *pb.SubscribeLogsRequest:
			for _, entry := range d.scenario.Logs {
				if entry.Level <= m.Level {
					if send(framer, proto.Clone(entry)) != nil {
						return
					}
				}
			}
		case *pb.PingRequest:
			if send(framer, &pb.PingResponse{}) != nil {
				return
			}
		case *pb.DisconnectRequest:
			_ = send(framer, &pb.DisconnectResponse{})
			return
		case *pb.SwitchCommandRequest:
			d.record(m)
			state := &pb.SwitchStateResponse{Key: m.Key, State: m.State}
			states[m.Key] = state
			if send(framer, state) != nil {
				return
			}
		case *pb.NumberCommandRequest:
			d.record(m)
			state := &pb.NumberStateResponse{Key: m.Key, State: m.State}
			states[m.Key] = state
			if send(framer, state) != nil {
				return
			}
		case *pb.ButtonCommandRequest:
			d.record(m)
		case *pb.FanCommandRequest:
			d.record(m)
			state, _ := states[m.Key].(*pb.FanStateResponse)
			if state == nil {
				state = &pb.FanStateResponse{Key: m.Key}
			} else {
				state = proto.Clone(state).(*pb.FanStateResponse)
			}
			applyFan(state, m)
			states[m.Key] = state
			if send(framer, state) != nil {
				return
			}
		case *pb.LightCommandRequest:
			d.record(m)
			state, _ := states[m.Key].(*pb.LightStateResponse)
			if state == nil {
				state = &pb.LightStateResponse{Key: m.Key}
			} else {
				state = proto.Clone(state).(*pb.LightStateResponse)
			}
			applyLight(state, m)
			states[m.Key] = state
			if send(framer, state) != nil {
				return
			}
		}
	}
}

func (d *Device) record(message proto.Message) {
	select {
	case d.commands <- proto.Clone(message):
	default:
		d.mu.Lock()
		d.droppedCommands++
		d.mu.Unlock()
	}
}

func send(framer wire.Framer, message proto.Message) error {
	id, err := wire.MessageID(message)
	if err != nil {
		return err
	}
	payload, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	return framer.WriteFrame(id, payload)
}

func stateKey(message proto.Message) (uint32, bool) {
	switch m := message.(type) {
	case *pb.BinarySensorStateResponse:
		return m.Key, true
	case *pb.SensorStateResponse:
		return m.Key, true
	case *pb.TextSensorStateResponse:
		return m.Key, true
	case *pb.SwitchStateResponse:
		return m.Key, true
	case *pb.NumberStateResponse:
		return m.Key, true
	case *pb.FanStateResponse:
		return m.Key, true
	case *pb.LightStateResponse:
		return m.Key, true
	default:
		return 0, false
	}
}

func applyFan(state *pb.FanStateResponse, command *pb.FanCommandRequest) {
	if command.HasState {
		state.State = command.State
	}
	if command.HasOscillating {
		state.Oscillating = command.Oscillating
	}
	if command.HasDirection {
		state.Direction = command.Direction
	}
	if command.HasSpeedLevel {
		state.SpeedLevel = command.SpeedLevel
	}
	if command.HasPresetMode {
		state.PresetMode = command.PresetMode
	}
}
func applyLight(state *pb.LightStateResponse, command *pb.LightCommandRequest) {
	if command.HasState {
		state.State = command.State
	}
	if command.HasBrightness {
		state.Brightness = command.Brightness
	}
	if command.HasColorMode {
		state.ColorMode = command.ColorMode
	}
	if command.HasColorBrightness {
		state.ColorBrightness = command.ColorBrightness
	}
	if command.HasRgb {
		state.Red, state.Green, state.Blue = command.Red, command.Green, command.Blue
	}
	if command.HasWhite {
		state.White = command.White
	}
	if command.HasColorTemperature {
		state.ColorTemperature = command.ColorTemperature
	}
	if command.HasColdWhite {
		state.ColdWhite = command.ColdWhite
	}
	if command.HasWarmWhite {
		state.WarmWhite = command.WarmWhite
	}
	if command.HasEffect {
		state.Effect = command.Effect
	}
}

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
	// States is the original name for InitialStates. It remains supported for
	// source compatibility; a scenario must set at most one of the two fields.
	States []proto.Message
	// InitialStates is the snapshot sent to every new state subscriber.
	InitialStates []proto.Message
	// StateTimeline contains absolute virtual-time updates in declaration
	// order. Events at the same time retain that order.
	StateTimeline []StateEvent
	Logs          []*pb.SubscribeLogsResponse
	// Commands declares the exact ordered commands a deterministic test expects
	// the device to receive. An empty slice leaves command observation in the
	// exploratory Commands-only mode.
	Commands []CommandExpectation
	Faults   []Fault
	// Network declares deterministic server-to-client wire shaping at exact
	// protocol triggers. Each action applies to the next response frame.
	Network []NetworkFault
}

// StateEvent changes one entity state at an absolute ManualClock time.
type StateEvent struct {
	At    time.Duration
	State proto.Message
}

type config struct {
	plaintext bool
	key       []byte
	clock     *ManualClock
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

// WithManualClock shares caller-controlled virtual time with the Device. A nil
// clock is ignored and the Device receives its own clock at time zero.
func WithManualClock(clock *ManualClock) Option {
	return func(c *config) {
		if clock != nil {
			c.clock = clock
		}
	}
}

type stateIdentity struct {
	family string
	key    uint32
}

type deviceSession struct {
	connection net.Conn
	network    *networkConn
	writeMu    sync.Mutex
	framer     wire.Framer
	subscribed bool
}

// Device accepts injected net.Pipe connections and records received commands.
type Device struct {
	scenario             Scenario
	validationErr        error
	config               config
	clock                *ManualClock
	commands             chan proto.Message
	commandMu            sync.Mutex
	commandNotify        chan struct{}
	commandIndex         int
	commandMatched       uint32
	commandObserved      uint64
	commandErr           error
	done                 chan struct{}
	closeOnce            sync.Once
	mu                   sync.Mutex
	accepted             uint64
	droppedCommands      uint64
	droppedSessions      uint64
	connections          map[net.Conn]*deviceSession
	listeners            map[net.Listener]struct{}
	wg                   sync.WaitGroup
	stateSerial          sync.Mutex
	stateMu              sync.Mutex
	currentStates        map[stateIdentity]proto.Message
	stateOrder           []stateIdentity
	timelineIndex        int
	networkMu            sync.Mutex
	fragmentedFrames     uint64
	fragmentedSegments   uint64
	coalescedFrames      uint64
	coalescedSegments    uint64
	delayedFrames        uint64
	pendingDelayedFrames uint64
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
	if cfg.clock == nil {
		cfg.clock = NewManualClock()
	}
	device := &Device{
		scenario:      scenario,
		validationErr: validationErr,
		config:        cfg,
		clock:         cfg.clock,
		commands:      make(chan proto.Message, 64),
		commandNotify: make(chan struct{}),
		done:          make(chan struct{}),
		connections:   make(map[net.Conn]*deviceSession),
		listeners:     make(map[net.Listener]struct{}),
		currentStates: make(map[stateIdentity]proto.Message),
	}
	if validationErr == nil {
		for _, state := range device.initialStates() {
			device.storeStateLocked(state)
		}
	}
	device.clock.register(device)
	if validationErr == nil {
		_ = device.advanceTimeline(device.clock.Now())
	}
	return device
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
	shaped := newNetworkConn(connection, d)
	session := &deviceSession{connection: shaped, network: shaped}
	d.connections[shaped] = session
	d.wg.Add(1)
	go d.serve(session)
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

// Clock returns the Device's deterministic clock. Callers that do not need to
// share time across Devices can advance this default clock directly.
func (d *Device) Clock() *ManualClock { return d.clock }

// Commands yields defensive copies of commands received by the device.
func (d *Device) Commands() <-chan proto.Message { return d.commands }

// DeviceStats is a point-in-time connection snapshot. It contains no network
// addresses, device identifiers, or credential material.
type DeviceStats struct {
	AcceptedConnections       uint64
	ActiveConnections         int
	DroppedCommands           uint64
	DroppedConnections        uint64
	NetworkFragmentedFrames   uint64
	NetworkFragmentedSegments uint64
	NetworkCoalescedFrames    uint64
	NetworkCoalescedSegments  uint64
	NetworkDelayedFrames      uint64
	NetworkPendingDelays      uint64
}

// Stats reports deterministic connection counts for cleanup, polling, and
// reconnect assertions.
func (d *Device) Stats() DeviceStats {
	d.mu.Lock()
	result := DeviceStats{
		AcceptedConnections: d.accepted,
		ActiveConnections:   len(d.connections),
		DroppedCommands:     d.droppedCommands,
		DroppedConnections:  d.droppedSessions,
	}
	d.mu.Unlock()
	d.networkMu.Lock()
	result.NetworkFragmentedFrames = d.fragmentedFrames
	result.NetworkFragmentedSegments = d.fragmentedSegments
	result.NetworkCoalescedFrames = d.coalescedFrames
	result.NetworkCoalescedSegments = d.coalescedSegments
	result.NetworkDelayedFrames = d.delayedFrames
	result.NetworkPendingDelays = d.pendingDelayedFrames
	d.networkMu.Unlock()
	return result
}

// DropConnections terminates all current sessions without closing the Device
// or its listeners. The latest device state and future timeline remain intact,
// allowing deterministic reconnect and outage tests.
func (d *Device) DropConnections() int {
	d.mu.Lock()
	connections := make([]net.Conn, 0, len(d.connections))
	for connection := range d.connections {
		connections = append(connections, connection)
		delete(d.connections, connection)
	}
	if len(connections) > 0 {
		d.droppedSessions += uint64(len(connections))
	}
	d.mu.Unlock()
	for _, connection := range connections {
		_ = connection.Close()
	}
	return len(connections)
}

// Close terminates every active simulated connection.
func (d *Device) Close() error {
	d.closeOnce.Do(func() {
		d.clock.unregister(d)
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

func (d *Device) serve(session *deviceSession) {
	connection := session.connection
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
	session.framer = framer
	id, payload, err := framer.ReadFrame()
	if err != nil || id != 1 {
		return
	}
	if _, err = wire.Decode(id, payload); err != nil {
		return
	}
	if session.send(&pb.HelloResponse{ApiVersionMajor: 1, ApiVersionMinor: 10, ServerInfo: "go-aioesphomeapi simulator", Name: d.scenario.Name}) != nil {
		return
	}
	if d.triggerFault(session, FaultAfterHello) {
		return
	}
	d.triggerNetwork(session, FaultAfterHello)

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
				if session.send(proto.Clone(entity)) != nil {
					return
				}
			}
			if d.triggerFault(session, FaultBeforeEntitiesDone) {
				return
			}
			d.triggerNetwork(session, FaultBeforeEntitiesDone)
			if session.send(&pb.ListEntitiesDoneResponse{}) != nil {
				return
			}
		case *pb.SubscribeStatesRequest:
			d.stateSerial.Lock()
			d.stateMu.Lock()
			session.subscribed = true
			snapshot := d.stateSnapshotLocked()
			d.stateMu.Unlock()
			for _, state := range snapshot {
				if session.send(state) != nil {
					d.stateSerial.Unlock()
					return
				}
			}
			d.stateSerial.Unlock()
			if d.triggerFault(session, FaultAfterInitialStates) {
				return
			}
			d.triggerNetwork(session, FaultAfterInitialStates)
		case *pb.SubscribeLogsRequest:
			for _, entry := range d.scenario.Logs {
				if entry.Level <= m.Level {
					if session.send(proto.Clone(entry)) != nil {
						return
					}
				}
			}
		case *pb.PingRequest:
			if session.send(&pb.PingResponse{}) != nil {
				return
			}
		case *pb.DisconnectRequest:
			_ = session.send(&pb.DisconnectResponse{})
			return
		case *pb.SwitchCommandRequest:
			d.record(m)
			if d.storeAndSend(session, &pb.SwitchStateResponse{Key: m.Key, State: m.State}) != nil {
				return
			}
		case *pb.NumberCommandRequest:
			d.record(m)
			if d.storeAndSend(session, &pb.NumberStateResponse{Key: m.Key, State: m.State}) != nil {
				return
			}
		case *pb.ButtonCommandRequest:
			d.record(m)
		case *pb.FanCommandRequest:
			d.record(m)
			d.stateSerial.Lock()
			d.stateMu.Lock()
			state, _ := d.currentStates[stateIdentity{family: "fan", key: m.Key}].(*pb.FanStateResponse)
			if state == nil {
				state = &pb.FanStateResponse{Key: m.Key}
			} else {
				state = proto.Clone(state).(*pb.FanStateResponse)
			}
			applyFan(state, m)
			d.storeStateLocked(state)
			d.stateMu.Unlock()
			err := session.send(state)
			d.stateSerial.Unlock()
			if err != nil {
				return
			}
		case *pb.LightCommandRequest:
			d.record(m)
			d.stateSerial.Lock()
			d.stateMu.Lock()
			state, _ := d.currentStates[stateIdentity{family: "light", key: m.Key}].(*pb.LightStateResponse)
			if state == nil {
				state = &pb.LightStateResponse{Key: m.Key}
			} else {
				state = proto.Clone(state).(*pb.LightStateResponse)
			}
			applyLight(state, m)
			d.storeStateLocked(state)
			d.stateMu.Unlock()
			err := session.send(state)
			d.stateSerial.Unlock()
			if err != nil {
				return
			}
		}
	}
}

func (d *Device) storeAndSend(session *deviceSession, state proto.Message) error {
	d.stateSerial.Lock()
	defer d.stateSerial.Unlock()
	d.stateMu.Lock()
	d.storeStateLocked(state)
	d.stateMu.Unlock()
	return session.send(state)
}

func (d *Device) advanceTimeline(now time.Duration) error {
	d.stateSerial.Lock()
	defer d.stateSerial.Unlock()
	for d.timelineIndex < len(d.scenario.StateTimeline) {
		event := d.scenario.StateTimeline[d.timelineIndex]
		if event.At > now {
			break
		}
		d.timelineIndex++
		d.stateMu.Lock()
		d.storeStateLocked(event.State)
		subscribers := make([]*deviceSession, 0, len(d.connections))
		d.mu.Lock()
		for _, session := range d.connections {
			if session.subscribed {
				subscribers = append(subscribers, session)
			}
		}
		d.mu.Unlock()
		d.stateMu.Unlock()
		for _, subscriber := range subscribers {
			// A concurrent disconnect is normal. State is already committed and
			// appears in the next subscriber's snapshot.
			_ = subscriber.send(proto.Clone(event.State))
		}
	}
	return nil
}

func (d *Device) initialStates() []proto.Message {
	if len(d.scenario.InitialStates) > 0 {
		return d.scenario.InitialStates
	}
	return d.scenario.States
}

func (d *Device) storeStateLocked(state proto.Message) {
	identity, ok := stateIdentityOf(state)
	if !ok {
		return
	}
	if _, exists := d.currentStates[identity]; !exists {
		d.stateOrder = append(d.stateOrder, identity)
	}
	d.currentStates[identity] = proto.Clone(state)
}

func (d *Device) stateSnapshotLocked() []proto.Message {
	result := make([]proto.Message, 0, len(d.stateOrder))
	for _, identity := range d.stateOrder {
		if state := d.currentStates[identity]; state != nil {
			result = append(result, proto.Clone(state))
		}
	}
	return result
}

func (d *Device) record(message proto.Message) {
	command := proto.Clone(message)
	d.commandMu.Lock()
	d.observeCommandLocked(command)
	select {
	case d.commands <- proto.Clone(command):
	default:
		d.mu.Lock()
		d.droppedCommands++
		d.mu.Unlock()
		if len(d.scenario.Commands) > 0 && d.commandErr == nil {
			d.setCommandErrorLocked(CommandOverflow)
		}
	}
	d.notifyCommandWaitersLocked()
	d.commandMu.Unlock()
}

func (s *deviceSession) send(message proto.Message) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.sendLocked(message)
}

func (s *deviceSession) sendLocked(message proto.Message) error {
	id, err := wire.MessageID(message)
	if err != nil {
		return err
	}
	payload, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	s.network.beginFrame()
	return s.network.endFrame(s.framer.WriteFrame(id, payload))
}

func (s *deviceSession) writeFrame(id uint32, payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.network.beginFrame()
	return s.network.endFrame(s.framer.WriteFrame(id, payload))
}

func stateIdentityOf(message proto.Message) (stateIdentity, bool) {
	switch m := message.(type) {
	case *pb.BinarySensorStateResponse:
		return stateIdentity{family: "binary_sensor", key: m.Key}, true
	case *pb.SensorStateResponse:
		return stateIdentity{family: "sensor", key: m.Key}, true
	case *pb.TextSensorStateResponse:
		return stateIdentity{family: "text_sensor", key: m.Key}, true
	case *pb.SwitchStateResponse:
		return stateIdentity{family: "switch", key: m.Key}, true
	case *pb.NumberStateResponse:
		return stateIdentity{family: "number", key: m.Key}, true
	case *pb.FanStateResponse:
		return stateIdentity{family: "fan", key: m.Key}, true
	case *pb.LightStateResponse:
		return stateIdentity{family: "light", key: m.Key}, true
	default:
		return stateIdentity{}, false
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

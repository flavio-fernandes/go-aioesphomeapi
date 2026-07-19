// Package aioesphomeapi is a small, secure Go client for ESPHome's Native API.
package aioesphomeapi

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/internal/wire"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

var (
	ErrEntityNotFound     = errors.New("entity not found")
	ErrEntityTypeMismatch = errors.New("entity type mismatch")
	ErrClientClosed       = errors.New("ESPHome client closed")
	ErrTransportPolicy    = wire.ErrTransportPolicy
)

var (
	// ErrNameResolution identifies a failed built-in .local mDNS lookup.
	ErrNameResolution = errors.New("ESPHome name resolution failed")
	// ErrHello identifies any failed stage of the Native API hello exchange.
	ErrHello = errors.New("ESPHome hello failed")
	// ErrPeerDisconnected means the device requested an orderly disconnect.
	ErrPeerDisconnected = errors.New("ESPHome peer requested disconnect")
	// ErrEventQueueFull means a slow consumer exhausted the bounded callback queue.
	ErrEventQueueFull = errors.New("ESPHome callback queue is full")
	// ErrPing identifies a failed caller-initiated liveness probe.
	ErrPing = errors.New("ESPHome ping failed")
	// ErrKeepalive identifies an automatic liveness probe the peer never
	// answered, or an invalid keepalive configuration.
	ErrKeepalive = errors.New("ESPHome keepalive failed")
	// ErrDeviceInfo identifies a failed device information exchange.
	ErrDeviceInfo = errors.New("ESPHome device info failed")
	// ErrNoiseHandshake identifies a failed encrypted transport handshake.
	ErrNoiseHandshake = wire.ErrNoiseHandshake
	// ErrPeerName identifies a configured peer-name mismatch on either transport.
	ErrPeerName = wire.ErrPeerName
	// ErrNoiseName is retained as a compatibility alias for ErrPeerName.
	ErrNoiseName = ErrPeerName
	// ErrNoiseKey identifies invalid Noise key configuration.
	ErrNoiseKey = wire.ErrNoiseKey
	// ErrNoiseKeyRejected means the peer explicitly rejected the configured key.
	ErrNoiseKeyRejected = wire.ErrNoiseKeyRejected
)

type callback func(proto.Message)
type listResult struct {
	messages []proto.Message
	done     chan struct{}
}

// Client represents exactly one connection. Reconnection policy intentionally
// belongs to the caller (MGMT's shared endpoint session in the primary use case).
type Client struct {
	framer             wire.Framer
	entities           *EntityRegistry
	done               chan struct{}
	closeOnce          sync.Once
	connected          atomic.Bool
	name, serverInfo   string
	apiMajor, apiMinor uint32
	closeReasonMu      sync.RWMutex
	closeReason        error

	handlerMu      sync.RWMutex
	nextHandler    uint64
	handlers       map[uint32]map[uint64]callback
	events         chan proto.Message
	callbacksDone  chan struct{}
	listMu         sync.Mutex
	list           *listResult
	pingGate       chan struct{}
	deviceInfoGate chan struct{}
}

// Dial connects using a background context.
func Dial(address string, timeout time.Duration, opts ...Option) (*Client, error) {
	return DialWithContext(context.Background(), address, timeout, opts...)
}

// DialWithContext connects, establishes the selected secure transport, and
// completes the Native API Hello exchange before returning.
func DialWithContext(ctx context.Context, address string, timeout time.Duration, opts ...Option) (*Client, error) {
	cfg := config{clientInfo: "go-aioesphomeapi", maxFrameSize: wire.DefaultMaxFrameSize, callbackQueueSize: 256}
	for _, option := range opts {
		if option != nil {
			option(&cfg)
		}
	}
	if cfg.encryptionKey == "" && !cfg.insecurePlaintext {
		return nil, ErrTransportPolicy
	}
	if cfg.encryptionKey != "" && cfg.insecurePlaintext {
		return nil, ErrTransportPolicy
	}
	if cfg.dialContext == nil {
		cfg.dialContext = defaultDialer(timeout)
	}
	if cfg.callbackQueueSize <= 0 {
		cfg.callbackQueueSize = 256
	}
	if cfg.keepaliveRequested && (cfg.keepaliveInterval <= 0 || cfg.keepaliveTimeout <= 0) {
		return nil, fmt.Errorf("%w: interval and timeout must both be positive", ErrKeepalive)
	}

	establishCtx := ctx
	cancelEstablish := func() {}
	if timeout > 0 {
		establishCtx, cancelEstablish = context.WithTimeout(ctx, timeout)
	} else {
		establishCtx, cancelEstablish = context.WithCancel(ctx)
	}
	defer cancelEstablish()

	conn, err := cfg.dialContext(establishCtx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial ESPHome target %q: %w", address, err)
	}
	stopEstablishmentClose := context.AfterFunc(establishCtx, func() { _ = conn.Close() })
	defer stopEstablishmentClose()
	establishmentDeadline, hasEstablishmentDeadline := establishCtx.Deadline()
	if hasEstablishmentDeadline {
		if err := conn.SetDeadline(establishmentDeadline); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			_ = conn.Close()
			return nil, fmt.Errorf("set establishment deadline for ESPHome target %q: %w", address, err)
		}
	}
	var framer wire.Framer
	if cfg.encryptionKey != "" {
		// Strict decoding rejects non-zero trailing pad bits. CR and LF remain
		// accepted by encoding/base64, so wrapped canonical ESPHome keys work,
		// while every accepted textual form re-encodes to the single canonical
		// value that rejection diagnostics know how to redact.
		key, decodeErr := base64.StdEncoding.Strict().DecodeString(cfg.encryptionKey)
		if decodeErr != nil {
			for i := range key {
				key[i] = 0
			}
			conn.Close()
			return nil, fmt.Errorf("configure Noise for ESPHome target %q: %w: %w", address, ErrNoiseKey, decodeErr)
		}
		if len(key) != 32 {
			for i := range key {
				key[i] = 0
			}
			conn.Close()
			return nil, fmt.Errorf("configure Noise for ESPHome target %q: %w", address, ErrNoiseKey)
		}
		handshakeTimeout := timeout
		if hasEstablishmentDeadline {
			handshakeTimeout = time.Until(establishmentDeadline)
		}
		framer, err = wire.NewNoiseClientFramer(conn, key, cfg.expectedName, handshakeTimeout, cfg.maxFrameSize)
		for i := range key {
			key[i] = 0
		}
	} else {
		framer = wire.NewPlainFramer(conn, cfg.maxFrameSize)
	}
	if err != nil {
		_ = conn.Close()
		if cause := establishmentCause(establishCtx, err); cause != nil {
			return nil, fmt.Errorf("establish Noise session with ESPHome target %q: %w: establishment context: %w", address, err, cause)
		}
		return nil, fmt.Errorf("establish Noise session with ESPHome target %q: %w", address, err)
	}
	// The Noise framer clears its handshake deadline. Restore the single
	// establishment deadline so Hello consumes only the original remaining budget.
	if hasEstablishmentDeadline {
		if err := conn.SetDeadline(establishmentDeadline); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("restore hello deadline for ESPHome target %q: %w", address, err)
		}
	}
	c := newClient(framer, cfg.callbackQueueSize)
	if err := c.hello(cfg.clientInfo, cfg.expectedName); err != nil {
		c.Close()
		if cause := establishmentCause(establishCtx, err); cause != nil {
			return nil, fmt.Errorf("complete hello with ESPHome target %q: %w: establishment context: %w", address, err, cause)
		}
		return nil, fmt.Errorf("complete hello with ESPHome target %q: %w", address, err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		c.Close()
		return nil, fmt.Errorf("clear establishment deadline for ESPHome target %q: %w", address, err)
	}
	if !stopEstablishmentClose() {
		c.Close()
		if cause := context.Cause(establishCtx); cause != nil {
			return nil, fmt.Errorf("complete connection to ESPHome target %q: %w", address, cause)
		}
		return nil, fmt.Errorf("complete connection to ESPHome target %q: establishment ended", address)
	}
	c.connected.Store(true)
	go c.dispatchLoop()
	go c.readLoop(ctx)
	if cfg.keepaliveRequested {
		go c.keepaliveLoop(cfg.keepaliveInterval, cfg.keepaliveTimeout)
	}
	return c, nil
}

func establishmentCause(ctx context.Context, err error) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	// DialWithContext installs the context's deadline on the connection. The
	// socket deadline can wake a few scheduler ticks before the context timer;
	// retain the caller-visible deadline category in that equivalent race.
	if _, hasDeadline := ctx.Deadline(); hasDeadline && errors.Is(err, os.ErrDeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}

func newClient(framer wire.Framer, callbackQueueSize int) *Client {
	client := &Client{
		framer:         framer,
		entities:       newEntityRegistry(),
		done:           make(chan struct{}),
		handlers:       make(map[uint32]map[uint64]callback),
		events:         make(chan proto.Message, callbackQueueSize),
		callbacksDone:  make(chan struct{}),
		pingGate:       make(chan struct{}, 1),
		deviceInfoGate: make(chan struct{}, 1),
	}
	client.pingGate <- struct{}{}
	client.deviceInfoGate <- struct{}{}
	return client
}

func (c *Client) hello(clientInfo, expectedName string) error {
	if err := c.send(&pb.HelloRequest{ClientInfo: clientInfo, ApiVersionMajor: 1, ApiVersionMinor: 10}); err != nil {
		return fmt.Errorf("%w: send request: %w", ErrHello, err)
	}
	id, payload, err := c.framer.ReadFrame()
	if err != nil {
		return fmt.Errorf("%w: read response: %w", ErrHello, err)
	}
	if id != 2 {
		return fmt.Errorf("%w: unexpected response message ID %d", ErrHello, id)
	}
	message, err := wire.Decode(id, payload)
	if err != nil {
		return fmt.Errorf("%w: decode response: %w", ErrHello, err)
	}
	response, ok := message.(*pb.HelloResponse)
	if !ok {
		return fmt.Errorf("%w: unexpected response type %T", ErrHello, message)
	}
	if response.ApiVersionMajor != 1 {
		return fmt.Errorf("%w: unsupported API major version %d", ErrHello, response.ApiVersionMajor)
	}
	if expectedName != "" && response.Name != expectedName {
		return fmt.Errorf("%w: %w", ErrHello, ErrPeerName)
	}
	c.apiMajor, c.apiMinor, c.serverInfo, c.name = response.ApiVersionMajor, response.ApiVersionMinor, response.ServerInfo, response.Name
	return nil
}

func (c *Client) readLoop(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			c.shutdown(fmt.Errorf("ESPHome connection context ended: %w", context.Cause(ctx)))
		case <-c.done:
		}
	}()
	for {
		id, payload, err := c.framer.ReadFrame()
		if err != nil {
			c.shutdown(fmt.Errorf("read ESPHome frame: %w", err))
			return
		}
		message, err := wire.Decode(id, payload)
		if err != nil {
			if errors.Is(err, wire.ErrUnknownMessage) {
				continue
			}
			c.shutdown(fmt.Errorf("decode ESPHome message ID %d: %w", id, err))
			return
		}
		if _, ok := message.(*pb.PingRequest); ok {
			if err := c.send(&pb.PingResponse{}); err != nil {
				c.shutdown(fmt.Errorf("answer ESPHome ping: %w", err))
				return
			}
			continue
		}
		if _, ok := message.(*pb.DisconnectRequest); ok {
			if err := c.send(&pb.DisconnectResponse{}); err != nil {
				c.shutdown(fmt.Errorf("answer ESPHome disconnect: %w", err))
				return
			}
			c.shutdown(ErrPeerDisconnected)
			return
		}
		switch message.(type) {
		case *pb.PingResponse, *pb.DeviceInfoResponse:
			// Probe and device-info completions bypass the bounded callback
			// queue: their only handlers are the client's own non-blocking
			// completion hooks, so a slow subscriber callback can never delay
			// a liveness verdict or make keepalive misreport a healthy peer.
			c.deliverDirect(message)
			continue
		}
		select {
		case c.events <- message:
		case <-c.done:
			return
		default:
			c.shutdown(ErrEventQueueFull)
			return
		}
	}
}

func (c *Client) dispatchLoop() {
	defer close(c.callbacksDone)
	for {
		select {
		case <-c.done:
			return
		default:
		}
		select {
		case message := <-c.events:
			select {
			case <-c.done:
				return
			default:
			}
			c.entities.handle(message)
			c.handleList(message)
			id, err := wire.MessageID(message)
			if err != nil {
				continue
			}
			c.handlerMu.RLock()
			callbacks := make([]callback, 0, len(c.handlers[id]))
			for _, fn := range c.handlers[id] {
				callbacks = append(callbacks, fn)
			}
			c.handlerMu.RUnlock()
			for _, fn := range callbacks {
				select {
				case <-c.done:
					return
				default:
				}
				fn(message)
			}
		case <-c.done:
			return
		}
	}
}

func (c *Client) handleList(message proto.Message) {
	c.listMu.Lock()
	defer c.listMu.Unlock()
	if c.list == nil {
		return
	}
	if _, ok := message.(*pb.ListEntitiesDoneResponse); ok {
		pending := c.list
		c.list = nil
		close(pending.done)
		return
	}
	if isListEntity(message) {
		c.list.messages = append(c.list.messages, message)
	}
}

func isListEntity(message proto.Message) bool {
	name := string(message.ProtoReflect().Descriptor().Name())
	return len(name) > len("ListEntitiesResponse") && len(name) >= 12 && name[:12] == "ListEntities" && name != "ListEntitiesDoneResponse"
}

func (c *Client) send(message proto.Message) error {
	if c.framer == nil {
		return ErrClientClosed
	}
	select {
	case <-c.done:
		return ErrClientClosed
	default:
	}
	id, err := wire.MessageID(message)
	if err != nil {
		return fmt.Errorf("identify ESPHome message: %w", err)
	}
	payload, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode ESPHome message %T: %w", message, err)
	}
	if err := c.framer.WriteFrame(id, payload); err != nil {
		return fmt.Errorf("write ESPHome message %T: %w", message, err)
	}
	return nil
}

func (c *Client) on(id uint32, fn callback) func() {
	c.handlerMu.Lock()
	c.nextHandler++
	token := c.nextHandler
	if c.handlers[id] == nil {
		c.handlers[id] = make(map[uint64]callback)
	}
	c.handlers[id][token] = fn
	c.handlerMu.Unlock()
	return func() { c.handlerMu.Lock(); delete(c.handlers[id], token); c.handlerMu.Unlock() }
}

func (c *Client) shutdown(reason error) {
	c.closeOnce.Do(func() {
		c.closeReasonMu.Lock()
		c.closeReason = reason
		c.closeReasonMu.Unlock()
		c.connected.Store(false)
		_ = c.framer.Close()
		close(c.done)
	})
}
func (c *Client) Close() error                 { c.shutdown(nil); return nil }
func (c *Client) Done() <-chan struct{}        { return c.done }
func (c *Client) Connected() bool              { return c.connected.Load() }
func (c *Client) Name() string                 { return c.name }
func (c *Client) ServerInfo() string           { return c.serverInfo }
func (c *Client) APIVersion() (uint32, uint32) { return c.apiMajor, c.apiMinor }
func (c *Client) Entities() *EntityRegistry    { return c.entities }

// CloseReason reports why an established connection ended. It returns nil
// while the client is open and after an intentional Close call.
func (c *Client) CloseReason() error {
	c.closeReasonMu.RLock()
	defer c.closeReasonMu.RUnlock()
	return c.closeReason
}

// WaitCallbacks waits for the serial callback dispatcher to stop after the
// client closes. It never forces a caller callback to return; the supplied
// context bounds the wait instead.
func (c *Client) WaitCallbacks(ctx context.Context) error {
	if ctx == nil {
		return errors.New("wait for ESPHome callbacks: nil context")
	}
	select {
	case <-c.callbacksDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for ESPHome callbacks: %w", context.Cause(ctx))
	}
}

// ListEntities refreshes the entity registry and returns the raw descriptors.
func (c *Client) ListEntities() ([]proto.Message, error) {
	return c.ListEntitiesWithTimeout(10 * time.Second)
}
func (c *Client) ListEntitiesWithTimeout(timeout time.Duration) ([]proto.Message, error) {
	c.listMu.Lock()
	if c.list != nil {
		c.listMu.Unlock()
		return nil, errors.New("entity listing already in progress")
	}
	pending := &listResult{done: make(chan struct{})}
	c.list = pending
	c.listMu.Unlock()
	defer func() {
		c.listMu.Lock()
		if c.list == pending {
			c.list = nil
		}
		c.listMu.Unlock()
	}()
	if err := c.send(&pb.ListEntitiesRequest{}); err != nil {
		return nil, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-pending.done:
		return append([]proto.Message(nil), pending.messages...), nil
	case <-timer.C:
		return nil, errors.New("entity listing timed out")
	case <-c.done:
		return nil, ErrClientClosed
	}
}

// SubscribeStates starts the device state stream and invokes handler serially.
func (c *Client) SubscribeStates(handler func(proto.Message)) (func(), error) {
	var removes []func()
	if handler != nil {
		for _, id := range stateResponseIDs {
			removes = append(removes, c.on(id, handler))
		}
	}
	unsubscribe := func() {
		for _, remove := range removes {
			remove()
		}
	}
	if err := c.send(&pb.SubscribeStatesRequest{}); err != nil {
		unsubscribe()
		return nil, err
	}
	return unsubscribe, nil
}

var stateResponseIDs = []uint32{21, 22, 23, 24, 25, 26, 27, 44, 47, 50, 53, 56, 59, 64, 95, 98, 101, 104, 108, 110, 113, 117, 126, 133}

// SubscribeLogs starts native logger streaming.
func (c *Client) SubscribeLogs(level pb.LogLevel, handler func(*pb.SubscribeLogsResponse)) (func(), error) {
	remove := c.on(29, func(message proto.Message) {
		if logMessage, ok := message.(*pb.SubscribeLogsResponse); ok && handler != nil {
			handler(logMessage)
		}
	})
	if err := c.send(&pb.SubscribeLogsRequest{Level: level}); err != nil {
		remove()
		return nil, err
	}
	return remove, nil
}

// Ping performs one context-bounded Native API liveness probe. Concurrent
// probes are serialized so one response can never satisfy multiple callers.
// A probe whose context ends once the request may be in flight closes the
// ambiguous connection; a late response can therefore never satisfy a later
// probe.
func (c *Client) Ping(ctx context.Context) error { return c.probe(ctx, ErrPing) }

// errProbeNotStarted classifies a probe that ended before it acquired the
// serialization gate, so no request was sent and nothing was proven about the
// peer. Its text keeps the established "wait to start" wording.
var errProbeNotStarted = errors.New("wait to start")

// probe implements Ping for both the caller-initiated and the automatic
// keepalive category so the recorded close reason names the actual initiator.
func (c *Client) probe(ctx context.Context, category error) error {
	if ctx == nil {
		return fmt.Errorf("%w: nil context", category)
	}
	// An already-ended context or client must never race the free gate below
	// into the send path, so both are checked on their own first.
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %w: %w", category, errProbeNotStarted, context.Cause(ctx))
	}
	select {
	case <-c.done:
		return c.closedError(category)
	default:
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %w: %w", category, errProbeNotStarted, context.Cause(ctx))
	case <-c.done:
		return c.closedError(category)
	case <-c.pingGate:
	}
	defer func() { c.pingGate <- struct{}{} }()

	response := make(chan struct{}, 1)
	remove := c.on(8, func(message proto.Message) {
		if _, ok := message.(*pb.PingResponse); ok {
			select {
			case response <- struct{}{}:
			default:
			}
		}
	})
	defer remove()
	// The watchdog bounds the whole exchange, including a send blocked by a
	// peer that accepted the connection but stopped reading: context expiry
	// records the timeout reason and closes the connection, which unblocks
	// any stuck write. Exactly one of completion and expiry wins the verdict,
	// so a response that arrives at the deadline instant can never be
	// reported as success while the watchdog closes a healthy connection.
	var verdict atomic.Uint32
	const verdictCompleted, verdictExpired = 1, 2
	watchdog := context.AfterFunc(ctx, func() {
		if verdict.CompareAndSwap(0, verdictExpired) {
			c.shutdown(fmt.Errorf("%w: wait for response: %w", category, context.Cause(ctx)))
		}
	})
	defer watchdog()
	if err := c.send(&pb.PingRequest{}); err != nil {
		// A send the watchdog had to unblock still reports the context cause,
		// so a timeout remains distinguishable from a transport failure.
		if ctx.Err() != nil {
			return fmt.Errorf("%w: send request: %w: %w", category, err, context.Cause(ctx))
		}
		return fmt.Errorf("%w: send request: %w", category, err)
	}
	select {
	case <-response:
		if verdict.CompareAndSwap(0, verdictCompleted) {
			return nil
		}
		return fmt.Errorf("%w: wait for response: %w", category, context.Cause(ctx))
	case <-ctx.Done():
		reason := fmt.Errorf("%w: wait for response: %w", category, context.Cause(ctx))
		if verdict.CompareAndSwap(0, verdictExpired) {
			c.shutdown(reason)
		}
		return reason
	case <-c.done:
		return c.closedError(category)
	}
}

// keepaliveLoop periodically proves peer liveness after WithKeepalive. Every
// probe carries its own timeout, so one silent peer can never park the loop.
// A cycle that never acquired the shared gate yields to the caller-initiated
// probe holding it; the first probe that actually fails records an
// ErrKeepalive close reason and ends the loop. Deliberate Close ends the
// loop without a failure.
func (c *Client) keepaliveLoop(interval, timeout time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-timer.C:
		}
		probeCtx, cancel := context.WithTimeout(context.Background(), timeout)
		err := c.probe(probeCtx, ErrKeepalive)
		cancel()
		if err != nil {
			// A cycle that never acquired the gate proved nothing about the
			// peer: a caller-initiated probe is in flight and its own outcome
			// governs liveness, so this cycle yields instead of condemning a
			// healthy connection.
			if errors.Is(err, errProbeNotStarted) {
				timer.Reset(interval)
				continue
			}
			c.shutdown(err)
			return
		}
		timer.Reset(interval)
	}
}

// DeviceInfo performs one context-bounded device information exchange and
// returns the peer's static identity description. Concurrent exchanges are
// serialized so one response can never satisfy multiple callers, and an
// exchange whose context ends once the request may be in flight closes the
// ambiguous connection for the same reason Ping does: the protocol carries
// no correlation ID, so a late response must never complete a different,
// later exchange.
func (c *Client) DeviceInfo(ctx context.Context) (*pb.DeviceInfoResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: nil context", ErrDeviceInfo)
	}
	// See probe: an already-ended context or client must never race the free
	// gate.
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w: wait to start: %w", ErrDeviceInfo, context.Cause(ctx))
	}
	select {
	case <-c.done:
		return nil, c.closedError(ErrDeviceInfo)
	default:
	}
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: wait to start: %w", ErrDeviceInfo, context.Cause(ctx))
	case <-c.done:
		return nil, c.closedError(ErrDeviceInfo)
	case <-c.deviceInfoGate:
	}
	defer func() { c.deviceInfoGate <- struct{}{} }()

	response := make(chan *pb.DeviceInfoResponse, 1)
	remove := c.on(10, func(message proto.Message) {
		if info, ok := message.(*pb.DeviceInfoResponse); ok {
			select {
			case response <- info:
			default:
			}
		}
	})
	defer remove()
	// Same watchdog contract as probe: bound the whole exchange, including a
	// send blocked by a peer that stopped reading, and let exactly one of
	// completion and expiry win the verdict.
	var verdict atomic.Uint32
	const verdictCompleted, verdictExpired = 1, 2
	watchdog := context.AfterFunc(ctx, func() {
		if verdict.CompareAndSwap(0, verdictExpired) {
			c.shutdown(fmt.Errorf("%w: wait for response: %w", ErrDeviceInfo, context.Cause(ctx)))
		}
	})
	defer watchdog()
	if err := c.send(&pb.DeviceInfoRequest{}); err != nil {
		// See probe: a send the watchdog had to unblock reports the context
		// cause alongside the transport failure.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: send request: %w: %w", ErrDeviceInfo, err, context.Cause(ctx))
		}
		return nil, fmt.Errorf("%w: send request: %w", ErrDeviceInfo, err)
	}
	select {
	case info := <-response:
		if verdict.CompareAndSwap(0, verdictCompleted) {
			return info, nil
		}
		return nil, fmt.Errorf("%w: wait for response: %w", ErrDeviceInfo, context.Cause(ctx))
	case <-ctx.Done():
		reason := fmt.Errorf("%w: wait for response: %w", ErrDeviceInfo, context.Cause(ctx))
		if verdict.CompareAndSwap(0, verdictExpired) {
			c.shutdown(reason)
		}
		return nil, reason
	case <-c.done:
		return nil, c.closedError(ErrDeviceInfo)
	}
}

// deliverDirect invokes the internal completion handlers for one message
// without entering the bounded subscriber queue. Only the client's own
// non-blocking hooks register for these message IDs.
func (c *Client) deliverDirect(message proto.Message) {
	id, err := wire.MessageID(message)
	if err != nil {
		return
	}
	c.handlerMu.RLock()
	callbacks := make([]callback, 0, len(c.handlers[id]))
	for _, fn := range c.handlers[id] {
		callbacks = append(callbacks, fn)
	}
	c.handlerMu.RUnlock()
	for _, fn := range callbacks {
		fn(message)
	}
}

func (c *Client) closedError(category error) error {
	if reason := c.CloseReason(); reason != nil {
		return fmt.Errorf("%w: connection closed: %w", category, reason)
	}
	return fmt.Errorf("%w: %w", category, ErrClientClosed)
}

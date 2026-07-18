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

	handlerMu     sync.RWMutex
	nextHandler   uint64
	handlers      map[uint32]map[uint64]callback
	events        chan proto.Message
	callbacksDone chan struct{}
	listMu        sync.Mutex
	list          *listResult
	pingGate      chan struct{}
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
		key, decodeErr := base64.StdEncoding.DecodeString(cfg.encryptionKey)
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
		framer:        framer,
		entities:      newEntityRegistry(),
		done:          make(chan struct{}),
		handlers:      make(map[uint32]map[uint64]callback),
		events:        make(chan proto.Message, callbackQueueSize),
		callbacksDone: make(chan struct{}),
		pingGate:      make(chan struct{}, 1),
	}
	client.pingGate <- struct{}{}
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
// A probe that times out after sending closes the ambiguous connection; a late
// response can therefore never satisfy a later probe.
func (c *Client) Ping(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: nil context", ErrPing)
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: wait to start: %w", ErrPing, context.Cause(ctx))
	case <-c.done:
		return c.pingClosedError()
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
	if err := c.send(&pb.PingRequest{}); err != nil {
		return fmt.Errorf("%w: send request: %w", ErrPing, err)
	}
	select {
	case <-response:
		return nil
	case <-ctx.Done():
		reason := fmt.Errorf("%w: wait for response: %w", ErrPing, context.Cause(ctx))
		c.shutdown(reason)
		return reason
	case <-c.done:
		return c.pingClosedError()
	}
}

func (c *Client) pingClosedError() error {
	if reason := c.CloseReason(); reason != nil {
		return fmt.Errorf("%w: connection closed: %w", ErrPing, reason)
	}
	return fmt.Errorf("%w: %w", ErrPing, ErrClientClosed)
}

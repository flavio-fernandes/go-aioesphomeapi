package aioesphomeapi_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

// countingDialer wraps the simulator dialer so a test can prove exactly how
// many times a connection owner dialed. The client stores no dialer after
// DialWithContext returns, so a count of one is structural proof of the
// one-dial-owner contract: the library cannot redial on its own.
func countingDialer(device *simulator.Device, dials *atomic.Int32) api.DialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dials.Add(1)
		return device.DialContext(ctx, network, address)
	}
}

func TestDeviceInfoTypedExchange(t *testing.T) {
	device := simulator.New(simulator.BasicIOScenario())
	t.Cleanup(func() { _ = device.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.DialWithContext(ctx, "simulator:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	info, err := client.DeviceInfo(ctx)
	if err != nil {
		t.Fatalf("device info: %v", err)
	}
	if info.Name != "basic-io-simulator" || info.FriendlyName != "basic-io-simulator" {
		t.Fatalf("unexpected device identity: %q / %q", info.Name, info.FriendlyName)
	}
	if info.MacAddress != "02:00:00:00:00:01" {
		t.Fatalf("expected the fixed synthetic MAC, got %q", info.MacAddress)
	}
	if info.EsphomeVersion != "2026.7.0" || info.Model != "go-aioesphomeapi-simulator" {
		t.Fatalf("unexpected device description: %q / %q", info.EsphomeVersion, info.Model)
	}
	if !info.ApiEncryptionSupported {
		t.Fatal("the Noise simulator must report encryption support")
	}
	// The serialization gate must be released for the next exchange.
	if _, err := client.DeviceInfo(ctx); err != nil {
		t.Fatalf("second device info: %v", err)
	}
}

func TestDeviceInfoContextGates(t *testing.T) {
	device := simulator.New(simulator.BasicIOScenario())
	t.Cleanup(func() { _ = device.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.DialWithContext(ctx, "simulator:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	canceled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if _, err := client.DeviceInfo(canceled); !errors.Is(err, api.ErrDeviceInfo) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: %v", err)
	}
	//lint:ignore SA1012 the nil-context contract is part of the public API.
	if _, err := client.DeviceInfo(nil); !errors.Is(err, api.ErrDeviceInfo) { //nolint:staticcheck
		t.Fatalf("nil context: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := client.DeviceInfo(ctx); !errors.Is(err, api.ErrDeviceInfo) || !errors.Is(err, api.ErrClientClosed) {
		t.Fatalf("closed client: %v", err)
	}
}

func TestDeviceInfoTimeoutClosesAmbiguousConnection(t *testing.T) {
	scenario := simulator.BasicIOScenario()
	scenario.Faults = []simulator.Fault{{Trigger: simulator.FaultAfterHello, Action: simulator.FaultStall}}
	device := simulator.New(scenario)
	t.Cleanup(func() { _ = device.Close() })
	client, err := api.Dial("simulator:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	// The stalled device stops reading entirely, so the exchange may fail
	// either while waiting for the response or while still writing the
	// request into the synchronous in-memory pipe. Both paths must stay
	// bounded by the context and must carry the typed category.
	_, err = client.DeviceInfo(ctx)
	if !errors.Is(err, api.ErrDeviceInfo) {
		t.Fatalf("stalled device info: %v", err)
	}
	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("ambiguous connection was not closed")
	}
	reason := client.CloseReason()
	if !errors.Is(reason, api.ErrDeviceInfo) || !errors.Is(reason, context.DeadlineExceeded) {
		t.Fatalf("close reason: %v", reason)
	}
}

func TestKeepaliveMaintainsHealthyConnection(t *testing.T) {
	device := simulator.New(simulator.BasicIOScenario())
	t.Cleanup(func() { _ = device.Close() })
	options := append(device.ClientOptions(), api.WithKeepalive(2*time.Millisecond, 2*time.Second))
	client, err := api.Dial("simulator:6053", time.Second, options...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	waitUntil(t, "the device to answer several automatic probes", func() bool {
		return device.Stats().AnsweredPings >= 3
	})
	if !client.Connected() {
		t.Fatal("healthy keepalive must not end the connection")
	}
	if reason := client.CloseReason(); reason != nil {
		t.Fatalf("healthy keepalive recorded a failure: %v", reason)
	}
}

func TestKeepaliveClosesSilentPeerWithoutRedial(t *testing.T) {
	scenario := simulator.BasicIOScenario()
	scenario.Faults = []simulator.Fault{{Trigger: simulator.FaultAfterHello, Action: simulator.FaultStall}}
	device := simulator.New(scenario)
	t.Cleanup(func() { _ = device.Close() })
	var dials atomic.Int32
	options := append(device.ClientOptions(),
		api.WithDialContext(countingDialer(device, &dials)),
		api.WithKeepalive(5*time.Millisecond, 100*time.Millisecond))
	client, err := api.Dial("simulator:6053", time.Second, options...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("keepalive never detected the silent peer")
	}
	reason := client.CloseReason()
	if !errors.Is(reason, api.ErrKeepalive) || !errors.Is(reason, context.DeadlineExceeded) {
		t.Fatalf("close reason: %v", reason)
	}
	if errors.Is(reason, api.ErrPing) || errors.Is(reason, api.ErrPeerDisconnected) {
		t.Fatalf("close reason blames the wrong initiator: %v", reason)
	}
	// A later manual probe reports both its own category and the recorded cause.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); !errors.Is(err, api.ErrPing) || !errors.Is(err, api.ErrKeepalive) {
		t.Fatalf("post-loss ping: %v", err)
	}
	waitUntil(t, "client-owned goroutines to end after keepalive loss", func() bool {
		return clientOwnedGoroutines() == 0
	})
	if got := dials.Load(); got != 1 {
		t.Fatalf("the one dial owner dialed %d times", got)
	}
	if accepted := device.Stats().AcceptedConnections; accepted != 1 {
		t.Fatalf("device accepted %d connections, expected the single original dial", accepted)
	}
}

func TestKeepaliveRejectsInvalidConfiguration(t *testing.T) {
	device := simulator.New(simulator.BasicIOScenario())
	t.Cleanup(func() { _ = device.Close() })
	var dials atomic.Int32
	for _, invalid := range []api.Option{
		api.WithKeepalive(0, time.Second),
		api.WithKeepalive(time.Second, -time.Second),
	} {
		options := append(device.ClientOptions(), api.WithDialContext(countingDialer(device, &dials)), invalid)
		if _, err := api.Dial("simulator:6053", time.Second, options...); !errors.Is(err, api.ErrKeepalive) {
			t.Fatalf("invalid keepalive configuration: %v", err)
		}
	}
	if got := dials.Load(); got != 0 {
		t.Fatalf("invalid configuration still dialed %d times", got)
	}
}

// TestProbeAndDeviceInfoIsolatedFromSlowCallbacks parks the serial dispatcher
// inside a subscriber callback and proves liveness and device-info exchanges
// still complete: subscriber callbacks run off the read loop, and probe
// completions bypass the subscriber queue, so a slow consumer can never make
// keepalive misreport a healthy peer as dead.
func TestProbeAndDeviceInfoIsolatedFromSlowCallbacks(t *testing.T) {
	device := simulator.New(simulator.BasicIOScenario())
	t.Cleanup(func() { _ = device.Close() })
	client, err := api.Dial("simulator:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	blocked := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unsubscribe, err := client.SubscribeStates(func(proto.Message) {
		once.Do(func() {
			close(blocked)
			<-release
		})
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()
	defer close(release)
	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("no initial state callback arrived")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping while a callback is blocked: %v", err)
	}
	info, err := client.DeviceInfo(ctx)
	if err != nil {
		t.Fatalf("device info while a callback is blocked: %v", err)
	}
	if info.Name != "basic-io-simulator" {
		t.Fatalf("unexpected device info: %q", info.Name)
	}
}

// TestLifecycleStateMachineOnVirtualTime walks one connection through its
// externally observable states — established, subscribed, virtual-time state
// flow, deliberate close, terminally closed — with the shared manual clock as
// the only time source for device behavior. No wall-clock sleep orders events.
func TestLifecycleStateMachineOnVirtualTime(t *testing.T) {
	clock := simulator.NewManualClock()
	scenario := simulator.Scenario{
		Name: "lifecycle-virtual-time",
		Entities: []proto.Message{
			&pb.ListEntitiesBinarySensorResponse{Key: 1, ObjectId: "presence", Name: "Presence"},
		},
		InitialStates: []proto.Message{
			&pb.BinarySensorStateResponse{Key: 1, State: false},
		},
		StateTimeline: []simulator.StateEvent{
			{At: time.Second, State: &pb.BinarySensorStateResponse{Key: 1, State: true}},
			{At: 2 * time.Second, State: &pb.BinarySensorStateResponse{Key: 1, State: false}},
		},
	}
	device := simulator.New(scenario, simulator.WithManualClock(clock))
	t.Cleanup(func() { _ = device.Close() })
	var dials atomic.Int32
	options := append(device.ClientOptions(), api.WithDialContext(countingDialer(device, &dials)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.DialWithContext(ctx, "simulator:6053", time.Second, options...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// Established: identity and version are fixed, nothing has failed.
	if !client.Connected() {
		t.Fatal("established client must report connected")
	}
	select {
	case <-client.Done():
		t.Fatal("established client must not be done")
	default:
	}
	if major, _ := client.APIVersion(); major != 1 {
		t.Fatalf("unexpected API major version %d", major)
	}
	if client.Name() != "lifecycle-virtual-time" || client.ServerInfo() == "" {
		t.Fatalf("unexpected identity %q / %q", client.Name(), client.ServerInfo())
	}
	if reason := client.CloseReason(); reason != nil {
		t.Fatalf("established client already failed: %v", reason)
	}

	// Subscribed: the initial snapshot, then each virtual-time transition, in
	// declaration order.
	states := make(chan bool, 8)
	unsubscribe, err := client.SubscribeStates(func(message proto.Message) {
		if state, ok := message.(*pb.BinarySensorStateResponse); ok {
			states <- state.State
		}
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()
	expectState := func(phase string, want bool) {
		t.Helper()
		select {
		case got := <-states:
			if got != want {
				t.Fatalf("%s: state %v, want %v", phase, got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: no state arrived", phase)
		}
	}
	expectState("initial snapshot", false)
	if err := clock.Advance(time.Second); err != nil {
		t.Fatalf("advance to 1s: %v", err)
	}
	expectState("virtual second one", true)
	if err := clock.Advance(time.Second); err != nil {
		t.Fatalf("advance to 2s: %v", err)
	}
	expectState("virtual second two", false)

	// Deliberate close: idempotent, callbacks drain, no recorded failure.
	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Done never closed")
	}
	if err := client.WaitCallbacks(ctx); err != nil {
		t.Fatalf("wait for callbacks: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if client.Connected() {
		t.Fatal("closed client must not report connected")
	}
	if reason := client.CloseReason(); reason != nil {
		t.Fatalf("deliberate close recorded a failure: %v", reason)
	}

	// Terminally closed: every operation fails with its own typed category
	// plus the closed-client cause, and the client cannot dial again.
	if err := client.Ping(ctx); !errors.Is(err, api.ErrPing) || !errors.Is(err, api.ErrClientClosed) {
		t.Fatalf("post-close ping: %v", err)
	}
	if _, err := client.DeviceInfo(ctx); !errors.Is(err, api.ErrDeviceInfo) || !errors.Is(err, api.ErrClientClosed) {
		t.Fatalf("post-close device info: %v", err)
	}
	waitUntil(t, "client-owned goroutines to end after close", func() bool {
		return clientOwnedGoroutines() == 0
	})
	if got := dials.Load(); got != 1 {
		t.Fatalf("the one dial owner dialed %d times", got)
	}
}

// TestPeerLossLifecycleCommandInterruptionNoReplay drops the connection at an
// exact protocol point and proves the failure lifecycle: a typed close reason,
// a command interrupted by the loss fails loudly instead of being queued, the
// client never redials, no goroutine survives, and a later connection by a new
// owner starts with no replayed command.
func TestPeerLossLifecycleCommandInterruptionNoReplay(t *testing.T) {
	scenario := simulator.BasicIOScenario()
	scenario.Faults = []simulator.Fault{{Trigger: simulator.FaultAfterInitialStates, Action: simulator.FaultDropConnection}}
	device := simulator.New(scenario)
	t.Cleanup(func() { _ = device.Close() })
	var dials atomic.Int32
	options := append(device.ClientOptions(), api.WithDialContext(countingDialer(device, &dials)))
	client, err := api.Dial("simulator:6053", time.Second, options...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.ListEntities(); err != nil {
		t.Fatalf("list entities: %v", err)
	}
	if _, err := client.SubscribeStates(nil); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("connection loss was never observed")
	}
	reason := client.CloseReason()
	if reason == nil || errors.Is(reason, api.ErrPeerDisconnected) {
		t.Fatalf("unclean loss must record a non-disconnect reason, got %v", reason)
	}
	if client.Connected() {
		t.Fatal("lost client must not report connected")
	}
	// The interrupted command fails with the typed closed-client error; the
	// client keeps no queue that could retry or replay it.
	if err := client.SetSwitch(simulator.BasicSwitchKey, false); !errors.Is(err, api.ErrClientClosed) {
		t.Fatalf("post-loss command: %v", err)
	}
	waitUntil(t, "client-owned goroutines to end after peer loss", func() bool {
		return clientOwnedGoroutines() == 0
	})
	if got := dials.Load(); got != 1 {
		t.Fatalf("the one dial owner dialed %d times after loss", got)
	}
	if accepted := device.Stats().AcceptedConnections; accepted != 1 {
		t.Fatalf("device accepted %d connections before the new owner", accepted)
	}

	// A brand-new owner's connection observes no replayed command.
	replacement, err := api.Dial("simulator:6053", time.Second, options...)
	if err != nil {
		t.Fatalf("replacement dial: %v", err)
	}
	t.Cleanup(func() { _ = replacement.Close() })
	if _, err := replacement.ListEntities(); err != nil {
		t.Fatalf("replacement list entities: %v", err)
	}
	select {
	case command := <-device.Commands():
		t.Fatalf("device received a replayed command: %v", command)
	default:
	}
	if got := dials.Load(); got != 2 {
		t.Fatalf("expected exactly one dial per owner, got %d", got)
	}
}

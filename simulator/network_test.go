package simulator_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

func TestNetworkFragmentFramePreservesEncryptedResponse(t *testing.T) {
	device := simulator.New(networkScenario(simulator.NetworkFault{
		Trigger: simulator.FaultAfterHello,
		Action:  simulator.NetworkFragmentFrame,
	}))
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	assertNetworkEntityList(t, client)
	assertClientPing(t, client)
	stats := device.Stats()
	if stats.NetworkFragmentedFrames != 1 || stats.NetworkFragmentedSegments <= 3 {
		t.Fatalf("fragment stats = %+v, want one frame split across byte-sized writes", stats)
	}
}

func TestNetworkCoalesceSegmentsPreservesEncryptedResponse(t *testing.T) {
	device := simulator.New(networkScenario(simulator.NetworkFault{
		Trigger: simulator.FaultAfterHello,
		Action:  simulator.NetworkCoalesceSegments,
	}))
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	assertNetworkEntityList(t, client)
	assertClientPing(t, client)
	stats := device.Stats()
	if stats.NetworkCoalescedFrames != 1 || stats.NetworkCoalescedSegments != 2 {
		t.Fatalf("coalescing stats = %+v, want one Noise frame combining header and payload", stats)
	}
}

func TestNetworkDelayReplyUsesOnlyManualTime(t *testing.T) {
	const delay = 5 * time.Second
	clock := simulator.NewManualClock()
	device := simulator.New(networkScenario(simulator.NetworkFault{
		Trigger: simulator.FaultBeforeEntitiesDone,
		Action:  simulator.NetworkDelayReply,
		Delay:   delay,
	}), simulator.WithManualClock(clock))
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	result := listEntitiesAsync(client)
	waitForPendingNetworkDelay(t, device)
	assertListStillPending(t, result)
	if err := clock.Advance(delay - time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	assertListStillPending(t, result)
	if err := clock.Advance(time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	assertNetworkEntityResult(t, result)
	assertClientPing(t, client)
	stats := device.Stats()
	if stats.NetworkDelayedFrames != 1 || stats.NetworkPendingDelays != 0 {
		t.Fatalf("delay stats after release = %+v, want one completed virtual delay", stats)
	}
}

func TestDelayedTimelineFrameDoesNotBlockManualClock(t *testing.T) {
	const delay = 5 * time.Second
	clock := simulator.NewManualClock()
	device := simulator.New(simulator.Scenario{
		Name: "delayed-timeline-simulator",
		InitialStates: []proto.Message{
			&pb.SwitchStateResponse{Key: 41, State: false},
		},
		StateTimeline: []simulator.StateEvent{
			{At: time.Second, State: &pb.SwitchStateResponse{Key: 41, State: true}},
			{At: time.Second, State: &pb.SwitchStateResponse{Key: 41, State: false}},
		},
		Network: []simulator.NetworkFault{{
			Trigger: simulator.FaultAfterInitialStates,
			Action:  simulator.NetworkDelayReply,
			Delay:   delay,
		}},
	}, simulator.WithManualClock(clock))
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	states := make(chan bool, 3)
	unsubscribe, err := client.SubscribeStates(func(message proto.Message) {
		if state, ok := message.(*pb.SwitchStateResponse); ok && state.Key == 41 {
			states <- state.State
		}
	})
	if err != nil {
		t.Fatalf("subscribe states: %v", err)
	}
	t.Cleanup(unsubscribe)
	select {
	case state := <-states:
		if state {
			t.Fatal("unexpected initial switch state")
		}
	case <-time.After(time.Second):
		t.Fatal("initial state was not received")
	}

	advanced := make(chan error, 1)
	go func() { advanced <- clock.AdvanceTo(time.Second) }()
	select {
	case err := <-advanced:
		if err != nil {
			t.Fatalf("advance to timeline event: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeline send blocked ManualClock.AdvanceTo before its delay target")
	}
	waitForPendingNetworkDelay(t, device)
	select {
	case state := <-states:
		t.Fatalf("delayed timeline state arrived early: %t", state)
	default:
	}
	// Advance by the full declared delay from the now-committed event. The
	// trigger may arm immediately before or after the first AdvanceTo updates
	// the shared clock, but this reaches either valid target deterministically.
	if err := clock.Advance(delay); err != nil {
		t.Fatal(err)
	}
	for index, want := range []bool{true, false} {
		select {
		case state := <-states:
			if state != want {
				t.Fatalf("delayed state %d = %t, want %t", index, state, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeline state %d did not arrive at the virtual delay target", index)
		}
	}
	assertClientPing(t, client)
}

func TestDeviceCloseCancelsPendingNetworkDelay(t *testing.T) {
	clock := simulator.NewManualClock()
	device := simulator.New(networkScenario(simulator.NetworkFault{
		Trigger: simulator.FaultBeforeEntitiesDone,
		Action:  simulator.NetworkDelayReply,
		Delay:   time.Hour,
	}), simulator.WithManualClock(clock))
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	result := listEntitiesAsync(client)
	waitForPendingNetworkDelay(t, device)
	closed := make(chan error, 1)
	go func() { closed <- device.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close simulator: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Device.Close did not cancel a pending virtual network delay")
	}
	select {
	case outcome := <-result:
		if outcome.err == nil {
			t.Fatal("entity listing unexpectedly succeeded after simulator shutdown")
		}
	case <-time.After(time.Second):
		t.Fatal("client operation did not stop after simulator shutdown")
	}
	if stats := device.Stats(); stats.NetworkPendingDelays != 0 || stats.ActiveConnections != 0 {
		t.Fatalf("simulator retained delayed work after Close: %+v", stats)
	}
}

func TestDropConnectionsCancelsPendingNetworkDelayAndKeepsDeviceReusable(t *testing.T) {
	device := simulator.New(networkScenario(simulator.NetworkFault{
		Trigger: simulator.FaultBeforeEntitiesDone,
		Action:  simulator.NetworkDelayReply,
		Delay:   time.Hour,
	}))
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	result := listEntitiesAsync(client)
	waitForPendingNetworkDelay(t, device)
	if dropped := device.DropConnections(); dropped != 1 {
		t.Fatalf("DropConnections = %d, want one delayed session", dropped)
	}
	select {
	case outcome := <-result:
		if outcome.err == nil {
			t.Fatal("entity listing unexpectedly succeeded after connection drop")
		}
	case <-time.After(time.Second):
		t.Fatal("client operation did not stop after connection drop")
	}
	if stats := device.Stats(); stats.NetworkPendingDelays != 0 || stats.ActiveConnections != 0 {
		t.Fatalf("simulator retained delayed work after DropConnections: %+v", stats)
	}

	reconnected := dialSimulator(t, device)
	t.Cleanup(func() { _ = reconnected.Close() })
	assertClientPing(t, reconnected)
}

func TestUnknownNetworkActionHasNoEffect(t *testing.T) {
	device := simulator.New(networkScenario(simulator.NetworkFault{
		Trigger: simulator.FaultAfterHello,
		Action:  simulator.NetworkAction("future-network-action"),
		Delay:   17 * time.Second,
	}))
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	assertNetworkEntityList(t, client)
	assertClientPing(t, client)
	stats := device.Stats()
	if stats.NetworkFragmentedFrames != 0 || stats.NetworkCoalescedFrames != 0 || stats.NetworkDelayedFrames != 0 {
		t.Fatalf("unknown network action changed transport behavior: %+v", stats)
	}
}

func TestNetworkFaultDurationValidation(t *testing.T) {
	tests := []struct {
		name  string
		fault simulator.NetworkFault
	}{
		{name: "zero delay", fault: simulator.NetworkFault{Action: simulator.NetworkDelayReply}},
		{name: "negative delay", fault: simulator.NetworkFault{Action: simulator.NetworkDelayReply, Delay: -time.Nanosecond}},
		{name: "excessive delay", fault: simulator.NetworkFault{Action: simulator.NetworkDelayReply, Delay: simulator.MaxNetworkDelay + time.Nanosecond}},
		{name: "fragment with delay", fault: simulator.NetworkFault{Action: simulator.NetworkFragmentFrame, Delay: time.Second}},
		{name: "coalesce with delay", fault: simulator.NetworkFault{Action: simulator.NetworkCoalesceSegments, Delay: time.Second}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (simulator.Scenario{Network: []simulator.NetworkFault{{}, test.fault}}).Validate()
			var validationErr *simulator.ValidationError
			if !errors.Is(err, simulator.ErrInvalidScenario) || !errors.As(err, &validationErr) ||
				validationErr.Field != "network" || validationErr.Index != 1 ||
				validationErr.RelatedIndex != nil || validationErr.Code != simulator.ValidationInvalidDuration {
				t.Fatalf("validation error = %#v", err)
			}
		})
	}
}

func TestInvalidNetworkScenarioDoesNoConnectionWork(t *testing.T) {
	device := simulator.New(simulator.Scenario{Network: []simulator.NetworkFault{{
		Action: simulator.NetworkDelayReply,
	}}})
	t.Cleanup(func() { _ = device.Close() })

	connection, err := device.DialContext(context.Background(), "tcp", "private-network-target:6053")
	if connection != nil || !errors.Is(err, simulator.ErrInvalidScenario) {
		t.Fatalf("DialContext = (%v, %v), want typed validation failure", connection, err)
	}
	if stats := device.Stats(); stats.AcceptedConnections != 0 || stats.ActiveConnections != 0 {
		t.Fatalf("invalid network scenario started a connection: %+v", stats)
	}
}

func networkScenario(fault simulator.NetworkFault) simulator.Scenario {
	return simulator.Scenario{
		Name: "network-shaping-simulator",
		Entities: []proto.Message{&pb.ListEntitiesBinarySensorResponse{
			Key:      31,
			ObjectId: "synthetic_sensor",
			Name:     "Synthetic Sensor",
		}},
		Network: []simulator.NetworkFault{fault},
	}
}

func assertNetworkEntityList(t *testing.T, client *api.Client) {
	t.Helper()
	entities, err := client.ListEntitiesWithTimeout(time.Second)
	if err != nil {
		t.Fatalf("list entities: %v", err)
	}
	assertNetworkEntities(t, entities)
}

type entityListOutcome struct {
	entities []proto.Message
	err      error
}

func listEntitiesAsync(client *api.Client) <-chan entityListOutcome {
	result := make(chan entityListOutcome, 1)
	go func() {
		entities, err := client.ListEntitiesWithTimeout(5 * time.Second)
		result <- entityListOutcome{entities: entities, err: err}
	}()
	return result
}

func assertNetworkEntityResult(t *testing.T, result <-chan entityListOutcome) {
	t.Helper()
	select {
	case outcome := <-result:
		if outcome.err != nil {
			t.Fatalf("list entities after virtual delay: %v", outcome.err)
		}
		assertNetworkEntities(t, outcome.entities)
	case <-time.After(time.Second):
		t.Fatal("entity listing did not resume after virtual delay elapsed")
	}
}

func assertNetworkEntities(t *testing.T, entities []proto.Message) {
	t.Helper()
	if len(entities) != 1 {
		t.Fatalf("entities = %#v, want one exact descriptor", entities)
	}
	entity, ok := entities[0].(*pb.ListEntitiesBinarySensorResponse)
	if !ok || entity.Key != 31 || entity.ObjectId != "synthetic_sensor" || entity.Name != "Synthetic Sensor" {
		t.Fatalf("network shaping changed response bytes: %#v", entities[0])
	}
}

func assertClientPing(t *testing.T, client *api.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping after network shaping: %v", err)
	}
}

func waitForPendingNetworkDelay(t *testing.T, device *simulator.Device) {
	t.Helper()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for device.Stats().NetworkPendingDelays != 1 {
		select {
		case <-timeout.C:
			t.Fatalf("network delay did not become pending: %+v", device.Stats())
		default:
			runtime.Gosched()
		}
	}
}

func assertListStillPending(t *testing.T, result <-chan entityListOutcome) {
	t.Helper()
	select {
	case outcome := <-result:
		t.Fatalf("entity listing completed before virtual deadline: entities=%#v err=%v", outcome.entities, outcome.err)
	default:
	}
}

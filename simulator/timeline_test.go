package simulator_test

import (
	"errors"
	"testing"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

func TestManualClockRejectsBackwardTime(t *testing.T) {
	clock := simulator.NewManualClock()
	if err := clock.Advance(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	if got := clock.Now(); got != 2*time.Second {
		t.Fatalf("Now = %v, want 2s", got)
	}
	for _, err := range []error{clock.Advance(-time.Nanosecond), clock.AdvanceTo(time.Second)} {
		if !errors.Is(err, simulator.ErrClockBackwards) {
			t.Fatalf("error = %v, want ErrClockBackwards", err)
		}
	}
	if got := clock.Now(); got != 2*time.Second {
		t.Fatalf("failed advance changed time to %v", got)
	}
	if err := clock.Advance(time.Duration(1<<63 - 1)); !errors.Is(err, simulator.ErrClockOverflow) {
		t.Fatalf("overflow error = %v, want ErrClockOverflow", err)
	}
}

func TestStateTimelinePushesInDeterministicOrder(t *testing.T) {
	clock := simulator.NewManualClock()
	device := simulator.New(simulator.Scenario{
		Name: "timeline-simulator",
		InitialStates: []proto.Message{
			&pb.SwitchStateResponse{Key: 7, State: false},
		},
		StateTimeline: []simulator.StateEvent{
			{At: time.Second, State: &pb.SwitchStateResponse{Key: 7, State: true}},
			{At: time.Second, State: &pb.SwitchStateResponse{Key: 7, State: false}},
			{At: 2 * time.Second, State: &pb.SwitchStateResponse{Key: 7, State: true}},
		},
	}, simulator.WithManualClock(clock))
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	states := make(chan bool, 4)
	unsubscribe, err := client.SubscribeStates(func(message proto.Message) {
		if state, ok := message.(*pb.SwitchStateResponse); ok && state.Key == 7 {
			states <- state.State
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribe)
	assertSwitchStates(t, states, false)

	if err := clock.AdvanceTo(time.Second); err != nil {
		t.Fatal(err)
	}
	assertSwitchStates(t, states, true, false)
	if err := clock.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	assertSwitchStates(t, states, true)
}

func TestLatestStateSurvivesReconnectWithoutCommandReplay(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name: "reconnect-simulator",
		InitialStates: []proto.Message{
			&pb.SwitchStateResponse{Key: 9, State: true},
		},
	})
	t.Cleanup(func() { _ = device.Close() })

	first := dialSimulator(t, device)
	firstStates := make(chan bool, 2)
	unsubscribeFirst, err := first.SubscribeStates(switchStateHandler(9, firstStates))
	if err != nil {
		t.Fatal(err)
	}
	assertSwitchStates(t, firstStates, true)
	if err := first.SendCommand(&pb.SwitchCommandRequest{Key: 9, State: false}); err != nil {
		t.Fatal(err)
	}
	assertSwitchStates(t, firstStates, false)
	select {
	case command := <-device.Commands():
		if got, ok := command.(*pb.SwitchCommandRequest); !ok || got.Key != 9 || got.State {
			t.Fatalf("unexpected recorded command: %#v", command)
		}
	case <-time.After(time.Second):
		t.Fatal("command was not recorded")
	}
	if dropped := device.DropConnections(); dropped != 1 {
		t.Fatalf("DropConnections = %d, want 1", dropped)
	}
	waitForClientClose(t, first)
	unsubscribeFirst()

	second := dialSimulator(t, device)
	t.Cleanup(func() { _ = second.Close() })
	secondStates := make(chan bool, 1)
	unsubscribeSecond, err := second.SubscribeStates(switchStateHandler(9, secondStates))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribeSecond)
	assertSwitchStates(t, secondStates, false)
	select {
	case command := <-device.Commands():
		t.Fatalf("reconnect replayed an unrequested command: %#v", command)
	case <-time.After(25 * time.Millisecond):
	}
	stats := device.Stats()
	if stats.AcceptedConnections != 2 || stats.DroppedConnections != 1 {
		t.Fatalf("unexpected reconnect stats: %+v", stats)
	}
}

func TestPastTimelineEventsBecomeOneReconnectSnapshot(t *testing.T) {
	clock := simulator.NewManualClock()
	device := simulator.New(simulator.Scenario{
		Name:          "past-event-simulator",
		InitialStates: []proto.Message{&pb.SwitchStateResponse{Key: 3, State: false}},
		StateTimeline: []simulator.StateEvent{{At: time.Second, State: &pb.SwitchStateResponse{Key: 3, State: true}}},
	}, simulator.WithManualClock(clock))
	t.Cleanup(func() { _ = device.Close() })
	if err := clock.Advance(time.Second); err != nil {
		t.Fatal(err)
	}

	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	states := make(chan bool, 2)
	unsubscribe, err := client.SubscribeStates(switchStateHandler(3, states))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribe)
	assertSwitchStates(t, states, true)
	select {
	case state := <-states:
		t.Fatalf("past event was replayed after snapshot: %t", state)
	case <-time.After(25 * time.Millisecond):
	}
}

func switchStateHandler(key uint32, states chan<- bool) func(proto.Message) {
	return func(message proto.Message) {
		if state, ok := message.(*pb.SwitchStateResponse); ok && state.Key == key {
			states <- state.State
		}
	}
}

func assertSwitchStates(t *testing.T, states <-chan bool, expected ...bool) {
	t.Helper()
	for index, want := range expected {
		select {
		case got := <-states:
			if got != want {
				t.Fatalf("state[%d] = %t, want %t", index, got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for state[%d] = %t", index, want)
		}
	}
}

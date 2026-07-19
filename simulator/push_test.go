package simulator_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

func TestPushStateReachesSubscribersInOrderAndSnapshot(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name:          "push-simulator",
		InitialStates: []proto.Message{&pb.SwitchStateResponse{Key: 5, State: true}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	states := make(chan bool, 8)
	unsubscribe, err := client.SubscribeStates(switchStateHandler(5, states))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribe)
	assertSwitchStates(t, states, true)

	for _, state := range []bool{false, true, false} {
		if err := device.PushState(&pb.SwitchStateResponse{Key: 5, State: state}); err != nil {
			t.Fatalf("push state %t: %v", state, err)
		}
	}
	assertSwitchStates(t, states, false, true, false)

	second := dialSimulator(t, device)
	t.Cleanup(func() { _ = second.Close() })
	snapshot := make(chan bool, 2)
	unsubscribeSecond, err := second.SubscribeStates(switchStateHandler(5, snapshot))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribeSecond)
	assertSwitchStates(t, snapshot, false)
}

func TestPushLogHonorsSubscriptionLevel(t *testing.T) {
	device := simulator.New(simulator.Scenario{Name: "push-log-simulator"})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	logs := make(chan string, 4)
	unsubscribe, err := client.SubscribeLogs(pb.LogLevel_LOG_LEVEL_INFO, func(entry *pb.SubscribeLogsResponse) {
		logs <- string(entry.Message)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribe)
	// SubscribeLogs returns once the request is written; ping the same serve
	// loop so the subscription is registered before the pushes below.
	pingCtx, cancelPing := context.WithTimeout(context.Background(), time.Second)
	defer cancelPing()
	if err := client.Ping(pingCtx); err != nil {
		t.Fatalf("Ping subscription barrier: %v", err)
	}

	if err := device.PushLog(&pb.SubscribeLogsResponse{Level: pb.LogLevel_LOG_LEVEL_DEBUG, Message: []byte("too detailed")}); err != nil {
		t.Fatal(err)
	}
	if err := device.PushLog(&pb.SubscribeLogsResponse{Level: pb.LogLevel_LOG_LEVEL_INFO, Message: []byte("admitted")}); err != nil {
		t.Fatal(err)
	}
	select {
	case entry := <-logs:
		if entry != "admitted" {
			t.Fatalf("log = %q, want the admitted INFO entry only", entry)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the admitted INFO log entry")
	}
	select {
	case entry := <-logs:
		t.Fatalf("unexpected extra log entry %q", entry)
	case <-time.After(25 * time.Millisecond):
	}

	// Pushed entries are transient: a later subscriber must not receive them.
	late := dialSimulator(t, device)
	t.Cleanup(func() { _ = late.Close() })
	lateLogs := make(chan string, 4)
	unsubscribeLate, err := late.SubscribeLogs(pb.LogLevel_LOG_LEVEL_DEBUG, func(entry *pb.SubscribeLogsResponse) {
		lateLogs <- string(entry.Message)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribeLate)
	select {
	case entry := <-lateLogs:
		t.Fatalf("pushed log entry was replayed to a later subscriber: %q", entry)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestPushStateDuringConnectionChurnIsRaceFree(t *testing.T) {
	device := simulator.New(simulator.Scenario{Name: "churn-simulator"})
	t.Cleanup(func() { _ = device.Close() })
	done := make(chan struct{})
	pusherStopped := make(chan struct{})
	t.Cleanup(func() {
		close(done)
		<-pusherStopped
	})
	go func() {
		defer close(pusherStopped)
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			default:
			}
			// Errors are expected once the device closes underneath the loop.
			_ = device.PushState(&pb.SwitchStateResponse{Key: 1, State: i%2 == 0})
		}
	}()
	for i := 0; i < 20; i++ {
		client := dialSimulator(t, device)
		unsubscribe, err := client.SubscribeStates(func(proto.Message) {})
		if err != nil {
			t.Fatal(err)
		}
		unsubscribe()
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPushRejectsUnsupportedPayloads(t *testing.T) {
	device := simulator.New(simulator.Scenario{Name: "push-reject-simulator"})
	t.Cleanup(func() { _ = device.Close() })
	for _, err := range []error{
		device.PushState(nil),
		device.PushState(&pb.HelloResponse{}),
		device.PushLog(nil),
	} {
		if !errors.Is(err, simulator.ErrUnsupportedPush) {
			t.Fatalf("error = %v, want ErrUnsupportedPush", err)
		}
	}
}

func TestPushAfterCloseFails(t *testing.T) {
	device := simulator.New(simulator.Scenario{Name: "push-closed-simulator"})
	if err := device.Close(); err != nil {
		t.Fatal(err)
	}
	if err := device.PushState(&pb.SwitchStateResponse{Key: 1, State: true}); err == nil {
		t.Fatal("PushState on a closed device unexpectedly succeeded")
	}
	if err := device.PushLog(&pb.SubscribeLogsResponse{Level: pb.LogLevel_LOG_LEVEL_INFO, Message: []byte("late")}); err == nil {
		t.Fatal("PushLog on a closed device unexpectedly succeeded")
	}
}

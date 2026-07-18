package simulator_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

func TestSlowStateSubscriberClosesBoundedQueueAndCleansUp(t *testing.T) {
	clock := simulator.NewManualClock()
	device := simulator.New(simulator.Scenario{
		Name: "slow-subscriber-simulator",
		InitialStates: []proto.Message{
			&pb.SwitchStateResponse{Key: 1, State: false},
		},
		StateTimeline: []simulator.StateEvent{
			{At: time.Second, State: &pb.SwitchStateResponse{Key: 1, State: true}},
			{At: time.Second, State: &pb.SwitchStateResponse{Key: 1, State: false}},
		},
	}, simulator.WithManualClock(clock))
	t.Cleanup(func() { _ = device.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	options := append(device.ClientOptions(), api.WithCallbackQueueSize(1))
	client, err := api.DialWithContext(ctx, "synthetic-simulator:6053", time.Second, options...)
	if err != nil {
		t.Fatalf("dial simulator: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	gate := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate) }) }
	t.Cleanup(release)
	entered := make(chan struct{})
	var enteredOnce sync.Once
	var callbackCount atomic.Uint32
	unsubscribe, err := client.SubscribeStates(func(message proto.Message) {
		if state, ok := message.(*pb.SwitchStateResponse); !ok || state.Key != 1 {
			return
		}
		callbackCount.Add(1)
		enteredOnce.Do(func() { close(entered) })
		<-gate
	})
	if err != nil {
		t.Fatalf("subscribe states: %v", err)
	}
	t.Cleanup(unsubscribe)

	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatalf("initial callback did not start: %v", ctx.Err())
	}
	canceledCtx, cancelWait := context.WithCancel(context.Background())
	cancelWait()
	if err := client.WaitCallbacks(canceledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitCallbacks = %v, want context.Canceled", err)
	}

	// The callback holds the dispatcher. One due event fills the queue and the
	// next due event must close the connection rather than block or disappear.
	if err := clock.AdvanceTo(time.Second); err != nil {
		t.Fatalf("advance state burst: %v", err)
	}
	select {
	case <-client.Done():
	case <-ctx.Done():
		t.Fatalf("client did not close after queue saturation: %v", ctx.Err())
	}
	if reason := client.CloseReason(); !errors.Is(reason, api.ErrEventQueueFull) {
		t.Fatalf("CloseReason = %v, want ErrEventQueueFull", reason)
	}
	if got := callbackCount.Load(); got != 1 {
		t.Fatalf("callbacks while blocked = %d, want 1", got)
	}

	deviceClosed := make(chan error, 1)
	go func() { deviceClosed <- device.Close() }()
	select {
	case err := <-deviceClosed:
		if err != nil {
			t.Fatalf("close device: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("Device.Close waited for caller callback: %v", ctx.Err())
	}
	if stats := device.Stats(); stats.ActiveConnections != 0 {
		t.Fatalf("active connections after Device.Close = %d, want 0", stats.ActiveConnections)
	}

	release()
	if err := client.WaitCallbacks(ctx); err != nil {
		t.Fatalf("wait for callback dispatcher cleanup: %v", err)
	}
	if got := callbackCount.Load(); got != 1 {
		t.Fatalf("final callback count = %d, want 1", got)
	}
}

package simulator_test

import (
	"context"
	"strings"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

func TestHostilePeerFaultsCloseClient(t *testing.T) {
	tests := []struct {
		name   string
		action simulator.FaultAction
	}{
		{name: "dropped connection", action: simulator.FaultDropConnection},
		{name: "malformed protobuf", action: simulator.FaultMalformedProtobuf},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			device := simulator.New(simulator.Scenario{
				Name:   "hostile-peer-simulator",
				Faults: []simulator.Fault{{Trigger: simulator.FaultAfterHello, Action: test.action}},
			})
			t.Cleanup(func() { _ = device.Close() })
			client := dialSimulator(t, device)
			t.Cleanup(func() { _ = client.Close() })
			waitForClientClose(t, client)
		})
	}
}

func TestUnknownMessageIsSkippedAndSubsequentTrafficContinues(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name: "future-message-simulator",
		Entities: []proto.Message{
			&pb.ListEntitiesBinarySensorResponse{Key: 1, ObjectId: "synthetic_sensor", Name: "Synthetic Sensor"},
		},
		Faults: []simulator.Fault{{Trigger: simulator.FaultBeforeEntitiesDone, Action: simulator.FaultUnknownMessage}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	entities, err := client.ListEntitiesWithTimeout(time.Second)
	if err != nil || len(entities) != 1 {
		t.Fatalf("discovery after unknown frame: entities=%d err=%v", len(entities), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("subsequent ping after unknown frame: %v", err)
	}
	if !client.Connected() {
		t.Fatal("bounded unknown frame closed the client")
	}
}

func TestDuplicateEntityCompletionCannotPanicOrPoisonConnection(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name: "duplicate-done-simulator",
		Entities: []proto.Message{
			&pb.ListEntitiesBinarySensorResponse{Key: 1, ObjectId: "synthetic_sensor", Name: "Synthetic Sensor"},
		},
		Faults: []simulator.Fault{{Trigger: simulator.FaultBeforeEntitiesDone, Action: simulator.FaultDuplicateEntitiesDone}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	entities, err := client.ListEntitiesWithTimeout(time.Second)
	if err != nil || len(entities) != 1 {
		t.Fatalf("duplicate completion: entities=%d err=%v", len(entities), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping after duplicate completion: %v", err)
	}
}

func TestFaultBeforeEntitiesDoneFailsClosed(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name: "incomplete-discovery-simulator",
		Entities: []proto.Message{
			&pb.ListEntitiesBinarySensorResponse{Key: 1, ObjectId: "synthetic_sensor", Name: "Synthetic Sensor"},
		},
		Faults: []simulator.Fault{{Trigger: simulator.FaultBeforeEntitiesDone, Action: simulator.FaultDropConnection}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	if _, err := client.ListEntitiesWithTimeout(time.Second); err == nil {
		t.Fatal("incomplete entity list unexpectedly succeeded")
	}
	waitForClientClose(t, client)
}

func TestStalledEntityListHonorsOperationDeadline(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name:   "stalled-discovery-simulator",
		Faults: []simulator.Fault{{Trigger: simulator.FaultBeforeEntitiesDone, Action: simulator.FaultStall}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	_, err := client.ListEntitiesWithTimeout(20 * time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("got %v, want sanitized entity-list timeout", err)
	}
	if !client.Connected() {
		t.Fatal("operation timeout unexpectedly closed the reusable client")
	}
}

func TestFaultAfterInitialStatesUsesRealSubscriptionPath(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name:   "state-stream-fault-simulator",
		States: []proto.Message{&pb.BinarySensorStateResponse{Key: 1, State: true}},
		Faults: []simulator.Fault{{Trigger: simulator.FaultAfterInitialStates, Action: simulator.FaultStall}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	states := make(chan proto.Message, 1)
	unsubscribe, err := client.SubscribeStates(func(message proto.Message) { states <- message })
	if err != nil {
		t.Fatalf("subscribe states: %v", err)
	}
	t.Cleanup(unsubscribe)
	select {
	case message := <-states:
		state, ok := message.(*pb.BinarySensorStateResponse)
		if !ok || !state.State {
			t.Fatalf("unexpected initial state: %#v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("initial state did not traverse the real subscription path")
	}
	if err := device.Close(); err != nil {
		t.Fatalf("close stalled simulator: %v", err)
	}
	waitForClientClose(t, client)
}

func TestDropConnectionsReleasesStalledSessionTask(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name:   "dropped-stall-simulator",
		Faults: []simulator.Fault{{Trigger: simulator.FaultAfterHello, Action: simulator.FaultStall}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	if dropped := device.DropConnections(); dropped != 1 {
		t.Fatalf("DropConnections = %d, want one stalled session", dropped)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := device.WaitForIdle(ctx); err != nil {
		t.Fatalf("stalled session remained after DropConnections: %v", err)
	}
	stats := device.Stats()
	if stats.ActiveConnections != 0 || stats.ActiveSessionTasks != 0 {
		t.Fatalf("stalled session resources remain: %+v", stats)
	}
}

func TestUnknownFaultValuesHaveNoEffect(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name:   "forward-compatible-fault-simulator",
		Faults: []simulator.Fault{{Trigger: simulator.FaultAfterHello, Action: simulator.FaultAction("future-action")}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.ListEntitiesWithTimeout(time.Second); err != nil {
		t.Fatalf("unknown fault action changed behavior: %v", err)
	}
}

func dialSimulator(t *testing.T, device *simulator.Device) *api.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	client, err := api.DialWithContext(ctx, "synthetic-simulator:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("dial simulator: %v", err)
	}
	return client
}

func waitForClientClose(t *testing.T, client *api.Client) {
	t.Helper()
	select {
	case <-client.Done():
	case <-time.After(time.Second):
		t.Fatal("client did not close after hostile peer fault")
	}
	if client.Connected() {
		t.Fatal("client still reports connected after hostile peer fault")
	}
}

package simulator_test

import (
	"context"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
)

func TestGenericCompatibilityScenarios(t *testing.T) {
	for _, scenario := range []simulator.Scenario{simulator.BasicIOScenario(), simulator.BlinkScenario()} {
		t.Run(scenario.Name, func(t *testing.T) {
			device := simulator.New(scenario)
			t.Cleanup(func() { _ = device.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			client, err := api.DialWithContext(ctx, "synthetic-simulator:6053", time.Second, device.ClientOptions()...)
			if err != nil {
				t.Fatalf("dial: %v", err)
			}
			if _, err := client.ListEntities(); err != nil {
				t.Fatalf("list entities: %v", err)
			}
			stats := device.Stats()
			if stats.AcceptedConnections != 1 || stats.ActiveConnections != 1 {
				t.Fatalf("unexpected active stats: %+v", stats)
			}
			if err := client.Close(); err != nil {
				t.Fatalf("close client: %v", err)
			}
			deadline := time.Now().Add(time.Second)
			for device.Stats().ActiveConnections != 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if stats := device.Stats(); stats.ActiveConnections != 0 {
				t.Fatalf("connection did not clean up: %+v", stats)
			}
		})
	}
}

func TestCommandOverflowIsObservable(t *testing.T) {
	device := simulator.New(simulator.ConveyorScenario())
	t.Cleanup(func() { _ = device.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := api.DialWithContext(ctx, "synthetic-simulator:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	for i := 0; i < 70; i++ {
		if err := client.SendCommand(&pb.ButtonCommandRequest{Key: simulator.ResetButtonKey}); err != nil {
			t.Fatalf("send command %d: %v", i, err)
		}
	}
	deadline := time.Now().Add(time.Second)
	for device.Stats().DroppedCommands == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if stats := device.Stats(); stats.DroppedCommands == 0 {
		t.Fatalf("overflow was silent: %+v", stats)
	}
}

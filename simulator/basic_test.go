package simulator_test

import (
	"context"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
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

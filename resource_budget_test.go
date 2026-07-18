package aioesphomeapi_test

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

// waitUntil polls a condition with a bounded deadline. The condition itself is
// always anchored to a lifecycle barrier (Done, WaitCallbacks, Device.Stats)
// or an owned-goroutine count, never to an arbitrary sleep; the loop only
// absorbs scheduler lag between a goroutine's last instruction and its exit
// becoming observable.
func waitUntil(t *testing.T, message string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", message)
}

// countOwnedGoroutines returns how many goroutine stack blocks contain any of
// the given frame markers. This counts library-owned goroutines specifically,
// never the process-wide goroutine total, so unrelated test and runtime
// goroutines cannot skew the budget.
func countOwnedGoroutines(markers ...string) int {
	buffer := make([]byte, 1<<21)
	stacks := string(buffer[:runtime.Stack(buffer, true)])
	count := 0
	for _, block := range strings.Split(stacks, "\n\n") {
		for _, marker := range markers {
			if strings.Contains(block, marker) {
				count++
				break
			}
		}
	}
	return count
}

// clientOwnedGoroutines counts the client's owned loops. The two markers also
// match the read loop's context-watcher closure (readLoop.func1).
func clientOwnedGoroutines() int {
	return countOwnedGoroutines(
		"aioesphomeapi.(*Client).readLoop",
		"aioesphomeapi.(*Client).dispatchLoop",
	)
}

// clientGoroutineBudget is the documented maximum for one connected client:
// the frame read loop, the read loop's context watcher, and the serial
// callback dispatcher. Dial starts exactly these three and nothing else.
const clientGoroutineBudget = 3

// simulatorGoroutineBudget is the documented maximum the simulator owns per
// accepted connection: one serve goroutine. It is the only `go` statement in
// the simulator package, and the device runs nothing between connections.
// The marker names the serve method rather than the whole package because
// client-owned goroutines legitimately execute simulator code while blocked
// inside the in-memory shaped connection's Read.
const simulatorGoroutineBudget = 1

func simulatorOwnedGoroutines() int {
	return countOwnedGoroutines("simulator.(*Device).serve")
}

// TestClientGoroutineBudgetAfterClose proves that one connected client owns at
// most clientGoroutineBudget goroutines while active and zero after Close,
// using Done and WaitCallbacks as the barriers, over several full
// connect/subscribe/command cycles. Waiting for the exact budget also proves
// the markers still match the implementation, so a rename cannot turn this
// test into a no-op.
func TestClientGoroutineBudgetAfterClose(t *testing.T) {
	device := simulator.New(simulator.BasicIOScenario())
	t.Cleanup(func() { _ = device.Close() })

	const cycles = 3
	for i := 0; i < cycles; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, err := api.DialWithContext(ctx, "simulator:6053", time.Second, device.ClientOptions()...)
		if err != nil {
			cancel()
			t.Fatalf("cycle %d dial: %v", i, err)
		}
		if _, err := client.ListEntities(); err != nil {
			cancel()
			t.Fatalf("cycle %d list: %v", i, err)
		}
		states := make(chan proto.Message, 32)
		unsubscribe, err := client.SubscribeStates(func(message proto.Message) { states <- message })
		if err != nil {
			cancel()
			t.Fatalf("cycle %d subscribe: %v", i, err)
		}
		select {
		case <-states:
		case <-time.After(5 * time.Second):
			t.Fatalf("cycle %d never delivered a state callback", i)
		}
		unsubscribe()

		waitUntil(t, "the connected client to settle at its goroutine budget", func() bool {
			return clientOwnedGoroutines() == clientGoroutineBudget
		})
		if owned := clientOwnedGoroutines(); owned > clientGoroutineBudget {
			t.Fatalf("cycle %d owns %d goroutines, budget %d", i, owned, clientGoroutineBudget)
		}

		if err := client.Close(); err != nil {
			cancel()
			t.Fatalf("cycle %d close: %v", i, err)
		}
		select {
		case <-client.Done():
		case <-time.After(5 * time.Second):
			t.Fatalf("cycle %d Done never closed", i)
		}
		if err := client.WaitCallbacks(ctx); err != nil {
			cancel()
			t.Fatalf("cycle %d WaitCallbacks: %v", i, err)
		}
		if reason := client.CloseReason(); reason != nil {
			t.Fatalf("cycle %d deliberate close recorded a failure: %v", i, reason)
		}
		cancel()

		waitUntil(t, "client-owned goroutines to end", func() bool {
			return clientOwnedGoroutines() == 0
		})
	}

	waitUntil(t, "simulator connections to drain", func() bool {
		return device.Stats().ActiveConnections == 0
	})
	if stats := device.Stats(); stats.AcceptedConnections != cycles {
		t.Fatalf("accepted connections = %d, want %d", stats.AcceptedConnections, cycles)
	}
}

// TestSimulatorConnectionCleanupBudget proves the simulator owns at most
// simulatorGoroutineBudget goroutines for one accepted connection, accounts
// for the connection in Stats, and releases both the accounting and the
// goroutines after Device.Close. Waiting for the exact budget also proves the
// package marker still matches the implementation.
func TestSimulatorConnectionCleanupBudget(t *testing.T) {
	device := simulator.New(simulator.BasicIOScenario())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.DialWithContext(ctx, "simulator:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	waitUntil(t, "simulator to account for the connection", func() bool {
		stats := device.Stats()
		return stats.AcceptedConnections == 1 && stats.ActiveConnections == 1
	})
	waitUntil(t, "the accepted connection to settle at the simulator budget", func() bool {
		return simulatorOwnedGoroutines() == simulatorGoroutineBudget
	})
	if owned := simulatorOwnedGoroutines(); owned > simulatorGoroutineBudget {
		t.Fatalf("simulator owns %d goroutines for one connection, budget %d", owned, simulatorGoroutineBudget)
	}

	if err := device.Close(); err != nil {
		t.Fatalf("device close: %v", err)
	}
	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("client did not observe the simulator shutdown")
	}
	waitUntil(t, "accepted connections to be released", func() bool {
		return device.Stats().ActiveConnections == 0
	})
	waitUntil(t, "simulator-owned goroutines to end", func() bool {
		return simulatorOwnedGoroutines() == 0
	})
}

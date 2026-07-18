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
// always anchored to a lifecycle barrier (Done, WaitCallbacks, Device.Stats),
// never to an arbitrary sleep; the loop only absorbs scheduler lag between a
// goroutine's last instruction and its exit becoming observable.
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

// clientFramesLive reports whether any goroutine is still executing one of the
// client's owned loops. This deliberately matches the library-owned method
// frames instead of asserting an exact process-wide goroutine count.
func clientFramesLive() bool {
	buffer := make([]byte, 1<<20)
	stacks := string(buffer[:runtime.Stack(buffer, true)])
	return strings.Contains(stacks, "aioesphomeapi.(*Client).readLoop") ||
		strings.Contains(stacks, "aioesphomeapi.(*Client).dispatchLoop")
}

// TestClientGoroutineBudgetAfterClose proves that every goroutine the client
// starts (the read loop, its context watcher, and the callback dispatcher)
// ends after Close, using Done and WaitCallbacks as the barriers, over several
// full connect/subscribe/command cycles.
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

		// The marker must be observable on a live connection, otherwise a
		// future rename would silently turn the cleanup check into a no-op.
		if i == 0 && !clientFramesLive() {
			t.Fatal("client loop frames are not observable while connected; update clientFramesLive")
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
	}

	waitUntil(t, "client-owned goroutines to end", func() bool { return !clientFramesLive() })
	waitUntil(t, "simulator connections to drain", func() bool {
		return device.Stats().ActiveConnections == 0
	})
	if stats := device.Stats(); stats.AcceptedConnections != cycles {
		t.Fatalf("accepted connections = %d, want %d", stats.AcceptedConnections, cycles)
	}
}

// TestSimulatorConnectionCleanupBudget proves the simulator releases every
// accepted connection and its goroutines: while a client is connected the
// device accounts for it, and after Device.Close the per-connection and
// accept-side goroutines are gone.
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
	simulatorFramesLive := func() bool {
		buffer := make([]byte, 1<<20)
		stacks := string(buffer[:runtime.Stack(buffer, true)])
		return strings.Contains(stacks, "go-aioesphomeapi/simulator.")
	}
	// The marker must be observable on a live connection, otherwise a future
	// package move would silently turn the cleanup check into a no-op.
	if !simulatorFramesLive() {
		t.Fatal("simulator frames are not observable while connected; update the marker")
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
		return !simulatorFramesLive()
	})
}

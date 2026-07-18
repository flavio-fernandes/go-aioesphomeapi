package simulator

import (
	"errors"
	"net"
	"runtime"
	"testing"
	"time"
)

func TestDelayedFrameQueueFailsClosedAtItsFixedBudget(t *testing.T) {
	device := New(Scenario{Name: "bounded-network-queue-simulator"})
	t.Cleanup(func() { _ = device.Close() })
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	connection := newNetworkConn(server, device)
	t.Cleanup(func() { _ = connection.Close() })

	connection.arm(NetworkDelayReply, time.Hour)
	connection.beginFrame()
	if _, err := connection.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := connection.endFrame(nil); err != nil {
		t.Fatalf("enqueue delayed head frame: %v", err)
	}
	waitForInternalNetworkDelay(t, device)

	for index := 0; index < networkFrameQueueSize; index++ {
		connection.beginFrame()
		if _, err := connection.Write([]byte{byte(index)}); err != nil {
			t.Fatalf("buffer frame %d: %v", index, err)
		}
		if err := connection.endFrame(nil); err != nil {
			t.Fatalf("enqueue frame %d within budget: %v", index, err)
		}
	}
	connection.beginFrame()
	if _, err := connection.Write([]byte{255}); err != nil {
		t.Fatal(err)
	}
	if err := connection.endFrame(nil); !errors.Is(err, errNetworkFrameQueueFull) {
		t.Fatalf("overflow error = %v, want errNetworkFrameQueueFull", err)
	}

	closed := make(chan struct{})
	go func() {
		_ = connection.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("bounded network queue did not release its writer during cleanup")
	}
}

func waitForInternalNetworkDelay(t *testing.T, device *Device) {
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

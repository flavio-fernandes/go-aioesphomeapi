package simulator_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
)

func TestDeviceConnectionLimitAndWaitForIdle(t *testing.T) {
	device := simulator.New(simulator.Scenario{Name: "bounded-simulator"})
	t.Cleanup(func() { _ = device.Close() })

	connections := make([]net.Conn, 0, simulator.MaxDeviceConnections)
	for index := 0; index < simulator.MaxDeviceConnections; index++ {
		connection, err := device.DialContext(context.Background(), "tcp", "ignored:6053")
		if err != nil {
			t.Fatalf("dial %d: %v", index, err)
		}
		connections = append(connections, connection)
	}
	connection, err := device.DialContext(context.Background(), "tcp", "private-target:6053")
	if connection != nil || !errors.Is(err, simulator.ErrConnectionLimit) {
		t.Fatalf("over-limit dial = (%v, %v), want ErrConnectionLimit", connection, err)
	}
	if strings.Contains(err.Error(), "private-target") {
		t.Fatalf("connection-limit error leaked target: %v", err)
	}
	stats := device.Stats()
	if stats.AcceptedConnections != simulator.MaxDeviceConnections ||
		stats.ActiveConnections != simulator.MaxDeviceConnections ||
		stats.ActiveSessionTasks != simulator.MaxDeviceConnections {
		t.Fatalf("full-capacity stats = %+v", stats)
	}

	for _, active := range connections {
		_ = active.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := device.WaitForIdle(ctx); err != nil {
		t.Fatalf("wait for connection cleanup: %v", err)
	}
	stats = device.Stats()
	if stats.ActiveConnections != 0 || stats.ActiveSessionTasks != 0 || stats.NetworkPendingDelays != 0 {
		t.Fatalf("post-cleanup stats = %+v", stats)
	}
}

func TestDeviceListenerLimitAndCloseCleanup(t *testing.T) {
	device := simulator.New(simulator.Scenario{Name: "listener-budget-simulator"})
	results := make([]<-chan error, 0, simulator.MaxDeviceListeners)
	for index := 0; index < simulator.MaxDeviceListeners; index++ {
		listener := newBlockingListener()
		result := make(chan error, 1)
		go func() { result <- device.Serve(listener) }()
		results = append(results, result)
		waitForActiveListeners(t, device, index+1)
	}

	extra := newBlockingListener()
	err := device.Serve(extra)
	_ = extra.Close()
	if !errors.Is(err, simulator.ErrListenerLimit) {
		t.Fatalf("over-limit Serve = %v, want ErrListenerLimit", err)
	}
	if err := device.Close(); err != nil {
		t.Fatalf("close device: %v", err)
	}
	for index, result := range results {
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("Serve %d after Close: %v", index, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("Serve %d did not stop", index)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := device.WaitForIdle(ctx); err != nil {
		t.Fatalf("wait for listener cleanup: %v", err)
	}
	if stats := device.Stats(); stats.ActiveListeners != 0 || stats.ActiveSessionTasks != 0 {
		t.Fatalf("post-close stats = %+v", stats)
	}
}

func TestWaitForIdlePreservesCancellationCause(t *testing.T) {
	device := simulator.New(simulator.Scenario{Name: "busy-simulator"})
	t.Cleanup(func() { _ = device.Close() })
	connection, err := device.DialContext(context.Background(), "tcp", "ignored:6053")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = device.WaitForIdle(ctx)
	if !errors.Is(err, simulator.ErrSimulatorBusy) || !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitForIdle = %v, want busy and canceled categories", err)
	}
	for _, privateValue := range []string{"busy-simulator", "ignored:6053", simulator.DefaultTestEncryptionKey} {
		if strings.Contains(err.Error(), privateValue) {
			t.Fatalf("WaitForIdle error leaked %q: %v", privateValue, err)
		}
	}
}

func waitForActiveListeners(t *testing.T, device *simulator.Device, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for device.Stats().ActiveListeners != want && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := device.Stats().ActiveListeners; got != want {
		t.Fatalf("active listeners = %d, want %d", got, want)
	}
}

type blockingListener struct {
	closed chan struct{}
	once   sync.Once
}

func newBlockingListener() *blockingListener {
	return &blockingListener{closed: make(chan struct{})}
}

func (l *blockingListener) Accept() (net.Conn, error) {
	<-l.closed
	return nil, net.ErrClosed
}

func (l *blockingListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *blockingListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 6053}
}

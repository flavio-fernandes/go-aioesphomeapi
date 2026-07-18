package simulator_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
)

func TestServeSecureLoopback(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	device := simulator.New(simulator.ConveyorScenario())
	t.Cleanup(func() { _ = device.Close() })
	serveDone := make(chan error, 1)
	go func() { serveDone <- device.Serve(listener) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.DialWithContext(ctx, listener.Addr().String(), time.Second,
		api.WithEncryptionKey(simulator.DefaultTestEncryptionKey),
		api.WithExpectedName("conveyor-simulator"),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if _, err := client.ListEntities(); err != nil {
		t.Fatalf("list entities: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	if err := device.Close(); err != nil {
		t.Fatalf("close device: %v", err)
	}
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not stop after Device.Close")
	}
}

func TestServeRejectsNonLoopback(t *testing.T) {
	device := simulator.New(simulator.ConveyorScenario())
	t.Cleanup(func() { _ = device.Close() })
	listener := &stubListener{address: &net.TCPAddr{IP: net.IPv4zero, Port: 6053}}
	if err := device.Serve(listener); !errors.Is(err, simulator.ErrNonLoopbackOnly) {
		t.Fatalf("got %v, want non-loopback error", err)
	}
}

func TestServeWrapsAcceptError(t *testing.T) {
	device := simulator.New(simulator.ConveyorScenario())
	t.Cleanup(func() { _ = device.Close() })
	underlying := errors.New("synthetic accept failure")
	listener := &stubListener{address: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 6053}, acceptErr: underlying}
	err := device.Serve(listener)
	if !errors.Is(err, underlying) {
		t.Fatalf("accept cause was not preserved: %v", err)
	}
}

type stubListener struct {
	address   net.Addr
	acceptErr error
}

func (s *stubListener) Accept() (net.Conn, error) {
	if s.acceptErr != nil {
		return nil, s.acceptErr
	}
	panic("Accept must not be called")
}
func (s *stubListener) Close() error   { return nil }
func (s *stubListener) Addr() net.Addr { return s.address }

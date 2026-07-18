package mdns

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakePacketConn struct {
	response    []byte
	readErr     error
	writeErr    error
	deadlineErr error
	written     []byte
	deadline    time.Time
	source      *net.UDPAddr
	writes      int
	closed      bool
	mutex       sync.Mutex
}

func (c *fakePacketConn) SetReadBuffer(int) error { return nil }
func (c *fakePacketConn) SetDeadline(deadline time.Time) error {
	c.deadline = deadline
	return c.deadlineErr
}
func (c *fakePacketConn) WriteToUDP(message []byte, _ *net.UDPAddr) (int, error) {
	c.writes++
	c.written = append([]byte(nil), message...)
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(message), nil
}
func (c *fakePacketConn) ReadFromUDP(buffer []byte) (int, *net.UDPAddr, error) {
	if c.readErr != nil {
		return 0, nil, c.readErr
	}
	if c.response == nil {
		return 0, nil, io.EOF
	}
	n := copy(buffer, c.response)
	c.response = nil
	source := c.source
	if source == nil {
		source = multicastAddress
	}
	return n, source, nil
}

func TestLookupRetransmitsWithinOverallDeadline(t *testing.T) {
	reply, err := response("retry-device.local", net.IPv4(192, 0, 2, 55))
	if err != nil {
		t.Fatal(err)
	}
	conn := &retryPacketConn{response: reply}
	listen := func(string, *net.Interface, *net.UDPAddr) (packetConn, error) { return conn, nil }
	ip, err := lookupWithSchedule(context.Background(), "retry-device.local", 100*time.Millisecond, listen, []time.Duration{5 * time.Millisecond, 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("lookup after retransmits: %v", err)
	}
	if !ip.Equal(net.IPv4(192, 0, 2, 55)) {
		t.Fatalf("resolved %v", ip)
	}
	if conn.writes != 3 {
		t.Fatalf("query writes = %d, want initial plus two retries", conn.writes)
	}
}

func TestLookupRejectsAnswerFromWrongSourcePort(t *testing.T) {
	reply, err := response("wrong-port.local", net.IPv4(192, 0, 2, 56))
	if err != nil {
		t.Fatal(err)
	}
	conn := &fakePacketConn{response: reply, source: &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 9999}}
	listen := func(string, *net.Interface, *net.UDPAddr) (packetConn, error) { return conn, nil }
	_, err = lookup(context.Background(), "wrong-port.local", time.Second, listen)
	if err == nil || !errors.Is(err, io.EOF) {
		t.Fatalf("got %v, want ignored packet followed by EOF", err)
	}
}

type retryPacketConn struct {
	response []byte
	deadline time.Time
	writes   int
}

func (c *retryPacketConn) SetReadBuffer(int) error { return nil }
func (c *retryPacketConn) SetDeadline(deadline time.Time) error {
	c.deadline = deadline
	return nil
}
func (c *retryPacketConn) WriteToUDP(message []byte, _ *net.UDPAddr) (int, error) {
	c.writes++
	return len(message), nil
}
func (c *retryPacketConn) ReadFromUDP(buffer []byte) (int, *net.UDPAddr, error) {
	if c.writes < 3 {
		if wait := time.Until(c.deadline); wait > 0 {
			time.Sleep(wait)
		}
		return 0, nil, &net.OpError{Op: "read", Net: "udp4", Err: os.ErrDeadlineExceeded}
	}
	return copy(buffer, c.response), multicastAddress, nil
}
func (c *retryPacketConn) Close() error { return nil }
func (c *fakePacketConn) Close() error {
	c.mutex.Lock()
	c.closed = true
	c.mutex.Unlock()
	return nil
}

func TestLookupUsesInjectedMulticastTransport(t *testing.T) {
	reply, err := response("ESPHOME-BLINK.LOCAL", net.IPv4(192, 0, 2, 44))
	if err != nil {
		t.Fatal(err)
	}
	conn := &fakePacketConn{response: reply}
	listen := func(network string, iface *net.Interface, address *net.UDPAddr) (packetConn, error) {
		if network != "udp4" || iface != nil || address.String() != "224.0.0.251:5353" {
			t.Fatalf("unexpected multicast listen: network=%q iface=%v address=%v", network, iface, address)
		}
		return conn, nil
	}
	ip, err := lookup(context.Background(), "esphome-blink.local", time.Second, listen)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !ip.Equal(net.IPv4(192, 0, 2, 44)) {
		t.Fatalf("resolved %v", ip)
	}
	items, err := questions(conn.written)
	if err != nil || len(items) != 1 || items[0].name != "esphome-blink.local." || items[0].qtype != typeA {
		t.Fatalf("unexpected query: items=%#v err=%v", items, err)
	}
	if conn.deadline.IsZero() {
		t.Fatal("lookup did not set a deadline")
	}
	conn.mutex.Lock()
	closed := conn.closed
	conn.mutex.Unlock()
	if !closed {
		t.Fatal("lookup did not close the multicast transport")
	}
}

func TestLookupPreservesTransportErrorAndHostname(t *testing.T) {
	underlying := errors.New("synthetic read failure")
	netErr := &net.OpError{Op: "read", Net: "udp4", Err: underlying}
	conn := &fakePacketConn{readErr: netErr}
	listen := func(string, *net.Interface, *net.UDPAddr) (packetConn, error) { return conn, nil }
	_, err := lookup(context.Background(), "missing-device.local", time.Second, listen)
	if err == nil {
		t.Fatal("lookup unexpectedly succeeded")
	}
	if !errors.Is(err, underlying) {
		t.Fatalf("error does not preserve cause: %v", err)
	}
	var gotNetErr *net.OpError
	if !errors.As(err, &gotNetErr) || gotNetErr != netErr {
		t.Fatalf("error does not preserve net.OpError: %v", err)
	}
	if !strings.Contains(err.Error(), "missing-device.local") {
		t.Fatalf("error omits hostname: %v", err)
	}
}

func TestLookupRespectsCanceledContextBeforeListen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	listen := func(string, *net.Interface, *net.UDPAddr) (packetConn, error) {
		called = true
		return nil, errors.New("must not be called")
	}
	_, err := lookup(ctx, "canceled.local", time.Second, listen)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context cancellation", err)
	}
	if called {
		t.Fatal("canceled lookup opened a multicast transport")
	}
}

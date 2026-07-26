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

func listedInterfaces(items ...net.Interface) interfaceListFunc {
	return func() ([]net.Interface, error) {
		return append([]net.Interface(nil), items...), nil
	}
}

func (c *fakePacketConn) SetReadBuffer(int) error { return nil }
func (c *fakePacketConn) SetReadDeadline(deadline time.Time) error {
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
	ip, err := lookupWithSchedule(
		context.Background(),
		"retry-device.local",
		100*time.Millisecond,
		listedInterfaces(net.Interface{Index: 1, Name: "device-lan"}),
		listen,
		[]time.Duration{5 * time.Millisecond, 10 * time.Millisecond},
	)
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
	_, err = lookup(context.Background(), "wrong-port.local", time.Second, listedInterfaces(net.Interface{Index: 1, Name: "device-lan"}), listen)
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
func (c *retryPacketConn) SetReadDeadline(deadline time.Time) error {
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

func TestLookupRetransmitDoesNotInheritExpiredReadDeadline(t *testing.T) {
	sink, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	listen := func(string, *net.Interface, *net.UDPAddr) (packetConn, error) {
		return net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	}
	originalAddress := multicastAddress
	multicastAddress = sink.LocalAddr().(*net.UDPAddr)
	t.Cleanup(func() { multicastAddress = originalAddress })

	const timeout = 80 * time.Millisecond
	started := time.Now()
	_, err = lookupWithSchedule(
		context.Background(),
		"silent.local",
		timeout,
		listedInterfaces(net.Interface{Index: 1, Name: "loopback"}),
		listen,
		[]time.Duration{10 * time.Millisecond, 25 * time.Millisecond},
	)
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("lookup unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), "send mDNS query") {
		t.Fatalf("retransmit inherited an expired read deadline after %v: %v", elapsed, err)
	}
	if !strings.Contains(err.Error(), "read mDNS answer") {
		t.Fatalf("lookup ended for an unexpected reason after %v: %v", elapsed, err)
	}
	if elapsed < timeout/2 {
		t.Fatalf("lookup ended at the first retry after %v, want the overall %v budget", elapsed, timeout)
	}
}

func TestLookupUsesInjectedMulticastTransport(t *testing.T) {
	reply, err := response("ESPHOME-BLINK.LOCAL", net.IPv4(192, 0, 2, 44))
	if err != nil {
		t.Fatal(err)
	}
	conn := &fakePacketConn{response: reply}
	listen := func(network string, iface *net.Interface, address *net.UDPAddr) (packetConn, error) {
		if network != "udp4" || iface == nil || iface.Name != "device-lan" || address.String() != "224.0.0.251:5353" {
			t.Fatalf("unexpected multicast listen: network=%q iface=%v address=%v", network, iface, address)
		}
		return conn, nil
	}
	ip, err := lookup(
		context.Background(),
		"esphome-blink.local",
		time.Second,
		listedInterfaces(net.Interface{Index: 1, Name: "device-lan"}),
		listen,
	)
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
	_, err := lookup(
		context.Background(),
		"missing-device.local",
		time.Second,
		listedInterfaces(net.Interface{Index: 1, Name: "device-lan"}),
		listen,
	)
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
	_, err := lookup(ctx, "canceled.local", time.Second, listedInterfaces(net.Interface{Index: 1, Name: "device-lan"}), listen)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context cancellation", err)
	}
	if called {
		t.Fatal("canceled lookup opened a multicast transport")
	}
}

func TestLookupReturnsAnswerFromSecondInterface(t *testing.T) {
	reply, err := response("multi-homed.local", net.IPv4(192, 0, 2, 20))
	if err != nil {
		t.Fatal(err)
	}
	silent := newBlockingPacketConn()
	answering := &fakePacketConn{response: reply}
	listen := func(_ string, iface *net.Interface, _ *net.UDPAddr) (packetConn, error) {
		switch iface.Name {
		case "default-route":
			return silent, nil
		case "device-lan":
			return answering, nil
		default:
			return nil, errors.New("unexpected interface")
		}
	}
	ip, err := lookup(
		context.Background(),
		"multi-homed.local",
		time.Second,
		listedInterfaces(
			net.Interface{Index: 1, Name: "default-route"},
			net.Interface{Index: 2, Name: "device-lan"},
		),
		listen,
	)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !ip.Equal(net.IPv4(192, 0, 2, 20)) {
		t.Fatalf("resolved %v", ip)
	}
	if !silent.isClosed() || !answering.isClosed() {
		t.Fatal("lookup did not close every interface socket after the answer")
	}
}

func TestLookupSkipsInterfaceThatFailsToOpen(t *testing.T) {
	reply, err := response("degraded.local", net.IPv4(192, 0, 2, 21))
	if err != nil {
		t.Fatal(err)
	}
	answering := &fakePacketConn{response: reply}
	openFailure := errors.New("synthetic interface failure")
	listen := func(_ string, iface *net.Interface, _ *net.UDPAddr) (packetConn, error) {
		if iface.Name == "restricted" {
			return nil, openFailure
		}
		return answering, nil
	}
	ip, err := lookup(
		context.Background(),
		"degraded.local",
		time.Second,
		listedInterfaces(
			net.Interface{Index: 1, Name: "restricted"},
			net.Interface{Index: 2, Name: "device-lan"},
		),
		listen,
	)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !ip.Equal(net.IPv4(192, 0, 2, 21)) {
		t.Fatalf("resolved %v", ip)
	}
}

func TestLookupFailsWhenNoInterfaceSocketOpens(t *testing.T) {
	firstErr := errors.New("synthetic first failure")
	secondErr := errors.New("synthetic second failure")
	listen := func(_ string, iface *net.Interface, _ *net.UDPAddr) (packetConn, error) {
		if iface.Name == "first" {
			return nil, firstErr
		}
		return nil, secondErr
	}
	_, err := lookup(
		context.Background(),
		"unavailable.local",
		time.Second,
		listedInterfaces(
			net.Interface{Index: 1, Name: "first"},
			net.Interface{Index: 2, Name: "second"},
		),
		listen,
	)
	if err == nil || !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("error = %v, want both socket-open failures", err)
	}
	want := `could not open an mDNS socket for "unavailable.local" on 2 interface(s): first, second`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestLookupFailureNamesEveryTriedInterface(t *testing.T) {
	listen := func(string, *net.Interface, *net.UDPAddr) (packetConn, error) {
		return &fakePacketConn{readErr: io.EOF}, nil
	}
	_, err := lookup(
		context.Background(),
		"silent.local",
		time.Second,
		listedInterfaces(
			net.Interface{Index: 3, Name: "loopback"},
			net.Interface{Index: 2, Name: "device-lan"},
			net.Interface{Index: 1, Name: "default-route"},
		),
		listen,
	)
	if err == nil {
		t.Fatal("lookup unexpectedly succeeded")
	}
	want := `no mDNS answer for "silent.local" on 3 interface(s): default-route, device-lan, loopback`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestLookupRetransmitsOnEachInterface(t *testing.T) {
	reply, err := response("retry-all.local", net.IPv4(192, 0, 2, 22))
	if err != nil {
		t.Fatal(err)
	}
	first := &retryPacketConn{response: reply}
	second := &retryPacketConn{response: reply}
	listen := func(_ string, iface *net.Interface, _ *net.UDPAddr) (packetConn, error) {
		if iface.Name == "first" {
			return first, nil
		}
		return second, nil
	}
	ip, err := lookupWithSchedule(
		context.Background(),
		"retry-all.local",
		100*time.Millisecond,
		listedInterfaces(
			net.Interface{Index: 1, Name: "first"},
			net.Interface{Index: 2, Name: "second"},
		),
		listen,
		[]time.Duration{5 * time.Millisecond, 10 * time.Millisecond},
	)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !ip.Equal(net.IPv4(192, 0, 2, 22)) {
		t.Fatalf("resolved %v", ip)
	}
	if first.writes < 2 || second.writes < 2 {
		t.Fatalf("writes = %d and %d, want a retry on every interface", first.writes, second.writes)
	}
}

func TestLookupCancellationClosesEveryInterfaceSocket(t *testing.T) {
	first := newBlockingPacketConn()
	second := newBlockingPacketConn()
	listen := func(_ string, iface *net.Interface, _ *net.UDPAddr) (packetConn, error) {
		if iface.Name == "first" {
			return first, nil
		}
		return second, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := lookup(
			ctx,
			"canceled.local",
			time.Second,
			listedInterfaces(
				net.Interface{Index: 1, Name: "first"},
				net.Interface{Index: 2, Name: "second"},
			),
			listen,
		)
		done <- err
	}()
	first.waitForWrite(t)
	second.waitForWrite(t)
	cancel()
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lookup error = %v, want context cancellation", err)
	}
	if !first.isClosed() || !second.isClosed() {
		t.Fatal("canceled lookup did not close every interface socket")
	}
}

func TestLookupAndResponderUseRealUDPLoopback(t *testing.T) {
	loopback := findIPv4Loopback(t)
	responder, err := NewResponder("real-loopback.local", net.IPv4(127, 0, 0, 42))
	if err != nil {
		t.Fatalf("new responder: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- responder.Serve(ctx) }()

	ip, err := LookupInterface(ctx, "real-loopback.local", time.Second, loopback)
	if err != nil {
		cancel()
		_ = responder.Close()
		<-serveDone
		t.Fatalf("real loopback lookup: %v", err)
	}
	if !ip.Equal(net.IPv4(127, 0, 0, 42)) {
		t.Fatalf("resolved %v", ip)
	}
	cancel()
	_ = responder.Close()
	if err := <-serveDone; err != nil {
		t.Fatalf("serve responder: %v", err)
	}
}

func TestMulticastInterfacesIncludesIPv4Loopback(t *testing.T) {
	loopback := findIPv4Loopback(t)
	interfaces, err := multicastInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	for i := range interfaces {
		if interfaces[i].Index == loopback.Index {
			return
		}
	}
	t.Fatalf("IPv4 loopback %q was omitted from multicast candidates", loopback.Name)
}

func TestUsableMulticastInterfacesFiltersAndSortsCandidates(t *testing.T) {
	interfaces := []net.Interface{
		{Index: 5, Name: "device-lan", Flags: net.FlagUp | net.FlagMulticast},
		{Index: 1, Name: "down", Flags: net.FlagMulticast},
		{Index: 4, Name: "loopback", Flags: net.FlagUp | net.FlagLoopback},
		{Index: 2, Name: "unicast-only", Flags: net.FlagUp},
		{Index: 3, Name: "ipv6-only", Flags: net.FlagUp | net.FlagMulticast},
	}
	addresses := map[int][]net.Addr{
		1: {&net.IPNet{IP: net.IPv4(192, 0, 2, 1), Mask: net.CIDRMask(24, 32)}},
		3: {&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)}},
		4: {&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}},
		5: {&net.IPNet{IP: net.IPv4(192, 0, 2, 10), Mask: net.CIDRMask(24, 32)}},
	}
	got := usableMulticastInterfaces(interfaces, func(iface *net.Interface) ([]net.Addr, error) {
		return addresses[iface.Index], nil
	})
	if len(got) != 2 || got[0].Name != "loopback" || got[1].Name != "device-lan" {
		t.Fatalf("candidates = %#v, want loopback then device-lan", got)
	}
}

type blockingPacketConn struct {
	closed    chan struct{}
	written   chan struct{}
	closeOnce sync.Once
	writeOnce sync.Once
	mutex     sync.Mutex
	writes    int
}

func newBlockingPacketConn() *blockingPacketConn {
	return &blockingPacketConn{closed: make(chan struct{}), written: make(chan struct{})}
}

func (c *blockingPacketConn) SetReadBuffer(int) error         { return nil }
func (c *blockingPacketConn) SetReadDeadline(time.Time) error { return nil }
func (c *blockingPacketConn) ReadFromUDP([]byte) (int, *net.UDPAddr, error) {
	<-c.closed
	return 0, nil, net.ErrClosed
}
func (c *blockingPacketConn) WriteToUDP(message []byte, _ *net.UDPAddr) (int, error) {
	c.mutex.Lock()
	c.writes++
	c.mutex.Unlock()
	c.writeOnce.Do(func() { close(c.written) })
	return len(message), nil
}
func (c *blockingPacketConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}
func (c *blockingPacketConn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}
func (c *blockingPacketConn) waitForWrite(t *testing.T) {
	t.Helper()
	select {
	case <-c.written:
	case <-time.After(time.Second):
		t.Fatal("mDNS query was not written")
	}
}

func (c *fakePacketConn) isClosed() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.closed
}

func findIPv4Loopback(t *testing.T) *net.Interface {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for i := range interfaces {
		iface := interfaces[i]
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err == nil && hasIPv4Address(addresses) {
			return &iface
		}
	}
	t.Fatal("no active IPv4 loopback interface")
	return nil
}

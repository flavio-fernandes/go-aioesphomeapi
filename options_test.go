package aioesphomeapi

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestIsMDNSHost(t *testing.T) {
	tests := map[string]bool{
		"esphome-blink.local":  true,
		"ESPHOME-BLINK.LOCAL.": true,
		"not-local.example":    false,
		"almostlocal":          false,
		"local":                false,
	}
	for host, want := range tests {
		if got := isMDNSHost(host); got != want {
			t.Errorf("isMDNSHost(%q) = %t, want %t", host, got, want)
		}
	}
}

func TestDefaultDialerResolvesLocalWithInjectedLookup(t *testing.T) {
	lookupCalled := false
	dialCalled := false
	lookup := func(_ context.Context, host string, timeout time.Duration, iface *net.Interface) (net.IP, error) {
		lookupCalled = true
		if host != "ESPHOME-BLINK.LOCAL" || timeout != 3*time.Second {
			t.Fatalf("unexpected lookup: host=%q timeout=%v", host, timeout)
		}
		if iface != nil {
			t.Fatalf("unexpected pinned interface: %v", iface)
		}
		return net.IPv4(192, 0, 2, 44), nil
	}
	dial := func(_ context.Context, network, address string) (net.Conn, error) {
		dialCalled = true
		if network != "tcp" || address != "192.0.2.44:6053" {
			t.Fatalf("unexpected dial: network=%q address=%q", network, address)
		}
		front, back := net.Pipe()
		t.Cleanup(func() { _ = back.Close() })
		return front, nil
	}
	conn, err := defaultDialerWith(3*time.Second, nil, lookup, dial)(context.Background(), "tcp", "ESPHOME-BLINK.LOCAL:6053")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()
	if !lookupCalled || !dialCalled {
		t.Fatalf("lookup=%t dial=%t", lookupCalled, dialCalled)
	}
}

func TestDefaultDialerPreservesMDNSError(t *testing.T) {
	underlying := errors.New("synthetic mDNS timeout")
	lookup := func(context.Context, string, time.Duration, *net.Interface) (net.IP, error) {
		return nil, underlying
	}
	dial := func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("TCP dial ran after mDNS failure")
		return nil, nil
	}
	_, err := defaultDialerWith(time.Second, nil, lookup, dial)(context.Background(), "tcp", "missing-device.local:6053")
	if !errors.Is(err, ErrNameResolution) || !errors.Is(err, underlying) {
		t.Fatalf("error chain lost mDNS category or cause: %v", err)
	}
	if !strings.Contains(err.Error(), "missing-device.local") {
		t.Fatalf("error omits hostname: %v", err)
	}
	if errors.Is(err, ErrHello) || errors.Is(err, ErrNoiseHandshake) {
		t.Fatalf("mDNS error has an unrelated failure category: %v", err)
	}
}

func TestDefaultDialerLeavesNonLocalNamesAlone(t *testing.T) {
	lookup := func(context.Context, string, time.Duration, *net.Interface) (net.IP, error) {
		t.Fatal("mDNS lookup ran for a non-local name")
		return nil, nil
	}
	underlying := errors.New("synthetic TCP failure")
	dial := func(_ context.Context, _, address string) (net.Conn, error) {
		if address != "device.example:6053" {
			t.Fatalf("address changed: %q", address)
		}
		return nil, underlying
	}
	_, err := defaultDialerWith(time.Second, nil, lookup, dial)(context.Background(), "tcp", "device.example:6053")
	if !errors.Is(err, underlying) {
		t.Fatalf("TCP cause was not preserved: %v", err)
	}
}

func TestDefaultDialerPinsMulticastInterface(t *testing.T) {
	selected := &net.Interface{Index: 7, Name: "device-lan"}
	lookup := func(_ context.Context, _ string, _ time.Duration, iface *net.Interface) (net.IP, error) {
		if iface == nil || iface.Index != selected.Index || iface.Name != selected.Name {
			t.Fatalf("lookup interface = %#v, want %#v", iface, selected)
		}
		return net.IPv4(192, 0, 2, 45), nil
	}
	dial := func(_ context.Context, _, address string) (net.Conn, error) {
		if address != "192.0.2.45:6053" {
			t.Fatalf("address = %q", address)
		}
		front, back := net.Pipe()
		t.Cleanup(func() { _ = back.Close() })
		return front, nil
	}
	conn, err := defaultDialerWith(time.Second, selected, lookup, dial)(context.Background(), "tcp", "pinned.local:6053")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()
}

func TestWithMulticastInterfaceCopiesSelection(t *testing.T) {
	selected := &net.Interface{Index: 9, Name: "device-lan"}
	var cfg config
	WithMulticastInterface(selected)(&cfg)
	selected.Index = 10
	selected.Name = "changed"
	if cfg.multicastInterface == nil || cfg.multicastInterface.Index != 9 || cfg.multicastInterface.Name != "device-lan" {
		t.Fatalf("configured interface changed with caller value: %#v", cfg.multicastInterface)
	}
}

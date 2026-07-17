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
	lookup := func(_ context.Context, host string, timeout time.Duration) (net.IP, error) {
		lookupCalled = true
		if host != "ESPHOME-BLINK.LOCAL" || timeout != 3*time.Second {
			t.Fatalf("unexpected lookup: host=%q timeout=%v", host, timeout)
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
	conn, err := defaultDialerWith(3*time.Second, lookup, dial)(context.Background(), "tcp", "ESPHOME-BLINK.LOCAL:6053")
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
	lookup := func(context.Context, string, time.Duration) (net.IP, error) {
		return nil, underlying
	}
	dial := func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("TCP dial ran after mDNS failure")
		return nil, nil
	}
	_, err := defaultDialerWith(time.Second, lookup, dial)(context.Background(), "tcp", "missing-device.local:6053")
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
	lookup := func(context.Context, string, time.Duration) (net.IP, error) {
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
	_, err := defaultDialerWith(time.Second, lookup, dial)(context.Background(), "tcp", "device.example:6053")
	if !errors.Is(err, underlying) {
		t.Fatalf("TCP cause was not preserved: %v", err)
	}
}

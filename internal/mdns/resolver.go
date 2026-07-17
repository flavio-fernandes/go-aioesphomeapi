// Package mdns provides the narrow multicast DNS behavior required to connect
// to ESPHome device names ending in .local. It is intentionally internal: this
// is hostname resolution, not general service discovery.
package mdns

import (
	"context"
	"fmt"
	"net"
	"time"
)

const fallbackTimeout = 5 * time.Second

var multicastAddress = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

type packetConn interface {
	SetReadBuffer(int) error
	SetDeadline(time.Time) error
	WriteToUDP([]byte, *net.UDPAddr) (int, error)
	ReadFromUDP([]byte) (int, *net.UDPAddr, error)
	Close() error
}

type listenMulticastFunc func(string, *net.Interface, *net.UDPAddr) (packetConn, error)

// Lookup resolves one .local host with a bounded IPv4 multicast query.
func Lookup(ctx context.Context, host string, timeout time.Duration) (net.IP, error) {
	return lookup(ctx, host, timeout, func(network string, iface *net.Interface, address *net.UDPAddr) (packetConn, error) {
		return net.ListenMulticastUDP(network, iface, address)
	})
}

func lookup(ctx context.Context, host string, timeout time.Duration, listen listenMulticastFunc) (net.IP, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("resolve %q with mDNS: %w", host, err)
	}
	message, err := query(host)
	if err != nil {
		return nil, fmt.Errorf("build mDNS query for %q: %w", host, err)
	}
	conn, err := listen("udp4", nil, multicastAddress)
	if err != nil {
		return nil, fmt.Errorf("join mDNS multicast group for %q: %w", host, err)
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(maxDNSMessageSize)
	deadline := time.Now().Add(effectiveTimeout(ctx, timeout))
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set mDNS deadline for %q: %w", host, err)
	}
	if _, err := conn.WriteToUDP(message, multicastAddress); err != nil {
		return nil, fmt.Errorf("send mDNS query for %q: %w", host, err)
	}

	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	buffer := make([]byte, maxDNSMessageSize)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, fmt.Errorf("resolve %q with mDNS: %w", host, ctxErr)
			}
			return nil, fmt.Errorf("read mDNS answer for %q: %w", host, err)
		}
		if ip, ok := answerIP(buffer[:n], host); ok {
			return ip, nil
		}
	}
}

func effectiveTimeout(ctx context.Context, timeout time.Duration) time.Duration {
	if timeout <= 0 {
		timeout = fallbackTimeout
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < timeout {
			return max(remaining, time.Millisecond)
		}
	}
	return timeout
}

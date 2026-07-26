// Package mdns provides the narrow multicast DNS behavior required to connect
// to ESPHome device names ending in .local. It is intentionally internal: this
// is hostname resolution, not general service discovery.
package mdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

const fallbackTimeout = 5 * time.Second

var defaultRetransmitSchedule = []time.Duration{time.Second, 3 * time.Second}

var multicastAddress = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

type packetConn interface {
	SetReadBuffer(int) error
	SetReadDeadline(time.Time) error
	WriteToUDP([]byte, *net.UDPAddr) (int, error)
	ReadFromUDP([]byte) (int, *net.UDPAddr, error)
	Close() error
}

type listenMulticastFunc func(string, *net.Interface, *net.UDPAddr) (packetConn, error)

type interfaceListFunc func() ([]net.Interface, error)

type interfaceAddrsFunc func(*net.Interface) ([]net.Addr, error)

// Lookup resolves one .local host with a bounded IPv4 multicast query.
func Lookup(ctx context.Context, host string, timeout time.Duration) (net.IP, error) {
	return LookupInterface(ctx, host, timeout, nil)
}

// LookupInterface resolves one .local host on iface. A nil interface queries
// every usable IPv4 multicast interface and returns the first valid answer.
func LookupInterface(ctx context.Context, host string, timeout time.Duration, iface *net.Interface) (net.IP, error) {
	listInterfaces := multicastInterfaces
	if iface != nil {
		selected := *iface
		listInterfaces = func() ([]net.Interface, error) {
			return []net.Interface{selected}, nil
		}
	}
	return lookup(ctx, host, timeout, listInterfaces, func(network string, iface *net.Interface, address *net.UDPAddr) (packetConn, error) {
		return net.ListenMulticastUDP(network, iface, address)
	})
}

func lookup(ctx context.Context, host string, timeout time.Duration, listInterfaces interfaceListFunc, listen listenMulticastFunc) (net.IP, error) {
	return lookupWithSchedule(ctx, host, timeout, listInterfaces, listen, defaultRetransmitSchedule)
}

func lookupWithSchedule(ctx context.Context, host string, timeout time.Duration, listInterfaces interfaceListFunc, listen listenMulticastFunc, retransmitSchedule []time.Duration) (net.IP, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("resolve %q with mDNS: %w", host, err)
	}
	message, err := query(host)
	if err != nil {
		return nil, fmt.Errorf("build mDNS query for %q: %w", host, err)
	}
	interfaces, err := listInterfaces()
	if err != nil {
		return nil, fmt.Errorf("list IPv4 multicast interfaces for %q: %w", host, err)
	}
	slices.SortFunc(interfaces, func(a, b net.Interface) int {
		if a.Index != b.Index {
			return a.Index - b.Index
		}
		return strings.Compare(a.Name, b.Name)
	})
	if len(interfaces) == 0 {
		return nil, fmt.Errorf("no IPv4 multicast interfaces available for %q", host)
	}

	type interfaceConn struct {
		conn packetConn
	}
	opened := make([]interfaceConn, 0, len(interfaces))
	openErrors := make([]error, 0, len(interfaces))
	names := make([]string, 0, len(interfaces))
	for i := range interfaces {
		iface := interfaces[i]
		names = append(names, iface.Name)
		conn, listenErr := listen("udp4", &iface, multicastAddress)
		if listenErr != nil {
			openErrors = append(openErrors, fmt.Errorf("%s: %w", iface.Name, listenErr))
			continue
		}
		_ = conn.SetReadBuffer(maxDNSMessageSize)
		opened = append(opened, interfaceConn{conn: conn})
	}
	if len(opened) == 0 {
		return nil, fmt.Errorf(
			"could not open an mDNS socket for %q on %d interface(s): %s: %w",
			host, len(names), strings.Join(names, ", "), errors.Join(openErrors...),
		)
	}

	start := time.Now()
	deadline := start.Add(effectiveTimeout(ctx, timeout))
	lookupCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var closeOnce sync.Once
	closeAll := func() {
		closeOnce.Do(func() {
			for _, item := range opened {
				_ = item.conn.Close()
			}
		})
	}
	stop := context.AfterFunc(lookupCtx, closeAll)
	defer stop()

	type result struct {
		ip  net.IP
		err error
	}
	results := make(chan result, len(opened))
	var workers sync.WaitGroup
	workers.Add(len(opened))
	for _, item := range opened {
		go func(conn packetConn) {
			defer workers.Done()
			ip, lookupErr := lookupOnConn(lookupCtx, host, message, conn, start, deadline, retransmitSchedule)
			results <- result{ip: ip, err: lookupErr}
		}(item.conn)
	}

	lookupErrors := append([]error(nil), openErrors...)
	for range opened {
		result := <-results
		if result.ip != nil {
			cancel()
			closeAll()
			workers.Wait()
			return result.ip, nil
		}
		if result.err != nil {
			lookupErrors = append(lookupErrors, result.err)
		}
	}
	cancel()
	closeAll()
	workers.Wait()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("resolve %q with mDNS: %w", host, ctxErr)
	}
	return nil, fmt.Errorf(
		"no mDNS answer for %q on %d interface(s): %s: %w",
		host, len(names), strings.Join(names, ", "), errors.Join(lookupErrors...),
	)
}

func lookupOnConn(ctx context.Context, host string, message []byte, conn packetConn, start, deadline time.Time, retransmitSchedule []time.Duration) (net.IP, error) {
	writeQuery := func() error {
		if _, err := conn.WriteToUDP(message, multicastAddress); err != nil {
			return fmt.Errorf("send mDNS query for %q: %w", host, err)
		}
		return nil
	}
	if err := writeQuery(); err != nil {
		return nil, err
	}

	buffer := make([]byte, maxDNSMessageSize)
	retransmit := 0
	for {
		readDeadline := deadline
		var retransmitAt time.Time
		if retransmit < len(retransmitSchedule) {
			candidate := start.Add(retransmitSchedule[retransmit])
			if candidate.Before(deadline) {
				retransmitAt = candidate
				readDeadline = candidate
			}
		}
		if err := conn.SetReadDeadline(readDeadline); err != nil {
			return nil, fmt.Errorf("set mDNS read deadline for %q: %w", host, err)
		}
		n, source, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, fmt.Errorf("resolve %q with mDNS: %w", host, ctxErr)
			}
			var netErr net.Error
			if !retransmitAt.IsZero() && errors.As(err, &netErr) && netErr.Timeout() && !time.Now().Before(retransmitAt) {
				if err := writeQuery(); err != nil {
					return nil, err
				}
				retransmit++
				continue
			}
			return nil, fmt.Errorf("read mDNS answer for %q: %w", host, err)
		}
		if source == nil || source.Port != multicastAddress.Port {
			continue
		}
		if ip, ok := answerIP(buffer[:n], host); ok {
			return ip, nil
		}
	}
}

func multicastInterfaces() ([]net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	return usableMulticastInterfaces(interfaces, func(iface *net.Interface) ([]net.Addr, error) {
		return iface.Addrs()
	}), nil
}

func usableMulticastInterfaces(interfaces []net.Interface, addresses interfaceAddrsFunc) []net.Interface {
	candidates := make([]net.Interface, 0, len(interfaces))
	for i := range interfaces {
		iface := interfaces[i]
		if iface.Flags&net.FlagUp == 0 || iface.Flags&(net.FlagMulticast|net.FlagLoopback) == 0 {
			continue
		}
		items, err := addresses(&iface)
		if err != nil || !hasIPv4Address(items) {
			continue
		}
		candidates = append(candidates, iface)
	}
	slices.SortFunc(candidates, func(a, b net.Interface) int {
		if a.Index != b.Index {
			return a.Index - b.Index
		}
		return strings.Compare(a.Name, b.Name)
	})
	return candidates
}

func hasIPv4Address(addresses []net.Addr) bool {
	for _, address := range addresses {
		var ip net.IP
		switch value := address.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		default:
			host, _, err := net.SplitHostPort(address.String())
			if err != nil {
				host = address.String()
			}
			ip = net.ParseIP(strings.TrimSuffix(host, "%"))
		}
		if ip != nil && ip.To4() != nil {
			return true
		}
	}
	return false
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

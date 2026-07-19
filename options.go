package aioesphomeapi

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/internal/mdns"
)

// DialContextFunc makes the transport injectable without exposing protocol
// internals. It is primarily useful for the deterministic device simulator.
type DialContextFunc func(context.Context, string, string) (net.Conn, error)

type config struct {
	clientInfo        string
	encryptionKey     string
	expectedName      string
	insecurePlaintext bool
	dialContext       DialContextFunc
	maxFrameSize      int
	callbackQueueSize int
	keepaliveInterval time.Duration
	keepaliveTimeout  time.Duration
}

// Option configures a Client before it connects.
type Option func(*config)

// WithClientInfo sets the value advertised to the ESPHome device.
func WithClientInfo(info string) Option {
	return func(c *config) { c.clientInfo = info }
}

// WithEncryptionKey configures the base64-encoded 32-byte ESPHome Noise key.
func WithEncryptionKey(key string) Option {
	return func(c *config) { c.encryptionKey = key }
}

// WithExpectedName requires both the Noise handshake name, when encrypted,
// and the Native API Hello name to match. An empty value disables the check.
func WithExpectedName(name string) Option {
	return func(c *config) { c.expectedName = name }
}

// WithInsecurePlaintext explicitly permits an unencrypted Native API
// connection. It should only be used on an isolated test network.
func WithInsecurePlaintext() Option {
	return func(c *config) { c.insecurePlaintext = true }
}

// WithDialContext injects connection creation. The simulator uses this option
// to provide net.Pipe connections without opening a listening socket.
func WithDialContext(fn DialContextFunc) Option {
	return func(c *config) { c.dialContext = fn }
}

// WithMaxFrameSize lowers the default 64 KiB peer-allocation bound.
func WithMaxFrameSize(bytes int) Option {
	return func(c *config) { c.maxFrameSize = bytes }
}

// WithCallbackQueueSize sets the bounded asynchronous callback queue.
func WithCallbackQueueSize(size int) Option {
	return func(c *config) { c.callbackQueueSize = size }
}

// WithKeepalive starts an automatic liveness probe every interval once the
// connection is established. Each probe is bounded by timeout; the first
// probe the device never answers closes the connection with an ErrKeepalive
// close reason. Both durations must be positive or dialing fails. Keepalive
// stays disabled unless requested, so the MGMT facade path, whose session
// layer owns liveness and reconnect policy, is unchanged.
func WithKeepalive(interval, timeout time.Duration) Option {
	return func(c *config) {
		c.keepaliveInterval = interval
		c.keepaliveTimeout = timeout
	}
}

func defaultDialer(timeout time.Duration) DialContextFunc {
	dialer := &net.Dialer{Timeout: timeout}
	return defaultDialerWith(timeout, mdns.Lookup, dialer.DialContext)
}

type mdnsLookupFunc func(context.Context, string, time.Duration) (net.IP, error)

func defaultDialerWith(timeout time.Duration, lookup mdnsLookupFunc, dial DialContextFunc) DialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err == nil && isMDNSHost(host) {
			ip, lookupErr := lookup(ctx, host, timeout)
			if lookupErr != nil {
				return nil, fmt.Errorf("%w for %q: %w", ErrNameResolution, host, lookupErr)
			}
			address = net.JoinHostPort(ip.String(), port)
		}
		return dial(ctx, network, address)
	}
}

func isMDNSHost(host string) bool {
	return strings.HasSuffix(strings.TrimSuffix(strings.ToLower(host), "."), ".local")
}

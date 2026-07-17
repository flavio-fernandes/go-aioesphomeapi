package aioesphomeapi

import (
	"context"
	"net"
	"time"
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

// WithExpectedName requires the name received during the Noise handshake to
// match name. An empty value disables this additional identity check.
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

func defaultDialer(timeout time.Duration) DialContextFunc {
	dialer := &net.Dialer{Timeout: timeout}
	return dialer.DialContext
}

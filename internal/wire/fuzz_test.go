package wire

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

func FuzzPlainFramerRead(f *testing.F) {
	f.Add([]byte{0, 0, 8})
	f.Add([]byte{0, 3, 31, 'f', 'a', 'n'})
	f.Add([]byte{1, 0, 8})
	f.Fuzz(func(t *testing.T, data []byte) {
		const limit = 256
		framer := NewPlainFramer(&memoryConn{Reader: bytes.NewReader(data)}, limit)
		_, payload, _ := framer.ReadFrame()
		if len(payload) > limit {
			t.Fatalf("decoded payload length %d exceeds limit %d", len(payload), limit)
		}
	})
}

func FuzzDecode(f *testing.F) {
	f.Add(uint32(2), []byte{})
	f.Add(uint32(8), []byte{0x80})
	f.Add(uint32(65000), []byte{})
	f.Fuzz(func(t *testing.T, id uint32, payload []byte) {
		if len(payload) > DefaultMaxFrameSize {
			payload = payload[:DefaultMaxFrameSize]
		}
		_, _ = Decode(id, payload)
	})
}

type memoryConn struct{ io.Reader }

func (c *memoryConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c *memoryConn) Close() error                     { return nil }
func (c *memoryConn) LocalAddr() net.Addr              { return memoryAddr("local") }
func (c *memoryConn) RemoteAddr() net.Addr             { return memoryAddr("remote") }
func (c *memoryConn) SetDeadline(time.Time) error      { return nil }
func (c *memoryConn) SetReadDeadline(time.Time) error  { return nil }
func (c *memoryConn) SetWriteDeadline(time.Time) error { return nil }

type memoryAddr string

func (a memoryAddr) Network() string { return "memory" }
func (a memoryAddr) String() string  { return string(a) }

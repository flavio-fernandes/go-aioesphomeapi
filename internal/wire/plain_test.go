package wire

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestPlainFramerRoundTripAndBound(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	writer, reader := NewPlainFramer(left, 8), NewPlainFramer(right, 8)
	done := make(chan error, 1)
	go func() { done <- writer.WriteFrame(31, []byte("fan")) }()
	id, payload, err := reader.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	if id != 31 || string(payload) != "fan" {
		t.Fatalf("got %d %q", id, payload)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteFrame(31, make([]byte, 9)); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("got %v", err)
	}
}

func TestPlainFramerHandlesFragmentedCoalescedFrames(t *testing.T) {
	var encoded bytes.Buffer
	writer := NewPlainFramer(&writeOnlyConn{Writer: &encoded}, 32)
	if err := writer.WriteFrame(31, []byte("fan")); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteFrame(32, []byte("light")); err != nil {
		t.Fatal(err)
	}

	reader := NewPlainFramer(&memoryConn{Reader: &chunkReader{reader: bytes.NewReader(encoded.Bytes()), chunk: 1}}, 32)
	for _, want := range []struct {
		id      uint32
		payload string
	}{{31, "fan"}, {32, "light"}} {
		id, payload, err := reader.ReadFrame()
		if err != nil {
			t.Fatal(err)
		}
		if id != want.id || string(payload) != want.payload {
			t.Fatalf("got (%d, %q), want (%d, %q)", id, payload, want.id, want.payload)
		}
	}
}

func TestPlainFramerHandlesPartialWrites(t *testing.T) {
	var output bytes.Buffer
	framer := NewPlainFramer(&writeOnlyConn{Writer: &chunkWriter{writer: &output, chunk: 2}}, 32)
	if err := framer.WriteFrame(7, []byte("partial")); err != nil {
		t.Fatal(err)
	}
	reader := NewPlainFramer(&memoryConn{Reader: bytes.NewReader(output.Bytes())}, 32)
	id, payload, err := reader.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	if id != 7 || string(payload) != "partial" {
		t.Fatalf("got (%d, %q)", id, payload)
	}
}

func TestPlainFramerCapsConfiguredAllocationAndRejectsBeforeRead(t *testing.T) {
	framer := NewPlainFramer(&memoryConn{Reader: bytes.NewReader([]byte{0, 0x81, 0x80, 0x04})}, 1<<30).(*plainFramer)
	if framer.maxFrame != DefaultMaxFrameSize {
		t.Fatalf("max frame = %d, want %d", framer.maxFrame, DefaultMaxFrameSize)
	}
	if _, _, err := framer.ReadFrame(); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("got %v, want frame-too-large", err)
	}
}

type chunkReader struct {
	reader io.Reader
	chunk  int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(p) > r.chunk {
		p = p[:r.chunk]
	}
	return r.reader.Read(p)
}

type chunkWriter struct {
	writer io.Writer
	chunk  int
}

func (w *chunkWriter) Write(p []byte) (int, error) {
	if len(p) > w.chunk {
		p = p[:w.chunk]
	}
	return w.writer.Write(p)
}

type writeOnlyConn struct {
	io.Writer
}

func (c *writeOnlyConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c *writeOnlyConn) Close() error                     { return nil }
func (c *writeOnlyConn) LocalAddr() net.Addr              { return memoryAddr("local") }
func (c *writeOnlyConn) RemoteAddr() net.Addr             { return memoryAddr("remote") }
func (c *writeOnlyConn) SetDeadline(time.Time) error      { return nil }
func (c *writeOnlyConn) SetReadDeadline(time.Time) error  { return nil }
func (c *writeOnlyConn) SetWriteDeadline(time.Time) error { return nil }

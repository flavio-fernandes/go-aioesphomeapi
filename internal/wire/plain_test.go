package wire

import (
	"errors"
	"net"
	"testing"
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

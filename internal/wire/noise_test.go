package wire

import (
	"bytes"
	"errors"
	"io"
	"math"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNoiseClientReportsSanitizedKeyRejection(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	key := bytes.Repeat([]byte{0x42}, 32)
	serverDone := make(chan error, 1)
	go func() {
		var preamble [3]byte
		if _, err := io.ReadFull(server, preamble[:]); err != nil {
			serverDone <- err
			return
		}
		if _, err := readNoisePacket(server); err != nil {
			serverDone <- err
			return
		}
		if err := writeNoisePacket(server, []byte{1, 't', 'e', 's', 't', 0, 0}); err != nil {
			serverDone <- err
			return
		}
		reason := append([]byte("Handshake MAC failure\n"), bytes.Repeat([]byte{'x'}, maxNoiseRejectionReason+20)...)
		serverDone <- writeNoisePacket(server, append([]byte{1}, reason...))
	}()

	_, err := NewNoiseClientFramer(client, key, "test", time.Second, DefaultMaxFrameSize)
	if !errors.Is(err, ErrNoiseHandshake) || !errors.Is(err, ErrNoiseKeyRejected) {
		t.Fatalf("got %v, want handshake and rejected-key categories", err)
	}
	if !strings.Contains(err.Error(), "Handshake MAC failure") {
		t.Fatalf("rejection reason was lost: %v", err)
	}
	if strings.ContainsAny(err.Error(), "\r\n\t") {
		t.Fatalf("rejection reason contains control characters: %q", err)
	}
	if len(err.Error()) > 180 {
		t.Fatalf("rejection reason was not capped: %d bytes", len(err.Error()))
	}
	if serverErr := <-serverDone; serverErr != nil {
		t.Fatalf("rejection peer: %v", serverErr)
	}
}

func TestNoiseFrameClampAndMessageTypeCategory(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	key := bytes.Repeat([]byte{0x24}, 32)
	serverResult := make(chan Framer, 1)
	serverErr := make(chan error, 1)
	go func() {
		framer, err := NewNoiseServerFramer(server, key, "test", time.Second, DefaultMaxFrameSize)
		if err != nil {
			serverErr <- err
			return
		}
		serverResult <- framer
	}()
	clientFramer, err := NewNoiseClientFramer(client, key, "test", time.Second, DefaultMaxFrameSize)
	if err != nil {
		t.Fatal(err)
	}
	var serverFramer Framer
	select {
	case err := <-serverErr:
		t.Fatal(err)
	case serverFramer = <-serverResult:
	}
	defer clientFramer.Close()
	defer serverFramer.Close()
	if got := clientFramer.(*noiseFramer).maxFrame; got != maxNoisePacketSize-20 {
		t.Fatalf("client max frame = %d, want %d", got, maxNoisePacketSize-20)
	}
	if got := serverFramer.(*noiseFramer).maxFrame; got != maxNoisePacketSize-20 {
		t.Fatalf("server max frame = %d, want %d", got, maxNoisePacketSize-20)
	}
	if err := clientFramer.WriteFrame(math.MaxUint16+1, nil); !errors.Is(err, ErrMessageType) || errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("got %v, want message-type category only", err)
	}
}

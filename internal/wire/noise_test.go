package wire

import (
	"bytes"
	"encoding/base64"
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

func TestNoiseClientRedactsKeyEchoedByPeer(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	key := bytes.Repeat([]byte{'B'}, 32)
	encodedKey := base64.StdEncoding.EncodeToString(key)
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
		reason := []byte("rejected " + string(key) + " " + encodedKey)
		serverDone <- writeNoisePacket(server, append([]byte{1}, reason...))
	}()

	_, err := NewNoiseClientFramer(client, key, "test", time.Second, DefaultMaxFrameSize)
	if !errors.Is(err, ErrNoiseHandshake) || !errors.Is(err, ErrNoiseKeyRejected) {
		t.Fatalf("got %v, want handshake and rejected-key categories", err)
	}
	if strings.Contains(err.Error(), string(key)) || strings.Contains(err.Error(), encodedKey) {
		t.Fatalf("peer-controlled error leaked key material: %q", err)
	}
	if !strings.Contains(err.Error(), "redacted") {
		t.Fatalf("redaction was not explicit: %v", err)
	}
	if serverErr := <-serverDone; serverErr != nil {
		t.Fatalf("rejection peer: %v", serverErr)
	}
}

func TestNoiseClientRedactsWrappedBase64KeyEchoedByPeer(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	key := bytes.Repeat([]byte{0x42}, 32)
	encodedKey := base64.StdEncoding.EncodeToString(key)
	wrappedKey := encodedKey[:12] + "\r\n" + encodedKey[12:28] + "\n" + encodedKey[28:]
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
		reason := []byte("rejected " + wrappedKey)
		serverDone <- writeNoisePacket(server, append([]byte{1}, reason...))
	}()

	_, err := NewNoiseClientFramer(client, key, "test", time.Second, DefaultMaxFrameSize)
	if !errors.Is(err, ErrNoiseHandshake) || !errors.Is(err, ErrNoiseKeyRejected) {
		t.Fatalf("got %v, want handshake and rejected-key categories", err)
	}
	if !strings.Contains(err.Error(), "redacted") {
		t.Fatalf("wrapped key echo was not redacted: %v", err)
	}
	for _, fragment := range []string{encodedKey[:12], encodedKey[12:28], encodedKey[28:]} {
		if strings.Contains(err.Error(), fragment) {
			t.Fatalf("peer-controlled error leaked wrapped key fragment %q: %q", fragment, err)
		}
	}
	if serverErr := <-serverDone; serverErr != nil {
		t.Fatalf("rejection peer: %v", serverErr)
	}
}

func TestNoisePacketsHandleFragmentedCoalescedAndPartialIO(t *testing.T) {
	var stream bytes.Buffer
	writer := &chunkWriter{writer: &stream, chunk: 2}
	for _, payload := range [][]byte{[]byte("first"), []byte("second")} {
		if err := writeNoisePacket(writer, payload); err != nil {
			t.Fatal(err)
		}
	}

	reader := &chunkReader{reader: bytes.NewReader(stream.Bytes()), chunk: 1}
	for _, want := range []string{"first", "second"} {
		payload, err := readNoisePacket(reader)
		if err != nil {
			t.Fatal(err)
		}
		if string(payload) != want {
			t.Fatalf("got %q, want %q", payload, want)
		}
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

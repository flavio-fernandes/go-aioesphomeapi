package aioesphomeapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/internal/wire"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

type scriptedFrame struct {
	id      uint32
	payload []byte
	err     error
}

type scriptedFramer struct {
	mutex        sync.Mutex
	frames       []scriptedFrame
	writeErr     error
	writeID      uint32
	writePayload []byte
	closed       chan struct{}
	once         sync.Once
}

func newScriptedFramer(frames ...scriptedFrame) *scriptedFramer {
	return &scriptedFramer{frames: frames, closed: make(chan struct{})}
}

func (f *scriptedFramer) ReadFrame() (uint32, []byte, error) {
	f.mutex.Lock()
	if len(f.frames) > 0 {
		frame := f.frames[0]
		f.frames = f.frames[1:]
		f.mutex.Unlock()
		return frame.id, frame.payload, frame.err
	}
	f.mutex.Unlock()
	<-f.closed
	return 0, nil, io.ErrClosedPipe
}

func (f *scriptedFramer) WriteFrame(id uint32, payload []byte) error {
	f.mutex.Lock()
	f.writeID = id
	f.writePayload = append([]byte(nil), payload...)
	err := f.writeErr
	f.mutex.Unlock()
	return err
}
func (f *scriptedFramer) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

func bareClient(f wire.Framer) *Client {
	return newClient(f, 4)
}

func TestDialPreservesNetOpErrorAndAddress(t *testing.T) {
	underlying := errors.New("synthetic connection refusal")
	netErr := &net.OpError{Op: "dial", Net: "tcp", Err: underlying}
	address := "unreachable-device.example:6053"
	_, err := DialWithContext(context.Background(), address, time.Second,
		WithEncryptionKey("not-decoded-before-dial"),
		WithDialContext(func(context.Context, string, string) (net.Conn, error) { return nil, netErr }),
	)
	if err == nil {
		t.Fatal("dial unexpectedly succeeded")
	}
	var gotNetErr *net.OpError
	if !errors.As(err, &gotNetErr) || gotNetErr != netErr || !errors.Is(err, underlying) {
		t.Fatalf("dial cause was not preserved: %v", err)
	}
	if !strings.Contains(err.Error(), address) {
		t.Fatalf("dial error omits address: %v", err)
	}
	if errors.Is(err, ErrNameResolution) || errors.Is(err, ErrHello) || errors.Is(err, ErrNoiseHandshake) {
		t.Fatalf("TCP dial error has an unrelated failure category: %v", err)
	}
}

func TestNoiseHandshakeErrorKeepsCategoryAndAddress(t *testing.T) {
	front, back := net.Pipe()
	_ = back.Close()
	address := "synthetic-device.local:6053"
	key := "kJ7hc0lJ0Zw9N3DcJzXn1kJ7hc0lJ0Zw9N3DcJzXn1k="
	_, err := DialWithContext(context.Background(), address, time.Second,
		WithEncryptionKey(key),
		WithDialContext(func(context.Context, string, string) (net.Conn, error) { return front, nil }),
	)
	if !errors.Is(err, ErrNoiseHandshake) {
		t.Fatalf("got %v, want Noise handshake category", err)
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("handshake error lost underlying transport cause: %v", err)
	}
	if !strings.Contains(err.Error(), address) || !strings.Contains(err.Error(), "Noise session") {
		t.Fatalf("handshake error lacks stage or address: %v", err)
	}
	if errors.Is(err, ErrNameResolution) || errors.Is(err, ErrHello) {
		t.Fatalf("Noise error has an unrelated failure category: %v", err)
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("handshake error leaked encoded key: %q", err)
	}
}

func TestDialReportsSanitizedPeerKeyRejection(t *testing.T) {
	front, back := net.Pipe()
	t.Cleanup(func() { _ = back.Close() })
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	peerDone := make(chan error, 1)
	go func() {
		var preamble [3]byte
		if _, err := io.ReadFull(back, preamble[:]); err != nil {
			peerDone <- err
			return
		}
		if _, err := readTestNoisePacket(back); err != nil {
			peerDone <- err
			return
		}
		if err := writeTestNoisePacket(back, []byte{1, 'r', 'e', 'j', 'e', 'c', 't', 'i', 'n', 'g', 0, 0}); err != nil {
			peerDone <- err
			return
		}
		peerDone <- writeTestNoisePacket(back, append([]byte{1}, []byte("Handshake MAC failure "+encoded)...))
	}()
	address := "rejecting-device.local:6053"
	_, err := DialWithContext(context.Background(), address, time.Second,
		WithEncryptionKey(encoded), WithExpectedName("rejecting"),
		WithDialContext(func(context.Context, string, string) (net.Conn, error) { return front, nil }),
	)
	if !errors.Is(err, ErrNoiseHandshake) || !errors.Is(err, ErrNoiseKeyRejected) {
		t.Fatalf("got %v, want handshake and key-rejected categories", err)
	}
	if !strings.Contains(err.Error(), address) || !strings.Contains(err.Error(), "redacted") {
		t.Fatalf("error omits target or explicit redaction: %v", err)
	}
	if strings.Contains(err.Error(), encoded) || strings.ContainsAny(err.Error(), "\r\n\t") {
		t.Fatalf("error leaks key or control characters: %q", err)
	}
	if peerErr := <-peerDone; peerErr != nil {
		t.Fatalf("rejecting peer: %v", peerErr)
	}
}

func TestDialRedactsCRLFWrappedPeerKeyRejection(t *testing.T) {
	front, back := net.Pipe()
	t.Cleanup(func() { _ = back.Close() })
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	wrapped := encoded[:16] + "\r\n" + encoded[16:32] + "\n" + encoded[32:]
	peerDone := make(chan error, 1)
	go func() {
		var preamble [3]byte
		if _, err := io.ReadFull(back, preamble[:]); err != nil {
			peerDone <- err
			return
		}
		if _, err := readTestNoisePacket(back); err != nil {
			peerDone <- err
			return
		}
		if err := writeTestNoisePacket(back, []byte{1, 'r', 'e', 'j', 'e', 'c', 't', 'i', 'n', 'g', 0, 0}); err != nil {
			peerDone <- err
			return
		}
		peerDone <- writeTestNoisePacket(back, append([]byte{1}, []byte("Handshake MAC failure "+wrapped)...))
	}()
	address := "rejecting-device.local:6053"
	_, err := DialWithContext(context.Background(), address, time.Second,
		WithEncryptionKey(wrapped), WithExpectedName("rejecting"),
		WithDialContext(func(context.Context, string, string) (net.Conn, error) { return front, nil }),
	)
	if !errors.Is(err, ErrNoiseHandshake) || !errors.Is(err, ErrNoiseKeyRejected) {
		t.Fatalf("got %v, want handshake and key-rejected categories", err)
	}
	if !strings.Contains(err.Error(), address) || !strings.Contains(err.Error(), "redacted") {
		t.Fatalf("error omits target or explicit redaction: %v", err)
	}
	for _, fragment := range []string{encoded[:16], encoded[16:32], encoded[32:]} {
		if strings.Contains(err.Error(), fragment) {
			t.Fatalf("error leaks wrapped key fragment %q: %q", fragment, err)
		}
	}
	if peerErr := <-peerDone; peerErr != nil {
		t.Fatalf("rejecting peer: %v", peerErr)
	}
}

func TestNoiseConfigurationErrorsRedactKeys(t *testing.T) {
	canonical := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	lastIndex := len(canonical) - 2
	canonicalValue := strings.IndexByte(alphabet, canonical[lastIndex])
	nonCanonical := canonical[:lastIndex] + string(alphabet[canonicalValue+1]) + canonical[lastIndex+1:]
	if decoded, err := base64.StdEncoding.DecodeString(nonCanonical); err != nil || len(decoded) != 32 {
		t.Fatalf("test setup did not produce accepted non-canonical base64: len=%d err=%v", len(decoded), err)
	}
	tests := []struct {
		name string
		key  string
	}{
		{name: "invalid base64", key: "synthetic-invalid-key-value"},
		{name: "wrong decoded length", key: base64.StdEncoding.EncodeToString([]byte("synthetic-short-key"))},
		{name: "non-canonical pad bits", key: nonCanonical[:16] + "\r\n" + nonCanonical[16:]},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			front, back := net.Pipe()
			defer back.Close()
			_, err := DialWithContext(context.Background(), "redaction-test.local:6053", time.Second,
				WithEncryptionKey(test.key),
				WithDialContext(func(context.Context, string, string) (net.Conn, error) { return front, nil }),
			)
			if !errors.Is(err, ErrNoiseKey) {
				t.Fatalf("got %v, want invalid-key category", err)
			}
			if strings.Contains(err.Error(), test.key) {
				t.Fatalf("configuration error leaked key: %q", err)
			}
			for _, fragment := range strings.FieldsFunc(test.key, func(r rune) bool { return r == '\r' || r == '\n' }) {
				if len(fragment) >= 12 && strings.Contains(err.Error(), fragment) {
					t.Fatalf("configuration error leaked key fragment %q: %q", fragment, err)
				}
			}
		})
	}
}

func TestHelloFailureStagesPreserveCauses(t *testing.T) {
	writeCause := errors.New("synthetic write failure")
	writeFramer := newScriptedFramer()
	writeFramer.writeErr = writeCause
	writeErr := bareClient(writeFramer).hello("test", "")
	if !errors.Is(writeErr, ErrHello) || !errors.Is(writeErr, writeCause) || !strings.Contains(writeErr.Error(), "send request") {
		t.Fatalf("write stage: %v", writeErr)
	}

	readCause := errors.New("synthetic read failure")
	readErr := bareClient(newScriptedFramer(scriptedFrame{err: readCause})).hello("test", "")
	if !errors.Is(readErr, ErrHello) || !errors.Is(readErr, readCause) || !strings.Contains(readErr.Error(), "read response") {
		t.Fatalf("read stage: %v", readErr)
	}

	idErr := bareClient(newScriptedFramer(scriptedFrame{id: 99})).hello("test", "")
	if !errors.Is(idErr, ErrHello) || !strings.Contains(idErr.Error(), "message ID 99") {
		t.Fatalf("message-ID stage: %v", idErr)
	}

	decodeErr := bareClient(newScriptedFramer(scriptedFrame{id: 2, payload: []byte{0xff}})).hello("test", "")
	if !errors.Is(decodeErr, ErrHello) || !errors.Is(decodeErr, wire.ErrMalformedFrame) || !strings.Contains(decodeErr.Error(), "decode response") {
		t.Fatalf("decode stage: %v", decodeErr)
	}

	payload, err := proto.Marshal(&pb.HelloResponse{ApiVersionMajor: 2})
	if err != nil {
		t.Fatal(err)
	}
	versionErr := bareClient(newScriptedFramer(scriptedFrame{id: 2, payload: payload})).hello("test", "")
	if !errors.Is(versionErr, ErrHello) || !strings.Contains(versionErr.Error(), "unsupported API major version 2") {
		t.Fatalf("version stage: %v", versionErr)
	}
}

func TestHelloExpectedNameMismatch(t *testing.T) {
	payload, err := proto.Marshal(&pb.HelloResponse{ApiVersionMajor: 1, Name: "actual-device"})
	if err != nil {
		t.Fatal(err)
	}
	err = bareClient(newScriptedFramer(scriptedFrame{id: 2, payload: payload})).hello("test", "expected-device")
	if !errors.Is(err, ErrHello) || !errors.Is(err, ErrPeerName) {
		t.Fatalf("got %v, want hello and peer-name categories", err)
	}
}

func TestDialBoundsPlaintextHelloByTimeout(t *testing.T) {
	front, back := net.Pipe()
	t.Cleanup(func() { _ = back.Close() })
	go func() {
		framer := wire.NewPlainFramer(back, wire.DefaultMaxFrameSize)
		_, _, _ = framer.ReadFrame()
	}()
	started := time.Now()
	_, err := DialWithContext(context.Background(), "silent-plaintext:6053", 30*time.Millisecond,
		WithInsecurePlaintext(),
		WithDialContext(func(context.Context, string, string) (net.Conn, error) { return front, nil }),
	)
	if !errors.Is(err, ErrHello) {
		t.Fatalf("got %v, want hello category", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("hello exceeded bound: %s", elapsed)
	}
}

func TestDialBoundsNoiseHelloByTimeout(t *testing.T) {
	front, back := net.Pipe()
	t.Cleanup(func() { _ = back.Close() })
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	go func() {
		framer, err := wire.NewNoiseServerFramer(back, key, "silent-noise", time.Second, wire.DefaultMaxFrameSize)
		if err == nil {
			_, _, _ = framer.ReadFrame()
		}
	}()
	_, err := DialWithContext(context.Background(), "silent-noise:6053", 50*time.Millisecond,
		WithEncryptionKey(encoded), WithExpectedName("silent-noise"),
		WithDialContext(func(context.Context, string, string) (net.Conn, error) { return front, nil }),
	)
	if !errors.Is(err, ErrHello) {
		t.Fatalf("got %v, want hello category", err)
	}
}

func TestDialBoundsNoiseHandshakeByTimeout(t *testing.T) {
	front, back := net.Pipe()
	t.Cleanup(func() { _ = back.Close() })
	key := bytes.Repeat([]byte{0x31}, 32)
	peerReady := make(chan struct{})
	go func() {
		var preamble [3]byte
		_, _ = io.ReadFull(back, preamble[:])
		_, _ = readTestNoisePacket(back)
		close(peerReady)
	}()
	started := time.Now()
	_, err := DialWithContext(context.Background(), "silent-noise-handshake:6053", 30*time.Millisecond,
		WithEncryptionKey(base64.StdEncoding.EncodeToString(key)),
		WithDialContext(func(context.Context, string, string) (net.Conn, error) { return front, nil }),
	)
	if !errors.Is(err, ErrNoiseHandshake) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want Noise handshake and deadline causes", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("Noise handshake exceeded bound: %s", elapsed)
	}
	select {
	case <-peerReady:
	case <-time.After(time.Second):
		t.Fatal("peer did not receive handshake request")
	}
}

func TestDialNoiseHandshakePreservesContextCancellation(t *testing.T) {
	front, back := net.Pipe()
	t.Cleanup(func() { _ = back.Close() })
	key := bytes.Repeat([]byte{0x32}, 32)
	requestRead := make(chan struct{})
	go func() {
		var preamble [3]byte
		_, _ = io.ReadFull(back, preamble[:])
		_, _ = readTestNoisePacket(back)
		close(requestRead)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := DialWithContext(ctx, "canceled-noise-handshake:6053", time.Second,
			WithEncryptionKey(base64.StdEncoding.EncodeToString(key)),
			WithDialContext(func(context.Context, string, string) (net.Conn, error) { return front, nil }),
		)
		result <- err
	}()
	select {
	case <-requestRead:
	case <-time.After(time.Second):
		t.Fatal("peer did not receive handshake request")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, ErrNoiseHandshake) || !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want Noise handshake and cancellation causes", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled Noise handshake did not return")
	}
}

func TestDialHelloPreservesContextCancellation(t *testing.T) {
	front, back := net.Pipe()
	t.Cleanup(func() { _ = back.Close() })
	helloRead := make(chan struct{})
	go func() {
		framer := wire.NewPlainFramer(back, wire.DefaultMaxFrameSize)
		_, _, _ = framer.ReadFrame()
		close(helloRead)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := DialWithContext(ctx, "canceled-hello:6053", time.Second,
			WithInsecurePlaintext(),
			WithDialContext(func(context.Context, string, string) (net.Conn, error) { return front, nil }),
		)
		result <- err
	}()
	select {
	case <-helloRead:
	case <-time.After(time.Second):
		t.Fatal("peer did not receive hello")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, ErrHello) || !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want hello and cancellation causes", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled hello did not return")
	}
}

func TestDialTimeoutBoundsInjectedDialer(t *testing.T) {
	started := time.Now()
	_, err := DialWithContext(context.Background(), "silent-dial:6053", 30*time.Millisecond,
		WithInsecurePlaintext(),
		WithDialContext(func(ctx context.Context, _, _ string) (net.Conn, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}),
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want dial deadline", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("dial exceeded bound: %s", elapsed)
	}
}

func TestSubscribeLogsDoesNotRequestConfigDump(t *testing.T) {
	framer := newScriptedFramer()
	client := bareClient(framer)
	if _, err := client.SubscribeLogs(pb.LogLevel_LOG_LEVEL_INFO, nil); err != nil {
		t.Fatal(err)
	}
	framer.mutex.Lock()
	id, payload := framer.writeID, append([]byte(nil), framer.writePayload...)
	framer.mutex.Unlock()
	message, err := wire.Decode(id, payload)
	if err != nil {
		t.Fatal(err)
	}
	request, ok := message.(*pb.SubscribeLogsRequest)
	if !ok || request.DumpConfig {
		t.Fatalf("unexpected log request: %#v", message)
	}
}

func TestPingHonorsContext(t *testing.T) {
	client := bareClient(newScriptedFramer())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := client.Ping(ctx)
	if !errors.Is(err, ErrPing) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want ping timeout", err)
	}
	select {
	case <-client.Done():
	case <-time.After(time.Second):
		t.Fatal("ambiguous timed-out ping did not close the connection")
	}
}

func TestSpuriousEntityCompletionIsHarmless(t *testing.T) {
	client := bareClient(newScriptedFramer())
	client.handleList(&pb.ListEntitiesDoneResponse{})
	client.handleList(&pb.ListEntitiesDoneResponse{})
}

func readTestNoisePacket(reader io.Reader) ([]byte, error) {
	var header [3]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	length := int(binary.BigEndian.Uint16(header[1:]))
	payload := make([]byte, length)
	_, err := io.ReadFull(reader, payload)
	return payload, err
}

func writeTestNoisePacket(writer io.Writer, payload []byte) error {
	header := []byte{1, byte(len(payload) >> 8), byte(len(payload))}
	if _, err := writer.Write(header); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

func TestReadLoopRecordsFailureReason(t *testing.T) {
	cause := errors.New("synthetic frame read failure")
	client := bareClient(newScriptedFramer(scriptedFrame{err: cause}))
	client.connected.Store(true)
	go client.readLoop(context.Background())
	select {
	case <-client.Done():
	case <-time.After(time.Second):
		t.Fatal("read loop did not terminate")
	}
	if reason := client.CloseReason(); !errors.Is(reason, cause) || !strings.Contains(reason.Error(), "read ESPHome frame") {
		t.Fatalf("unexpected close reason: %v", reason)
	}
}

func TestReadLoopRecordsContextAndPeerDisconnect(t *testing.T) {
	t.Run("context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := bareClient(newScriptedFramer())
		client.connected.Store(true)
		go client.readLoop(ctx)
		cancel()
		select {
		case <-client.Done():
		case <-time.After(time.Second):
			t.Fatal("context cancellation did not close client")
		}
		if !errors.Is(client.CloseReason(), context.Canceled) {
			t.Fatalf("unexpected context close reason: %v", client.CloseReason())
		}
	})

	t.Run("peer disconnect", func(t *testing.T) {
		message := &pb.DisconnectRequest{}
		id, err := wire.MessageID(message)
		if err != nil {
			t.Fatal(err)
		}
		payload, err := proto.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		client := bareClient(newScriptedFramer(scriptedFrame{id: id, payload: payload}))
		client.connected.Store(true)
		go client.readLoop(context.Background())
		select {
		case <-client.Done():
		case <-time.After(time.Second):
			t.Fatal("peer disconnect did not close client")
		}
		if !errors.Is(client.CloseReason(), ErrPeerDisconnected) {
			t.Fatalf("unexpected peer close reason: %v", client.CloseReason())
		}
	})
}

func TestIntentionalCloseHasNoFailureReason(t *testing.T) {
	client := bareClient(newScriptedFramer())
	client.connected.Store(true)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if reason := client.CloseReason(); reason != nil {
		t.Fatalf("intentional close has failure reason: %v", reason)
	}
}

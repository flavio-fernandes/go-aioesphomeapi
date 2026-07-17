package aioesphomeapi

import (
	"context"
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
	mutex    sync.Mutex
	frames   []scriptedFrame
	writeErr error
	closed   chan struct{}
	once     sync.Once
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

func (f *scriptedFramer) WriteFrame(uint32, []byte) error { return f.writeErr }
func (f *scriptedFramer) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

func bareClient(f wire.Framer) *Client {
	return &Client{
		framer:   f,
		entities: newEntityRegistry(),
		done:     make(chan struct{}),
		handlers: make(map[uint32]map[uint64]callback),
		events:   make(chan proto.Message, 4),
	}
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
	_, err := DialWithContext(context.Background(), address, time.Second,
		WithEncryptionKey("kJ7hc0lJ0Zw9N3DcJzXn1kJ7hc0lJ0Zw9N3DcJzXn1k="),
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
}

func TestHelloFailureStagesPreserveCauses(t *testing.T) {
	writeCause := errors.New("synthetic write failure")
	writeFramer := newScriptedFramer()
	writeFramer.writeErr = writeCause
	writeErr := bareClient(writeFramer).hello("test")
	if !errors.Is(writeErr, ErrHello) || !errors.Is(writeErr, writeCause) || !strings.Contains(writeErr.Error(), "send request") {
		t.Fatalf("write stage: %v", writeErr)
	}

	readCause := errors.New("synthetic read failure")
	readErr := bareClient(newScriptedFramer(scriptedFrame{err: readCause})).hello("test")
	if !errors.Is(readErr, ErrHello) || !errors.Is(readErr, readCause) || !strings.Contains(readErr.Error(), "read response") {
		t.Fatalf("read stage: %v", readErr)
	}

	idErr := bareClient(newScriptedFramer(scriptedFrame{id: 99})).hello("test")
	if !errors.Is(idErr, ErrHello) || !strings.Contains(idErr.Error(), "message ID 99") {
		t.Fatalf("message-ID stage: %v", idErr)
	}

	decodeErr := bareClient(newScriptedFramer(scriptedFrame{id: 2, payload: []byte{0xff}})).hello("test")
	if !errors.Is(decodeErr, ErrHello) || !errors.Is(decodeErr, wire.ErrMalformedFrame) || !strings.Contains(decodeErr.Error(), "decode response") {
		t.Fatalf("decode stage: %v", decodeErr)
	}

	payload, err := proto.Marshal(&pb.HelloResponse{ApiVersionMajor: 2})
	if err != nil {
		t.Fatal(err)
	}
	versionErr := bareClient(newScriptedFramer(scriptedFrame{id: 2, payload: payload})).hello("test")
	if !errors.Is(versionErr, ErrHello) || !strings.Contains(versionErr.Error(), "unsupported API major version 2") {
		t.Fatalf("version stage: %v", versionErr)
	}
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

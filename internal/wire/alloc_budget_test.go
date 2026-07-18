package wire

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

// replayConn serves the same byte stream over and over so a framer can read an
// unbounded sequence of identical, well-formed frames without touching a
// network.
type replayConn struct {
	writeOnlyConn
	data   []byte
	offset int
}

func (c *replayConn) Read(p []byte) (int, error) {
	if c.offset == len(c.data) {
		c.offset = 0
	}
	n := copy(p, c.data[c.offset:])
	c.offset += n
	return n, nil
}

func buildPlainFrame(messageType uint32, payload []byte) []byte {
	frame := []byte{0}
	frame = binary.AppendUvarint(frame, uint64(len(payload)))
	frame = binary.AppendUvarint(frame, uint64(messageType))
	return append(frame, payload...)
}

// The allocation ceilings below are deliberately several times the measured
// steady-state cost so that compiler and platform variation never trips them;
// they exist to catch an accidental change to per-frame allocation behavior
// (for example, quadratic buffering) on the untrusted read path.

func TestPlainFramerReadFrameAllocationBudget(t *testing.T) {
	payload := bytes.Repeat([]byte{0xAB}, 64)
	conn := &replayConn{data: buildPlainFrame(42, payload)}
	framer := NewPlainFramer(conn, 1024)
	average := testing.AllocsPerRun(200, func() {
		id, got, err := framer.ReadFrame()
		if err != nil || id != 42 || len(got) != len(payload) {
			t.Fatalf("ReadFrame = %d, %d bytes, %v", id, len(got), err)
		}
	})
	t.Logf("measured %.1f allocations per frame", average)
	if average > 8 {
		t.Fatalf("ReadFrame allocations per frame = %.1f, budget 8", average)
	}
}

func TestPlainFramerWriteFrameAllocationBudget(t *testing.T) {
	payload := bytes.Repeat([]byte{0xCD}, 64)
	framer := NewPlainFramer(&writeOnlyConn{Writer: io.Discard}, 1024)
	average := testing.AllocsPerRun(200, func() {
		if err := framer.WriteFrame(42, payload); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	})
	t.Logf("measured %.1f allocations per frame", average)
	if average > 4 {
		t.Fatalf("WriteFrame allocations per frame = %.1f, budget 4", average)
	}
}

func TestDecodeAllocationBudget(t *testing.T) {
	message := &pb.DeviceInfoResponse{Name: "budget-device", MacAddress: "00:00:00:00:00:00"}
	id, err := MessageID(message)
	if err != nil {
		t.Fatalf("message id: %v", err)
	}
	payload, err := proto.Marshal(message)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	average := testing.AllocsPerRun(200, func() {
		decoded, err := Decode(id, payload)
		if err != nil || decoded.(*pb.DeviceInfoResponse).GetName() != "budget-device" {
			t.Fatalf("Decode = %v, %v", decoded, err)
		}
	})
	t.Logf("measured %.1f allocations per message", average)
	if average > 24 {
		t.Fatalf("Decode allocations per message = %.1f, budget 24", average)
	}
}

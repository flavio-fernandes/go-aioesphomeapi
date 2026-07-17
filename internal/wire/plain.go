package wire

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

type plainFramer struct {
	conn     net.Conn
	reader   *bufio.Reader
	maxFrame uint64
	writeMu  sync.Mutex
}

// NewPlainFramer wraps a connection using the explicitly insecure plaintext framing.
func NewPlainFramer(conn net.Conn, maxFrame int) Framer {
	if maxFrame <= 0 {
		maxFrame = DefaultMaxFrameSize
	}
	return &plainFramer{
		conn:     conn,
		reader:   bufio.NewReaderSize(conn, 4096),
		maxFrame: uint64(maxFrame),
	}
}

func (f *plainFramer) ReadFrame() (uint32, []byte, error) {
	preamble, err := f.reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	if preamble != 0 {
		return 0, nil, fmt.Errorf("%w: plaintext preamble", ErrMalformedFrame)
	}
	size, err := binary.ReadUvarint(f.reader)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: payload length", ErrMalformedFrame)
	}
	if size > f.maxFrame {
		return 0, nil, ErrFrameTooLarge
	}
	messageType, err := binary.ReadUvarint(f.reader)
	if err != nil || messageType > uint64(^uint32(0)) {
		return 0, nil, fmt.Errorf("%w: message type", ErrMalformedFrame)
	}
	payload := make([]byte, int(size))
	if _, err := io.ReadFull(f.reader, payload); err != nil {
		return 0, nil, fmt.Errorf("%w: truncated payload", ErrMalformedFrame)
	}
	return uint32(messageType), payload, nil
}

func (f *plainFramer) WriteFrame(messageType uint32, payload []byte) error {
	if uint64(len(payload)) > f.maxFrame {
		return ErrFrameTooLarge
	}
	buffer := make([]byte, 1+binary.MaxVarintLen64+binary.MaxVarintLen32+len(payload))
	buffer[0] = 0
	offset := 1
	offset += binary.PutUvarint(buffer[offset:], uint64(len(payload)))
	offset += binary.PutUvarint(buffer[offset:], uint64(messageType))
	copy(buffer[offset:], payload)
	buffer = buffer[:offset+len(payload)]

	f.writeMu.Lock()
	defer f.writeMu.Unlock()
	return writeFull(f.conn, buffer)
}

func (f *plainFramer) Close() error {
	return f.conn.Close()
}

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrUnexpectedEOF
		}
		data = data[written:]
	}
	return nil
}

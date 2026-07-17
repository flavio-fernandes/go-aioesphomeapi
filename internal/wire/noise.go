package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/flynn/noise"
)

var noisePrologue = []byte("NoiseAPIInit\x00\x00")

type noiseFramer struct {
	conn     net.Conn
	encrypt  *noise.CipherState
	decrypt  *noise.CipherState
	maxFrame int
	writeMu  sync.Mutex
}

func handshakeState(initiator bool, psk []byte) (*noise.HandshakeState, error) {
	suite := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256)
	state, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:           suite,
		Pattern:               noise.HandshakeNN,
		Initiator:             initiator,
		Prologue:              noisePrologue,
		PresharedKey:          psk,
		PresharedKeyPlacement: 0,
	})
	if err != nil {
		return nil, ErrNoiseHandshake
	}
	return state, nil
}

// NewNoiseClientFramer performs the ESPHome Noise client handshake.
func NewNoiseClientFramer(conn net.Conn, psk []byte, expectedName string, timeout time.Duration, maxFrame int) (Framer, error) {
	if len(psk) != 32 {
		return nil, ErrNoiseKey
	}
	if maxFrame <= 0 || maxFrame > maxNoisePacketSize-20 {
		maxFrame = DefaultMaxFrameSize - 20
	}
	if timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return nil, ErrNoiseHandshake
		}
		defer conn.SetDeadline(time.Time{})
	}
	state, err := handshakeState(true, psk)
	if err != nil {
		return nil, err
	}
	if err := writeFull(conn, []byte{1, 0, 0}); err != nil {
		return nil, ErrNoiseHandshake
	}
	first, _, _, err := state.WriteMessage(nil, nil)
	if err != nil {
		return nil, ErrNoiseHandshake
	}
	if err := writeNoisePacket(conn, append([]byte{0}, first...)); err != nil {
		return nil, ErrNoiseHandshake
	}
	serverHello, err := readNoisePacket(conn)
	if err != nil || len(serverHello) < 2 || serverHello[0] != 1 {
		return nil, ErrNoiseHandshake
	}
	nameEnd := bytes.IndexByte(serverHello[1:], 0)
	if nameEnd < 0 {
		return nil, ErrNoiseHandshake
	}
	serverName := string(serverHello[1 : nameEnd+1])
	if expectedName != "" && serverName != expectedName {
		return nil, ErrNoiseName
	}
	response, err := readNoisePacket(conn)
	if err != nil || len(response) < 2 || response[0] != 0 {
		return nil, ErrNoiseHandshake
	}
	_, encrypt, decrypt, err := state.ReadMessage(nil, response[1:])
	if err != nil || encrypt == nil || decrypt == nil {
		return nil, ErrNoiseHandshake
	}
	return &noiseFramer{conn: conn, encrypt: encrypt, decrypt: decrypt, maxFrame: maxFrame}, nil
}

// NewNoiseServerFramer performs the device side of the ESPHome Noise handshake.
// It exists for the deterministic simulator and protocol conformance tests.
func NewNoiseServerFramer(conn net.Conn, psk []byte, serverName string, timeout time.Duration, maxFrame int) (Framer, error) {
	if len(psk) != 32 {
		return nil, ErrNoiseKey
	}
	if maxFrame <= 0 || maxFrame > maxNoisePacketSize-20 {
		maxFrame = DefaultMaxFrameSize - 20
	}
	if timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return nil, ErrNoiseHandshake
		}
		defer conn.SetDeadline(time.Time{})
	}
	var prefix [3]byte
	if _, err := io.ReadFull(conn, prefix[:]); err != nil || prefix != [3]byte{1, 0, 0} {
		return nil, ErrNoiseHandshake
	}
	state, err := handshakeState(false, psk)
	if err != nil {
		return nil, err
	}
	request, err := readNoisePacket(conn)
	if err != nil || len(request) < 2 || request[0] != 0 {
		return nil, ErrNoiseHandshake
	}
	if _, _, _, err := state.ReadMessage(nil, request[1:]); err != nil {
		return nil, ErrNoiseHandshake
	}
	hello := make([]byte, 0, len(serverName)+3)
	hello = append(hello, 1)
	hello = append(hello, serverName...)
	hello = append(hello, 0, 0)
	if err := writeNoisePacket(conn, hello); err != nil {
		return nil, ErrNoiseHandshake
	}
	response, decrypt, encrypt, err := state.WriteMessage(nil, nil)
	if err != nil || encrypt == nil || decrypt == nil {
		return nil, ErrNoiseHandshake
	}
	if err := writeNoisePacket(conn, append([]byte{0}, response...)); err != nil {
		return nil, ErrNoiseHandshake
	}
	return &noiseFramer{conn: conn, encrypt: encrypt, decrypt: decrypt, maxFrame: maxFrame}, nil
}

func (f *noiseFramer) WriteFrame(messageType uint32, payload []byte) error {
	if len(payload) > f.maxFrame || messageType > uint32(^uint16(0)) {
		return ErrFrameTooLarge
	}
	plain := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint16(plain[0:2], uint16(messageType))
	binary.BigEndian.PutUint16(plain[2:4], uint16(len(payload)))
	copy(plain[4:], payload)

	f.writeMu.Lock()
	defer f.writeMu.Unlock()
	ciphertext, err := f.encrypt.Encrypt(nil, nil, plain)
	if err != nil {
		return ErrClosed
	}
	if len(ciphertext) > maxNoisePacketSize {
		return ErrFrameTooLarge
	}
	return writeNoisePacket(f.conn, ciphertext)
}

func (f *noiseFramer) ReadFrame() (uint32, []byte, error) {
	ciphertext, err := readNoisePacket(f.conn)
	if err != nil {
		return 0, nil, err
	}
	plain, err := f.decrypt.Decrypt(nil, nil, ciphertext)
	if err != nil {
		return 0, nil, ErrMalformedFrame
	}
	if len(plain) < 4 {
		return 0, nil, ErrMalformedFrame
	}
	messageType := binary.BigEndian.Uint16(plain[:2])
	declared := int(binary.BigEndian.Uint16(plain[2:4]))
	if declared != len(plain)-4 || declared > f.maxFrame {
		return 0, nil, ErrMalformedFrame
	}
	payload := append([]byte(nil), plain[4:]...)
	return uint32(messageType), payload, nil
}

func (f *noiseFramer) Close() error {
	return f.conn.Close()
}

func writeNoisePacket(writer io.Writer, payload []byte) error {
	if len(payload) > maxNoisePacketSize {
		return ErrFrameTooLarge
	}
	header := []byte{1, byte(len(payload) >> 8), byte(len(payload))}
	if err := writeFull(writer, header); err != nil {
		return err
	}
	return writeFull(writer, payload)
}

func readNoisePacket(reader io.Reader) ([]byte, error) {
	var header [3]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	if header[0] != 1 {
		return nil, fmt.Errorf("%w: Noise preamble", ErrMalformedFrame)
	}
	length := int(binary.BigEndian.Uint16(header[1:]))
	if length == 0 {
		return nil, ErrMalformedFrame
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, ErrMalformedFrame
	}
	return payload, nil
}

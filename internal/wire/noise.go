package wire

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/flynn/noise"
)

var noisePrologue = []byte("NoiseAPIInit\x00\x00")

const maxNoiseRejectionReason = 96

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
		return nil, fmt.Errorf("%w: initialize state: %w", ErrNoiseHandshake, err)
	}
	return state, nil
}

// NewNoiseClientFramer performs the ESPHome Noise client handshake.
func NewNoiseClientFramer(conn net.Conn, psk []byte, expectedName string, timeout time.Duration, maxFrame int) (Framer, error) {
	if len(psk) != 32 {
		return nil, ErrNoiseKey
	}
	if maxFrame <= 0 || maxFrame > maxNoisePacketSize-20 {
		maxFrame = maxNoisePacketSize - 20
	}
	if timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return nil, fmt.Errorf("%w: set client deadline: %w", ErrNoiseHandshake, err)
		}
		defer conn.SetDeadline(time.Time{})
	}
	state, err := handshakeState(true, psk)
	if err != nil {
		return nil, err
	}
	if err := writeFull(conn, []byte{1, 0, 0}); err != nil {
		return nil, fmt.Errorf("%w: write client preamble: %w", ErrNoiseHandshake, err)
	}
	first, _, _, err := state.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: create client handshake message: %w", ErrNoiseHandshake, err)
	}
	if err := writeNoisePacket(conn, append([]byte{0}, first...)); err != nil {
		return nil, fmt.Errorf("%w: write client handshake message: %w", ErrNoiseHandshake, err)
	}
	serverHello, err := readNoisePacket(conn)
	if err != nil {
		return nil, fmt.Errorf("%w: read server name: %w", ErrNoiseHandshake, err)
	}
	if len(serverHello) < 2 || serverHello[0] != 1 {
		return nil, fmt.Errorf("%w: invalid server-name packet", ErrNoiseHandshake)
	}
	nameEnd := bytes.IndexByte(serverHello[1:], 0)
	if nameEnd < 0 {
		return nil, fmt.Errorf("%w: unterminated server name", ErrNoiseHandshake)
	}
	serverName := string(serverHello[1 : nameEnd+1])
	if expectedName != "" && serverName != expectedName {
		return nil, ErrPeerName
	}
	response, err := readNoisePacket(conn)
	if err != nil {
		return nil, fmt.Errorf("%w: read server handshake message: %w", ErrNoiseHandshake, err)
	}
	if len(response) >= 1 && response[0] == 1 {
		return nil, fmt.Errorf("%w: %w: %s", ErrNoiseHandshake, ErrNoiseKeyRejected, sanitizeNoiseRejection(response[1:], psk))
	}
	if len(response) < 2 || response[0] != 0 {
		return nil, fmt.Errorf("%w: invalid server handshake packet", ErrNoiseHandshake)
	}
	_, encrypt, decrypt, err := state.ReadMessage(nil, response[1:])
	if err != nil {
		return nil, fmt.Errorf("%w: authenticate server handshake: %w", ErrNoiseHandshake, err)
	}
	if encrypt == nil || decrypt == nil {
		return nil, fmt.Errorf("%w: incomplete client cipher state", ErrNoiseHandshake)
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
		maxFrame = maxNoisePacketSize - 20
	}
	if timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return nil, fmt.Errorf("%w: set server deadline: %w", ErrNoiseHandshake, err)
		}
		defer conn.SetDeadline(time.Time{})
	}
	var prefix [3]byte
	if _, err := io.ReadFull(conn, prefix[:]); err != nil {
		return nil, fmt.Errorf("%w: read client preamble: %w", ErrNoiseHandshake, err)
	}
	if prefix != [3]byte{1, 0, 0} {
		return nil, fmt.Errorf("%w: invalid client preamble", ErrNoiseHandshake)
	}
	state, err := handshakeState(false, psk)
	if err != nil {
		return nil, err
	}
	request, err := readNoisePacket(conn)
	if err != nil {
		return nil, fmt.Errorf("%w: read client handshake message: %w", ErrNoiseHandshake, err)
	}
	if len(request) < 2 || request[0] != 0 {
		return nil, fmt.Errorf("%w: invalid client handshake packet", ErrNoiseHandshake)
	}
	if _, _, _, err := state.ReadMessage(nil, request[1:]); err != nil {
		return nil, fmt.Errorf("%w: authenticate client handshake: %w", ErrNoiseHandshake, err)
	}
	hello := make([]byte, 0, len(serverName)+3)
	hello = append(hello, 1)
	hello = append(hello, serverName...)
	hello = append(hello, 0, 0)
	if err := writeNoisePacket(conn, hello); err != nil {
		return nil, fmt.Errorf("%w: write server name: %w", ErrNoiseHandshake, err)
	}
	response, decrypt, encrypt, err := state.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: create server handshake message: %w", ErrNoiseHandshake, err)
	}
	if encrypt == nil || decrypt == nil {
		return nil, fmt.Errorf("%w: incomplete server cipher state", ErrNoiseHandshake)
	}
	if err := writeNoisePacket(conn, append([]byte{0}, response...)); err != nil {
		return nil, fmt.Errorf("%w: write server handshake message: %w", ErrNoiseHandshake, err)
	}
	return &noiseFramer{conn: conn, encrypt: encrypt, decrypt: decrypt, maxFrame: maxFrame}, nil
}

func (f *noiseFramer) WriteFrame(messageType uint32, payload []byte) error {
	if messageType > uint32(^uint16(0)) {
		return ErrMessageType
	}
	if len(payload) > f.maxFrame {
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
		return fmt.Errorf("%w: encrypt frame: %w", ErrClosed, err)
	}
	if len(ciphertext) > maxNoisePacketSize {
		return ErrFrameTooLarge
	}
	return writeNoisePacket(f.conn, ciphertext)
}

func sanitizeNoiseRejection(raw, psk []byte) string {
	// The rejection text is controlled by the peer. Treat an echo of either
	// accepted key representation as entirely sensitive, including when a key
	// crosses the printable-length boundary below.
	if len(psk) > 0 && (bytes.Contains(raw, psk) || bytes.Contains(raw, []byte(base64.StdEncoding.EncodeToString(psk)))) {
		return "peer rejection reason redacted"
	}
	if len(raw) > maxNoiseRejectionReason {
		raw = raw[:maxNoiseRejectionReason]
	}
	clean := make([]byte, len(raw))
	for i, value := range raw {
		if value >= 0x20 && value <= 0x7e {
			clean[i] = value
		} else {
			clean[i] = '?'
		}
	}
	reason := strings.TrimSpace(string(clean))
	if reason == "" {
		return "unspecified rejection"
	}
	return reason
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
		return nil, fmt.Errorf("%w: Noise payload: %w", ErrMalformedFrame, err)
	}
	return payload, nil
}

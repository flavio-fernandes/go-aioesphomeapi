// Package wire implements bounded ESPHome Native API framing.
package wire

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	// DefaultMaxFrameSize bounds allocations from an untrusted peer.
	DefaultMaxFrameSize = 64 * 1024
	maxNoisePacketSize  = math.MaxUint16
)

var (
	ErrClosed          = errors.New("native API connection closed")
	ErrMalformedFrame  = errors.New("malformed native API frame")
	ErrFrameTooLarge   = errors.New("native API frame exceeds limit")
	ErrUnknownMessage  = errors.New("unknown native API message")
	ErrNoiseHandshake  = errors.New("Noise handshake failed")
	ErrNoiseName       = errors.New("Noise peer name mismatch")
	ErrNoiseKey        = errors.New("invalid Noise key")
	ErrTransportPolicy = errors.New("secure transport configuration required")
)

// Framer exchanges typed protobuf payloads over one connection.
type Framer interface {
	ReadFrame() (uint32, []byte, error)
	WriteFrame(uint32, []byte) error
	Close() error
}

var (
	registryOnce sync.Once
	messageByID  map[uint32]protoreflect.MessageType
	idByName     map[protoreflect.FullName]uint32
)

func buildRegistry() {
	messageByID = make(map[uint32]protoreflect.MessageType)
	idByName = make(map[protoreflect.FullName]uint32)
	messages := pb.File_api_proto.Messages()
	for i := 0; i < messages.Len(); i++ {
		descriptor := messages.Get(i)
		options, ok := descriptor.Options().(*descriptorpb.MessageOptions)
		if !ok || !proto.HasExtension(options, pb.E_Id) {
			continue
		}
		id, ok := proto.GetExtension(options, pb.E_Id).(uint32)
		if !ok || id == 0 {
			continue
		}
		messageType, err := protoregistry.GlobalTypes.FindMessageByName(descriptor.FullName())
		if err != nil {
			continue
		}
		messageByID[id] = messageType
		idByName[descriptor.FullName()] = id
	}
}

// NewMessage returns a new generated message for a Native API message ID.
func NewMessage(id uint32) (proto.Message, error) {
	registryOnce.Do(buildRegistry)
	messageType, exists := messageByID[id]
	if !exists {
		return nil, fmt.Errorf("%w: id %d", ErrUnknownMessage, id)
	}
	return messageType.New().Interface(), nil
}

// MessageID returns the Native API message ID carried in the descriptor.
func MessageID(message proto.Message) (uint32, error) {
	if message == nil {
		return 0, fmt.Errorf("%w: nil message", ErrUnknownMessage)
	}
	registryOnce.Do(buildRegistry)
	id, exists := idByName[message.ProtoReflect().Descriptor().FullName()]
	if !exists {
		return 0, fmt.Errorf("%w: %s", ErrUnknownMessage, message.ProtoReflect().Descriptor().Name())
	}
	return id, nil
}

// Decode unmarshals one payload using the pinned generated registry.
func Decode(id uint32, payload []byte) (proto.Message, error) {
	message, err := NewMessage(id)
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(payload, message); err != nil {
		return nil, fmt.Errorf("%w: protobuf payload", ErrMalformedFrame)
	}
	return message, nil
}

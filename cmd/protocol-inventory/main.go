// Command protocol-inventory emits the pinned ESPHome message inventory.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type entry struct {
	ID        uint32 `json:"id"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Ifdef     string `json:"ifdef,omitempty"`
}

func extension[T any](options *descriptorpb.MessageOptions, extension protoreflect.ExtensionType, fallback T) T {
	if !proto.HasExtension(options, extension) {
		return fallback
	}
	value, ok := proto.GetExtension(options, extension).(T)
	if !ok {
		return fallback
	}
	return value
}

func main() {
	messages := pb.File_api_proto.Messages()
	entries := make([]entry, 0, messages.Len())
	seen := make(map[uint32]string)
	for i := 0; i < messages.Len(); i++ {
		descriptor := messages.Get(i)
		options, ok := descriptor.Options().(*descriptorpb.MessageOptions)
		if !ok {
			continue
		}
		id := extension(options, pb.E_Id, uint32(0))
		if id == 0 {
			continue
		}
		name := string(descriptor.Name())
		if previous, exists := seen[id]; exists {
			fmt.Fprintf(os.Stderr, "duplicate message id %d: %s and %s\n", id, previous, name)
			os.Exit(1)
		}
		seen[id] = name
		source := extension(options, pb.E_Source, pb.APISourceType_SOURCE_BOTH)
		entries = append(entries, entry{
			ID:        id,
			Name:      name,
			Direction: source.String(),
			Ifdef:     extension(options, pb.E_Ifdef, ""),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(entries); err != nil {
		fmt.Fprintf(os.Stderr, "encode inventory: %v\n", err)
		os.Exit(1)
	}
}

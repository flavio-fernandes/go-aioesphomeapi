package simulator

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

// ErrInvalidScenario classifies simulator scenario validation failures.
var ErrInvalidScenario = errors.New("invalid simulator scenario")

// ValidationCode is a stable, secret-safe reason for rejecting a scenario.
type ValidationCode string

const (
	ValidationInvalidType    ValidationCode = "invalid_type"
	ValidationDuplicateKey   ValidationCode = "duplicate_key"
	ValidationNegativeTime   ValidationCode = "negative_time"
	ValidationDecreasingTime ValidationCode = "decreasing_time"
	ValidationSeedRequired   ValidationCode = "seed_required"
	ValidationExpectation    ValidationCode = "impossible_expectation"
)

// ValidationError identifies one invalid scenario field without retaining or
// displaying entity names, state values, keys, credentials, or network data.
type ValidationError struct {
	Field        string
	Index        int
	RelatedIndex *int
	Code         ValidationCode
}

func (e *ValidationError) Error() string {
	if e.RelatedIndex != nil {
		return fmt.Sprintf("%v: %s[%d] conflicts with index %d (%s)", ErrInvalidScenario, e.Field, e.Index, *e.RelatedIndex, e.Code)
	}
	return fmt.Sprintf("%v: %s[%d] (%s)", ErrInvalidScenario, e.Field, e.Index, e.Code)
}

// Unwrap preserves errors.Is(err, ErrInvalidScenario).
func (e *ValidationError) Unwrap() error { return ErrInvalidScenario }

// Validate checks the scenario model that is implemented today. Future state
// timelines, randomized actions, and command expectations extend this same
// method before their behavior is enabled.
func (s Scenario) Validate() error {
	seen := make(map[string]map[uint32]int)
	for index, message := range s.Entities {
		family, key, ok := entityIdentity(message)
		if !ok {
			return validationError("entities", index, -1, ValidationInvalidType)
		}
		familyKeys := seen[family]
		if familyKeys == nil {
			familyKeys = make(map[uint32]int)
			seen[family] = familyKeys
		}
		if previous, exists := familyKeys[key]; exists {
			return validationError("entities", index, previous, ValidationDuplicateKey)
		}
		familyKeys[key] = index
	}
	for index, message := range s.States {
		if !validState(message) {
			return validationError("states", index, -1, ValidationInvalidType)
		}
	}
	for index, entry := range s.Logs {
		if entry == nil {
			return validationError("logs", index, -1, ValidationInvalidType)
		}
	}
	return nil
}

func cloneScenario(s Scenario) Scenario {
	clone := Scenario{
		Name:     s.Name,
		Seed:     s.Seed,
		Entities: make([]proto.Message, len(s.Entities)),
		States:   make([]proto.Message, len(s.States)),
		Logs:     make([]*pb.SubscribeLogsResponse, len(s.Logs)),
		Faults:   append([]Fault(nil), s.Faults...),
	}
	for index, message := range s.Entities {
		clone.Entities[index] = proto.Clone(message)
	}
	for index, message := range s.States {
		clone.States[index] = proto.Clone(message)
	}
	for index, entry := range s.Logs {
		clone.Logs[index] = proto.Clone(entry).(*pb.SubscribeLogsResponse)
	}
	return clone
}

func validationError(field string, index, related int, code ValidationCode) error {
	result := &ValidationError{Field: field, Index: index, Code: code}
	if related >= 0 {
		result.RelatedIndex = &related
	}
	return result
}

func nilMessage(message proto.Message) bool {
	if message == nil {
		return true
	}
	value := reflect.ValueOf(message)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

func entityIdentity(message proto.Message) (string, uint32, bool) {
	if nilMessage(message) {
		return "", 0, false
	}
	switch value := message.(type) {
	case *pb.ListEntitiesBinarySensorResponse:
		return "binary_sensor", value.Key, true
	case *pb.ListEntitiesSensorResponse:
		return "sensor", value.Key, true
	case *pb.ListEntitiesTextSensorResponse:
		return "text_sensor", value.Key, true
	case *pb.ListEntitiesSwitchResponse:
		return "switch", value.Key, true
	case *pb.ListEntitiesNumberResponse:
		return "number", value.Key, true
	case *pb.ListEntitiesButtonResponse:
		return "button", value.Key, true
	case *pb.ListEntitiesFanResponse:
		return "fan", value.Key, true
	case *pb.ListEntitiesLightResponse:
		return "light", value.Key, true
	default:
		return "", 0, false
	}
}

func validState(message proto.Message) bool {
	if nilMessage(message) {
		return false
	}
	switch message.(type) {
	case *pb.BinarySensorStateResponse,
		*pb.SensorStateResponse,
		*pb.TextSensorStateResponse,
		*pb.SwitchStateResponse,
		*pb.NumberStateResponse,
		*pb.FanStateResponse,
		*pb.LightStateResponse:
		return true
	default:
		return false
	}
}

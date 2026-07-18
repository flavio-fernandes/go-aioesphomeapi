package simulator

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

// ErrInvalidScenario classifies simulator scenario validation failures.
var ErrInvalidScenario = errors.New("invalid simulator scenario")

// ValidationCode is a stable, secret-safe reason for rejecting a scenario.
type ValidationCode string

const (
	ValidationInvalidType     ValidationCode = "invalid_type"
	ValidationDuplicateKey    ValidationCode = "duplicate_key"
	ValidationNegativeTime    ValidationCode = "negative_time"
	ValidationDecreasingTime  ValidationCode = "decreasing_time"
	ValidationInvalidDuration ValidationCode = "invalid_duration"
	ValidationSeedRequired    ValidationCode = "seed_required"
	ValidationExpectation     ValidationCode = "impossible_expectation"
)

// MaxCommandExpectationCount bounds one declared repeated command so invalid
// scenarios cannot request impractical counters or work.
const MaxCommandExpectationCount uint32 = 1_000_000

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

// Validate checks the scenario model that is implemented today. Future
// randomized actions, network shaping, and command expectations extend this
// same method before their behavior is enabled.
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
	if len(s.States) > 0 && len(s.InitialStates) > 0 {
		return validationError("initial_states", 0, -1, ValidationExpectation)
	}
	if err := validateInitialStates("states", s.States); err != nil {
		return err
	}
	if err := validateInitialStates("initial_states", s.InitialStates); err != nil {
		return err
	}
	previous := time.Duration(0)
	for index, event := range s.StateTimeline {
		if event.At < 0 {
			return validationError("state_timeline", index, -1, ValidationNegativeTime)
		}
		if index > 0 && event.At < previous {
			return validationError("state_timeline", index, index-1, ValidationDecreasingTime)
		}
		if !validState(event.State) {
			return validationError("state_timeline", index, -1, ValidationInvalidType)
		}
		previous = event.At
	}
	for index, entry := range s.Logs {
		if entry == nil {
			return validationError("logs", index, -1, ValidationInvalidType)
		}
	}
	for index, expectation := range s.Commands {
		if !validCommand(expectation.Command) {
			return validationError("commands", index, -1, ValidationInvalidType)
		}
		if expectation.Count == 0 || expectation.Count > MaxCommandExpectationCount {
			return validationError("commands", index, -1, ValidationExpectation)
		}
	}
	for index, fault := range s.Network {
		switch fault.Action {
		case NetworkDelayReply:
			if fault.Delay <= 0 || fault.Delay > MaxNetworkDelay {
				return validationError("network", index, -1, ValidationInvalidDuration)
			}
		case NetworkFragmentFrame, NetworkCoalesceSegments:
			if fault.Delay != 0 {
				return validationError("network", index, -1, ValidationInvalidDuration)
			}
		}
	}
	return nil
}

func validateInitialStates(field string, states []proto.Message) error {
	seen := make(map[stateIdentity]int)
	for index, message := range states {
		if !validState(message) {
			return validationError(field, index, -1, ValidationInvalidType)
		}
		identity, _ := stateIdentityOf(message)
		if previous, exists := seen[identity]; exists {
			return validationError(field, index, previous, ValidationDuplicateKey)
		}
		seen[identity] = index
	}
	return nil
}

func cloneScenario(s Scenario) Scenario {
	clone := Scenario{
		Name:          s.Name,
		Seed:          s.Seed,
		Entities:      make([]proto.Message, len(s.Entities)),
		States:        make([]proto.Message, len(s.States)),
		InitialStates: make([]proto.Message, len(s.InitialStates)),
		StateTimeline: make([]StateEvent, len(s.StateTimeline)),
		Logs:          make([]*pb.SubscribeLogsResponse, len(s.Logs)),
		Commands:      make([]CommandExpectation, len(s.Commands)),
		Faults:        append([]Fault(nil), s.Faults...),
		Network:       append([]NetworkFault(nil), s.Network...),
	}
	for index, message := range s.Entities {
		clone.Entities[index] = proto.Clone(message)
	}
	for index, message := range s.States {
		clone.States[index] = proto.Clone(message)
	}
	for index, message := range s.InitialStates {
		clone.InitialStates[index] = proto.Clone(message)
	}
	for index, event := range s.StateTimeline {
		clone.StateTimeline[index] = StateEvent{At: event.At, State: proto.Clone(event.State)}
	}
	for index, entry := range s.Logs {
		clone.Logs[index] = proto.Clone(entry).(*pb.SubscribeLogsResponse)
	}
	for index, expectation := range s.Commands {
		clone.Commands[index] = CommandExpectation{
			Command: proto.Clone(expectation.Command),
			Count:   expectation.Count,
		}
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

func validCommand(message proto.Message) bool {
	if nilMessage(message) {
		return false
	}
	switch message.(type) {
	case *pb.SwitchCommandRequest,
		*pb.NumberCommandRequest,
		*pb.ButtonCommandRequest,
		*pb.FanCommandRequest,
		*pb.LightCommandRequest:
		return true
	default:
		return false
	}
}

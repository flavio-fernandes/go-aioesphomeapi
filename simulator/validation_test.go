package simulator_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

func TestScenarioValidateCurrentModel(t *testing.T) {
	tests := []struct {
		name     string
		scenario simulator.Scenario
		code     simulator.ValidationCode
	}{
		{
			name:     "invalid entity type",
			scenario: simulator.Scenario{Entities: []proto.Message{&pb.SwitchStateResponse{Key: 1}}},
			code:     simulator.ValidationInvalidType,
		},
		{
			name:     "invalid state type",
			scenario: simulator.Scenario{States: []proto.Message{&pb.ListEntitiesSwitchResponse{Key: 1}}},
			code:     simulator.ValidationInvalidType,
		},
		{
			name: "ambiguous initial state aliases",
			scenario: simulator.Scenario{
				States:        []proto.Message{&pb.SwitchStateResponse{Key: 1}},
				InitialStates: []proto.Message{&pb.SwitchStateResponse{Key: 1}},
			},
			code: simulator.ValidationExpectation,
		},
		{
			name: "duplicate initial state key within family",
			scenario: simulator.Scenario{InitialStates: []proto.Message{
				&pb.SwitchStateResponse{Key: 4},
				&pb.SwitchStateResponse{Key: 4},
			}},
			code: simulator.ValidationDuplicateKey,
		},
		{
			name: "negative timeline time",
			scenario: simulator.Scenario{StateTimeline: []simulator.StateEvent{
				{At: -time.Nanosecond, State: &pb.SwitchStateResponse{Key: 1}},
			}},
			code: simulator.ValidationNegativeTime,
		},
		{
			name: "decreasing timeline time",
			scenario: simulator.Scenario{StateTimeline: []simulator.StateEvent{
				{At: time.Second, State: &pb.SwitchStateResponse{Key: 1}},
				{At: time.Millisecond, State: &pb.SwitchStateResponse{Key: 1}},
			}},
			code: simulator.ValidationDecreasingTime,
		},
		{
			name: "invalid timeline state",
			scenario: simulator.Scenario{StateTimeline: []simulator.StateEvent{
				{At: time.Second, State: &pb.ListEntitiesSwitchResponse{Key: 1}},
			}},
			code: simulator.ValidationInvalidType,
		},
		{
			name: "duplicate key within family",
			scenario: simulator.Scenario{Entities: []proto.Message{
				&pb.ListEntitiesSwitchResponse{Key: 7},
				&pb.ListEntitiesSwitchResponse{Key: 7},
			}},
			code: simulator.ValidationDuplicateKey,
		},
		{
			name:     "typed nil entity",
			scenario: simulator.Scenario{Entities: []proto.Message{(*pb.ListEntitiesSwitchResponse)(nil)}},
			code:     simulator.ValidationInvalidType,
		},
		{
			name:     "nil log",
			scenario: simulator.Scenario{Logs: []*pb.SubscribeLogsResponse{nil}},
			code:     simulator.ValidationInvalidType,
		},
		{
			name: "invalid command type",
			scenario: simulator.Scenario{Commands: []simulator.CommandExpectation{{
				Command: &pb.SwitchStateResponse{Key: 1}, Count: 1,
			}}},
			code: simulator.ValidationInvalidType,
		},
		{
			name: "typed nil command",
			scenario: simulator.Scenario{Commands: []simulator.CommandExpectation{{
				Command: (*pb.SwitchCommandRequest)(nil), Count: 1,
			}}},
			code: simulator.ValidationInvalidType,
		},
		{
			name: "zero command count",
			scenario: simulator.Scenario{Commands: []simulator.CommandExpectation{{
				Command: &pb.SwitchCommandRequest{Key: 1}, Count: 0,
			}}},
			code: simulator.ValidationExpectation,
		},
		{
			name: "excessive command count",
			scenario: simulator.Scenario{Commands: []simulator.CommandExpectation{{
				Command: &pb.SwitchCommandRequest{Key: 1}, Count: simulator.MaxCommandExpectationCount + 1,
			}}},
			code: simulator.ValidationExpectation,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.scenario.Validate()
			if !errors.Is(err, simulator.ErrInvalidScenario) {
				t.Fatalf("error = %v, want ErrInvalidScenario", err)
			}
			var validationErr *simulator.ValidationError
			if !errors.As(err, &validationErr) || validationErr.Code != test.code {
				t.Fatalf("error = %#v, want code %s", err, test.code)
			}
		})
	}
}

func TestNewDefensivelyCopiesValidatedScenario(t *testing.T) {
	entity := &pb.ListEntitiesSwitchResponse{Key: 17, Name: "Synthetic Switch"}
	state := &pb.SwitchStateResponse{Key: 17, State: true}
	logEntry := &pb.SubscribeLogsResponse{Level: pb.LogLevel_LOG_LEVEL_INFO, Message: []byte("synthetic log\n")}
	scenario := simulator.Scenario{
		Name:     "copy-simulator",
		Entities: []proto.Message{entity},
		States:   []proto.Message{state},
		Logs:     []*pb.SubscribeLogsResponse{logEntry},
	}
	device := simulator.New(scenario)
	t.Cleanup(func() { _ = device.Close() })

	entity.Key = 99
	state.State = false
	logEntry.Message[0] = 'X'
	scenario.Entities[0] = &pb.ListEntitiesNumberResponse{Key: 88}

	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	entities, err := client.ListEntities()
	if err != nil {
		t.Fatal(err)
	}
	switches := client.Entities().Switches()
	if len(entities) != 1 || len(switches) != 1 || switches[0].Key != 17 {
		t.Fatalf("device observed caller mutation: %#v", entities)
	}
	states := make(chan proto.Message, 1)
	unsubscribeStates, err := client.SubscribeStates(func(message proto.Message) { states <- message })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribeStates)
	select {
	case message := <-states:
		observed, ok := message.(*pb.SwitchStateResponse)
		if !ok || !observed.State {
			t.Fatalf("device observed caller state mutation: %#v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("copied initial state was not received")
	}
	logs := make(chan *pb.SubscribeLogsResponse, 1)
	unsubscribeLogs, err := client.SubscribeLogs(pb.LogLevel_LOG_LEVEL_INFO, func(message *pb.SubscribeLogsResponse) { logs <- message })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribeLogs)
	select {
	case message := <-logs:
		if string(message.Message) != "synthetic log\n" {
			t.Fatalf("device observed caller log mutation: %q", message.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("copied log was not received")
	}
}

func TestNewDefensivelyCopiesInitialStatesAndTimeline(t *testing.T) {
	clock := simulator.NewManualClock()
	initial := &pb.SwitchStateResponse{Key: 23, State: false}
	future := &pb.SwitchStateResponse{Key: 23, State: true}
	device := simulator.New(simulator.Scenario{
		Name:          "timeline-copy-simulator",
		InitialStates: []proto.Message{initial},
		StateTimeline: []simulator.StateEvent{{At: time.Second, State: future}},
	}, simulator.WithManualClock(clock))
	t.Cleanup(func() { _ = device.Close() })
	initial.State = true
	future.State = false

	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	states := make(chan bool, 2)
	unsubscribe, err := client.SubscribeStates(func(message proto.Message) {
		if state, ok := message.(*pb.SwitchStateResponse); ok && state.Key == 23 {
			states <- state.State
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribe)
	select {
	case state := <-states:
		if state {
			t.Fatal("device observed caller mutation to initial state")
		}
	case <-time.After(time.Second):
		t.Fatal("initial state was not received")
	}
	if err := clock.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	select {
	case state := <-states:
		if !state {
			t.Fatal("device observed caller mutation to timeline state")
		}
	case <-time.After(time.Second):
		t.Fatal("timeline state was not received")
	}
}

func TestScenarioValidateAllowsCompatibleZeroValues(t *testing.T) {
	scenario := simulator.Scenario{
		Seed: 0,
		Entities: []proto.Message{
			&pb.ListEntitiesBinarySensorResponse{Key: 9},
			&pb.ListEntitiesSwitchResponse{Key: 9},
		},
		States: []proto.Message{&pb.SwitchStateResponse{Key: 9}},
	}
	if err := scenario.Validate(); err != nil {
		t.Fatalf("non-random zero-seed scenario rejected: %v", err)
	}
}

func TestScenarioValidateRejectsResourceBudgets(t *testing.T) {
	tests := []struct {
		name     string
		scenario simulator.Scenario
		field    string
		index    int
	}{
		{
			name:     "name bytes",
			scenario: simulator.Scenario{Name: strings.Repeat("n", simulator.MaxScenarioMessageBytes+1)},
			field:    "name",
		},
		{
			name: "items before identity allocation",
			scenario: simulator.Scenario{Entities: make(
				[]proto.Message, simulator.MaxScenarioItemsPerField+1,
			)},
			field: "entities",
			index: simulator.MaxScenarioItemsPerField,
		},
		{
			name: "single encoded message",
			scenario: simulator.Scenario{Logs: []*pb.SubscribeLogsResponse{{
				Message: make([]byte, simulator.MaxScenarioMessageBytes),
			}}},
			field: "logs",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.scenario.Validate()
			var validationErr *simulator.ValidationError
			if !errors.Is(err, simulator.ErrInvalidScenario) || !errors.As(err, &validationErr) ||
				validationErr.Code != simulator.ValidationResourceLimit ||
				validationErr.Field != test.field || validationErr.Index != test.index {
				t.Fatalf("validation error = %#v", err)
			}
		})
	}

	logs := make([]*pb.SubscribeLogsResponse, 65)
	for index := range logs {
		logs[index] = &pb.SubscribeLogsResponse{Message: make([]byte, 65_500)}
	}
	err := (simulator.Scenario{Logs: logs}).Validate()
	var validationErr *simulator.ValidationError
	if !errors.Is(err, simulator.ErrInvalidScenario) || !errors.As(err, &validationErr) ||
		validationErr.Code != simulator.ValidationResourceLimit || validationErr.Field != "logs" ||
		validationErr.Index < 1 || validationErr.Index >= len(logs) {
		t.Fatalf("aggregate validation error = %#v", err)
	}
}

func TestScenarioValidateRejectsDuplicateInitialStateKeys(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		scenario simulator.Scenario
	}{
		{
			name:  "legacy states",
			field: "states",
			scenario: simulator.Scenario{States: []proto.Message{
				&pb.SwitchStateResponse{Key: 41, State: false},
				&pb.SwitchStateResponse{Key: 41, State: true},
			}},
		},
		{
			name:  "initial states",
			field: "initial_states",
			scenario: simulator.Scenario{InitialStates: []proto.Message{
				&pb.TextSensorStateResponse{Key: 73, State: "first-private-value"},
				&pb.TextSensorStateResponse{Key: 73, State: "second-private-value"},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.scenario.Validate()
			if !errors.Is(err, simulator.ErrInvalidScenario) {
				t.Fatalf("error = %v, want ErrInvalidScenario", err)
			}
			var validationErr *simulator.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error = %T, want *ValidationError", err)
			}
			if validationErr.Field != test.field || validationErr.Index != 1 ||
				validationErr.RelatedIndex == nil || *validationErr.RelatedIndex != 0 ||
				validationErr.Code != simulator.ValidationDuplicateKey {
				t.Fatalf("validation error = %#v", validationErr)
			}
			for _, privateValue := range []string{"41", "73", "first-private-value", "second-private-value"} {
				if strings.Contains(err.Error(), privateValue) {
					t.Fatalf("validation error leaked scenario data %q: %v", privateValue, err)
				}
			}
		})
	}
}

func TestScenarioValidateAllowsSameInitialStateKeyAcrossFamilies(t *testing.T) {
	scenario := simulator.Scenario{States: []proto.Message{
		&pb.BinarySensorStateResponse{Key: 29},
		&pb.SensorStateResponse{Key: 29},
		&pb.TextSensorStateResponse{Key: 29},
		&pb.SwitchStateResponse{Key: 29},
		&pb.NumberStateResponse{Key: 29},
		&pb.FanStateResponse{Key: 29},
		&pb.LightStateResponse{Key: 29},
	}}
	if err := scenario.Validate(); err != nil {
		t.Fatalf("same key in distinct state families was rejected: %v", err)
	}
}

func TestScenarioValidateKeepsInvalidInitialStateClassification(t *testing.T) {
	tests := []struct {
		name  string
		state proto.Message
	}{
		{name: "nil", state: nil},
		{name: "typed nil", state: (*pb.SwitchStateResponse)(nil)},
		{name: "entity response", state: &pb.ListEntitiesSwitchResponse{Key: 1}},
		{name: "unsupported message", state: &pb.PingRequest{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (simulator.Scenario{States: []proto.Message{test.state}}).Validate()
			var validationErr *simulator.ValidationError
			if !errors.Is(err, simulator.ErrInvalidScenario) || !errors.As(err, &validationErr) ||
				validationErr.Field != "states" || validationErr.Index != 0 ||
				validationErr.RelatedIndex != nil || validationErr.Code != simulator.ValidationInvalidType {
				t.Fatalf("validation error = %#v", err)
			}
		})
	}
}

func TestDuplicateInitialStateValidationPrecedesConnectionWork(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name: "private-device.local:6053",
		States: []proto.Message{
			&pb.TextSensorStateResponse{Key: 424242, State: "private-first-value"},
			&pb.TextSensorStateResponse{Key: 424242, State: "private-second-value"},
		},
	})
	t.Cleanup(func() { _ = device.Close() })

	connection, err := device.DialContext(context.Background(), "tcp", "private-network-target:6053")
	if connection != nil || !errors.Is(err, simulator.ErrInvalidScenario) {
		t.Fatalf("DialContext = (%v, %v), want typed validation failure", connection, err)
	}
	var validationErr *simulator.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "states" ||
		validationErr.Index != 1 || validationErr.RelatedIndex == nil ||
		*validationErr.RelatedIndex != 0 || validationErr.Code != simulator.ValidationDuplicateKey {
		t.Fatalf("DialContext error = %#v", err)
	}
	if stats := device.Stats(); stats.AcceptedConnections != 0 || stats.ActiveConnections != 0 {
		t.Fatalf("invalid scenario started a connection: %+v", stats)
	}

	listener := &validationPanicListener{}
	err = device.Serve(listener)
	if !errors.Is(err, simulator.ErrInvalidScenario) || listener.addressRead {
		t.Fatalf("Serve = %v, addressRead=%t; want validation before listener use", err, listener.addressRead)
	}
	for _, privateValue := range []string{
		"private-device.local", "private-network-target", "424242",
		"private-first-value", "private-second-value", simulator.DefaultTestEncryptionKey,
	} {
		if strings.Contains(err.Error(), privateValue) {
			t.Fatalf("validation error leaked scenario data %q: %v", privateValue, err)
		}
	}
}

func TestScenarioValidationErrorIsSecretSafe(t *testing.T) {
	scenario := simulator.Scenario{
		Name: "private-device-name",
		Entities: []proto.Message{
			&pb.ListEntitiesSwitchResponse{Key: 42, Name: "private-entity-name"},
			&pb.ListEntitiesSwitchResponse{Key: 42, Name: "another-private-name"},
		},
	}
	err := scenario.Validate()
	for _, privateValue := range []string{"private-device-name", "private-entity-name", "another-private-name", "42"} {
		if strings.Contains(err.Error(), privateValue) {
			t.Fatalf("validation error leaked scenario data %q: %v", privateValue, err)
		}
	}
}

func TestNewDefersValidationWithoutChangingSignature(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Entities: []proto.Message{&pb.SwitchStateResponse{Key: 1}},
	})
	t.Cleanup(func() { _ = device.Close() })

	connection, err := device.DialContext(context.Background(), "tcp", "synthetic:6053")
	if connection != nil || !errors.Is(err, simulator.ErrInvalidScenario) {
		t.Fatalf("DialContext = (%v, %v), want typed validation failure", connection, err)
	}
	var validationErr *simulator.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("DialContext error does not retain ValidationError: %v", err)
	}
	if stats := device.Stats(); stats.AcceptedConnections != 0 || stats.ActiveConnections != 0 {
		t.Fatalf("invalid scenario started a connection: %+v", stats)
	}

	listener := &validationPanicListener{}
	err = device.Serve(listener)
	if !errors.Is(err, simulator.ErrInvalidScenario) || listener.addressRead {
		t.Fatalf("Serve = %v, addressRead=%t; want validation before listener use", err, listener.addressRead)
	}
}

type validationPanicListener struct{ addressRead bool }

func (l *validationPanicListener) Accept() (net.Conn, error) { panic("Accept must not be called") }
func (l *validationPanicListener) Close() error              { return nil }
func (l *validationPanicListener) Addr() net.Addr {
	l.addressRead = true
	panic("Addr must not be called")
}

func ExampleScenario_Validate() {
	scenario := simulator.Scenario{
		Entities: []proto.Message{&pb.SwitchStateResponse{Key: 1}},
	}
	err := scenario.Validate()
	var validationErr *simulator.ValidationError
	fmt.Println(errors.Is(err, simulator.ErrInvalidScenario), errors.As(err, &validationErr))
	fmt.Println(validationErr.Field, validationErr.Code)
	// Output:
	// true true
	// entities invalid_type
}

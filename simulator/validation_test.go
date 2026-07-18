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

package simulator

import (
	"reflect"
	"testing"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

func TestScenarioFieldInventoryRequiresValidationAndCloneReview(t *testing.T) {
	want := []string{
		"Name",
		"Seed",
		"Entities",
		"States",
		"InitialStates",
		"StateTimeline",
		"Logs",
		"Commands",
		"Faults",
	}
	typeOfScenario := reflect.TypeOf(Scenario{})
	got := make([]string, typeOfScenario.NumField())
	for index := 0; index < typeOfScenario.NumField(); index++ {
		got[index] = typeOfScenario.Field(index).Name
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scenario fields = %v, want %v; extend Scenario.Validate, cloneScenario, and this inventory together", got, want)
	}
}

func TestCloneScenarioCopiesEveryKnownFieldWithoutAliasing(t *testing.T) {
	source := Scenario{
		Name: "synthetic-completeness-test",
		Seed: 17,
		Entities: []proto.Message{
			&pb.ListEntitiesSwitchResponse{Key: 1, Name: "Synthetic Switch"},
		},
		States: []proto.Message{
			&pb.SwitchStateResponse{Key: 1, State: true},
		},
		InitialStates: []proto.Message{
			&pb.NumberStateResponse{Key: 2, State: 12.5},
		},
		StateTimeline: []StateEvent{
			{At: time.Second, State: &pb.FanStateResponse{Key: 3, State: true, SpeedLevel: 42}},
		},
		Logs: []*pb.SubscribeLogsResponse{
			{Level: pb.LogLevel_LOG_LEVEL_INFO, Message: []byte("synthetic log")},
		},
		Commands: []CommandExpectation{
			{Command: &pb.SwitchCommandRequest{Key: 1, State: true}, Count: 2},
		},
		Faults: []Fault{
			{Trigger: FaultAfterHello, Action: FaultUnknownMessage},
		},
	}
	sourceValue := reflect.ValueOf(source)
	for index := 0; index < sourceValue.NumField(); index++ {
		if sourceValue.Field(index).IsZero() {
			t.Fatalf("clone completeness fixture does not populate Scenario.%s; extend the fixture before accepting a Scenario field", sourceValue.Type().Field(index).Name)
		}
	}

	cloned := cloneScenario(source)
	if !reflect.DeepEqual(cloned, source) {
		t.Fatalf("cloneScenario did not preserve every known Scenario field:\nclone:  %#v\nsource: %#v", cloned, source)
	}

	source.Entities[0].(*pb.ListEntitiesSwitchResponse).Name = "mutated entity"
	source.States[0].(*pb.SwitchStateResponse).State = false
	source.InitialStates[0].(*pb.NumberStateResponse).State = 99
	source.StateTimeline[0].At = 2 * time.Second
	source.StateTimeline[0].State.(*pb.FanStateResponse).SpeedLevel = 7
	source.Logs[0].Message[0] = 'X'
	source.Commands[0].Command.(*pb.SwitchCommandRequest).State = false
	source.Commands[0].Count = 99
	source.Faults[0].Action = FaultDropConnection

	if cloned.Entities[0].(*pb.ListEntitiesSwitchResponse).Name != "Synthetic Switch" ||
		!cloned.States[0].(*pb.SwitchStateResponse).State ||
		cloned.InitialStates[0].(*pb.NumberStateResponse).State != 12.5 ||
		cloned.StateTimeline[0].At != time.Second ||
		cloned.StateTimeline[0].State.(*pb.FanStateResponse).SpeedLevel != 42 ||
		string(cloned.Logs[0].Message) != "synthetic log" ||
		!cloned.Commands[0].Command.(*pb.SwitchCommandRequest).State ||
		cloned.Commands[0].Count != 2 ||
		cloned.Faults[0].Action != FaultUnknownMessage {
		t.Fatalf("cloneScenario retained mutable source aliases: %#v", cloned)
	}
}

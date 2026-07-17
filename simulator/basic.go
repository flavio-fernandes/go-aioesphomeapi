package simulator

import (
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

const (
	BasicButtonKey uint32 = 201
	BasicSwitchKey uint32 = 202
	BasicNumberKey uint32 = 203
	BlinkSensorKey uint32 = 211
	BlinkSwitchKey uint32 = 212
)

// BasicIOScenario is a generic input, switch, and number device. Its initial
// outputs intentionally differ from the safe desired values used by the MGMT
// compatibility example so a test observes real corrective commands.
func BasicIOScenario() Scenario {
	return Scenario{
		Name: "basic-io-simulator",
		Entities: []proto.Message{
			&pb.ListEntitiesBinarySensorResponse{Key: BasicButtonKey, ObjectId: "button_a", Name: "Button A"},
			&pb.ListEntitiesSwitchResponse{Key: BasicSwitchKey, ObjectId: "led_1", Name: "LED 1"},
			&pb.ListEntitiesNumberResponse{Key: BasicNumberKey, ObjectId: "motor_speed", Name: "Motor Speed", MinValue: 0, MaxValue: 1, Step: 0.01},
		},
		States: []proto.Message{
			&pb.BinarySensorStateResponse{Key: BasicButtonKey, State: false},
			&pb.SwitchStateResponse{Key: BasicSwitchKey, State: true},
			&pb.NumberStateResponse{Key: BasicNumberKey, State: 0.75},
		},
		Logs: []*pb.SubscribeLogsResponse{{Level: pb.LogLevel_LOG_LEVEL_INFO, Message: []byte("basic I/O simulator ready\n")}},
	}
}

// BlinkScenario is a generic read-only indicator mirrored by a writable
// switch. The indicator starts on so an MGMT compatibility run must command
// the switch off.
func BlinkScenario() Scenario {
	return Scenario{
		Name: "esphome-blink",
		Entities: []proto.Message{
			&pb.ListEntitiesBinarySensorResponse{Key: BlinkSensorKey, ObjectId: "onboard_led_state", Name: "Onboard LED State"},
			&pb.ListEntitiesSwitchResponse{Key: BlinkSwitchKey, ObjectId: "onboard_led", Name: "Onboard LED"},
		},
		States: []proto.Message{
			&pb.BinarySensorStateResponse{Key: BlinkSensorKey, State: true},
			&pb.SwitchStateResponse{Key: BlinkSwitchKey, State: true},
		},
		Logs: []*pb.SubscribeLogsResponse{{Level: pb.LogLevel_LOG_LEVEL_INFO, Message: []byte("blink simulator ready\n")}},
	}
}

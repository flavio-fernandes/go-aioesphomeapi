package simulator

import (
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

const (
	ConveyorFanKey  uint32 = 101
	StatusLightKey  uint32 = 102
	EntrySensorKey  uint32 = 103
	ExitSensorKey   uint32 = 104
	RunRequestKey   uint32 = 105
	RedSensorKey    uint32 = 106
	GreenSensorKey  uint32 = 107
	BlueSensorKey   uint32 = 108
	ClearSensorKey  uint32 = 109
	EnableSwitchKey uint32 = 110
	SpeedNumberKey  uint32 = 111
	ResetButtonKey  uint32 = 112
	StatusTextKey   uint32 = 113
)

// ConveyorScenario models the entities required by the MGMT conveyor demo.
func ConveyorScenario() Scenario {
	return Scenario{Name: "conveyor-simulator", Entities: []proto.Message{
		&pb.ListEntitiesFanResponse{Key: ConveyorFanKey, ObjectId: "conveyor_motor", Name: "Conveyor Motor", SupportsSpeed: true, SupportsDirection: true, SupportedSpeedCount: 100},
		&pb.ListEntitiesLightResponse{Key: StatusLightKey, ObjectId: "status_light", Name: "Status Light", SupportedColorModes: []pb.ColorMode{pb.ColorMode_COLOR_MODE_RGB}},
		&pb.ListEntitiesBinarySensorResponse{Key: EntrySensorKey, ObjectId: "entry_object_present", Name: "Entry Object Present"},
		&pb.ListEntitiesBinarySensorResponse{Key: ExitSensorKey, ObjectId: "exit_object_present", Name: "Exit Object Present"},
		&pb.ListEntitiesBinarySensorResponse{Key: RunRequestKey, ObjectId: "conveyor_run_request", Name: "Conveyor Run Request"},
		&pb.ListEntitiesSensorResponse{Key: RedSensorKey, ObjectId: "entry_red", Name: "Entry Red"},
		&pb.ListEntitiesSensorResponse{Key: GreenSensorKey, ObjectId: "entry_green", Name: "Entry Green"},
		&pb.ListEntitiesSensorResponse{Key: BlueSensorKey, ObjectId: "entry_blue", Name: "Entry Blue"},
		&pb.ListEntitiesSensorResponse{Key: ClearSensorKey, ObjectId: "color_clear", Name: "Color Clear"},
		&pb.ListEntitiesSwitchResponse{Key: EnableSwitchKey, ObjectId: "conveyor_enable", Name: "Conveyor Enable"},
		&pb.ListEntitiesNumberResponse{Key: SpeedNumberKey, ObjectId: "conveyor_speed", Name: "Conveyor Speed", MinValue: 0, MaxValue: 100, Step: 1},
		&pb.ListEntitiesButtonResponse{Key: ResetButtonKey, ObjectId: "reset_cycle", Name: "Reset Cycle"},
		&pb.ListEntitiesTextSensorResponse{Key: StatusTextKey, ObjectId: "system_status", Name: "System Status"},
	}, States: []proto.Message{
		&pb.FanStateResponse{Key: ConveyorFanKey, Direction: pb.FanDirection_FAN_DIRECTION_FORWARD},
		&pb.LightStateResponse{Key: StatusLightKey, ColorMode: pb.ColorMode_COLOR_MODE_RGB},
		&pb.BinarySensorStateResponse{Key: EntrySensorKey}, &pb.BinarySensorStateResponse{Key: ExitSensorKey}, &pb.BinarySensorStateResponse{Key: RunRequestKey},
		&pb.SensorStateResponse{Key: RedSensorKey}, &pb.SensorStateResponse{Key: GreenSensorKey}, &pb.SensorStateResponse{Key: BlueSensorKey}, &pb.SensorStateResponse{Key: ClearSensorKey},
		&pb.SwitchStateResponse{Key: EnableSwitchKey}, &pb.NumberStateResponse{Key: SpeedNumberKey},
		&pb.TextSensorStateResponse{Key: StatusTextKey, State: "ready"},
	}, Logs: []*pb.SubscribeLogsResponse{{Level: pb.LogLevel_LOG_LEVEL_INFO, Message: []byte("conveyor simulator ready\n")}}}
}

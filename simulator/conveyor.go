package simulator

import (
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

// Keys for the conveyor demo contract. The conveyor firmware exports exactly
// four entities, because those are the only four MGMT reads or writes. Every
// raw sensor behind them is marked internal on the device and never reaches
// the wire.
const (
	ConveyorFanKey  uint32 = 101
	StatusLightKey  uint32 = 102
	BrickEnRouteKey uint32 = 103
	ExitRedRatioKey uint32 = 104
)

// Effect names the conveyor light declares. The device owns the animation and
// MGMT only selects one by name, so these strings are part of the contract.
// ESPHome always advertises the absence of an effect first, under the name the
// native api uses for it.
const (
	ConveyorEffectNone      = "None"
	ConveyorEffectRainbow   = "Rainbow"
	ConveyorEffectTraveling = "Traveling Blink"
)

// ExitRedRatioNoReading is what the conveyor publishes when it has no
// trustworthy colour reading: nothing at the exit, a brick lifted off
// mid-capture, or a capture too dark to mean anything. A real ratio is a share
// of a positive total, so a negative value can never collide with one.
const ExitRedRatioNoReading float32 = -1

// Keys for the entity-family scenario. These exercise the client's generic
// per-domain API. They are not part of any device contract.
const (
	FamilySwitchKey     uint32 = 201
	FamilyNumberKey     uint32 = 202
	FamilyButtonKey     uint32 = 203
	FamilyTextSensorKey uint32 = 204
	FamilySensorKey     uint32 = 205
	FamilyBinaryKey     uint32 = 206
)

// ConveyorScenario models the entities the MGMT conveyor demo exchanges with
// the device, and nothing else.
//
// The device reports two facts, whether a brick is on its way and how red the
// brick at the exit is, and accepts two commands, the belt motor and the
// status light. It never steers the belt itself while a controller is
// connected. MGMT owns every decision. Keeping the exported set this small is
// the point rather than an accident: it is what keeps the demo responsive,
// because the device sends a handful of messages per brick instead of a
// telemetry stream that MGMT would have to filter.
//
// The initial states match what the firmware publishes on boot: the motor is
// off, the light is off, no brick is en route, and there is no colour reading.
func ConveyorScenario() Scenario {
	return Scenario{Name: "conveyor-simulator", Entities: []proto.Message{
		&pb.ListEntitiesFanResponse{Key: ConveyorFanKey, ObjectId: "conveyor_motor", Name: "Conveyor Motor", SupportsSpeed: true, SupportsDirection: true, SupportedSpeedCount: 100},
		&pb.ListEntitiesLightResponse{Key: StatusLightKey, ObjectId: "status_light", Name: "Status Light", SupportedColorModes: []pb.ColorMode{pb.ColorMode_COLOR_MODE_RGB}, Effects: []string{ConveyorEffectNone, ConveyorEffectRainbow, ConveyorEffectTraveling}},
		&pb.ListEntitiesBinarySensorResponse{Key: BrickEnRouteKey, ObjectId: "brick_en_route", Name: "Brick En Route"},
		&pb.ListEntitiesSensorResponse{Key: ExitRedRatioKey, ObjectId: "exit_red_ratio", Name: "Exit Red Ratio", UnitOfMeasurement: "%", AccuracyDecimals: 1},
	}, States: []proto.Message{
		&pb.FanStateResponse{Key: ConveyorFanKey, Direction: pb.FanDirection_FAN_DIRECTION_FORWARD},
		&pb.LightStateResponse{Key: StatusLightKey, ColorMode: pb.ColorMode_COLOR_MODE_RGB, Effect: ConveyorEffectNone},
		&pb.BinarySensorStateResponse{Key: BrickEnRouteKey, State: false},
		&pb.SensorStateResponse{Key: ExitRedRatioKey, State: ExitRedRatioNoReading},
	}, Logs: []*pb.SubscribeLogsResponse{{Level: pb.LogLevel_LOG_LEVEL_INFO, Message: []byte("conveyor simulator ready\n")}}}
}

// EntityFamilyScenario advertises one entity per supported domain so a client
// test can exercise every generic command path against a single device. It
// deliberately describes no real appliance: use ConveyorScenario when the
// point is device behavior rather than API coverage.
func EntityFamilyScenario() Scenario {
	return Scenario{Name: "entity-family-simulator", Entities: []proto.Message{
		&pb.ListEntitiesFanResponse{Key: ConveyorFanKey, ObjectId: "example_fan", Name: "Example Fan", SupportsSpeed: true, SupportsDirection: true, SupportedSpeedCount: 100},
		&pb.ListEntitiesLightResponse{Key: StatusLightKey, ObjectId: "example_light", Name: "Example Light", SupportedColorModes: []pb.ColorMode{pb.ColorMode_COLOR_MODE_RGB}},
		&pb.ListEntitiesSwitchResponse{Key: FamilySwitchKey, ObjectId: "example_switch", Name: "Example Switch"},
		&pb.ListEntitiesNumberResponse{Key: FamilyNumberKey, ObjectId: "example_number", Name: "Example Number", MinValue: 0, MaxValue: 100, Step: 1},
		&pb.ListEntitiesButtonResponse{Key: FamilyButtonKey, ObjectId: "example_button", Name: "Example Button"},
		&pb.ListEntitiesTextSensorResponse{Key: FamilyTextSensorKey, ObjectId: "example_text", Name: "Example Text"},
		&pb.ListEntitiesSensorResponse{Key: FamilySensorKey, ObjectId: "example_sensor", Name: "Example Sensor"},
		&pb.ListEntitiesBinarySensorResponse{Key: FamilyBinaryKey, ObjectId: "example_binary", Name: "Example Binary"},
	}, States: []proto.Message{
		// The initial states are published in this order, and a subscriber that
		// waits for one type at a time discards whatever arrives before it.
		&pb.FanStateResponse{Key: ConveyorFanKey, Direction: pb.FanDirection_FAN_DIRECTION_FORWARD},
		&pb.BinarySensorStateResponse{Key: FamilyBinaryKey},
		&pb.SensorStateResponse{Key: FamilySensorKey},
		&pb.SwitchStateResponse{Key: FamilySwitchKey},
		&pb.NumberStateResponse{Key: FamilyNumberKey},
		&pb.TextSensorStateResponse{Key: FamilyTextSensorKey, State: "ready"},
		&pb.LightStateResponse{Key: StatusLightKey, ColorMode: pb.ColorMode_COLOR_MODE_RGB},
	}, Logs: []*pb.SubscribeLogsResponse{{Level: pb.LogLevel_LOG_LEVEL_INFO, Message: []byte("entity family simulator ready\n")}}}
}

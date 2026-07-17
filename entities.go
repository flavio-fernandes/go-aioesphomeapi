package aioesphomeapi

import (
	"sync"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

// EntityDomain identifies an ESPHome entity kind.
type EntityDomain string

const (
	DomainBinarySensor EntityDomain = "binary_sensor"
	DomainSensor       EntityDomain = "sensor"
	DomainTextSensor   EntityDomain = "text_sensor"
	DomainSwitch       EntityDomain = "switch"
	DomainNumber       EntityDomain = "number"
	DomainButton       EntityDomain = "button"
	DomainFan          EntityDomain = "fan"
	DomainLight        EntityDomain = "light"
)

type entityBase struct {
	Key      uint32
	ObjectID string
	Name     string
}

type BinarySensorEntity struct {
	entityBase
	State, MissingState bool
}
type SensorEntity struct {
	entityBase
	State        float32
	MissingState bool
}
type TextSensorEntity struct {
	entityBase
	State        string
	MissingState bool
}
type SwitchEntity struct {
	entityBase
	State bool
}
type NumberEntity struct {
	entityBase
	State        float32
	MissingState bool
}
type ButtonEntity struct{ entityBase }
type FanEntity struct {
	entityBase
	SupportsOscillation  bool
	SupportsSpeed        bool
	SupportsDirection    bool
	SupportedSpeedCount  int32
	SupportedPresetModes []string
	State                bool
	Oscillating          bool
	Direction            pb.FanDirection
	SpeedLevel           int32
	PresetMode           string
}
type LightEntity struct {
	entityBase
	SupportedColorModes     []pb.ColorMode
	Effects                 []string
	State                   bool
	Brightness              float32
	ColorMode               pb.ColorMode
	ColorBrightness         float32
	Red, Green, Blue, White float32
	ColorTemperature        float32
	ColdWhite, WarmWhite    float32
	Effect                  string
}

// EntityRegistry holds entity metadata and the most recently observed state.
// Returned slices are snapshots and are safe to iterate concurrently.
type EntityRegistry struct {
	mu            sync.RWMutex
	binarySensors map[uint32]*BinarySensorEntity
	sensors       map[uint32]*SensorEntity
	textSensors   map[uint32]*TextSensorEntity
	switches      map[uint32]*SwitchEntity
	numbers       map[uint32]*NumberEntity
	buttons       map[uint32]*ButtonEntity
	fans          map[uint32]*FanEntity
	lights        map[uint32]*LightEntity
}

func newEntityRegistry() *EntityRegistry {
	return &EntityRegistry{
		binarySensors: make(map[uint32]*BinarySensorEntity), sensors: make(map[uint32]*SensorEntity),
		textSensors: make(map[uint32]*TextSensorEntity), switches: make(map[uint32]*SwitchEntity),
		numbers: make(map[uint32]*NumberEntity), buttons: make(map[uint32]*ButtonEntity),
		fans: make(map[uint32]*FanEntity), lights: make(map[uint32]*LightEntity),
	}
}

func (r *EntityRegistry) handle(message proto.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch m := message.(type) {
	case *pb.ListEntitiesBinarySensorResponse:
		r.binarySensors[m.Key] = &BinarySensorEntity{entityBase: entityBase{m.Key, m.ObjectId, m.Name}}
	case *pb.ListEntitiesSensorResponse:
		r.sensors[m.Key] = &SensorEntity{entityBase: entityBase{m.Key, m.ObjectId, m.Name}}
	case *pb.ListEntitiesTextSensorResponse:
		r.textSensors[m.Key] = &TextSensorEntity{entityBase: entityBase{m.Key, m.ObjectId, m.Name}}
	case *pb.ListEntitiesSwitchResponse:
		r.switches[m.Key] = &SwitchEntity{entityBase: entityBase{m.Key, m.ObjectId, m.Name}}
	case *pb.ListEntitiesNumberResponse:
		r.numbers[m.Key] = &NumberEntity{entityBase: entityBase{m.Key, m.ObjectId, m.Name}}
	case *pb.ListEntitiesButtonResponse:
		r.buttons[m.Key] = &ButtonEntity{entityBase: entityBase{m.Key, m.ObjectId, m.Name}}
	case *pb.ListEntitiesFanResponse:
		r.fans[m.Key] = &FanEntity{entityBase: entityBase{m.Key, m.ObjectId, m.Name}, SupportsOscillation: m.SupportsOscillation, SupportsSpeed: m.SupportsSpeed, SupportsDirection: m.SupportsDirection, SupportedSpeedCount: m.SupportedSpeedCount, SupportedPresetModes: append([]string(nil), m.SupportedPresetModes...)}
	case *pb.ListEntitiesLightResponse:
		r.lights[m.Key] = &LightEntity{entityBase: entityBase{m.Key, m.ObjectId, m.Name}, SupportedColorModes: append([]pb.ColorMode(nil), m.SupportedColorModes...), Effects: append([]string(nil), m.Effects...)}
	case *pb.BinarySensorStateResponse:
		if e := r.binarySensors[m.Key]; e != nil {
			e.State, e.MissingState = m.State, m.MissingState
		}
	case *pb.SensorStateResponse:
		if e := r.sensors[m.Key]; e != nil {
			e.State, e.MissingState = m.State, m.MissingState
		}
	case *pb.TextSensorStateResponse:
		if e := r.textSensors[m.Key]; e != nil {
			e.State, e.MissingState = m.State, m.MissingState
		}
	case *pb.SwitchStateResponse:
		if e := r.switches[m.Key]; e != nil {
			e.State = m.State
		}
	case *pb.NumberStateResponse:
		if e := r.numbers[m.Key]; e != nil {
			e.State, e.MissingState = m.State, m.MissingState
		}
	case *pb.FanStateResponse:
		if e := r.fans[m.Key]; e != nil {
			e.State, e.Oscillating, e.Direction, e.SpeedLevel, e.PresetMode = m.State, m.Oscillating, m.Direction, m.SpeedLevel, m.PresetMode
		}
	case *pb.LightStateResponse:
		if e := r.lights[m.Key]; e != nil {
			e.State, e.Brightness, e.ColorMode, e.ColorBrightness, e.Red, e.Green, e.Blue, e.White, e.ColorTemperature, e.ColdWhite, e.WarmWhite, e.Effect = m.State, m.Brightness, m.ColorMode, m.ColorBrightness, m.Red, m.Green, m.Blue, m.White, m.ColorTemperature, m.ColdWhite, m.WarmWhite, m.Effect
		}
	}
}

func values[T any](source map[uint32]*T) []*T {
	result := make([]*T, 0, len(source))
	for _, value := range source {
		copy := *value
		result = append(result, &copy)
	}
	return result
}
func (r *EntityRegistry) BinarySensors() []*BinarySensorEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return values(r.binarySensors)
}
func (r *EntityRegistry) Sensors() []*SensorEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return values(r.sensors)
}
func (r *EntityRegistry) TextSensors() []*TextSensorEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return values(r.textSensors)
}
func (r *EntityRegistry) Switches() []*SwitchEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return values(r.switches)
}
func (r *EntityRegistry) Numbers() []*NumberEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return values(r.numbers)
}
func (r *EntityRegistry) Buttons() []*ButtonEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return values(r.buttons)
}
func (r *EntityRegistry) Fans() []*FanEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := values(r.fans)
	for _, entity := range result {
		entity.SupportedPresetModes = append([]string(nil), entity.SupportedPresetModes...)
	}
	return result
}
func (r *EntityRegistry) Lights() []*LightEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := values(r.lights)
	for _, entity := range result {
		entity.SupportedColorModes = append([]pb.ColorMode(nil), entity.SupportedColorModes...)
		entity.Effects = append([]string(nil), entity.Effects...)
	}
	return result
}

func (r *EntityRegistry) domain(key uint32) (EntityDomain, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.binarySensors[key] != nil {
		return DomainBinarySensor, true
	}
	if r.sensors[key] != nil {
		return DomainSensor, true
	}
	if r.textSensors[key] != nil {
		return DomainTextSensor, true
	}
	if r.switches[key] != nil {
		return DomainSwitch, true
	}
	if r.numbers[key] != nil {
		return DomainNumber, true
	}
	if r.buttons[key] != nil {
		return DomainButton, true
	}
	if r.fans[key] != nil {
		return DomainFan, true
	}
	if r.lights[key] != nil {
		return DomainLight, true
	}
	return "", false
}

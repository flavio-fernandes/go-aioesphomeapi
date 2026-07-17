package aioesphomeapi

import (
	"fmt"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
)

func (c *Client) validateEntity(key uint32, want EntityDomain) error {
	domain, found := c.entities.domain(key)
	if !found {
		return fmt.Errorf("%w: key %d", ErrEntityNotFound, key)
	}
	if domain != want {
		return fmt.Errorf("%w: key %d is %s, not %s", ErrEntityTypeMismatch, key, domain, want)
	}
	return nil
}

// SendCommand sends any command present in the pinned protocol inventory.
func (c *Client) SendCommand(command proto.Message) error { return c.send(command) }
func (c *Client) SetSwitch(key uint32, state bool) error {
	if err := c.validateEntity(key, DomainSwitch); err != nil {
		return err
	}
	return c.send(&pb.SwitchCommandRequest{Key: key, State: state})
}
func (c *Client) SetNumber(key uint32, value float32) error {
	if err := c.validateEntity(key, DomainNumber); err != nil {
		return err
	}
	return c.send(&pb.NumberCommandRequest{Key: key, State: value})
}
func (c *Client) PressButton(key uint32) error {
	if err := c.validateEntity(key, DomainButton); err != nil {
		return err
	}
	return c.send(&pb.ButtonCommandRequest{Key: key})
}

type FanCommandOpts struct {
	HasState       bool
	State          bool
	HasOscillating bool
	Oscillating    bool
	HasDirection   bool
	Direction      pb.FanDirection
	HasSpeedLevel  bool
	SpeedLevel     int32
	HasPresetMode  bool
	PresetMode     string
}

func (c *Client) SetFan(key uint32, options FanCommandOpts) error {
	if err := c.validateEntity(key, DomainFan); err != nil {
		return err
	}
	return c.send(&pb.FanCommandRequest{Key: key, HasState: options.HasState, State: options.State, HasOscillating: options.HasOscillating, Oscillating: options.Oscillating, HasDirection: options.HasDirection, Direction: options.Direction, HasSpeedLevel: options.HasSpeedLevel, SpeedLevel: options.SpeedLevel, HasPresetMode: options.HasPresetMode, PresetMode: options.PresetMode})
}

type LightCommandOpts struct {
	HasState            bool
	State               bool
	HasBrightness       bool
	Brightness          float32
	HasColorMode        bool
	ColorMode           pb.ColorMode
	HasColorBrightness  bool
	ColorBrightness     float32
	HasRGB              bool
	Red, Green, Blue    float32
	HasWhite            bool
	White               float32
	HasColorTemperature bool
	ColorTemperature    float32
	HasColdWhite        bool
	ColdWhite           float32
	HasWarmWhite        bool
	WarmWhite           float32
	HasTransitionLength bool
	TransitionLength    uint32
	HasFlashLength      bool
	FlashLength         uint32
	HasEffect           bool
	Effect              string
}

func (c *Client) SetLight(key uint32, options LightCommandOpts) error {
	if err := c.validateEntity(key, DomainLight); err != nil {
		return err
	}
	return c.send(&pb.LightCommandRequest{Key: key, HasState: options.HasState, State: options.State, HasBrightness: options.HasBrightness, Brightness: options.Brightness, HasColorMode: options.HasColorMode, ColorMode: options.ColorMode, HasColorBrightness: options.HasColorBrightness, ColorBrightness: options.ColorBrightness, HasRgb: options.HasRGB, Red: options.Red, Green: options.Green, Blue: options.Blue, HasWhite: options.HasWhite, White: options.White, HasColorTemperature: options.HasColorTemperature, ColorTemperature: options.ColorTemperature, HasColdWhite: options.HasColdWhite, ColdWhite: options.ColdWhite, HasWarmWhite: options.HasWarmWhite, WarmWhite: options.WarmWhite, HasTransitionLength: options.HasTransitionLength, TransitionLength: options.TransitionLength, HasFlashLength: options.HasFlashLength, FlashLength: options.FlashLength, HasEffect: options.HasEffect, Effect: options.Effect})
}

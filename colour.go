package aioesphomeapi

import (
	"strings"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
)

// ColourMode is the light colour mode a device advertises or is asked to use.
// It aliases the generated protocol enum so that callers can spell the concept
// the way the rest of this library does. The generated identifiers keep the
// upstream ESPHome spelling because pb is regenerated from the pinned
// api.proto and checked for drift; nothing under pb may be respelled by hand.
type ColourMode = pb.ColorMode

// The colour modes defined by the pinned ESPHome protocol. These are aliases
// for the generated enum values, so they compare and serialize identically.
const (
	ColourModeUnknown              = pb.ColorMode_COLOR_MODE_UNKNOWN
	ColourModeOnOff                = pb.ColorMode_COLOR_MODE_ON_OFF
	ColourModeLegacyBrightness     = pb.ColorMode_COLOR_MODE_LEGACY_BRIGHTNESS
	ColourModeBrightness           = pb.ColorMode_COLOR_MODE_BRIGHTNESS
	ColourModeWhite                = pb.ColorMode_COLOR_MODE_WHITE
	ColourModeColourTemperature    = pb.ColorMode_COLOR_MODE_COLOR_TEMPERATURE
	ColourModeColdWarmWhite        = pb.ColorMode_COLOR_MODE_COLD_WARM_WHITE
	ColourModeRGB                  = pb.ColorMode_COLOR_MODE_RGB
	ColourModeRGBWhite             = pb.ColorMode_COLOR_MODE_RGB_WHITE
	ColourModeRGBColourTemperature = pb.ColorMode_COLOR_MODE_RGB_COLOR_TEMPERATURE
	ColourModeRGBColdWarmWhite     = pb.ColorMode_COLOR_MODE_RGB_COLD_WARM_WHITE
)

// ColourModeName returns the name of a colour mode in this library's spelling.
// The generated enum spells its names COLOR_MODE_*, which is the ESPHome
// protocol's own spelling and is wire truth we do not get to choose. Callers
// that surface a mode name to a user or match on one should go through here so
// that COLOUR is the only spelling they ever see; ParseColourMode reverses it.
// A mode with no name in the pinned protocol stringifies to its number, which
// passes through unchanged.
func ColourModeName(mode ColourMode) string {
	return strings.ReplaceAll(mode.String(), "COLOR", "COLOUR")
}

// ParseColourMode resolves a name produced by ColourModeName back to its mode.
// The second return is false for a name the pinned protocol does not define.
func ParseColourMode(name string) (ColourMode, bool) {
	value, ok := pb.ColorMode_value[strings.ReplaceAll(name, "COLOUR", "COLOR")]
	if !ok {
		return ColourModeUnknown, false
	}
	return ColourMode(value), true
}

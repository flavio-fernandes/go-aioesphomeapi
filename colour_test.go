package aioesphomeapi

import (
	"strings"
	"testing"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
)

// TestColourModeNameRespellsEveryPinnedMode checks that no mode in the pinned
// protocol reaches a caller spelled COLOR, and that the number a mode carries
// is untouched by the respelling.
func TestColourModeNameRespellsEveryPinnedMode(t *testing.T) {
	if len(pb.ColorMode_name) == 0 {
		t.Fatal("pinned protocol defines no colour modes")
	}
	for number, generated := range pb.ColorMode_name {
		mode := ColourMode(number)
		name := ColourModeName(mode)
		if strings.Contains(name, "COLOR") {
			t.Errorf("mode %d: name %q still spells COLOR", number, name)
		}
		if want := strings.ReplaceAll(generated, "COLOR", "COLOUR"); name != want {
			t.Errorf("mode %d: name is %q, want %q", number, name, want)
		}
		parsed, ok := ParseColourMode(name)
		if !ok {
			t.Errorf("mode %d: name %q does not parse back", number, name)
			continue
		}
		if parsed != mode {
			t.Errorf("mode %d: name %q parses back to %d", number, name, parsed)
		}
	}
}

// TestColourModeRGBIsTheProtocolValue guards the alias the MGMT light resource
// sends: it has to stay the RGB mode the pinned protocol defines, named the way
// MGMT matches on it.
func TestColourModeRGBIsTheProtocolValue(t *testing.T) {
	if ColourModeRGB != pb.ColorMode_COLOR_MODE_RGB {
		t.Errorf("ColourModeRGB is %d, want %d", ColourModeRGB, pb.ColorMode_COLOR_MODE_RGB)
	}
	if name := ColourModeName(ColourModeRGB); name != "COLOUR_MODE_RGB" {
		t.Errorf("ColourModeRGB is named %q, want %q", name, "COLOUR_MODE_RGB")
	}
}

// TestParseColourModeRejectsUnknownNames checks that a name outside the pinned
// protocol is reported rather than silently resolving to the unknown mode.
func TestParseColourModeRejectsUnknownNames(t *testing.T) {
	for _, name := range []string{"", "COLOUR_MODE_MAUVE", "RGB", "colour_mode_rgb"} {
		if mode, ok := ParseColourMode(name); ok {
			t.Errorf("name %q parsed to mode %d, want rejection", name, mode)
		}
	}
}

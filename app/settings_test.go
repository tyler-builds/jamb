package app

import (
	"testing"

	"github.com/tyler-builds/jamb/platform"
	"github.com/tyler-builds/jamb/ui"
)

// The overrides replace the matching setting only when they are set; an unset
// variable keeps the platform's value.
func TestOverrideSettings(t *testing.T) {
	base := ui.SystemSettings{TextScale: 100, CaretBlink: 530, CaretWidth: 1}
	tests := []struct {
		name string
		d    devOptions
		want func(ui.SystemSettings) bool
	}{
		{"unset keeps the platform's settings", devOptions{}, func(s ui.SystemSettings) bool {
			return !s.HighContrast && s.TextScale == 100 && !s.ReducedMotion
		}},
		{"high contrast black", devOptions{highContrast: "1"}, func(s ui.SystemSettings) bool {
			return s.HighContrast && s.Colors == ui.HighContrastBlack()
		}},
		{"high contrast white", devOptions{highContrast: "white"}, func(s ui.SystemSettings) bool {
			return s.HighContrast && s.Colors == ui.HighContrastWhite()
		}},
		{"high contrast off", devOptions{highContrast: "0"}, func(s ui.SystemSettings) bool {
			return !s.HighContrast
		}},
		{"text scale 150", devOptions{textScale: 150}, func(s ui.SystemSettings) bool {
			return s.TextScale == 150
		}},
		{"reduced motion on", devOptions{reducedMotion: "1"}, func(s ui.SystemSettings) bool {
			return s.ReducedMotion
		}},
		{"reduced motion off", devOptions{reducedMotion: "0"}, func(s ui.SystemSettings) bool {
			return !s.ReducedMotion
		}},
	}
	for _, tt := range tests {
		if got := tt.d.overrideSettings(base); !tt.want(got) {
			t.Errorf("%s: %+v", tt.name, got)
		}
	}
}

// The text scale variable is clamped to 100 to 225 percent, and a value that is
// not a number is ignored.
func TestLoadDevOptionsTextScale(t *testing.T) {
	for in, want := range map[string]int{"": 0, "150": 150, "300": 225, "50": 100, "abc": 0, "0": 0} {
		t.Setenv("JAMB_TEXT_SCALE", in)
		if got := loadDevOptions().textScale; got != want {
			t.Errorf("JAMB_TEXT_SCALE=%q gives %d, want %d", in, got, want)
		}
	}
}

// A system color is 0x00RRGGBB.
func TestUIColorsFromPlatform(t *testing.T) {
	c := uiColors(platform.SystemColors{Highlight: 0x1aebff, Window: 0x000000})
	if got := c.Highlight; got.R != 0x1a/255.0 || got.G != 0xeb/255.0 || got.B != 0xff/255.0 || got.A != 1 {
		t.Errorf("Highlight %v, want #1aebff", got)
	}
	if c.Window != ui.Hex("#000000") {
		t.Errorf("Window %v, want black", c.Window)
	}
}

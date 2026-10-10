//go:build windows

package windows

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/win32"
	"github.com/tyler-builds/jamb/platform"
)

// baseRaw is the raw state of a system with the default settings.
func baseRaw() rawSettings {
	return rawSettings{clientAnimation: 1, caretBlink: 530, caretWidth: 1, textScale: 100}
}

func TestSettingsFromTextScale(t *testing.T) {
	cases := []struct {
		raw  uint32
		want int
	}{
		{0, 100}, // missing
		{99, 100},
		{100, 100},
		{150, 150},
		{225, 225},
		{226, 225},
	}
	for _, c := range cases {
		raw := baseRaw()
		raw.textScale = c.raw
		if got := settingsFrom(raw).TextScale; got != c.want {
			t.Errorf("textScale %d: TextScale = %d, want %d", c.raw, got, c.want)
		}
	}
}

func TestSettingsFromCaretBlink(t *testing.T) {
	cases := []struct {
		raw  uint32
		want time.Duration
	}{
		{win32.INFINITE, 0},
		{530, 530 * time.Millisecond},
		{1000, time.Second},
	}
	for _, c := range cases {
		raw := baseRaw()
		raw.caretBlink = c.raw
		if got := settingsFrom(raw).CaretBlink; got != c.want {
			t.Errorf("caretBlink %d: CaretBlink = %v, want %v", c.raw, got, c.want)
		}
	}
}

func TestSettingsFromCaretWidth(t *testing.T) {
	cases := []struct {
		raw  uint32
		want int
	}{
		{0, 1},
		{1, 1},
		{3, 3},
		{20, 20},
		{21, 1},
		{25, 1},
	}
	for _, c := range cases {
		raw := baseRaw()
		raw.caretWidth = c.raw
		if got := settingsFrom(raw).CaretWidth; got != c.want {
			t.Errorf("caretWidth %d: CaretWidth = %d, want %d", c.raw, got, c.want)
		}
	}
}

func TestSettingsFromHighContrast(t *testing.T) {
	cases := []struct {
		flags uint32
		want  bool
	}{
		{0, false},
		{win32.HCF_HIGHCONTRASTON, true},
		{win32.HCF_HIGHCONTRASTON | 0x2, true}, // HCF_AVAILABLE and other bits
		{0x2, false},
	}
	for _, c := range cases {
		raw := baseRaw()
		raw.highContrast = c.flags
		if got := settingsFrom(raw).HighContrast; got != c.want {
			t.Errorf("flags %#x: HighContrast = %v, want %v", c.flags, got, c.want)
		}
	}
}

func TestSettingsFromReducedMotion(t *testing.T) {
	cases := []struct {
		value int32
		want  bool
	}{
		{0, true}, // animations off
		{1, false},
	}
	for _, c := range cases {
		raw := baseRaw()
		raw.clientAnimation = c.value
		if got := settingsFrom(raw).ReducedMotion; got != c.want {
			t.Errorf("BOOL %d: ReducedMotion = %v, want %v", c.value, got, c.want)
		}
	}
}

func TestRGBOf(t *testing.T) {
	cases := []struct {
		colorref uint32 // 0x00BBGGRR
		want     uint32 // 0x00RRGGBB
	}{
		{0x00000000, 0x000000},
		{0x000000FF, 0xFF0000}, // red
		{0x0000FF00, 0x00FF00}, // green
		{0x00FF0000, 0x0000FF}, // blue
		{0x00112233, 0x332211},
	}
	for _, c := range cases {
		if got := rgbOf(c.colorref); got != c.want {
			t.Errorf("rgbOf(%#x) = %#x, want %#x", c.colorref, got, c.want)
		}
	}
}

func TestReadSettings(t *testing.T) {
	// The values depend on the user's settings, so only their ranges are checked.
	s := ReadSettings()
	if s.TextScale < 100 || s.TextScale > 225 {
		t.Errorf("TextScale = %d, want 100..225", s.TextScale)
	}
	if s.CaretWidth < 1 {
		t.Errorf("CaretWidth = %d, want at least 1", s.CaretWidth)
	}
}

func TestDefaultSettings(t *testing.T) {
	want := platform.Settings{
		HighContrast:  false,
		TextScale:     100,
		ReducedMotion: false,
		CaretBlink:    530 * time.Millisecond,
		CaretWidth:    1,
	}
	if got := platform.DefaultSettings(); got != want {
		t.Errorf("DefaultSettings() = %+v, want %+v", got, want)
	}
	// The raw values of a system with default settings map to the same thing.
	if got := settingsFrom(baseRaw()); got != want {
		t.Errorf("settingsFrom(default raw) = %+v, want %+v", got, want)
	}
}

//go:build windows

package windows

import (
	"time"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

// ReadSettings reads the operating system's accessibility and text settings.
// It only reads; a setting that cannot be read takes its DefaultSettings value.
// The application calls it again when platform.EventSettingsChanged arrives.
func ReadSettings() platform.Settings {
	return settingsFrom(readRaw())
}

// rawSettings holds the values as Windows reports them, before settingsFrom
// clamps them and turns them into a platform.Settings.
type rawSettings struct {
	highContrast    uint32                // dwFlags of SPI_GETHIGHCONTRAST
	colors          platform.SystemColors // 0x00RRGGBB, already converted from COLORREF
	clientAnimation int32                 // BOOL of SPI_GETCLIENTAREAANIMATION
	caretBlink      uint32                // GetCaretBlinkTime in ms, or INFINITE
	caretWidth      uint32                // SPI_GETCARETWIDTH in device pixels
	textScale       uint32                // TextScaleFactor, or 0 when it is missing
	appsUseLight    uint32                // AppsUseLightTheme: 0 when apps use the dark mode
}

// readRaw gathers the raw values. A read that fails leaves the value that
// DefaultSettings implies: animations on, a caret one pixel wide, and no text
// scale (0 is clamped to 100).
func readRaw() rawSettings {
	var r rawSettings
	if flags, ok := win32.HighContrastFlags(); ok {
		r.highContrast = flags
	}
	r.colors = readSystemColors()
	r.clientAnimation = 1
	if on, ok := win32.ClientAreaAnimation(); ok {
		r.clientAnimation = on
	}
	r.caretBlink = win32.CaretBlinkTime()
	if w, ok := win32.CaretWidth(); ok {
		r.caretWidth = w
	}
	// The text size is stored in the registry by the Settings app. It is only
	// read here, never written.
	if scale, ok := win32.CurrentUserDWORD(`Software\Microsoft\Accessibility`, "TextScaleFactor"); ok {
		r.textScale = scale
	}
	// The app mode (light or dark) is stored there by the Settings app, and
	// Windows broadcasts WM_SETTINGCHANGE ("ImmersiveColorSet") when it changes.
	// Without the value, apps are light, as on a new Windows installation.
	r.appsUseLight = 1
	if v, ok := win32.CurrentUserDWORD(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "AppsUseLightTheme"); ok {
		r.appsUseLight = v
	}
	return r
}

// readSystemColors reads the colors of platform.SystemColors with GetSysColor,
// using the COLOR_* indices noted on its fields.
func readSystemColors() platform.SystemColors {
	c := func(index int) uint32 { return rgbOf(win32.SysColor(index)) }
	return platform.SystemColors{
		Window:        c(5),
		WindowText:    c(8),
		Highlight:     c(13),
		HighlightText: c(14),
		HotLight:      c(26),
		GrayText:      c(17),
		ButtonFace:    c(15),
		ButtonText:    c(18),
		ButtonShadow:  c(16),
		WindowFrame:   c(6),
		CaptionText:   c(9),
		ActiveCaption: c(2),
		Menu:          c(4),
		MenuText:      c(7),
		InfoBack:      c(24),
	}
}

// rgbOf converts a COLORREF (0x00BBGGRR) to 0x00RRGGBB.
func rgbOf(colorref uint32) uint32 {
	r := colorref & 0xFF
	g := (colorref >> 8) & 0xFF
	b := (colorref >> 16) & 0xFF
	return r<<16 | g<<8 | b
}

// settingsFrom maps the raw values to a platform.Settings. It clamps the text
// scale to 100..225, the caret width to 1..20 (1 otherwise), and reads a caret
// blink of INFINITE as no blink.
func settingsFrom(raw rawSettings) platform.Settings {
	s := platform.Settings{
		HighContrast:  raw.highContrast&win32.HCF_HIGHCONTRASTON != 0,
		Colors:        raw.colors,
		ReducedMotion: raw.clientAnimation == 0,
		CaretWidth:    1,
		Dark:          raw.appsUseLight == 0,
	}
	switch {
	case raw.textScale < 100:
		s.TextScale = 100
	case raw.textScale > 225:
		s.TextScale = 225
	default:
		s.TextScale = int(raw.textScale)
	}
	if raw.caretWidth >= 1 && raw.caretWidth <= 20 {
		s.CaretWidth = int(raw.caretWidth)
	}
	if raw.caretBlink != win32.INFINITE {
		s.CaretBlink = time.Duration(raw.caretBlink) * time.Millisecond
	}
	return s
}

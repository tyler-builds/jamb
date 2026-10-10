package app

import (
	"github.com/tyler-builds/jamb/platform"
	"github.com/tyler-builds/jamb/ui"
)

// systemSettings returns the settings the runtime uses: the operating system's,
// as the platform reads them, with the development overrides applied.
func systemSettings(d devOptions) ui.SystemSettings {
	p := readPlatformSettings()
	s := ui.SystemSettings{
		HighContrast:  p.HighContrast,
		Colors:        uiColors(p.Colors),
		TextScale:     p.TextScale,
		ReducedMotion: p.ReducedMotion,
		CaretBlinkOff: p.CaretBlink == 0,
		CaretBlink:    p.CaretBlink,
		CaretWidth:    p.CaretWidth,
	}
	return d.overrideSettings(s)
}

// uiColors converts the platform's system colors, which are 0x00RRGGBB.
func uiColors(c platform.SystemColors) ui.SystemColors {
	rgb := func(v uint32) ui.Color { return ui.RGB(uint8(v>>16), uint8(v>>8), uint8(v)) }
	return ui.SystemColors{
		Window: rgb(c.Window), WindowText: rgb(c.WindowText),
		Highlight: rgb(c.Highlight), HighlightText: rgb(c.HighlightText),
		HotLight: rgb(c.HotLight), GrayText: rgb(c.GrayText),
		ButtonFace: rgb(c.ButtonFace), ButtonText: rgb(c.ButtonText),
		ButtonShadow: rgb(c.ButtonShadow), WindowFrame: rgb(c.WindowFrame),
		CaptionText: rgb(c.CaptionText), ActiveCaption: rgb(c.ActiveCaption),
		Menu: rgb(c.Menu), MenuText: rgb(c.MenuText), InfoBack: rgb(c.InfoBack),
	}
}

// applySettings gives the window's runtime the current settings, and traces them
// when JAMB_TRACE is on.
func (h *host) applySettings() {
	s := systemSettings(h.dev)
	h.rt.SetSystemSettings(s)
	h.dev.tracef("settings: text %d%%, high contrast %v, reduced motion %v, caret blink %v, caret width %d",
		s.TextScale, s.HighContrast, s.ReducedMotion, s.CaretBlink, s.CaretWidth)
}

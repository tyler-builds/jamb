package ui

import "time"

// SystemSettings are the operating system's accessibility and text settings as
// the runtime uses them. The app package reads them from the platform and
// passes them in with Runtime.SetSystemSettings, at start and whenever they
// change. The zero value is the default: 100% text, no high contrast, motion on,
// a caret that blinks every 530 ms and is 1.5 px wide.
type SystemSettings struct {
	// Dark is the system's app mode: Windows' "Choose your app mode" is Dark.
	// A runtime that follows the system (see Runtime.SetThemes) uses its dark
	// theme then, and its light theme otherwise.
	Dark bool
	// HighContrast selects the high contrast theme, built from Colors. The
	// theme is used automatically; see HighContrastTheme.
	HighContrast bool
	// Colors are the system colors the high contrast theme is built from. A
	// zero Colors uses the representative black scheme (HighContrastBlack).
	Colors SystemColors
	// TextScale is the Text size setting, in percent from 100 to 225. It scales
	// text only, not the layout or the zoom. Zero means 100.
	TextScale int
	// ReducedMotion asks for less motion: transitions, enter and exit motions
	// and smooth scrolling jump to their end, and animations stop (see
	// Animate).
	ReducedMotion bool
	// CaretBlinkOff keeps the text caret solid. CaretBlink is how long the
	// caret stays on, and then off, in each blink; zero means 530 ms.
	CaretBlinkOff bool
	// CaretBlink is how long the caret stays on, and then off, in each blink.
	CaretBlink time.Duration
	// CaretWidth scales the caret's thickness, whose default is 1.5 logical
	// pixels. Zero means 1. A display's scale is applied as for the rest of
	// the UI.
	CaretWidth int
}

// SystemColors are the Windows system colors that a high contrast theme uses
// (see GetSysColor). Each is opaque.
type SystemColors struct {
	// Window and WindowText are the system colors of window backgrounds and text.
	Window, WindowText Color
	// Highlight and HighlightText are the system colors of a selection.
	Highlight, HighlightText Color
	// HotLight and GrayText are the system colors of a hot-tracked item and of disabled text.
	HotLight, GrayText Color
	// ButtonFace and ButtonText are the system colors of buttons.
	ButtonFace, ButtonText Color
	// ButtonShadow and WindowFrame are the system colors of a button's shadow and of a window frame.
	ButtonShadow, WindowFrame Color
	// CaptionText and ActiveCaption are the system colors of a title bar.
	CaptionText, ActiveCaption Color
	// Menu, MenuText and InfoBack are the system colors of menus and of tooltips.
	Menu, MenuText, InfoBack Color
}

const (
	defaultCaretBlink = 530 * time.Millisecond
	defaultCaretWidth = 1.5 // logical pixels, at a CaretWidth of 1
)

// SetSystemSettings applies the operating system's settings. Every component
// is rendered again, so that a change of theme or text size shows at once.
func (rt *Runtime) SetSystemSettings(s SystemSettings) {
	if s.TextScale <= 0 {
		s.TextScale = 100
	}
	s.TextScale = min(max(s.TextScale, 100), 225)
	if s.CaretBlink <= 0 {
		s.CaretBlink = defaultCaretBlink
	}
	if s.CaretWidth <= 0 {
		s.CaretWidth = 1
	}
	if s.HighContrast && s.Colors.isZero() {
		s.Colors = HighContrastBlack()
	}
	rt.sys = s
	rt.textFactor = float32(s.TextScale) / 100
	rt.caretWidth = defaultCaretWidth * float32(s.CaretWidth)
	rt.applyTheme()
	rt.hostInvalidateAll()
}

// SystemSettings returns the settings the runtime uses, normalized.
func (rt *Runtime) SystemSettings() SystemSettings { return rt.sys }

// HighContrast reports whether the high contrast theme is in use.
func (rt *Runtime) HighContrast() bool { return rt.sys.HighContrast }

// ReducedMotion reports whether the user asked for less motion.
func (rt *Runtime) ReducedMotion() bool { return rt.sys.ReducedMotion }

// TextScale returns the Text size setting in percent, from 100 to 225.
func (rt *Runtime) TextScale() int { return rt.sys.TextScale }

// isZero reports whether no color is set.
func (c SystemColors) isZero() bool { return c == SystemColors{} }

package ui

import (
	"time"

	"github.com/tyler-builds/jamb/access"
)

// SwitchElement is an on/off switch bound to a bool signal. It is drawn as a
// track and a knob, without text: its label is what assistive technology reads,
// and the text beside it is drawn by the app.
type SwitchElement struct {
	label    string
	on       *Signal[bool]
	key      string
	disabled bool
	onChange func(bool)
}

// Switch creates a switch that is on while on is true. label names the switch
// for assistive technology.
func Switch(label string, on *Signal[bool]) *SwitchElement {
	return &SwitchElement{label: label, on: on}
}

func (*SwitchElement) isElement()           {}
func (s *SwitchElement) elementKey() string { return s.key }

func (s *SwitchElement) Key(k string) *SwitchElement { s.key = k; return s }

// Disabled shows the switch but does not let it change.
func (s *SwitchElement) Disabled(d bool) *SwitchElement { s.disabled = d; return s }

// OnChange runs fn with the new state after the user changes the switch. The
// signal is set first.
func (s *SwitchElement) OnChange(fn func(bool)) *SwitchElement { s.onChange = fn; return s }

func (s *SwitchElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	on := s.on.Get()
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	track, knob, knobX := t.SurfaceActive, Hex("#ffffff"), float32(3)
	if on {
		track, knobX = t.Accent, 19
	}
	d := Div().Size(Px(38), Px(22)).Shrink(0).Radius(11).Background(track).
		Role(access.RoleSwitch).Label(s.label).Checked(on)
	d.spaceOnly = true // Enter goes on to the modal default button, as on Windows
	k := Div().Absolute().Left(knobX).Top(3).Size(Px(16), Px(16)).Radius(8).Background(knob)
	if t.HighContrast {
		// On is a solid Highlight track with an AccentText knob. Off is an outlined
		// empty track with a text-colored knob, so the two states differ in color
		// and in outline, not only in the knob's place. A disabled switch is drawn in
		// GrayText, not dimmed: its outline and its knob are GrayText, and an on
		// track is filled with GrayText.
		track, knob, edge := t.Surface, t.Text, t.Text
		if on {
			track, knob = t.Accent, t.AccentText
		}
		if s.disabled {
			edge, knob = t.Disabled, t.Disabled
			if on {
				track, knob = t.Disabled, t.Surface
			} else {
				track = t.Surface
			}
		}
		d.Background(track).Border(2, edge)
		k.Background(knob)
	} else {
		d.Transition(120 * time.Millisecond)
		k.Shadow(ShadowSm)
	}
	d.Child(k)
	if s.disabled {
		d.Cursor(CursorNotAllowed)
		d.access().disabled = true
		if !t.HighContrast {
			d.Opacity(0.5)
		}
		return d
	}
	if !t.HighContrast {
		d.Hover(Style{Background: track.Lighten(0.08)})
	}
	return d.Cursor(CursorPointer).Focusable().FocusVisible(Style{Shadows: []Shadow{ring}}).
		OnClick(func() {
			s.on.Set(!on)
			if s.onChange != nil {
				s.onChange(!on)
			}
		})
}

package ui

import (
	"time"

	"github.com/tyler-builds/jamb/access"
)

// SwitchStyle is the look of each part of a switch. A part's style merges over
// the built-in defaults, then over the theme's Switch, then over the Styles of
// the instance; see docs/styling.md. Track is the pill, and Knob the circle that
// moves along it. The position of the knob follows the state and is not a style:
// a Knob style sets its color, border, shadow and size. The states are parts of
// their own: TrackChecked when the switch is on, TrackHover while the pointer is
// over an enabled switch, TrackFocusVisible while it has keyboard focus,
// TrackDisabled and KnobDisabled when it cannot change, and KnobChecked for the
// knob of an on switch.
type SwitchStyle struct {
	Track             Style // the track: its size, shape, fill and border
	TrackChecked      Style // the track when the switch is on
	TrackHover        Style // the track while the pointer is over an enabled switch
	TrackFocusVisible Style // the track while it has keyboard focus
	TrackDisabled     Style // the track when the switch is disabled
	Knob              Style // the knob: its size, color and shadow
	KnobChecked       Style // the knob when the switch is on
	KnobDisabled      Style // the knob when the switch is disabled
}

// merge returns s with the set fields of o applied on top, part by part.
func (s SwitchStyle) merge(o SwitchStyle) SwitchStyle {
	return SwitchStyle{
		Track:             s.Track.Merge(o.Track),
		TrackChecked:      s.TrackChecked.Merge(o.TrackChecked),
		TrackHover:        s.TrackHover.Merge(o.TrackHover),
		TrackFocusVisible: s.TrackFocusVisible.Merge(o.TrackFocusVisible),
		TrackDisabled:     s.TrackDisabled.Merge(o.TrackDisabled),
		Knob:              s.Knob.Merge(o.Knob),
		KnobChecked:       s.KnobChecked.Merge(o.KnobChecked),
		KnobDisabled:      s.KnobDisabled.Merge(o.KnobDisabled),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s SwitchStyle) plain() SwitchStyle {
	return SwitchStyle{
		Track:             plainStyle(s.Track),
		TrackChecked:      plainStyle(s.TrackChecked),
		TrackHover:        plainStyle(s.TrackHover),
		TrackFocusVisible: plainStyle(s.TrackFocusVisible),
		TrackDisabled:     plainStyle(s.TrackDisabled),
		Knob:              plainStyle(s.Knob),
		KnobChecked:       plainStyle(s.KnobChecked),
		KnobDisabled:      plainStyle(s.KnobDisabled),
	}
}

// switchDefaults returns the built-in look of a switch that is on or off and
// disabled or not, from the colors of t. Under high contrast the track is an
// outlined empty track, or a solid Highlight track when on, with the knob in the
// text color (AccentText when on); a disabled switch is GrayText and not dimmed.
func switchDefaults(t *Theme, on, disabled bool) SwitchStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := SwitchStyle{
		Track: Style{Width: Px(38), Height: Px(22), NoShrink: true, Radius: 11,
			Background: t.SurfaceActive},
		TrackChecked:      Style{Background: t.Accent},
		TrackFocusVisible: Style{Shadows: []Shadow{ring}},
		Knob: Style{Absolute: true, Width: Px(16), Height: Px(16), NoShrink: true, Radius: 8,
			Background: Hex("#ffffff")},
	}
	if t.HighContrast {
		d.Track.Background, d.Track.BorderWidth, d.Track.BorderColor = t.Surface, 2, t.Text
		d.TrackChecked = Style{Background: t.Accent}
		d.Knob.Background = t.Text
		d.KnobChecked = Style{Background: t.AccentText}
	} else {
		d.Track.Transition = 120 * time.Millisecond
		d.Knob.Shadows = []Shadow{ShadowSm}
		hover := t.SurfaceActive
		if on {
			hover = t.Accent
		}
		d.TrackHover = Style{Background: hover.Lighten(0.08)}
	}
	if disabled {
		if t.HighContrast {
			d.TrackDisabled = Style{BorderColor: t.Disabled}
			d.KnobDisabled = Style{Background: t.Disabled}
			if on {
				d.TrackDisabled.Background = t.Disabled
				d.KnobDisabled.Background = t.Surface
			}
		} else {
			d.TrackDisabled = Style{Opacity: 0.5}
		}
	}
	return d
}

// resolveSwitch returns the look of a switch: the defaults for its state, merged
// with the theme's Switch and then the instance's Styles. Under high contrast the
// two overrides are plain (see plainStyle).
func resolveSwitch(ctx *Context, on, disabled bool, inst SwitchStyle) SwitchStyle {
	t := ctx.Theme()
	return layered(ctx, switchDefaults(t, on, disabled), SwitchStyle.merge, SwitchStyle.plain,
		themeLayer(t.Switch), instanceLayer(inst))
}

// SwitchElement is an on/off switch bound to a bool signal. It is drawn as a
// track and a knob, without text: its label is what assistive technology reads,
// and the text beside it is drawn by the app.
type SwitchElement struct {
	label       string
	on          *Signal[bool]
	key         string
	disabled    bool
	onChange    func(bool)
	styles      SwitchStyle
	size        sizing       // the track's sizes, from the sizing methods (see Sizing in docs/styling.md)
	focusHandle *FocusHandle // see FocusHandle
	tooltip     string       // see Tooltip
}

// Tooltip shows s below the switch after the pointer rests on it. Assistive
// technology reads it as the switch's description, unless it has another.
func (s *SwitchElement) Tooltip(text string) *SwitchElement {
	s.tooltip = text
	return s
}

// FocusHandle lets code focus the switch (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (s *SwitchElement) FocusHandle(h *FocusHandle) *SwitchElement {
	s.focusHandle = h
	return s
}

// Switch creates a switch that is on while on is true. label names the switch
// for assistive technology.
func Switch(label string, on *Signal[bool]) *SwitchElement {
	return &SwitchElement{label: label, on: on}
}

func (*SwitchElement) isElement()           {}
func (s *SwitchElement) elementKey() string { return s.key }

// Key identifies the switch among its siblings (see DivElement.Key).
func (s *SwitchElement) Key(k string) *SwitchElement { s.key = k; return s }

// Disabled shows the switch but does not let it change.
func (s *SwitchElement) Disabled(d bool) *SwitchElement { s.disabled = d; return s }

// OnChange runs fn with the new state after the user changes the switch. The
// signal is set first.
func (s *SwitchElement) OnChange(fn func(bool)) *SwitchElement { s.onChange = fn; return s }

// The sizing methods give the track a width. The track is 38 px wide by default, and
// the knob stays 3 px from the end it is at. See Sizing in docs/styling.md.
func (s *SwitchElement) Width(l Length) *SwitchElement { s.size.setWidth(l); return s }

// MinWidth sets the smallest width of the switch.
func (s *SwitchElement) MinWidth(l Length) *SwitchElement { s.size.setMinWidth(l); return s }

// MaxWidth sets the largest width of the switch.
func (s *SwitchElement) MaxWidth(l Length) *SwitchElement { s.size.setMaxWidth(l); return s }

// Grow makes the switch take a share of the free space on its parent's main axis.
func (s *SwitchElement) Grow(f float32) *SwitchElement { s.size.setGrow(f); return s }

// GridSpan sets how many columns and rows the switch covers in its parent's grid.
func (s *SwitchElement) GridSpan(cols, rows int) *SwitchElement { s.size.setSpan(cols, rows); return s }

// Styles sets the look of the parts of the switch, for this instance only. Set
// fields win over the theme's Switch and the defaults; see SwitchStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (s *SwitchElement) Styles(st SwitchStyle) *SwitchElement {
	s.styles = s.styles.merge(st)
	return s
}

func (s *SwitchElement) expand(ctx *Context) Element {
	on := s.on.Get()
	st := resolveSwitch(ctx, on, s.disabled, s.styles)
	d := Div().asPart("Switch.Track").Style(st.Track).
		Role(access.RoleSwitch).Label(s.label).AccessChecked(on)
	d.spaceOnly = true // Enter goes on to the modal default button, as on Windows
	if on {
		d.Style(st.TrackChecked)
	}
	// The knob sits 3 px in from the end it is at, so the track can be any width.
	k := Div().asPart("Switch.Knob").Style(st.Knob).Absolute().Top(3)
	if on {
		k.Right(3).Style(st.KnobChecked)
	} else {
		k.Left(3)
	}
	if s.disabled {
		k.Style(st.KnobDisabled)
	}
	d.Child(k)
	// A tooltip also shows on a disabled switch, to say why it is disabled.
	d.Tooltip(s.tooltip)
	if s.disabled {
		d.Style(st.TrackDisabled).Cursor(CursorNotAllowed)
		d.access().disabled = true
		return s.size.apply(d)
	}
	d.Hover(st.TrackHover)
	if s.focusHandle != nil {
		d.FocusHandle(s.focusHandle)
	}
	return s.size.apply(d.Cursor(CursorPointer).Focusable().FocusVisible(st.TrackFocusVisible).
		OnClick(func() {
			s.on.Set(!on)
			if s.onChange != nil {
				s.onChange(!on)
			}
		}))
}

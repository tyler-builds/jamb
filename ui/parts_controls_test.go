package ui

import (
	"strings"
	"testing"
)

// partCheckbox is a check box in state s, disabled or not, with the instance
// style st. It is made by Checkbox for an on or off box and by CheckboxTri for
// any state, as the widget's two constructors are; tri selects CheckboxTri.
func partCheckbox(tri bool, s CheckState, st CheckboxStyle, disabled bool) Element {
	if !tri {
		return Div().Column().Child(Checkbox("Include", NewSignal(s == CheckOn)).Key("inc").Disabled(disabled).Styles(st))
	}
	sig := NewSignal(CheckOff)
	sig.Set(s)
	return Div().Column().Child(CheckboxTri("Include", sig).Key("inc").Disabled(disabled).Styles(st))
}

func testCheckboxParts(t *testing.T, applied map[string]bool) {
	type checkCase struct {
		part     string
		set      func(*CheckboxStyle, Style)
		state    CheckState
		disabled bool
		drawn    string // the element that draws the part
		text     bool
		act      func(*testing.T, *accessRig, *node)
	}
	hoverBox := func(t *testing.T, r *accessRig, _ *node) { hoverNode(r, partNode(t, r.rt, "Checkbox.Box", "")) }
	cases := []checkCase{
		{part: "Checkbox.Control", set: func(s *CheckboxStyle, m Style) { s.Control = m }, drawn: "Checkbox.Control"},
		{part: "Checkbox.ControlFocusVisible", set: func(s *CheckboxStyle, m Style) { s.ControlFocusVisible = m }, drawn: "Checkbox.Control",
			act: func(t *testing.T, r *accessRig, n *node) { focusNode(r, n) }},
		{part: "Checkbox.ControlDisabled", set: func(s *CheckboxStyle, m Style) { s.ControlDisabled = m }, disabled: true, drawn: "Checkbox.Control"},
		{part: "Checkbox.Box", set: func(s *CheckboxStyle, m Style) { s.Box = m }, drawn: "Checkbox.Box"},
		{part: "Checkbox.BoxHover", set: func(s *CheckboxStyle, m Style) { s.BoxHover = m }, drawn: "Checkbox.Box", act: hoverBox},
		{part: "Checkbox.BoxChecked", set: func(s *CheckboxStyle, m Style) { s.BoxChecked = m }, state: CheckOn, drawn: "Checkbox.Box"},
		{part: "Checkbox.BoxMixed", set: func(s *CheckboxStyle, m Style) { s.BoxMixed = m }, state: CheckMixed, drawn: "Checkbox.Box"},
		{part: "Checkbox.BoxDisabled", set: func(s *CheckboxStyle, m Style) { s.BoxDisabled = m }, disabled: true, drawn: "Checkbox.Box"},
		{part: "Checkbox.Mark", set: func(s *CheckboxStyle, m Style) { s.Mark = m }, state: CheckOn, drawn: "Checkbox.Mark", text: true},
		{part: "Checkbox.Dash", set: func(s *CheckboxStyle, m Style) { s.Dash = m }, state: CheckMixed, drawn: "Checkbox.Dash", text: true},
		{part: "Checkbox.Label", set: func(s *CheckboxStyle, m Style) { s.Label = m }, drawn: "Checkbox.Label", text: true},
		{part: "Checkbox.LabelDisabled", set: func(s *CheckboxStyle, m Style) { s.LabelDisabled = m }, disabled: true, drawn: "Checkbox.Label", text: true},
	}
	// CheckboxTri shares the parts of Checkbox, so each case runs for both.
	for _, c := range cases {
		for _, w := range []string{"Checkbox", "CheckboxTri"} {
			tri := w == "CheckboxTri"
			name := w + strings.TrimPrefix(c.part, "Checkbox")
			applied[name] = true
			if !tri && c.state == CheckMixed {
				continue // Checkbox has no mixed state; CheckboxTri draws the part
			}
			t.Run(name, func(t *testing.T) {
				var st CheckboxStyle
				c.set(&st, Style{Background: markBG, TextColor: markFG})
				r := newAccessRig(func(ctx *Context) Element { return partCheckbox(tri, c.state, st, c.disabled) })
				n := partNode(t, r.rt, c.drawn, "")
				if c.act != nil {
					c.act(t, r, n)
				}
				n = partNode(t, r.rt, c.drawn, "")
				if c.text {
					wantTextColor(t, n, name, markFG)
					return
				}
				wantBackground(t, n, name, markBG)
			})
		}
	}
}

// partRadio is a radio group of one option, selected or not, disabled or not,
// with the instance style st.
func partRadio(selected bool, st RadioGroupStyle, disabled bool) Element {
	sig := NewSignal("b")
	if selected {
		sig.Set("a")
	}
	return Div().Column().Child(RadioGroup(sig, Option[string]{Value: "a", Label: "Alpha", Disabled: disabled}).
		Key("freq").Label("Frequency").Disabled(false).Styles(st))
}

func testRadioGroupParts(t *testing.T, applied map[string]bool) {
	type radioCase struct {
		part     string
		set      func(*RadioGroupStyle, Style)
		selected bool
		disabled bool
		drawn    string
		text     bool
		act      func(*testing.T, *accessRig, *node)
	}
	cases := []radioCase{
		{part: "RadioGroup.Group", set: func(s *RadioGroupStyle, m Style) { s.Group = m }, drawn: "RadioGroup.Group"},
		{part: "RadioGroup.Radio", set: func(s *RadioGroupStyle, m Style) { s.Radio = m }, drawn: "RadioGroup.Radio"},
		{part: "RadioGroup.RadioFocusVisible", set: func(s *RadioGroupStyle, m Style) { s.RadioFocusVisible = m }, drawn: "RadioGroup.Radio",
			act: func(t *testing.T, r *accessRig, n *node) { focusNode(r, n) }},
		{part: "RadioGroup.RadioDisabled", set: func(s *RadioGroupStyle, m Style) { s.RadioDisabled = m }, disabled: true, drawn: "RadioGroup.Radio"},
		{part: "RadioGroup.Ring", set: func(s *RadioGroupStyle, m Style) { s.Ring = m }, drawn: "RadioGroup.Ring"},
		{part: "RadioGroup.RingChecked", set: func(s *RadioGroupStyle, m Style) { s.RingChecked = m }, selected: true, drawn: "RadioGroup.Ring"},
		{part: "RadioGroup.RingDisabled", set: func(s *RadioGroupStyle, m Style) { s.RingDisabled = m }, disabled: true, drawn: "RadioGroup.Ring"},
		{part: "RadioGroup.Dot", set: func(s *RadioGroupStyle, m Style) { s.Dot = m }, selected: true, drawn: "RadioGroup.Dot"},
		{part: "RadioGroup.DotDisabled", set: func(s *RadioGroupStyle, m Style) { s.DotDisabled = m }, selected: true, disabled: true, drawn: "RadioGroup.Dot"},
		{part: "RadioGroup.Label", set: func(s *RadioGroupStyle, m Style) { s.Label = m }, drawn: "RadioGroup.Label", text: true},
		{part: "RadioGroup.LabelDisabled", set: func(s *RadioGroupStyle, m Style) { s.LabelDisabled = m }, disabled: true, drawn: "RadioGroup.Label", text: true},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st RadioGroupStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return partRadio(c.selected, st, c.disabled) })
			n := partNode(t, r.rt, c.drawn, "")
			if c.act != nil {
				c.act(t, r, n)
			}
			n = partNode(t, r.rt, c.drawn, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

// partSwitch is a switch, on or off, disabled or not, with the instance style st.
func partSwitch(on bool, st SwitchStyle, disabled bool) Element {
	sig := NewSignal(on)
	return Div().Column().Child(Switch("Notifications", sig).Key("sw").Disabled(disabled).Styles(st))
}

func testSwitchParts(t *testing.T, applied map[string]bool) {
	type switchCase struct {
		part     string
		set      func(*SwitchStyle, Style)
		on       bool
		disabled bool
		drawn    string
		act      func(*testing.T, *accessRig, *node)
	}
	cases := []switchCase{
		{part: "Switch.Track", set: func(s *SwitchStyle, m Style) { s.Track = m }, drawn: "Switch.Track"},
		{part: "Switch.TrackChecked", set: func(s *SwitchStyle, m Style) { s.TrackChecked = m }, on: true, drawn: "Switch.Track"},
		{part: "Switch.TrackHover", set: func(s *SwitchStyle, m Style) { s.TrackHover = m }, drawn: "Switch.Track",
			act: func(t *testing.T, r *accessRig, _ *node) { hoverNode(r, partNode(t, r.rt, "Switch.Track", "")) }},
		{part: "Switch.TrackFocusVisible", set: func(s *SwitchStyle, m Style) { s.TrackFocusVisible = m }, drawn: "Switch.Track",
			act: func(t *testing.T, r *accessRig, n *node) { focusNode(r, n) }},
		{part: "Switch.TrackDisabled", set: func(s *SwitchStyle, m Style) { s.TrackDisabled = m }, disabled: true, drawn: "Switch.Track"},
		{part: "Switch.Knob", set: func(s *SwitchStyle, m Style) { s.Knob = m }, drawn: "Switch.Knob"},
		{part: "Switch.KnobChecked", set: func(s *SwitchStyle, m Style) { s.KnobChecked = m }, on: true, drawn: "Switch.Knob"},
		{part: "Switch.KnobDisabled", set: func(s *SwitchStyle, m Style) { s.KnobDisabled = m }, disabled: true, drawn: "Switch.Knob"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st SwitchStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return partSwitch(c.on, st, c.disabled) })
			n := partNode(t, r.rt, c.drawn, "")
			if c.act != nil {
				c.act(t, r, n)
			}
			wantBackground(t, partNode(t, r.rt, c.drawn, ""), c.part, markBG)
		})
	}
}

// TestControlsUnderHighContrast checks the guardrails for the check box, the
// radio group and the switch: an app color does not reach their boxes, rings
// and tracks, and the focus ring of the control stays.
func TestControlsUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Checkbox = CheckboxStyle{Box: Style{Background: markBG, Padding: All(3)}}
	theme.RadioGroup = RadioGroupStyle{Ring: Style{Background: markBG}}
	theme.Switch = SwitchStyle{Track: Style{Background: markBG}}
	sig := NewSignal(CheckOn)
	on := NewSignal(true)
	rg := NewSignal("a")
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Div().Column().Child(CheckboxTri("Include", sig).Key("inc")).
			Child(RadioGroup(rg, Option[string]{Value: "a", Label: "Alpha"}).Key("freq")).
			Child(Switch("Notifications", on).Key("sw"))
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	if n := partNode(t, r.rt, "Checkbox.Box", ""); n.style.Background == markBG {
		t.Errorf("high contrast: the check box took an app color")
	} else if n.style.Padding != All(3) {
		t.Errorf("high contrast: the check box's padding %v did not apply", n.style.Padding)
	}
	if n := partNode(t, r.rt, "RadioGroup.Ring", ""); n.style.Background == markBG {
		t.Errorf("high contrast: the radio ring took an app color")
	}
	if n := partNode(t, r.rt, "Switch.Track", ""); n.style.Background == markBG || n.style.Background != th.Accent {
		t.Errorf("high contrast: the on switch track is %v, want Highlight %v", n.style.Background, th.Accent)
	}
	focusNode(r, partNode(t, r.rt, "Checkbox.Control", ""))
	if len(partNode(t, r.rt, "Checkbox.Control", "").style.Shadows) == 0 {
		t.Errorf("high contrast: the focused check box has no focus ring")
	}
}

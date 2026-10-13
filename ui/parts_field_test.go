package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// partTextInput is a text field with the instance style st.
func partTextInput(sig *Signal[string], st FieldStyle) Element {
	return Div().Column().Child(TextInput(sig).Label("Name").Placeholder("Your name").Styles(st))
}

// partField is a text field of widget ("TextInput" or "TextArea") with the
// instance style st. With slots, it has a Leading and a Trailing element, and a
// TextInput also has an icon before its text. state is "", "disabled" or
// "readonly".
func partField(widget string, sig *Signal[string], st FieldStyle, slots bool, state string) Element {
	if widget == "TextArea" {
		ta := TextArea(sig).Label("Notes").Placeholder("Write here").Styles(st).
			Disabled(state == "disabled").ReadOnly(state == "readonly")
		if slots {
			ta.Leading(Text("lead")).Trailing(Text("trail"))
		}
		return Div().Column().Child(ta)
	}
	ti := TextInput(sig).Label("Name").Placeholder("Your name").Styles(st).
		Disabled(state == "disabled").ReadOnly(state == "readonly")
	if slots {
		ti.Icon(icons.Check).Leading(Text("lead")).Trailing(Text("trail"))
	}
	return Div().Column().Child(ti)
}

// testFieldParts sets a distinctive style on each part of a text field of
// widget, puts the part's state in effect, and asserts that the style reaches
// the node that draws it. The Text, Placeholder and Selection parts are read
// from the text editor, which draws them.
func testFieldParts(t *testing.T, widget string, applied map[string]bool) {
	type fieldCase struct {
		part  string // the part's name, without the widget
		set   func(*FieldStyle, Style)
		slots bool   // the field has a Leading and a Trailing element
		state string // "disabled" or "readonly" builds the field in that state
		act   func(*testing.T, *accessRig)
	}
	cases := []fieldCase{
		{part: "Frame", set: func(s *FieldStyle, m Style) { s.Frame = m }},
		{part: "FrameHover", set: func(s *FieldStyle, m Style) { s.FrameHover = m },
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, widget+".Frame", "")) }},
		{part: "FrameFocusWithin", set: func(s *FieldStyle, m Style) { s.FrameFocusWithin = m },
			act: func(t *testing.T, r *accessRig) { focusNode(r, editorNode(t, r.rt)) }},
		{part: "FrameDisabled", set: func(s *FieldStyle, m Style) { s.FrameDisabled = m }, state: "disabled"},
		{part: "FrameReadOnly", set: func(s *FieldStyle, m Style) { s.FrameReadOnly = m }, state: "readonly"},
		{part: "Text", set: func(s *FieldStyle, m Style) { s.Text = m }},
		{part: "Placeholder", set: func(s *FieldStyle, m Style) { s.Placeholder = m }},
		{part: "Selection", set: func(s *FieldStyle, m Style) { s.Selection = m }},
		{part: "Leading", set: func(s *FieldStyle, m Style) { s.Leading = m }, slots: true},
		{part: "Trailing", set: func(s *FieldStyle, m Style) { s.Trailing = m }, slots: true},
	}
	for _, c := range cases {
		name := widget + "." + c.part
		applied[name] = true
		t.Run(name, func(t *testing.T) {
			var st FieldStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return partField(widget, NewSignal(""), st, c.slots, c.state) })
			if c.act != nil {
				c.act(t, r)
			}
			switch c.part {
			case "Text":
				wantTextColor(t, editorNode(t, r.rt), name, markFG)
			case "Placeholder":
				if got := editorNode(t, r.rt).editor.placeholderColor; got != markFG {
					t.Errorf("%s: placeholder color %v, want %v", name, got, markFG)
				}
			case "Selection":
				if got := editorNode(t, r.rt).editor.selection; got != markBG {
					t.Errorf("%s: selection color %v, want %v", name, got, markBG)
				}
			default:
				wantBackground(t, partNode(t, r.rt, drawnBy(name), ""), name, markBG)
			}
		})
	}
}

// TestFieldOverridesUnderHighContrast checks the guardrails of the text frame:
// under high contrast an app's colors for the frame, the text and the invalid
// frame of a number input are not used, while its padding and the frame's border
// are, the focus is drawn in Highlight, and a disabled number is GrayText.
func TestFieldOverridesUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Field = FieldStyle{Frame: Style{Background: markBG, Padding: All(7)}, Text: Style{TextColor: markFG}}
	theme.NumberInput.FrameInvalid = Style{BorderColor: markBG, BorderWidth: 4}

	sig := NewSignal("x")
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return partTextInput(sig, FieldStyle{Frame: Style{Padding: All(9)}})
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	frame := partNode(t, r.rt, "TextInput.Frame", "")
	if frame.style.Background != th.Surface {
		t.Errorf("high contrast: the frame's background is %v, want the system Window %v", frame.style.Background, th.Surface)
	}
	if frame.style.Padding != All(9) {
		t.Errorf("high contrast: the instance's padding %v did not apply, got %v", All(9), frame.style.Padding)
	}
	if frame.style.BorderWidth != 1 || frame.style.BorderColor != th.BorderStrong {
		t.Errorf("high contrast: the frame's border is %v in %v, want 1 in %v", frame.style.BorderWidth, frame.style.BorderColor, th.BorderStrong)
	}
	if got := editorNode(t, r.rt).style.TextColor; got != th.Text {
		t.Errorf("high contrast: the text is %v, want the system WindowText %v", got, th.Text)
	}
	focusNode(r, editorNode(t, r.rt))
	if got := partNode(t, r.rt, "TextInput.Frame", "").style.BorderColor; got != th.Accent {
		t.Errorf("high contrast: the focused frame's border is %v, want Highlight %v", got, th.Accent)
	}

	// An invalid number input keeps the border width the app gives it, and its
	// color is the text color. The focus of the field draws its frame in
	// Highlight, as it does without high contrast, so the color is checked on the
	// resolved style, which the paint uses.
	nv := NewSignal(8)
	r = newThemedRig(&theme, func(ctx *Context) Element { return partNumberInput(nv, NumberInputStyle{}, SpinInline, false) })
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th = r.rt.theme
	if got := resolveNumberInput(th, SpinInline, 1, false, false, false, NumberInputStyle{}).FrameInvalid.BorderColor; got != th.Text {
		t.Errorf("high contrast: the invalid frame is drawn in %v, want the text color %v", got, th.Text)
	}
	focusNode(r, r.keyed(t, "nf"))
	r.typeText("x")
	r.press(input.KeyEnter, 0)
	if got := partNode(t, r.rt, "NumberInput.Frame", "").style.BorderWidth; got != 4 {
		t.Errorf("high contrast: the invalid frame is %v px wide, want the app's 4", got)
	}

	// A disabled number input is GrayText, not dimmed, and its text is too.
	r = newThemedRig(&theme, func(ctx *Context) Element { return partNumberInput(nv, NumberInputStyle{}, SpinInline, true) })
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th = r.rt.theme
	if got := effectiveTextColor(partNode(t, r.rt, "NumberInput.Text", "")); got != th.Disabled {
		t.Errorf("high contrast: the disabled number's text is %v, want GrayText %v", got, th.Disabled)
	}
	if got := partNode(t, r.rt, "NumberInput.Frame", "").style.Opacity; got != 0 {
		t.Errorf("high contrast: the disabled frame is dimmed to %v, want no dimming", got)
	}
}

// TestFieldThemeLayers checks the layers of a number input's frame: the built-in
// defaults, then the theme's Field, then the theme's NumberInput, then the
// instance's Styles. A text field takes the theme's Field, which the number
// field takes too.
func TestFieldThemeLayers(t *testing.T) {
	fromField, fromNumber, fromInstance := Hex("#101112"), Hex("#131415"), Hex("#161718")
	for _, tc := range []struct {
		name  string
		field Style // the theme's Field frame
		num   Style // the theme's NumberInput frame
		inst  Style // the instance's frame
		want  Color
	}{
		{"the defaults", Style{}, Style{}, Style{}, DarkTheme().Surface},
		{"the theme's Field", Style{Background: fromField}, Style{}, Style{}, fromField},
		{"the theme's NumberInput over Field", Style{Background: fromField}, Style{Background: fromNumber}, Style{}, fromNumber},
		{"the instance over both", Style{Background: fromField}, Style{Background: fromNumber}, Style{Background: fromInstance}, fromInstance},
	} {
		t.Run(tc.name, func(t *testing.T) {
			theme := *DarkTheme()
			theme.Field.Frame = tc.field
			theme.NumberInput.Frame = tc.num
			r := newThemedRig(&theme, func(ctx *Context) Element {
				return partNumberInput(NewSignal(8), NumberInputStyle{FieldStyle: FieldStyle{Frame: tc.inst}}, SpinInline, false)
			})
			if got := partNode(t, r.rt, "NumberInput.Frame", "").style.Background; got != tc.want {
				t.Errorf("the frame's background is %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBorderlessFieldDrawsNoFrame checks that a borderless field draws no Frame
// part, and that its icon and its Leading element are still drawn.
func TestBorderlessFieldDrawsNoFrame(t *testing.T) {
	sig := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(TextInput(sig).Label("Title").Borderless().Icon(icons.Check).Leading(Text("lead")))
	})
	if got := len(partNodes(r.rt, "TextInput.Frame")); got != 0 {
		t.Errorf("a borderless field draws %d Frame parts, want none", got)
	}
	partNode(t, r.rt, "TextInput.Leading", "")
}

// TestTextAreaMinRowsFollowTextScale checks that the minimum height of a text
// area, in rows, grows with the Text size setting.
func TestTextAreaMinRowsFollowTextScale(t *testing.T) {
	height := func(scale int) float32 {
		r := newAccessRig(func(ctx *Context) Element { return partField("TextArea", NewSignal(""), FieldStyle{}, false, "") })
		if scale != 100 {
			r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
			r.frame()
		}
		return partNode(t, r.rt, "TextArea.Text", "").lay.H
	}
	if base, big := height(100), height(225); big <= base {
		t.Errorf("the text area is %v px tall at 225%%, %v px at 100%%: it does not grow", big, base)
	}
}

// TestFieldSelectionUnderHighContrast checks that the selected text of a text
// frame is the Highlight color with HighlightText on it, and that the normal
// theme's selection keeps its text color.
func TestFieldSelectionUnderHighContrast(t *testing.T) {
	hc := HighContrastTheme(HighContrastBlack(), DarkTheme())
	sel := fieldDefaults(hc, fieldLayout{}).Selection
	if sel.Background != hc.Accent || sel.TextColor != hc.AccentText {
		t.Errorf("high contrast selection: background %v text %v, want Highlight %v with HighlightText %v",
			sel.Background, sel.TextColor, hc.Accent, hc.AccentText)
	}
	if got := fieldDefaults(DarkTheme(), fieldLayout{}).Selection.TextColor; !got.IsZero() {
		t.Errorf("normal selection sets a text color %v, want none", got)
	}
}

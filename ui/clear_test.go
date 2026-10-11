package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// fullStyle sets every field that a Clear constant removes.
func fullStyle() Style {
	return Style{
		Background:  Hex("#123456"),
		Gradient:    LinearGradient(90, Hex("#000000"), Hex("#ffffff")),
		TextColor:   Hex("#abcdef"),
		BorderWidth: 2,
		BorderColor: Hex("#fedcba"),
		Radius:      9,
		Corners:     Corners{TopLeft: 1, TopRight: 2, BottomRight: 3, BottomLeft: 4},
		Shadows:     []Shadow{ShadowMd},
		Padding:     All(7),
		Margin:      All(5),
		Gap:         6,
	}
}

// TestClearRemovesEachDefault checks that each Clear constant removes the fields
// it names, and only those, so that an override can turn a default off where the
// zero value could not.
func TestClearRemovesEachDefault(t *testing.T) {
	full := fullStyle()
	cases := []struct {
		name  string
		clear ClearSet
		gone  func(Style) bool
		kept  func(Style) bool
	}{
		{"ClearBackground", ClearBackground, func(s Style) bool { return s.Background.IsZero() && s.Gradient.isZero() },
			func(s Style) bool { return s.BorderWidth != 0 }},
		{"ClearTextColor", ClearTextColor, func(s Style) bool { return s.TextColor.IsZero() },
			func(s Style) bool { return !s.Background.IsZero() }},
		{"ClearBorder", ClearBorder, func(s Style) bool { return s.BorderWidth == 0 && s.BorderColor.IsZero() },
			func(s Style) bool { return s.Radius != 0 }},
		{"ClearRadius", ClearRadius, func(s Style) bool { return s.Radius == 0 && s.Corners.isZero() },
			func(s Style) bool { return s.BorderWidth != 0 }},
		{"ClearShadows", ClearShadows, func(s Style) bool { return s.Shadows == nil },
			func(s Style) bool { return s.Radius != 0 }},
		{"ClearPadding", ClearPadding, func(s Style) bool { return s.Padding.isZero() },
			func(s Style) bool { return s.Margin != Edges{} }},
		{"ClearMargin", ClearMargin, func(s Style) bool { return s.Margin.isZero() },
			func(s Style) bool { return s.Padding != Edges{} }},
		{"ClearGap", ClearGap, func(s Style) bool { return s.Gap == 0 },
			func(s Style) bool { return s.Padding != Edges{} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := full.Merge(Style{Clear: c.clear})
			if !c.gone(got) {
				t.Errorf("%s did not remove its fields: %+v", c.name, got)
			}
			if !c.kept(got) {
				t.Errorf("%s removed fields it does not name: %+v", c.name, got)
			}
			if got.Clear != c.clear {
				t.Errorf("the merged style keeps clear set %v, want %v", got.Clear, c.clear)
			}
		})
	}
}

// TestClearThenValueSetsTheField checks that a value given with a clear for the
// same field is the value, so a clear can be followed by a new look.
func TestClearThenValueSetsTheField(t *testing.T) {
	red := Hex("#ff0000")
	got := fullStyle().Merge(Style{Clear: ClearBackground, Background: red})
	if got.Background != red {
		t.Errorf("background %v, want the new value %v", got.Background, red)
	}
	if !got.Gradient.isZero() {
		t.Errorf("the gradient survived the clear")
	}
}

// TestZeroStyleKeepsDefaults checks backward compatibility: a style with no
// clear and zero fields leaves what it merges into alone.
func TestZeroStyleKeepsDefaults(t *testing.T) {
	base := fullStyle()
	got := base.Merge(Style{})
	if got.Background != base.Background || got.Padding != base.Padding || got.Radius != base.Radius ||
		got.BorderWidth != base.BorderWidth || len(got.Shadows) != 1 {
		t.Errorf("a zero style changed the defaults: %+v", got)
	}
}

// TestPartOverridesTurnDefaultsOff checks the clear through a widget's parts:
// a transparent tab, no radius and no padding, no border on a select, no shadow
// on its popup, and no radius on a slider's rail.
func TestPartOverridesTurnDefaultsOff(t *testing.T) {
	sel := NewSignal("a")
	r := newAccessRig(func(ctx *Context) Element {
		return partTabs(sel, TabsUnderline, TabsStyle{Tab: Style{Clear: ClearBackground | ClearRadius | ClearPadding, Background: Transparent}})
	})
	tab := keyNode(t, r.rt, "settings/tab:c")
	if !tab.style.Background.IsZero() || tab.style.Radius != 0 || !tab.style.Padding.isZero() {
		t.Errorf("the tab kept a default: bg %v radius %v padding %v", tab.style.Background, tab.style.Radius, tab.style.Padding)
	}

	r = newAccessRig(func(ctx *Context) Element {
		return partSelect(NewSignal(""), SelectStyle{
			Trigger: Style{Clear: ClearBorder},
			Popup:   Style{Clear: ClearShadows},
		}, false)
	})
	if n := partNode(t, r.rt, "Select.Trigger", ""); n.style.BorderWidth != 0 || !n.style.BorderColor.IsZero() {
		t.Errorf("the select trigger kept its border: width %v", n.style.BorderWidth)
	}
	r.clickKey(t, "chan")
	if n := partNode(t, r.rt, "Select.Popup", ""); len(n.style.Shadows) != 0 {
		t.Errorf("the select popup kept its shadow")
	}

	v := NewSignal(50.0)
	r = newAccessRig(func(ctx *Context) Element {
		return partSlider(v, SliderStyle{Rail: Style{Clear: ClearRadius}})
	})
	if n := partNode(t, r.rt, "Slider.Rail", ""); n.style.Radius != 0 {
		t.Errorf("the slider rail kept its radius %v", n.style.Radius)
	}
}

// TestHighContrastKeepsSystemColorsAndBorders checks the guardrail: under high
// contrast an override cannot clear a system color, a border or a shadow, and
// the sizes and spacing it clears do go.
func TestHighContrastKeepsSystemColorsAndBorders(t *testing.T) {
	sel := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return partSelect(sel, SelectStyle{
			Trigger: Style{Clear: ClearBorder | ClearBackground | ClearShadows | ClearPadding, Padding: Edges{}},
		}, false)
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	n := partNode(t, r.rt, "Select.Trigger", "")
	if n.style.BorderWidth == 0 || n.style.BorderColor != th.BorderStrong {
		t.Errorf("high contrast lost the trigger's border: width %v color %v", n.style.BorderWidth, n.style.BorderColor)
	}
	if n.style.Background != th.Surface {
		t.Errorf("high contrast lost the system background: %v", n.style.Background)
	}
	if !n.style.Padding.isZero() {
		t.Errorf("the padding clear did not apply under high contrast: %v", n.style.Padding)
	}
}

// TestSelectedTabStaysPerceivable checks the guardrail for a state that carries
// meaning: when an app removes the selected tab's indicator and its fill, the
// selected tab still has semibold text, so the selection shows.
func TestSelectedTabStaysPerceivable(t *testing.T) {
	for _, hc := range []bool{false, true} {
		st := TabsStyle{
			Indicator:   Style{Clear: ClearBackground, Height: Px(0)},
			TabSelected: Style{Clear: ClearBackground | ClearTextColor, FontWeight: FontLight},
		}
		sel := NewSignal("a")
		r := newAccessRig(func(ctx *Context) Element { return partTabs(sel, TabsUnderline, st) })
		if hc {
			r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
			r.frame()
		}
		box := keyNode(t, r.rt, "settings/tab:a")
		if box.style.FontWeight < FontSemibold {
			t.Errorf("high contrast=%v: the selected tab's weight is %v, want at least semibold", hc, box.style.FontWeight)
		}
		if got := len(partNodes(r.rt, "Tabs.Indicator")); got != 3 {
			t.Errorf("high contrast=%v: %d indicator nodes, want one per tab", hc, got)
		}
	}
}

// TestHighlightedOptionStaysPerceivable checks that a highlighted option keeps
// semibold text with its highlight cleared.
func TestHighlightedOptionStaysPerceivable(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return partSelect(NewSignal(""), SelectStyle{OptionHighlighted: Style{Clear: ClearBackground}}, false)
	})
	r.clickKey(t, "chan")
	row := accessNode(t, r, access.RoleListItem, "Alpha")
	if row.style.FontWeight < FontSemibold {
		t.Errorf("the highlighted option's weight is %v, want at least semibold", row.style.FontWeight)
	}
	if !row.style.Background.IsZero() {
		t.Errorf("the highlight was not cleared: %v", row.style.Background)
	}
}

// TestStackedSpinButtonsSitInsideTheField checks the Windows compact layout:
// the two buttons are inside the field's right edge, each half the field's
// height, at least 16 px wide, and the text stops before them, at the text size
// setting and at 225 percent.
func TestStackedSpinButtonsSitInsideTheField(t *testing.T) {
	for _, scale := range []int{100, 225} {
		r := newAccessRig(func(ctx *Context) Element {
			return partNumberField(NewSignal(8), NumberFieldStyle{}, SpinStacked, false)
		})
		if scale != 100 {
			r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
			r.frame()
		}
		frame := rectOf(r, partNode(t, r.rt, "NumberField.Frame", ""))
		group := rectOf(r, partNode(t, r.rt, "NumberField.SpinButtons", ""))
		up := rectOf(r, buttonNode(t, r, "Increase"))
		down := rectOf(r, buttonNode(t, r, "Decrease"))
		var text rectT
		for _, n := range r.rt.byID {
			if n.kind == kindEditor {
				text = rectOf(r, n)
			}
		}
		if group.X < frame.X || group.X+group.W > frame.X+frame.W+0.5 {
			t.Errorf("%d%%: the spin buttons %v are not inside the field %v", scale, group, frame)
		}
		if text.X+text.W > group.X+0.5 {
			t.Errorf("%d%%: the text (right edge %v) runs under the spin buttons (left edge %v)", scale, text.X+text.W, group.X)
		}
		if up.W < 16 || down.W < 16 {
			t.Errorf("%d%%: the buttons are %v and %v px wide, want at least 16", scale, up.W, down.W)
		}
		if d := up.H - down.H; d > 1 || d < -1 || up.H+down.H > frame.H+1 || up.H < frame.H/2-2 {
			t.Errorf("%d%%: the buttons are %v and %v px tall in a field %v px tall; each want half", scale, up.H, down.H, frame.H)
		}
		if up.Y > down.Y {
			t.Errorf("%d%%: the increase button is below the decrease button", scale)
		}
	}
}

// TestFieldPartClears checks the clear through the text frame's parts: a text
// field with no border and no fill, a placeholder that is not given a color, a
// selection with no color, and a number field's frame with no border.
func TestFieldPartClears(t *testing.T) {
	st := FieldStyle{
		Frame:       Style{Clear: ClearBorder | ClearBackground},
		Placeholder: Style{Clear: ClearTextColor},
		Selection:   Style{Clear: ClearBackground},
	}
	r := newAccessRig(func(ctx *Context) Element { return partTextInput(NewSignal(""), st) })
	frame := partNode(t, r.rt, "TextInput.Frame", "")
	if frame.style.BorderWidth != 0 || !frame.style.BorderColor.IsZero() {
		t.Errorf("the text field kept its border: width %v color %v", frame.style.BorderWidth, frame.style.BorderColor)
	}
	if !frame.style.Background.IsZero() {
		t.Errorf("the text field kept its fill: %v", frame.style.Background)
	}
	ed := editorNode(t, r.rt)
	if !ed.editor.placeholderColor.IsZero() {
		t.Errorf("the placeholder kept its color: %v", ed.editor.placeholderColor)
	}
	if !ed.editor.selection.IsZero() {
		t.Errorf("the selection kept its color: %v", ed.editor.selection)
	}

	r = newAccessRig(func(ctx *Context) Element {
		return partNumberField(NewSignal(8), NumberFieldStyle{FieldStyle: FieldStyle{Frame: Style{Clear: ClearBorder}}}, SpinInline, false)
	})
	if n := partNode(t, r.rt, "NumberField.Frame", ""); n.style.BorderWidth != 0 {
		t.Errorf("the number field kept its frame border: %v", n.style.BorderWidth)
	}
}

// rectT is a rectangle in window coordinates.
type rectT struct{ X, Y, W, H float32 }

// rectOf returns the rectangle of a built node.
func rectOf(r *accessRig, n *node) rectT {
	rc := r.rt.nodeRect(n)
	return rectT{rc.X, rc.Y, rc.W, rc.H}
}

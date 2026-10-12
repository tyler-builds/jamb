package ui

import (
	"testing"
	"time"
)

// fieldInShrinkingRow lays out el, a text field, in a 96 px row, which is narrower
// than the field's 240 px default, so the field must shrink. It returns the runtime.
func fieldInShrinkingRow(scale int, el Element) *Runtime {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Column().Child(Div().Key("row").Row().Width(Px(96)).Child(el))
	}, 400, 300, 1)
	rt.SetSystemSettings(SystemSettings{TextScale: scale})
	rt.Frame(time.Now())
	return rt
}

// A text field in a shrinking row keeps room for a few characters, so it does not
// collapse to a letter's width. The minimum grows with the Text size.
func TestTextFieldKeepsRoomForFewCharactersInAShrinkingRow(t *testing.T) {
	widths := map[int]float32{}
	for _, scale := range []int{100, 225} {
		field := TextInput(NewSignal("")).Width(Px(240))
		rt := fieldInShrinkingRow(scale, field)
		w := partNode(t, rt, "TextInput.Frame", "").lay.W
		if w >= 240 {
			t.Errorf("scale %d: field is %v wide; it should shrink in a 96 px row", scale, w)
		}
		if w < 40 {
			t.Errorf("scale %d: field collapsed to %v wide, below room for a few characters", scale, w)
		}
		widths[scale] = w
	}
	if widths[225] <= widths[100] {
		t.Errorf("minimum at 225%% is %v, not wider than at 100%% (%v)", widths[225], widths[100])
	}
}

// A text area keeps the same minimum in a shrinking row.
func TestTextAreaKeepsRoomForFewCharactersInAShrinkingRow(t *testing.T) {
	for _, scale := range []int{100, 225} {
		rt := fieldInShrinkingRow(scale, TextArea(NewSignal("")).Width(Px(240)))
		if w := partNode(t, rt, "TextArea.Frame", "").lay.W; w < 40 {
			t.Errorf("scale %d: text area collapsed to %v wide", scale, w)
		}
	}
}

// MinWidth replaces the automatic minimum, so a field can be held wider.
func TestTextFieldMinWidthOverridesTheAutomaticMinimum(t *testing.T) {
	rt := fieldInShrinkingRow(100, TextInput(NewSignal("")).Width(Px(240)).MinWidth(Px(200)))
	if w := partNode(t, rt, "TextInput.Frame", "").lay.W; w != 200 {
		t.Errorf("field with MinWidth(200) is %v wide in a 96 px row, want 200", w)
	}
}

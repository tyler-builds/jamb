package ui

import (
	"math"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// numberRig returns a rig of an int number field with value, built by build, at the
// Text size setting scale.
func numberRig(value, scale int, build func(*NumberFieldElement[int]) *NumberFieldElement[int]) *accessRig {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(build(NumberField(NewSignal(value), 0, 999999, 1).Label("Hour")))
	})
	if scale != 100 {
		r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
		r.frame()
	}
	return r
}

// valueText returns the editor's rectangle in window coordinates, and the left edge and
// the width of its text as it is painted.
func valueText(t *testing.T, r *accessRig) (box rectT, left, width float32) {
	t.Helper()
	n := editorNode(t, r.rt)
	box = rectOf(r, n)
	return box, box.X + n.st.edit.originX, n.st.edit.layout.Width
}

// plainField is the default number field: no Align call.
func plainField(n *NumberFieldElement[int]) *NumberFieldElement[int] { return n }

// TestNumberFieldCentersTheValueByDefault checks that the value sits in the middle of
// its text area by default, at 100 and 225 percent. Without spin buttons, and with the
// inline ones, the text area is the frame's inside with the same room on each side, so
// the value is in the field's middle. The stacked column is on the right, so its text
// area is the room between the frame's left padding and the column, and the value is in
// the middle of that room.
func TestNumberFieldCentersTheValueByDefault(t *testing.T) {
	cases := []struct {
		name      string
		build     func(*NumberFieldElement[int]) *NumberFieldElement[int]
		symmetric bool // the text area has the same room on both sides of the frame
	}{
		{"plain", plainField, true},
		{"inline", func(n *NumberFieldElement[int]) *NumberFieldElement[int] { return n.SpinButtons(SpinInline) }, true},
		{"stacked", func(n *NumberFieldElement[int]) *NumberFieldElement[int] { return n.SpinButtons(SpinStacked) }, false},
	}
	for _, c := range cases {
		for _, scale := range []int{100, 225} {
			r := numberRig(8, scale, c.build)
			box, left, width := valueText(t, r)
			if !nearPx(left+width/2, box.X+box.W/2) {
				t.Errorf("%s %d%%: the value is centered at %v, want the text area's middle %v", c.name, scale, left+width/2, box.X+box.W/2)
			}
			if c.symmetric {
				fr := rectOf(r, partNode(t, r.rt, "NumberField.Frame", ""))
				if !nearPx(box.X+box.W/2, fr.X+fr.W/2) {
					t.Errorf("%s %d%%: the text area's middle is %v, want the field's middle %v", c.name, scale, box.X+box.W/2, fr.X+fr.W/2)
				}
			}
		}
	}
}

// TestNumberFieldAlignStartAndEnd checks that Align(Start) puts the value at the left
// of the text area and Align(End) at its right, and that the inline buttons do not move
// the text out of the room between them.
func TestNumberFieldAlignStartAndEnd(t *testing.T) {
	for _, spin := range []SpinPlacement{SpinInline, SpinStacked} {
		r := numberRig(8, 100, func(n *NumberFieldElement[int]) *NumberFieldElement[int] {
			return n.SpinButtons(spin).Align(Start)
		})
		box, left, _ := valueText(t, r)
		if !nearPx(left, box.X) {
			t.Errorf("%v Start: the value starts at %v, want the text area's left %v", spin, left, box.X)
		}
		r = numberRig(8, 100, func(n *NumberFieldElement[int]) *NumberFieldElement[int] {
			return n.SpinButtons(spin).Align(End)
		})
		box, left, width := valueText(t, r)
		if !nearPx(left+width, box.X+box.W) {
			t.Errorf("%v End: the value ends at %v, want the text area's right %v", spin, left+width, box.X+box.W)
		}
	}
}

// TestNumberFieldOverflowingValueIsNotAligned checks that a value wider than the field
// is not moved by its alignment: it starts at the left, as it scrolls, when the field is
// not focused.
func TestNumberFieldOverflowingValueIsNotAligned(t *testing.T) {
	r := numberRig(123456, 100, func(n *NumberFieldElement[int]) *NumberFieldElement[int] {
		return n.Width(Px(40)).Align(End)
	})
	box, left, width := valueText(t, r)
	if width <= box.W {
		t.Fatalf("the value is %v px wide in a text area %v px wide; it must overflow", width, box.W)
	}
	if !nearPx(left, box.X) {
		t.Errorf("an overflowing value starts at %v, want the text area's left %v", left, box.X)
	}
}

// TestNumberFieldCaretFollowsTheAlignment checks that the caret at the end of the value
// is at the end of the aligned text, for each alignment.
func TestNumberFieldCaretFollowsTheAlignment(t *testing.T) {
	for _, a := range []Alignment{Start, Center, End} {
		r := numberRig(8, 100, func(n *NumberFieldElement[int]) *NumberFieldElement[int] { return n.Align(a) })
		focusNode(r, editorNode(t, r.rt))
		r.press(input.KeyEnd, 0)
		r.frame()
		_, left, width := valueText(t, r)
		caret := r.rt.textInput
		if !nearPx(caret.X+caret.W/2, left+width) {
			t.Errorf("alignment %v: the caret is at %v, want the end of the text at %v", a, caret.X+caret.W/2, left+width)
		}
	}
}

// TestNumberFieldIsVerticallyCentered checks that the value's line is centered in the
// frame at 100, 150 and 225 percent text scale, and at fractional DPI, where the middle
// may differ by half a physical pixel. It is checked with no spin buttons, with inline
// and with stacked ones.
func TestNumberFieldIsVerticallyCentered(t *testing.T) {
	builds := map[string]func(*NumberFieldElement[int]) *NumberFieldElement[int]{
		"plain":   plainField,
		"inline":  func(n *NumberFieldElement[int]) *NumberFieldElement[int] { return n.SpinButtons(SpinInline) },
		"stacked": func(n *NumberFieldElement[int]) *NumberFieldElement[int] { return n.SpinButtons(SpinStacked) },
	}
	for name, build := range builds {
		for _, scale := range []int{100, 150, 225} {
			for _, dpi := range []float32{1, 1.25, 1.5} {
				r := numberRig(8, scale, build)
				r.rt.SetViewport(500, 400, dpi)
				r.frame()
				box, _, _ := valueText(t, r)
				fr := rectOf(r, partNode(t, r.rt, "NumberField.Frame", ""))
				tol := float32(0.5) / dpi
				if d := (box.Y + box.H/2) - (fr.Y + fr.H/2); float32(math.Abs(float64(d))) > tol {
					t.Errorf("%s %d%% %v DPI: the value's line is centered %v px off the frame's middle, want within %v", name, scale, dpi, d, tol)
				}
			}
		}
	}
}

// TestDisabledNumberFieldIsCentered checks that a disabled number field centers its
// value horizontally and vertically, as an enabled one does.
func TestDisabledNumberFieldIsCentered(t *testing.T) {
	for _, scale := range []int{100, 225} {
		r := numberRig(8, scale, func(n *NumberFieldElement[int]) *NumberFieldElement[int] { return n.Disabled(true) })
		fr := rectOf(r, partNode(t, r.rt, "NumberField.Frame", ""))
		text := rectOf(r, partNode(t, r.rt, "NumberField.Text", ""))
		if !nearPx(text.X+text.W/2, fr.X+fr.W/2) {
			t.Errorf("%d%%: the disabled value is centered at %v, want %v", scale, text.X+text.W/2, fr.X+fr.W/2)
		}
		if !nearPx(text.Y+text.H/2, fr.Y+fr.H/2) {
			t.Errorf("%d%%: the disabled value is vertically centered at %v, want %v", scale, text.Y+text.H/2, fr.Y+fr.H/2)
		}
	}
}

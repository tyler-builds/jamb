package ui

import (
	"math"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// nearPx reports whether a and b differ by less than half a pixel.
func nearPx(a, b float32) bool { return math.Abs(float64(a-b)) < 0.5 }

// spinRig returns a rig of a number field with spin buttons of placement p, at the
// Text size setting scale.
func spinRig(p SpinPlacement, scale int) *accessRig {
	r := newAccessRig(func(ctx *Context) Element {
		return partNumberField(NewSignal(8), NumberFieldStyle{}, p, false)
	})
	if scale != 100 {
		r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
		r.frame()
	}
	return r
}

// TestInlineSpinButtonsSitInsideTheFrame checks the inline buttons: the decrease
// button is at the frame's left edge and the increase button at its right, both
// inside the border, at the frame's full height, at least 16 px wide, and the text
// is between them. It is checked at the Text size setting and at 225 percent.
func TestInlineSpinButtonsSitInsideTheFrame(t *testing.T) {
	for _, scale := range []int{100, 225} {
		r := spinRig(SpinInline, scale)
		fn := partNode(t, r.rt, "NumberField.Frame", "")
		frame := rectOf(r, fn)
		bw := fn.style.BorderWidth
		dec := rectOf(r, buttonNode(t, r, "Decrease"))
		inc := rectOf(r, buttonNode(t, r, "Increase"))
		var text rectT
		for _, n := range r.rt.byID {
			if n.kind == kindEditor {
				text = rectOf(r, n)
			}
		}
		if !nearPx(dec.X, frame.X+bw) {
			t.Errorf("%d%%: the decrease button starts at %v, want the frame's left inside edge %v", scale, dec.X, frame.X+bw)
		}
		if !nearPx(inc.X+inc.W, frame.X+frame.W-bw) {
			t.Errorf("%d%%: the increase button ends at %v, want the frame's right inside edge %v", scale, inc.X+inc.W, frame.X+frame.W-bw)
		}
		if dec.X+dec.W > text.X+0.5 || text.X+text.W > inc.X+0.5 {
			t.Errorf("%d%%: the text %v is not between the buttons %v and %v", scale, text, dec, inc)
		}
		if text.W <= 0 {
			t.Errorf("%d%%: the text has no width", scale)
		}
		if dec.W < 16 || inc.W < 16 {
			t.Errorf("%d%%: the buttons are %v and %v px wide, want at least 16", scale, dec.W, inc.W)
		}
		if !nearPx(dec.H, frame.H-2*bw) || !nearPx(inc.H, frame.H-2*bw) || !nearPx(dec.Y, frame.Y+bw) {
			t.Errorf("%d%%: the buttons are %v and %v px tall at %v in a frame %v px tall", scale, dec.H, inc.H, dec.Y, frame.H)
		}
	}
}

// TestInlineSpinFieldScalesWithTheText checks that the field grows with the Text
// size, as its buttons do, so the text keeps a width of its own at 225 percent.
func TestInlineSpinFieldScalesWithTheText(t *testing.T) {
	r100 := spinRig(SpinInline, 100)
	r225 := spinRig(SpinInline, 225)
	w100 := rectOf(r100, partNode(t, r100.rt, "NumberField.Frame", "")).W
	w225 := rectOf(r225, partNode(t, r225.rt, "NumberField.Frame", "")).W
	if !nearPx(w225, w100*2.25) {
		t.Errorf("the field is %v px wide at 225 percent, want %v", w225, w100*2.25)
	}
}

// TestSpinButtonCornersFollowTheFrame checks that a spin button's corners are the
// frame's inner corners on its edge, so that its fill follows the rounded border and
// does not cover it. The stacked buttons are on the right, and the inline buttons on
// both sides. It is checked for the normal border and for the invalid frame, whose
// border is wider.
func TestSpinButtonCornersFollowTheFrame(t *testing.T) {
	for _, p := range []SpinPlacement{SpinInline, SpinStacked} {
		r := spinRig(p, 100)
		fn := partNode(t, r.rt, "NumberField.Frame", "")
		check := func(label string) {
			t.Helper()
			want := innerCorners(fn.style.radii(), fn.style.borderEdges())
			if want.TopRight == 0 {
				t.Fatalf("%s: the frame has no rounded corner to follow", label)
			}
			up := buttonNode(t, r, "Increase").style.radii()
			down := buttonNode(t, r, "Decrease").style.radii()
			if p == SpinStacked {
				if up.TopRight != want.TopRight || up.BottomRight != 0 {
					t.Errorf("%s stacked: the increase button's corners are %+v, want the top right %v", label, up, want.TopRight)
				}
				if down.BottomRight != want.BottomRight || down.TopRight != 0 {
					t.Errorf("%s stacked: the decrease button's corners are %+v, want the bottom right %v", label, down, want.BottomRight)
				}
				return
			}
			if up.TopRight != want.TopRight || up.BottomRight != want.BottomRight || up.TopLeft != 0 {
				t.Errorf("%s inline: the increase button's corners are %+v, want the right %v", label, up, want)
			}
			if down.TopLeft != want.TopLeft || down.BottomLeft != want.BottomLeft || down.TopRight != 0 {
				t.Errorf("%s inline: the decrease button's corners are %+v, want the left %v", label, down, want)
			}
		}
		check("normal")

		// An invalid field draws its frame at 2 px, and the buttons follow that border.
		focusNode(r, editorNode(t, r.rt))
		r.typeText("x")
		r.press(input.KeyEnter, 0)
		fn = partNode(t, r.rt, "NumberField.Frame", "")
		if fn.style.BorderWidth != 2 {
			t.Fatalf("the invalid frame is %v px wide, want 2", fn.style.BorderWidth)
		}
		check("invalid")
	}
}

// TestInlineSpinButtonsKeepTheTabOrder checks that the Tab order is the text, the
// decrease button and the increase button, as it was before the buttons were laid
// over the frame's edges.
func TestInlineSpinButtonsKeepTheTabOrder(t *testing.T) {
	r := spinRig(SpinInline, 100)
	focusNode(r, editorNode(t, r.rt))
	r.press(input.KeyTab, 0)
	if !r.stateOf(t, access.RoleButton, "Decrease").States.Has(access.StateFocused) {
		t.Fatal("Tab from the text did not reach the decrease button")
	}
	r.press(input.KeyTab, 0)
	if !r.stateOf(t, access.RoleButton, "Increase").States.Has(access.StateFocused) {
		t.Fatal("Tab from the decrease button did not reach the increase button")
	}
}

package ui

import (
	"math"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// nearPx reports whether a and b differ by less than half a pixel.
func nearPx(a, b float32) bool { return math.Abs(float64(a-b)) < 0.5 }

// spinRig returns a rig of a number input with spin buttons of placement p, at the
// Text size setting scale.
func spinRig(p SpinPlacement, scale int) *accessRig {
	r := newAccessRig(func(ctx *Context) Element {
		return partNumberInput(NewSignal(8), NumberInputStyle{}, p, false)
	})
	if scale != 100 {
		r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
		r.frame()
	}
	return r
}

// makeInvalid types text that is not a number into the field of r and checks it, so the
// frame is drawn with its wider invalid border.
func makeInvalid(t *testing.T, r *accessRig) {
	t.Helper()
	focusNode(r, editorNode(t, r.rt))
	r.typeText("x")
	r.press(input.KeyEnter, 0)
	if bw := partNode(t, r.rt, "NumberInput.Frame", "").style.BorderWidth; bw != 2 {
		t.Fatalf("the invalid frame is %v px wide, want 2", bw)
	}
}

// spinGapAt returns the gap between a spin button's fill and the border, in logical
// pixels, at the Text size setting scale (see spinGap).
func spinGapAt(scale int) float32 { return spinGap * float32(scale) / 100 }

// insideBorder returns the rectangle inside the border of the frame node fn, in window
// coordinates.
func insideBorder(r *accessRig, fn *node) rectT {
	f := rectOf(r, fn)
	be := fn.style.borderEdges()
	return rectT{f.X + be.Left, f.Y + be.Top, f.W - be.Left - be.Right, f.H - be.Top - be.Bottom}
}

// TestInlineSpinButtonsSitInsideTheFrame checks the inline buttons: the decrease button
// is at the frame's left edge and the increase button at its right, each inset from the
// border's inside edge by the gap on every side, and the text is between them. It is
// checked at the Text size setting, at 225 percent, and with the invalid frame's border.
func TestInlineSpinButtonsSitInsideTheFrame(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		for _, scale := range []int{100, 225} {
			r := spinRig(SpinInline, scale)
			if invalid {
				makeInvalid(t, r)
			}
			in := insideBorder(r, partNode(t, r.rt, "NumberInput.Frame", ""))
			gap := spinGapAt(scale)
			dec := rectOf(r, buttonNode(t, r, "Decrease"))
			inc := rectOf(r, buttonNode(t, r, "Increase"))
			var text rectT
			for _, n := range r.rt.byID {
				if n.kind == kindEditor {
					text = rectOf(r, n)
				}
			}
			if !nearPx(dec.X, in.X+gap) {
				t.Errorf("%d%% invalid=%v: the decrease fill starts at %v, want %v, the gap inside the border", scale, invalid, dec.X, in.X+gap)
			}
			if !nearPx(inc.X+inc.W, in.X+in.W-gap) {
				t.Errorf("%d%% invalid=%v: the increase fill ends at %v, want %v, the gap inside the border", scale, invalid, inc.X+inc.W, in.X+in.W-gap)
			}
			if !nearPx(dec.Y, in.Y+gap) || !nearPx(dec.H, in.H-2*gap) || !nearPx(inc.H, in.H-2*gap) {
				t.Errorf("%d%% invalid=%v: the buttons are %v and %v px tall at %v in a border's inside %v px tall", scale, invalid, dec.H, inc.H, dec.Y, in.H)
			}
			if dec.X+dec.W > text.X+0.5 || text.X+text.W > inc.X+0.5 {
				t.Errorf("%d%% invalid=%v: the text %v is not between the buttons %v and %v", scale, invalid, text, dec, inc)
			}
			if text.W <= 0 {
				t.Errorf("%d%% invalid=%v: the text has no width", scale, invalid)
			}
			if dec.W < 16 || inc.W < 16 {
				t.Errorf("%d%% invalid=%v: the buttons are %v and %v px wide, want at least 16", scale, invalid, dec.W, inc.W)
			}
		}
	}
}

// TestStackedSpinFillsAreInsetFromTheBorder checks the stacked buttons: the increase
// fill is inset from the border's top and right sides by the gap, the decrease fill from
// its bottom and right sides, and the two fills are twice the gap apart. They are the
// same height. It is checked at the Text size setting, at 225 percent, and with the
// invalid frame's border.
func TestStackedSpinFillsAreInsetFromTheBorder(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		for _, scale := range []int{100, 225} {
			r := spinRig(SpinStacked, scale)
			if invalid {
				makeInvalid(t, r)
			}
			in := insideBorder(r, partNode(t, r.rt, "NumberInput.Frame", ""))
			gap := spinGapAt(scale)
			up := rectOf(r, buttonNode(t, r, "Increase"))
			down := rectOf(r, buttonNode(t, r, "Decrease"))
			if !nearPx(up.Y, in.Y+gap) || !nearPx(down.Y+down.H, in.Y+in.H-gap) {
				t.Errorf("%d%% invalid=%v: the stacked fills run from %v to %v, want %v to %v", scale, invalid, up.Y, down.Y+down.H, in.Y+gap, in.Y+in.H-gap)
			}
			if !nearPx(up.X+up.W, in.X+in.W-gap) || !nearPx(down.X+down.W, in.X+in.W-gap) {
				t.Errorf("%d%% invalid=%v: the stacked fills end at %v and %v, want %v", scale, invalid, up.X+up.W, down.X+down.W, in.X+in.W-gap)
			}
			if !nearPx(down.Y-(up.Y+up.H), 2*gap) {
				t.Errorf("%d%% invalid=%v: the fills are %v apart, want %v", scale, invalid, down.Y-(up.Y+up.H), 2*gap)
			}
			if !nearPx(up.H, down.H) {
				t.Errorf("%d%% invalid=%v: the fills are %v and %v px tall", scale, invalid, up.H, down.H)
			}
		}
	}
}

// TestSpinFillGapIsAtLeastOnePhysicalPixel checks the gap in physical pixels at
// fractional DPI, where the edges are snapped to the pixel grid: the fill of each
// button is at least one pixel inside the border's inside edge on the sides it meets.
func TestSpinFillGapIsAtLeastOnePhysicalPixel(t *testing.T) {
	for _, p := range []SpinPlacement{SpinInline, SpinStacked} {
		for _, dpi := range []float32{1, 1.25, 1.5, 2.25} {
			r := spinRig(p, 100)
			r.rt.SetViewport(500, 400, dpi)
			r.frame()
			fn := partNode(t, r.rt, "NumberInput.Frame", "")
			fp := r.rt.px(r.rt.nodeRect(fn))
			be := r.rt.borderSides(fn.style.borderEdges())
			inLeft, inTop := fp.X+be.Left, fp.Y+be.Top
			inRight, inBottom := fp.X+fp.W-be.Right, fp.Y+fp.H-be.Bottom
			check := func(label string, n *node, left, top, right, bottom bool) {
				t.Helper()
				b := r.rt.px(r.rt.nodeRect(n))
				if left && b.X-inLeft < 1 || right && inRight-(b.X+b.W) < 1 ||
					top && b.Y-inTop < 1 || bottom && inBottom-(b.Y+b.H) < 1 {
					t.Errorf("%v at %v DPI: %s fill %v is not a physical pixel inside the border %v %v %v %v",
						p, dpi, label, b, inLeft, inTop, inRight, inBottom)
				}
			}
			if p == SpinInline {
				check("decrease", buttonNode(t, r, "Decrease"), true, true, false, true)
				check("increase", buttonNode(t, r, "Increase"), false, true, true, true)
				continue
			}
			check("increase", buttonNode(t, r, "Increase"), false, true, true, false)
			check("decrease", buttonNode(t, r, "Decrease"), false, false, true, true)
		}
	}
}

// TestSpinButtonCornersStayInsideTheCurve checks that a spin button's fill has the
// frame's inner radius on its side less the gap, on each of its corners. That radius
// is concentric with the frame's inner corner, so the fill never reaches past the
// border's curve. It is checked for the normal border and the invalid frame's wider
// border, at 100 and 225 percent.
func TestSpinButtonCornersStayInsideTheCurve(t *testing.T) {
	for _, p := range []SpinPlacement{SpinInline, SpinStacked} {
		for _, invalid := range []bool{false, true} {
			for _, scale := range []int{100, 225} {
				r := spinRig(p, scale)
				if invalid {
					makeInvalid(t, r)
				}
				fn := partNode(t, r.rt, "NumberInput.Frame", "")
				ic := innerCorners(fn.style.radii(), fn.style.borderEdges())
				if ic.TopRight == 0 {
					t.Fatalf("the frame has no rounded corner to follow")
				}
				gap := spinGapAt(scale)
				up := buttonNode(t, r, "Increase").style.radii()
				down := buttonNode(t, r, "Decrease").style.radii()
				var wantUp, wantDown float32
				if p == SpinStacked {
					wantUp = max(ic.TopRight-gap, 0)
					wantDown = max(ic.BottomRight-gap, 0)
				} else {
					wantUp = max(min(ic.TopRight, ic.BottomRight)-gap, 0)
					wantDown = max(min(ic.TopLeft, ic.BottomLeft)-gap, 0)
				}
				if want := (Corners{TopLeft: wantUp, TopRight: wantUp, BottomRight: wantUp, BottomLeft: wantUp}); up != want {
					t.Errorf("%v %d%% invalid=%v: the increase fill's corners are %+v, want %v", p, scale, invalid, up, wantUp)
				}
				if want := (Corners{TopLeft: wantDown, TopRight: wantDown, BottomRight: wantDown, BottomLeft: wantDown}); down != want {
					t.Errorf("%v %d%% invalid=%v: the decrease fill's corners are %+v, want %v", p, scale, invalid, down, wantDown)
				}
			}
		}
	}
}

// TestSpinButtonKeepsItsSizeOnHoverAndPress checks that the hover and pressed fills
// change only the background: the button's rectangle is the same at rest, hovered and
// pressed.
func TestSpinButtonKeepsItsSizeOnHoverAndPress(t *testing.T) {
	for _, p := range []SpinPlacement{SpinInline, SpinStacked} {
		r := spinRig(p, 100)
		rest := rectOf(r, buttonNode(t, r, "Increase"))
		x, y := rest.X+rest.W/2, rest.Y+rest.H/2
		r.rt.PointerMove(x, y, 0)
		r.frame()
		if bg := buttonNode(t, r, "Increase").style.Background; bg != DarkTheme().SurfaceHover {
			t.Errorf("%v: hovered fill is %v, want SurfaceHover", p, bg)
		}
		if got := rectOf(r, buttonNode(t, r, "Increase")); got != rest {
			t.Errorf("%v: the button is %v when hovered, want its rest rectangle %v", p, got, rest)
		}
		r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
		r.frame()
		if bg := buttonNode(t, r, "Increase").style.Background; bg != DarkTheme().SurfaceActive {
			t.Errorf("%v: pressed fill is %v, want SurfaceActive", p, bg)
		}
		if got := rectOf(r, buttonNode(t, r, "Increase")); got != rest {
			t.Errorf("%v: the button is %v when pressed, want its rest rectangle %v", p, got, rest)
		}
		r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	}
}

// TestInlineSpinFieldScalesWithTheText checks that the field grows with the Text
// size, as its buttons do, so the text keeps a width of its own at 225 percent.
func TestInlineSpinFieldScalesWithTheText(t *testing.T) {
	r100 := spinRig(SpinInline, 100)
	r225 := spinRig(SpinInline, 225)
	w100 := rectOf(r100, partNode(t, r100.rt, "NumberInput.Frame", "")).W
	w225 := rectOf(r225, partNode(t, r225.rt, "NumberInput.Frame", "")).W
	if !nearPx(w225, w100*2.25) {
		t.Errorf("the field is %v px wide at 225 percent, want %v", w225, w100*2.25)
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

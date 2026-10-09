package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// escape presses Escape and lets the frame run.
func (r *overlayRig) escape() {
	r.rt.KeyDown(input.KeyEscape, 0, false)
	r.rt.KeyUp(input.KeyEscape, 0)
	r.frame()
}

func (r *overlayRig) has(key string) bool {
	_, ok := r.rt.Find(key)
	return ok
}

// Escape closes an overlay with a dismiss handler, once.
func TestEscapeClosesOverlay(t *testing.T) {
	open := NewSignal(true)
	closed := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).
			Child(Div().Key("anchor").Size(Px(100), Px(30)).
				Child(If(open.Get(), Div().Key("menu").Overlay(BelowStart).Size(Px(120), Px(80)).
					OnClickOutside(func() { closed++; open.Set(false) }))))
	}, 800, 600)
	if !r.has("menu") {
		t.Fatal("menu not open")
	}
	r.escape()
	if r.has("menu") || closed != 1 {
		t.Errorf("after Escape: menu shown %v, closed %d times; want hidden, once", r.has("menu"), closed)
	}
	r.escape()
	if closed != 1 {
		t.Errorf("Escape with no overlay open dismissed something: %d closes", closed)
	}
}

// With two overlays open, Escape closes the one painted last and leaves the
// other, so each press closes one overlay.
func TestEscapeClosesTopmostOverlayOnly(t *testing.T) {
	a, b := NewSignal(true), NewSignal(true)
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).
			Child(Div().Key("anchor").Size(Px(100), Px(30)).
				Child(If(a.Get(), Div().Key("a").Overlay(BelowStart).Size(Px(120), Px(80)).
					OnClickOutside(func() { a.Set(false) }))).
				Child(If(b.Get(), Div().Key("b").Overlay(BelowStart).Size(Px(120), Px(80)).
					OnClickOutside(func() { b.Set(false) }))))
	}, 800, 600)
	r.escape()
	if r.has("b") || !r.has("a") {
		t.Fatalf("first Escape: b shown %v, a shown %v; want b closed and a open", r.has("b"), r.has("a"))
	}
	r.escape()
	if r.has("a") {
		t.Error("second Escape did not close the remaining overlay")
	}
}

// A popover inside a menu closes first, then the menu.
func TestEscapeClosesNestedOverlaysOneLevelAtATime(t *testing.T) {
	outer, inner := NewSignal(true), NewSignal(true)
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).
			Child(Div().Key("anchor").Size(Px(100), Px(30)).
				Child(If(outer.Get(), Div().Key("outer").Overlay(BelowStart).Size(Px(160), Px(120)).
					OnClickOutside(func() { outer.Set(false) }).
					Child(If(inner.Get(), Div().Key("inner").Overlay(RightStart).Size(Px(120), Px(60)).
						OnClickOutside(func() { inner.Set(false) }))))))
	}, 800, 600)
	if !r.has("inner") {
		t.Fatal("inner overlay not open")
	}
	r.escape()
	if r.has("inner") || !r.has("outer") {
		t.Fatalf("first Escape: inner shown %v, outer shown %v; want inner closed, outer open", r.has("inner"), r.has("outer"))
	}
	r.escape()
	if r.has("outer") {
		t.Error("second Escape did not close the outer overlay")
	}
}

// Focus inside a text field of the overlay does not stop the overlay closing:
// the field's own Cancel (which blurs it) does not run, so one Escape closes
// the overlay and it is not blurred first.
func TestEscapeInsideTextFieldClosesOverlay(t *testing.T) {
	open, text := NewSignal(true), NewSignal("")
	field := NewFocusHandle()
	closed := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).
			Child(Div().Key("anchor").Size(Px(100), Px(30)).
				Child(If(open.Get(), Div().Key("palette").Overlay(BelowStart).Size(Px(200), Px(120)).
					OnClickOutside(func() { closed++; open.Set(false) }).
					Child(TextInput(text).FocusHandle(field)))))
	}, 800, 600)
	field.Focus()
	r.frame()
	if !field.Focused() {
		t.Fatal("field not focused")
	}
	r.escape()
	if closed != 1 || r.has("palette") {
		t.Errorf("Escape closed the overlay %d times (shown %v); want once and hidden", closed, r.has("palette"))
	}
}

// An overlay without a dismiss handler does not change Escape: a field inside
// it is still blurred, as it is with no overlay at all.
func TestEscapeWithoutDismissHandlerStillBlursField(t *testing.T) {
	text := NewSignal("")
	field := NewFocusHandle()
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).
			Child(Div().Key("anchor").Size(Px(100), Px(30)).
				Child(Div().Key("panel").Overlay(BelowStart).Size(Px(200), Px(120)).
					Child(TextInput(text).FocusHandle(field))))
	}, 800, 600)
	field.Focus()
	r.frame()
	r.escape()
	if field.Focused() {
		t.Error("Escape did not blur the field")
	}
	if !r.has("panel") {
		t.Error("overlay without a dismiss handler closed")
	}
}

// An app binding for Escape runs before the overlay closes, and the overlay
// stays open.
func TestAppBindingTakesEscapeBeforeOverlay(t *testing.T) {
	open := NewSignal(true)
	cancel := NewAction("test.cancel", "Cancel")
	ran := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).OnAction(cancel, func() { ran++ }).
			Child(Div().Key("anchor").Size(Px(100), Px(30)).
				Child(If(open.Get(), Div().Key("menu").Overlay(BelowStart).Size(Px(120), Px(80)).
					OnClickOutside(func() { open.Set(false) }))))
	}, 800, 600)
	r.rt.AddBindings(Bind("escape", cancel))
	r.escape()
	if ran != 1 || !r.has("menu") {
		t.Errorf("binding ran %d times, menu shown %v; want the binding to take Escape and the menu to stay", ran, r.has("menu"))
	}
}

// An OnKeyDown that stops the event takes Escape before the overlay closes.
func TestAppKeyDownTakesEscapeBeforeOverlay(t *testing.T) {
	open := NewSignal(true)
	seen := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).OnKeyDown(func(e *KeyEvent) {
			if e.Key == input.KeyEscape {
				seen++
				e.StopPropagation()
			}
		}).
			Child(Div().Key("anchor").Size(Px(100), Px(30)).
				Child(If(open.Get(), Div().Key("menu").Overlay(BelowStart).Size(Px(120), Px(80)).
					OnClickOutside(func() { open.Set(false) }))))
	}, 800, 600)
	r.escape()
	if seen != 1 || !r.has("menu") {
		t.Errorf("OnKeyDown saw %d Escapes, menu shown %v; want 1 and open", seen, r.has("menu"))
	}
}

// Escape hides a visible tooltip and keeps it hidden, and the field focused
// inside the element with the tooltip keeps focus.
func TestEscapeHidesTooltipWithoutBlurring(t *testing.T) {
	text := NewSignal("")
	field := NewFocusHandle()
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Key("t").Padding(20).Tooltip("Name").
			Child(TextInput(text).FocusHandle(field))
	}, 800, 600)
	field.Focus()
	r.frame()
	rc := r.find(t, "t")
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
	r.now = r.now.Add(600 * time.Millisecond)
	r.frame()
	if !r.has("jamb.tooltip") {
		t.Fatal("tooltip not shown after the delay")
	}
	r.escape()
	if r.has("jamb.tooltip") {
		t.Error("Escape did not hide the tooltip")
	}
	if !field.Focused() {
		t.Error("Escape blurred the field instead of only hiding the tooltip")
	}
	r.now = r.now.Add(time.Second)
	r.frame()
	if r.has("jamb.tooltip") {
		t.Error("tooltip came back without the pointer moving")
	}
}

// Holding Escape closes one overlay: the auto-repeats of the press that closed
// it are ignored, so they neither close the next overlay nor reach the
// focused field's Cancel. The next press closes the next overlay.
func TestHeldEscapeClosesOneOverlay(t *testing.T) {
	outer, inner, text := NewSignal(true), NewSignal(true), NewSignal("")
	field := NewFocusHandle()
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).
			Child(TextInput(text).FocusHandle(field)).
			Child(Div().Key("anchor").Size(Px(100), Px(30)).
				Child(If(outer.Get(), Div().Key("outer").Overlay(BelowStart).Size(Px(160), Px(120)).
					OnClickOutside(func() { outer.Set(false) }).
					Child(If(inner.Get(), Div().Key("inner").Overlay(RightStart).Size(Px(120), Px(60)).
						OnClickOutside(func() { inner.Set(false) }))))))
	}, 800, 600)
	field.Focus()
	r.frame()
	r.rt.KeyDown(input.KeyEscape, 0, false)
	r.frame()
	for range 3 {
		r.rt.KeyDown(input.KeyEscape, 0, true)
		r.frame()
	}
	r.rt.KeyUp(input.KeyEscape, 0)
	r.frame()
	if r.has("inner") || !r.has("outer") {
		t.Fatalf("held Escape: inner shown %v, outer shown %v; want inner closed, outer open", r.has("inner"), r.has("outer"))
	}
	if !field.Focused() {
		t.Fatal("an auto-repeat reached the field and blurred it")
	}
	r.escape()
	if r.has("outer") {
		t.Error("the next press did not close the outer overlay")
	}
}

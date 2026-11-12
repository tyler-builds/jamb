//go:build js

package web

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
)

// eventWindow returns a window whose events are kept in the order they arrive,
// in a page where the window has the focus.
func eventWindow(t *testing.T) (*window, *[]platform.Event) {
	w, rec := touchWindow(t)
	var got []platform.Event
	rec.onEvent = func(e platform.Event) { got = append(got, e) }
	withPage(t, w)
	return w, &got
}

func kinds(events []platform.Event) []platform.EventKind {
	var k []platform.EventKind
	for _, e := range events {
		k = append(k, e.Kind)
	}
	return k
}

func sameKinds(a []platform.EventKind, b ...platform.EventKind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPageMods(t *testing.T) {
	meta := input.ModSuper
	if appleKeys {
		meta = input.ModCtrl
	}
	for _, c := range []struct {
		bits int
		want input.Modifiers
	}{
		{0, 0},
		{pageShift, input.ModShift},
		{pageCtrl | pageAlt, input.ModCtrl | input.ModAlt},
		{pageMeta, meta},
		{pageShift | pageCtrl | pageAlt | pageMeta, input.ModShift | input.ModCtrl | input.ModAlt | meta},
	} {
		if got := pageMods(c.bits); got != c.want {
			t.Errorf("pageMods(%b) = %v, want %v", c.bits, got, c.want)
		}
	}
}

// A press, a move and a release of the mouse arrive as the app's mouse events,
// in device pixels, and the press's default action is prevented.
func TestMousePressMoveRelease(t *testing.T) {
	w, got := eventWindow(t)
	w.scale = 2
	if !w.pointer(pointerEvent{kind: pointerDown, x: 10, y: 20, button: 0, buttons: 1, mods: input.ModShift}) {
		t.Error("a press keeps its default action")
	}
	w.pointer(pointerEvent{kind: pointerMove, x: 30, y: 40, buttons: 1})
	w.pointer(pointerEvent{kind: pointerUp, x: 30, y: 40, button: 0})
	runPosted()
	if !sameKinds(kinds(*got), platform.EventMouseMove, platform.EventMouseDown, platform.EventMouseMove, platform.EventMouseUp) {
		t.Fatalf("events %v, want a move, the press, the move and the release", kinds(*got))
	}
	down, move := (*got)[1], (*got)[2]
	if down.X != 20 || down.Y != 40 || down.Button != input.ButtonLeft || down.Clicks != 1 || down.Mods != input.ModShift {
		t.Errorf("the press arrived as %+v, want the left button at 20, 40 with Shift", down)
	}
	if move.X != 60 || move.Y != 80 {
		t.Errorf("the move arrived at %v, %v, want 60, 80", move.X, move.Y)
	}
	// A move elsewhere on the page, with no press of the canvas, is not the window's.
	*got = nil
	w.pointer(pointerEvent{kind: pointerMove, x: -5, y: 40})
	w.pointer(pointerEvent{kind: pointerUp, x: 30, y: 40, button: 0})
	runPosted()
	if len(*got) != 0 {
		t.Errorf("a move outside the canvas and a release without a press delivered %v", kinds(*got))
	}
}

// A press that ends outside the browser is released by the next move that
// holds no button.
func TestMouseReleaseOutsideBrowser(t *testing.T) {
	w, got := eventWindow(t)
	w.pointer(pointerEvent{kind: pointerDown, x: 10, y: 20, button: 0, buttons: 1})
	runPosted()
	*got = nil
	w.pointer(pointerEvent{kind: pointerMove, x: 500, y: 20, buttons: 0})
	runPosted()
	if !sameKinds(kinds(*got), platform.EventMouseUp, platform.EventMouseMove) || w.in.pressed != 0 {
		t.Errorf("events %v with %d buttons pressed, want the release and the move", kinds(*got), w.in.pressed)
	}
}

func TestWheel(t *testing.T) {
	w, got := eventWindow(t)
	for _, c := range []struct {
		e       wheelEvent
		dy      float32
		precise bool
	}{
		{wheelEvent{x: 5, y: 6, dy: 12.5}, 12.5, true},       // a touchpad: small, fractional pixels
		{wheelEvent{x: 5, y: 6, dy: 120}, 120, false},        // a mouse wheel's step, in pixels
		{wheelEvent{x: 5, y: 6, dy: 3, mode: 1}, 48, false},  // lines
		{wheelEvent{x: 5, y: 6, dy: 1, mode: 2}, 800, false}, // pages: the canvas's height
	} {
		*got = nil
		if !w.wheel(c.e) {
			t.Error("a wheel event keeps its default action, which scrolls the page")
		}
		runPosted()
		if len(*got) != 1 || (*got)[0].Kind != platform.EventScroll || (*got)[0].DeltaY != c.dy || (*got)[0].Precise != c.precise {
			t.Errorf("wheel %+v delivered %+v, want a scroll by %v, precise %v", c.e, *got, c.dy, c.precise)
		}
	}
}

// A key press reports whether the browser keeps it, and types its text unless
// a text field has the focus, whose textarea types it.
func TestKey(t *testing.T) {
	for _, c := range []struct {
		name    string
		e       keyEvent
		field   bool
		prevent bool
		want    []platform.EventKind
	}{
		{"a letter", keyEvent{down: true, code: "KeyA", key: "a"}, false, true,
			[]platform.EventKind{platform.EventKeyDown, platform.EventTextInput}},
		{"a letter in a text field", keyEvent{down: true, code: "KeyA", key: "a"}, true, false,
			[]platform.EventKind{platform.EventKeyDown}},
		{"a release", keyEvent{code: "KeyA", key: "a"}, false, true, []platform.EventKind{platform.EventKeyUp}},
		{"reload", keyEvent{down: true, code: "KeyR", key: "r", mods: input.ModCtrl}, false, false,
			[]platform.EventKind{platform.EventKeyDown}},
		{"copy", keyEvent{down: true, code: "KeyC", key: "c", mods: input.ModCtrl}, false, true,
			[]platform.EventKind{platform.EventKeyDown}},
		{"paste in a text field", keyEvent{down: true, code: "KeyV", key: "v", mods: input.ModCtrl}, true, false, nil},
		{"an input method's press", keyEvent{down: true, code: "KeyA", key: "Process", composing: true}, true, false, nil},
		{"a key jamb has not", keyEvent{down: true, code: "F13", key: "F13"}, false, false, nil},
		{"a phone keyboard's key, by its name", keyEvent{down: true, key: "Backspace"}, true, true,
			[]platform.EventKind{platform.EventKeyDown}},
	} {
		w, got := eventWindow(t)
		w.text.focused = c.field
		if prevent := w.key(c.e); prevent != c.prevent {
			t.Errorf("%s: default action prevented %v, want %v", c.name, prevent, c.prevent)
		}
		runPosted()
		if !sameKinds(kinds(*got), c.want...) {
			t.Errorf("%s: events %v, want %v", c.name, kinds(*got), c.want)
		}
	}
}

// dragRecorder is a handler whose elements take a touch's drag.
type dragRecorder struct {
	*scrollRecorder
}

func (dragRecorder) TouchDrags(platform.Window, float32, float32) bool { return true }

func (dragRecorder) TouchLongPress(platform.Window, float32, float32) platform.LongPress {
	return platform.LongPressNone
}

// A touch on an element that takes the drag presses the mouse and drags it,
// and scrolls nothing.
func TestTouchDragsElement(t *testing.T) {
	w, got := eventWindow(t)
	rec := w.handler.(*scrollRecorder)
	w.handler = dragRecorder{rec}
	w.pointer(touchEvent(pointerDown, 1, 100, 300, 10))
	runPosted()
	if !w.in.dragging {
		t.Fatal("the touch does not drag the element under it")
	}
	w.pointer(touchEvent(pointerMove, 1, 100, 340, 30))
	w.pointer(touchEvent(pointerUp, 1, 100, 340, 60))
	runPosted()
	if !sameKinds(kinds(*got), platform.EventMouseMove, platform.EventMouseDown, platform.EventMouseMove,
		platform.EventMouseUp, platform.EventMouseLeave) {
		t.Errorf("events %v, want a press, a move, a release and the pointer leaving", kinds(*got))
	}
	if len(rec.scrolls) != 0 {
		t.Errorf("the drag scrolled %d times", len(rec.scrolls))
	}
}

// A finger that moved before the app could say whether its element takes the
// drag pans, and the answer that comes later does not turn the pan into a drag.
func TestTouchPansBeforeDragAnswer(t *testing.T) {
	w, got := eventWindow(t)
	rec := w.handler.(*scrollRecorder)
	w.handler = dragRecorder{rec}
	w.pointer(touchEvent(pointerDown, 1, 100, 300, 10))
	w.pointer(touchEvent(pointerMove, 1, 100, 340, 30)) // before the posted work runs
	runPosted()
	if w.in.dragging || !w.in.panning {
		t.Errorf("dragging %v, panning %v; want the pan that the touch became", w.in.dragging, w.in.panning)
	}
	for _, e := range *got {
		if e.Kind == platform.EventMouseDown {
			t.Error("the pan pressed the mouse")
		}
	}
}

// The edits that a phone's keyboard makes without a key press arrive as key
// presses, text goes on to the input event, and formatting is refused.
func TestBeforeInput(t *testing.T) {
	for _, c := range []struct {
		inputType string
		prevent   bool
		want      []platform.EventKind
	}{
		{"deleteContentBackward", true, []platform.EventKind{platform.EventKeyDown, platform.EventKeyUp}},
		{"insertParagraph", true, []platform.EventKind{platform.EventKeyDown, platform.EventKeyUp}},
		{"insertText", false, nil},
		{"formatBold", true, nil},
	} {
		w, got := eventWindow(t)
		if prevent := w.beforeInput(c.inputType); prevent != c.prevent {
			t.Errorf("%s: prevented %v, want %v", c.inputType, prevent, c.prevent)
		}
		runPosted()
		if !sameKinds(kinds(*got), c.want...) {
			t.Errorf("%s: events %v, want %v", c.inputType, kinds(*got), c.want)
		}
	}
	// A Backspace whose key press was just delivered is not delivered again.
	w, got := eventWindow(t)
	w.text.focused = true
	w.key(keyEvent{down: true, code: "Backspace", key: "Backspace"})
	if !w.beforeInput("deleteContentBackward") {
		t.Error("the edit of a delivered press is not prevented")
	}
	runPosted()
	if !sameKinds(kinds(*got), platform.EventKeyDown) {
		t.Errorf("events %v, want the one press", kinds(*got))
	}
}

// Text typed in the textarea is delivered and the textarea emptied, except
// while an input method composes, whose text arrives when it commits.
func TestTextInputAndComposition(t *testing.T) {
	w, got := eventWindow(t)
	if !w.textInput("a") {
		t.Error("typed text leaves the textarea full")
	}
	w.composition(compositionStart, "")
	if w.textInput("に") {
		t.Error("an input during a composition empties the textarea")
	}
	w.composition(compositionUpdate, "に")
	if !w.composition(compositionEnd, "日本") {
		t.Error("the end of a composition leaves the textarea full")
	}
	if w.composition(compositionEnd, "日本") {
		t.Error("the end of a composition that is over empties the textarea")
	}
	runPosted()
	if !sameKinds(kinds(*got), platform.EventTextInput, platform.EventComposition, platform.EventComposition, platform.EventTextInput) {
		t.Fatalf("events %v, want the text, the composition, its end and its text", kinds(*got))
	}
	if e := (*got)[1]; e.Text != "に" || e.Cursor != len("に") {
		t.Errorf("the composition arrived as %+v", e)
	}
	if e := (*got)[3]; e.Text != "日本" {
		t.Errorf("the composition committed %q, want 日本", e.Text)
	}
}

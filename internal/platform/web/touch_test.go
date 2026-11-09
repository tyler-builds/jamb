//go:build js

package web

import (
	"math"
	"syscall/js"
	"testing"

	"github.com/tyler-builds/jamb/internal/platform"
)

// scrollRecorder is a Handler that keeps the scrolls it gets.
type scrollRecorder struct {
	platform.Handler
	scrolls []platform.Event
	onEvent func(platform.Event) // sees every event, when set
}

func (r *scrollRecorder) OnEvent(_ platform.Window, e platform.Event) {
	if e.Kind == platform.EventScroll {
		r.scrolls = append(r.scrolls, e)
	}
	if r.onEvent != nil {
		r.onEvent(e)
	}
}

func (r *scrollRecorder) OnFrame(platform.Window) bool { return false }

// touchWindow returns a window for touch tests, at a pixel ratio of 1, whose
// events go to the recorder.
func touchWindow(t *testing.T) (*window, *scrollRecorder) {
	focus := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	t.Cleanup(focus.Release)
	canvas := js.Global().Get("Object").New()
	canvas.Set("focus", focus)
	rec := &scrollRecorder{}
	return &window{handler: rec, canvas: canvas, scale: 1, ready: true, cssW: 400, cssH: 800}, rec
}

// pointer is a finger's move to x, y, in CSS pixels relative to the canvas.
func pointer(id int, x, y, at float64) pointerEvent {
	return pointerEvent{kind: pointerMove, touch: true, id: id, x: x, y: y, at: at}
}

// A pan scrolls once in each frame, by all that the finger moved since the
// last frame, however many moves the frame had.
func TestPanScrollsOncePerFrame(t *testing.T) {
	w, rec := touchWindow(t)
	p := &w.in
	p.touching, p.touchID = true, 1
	p.touchX0, p.touchY0, p.touchX, p.touchY = 100, 500, 100, 500
	p.moves = []touchMove{{0, 100, 500}}

	// Moves 8 ms apart against frames 16.7 ms apart: two moves in some
	// frames, one in others.
	y, at, frame := 500.0, 0.0, 0.0
	var want []float32
	for range 6 {
		frame += 1000.0 / 60
		moved := 0.0
		for at+8 <= frame {
			at += 8
			y -= 6
			moved += 6
			w.touchMove(pointer(1, 100, y, at))
		}
		w.touchFrame(frame)
		want = append(want, float32(moved))
	}
	// The first frame's moves cross the slop; it scrolls by them too, so the
	// content stays under the finger.
	if len(rec.scrolls) != len(want) {
		t.Fatalf("%d scrolls for %d frames, want one each", len(rec.scrolls), len(want))
	}
	total := float32(0)
	for i, s := range rec.scrolls {
		total += s.DeltaY
		if s.DeltaY != want[i] {
			t.Errorf("frame %d scrolled %v, want the %v moved since the last frame", i, s.DeltaY, want[i])
		}
	}
	if total != float32(500-y) {
		t.Errorf("the pan scrolled %v in all, and the finger moved %v", total, 500-y)
	}
}

// The momentum scrolls in each frame by how far it carries in the frame's
// time, so equal frames scroll by steadily shrinking steps, with no frame of
// two steps or of none.
func TestFlingFollowsFrames(t *testing.T) {
	w, rec := touchWindow(t)
	p := &w.in
	p.flinging, p.flingVX, p.flingVY, p.flingAt = true, 0, 2, 1000
	frame := 1000.0
	for range 30 {
		frame += 1000.0 / 60
		w.touchFrame(frame)
	}
	if len(rec.scrolls) != 30 {
		t.Fatalf("%d scrolls in 30 frames, want 30", len(rec.scrolls))
	}
	ratio := float64(rec.scrolls[1].DeltaY / rec.scrolls[0].DeltaY)
	for i := 1; i < len(rec.scrolls); i++ {
		r := float64(rec.scrolls[i].DeltaY / rec.scrolls[i-1].DeltaY)
		if math.Abs(r-ratio) > 1e-3 {
			t.Fatalf("step %d shrank by %v, step 1 by %v: the steps are uneven", i, r, ratio)
		}
	}
	// A frame that comes late scrolls by the longer time, not by one step.
	before := len(rec.scrolls)
	last := rec.scrolls[before-1].DeltaY
	frame += 2 * 1000.0 / 60
	w.touchFrame(frame)
	if got := rec.scrolls[before].DeltaY; got < 1.9*last*float32(ratio) {
		t.Errorf("a frame two frames late scrolled %v after a step of %v", got, last)
	}
}

// A touch during momentum stops it, and a touch that lifts without moving
// taps what it landed on, however fast the momentum was.
func TestTapDuringFling(t *testing.T) {
	w, rec := touchWindow(t)
	taps := 0
	rec.onEvent = func(e platform.Event) {
		if e.Kind == platform.EventMouseUp {
			taps++
		}
	}
	withPage(t, w)
	p := &w.in
	p.flinging, p.flingVX, p.flingVY, p.flingAt = true, 0, 3, 0
	if !w.pointer(touchEvent(pointerDown, 1, 100, 300, 10)) {
		t.Error("a touch's press keeps its default action, which moves the focus")
	}
	runPosted()
	if p.flinging {
		t.Error("a touch left the momentum going")
	}
	w.pointer(touchEvent(pointerUp, 1, 100, 300, 60))
	runPosted()
	if taps != 1 {
		t.Errorf("a tap during fast momentum delivered %d clicks, want 1", taps)
	}
}

// touchEvent is a touch's pointer event of the given kind.
func touchEvent(kind pointerKind, id int, x, y, at float64) pointerEvent {
	e := pointer(id, x, y, at)
	e.kind = kind
	return e
}

// withPage gives the test a page whose focus is on w's canvas, and has
// runNow post its work, which the test runs with runPosted.
func withPage(t *testing.T, w *window) {
	g := js.Global()
	old := g.Get("document")
	doc := g.Get("Object").New()
	doc.Set("activeElement", w.canvas)
	doc.Set("visibilityState", "visible")
	g.Set("document", doc)
	uiActive.Store(true)
	t.Cleanup(func() {
		uiActive.Store(false)
		g.Set("document", old)
	})
}

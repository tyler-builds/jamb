package ui

import (
	"testing"
	"time"
)

// TestCaretBlinkWakeRepaints checks that the frame a blink wake-up asks for repaints the caret of a
// focused text field. Once the focus has settled, nothing else changes between the frames, so only
// the blink can repaint it.
func TestCaretBlinkWakeRepaints(t *testing.T) {
	v := NewSignal("hello")
	h := NewFocusHandle()
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Child(TextInput(v).FocusHandle(h))
	})
	r.now = time.Now()
	var wakes []time.Time
	r.rt.hostSetServices(platformServices{WakeAt: func(at time.Time) { wakes = append(wakes, at) }})
	h.Focus()
	r.rt.Frame(r.now)
	// The focus animation runs for a while; frames step past it.
	for i := 0; i < 40 && r.rt.animating; i++ {
		r.now = r.now.Add(50 * time.Millisecond)
		r.rt.Frame(r.now)
	}
	if r.rt.animating {
		t.Fatal("the focus animation did not end")
	}
	if len(wakes) == 0 {
		t.Fatal("a focused text field asked for no blink wake-up")
	}
	wake := wakes[len(wakes)-1]
	wakes = nil
	// A frame before the wake-up paints nothing, so a paint at the wake-up is the blink's.
	r.now = r.now.Add(time.Millisecond)
	r.rt.Frame(r.now)
	if n := r.rt.Stats().NodesPainted; n != 0 {
		t.Fatalf("an idle frame painted %d nodes", n)
	}
	r.now = wake
	r.rt.Frame(r.now)
	if r.rt.Stats().NodesPainted == 0 {
		t.Errorf("the blink wake-up repainted nothing, so the caret does not blink")
	}
	if len(wakes) == 0 {
		t.Error("the frame after a blink asked for no further wake-up")
	}
}

// A window keeps only the earliest wake-up it was asked for. A timer that
// comes before a caret's next blink, such as a search's debounce after a key,
// takes the window's wake-up, and the frame it wakes for asks again for the
// blink, which would otherwise stop.
func TestWakeAfterEarlierTimer(t *testing.T) {
	v := NewSignal("hello")
	h := NewFocusHandle()
	var after func(time.Duration, func()) func()
	r := newEditorRig(t, func(ctx *Context) Element {
		after = ctx.After
		return Div().Child(TextInput(v).FocusHandle(h))
	})
	r.now = time.Now()
	r.rt.clock = func() time.Time { return r.now }
	// A window's wake-up, as the platforms keep it: the earliest asked for,
	// until it fires.
	var pending time.Time
	r.rt.hostSetServices(platformServices{WakeAt: func(at time.Time) {
		if pending.IsZero() || at.Before(pending) {
			pending = at
		}
	}})
	h.Focus()
	r.rt.Frame(r.now)
	for i := 0; i < 40 && r.rt.animating; i++ {
		r.now = r.now.Add(50 * time.Millisecond)
		r.rt.Frame(r.now)
	}
	blink := pending
	if blink.IsZero() {
		t.Fatal("a focused text field asked for no blink wake-up")
	}
	ran := false
	after(blink.Sub(r.now)/2, func() { ran = true }) // before the blink
	if !pending.Before(blink) {
		t.Fatal("the timer did not take the window's wake-up")
	}
	r.now, pending = pending, time.Time{} // the wake-up fires
	r.rt.Frame(r.now)
	if !ran {
		t.Fatal("the timer did not run at its wake-up")
	}
	if pending.IsZero() || pending.After(blink) {
		t.Errorf("after the timer's frame the window wakes at %v, want the blink's %v", pending, blink)
	}
}

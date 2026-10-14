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

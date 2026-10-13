package ui

import (
	"testing"
	"time"
)

// firstEditor returns the first text editor in the subtree of n, or nil.
func firstEditor(n *node) *node {
	if n.kind == kindEditor {
		return n
	}
	for _, c := range n.children {
		if e := firstEditor(c); e != nil {
			return e
		}
	}
	return nil
}

// In a window that is not active the caret is hidden and nothing blinks: the
// runtime asks for no frames and schedules no wake-ups. Activating the window
// makes the caret blink again.
func TestCaretDoesNotBlinkInAnInactiveWindow(t *testing.T) {
	v := NewSignal("hello")
	h := NewFocusHandle()
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Child(TextInput(v).FocusHandle(h))
	})
	// The rig's first frame ran on the real clock, so the test clock starts there too.
	r.now = time.Now()
	var wakes []time.Time
	r.rt.SetServices(Services{WakeAt: func(at time.Time) { wakes = append(wakes, at) }})
	h.Focus()
	r.rt.Frame(r.now)
	ed := firstEditor(r.rt.tree)
	if ed == nil || ed.st == nil || ed.st.edit == nil {
		t.Fatal("no focused text editor")
	}
	es := ed.st.edit
	es.activity = r.now

	r.now = r.now.Add(100 * time.Millisecond)
	r.rt.Frame(r.now)
	if len(wakes) == 0 {
		t.Fatal("an active window's caret schedules no blink")
	}
	if !r.rt.caretVisible(es) {
		t.Fatal("the caret is off in the first half of its blink")
	}

	// The focus change ends the field's focus transition after a few frames.
	wakes = nil
	r.rt.WindowFocus(false)
	for i := 0; i < 100 && r.rt.NeedsFrame(); i++ {
		r.now = r.now.Add(16 * time.Millisecond)
		r.rt.Frame(r.now)
	}
	r.now = r.now.Add(time.Second)
	r.rt.Frame(r.now)
	if r.rt.caretVisible(es) {
		t.Error("the caret is drawn in an inactive window")
	}
	if len(wakes) != 0 {
		t.Errorf("an inactive window scheduled %d blink wake-ups, want none", len(wakes))
	}
	if r.rt.NeedsFrame() {
		t.Error("an inactive window with a caret asks for frames")
	}

	r.rt.WindowFocus(true)
	r.rt.Frame(r.now)
	if len(wakes) == 0 {
		t.Error("the caret does not blink again once the window is active")
	}
}

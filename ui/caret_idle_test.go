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
	r.rt.hostSetServices(platformServices{WakeAt: func(at time.Time) { wakes = append(wakes, at) }})
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
	r.rt.hostWindowFocus(false)
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

	r.rt.hostWindowFocus(true)
	r.rt.Frame(r.now)
	if len(wakes) == 0 {
		t.Error("the caret does not blink again once the window is active")
	}
}

// A caret that stopped blinking after caretBlinkStop without activity blinks
// again when the window is activated, as when the user comes back to it: on at
// once, then off after the blink period, with a wake-up for each change.
func TestCaretBlinksAgainWhenTheWindowIsActivated(t *testing.T) {
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
	ed := firstEditor(r.rt.tree)
	if ed == nil || ed.st == nil || ed.st.edit == nil {
		t.Fatal("no focused text editor")
	}
	es := ed.st.edit
	es.activity = r.now

	// The user leaves the window, and comes back after the blink has stopped.
	r.rt.hostWindowFocus(false)
	for i := 0; i < 100 && r.rt.NeedsFrame(); i++ {
		r.now = r.now.Add(16 * time.Millisecond)
		r.rt.Frame(r.now)
	}
	r.now = r.now.Add(caretBlinkStop + time.Second)
	r.rt.Frame(r.now)
	wakes = nil
	r.rt.clock = func() time.Time { return r.now }
	r.rt.hostWindowFocus(true)
	r.now = r.now.Add(10 * time.Millisecond)
	r.rt.Frame(r.now)
	if !r.rt.caretVisible(es) {
		t.Fatal("the caret is off right after the window was activated")
	}
	if len(wakes) == 0 {
		t.Fatal("the caret of a window activated after its blink stopped schedules no blink")
	}
	r.now = r.now.Add(r.rt.sys.CaretBlink)
	r.rt.Frame(r.now)
	if r.rt.caretVisible(es) {
		t.Error("the caret does not go off one blink period after the window was activated")
	}

	// Without activity the restarted blink stops again after caretBlinkStop.
	r.now = r.now.Add(caretBlinkStop)
	r.rt.Frame(r.now)
	wakes = nil
	if !r.rt.caretVisible(es) || len(wakes) != 0 {
		t.Errorf("caret on = %v with %d wake-ups after caretBlinkStop, want solid with none", r.rt.caretVisible(es), len(wakes))
	}
}

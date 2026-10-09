package ui

import (
	"testing"
)

// A snapshot of the focused field restores it at once, even after focus moved
// to another field and a frame ran.
func TestFocusedHandleRestoresFieldAtOnce(t *testing.T) {
	a, b := NewFocusHandle(), NewFocusHandle()
	va, vb := NewSignal("one"), NewSignal("two")
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(TextInput(va).FocusHandle(a)).
			Child(TextInput(vb).FocusHandle(b))
	})
	a.Focus()
	r.frame()
	saved := r.rt.FocusedHandle()
	b.Focus()
	r.frame()
	if !saved.FocusNow() {
		t.Fatal("FocusNow of a field still in the tree reported false")
	}
	if !a.Focused() {
		t.Error("FocusNow did not focus the first field immediately")
	}
	if b.Focused() {
		t.Error("the second field kept focus")
	}
}

// A snapshot taken with nothing focused clears focus when restored.
func TestFocusedHandleNothingFocusedClearsFocus(t *testing.T) {
	a := NewFocusHandle()
	va := NewSignal("")
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().Child(TextInput(va).FocusHandle(a))
	})
	saved := r.rt.FocusedHandle()
	a.Focus()
	r.frame()
	if !saved.FocusNow() || r.rt.focusID != 0 {
		t.Errorf("FocusNow of a nothing-focused snapshot: focus %d, want none", r.rt.focusID)
	}

	// Focus on a "nothing focused" handle clears focus on the next frame.
	a.Focus()
	r.frame()
	saved.Focus()
	if !a.Focused() {
		t.Error("Focus on a nothing-focused handle cleared focus before the next frame")
	}
	r.frame()
	if r.rt.focusID != 0 {
		t.Errorf("after the next frame focus is %d, want none", r.rt.focusID)
	}
}

// Restoring focus to a field that has gone does nothing, and does not wait to
// take focus if a field with the same identity appears later.
func TestFocusNowOfRemovedFieldDoesNotWait(t *testing.T) {
	show := NewSignal(true)
	a, b := NewFocusHandle(), NewFocusHandle()
	va, vb := NewSignal(""), NewSignal("")
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Column().
			Child(If(show.Get(), TextInput(va).Key("gone").FocusHandle(a))).
			Child(TextInput(vb).Key("other").FocusHandle(b))
	})
	a.Focus()
	r.frame()
	saved := r.rt.FocusedHandle()
	show.Set(false)
	b.Focus()
	r.frame()
	if saved.FocusNow() {
		t.Fatal("FocusNow reported true for a field that is no longer in the tree")
	}
	if !b.Focused() {
		t.Fatal("FocusNow of a removed field moved focus")
	}
	show.Set(true) // the same field comes back
	r.frame()
	r.frame()
	if !b.Focused() {
		t.Error("the restore waited and took focus when the field came back")
	}
}

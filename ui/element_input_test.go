package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/internal/scene"
)

// A focused element that takes the composition itself gets each change of it,
// an empty one when the input method ends it, and its commit when focus moves
// away. Typed text ends the composition before it arrives, and the element's
// caret places the input method's window.
func TestElementTakesComposition(t *testing.T) {
	var last Composition
	var shown []Composition
	var committed, typed []string
	cancels := 0
	field, other := NewFocusHandle(), NewFocusHandle()
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("canvas").Size(Px(200), Px(40)).FocusHandle(field).
				OnComposition(func(c Composition) {
					last = c
					shown = append(shown, c)
				}).
				OnCommitComposition(func() {
					committed = append(committed, last.Text)
					last = Composition{}
				}).
				CompositionCaret(func() scene.Rect { return scene.Rect{X: 5, Y: 6, W: 2, H: 14} }).
				OnTextInput(func(s string) { typed = append(typed, s) })).
			Child(Button("Other").FocusHandle(other))
	})
	r.rt.services.CancelComposition = func() { cancels++ }
	field.FocusNow()
	frame(r.rt)
	h := (*runtimeHost)(r.rt)

	h.Compose("か", 3, 0, 3)
	frame(r.rt)
	if last.Text != "か" || last.Cursor != 3 || last.ClauseEnd != 3 || r.rt.composing == 0 {
		t.Fatalf("the element got %+v while composing", last)
	}
	f, ok := r.rt.Find("canvas")
	if !ok {
		t.Fatal("the canvas is not in the tree")
	}
	if rect, ok := h.TextInputRect(); !ok || rect.X != f.X+5 || rect.Y != f.Y+6 {
		t.Fatalf("the input rectangle is %+v (%v), want the element's caret at %v, %v", rect, ok, f.X+5, f.Y+6)
	}

	h.Compose("", 0, 0, 0)
	frame(r.rt)
	if last.Text != "" || r.rt.composing != 0 || cancels != 0 {
		t.Fatalf("composing %d, %d cancels, %q after the input method ended it", r.rt.composing, cancels, last.Text)
	}

	h.Compose("中", 3, 0, 0)
	r.rt.TextInput("中")
	frame(r.rt)
	if len(typed) != 1 || typed[0] != "中" || r.rt.composing != 0 || last.Text != "" {
		t.Fatalf("typed %q, composing %d after a confirmed result", typed, r.rt.composing)
	}

	h.Compose("中文", 6, 0, 0)
	frame(r.rt)
	other.FocusNow()
	frame(r.rt)
	if len(committed) != 1 || committed[0] != "中文" || cancels != 1 || r.rt.composing != 0 {
		t.Fatalf("committed %q, %d cancels after focus moved", committed, cancels)
	}
}

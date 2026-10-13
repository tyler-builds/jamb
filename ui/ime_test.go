package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// imeRig is a field and a button in a 500x400 window, the field focused with
// "ab" typed, the number of times the input method was told to cancel, and
// the button's focus handle.
func imeRig(t *testing.T, v *Signal[string], readOnly bool) (*editorRig, *int, *FocusHandle) {
	t.Helper()
	cancels := 0
	field, other := NewFocusHandle(), NewFocusHandle()
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(TextInput(v).FocusHandle(field).ReadOnly(readOnly)).
			Child(Button("Other").FocusHandle(other))
	})
	r.rt.services.CancelComposition = func() { cancels++ }
	field.FocusNow()
	frame(r.rt)
	if _, es := r.editor(); es == nil {
		t.Fatal("the field did not take focus")
	}
	if !readOnly {
		r.typeText("ab")
	}
	return r, &cancels, other
}

func (r *editorRig) compose(text string, cursor, clauseStart, clauseEnd int) {
	(*runtimeHost)(r.rt).Compose(text, cursor, clauseStart, clauseEnd)
	frame(r.rt)
}

// A composition shows at the caret without changing the value, and the text
// the input method confirms takes its place.
func TestCompositionShowsInPlace(t *testing.T) {
	v := NewSignal("")
	r, cancels, _ := imeRig(t, v, false)
	r.key(input.KeyLeft, 0)            // a|b
	r.now = r.now.Add(2 * time.Second) // a new undo step
	r.compose("にほん", 6, 3, 9)
	n, es := r.editor()
	if v.Peek() != "ab" {
		t.Fatalf("composing changed the value to %q", v.Peek())
	}
	if got := es.layout.Text; got != "aにほんb" {
		t.Fatalf("the field shows %q, want the composition at the caret", got)
	}
	if !r.rt.hasTextInput {
		t.Fatal("no text input rectangle while composing")
	}
	// The candidate list opens at the clause, after the composition's first
	// character.
	clauseX, _ := es.layout.Caret(len("aに"))
	if got := r.rt.textInput.X - n.abs.X - es.originX; got < clauseX-2 || got > clauseX+2 {
		t.Errorf("the input rectangle is at %v in the text, want the clause at %v", got, clauseX)
	}

	r.rt.TextInput("日本")
	(*runtimeHost)(r.rt).Compose("", 0, 0, 0)
	frame(r.rt)
	if v.Peek() != "a日本b" {
		t.Fatalf("value %q after the result, want a日本b", v.Peek())
	}
	if es.caret != len("a日本") || es.comp.text != "" || r.rt.composing != 0 {
		t.Fatalf("caret %d, composition %q after the result", es.caret, es.comp.text)
	}
	if *cancels != 0 {
		t.Error("a confirmed composition was canceled")
	}
	r.key(input.KeyZ, ctrl)
	if v.Peek() != "ab" {
		t.Errorf("undo gave %q, want the text before the composition", v.Peek())
	}
}

// An input method that is canceled types nothing.
func TestCompositionCanceled(t *testing.T) {
	v := NewSignal("")
	r, _, _ := imeRig(t, v, false)
	r.compose("か", 3, 0, 0)
	r.compose("", 0, 0, 0)
	_, es := r.editor()
	if v.Peek() != "ab" || es.layout.Text != "ab" || r.rt.composing != 0 {
		t.Fatalf("value %q, shown %q after a canceled composition", v.Peek(), es.layout.Text)
	}
}

// A composition replaces the selection, as typing does.
func TestCompositionReplacesSelection(t *testing.T) {
	v := NewSignal("")
	r, _, _ := imeRig(t, v, false)
	r.key(input.KeyA, ctrl)
	r.compose("한", 3, 0, 0)
	if v.Peek() != "" {
		t.Fatalf("value %q while composing over a selection, want it removed", v.Peek())
	}
	r.rt.TextInput("한")
	frame(r.rt)
	if v.Peek() != "한" {
		t.Fatalf("value %q, want 한", v.Peek())
	}
}

// Moving focus away types the composition, before the field's OnBlur, and
// tells the input method to drop it.
func TestCompositionCommittedOnFocusChange(t *testing.T) {
	v := NewSignal("")
	r, cancels, other := imeRig(t, v, false)
	r.compose("中", 3, 0, 3)
	other.FocusNow()
	frame(r.rt)
	if v.Peek() != "ab中" {
		t.Fatalf("value %q after focus moved, want the composition typed", v.Peek())
	}
	if *cancels != 1 || r.rt.composing != 0 {
		t.Fatalf("the input method was told to cancel %d times, want 1", *cancels)
	}
}

// A click in the field types the composition, then places the caret.
func TestCompositionCommittedOnClick(t *testing.T) {
	v := NewSignal("")
	r, cancels, _ := imeRig(t, v, false)
	r.compose("中文", 6, 0, 0)
	n, es := r.editor()
	r.click(n.abs.X+es.originX+1, n.abs.Y+n.abs.H/2) // before the first character
	if v.Peek() != "ab中文" || *cancels != 1 {
		t.Fatalf("value %q, %d cancels after a click", v.Peek(), *cancels)
	}
	if es.caret != 0 {
		t.Errorf("caret %d after the click, want 0", es.caret)
	}
}

// A key the field handles types the composition first.
func TestCompositionCommittedOnKey(t *testing.T) {
	v := NewSignal("")
	r, cancels, _ := imeRig(t, v, false)
	r.compose("中", 3, 0, 0)
	r.key(input.KeyHome, 0)
	_, es := r.editor()
	if v.Peek() != "ab中" || es.caret != 0 || *cancels != 1 {
		t.Fatalf("value %q, caret %d, %d cancels after Home", v.Peek(), es.caret, *cancels)
	}
}

// A read-only field shows no composition, and a field that goes away while
// composing tells the input method to drop it.
func TestCompositionReadOnlyAndRemoved(t *testing.T) {
	v := NewSignal("x")
	r, _, _ := imeRig(t, v, true)
	r.compose("か", 3, 0, 0)
	if _, es := r.editor(); es.layout.Text != "x" || r.rt.composing != 0 {
		t.Fatalf("a read-only field shows %q", es.layout.Text)
	}

	show := NewSignal(true)
	cancels := 0
	field := NewFocusHandle()
	r = newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(If(show.Get(), TextInput(NewSignal("")).FocusHandle(field)))
	})
	r.rt.services.CancelComposition = func() { cancels++ }
	field.FocusNow()
	frame(r.rt)
	r.compose("か", 3, 0, 0)
	show.Set(false)
	frame(r.rt)
	if r.rt.composing != 0 || cancels != 1 {
		t.Fatalf("composing %d, %d cancels after the field went away", r.rt.composing, cancels)
	}
}

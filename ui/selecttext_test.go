package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// textSelectRig is a selectable text, a button before it, and a plain text after
// it, in a 500x400 window.
func textSelectRig(t *testing.T, el *TextElement) *editorRig {
	t.Helper()
	return newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).Child(Button("Before").Key("before")).
			Child(el.Key("sel")).Child(Text("not selectable").Key("plain"))
	})
}

// at returns the window position of byte i of the selectable text, on its
// first line's middle.
func (r *editorRig) at(t *testing.T, i int) (float32, float32) {
	t.Helper()
	n := r.rt.byID[r.rt.keyID(t, "sel")]
	l := n.st.sel.layout
	if l == nil {
		t.Fatal("the text has not been laid out")
	}
	x, top := l.Caret(i)
	return n.abs.X + x, n.abs.Y + top + l.LineHeight/2
}

// keyID returns the id of the node with key, failing the test without one.
func (rt *Runtime) keyID(t *testing.T, key string) uint64 {
	t.Helper()
	var found uint64
	var walk func(n *node)
	walk = func(n *node) {
		if n.key == key {
			found = n.id
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(rt.tree)
	if found == 0 {
		t.Fatalf("no element with key %q", key)
	}
	return found
}

func (r *editorRig) drag(x0, y0, x1, y1 float32) {
	r.rt.PointerMove(x0, y0, 0)
	r.rt.PointerDown(x0, y0, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(x1, y1, 0)
	r.rt.PointerUp(x1, y1, input.ButtonLeft, 0)
	frame(r.rt)
}

// Dragging selects, Ctrl+C copies the selection, and Ctrl+A selects all.
func TestSelectableTextDragAndCopy(t *testing.T) {
	r := textSelectRig(t, Text("Error 404: page not found").Selectable())
	x0, y := r.at(t, len("Error "))
	x1, _ := r.at(t, len("Error 404"))
	r.drag(x0, y, x1, y)
	r.key(input.KeyC, ctrl)
	if got := r.rt.memClipboard; got != "404" {
		t.Fatalf("copied %q, want 404", got)
	}
	r.key(input.KeyA, ctrl)
	r.key(input.KeyC, ctrl)
	if got := r.rt.memClipboard; got != "Error 404: page not found" {
		t.Fatalf("copied %q after Ctrl+A", got)
	}
	if len(r.rt.scene.Quads) == 0 {
		t.Error("no selection was painted")
	}
}

// A double-click selects a word, a triple-click the paragraph, and Shift+click
// extends the selection.
func TestSelectableTextClicks(t *testing.T) {
	r := textSelectRig(t, Text("first line\nsecond line").Selectable())
	x, y := r.at(t, 2)
	for range 2 {
		r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
		r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	}
	r.key(input.KeyC, ctrl)
	if got := r.rt.memClipboard; got != "first " && got != "first" {
		t.Fatalf("double-click copied %q, want the word", got)
	}
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.key(input.KeyC, ctrl)
	if got := r.rt.memClipboard; got != "first line" {
		t.Fatalf("triple-click copied %q, want the paragraph", got)
	}
	r.now = r.now.Add(time.Second)
	r.click(x, y)
	x2, y2 := r.at(t, 4)
	r.rt.PointerDown(x2, y2, input.ButtonLeft, 1, input.ModShift)
	r.rt.PointerUp(x2, y2, input.ButtonLeft, input.ModShift)
	r.key(input.KeyC, ctrl)
	if got := r.rt.memClipboard; got != "rs" {
		t.Fatalf("Shift+click copied %q, want rs", got)
	}
}

// A click puts focus on the text, Tab does not stop there, and focus moving
// away clears the selection.
func TestSelectableTextFocus(t *testing.T) {
	r := textSelectRig(t, Text("some details").Selectable())
	r.key(input.KeyTab, 0)
	if !r.rt.focusWithin("before") {
		t.Fatal("Tab did not focus the button")
	}
	r.key(input.KeyTab, 0)
	if r.rt.focusWithin("sel") {
		t.Fatal("Tab stopped on the selectable text")
	}
	x, y := r.at(t, 0)
	x1, _ := r.at(t, 4)
	r.drag(x, y, x1, y)
	if !r.rt.focusWithin("sel") {
		t.Fatal("a click did not focus the text")
	}
	n := r.rt.byID[r.rt.focusID]
	r.key(input.KeyTab, 0)
	if a, b := n.st.sel.bounds(); a != b {
		t.Errorf("the selection %d..%d stayed after focus moved", a, b)
	}
}

// In selectable rich text a click on a link follows it rather than starting a
// selection.
func TestSelectableRichTextLink(t *testing.T) {
	clicked := 0
	r := textSelectRig(t, Rich(Seg("See the "), Link("docs", func() { clicked++ }), Seg(" for more")).Selectable())
	x, y := r.at(t, len("See the d"))
	r.click(x, y)
	if clicked != 1 {
		t.Fatalf("the link was clicked %d times", clicked)
	}
	x0, _ := r.at(t, 0)
	x1, _ := r.at(t, 3)
	r.drag(x0, y, x1, y)
	r.key(input.KeyC, ctrl)
	if got := r.rt.memClipboard; got != "See" {
		t.Errorf("copied %q, want See", got)
	}
}

// Plain text is not selectable: Ctrl+C after clicking it copies nothing.
func TestPlainTextNotSelectable(t *testing.T) {
	r := textSelectRig(t, Text("x").Selectable())
	n := r.rt.byID[r.rt.keyID(t, "plain")]
	r.drag(n.abs.X+1, n.abs.Y+5, n.abs.X+n.abs.W-1, n.abs.Y+5)
	r.key(input.KeyC, ctrl)
	if r.rt.memClipboard != "" {
		t.Errorf("copied %q from plain text", r.rt.memClipboard)
	}
}

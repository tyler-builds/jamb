package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// selected returns the selected part of the rig's selectable text.
func (r *editorRig) selected(t *testing.T) string {
	t.Helper()
	n := r.rt.byID[r.rt.keyID(t, "sel")]
	a, b := n.st.sel.bounds()
	return n.textEl.text[a:b]
}

// A finger's long press on a selectable text selects the word under it, its
// drag selects more words, and the text's menu opens when it lifts. The press
// is no click of an element around the text, and a finger's drag there scrolls
// until then.
func TestLongPressSelectsWords(t *testing.T) {
	clicks := 0
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).OnClick(func() { clicks++ }).
			Child(Text("Error 404: page not found").Selectable().Key("sel"))
	})
	x, y := r.at(t, len("Error 4"))
	if r.rt.hostDragsAt(x, y) {
		t.Error("a finger's drag on a selectable text selects, want it to scroll")
	}
	taken, drags := r.rt.hostLongPress(x, y)
	frame(r.rt)
	if !taken || !drags {
		t.Fatalf("long press on the text: taken %v, drags %v; want both", taken, drags)
	}
	if got := r.selected(t); got != "404" {
		t.Fatalf("the long press selected %q, want the word 404", got)
	}
	if r.rt.ctxMenu != nil {
		t.Error("the menu opened before the finger lifted")
	}
	x1, _ := r.at(t, len("Error 404: pa"))
	r.rt.PointerMove(x1, y, 0)
	frame(r.rt)
	if got := r.selected(t); got != "404: page" {
		t.Errorf("the drag selected %q, want the words up to the finger's", got)
	}
	x0, _ := r.at(t, 2)
	r.rt.PointerMove(x0, y, 0)
	frame(r.rt)
	if got := r.selected(t); got != "Error 404" {
		t.Errorf("the drag back selected %q, want the words from the finger's to the first", got)
	}
	r.rt.PointerUp(x0, y, input.ButtonLeft, 0)
	frame(r.rt)
	if r.rt.ctxMenu == nil {
		t.Error("no menu opened when the finger lifted")
	}
	if got := r.selected(t); got != "Error 404" {
		t.Errorf("the selection is %q once the menu is open", got)
	}
	if clicks != 0 {
		t.Errorf("the long press clicked the element around the text %d times", clicks)
	}
	// The next press is a mouse's again.
	r.rt.KeyDown(input.KeyEscape, 0, false)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	frame(r.rt)
	if r.rt.ctxMenu != nil {
		t.Error("a click after the long press opened the menu")
	}
}

// A long press in a text field selects the word, and the field's menu opens
// when the finger lifts.
func TestLongPressTextField(t *testing.T) {
	v := NewSignal("hello brave world")
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(TextInput(v).Label("Greeting").Width(Px(300)))
	})
	var ed *node
	var walk func(n *node)
	walk = func(n *node) {
		if n.editor != nil {
			ed = n
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(r.rt.tree)
	if ed == nil {
		t.Fatal("no editor in the field")
	}
	x, y := ed.abs.X+ed.style.Padding.Left+60, ed.abs.Y+ed.lay.H/2
	if r.rt.hostDragsAt(x, y) {
		t.Error("a finger's drag in a text field selects, want it to scroll")
	}
	if taken, drags := r.rt.hostLongPress(x, y); !taken || !drags {
		t.Fatalf("long press in the field: taken %v, drags %v; want both", taken, drags)
	}
	es := ed.st.edit
	a, b := es.selection()
	if got := v.Get()[a:b]; got != "brave" {
		t.Fatalf("the long press selected %q, want the word brave", got)
	}
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	frame(r.rt)
	if r.rt.ctxMenu == nil {
		t.Error("no menu opened when the finger lifted")
	}
	if r.rt.focusID != ed.id && r.rt.ctxMenu == nil {
		t.Error("the field lost the focus")
	}
}

// A long press opens the context menu of an element that has one, as a right
// click does, and does nothing on one that has none: the touch stays a tap.
func TestLongPressOpensMenu(t *testing.T) {
	clicks := 0
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Width(Px(200)).Height(Px(40)).Key("menu").
				ContextMenu(func() []MenuItem { return []MenuItem{{Label: "Rename"}} })).
			Child(Div().Width(Px(200)).Height(Px(40)).Key("plain").OnClick(func() { clicks++ }))
	})
	if taken, drags := r.rt.hostLongPress(50, 100); taken || drags {
		t.Errorf("long press on an element without a menu: taken %v, drags %v; want neither", taken, drags)
	}
	if clicks != 0 || r.rt.ctxMenu != nil {
		t.Errorf("a long press that did nothing clicked %d times, menu %v", clicks, r.rt.ctxMenu != nil)
	}
	taken, drags := r.rt.hostLongPress(50, 30)
	frame(r.rt)
	if !taken || drags {
		t.Errorf("long press on an element with a menu: taken %v, drags %v; want taken only", taken, drags)
	}
	if r.rt.ctxMenu == nil {
		t.Error("the long press opened no menu")
	}
}

// While a text field's context menu is open, text input stays at the field, so
// that a phone keeps its keyboard up, and the field's selection stays drawn.
func TestFieldMenuKeepsTextInput(t *testing.T) {
	v := NewSignal("hello brave world")
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(TextInput(v).Label("Greeting").Width(Px(300)))
	})
	r.click(60, 30)
	if _, ok := r.rt.hostTextInputRect(); !ok {
		t.Fatal("a focused field has no text input")
	}
	r.rt.PointerDown(60, 30, input.ButtonRight, 1, 0)
	r.rt.PointerUp(60, 30, input.ButtonRight, 0)
	frame(r.rt)
	if r.rt.ctxMenu == nil {
		t.Fatal("the field's menu did not open")
	}
	if _, ok := r.rt.hostTextInputRect(); !ok {
		t.Error("text input left the field while its menu is open")
	}
	r.rt.KeyDown(input.KeyEscape, 0, false)
	frame(r.rt)
	if _, ok := r.rt.hostTextInputRect(); !ok || r.rt.ctxMenu != nil {
		t.Error("the field has no text input after its menu closed")
	}
}

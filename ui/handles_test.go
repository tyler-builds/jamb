package ui

import (
	"math"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// longPress presses a finger long at x, y and lifts it there, as the web
// platform delivers it (see hostLongPress), and runs a frame.
func (r *editorRig) longPress(t *testing.T, x, y float32) {
	t.Helper()
	if taken, drags := r.rt.hostLongPress(x, y); !taken || !drags {
		t.Fatalf("long press at %v,%v: taken %v, drags %v; want both", x, y, taken, drags)
	}
	frame(r.rt)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	frame(r.rt)
}

// knob returns the middle of the knob of handle i (0 the start's, 1 the end's)
// as the last paint drew it.
func (r *editorRig) knob(t *testing.T, i int) (float32, float32) {
	t.Helper()
	e := r.rt.handles.ends[i]
	if !e.ok {
		t.Fatalf("handle %d is not drawn", i)
	}
	return e.x, e.knobY(i == 0)
}

// fingerDrag drags a finger from x0, y0 to x1, y1 as the web platform
// delivers it: the press, when the element there takes the drag, then the
// moves, and the release. It reports whether the press took the drag.
func (r *editorRig) fingerDrag(x0, y0, x1, y1 float32) bool {
	if !r.rt.hostDragsAt(x0, y0) {
		return false
	}
	r.rt.PointerMove(x0, y0, 0)
	r.rt.PointerDown(x0, y0, input.ButtonLeft, 1, 0)
	frame(r.rt)
	for i := 1; i <= 4; i++ {
		f := float32(i) / 4
		r.rt.PointerMove(x0+(x1-x0)*f, y0+(y1-y0)*f, 0)
		frame(r.rt)
	}
	r.rt.PointerUp(x1, y1, input.ButtonLeft, 0)
	frame(r.rt)
	return true
}

// editorNode returns the editor of the rig's only text field.
func (r *editorRig) editorNode(t *testing.T) *node {
	t.Helper()
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
		t.Fatal("no editor")
	}
	return ed
}

// editorAt returns the window position of byte i of the editor ed's text, on
// its line's middle.
func editorAt(ed *node, i int) (float32, float32) {
	es := ed.st.edit
	x, top := es.layout.Caret(es.drawn.toShown(i))
	return ed.abs.X + es.originX + x, ed.abs.Y + es.originY + top + es.layout.LineHeight/2
}

func nearX(a, b float32) bool { return math.Abs(float64(a-b)) < 0.75 }

// A finger's long press on a selectable text shows a handle at each end of the
// word it selected: a bar on the selection's edge, with the start's knob above
// its line and the end's below. A finger grabs a handle within 22 pixels of
// it, and nowhere else on the text.
func TestHandlesShowAfterLongPress(t *testing.T) {
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(Text("Error 404: page not found").Selectable().Key("sel"))
	})
	x, y := r.pos(t, "sel", len("Error 4"))
	r.longPress(t, x, y)
	if got := r.selected(t); got != "404" {
		t.Fatalf("the long press selected %q, want 404", got)
	}
	n := r.rt.byID[r.rt.keyID(t, "sel")]
	l := n.st.sel.layout
	start, end := r.rt.handles.ends[0], r.rt.handles.ends[1]
	if !start.ok || !end.ok {
		t.Fatalf("handles drawn: start %v, end %v; want both", start.ok, end.ok)
	}
	sx, _ := r.pos(t, "sel", len("Error "))
	ex, _ := r.pos(t, "sel", len("Error 404"))
	if !nearX(start.x, sx) || !nearX(end.x, ex) {
		t.Errorf("the handles are at x %v and %v, want the selection's edges %v and %v", start.x, end.x, sx, ex)
	}
	if !nearX(start.top, n.abs.Y) || !nearX(start.h, l.LineHeight) || !nearX(end.top, n.abs.Y) {
		t.Errorf("the handles' bars run from %v (%v high) and %v, want the line's top %v and height %v",
			start.top, start.h, end.top, n.abs.Y, l.LineHeight)
	}
	if kx, ky := r.knob(t, 0); kx != start.x || ky >= start.top {
		t.Errorf("the start's knob is at %v,%v, want above the line at %v", kx, ky, start.top)
	}
	if _, ky := r.knob(t, 1); ky <= end.top+end.h {
		t.Errorf("the end's knob is at y %v, want below the line's bottom %v", ky, end.top+end.h)
	}
	kx, ky := r.knob(t, 1)
	for _, c := range []struct {
		x, y float32
		want int
	}{
		{kx, ky, 1}, {kx + 20, ky + 20, 1}, {kx - 20, ky, 1}, {kx, end.top + 2, 1},
		{kx + 30, ky, -1}, {kx, ky + 30, -1},
	} {
		if got := r.rt.handleAt(c.x, c.y); got != c.want {
			t.Errorf("handleAt %+.0f,%+.0f from the end's knob = %d, want %d", c.x-kx, c.y-ky, got, c.want)
		}
	}
	if r.rt.hostDragsAt(kx, ky) != true {
		t.Error("a finger on the end's knob does not take the drag")
	}
	r.rt.handles.pending = false
	far, _ := r.pos(t, "sel", len("Error 404: page not"))
	if r.rt.hostDragsAt(far, y) {
		t.Error("a finger on the text away from the handles takes the drag, want it to scroll")
	}
}

// Dragging the end's handle moves the end of the selection by characters, and
// the menu opens again when the finger lifts; the selection is what Copy
// copies. Dragging the start's handle past the end makes the end the start.
func TestDragHandles(t *testing.T) {
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(Text("Error 404: page not found").Selectable().Key("sel"))
	})
	x, y := r.pos(t, "sel", len("Error 4"))
	r.longPress(t, x, y)
	if r.rt.ctxMenu == nil {
		t.Fatal("no menu after the long press")
	}
	kx, ky := r.knob(t, 1)
	tx, _ := r.pos(t, "sel", len("Error 404: pa"))
	if !r.fingerDrag(kx, ky, tx, ky) {
		t.Fatal("the end's handle did not take the drag")
	}
	if got := r.selected(t); got != "404: pa" {
		t.Errorf("dragging the end's handle selected %q, want 404: pa", got)
	}
	if m := r.rt.ctxMenu; m == nil {
		t.Error("no menu opened when the finger lifted from the handle")
	} else if host := r.rt.byID[m.host]; host != nil {
		e := r.rt.handles.ends[1]
		s := r.rt.handles.ends[0]
		if top, bottom := host.abs.Y+m.y, host.abs.Y+m.y+m.h; top > s.knobY(true)-handleKnob || bottom < e.knobY(false)+handleKnob {
			t.Errorf("the menu hangs from %v to %v, want around the selection and its knobs, from %v to %v",
				top, bottom, s.knobY(true)-handleKnob, e.knobY(false)+handleKnob)
		}
	}
	if !r.rt.handles.ends[0].ok || !r.rt.handles.ends[1].ok {
		t.Error("the handles went with the drag")
	}
	// The start's handle, dragged past the end, selects from the end.
	kx, ky = r.knob(t, 0)
	tx, _ = r.pos(t, "sel", len("Error 404: page n"))
	if !r.fingerDrag(kx, ky, tx, ky) {
		t.Fatal("the start's handle did not take the drag")
	}
	if got := r.selected(t); got != "ge n" {
		t.Errorf("dragging the start's handle past the end selected %q, want ge n", got)
	}
	r.key(input.KeyEscape, 0) // closes the menu
	if got := r.copied(); got != "ge n" {
		t.Errorf("Copy copied %q, want ge n", got)
	}
}

// A mouse's selection shows no handles, and a mouse's press, a key and a
// finger's press elsewhere take a finger's handles away, as does clearing the
// selection.
func TestHandlesOnlyForFingers(t *testing.T) {
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).Child(Text("Error 404: page not found").Selectable().Key("sel")).
			Child(Text("Other text").Selectable().Key("other"))
	})
	x, y := r.pos(t, "sel", len("Error 4"))
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 2, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	frame(r.rt)
	if got := r.selected(t); got != "404" {
		t.Fatalf("a double click selected %q", got)
	}
	if r.rt.handles.ends[0].ok || r.rt.handles.ends[1].ok {
		t.Error("a mouse's selection shows handles")
	}
	shown := func() bool { return r.rt.handles.ends[0].ok || r.rt.handles.ends[1].ok }
	for _, c := range []struct {
		what string
		do   func()
	}{
		{"a press of the mouse", func() {
			ox, oy := r.pos(t, "other", 2)
			r.click(ox, oy)
		}},
		{"a key", func() { r.key(input.KeyRight, 0) }},
		{"a finger's tap elsewhere", func() {
			ox, oy := r.pos(t, "other", 2)
			if r.rt.hostDragsAt(ox, oy) {
				t.Error("a finger away from the handles takes the drag")
			}
			r.click(ox, oy)
		}},
		{"clearing the selection", func() { r.rt.clearTextSelection(); frame(r.rt) }},
	} {
		r.key(input.KeyEscape, 0)
		r.longPress(t, x, y)
		r.key(input.KeyEscape, 0) // the menu
		if !shown() {
			t.Fatalf("before %s: no handles after a long press", c.what)
		}
		c.do()
		if shown() {
			t.Errorf("%s left the handles", c.what)
		}
		if r.rt.handles.owner != 0 {
			t.Errorf("%s left the handles' owner", c.what)
		}
	}
}

// In a text field, a long press's word shows handles too, and dragging one
// moves that end of the field's selection, which the field's menu then cuts
// or copies.
func TestHandlesInTextField(t *testing.T) {
	v := NewSignal("hello brave new world")
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(TextInput(v).Label("Greeting").Width(Px(300)))
	})
	ed := r.editorNode(t)
	x, y := editorAt(ed, len("hello br"))
	r.longPress(t, x, y)
	es := ed.st.edit
	if a, b := es.selection(); v.Get()[a:b] != "brave" {
		t.Fatalf("the long press selected %q, want brave", v.Get()[a:b])
	}
	sx, _ := editorAt(ed, len("hello "))
	if e := r.rt.handles.ends[0]; !e.ok || !nearX(e.x, sx) {
		t.Fatalf("the start's handle is at %+v, want at x %v", e, sx)
	}
	kx, ky := r.knob(t, 1)
	tx, _ := editorAt(ed, len("hello brave new wor"))
	if !r.fingerDrag(kx, ky, tx, ky) {
		t.Fatal("the end's handle did not take the drag")
	}
	if a, b := es.selection(); v.Get()[a:b] != "brave new wor" {
		t.Errorf("dragging the end's handle selected %q, want brave new wor", v.Get()[a:b])
	}
	if r.rt.ctxMenu == nil {
		t.Error("no menu opened when the finger lifted")
	}
	if r.rt.focusID != ed.id && r.rt.menuHeld != ed.id {
		t.Error("the field lost the focus")
	}
	// Typing over the selection replaces it, and the handles go.
	r.key(input.KeyEscape, 0)
	r.typeText("x")
	if v.Get() != "hello xld" {
		t.Errorf("typing over the selection made %q", v.Get())
	}
	if r.rt.handles.ends[0].ok || r.rt.handles.ends[1].ok {
		t.Error("the handles stayed once the selection was typed over")
	}
}

// Dragging a handle in a SelectAcross group, as a markdown.View is, moves the
// end across its texts, and Copy copies the texts' parts.
func TestHandlesAcrossGroup(t *testing.T) {
	r := selectGroupRig(t)
	x, y := r.pos(t, "a", len("Alpha o"))
	r.longPress(t, x, y)
	if got := r.selOf(t, "a"); got != "one" {
		t.Fatalf("the long press selected %q, want one", got)
	}
	kx, ky := r.knob(t, 1)
	tx, ty := r.pos(t, "b", len("Bravo t"))
	// The end follows the middle of the line the finger held, which is
	// above the knob.
	if !r.fingerDrag(kx, ky, tx, ty+(ky-y)) {
		t.Fatal("the end's handle did not take the drag")
	}
	if a, b := r.selOf(t, "a"), r.selOf(t, "b"); a != "one" || b != "Bravo t" {
		t.Errorf("dragging the end's handle into the next text selected %q and %q, want one and Bravo t", a, b)
	}
	if e := r.rt.handles.ends[1]; !e.ok || !nearX(e.x, tx) {
		t.Errorf("the end's handle is at %+v, want in the second text at x %v", e, tx)
	}
	// The start's handle, dragged down past the end into the third text.
	kx, ky = r.knob(t, 0)
	cx, cy := r.pos(t, "c", len("Char"))
	if !r.fingerDrag(kx, ky, cx, cy+(ky-y)) {
		t.Fatal("the start's handle did not take the drag")
	}
	if a, b, c := r.selOf(t, "a"), r.selOf(t, "b"), r.selOf(t, "c"); a != "" || b != "wo" || c != "Char" {
		t.Errorf("dragging the start's handle past the end selected %q, %q and %q, want nothing, wo and Char", a, b, c)
	}
	r.key(input.KeyEscape, 0)
	if got := r.copied(); got != "wo\n\nChar" {
		t.Errorf("Copy copied %q, want wo, a blank line and Char", got)
	}
}

// A handle dragged below a text area that scrolls its text scrolls it, as a
// mouse's drag does, and the selection grows into the lines that come into
// view.
func TestHandleDragScrollsTextArea(t *testing.T) {
	v := NewSignal("one two\nthree four\nfive six\nseven eight\nnine ten\neleven twelve\nthirteen fourteen")
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Padding(10).Child(TextArea(v).Label("Notes").Width(Px(300)).Height(Px(60)))
	})
	ed := r.editorNode(t)
	x, y := editorAt(ed, 1)
	r.longPress(t, x, y)
	var scroller *node
	for a := ed.parent; a != nil && scroller == nil; a = a.parent {
		if a.style.OverflowY == OverflowScroll && a.st != nil {
			scroller = a
		}
	}
	if scroller == nil {
		t.Fatal("the text area does not scroll its text")
	}
	kx, ky := r.knob(t, 1)
	if !r.rt.hostDragsAt(kx, ky) {
		t.Fatal("the end's handle did not take the drag")
	}
	r.rt.PointerDown(kx, ky, input.ButtonLeft, 1, 0)
	frame(r.rt)
	below := scroller.abs.Bottom() + 10
	for i := 0; i < 6; i++ {
		r.rt.PointerMove(kx, below, 0)
		frame(r.rt)
	}
	r.rt.PointerUp(kx, below, input.ButtonLeft, 0)
	frame(r.rt)
	if scroller.st.scrollY <= 0 {
		t.Error("dragging the handle below the text area did not scroll it")
	}
	if a, b := ed.st.edit.selection(); a != 0 || b <= len("one two\nthree four\nfive") {
		t.Errorf("the selection is %q, want it to run from the start into the lines scrolled into view", v.Get()[a:b])
	}
}

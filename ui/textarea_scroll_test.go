package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// textAreaFrame returns the frame of the first text area under n: the parent of
// its editor.
func textAreaFrame(n *node) *node {
	if n.editor != nil {
		return n.parent
	}
	for _, c := range n.children {
		if f := textAreaFrame(c); f != nil {
			return f
		}
	}
	return nil
}

// boundedTextArea is a page that scrolls, holding a title, a text area that is
// at most 140 pixels tall, and a line under it, as a settings page does.
func boundedTextArea(v *Signal[string], before float32) func(*Context) Element {
	return func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().Child(Div().Column().Gap(8).
			Child(Div().Height(Px(before))).
			Child(TextArea(v).MinRows(1).MaxHeight(Px(140)).Width(Full).Label("Rules")).
			Child(Text("below").Key("below")))
	}
}

// A text area with a MaxHeight grows with its text up to it, and then keeps that
// height and scrolls the text inside its frame: the text is clipped to the frame,
// and what comes after the text area starts under the frame, not under the text.
func TestTextAreaMaxHeightClipsAndScrolls(t *testing.T) {
	v := NewSignal("one line")
	r := newEditorRig(t, boundedTextArea(v, 0))
	fr := textAreaFrame(r.rt.tree)
	if fr == nil {
		t.Fatal("no text area frame")
	}
	short := fr.lay.H
	if short >= 140 {
		t.Fatalf("a one-line text area is %v tall; want less than its MaxHeight", short)
	}
	if fr.lay.ContentH > fr.lay.H+0.5 {
		t.Errorf("a short text overflows its frame: content %v in %v", fr.lay.ContentH, fr.lay.H)
	}

	v.Set(strings.Repeat("a line of text in the rules\n", 30))
	frame(r.rt)
	fr = textAreaFrame(r.rt.tree)
	if fr.lay.H != 140 {
		t.Fatalf("a long text area is %v tall; want its MaxHeight, 140", fr.lay.H)
	}
	if fr.style.OverflowY != OverflowScroll {
		t.Fatalf("the bounded frame's OverflowY is %v; want OverflowScroll", fr.style.OverflowY)
	}
	if fr.lay.ContentH <= fr.lay.H {
		t.Fatalf("the text's height %v does not overflow the frame (%v)", fr.lay.ContentH, fr.lay.H)
	}
	below := findText(r.rt.tree, "below")
	if below == nil {
		t.Fatal("no text under the text area")
	}
	if fb := fr.abs.Bottom(); below.abs.Y < fb {
		t.Errorf("the text under the text area is at y=%v, above the frame's bottom %v", below.abs.Y, fb)
	}
	// The editor is drawn clipped to the frame: everything inside it is clipped to
	// the frame's padding box, inside the border.
	ed := fr.children[0]
	if ed.editor == nil {
		t.Fatal("the frame's first child is not its editor")
	}
	be := fr.style.borderEdges()
	if ed.clip.Bottom() > fr.abs.Bottom()-be.Bottom+0.01 || ed.clip.Y < fr.abs.Y+be.Top-0.01 {
		t.Errorf("the editor's clip %+v is not inside the frame %+v", ed.clip, fr.abs)
	}

	// The wheel over the text area scrolls its text, not the page.
	page := r.rt.tree
	r.rt.Scroll(fr.abs.X+20, fr.abs.Y+20, 0, 60, true, 0)
	frame(r.rt)
	if fr.st.scrollY <= 0 {
		t.Errorf("the wheel over the text area did not scroll it (scrollY %v)", fr.st.scrollY)
	}
	if page.st != nil && page.st.scrollY != 0 {
		t.Errorf("the wheel over a text area that can still scroll moved the page (scrollY %v)", page.st.scrollY)
	}
}

// The caret is kept inside a bounded text area's frame as it moves, by the
// keyboard and by typing.
func TestTextAreaMaxHeightKeepsCaretInView(t *testing.T) {
	v := NewSignal(strings.Repeat("line\n", 30))
	r := newEditorRig(t, boundedTextArea(v, 0))
	fr := textAreaFrame(r.rt.tree)
	r.click(fr.abs.X+20, fr.abs.Y+15)
	if _, es := r.editor(); es == nil {
		t.Fatal("clicking the text area did not focus it")
	}
	inFrame := func(what string) {
		t.Helper()
		caret, ok := r.rt.hostTextInputRect()
		if !ok {
			t.Fatalf("%s: no caret", what)
		}
		if caret.Y < fr.abs.Y || caret.Bottom() > fr.abs.Bottom() {
			t.Errorf("%s: caret at y=%v..%v is outside the frame %v..%v", what, caret.Y, caret.Bottom(), fr.abs.Y, fr.abs.Bottom())
		}
	}
	r.key(input.KeyEnd, ctrl)
	maxY := fr.lay.ContentH - fr.lay.H
	if fr.st.scrollY < maxY-0.5 {
		t.Errorf("Ctrl+End scrolled the text area to %v; want its end, %v", fr.st.scrollY, maxY)
	}
	inFrame("Ctrl+End")
	r.typeText("more")
	inFrame("typing at the end")
	r.key(input.KeyHome, ctrl)
	if fr.st.scrollY != 0 {
		t.Errorf("Ctrl+Home left the text area at scrollY %v", fr.st.scrollY)
	}
	inFrame("Ctrl+Home")
	for range 12 {
		r.key(input.KeyDown, 0)
	}
	if fr.st.scrollY <= 0 {
		t.Error("moving down past the frame's bottom did not scroll the text area")
	}
	inFrame("Down")
	if page := r.rt.tree; page.st != nil && page.st.scrollY != 0 {
		t.Errorf("a text area in view moved the page (scrollY %v)", page.st.scrollY)
	}
}

// A bounded text area low on a page that scrolls keeps the caret in view on
// both: it scrolls its text into its frame, and the page scrolls that part of
// the frame into the window.
func TestTextAreaMaxHeightRevealsInScrollingPage(t *testing.T) {
	v := NewSignal(strings.Repeat("line\n", 30))
	r := newEditorRig(t, boundedTextArea(v, 330))
	fr := textAreaFrame(r.rt.tree)
	page := r.rt.tree
	// The frame starts at 338, and the window is 400 tall: click its first line.
	r.click(fr.abs.X+20, fr.abs.Y+15)
	if _, es := r.editor(); es == nil {
		t.Fatal("clicking the text area did not focus it")
	}
	r.key(input.KeyEnd, ctrl)
	if fr.st.scrollY <= 0 {
		t.Error("Ctrl+End did not scroll the text area")
	}
	if page.st.scrollY <= 0 {
		t.Error("Ctrl+End did not scroll the page to the text area's last line")
	}
	caret, _ := r.rt.hostTextInputRect()
	if caret.Y < 0 || caret.Bottom() > 400 {
		t.Errorf("caret at y=%v..%v is outside the 400px window", caret.Y, caret.Bottom())
	}
	if caret.Y < fr.abs.Y || caret.Bottom() > fr.abs.Bottom() {
		t.Errorf("caret at y=%v..%v is outside the frame %v..%v", caret.Y, caret.Bottom(), fr.abs.Y, fr.abs.Bottom())
	}
}

// A text area with no height bound keeps its text's height, so a page that
// scrolls around it does the scrolling, and its frame is no scroll container.
func TestTextAreaWithoutBoundGrows(t *testing.T) {
	v := NewSignal(strings.Repeat("line\n", 60))
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().Child(TextArea(v))
	})
	fr := textAreaFrame(r.rt.tree)
	if fr.style.OverflowY != OverflowVisible {
		t.Errorf("an unbounded text area's frame has OverflowY %v; want OverflowVisible", fr.style.OverflowY)
	}
	if fr.lay.H <= 400 {
		t.Errorf("an unbounded text area of 60 lines is %v tall; want its text's height", fr.lay.H)
	}
}

// A bounded text area scrolls its text, but it is a text field to assistive
// technology, not a scroll area holding one.
func TestTextAreaMaxHeightIsNoScrollArea(t *testing.T) {
	v := NewSignal(strings.Repeat("line\n", 30))
	ar := newAccessRig(func(ctx *Context) Element {
		return TextArea(v).MaxHeight(Px(100)).Label("Notes")
	})
	tr := ar.tree()
	if n := countRole(tr, access.RoleScrollArea); n != 0 {
		t.Errorf("the text area exposes %d scroll areas; want none:\n%s", n, tr.Dump())
	}
	if tr.Find(access.RoleTextArea, "Notes") == nil {
		t.Errorf("no text area named Notes:\n%s", tr.Dump())
	}
}

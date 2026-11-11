package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// boldMarks marks each "**text**" of a line as bold, with its stars as syntax:
// the marks a Markdown helper gives, in small.
func boldMarks(v string) []TextMark {
	var ms []TextMark
	for i := 0; ; {
		a := strings.Index(v[i:], "**")
		if a < 0 {
			return ms
		}
		a += i
		b := strings.Index(v[a+2:], "**")
		if b < 0 {
			return ms
		}
		b += a + 2
		ms = append(ms,
			TextMark{Start: a, End: a + 2, Syntax: true, ScopeStart: a, ScopeEnd: b + 2},
			TextMark{Start: a + 2, End: b, Style: Highlight{Weight: FontBold}},
			TextMark{Start: b, End: b + 2, Syntax: true, ScopeStart: a, ScopeEnd: b + 2})
		i = b + 2
	}
}

func TestPresentMarksHidesSyntaxAndMapsOffsets(t *testing.T) {
	v := "a **b** c"
	marks := boldMarks(v)
	p := presentMarks(v, marks, hiddenSyntax(marks, false, 0, 0))
	if p.shown != "a b c" {
		t.Fatalf("shown %q, want %q", p.shown, "a b c")
	}
	if len(p.spans) != 1 || p.spans[0].Start != 2 || p.spans[0].End != 3 || p.paint[0].Weight != FontBold {
		t.Errorf("spans %+v paint %+v, want bold over the b at 2..3", p.spans, p.paint)
	}
	for raw, shown := range map[int]int{0: 0, 2: 2, 3: 2, 4: 2, 5: 3, 7: 3, 8: 4, 9: 5} {
		if got := p.toShown(raw); got != shown {
			t.Errorf("toShown(%d) = %d, want %d", raw, got, shown)
		}
	}
	// A click before the b puts the caret before the stars, and after it before
	// the closing ones: both touch the bold text, so its syntax shows.
	if got := p.fromShown(2); got != 2 {
		t.Errorf("fromShown(2) = %d, want 2", got)
	}
	if got := p.fromShown(3); got != 5 {
		t.Errorf("fromShown(3) = %d, want 5", got)
	}
	if got := p.lastFromShown(3); got != 7 {
		t.Errorf("lastFromShown(3) = %d, want 7", got)
	}

	// With the caret at either end of the bold text, or in it, nothing is hidden.
	for _, at := range []int{2, 4, 7} {
		if h := hiddenSyntax(marks, true, at, at); len(h) != 0 {
			t.Errorf("caret at %d hides %v", at, h)
		}
	}
	if h := hiddenSyntax(marks, true, 1, 1); len(h) != 2 {
		t.Errorf("caret at 1 hides %v, want both runs of stars", h)
	}
	// Without focus the syntax hides wherever the caret was.
	if h := hiddenSyntax(marks, false, 4, 4); len(h) != 2 {
		t.Errorf("without focus %v hidden, want both runs of stars", h)
	}
}

func TestTextAreaMarksShowSyntaxOnlyAtTheCaret(t *testing.T) {
	v := NewSignal("say **hi** there\nnext")
	r := newEditorRig(t, func(ctx *Context) Element {
		return TextArea(v).Width(Px(300)).Marks(boldMarks)
	})
	n := r.rt.tree
	for n != nil && n.editor == nil {
		if len(n.children) == 0 {
			t.Fatal("no editor")
		}
		n = n.children[0]
	}
	if got := n.st.edit.drawn.shown; got != "say hi there\nnext" {
		t.Fatalf("without focus the field draws %q", got)
	}

	r.click(5, 5)
	r.key(input.KeyEnd, ctrl)
	_, es := r.editor()
	if es == nil {
		t.Fatal("the field has no focus")
	}
	if got := es.drawn.shown; got != "say hi there\nnext" {
		t.Errorf("caret on the next line: drawn %q", got)
	}

	// Into the bold text: its stars show, and the field lays out again for them.
	r.key(input.KeyHome, ctrl)
	for range len("say ") {
		r.key(input.KeyRight, 0)
	}
	if es.caret != 4 {
		t.Fatalf("caret %d, want 4", es.caret)
	}
	if got := es.drawn.shown; got != v.Peek() {
		t.Errorf("caret at the bold text: drawn %q, want all of it", got)
	}
	if es.layout == nil || !strings.HasPrefix(v.Peek(), es.drawn.shown[:4]) {
		t.Error("no layout of the text drawn")
	}

	// End of the line puts the caret after the closing stars, not before them.
	v.Set("say **hi**")
	frame(r.rt)
	r.key(input.KeyHome, 0)
	r.key(input.KeyEnd, 0)
	if es.caret != len("say **hi**") {
		t.Errorf("End: caret %d, want %d", es.caret, len("say **hi**"))
	}

	// Copy takes the value as typed, syntax and all.
	r.key(input.KeyA, ctrl)
	r.key(input.KeyC, ctrl)
	if r.rt.memClipboard != "say **hi**" {
		t.Errorf("copied %q", r.rt.memClipboard)
	}
}

func TestTextAreaMarksUpAndDownMapThroughHiddenSyntax(t *testing.T) {
	v := NewSignal("**ab** cdef\nxyz")
	r := newEditorRig(t, func(ctx *Context) Element {
		return TextArea(v).Width(Px(300)).Marks(boldMarks)
	})
	r.click(5, 5)
	r.key(input.KeyEnd, ctrl)
	_, es := r.editor()
	r.key(input.KeyUp, 0)
	// The caret was at the end of "xyz", three characters in; on the line above,
	// as drawn ("ab cdef"), that is after "ab ", and so after the hidden stars.
	if es.caret <= len("**ab**") || es.caret > len("**ab** cd") {
		t.Errorf("Up: caret %d, want just after %q", es.caret, "**ab** ")
	}
}

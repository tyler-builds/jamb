package ui

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/text"
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

// The selection of a field is drawn over the background of a mark, and its
// glyphs over both.
func TestTextAreaSelectionDrawsOverMarkBackground(t *testing.T) {
	yellow := RGB(255, 255, 0)
	v := NewSignal("say hi there")
	r := newEditorRig(t, func(ctx *Context) Element {
		return TextArea(v).Width(Px(300)).Marks(func(string) []TextMark {
			return []TextMark{{Start: 4, End: 6, Style: Highlight{Background: yellow}}}
		})
	})
	r.click(5, 5)
	r.key(input.KeyA, ctrl)
	n, _ := r.editor()
	if n == nil {
		t.Fatal("the field has no focus")
	}
	sc := &r.rt.scene
	bg := quadIndex(sc, yellow.scene())
	sel := quadIndex(sc, n.editor.selection.scene())
	if bg < 0 || sel < 0 {
		t.Fatalf("mark background quad at %d, selection quad at %d; want both drawn", bg, sel)
	}
	if sel < bg {
		t.Errorf("the selection (quad %d) is drawn under the mark's background (quad %d)", sel, bg)
	}
}

// slowSpans is how presentMarks styled the pieces of a text before it swept
// its marks: each piece between the bounds of the marks and the hidden ranges
// takes the marks over it, in their order, from all of them.
func slowSpans(v string, marks []TextMark, hide [][2]int) ([]text.Span, []Highlight) {
	bounds := []int{0, len(v)}
	for _, m := range marks {
		bounds = append(bounds, m.Start, m.End)
	}
	for _, r := range hide {
		bounds = append(bounds, r[0], r[1])
	}
	slices.Sort(bounds)
	bounds = slices.Compact(bounds)
	var spans []text.Span
	var paint []Highlight
	removed := 0
	for i := 0; i+1 < len(bounds); i++ {
		x, y := bounds[i], bounds[i+1]
		if slices.ContainsFunc(hide, func(r [2]int) bool { return r[0] <= x && y <= r[1] }) {
			removed += y - x
			continue
		}
		var hl Highlight
		for _, m := range marks {
			if m.Start <= x && y <= m.End {
				hl = hl.over(m.Style)
			}
		}
		if hl == (Highlight{}) {
			continue
		}
		s, t := x-removed, y-removed
		if k := len(spans) - 1; k >= 0 && paint[k] == hl && spans[k].End == s {
			spans[k].End = t
			continue
		}
		sp := text.Span{Start: s, End: t, Weight: hl.Weight, Italic: hl.Italic}
		if hl.Mono {
			sp.Family = text.FamilyMono
		}
		spans, paint = append(spans, sp), append(paint, hl)
	}
	return spans, paint
}

// Marks that overlap, nest and come in any order style the text as they did
// when each piece looked at every mark: later marks over earlier ones.
func TestPresentMarksCombinesOverlappingMarksInOrder(t *testing.T) {
	looks := []Highlight{
		{Weight: FontBold},
		{Italic: true},
		{Color: Hex("#ff0000")},
		{Color: Hex("#0000ff"), Underline: true},
		{Mono: true, Background: Hex("#eeeeee")},
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for round := range 200 {
		v := strings.Repeat("ab cd ", 1+rng.IntN(20))
		var marks []TextMark
		for range rng.IntN(12) {
			a := rng.IntN(len(v))
			b := a + 1 + rng.IntN(len(v)-a)
			m := TextMark{Start: a, End: b, Style: looks[rng.IntN(len(looks))]}
			if rng.IntN(3) == 0 {
				m.Syntax, m.ScopeStart, m.ScopeEnd = true, max(a-2, 0), min(b+2, len(v))
			}
			marks = append(marks, m)
		}
		marks = cleanMarks(marks, v)
		hide := hiddenSyntax(marks, rng.IntN(2) == 0, rng.IntN(len(v)), rng.IntN(len(v)))
		p := presentMarks(v, marks, hide)
		spans, paint := slowSpans(v, marks, hide)
		if !slices.Equal(p.spans, spans) || !slices.Equal(p.paint, paint) {
			t.Fatalf("round %d: %q with marks %+v, hiding %v:\nspans %+v paint %+v\nwant  %+v paint %+v",
				round, v, marks, hide, p.spans, p.paint, spans, paint)
		}
	}
}

// A long text with many marks, as a large paste of Markdown is, is drawn in
// time that grows with its length. Each piece of it used to look at every
// mark, and a paste of a few hundred kilobytes stopped the app for seconds at
// each key, or for minutes in a web page.
func BenchmarkPresentMarksLargeText(b *testing.B) {
	v := strings.Repeat("some **bold** and **more** text\n", 6000) // about 200 KB
	marks := cleanMarks(boldMarks(v), v)
	hide := hiddenSyntax(marks, true, 0, 0)
	b.ReportAllocs()
	for b.Loop() {
		presentMarks(v, marks, hide)
	}
}

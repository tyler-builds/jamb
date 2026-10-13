package text

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

var testSys = NewSystem()

func lay(s string, maxW float32) *Layout {
	return testSys.Layout(s, Style{Size: 16}, maxW)
}

var inf = float32(math.Inf(1))

func TestLatinWrapsAtWordBoundaries(t *testing.T) {
	l := lay("the quick brown fox jumps over the lazy dog", 120)
	if len(l.Lines) < 3 {
		t.Fatalf("got %d lines, want wrapping into at least 3", len(l.Lines))
	}
	for i, ln := range l.Lines {
		if ln.Width > 120.5 {
			t.Errorf("line %d is %v wide, over the 120 limit", i, ln.Width)
		}
		a, b := l.LineBounds(i)
		if strings.HasPrefix(l.Text[a:b], " ") {
			t.Errorf("line %d starts with a space: %q", i, l.Text[a:b])
		}
	}
}

func TestCaretAndHitTestRoundTrip(t *testing.T) {
	s := "Hello, wörld!"
	l := lay(s, inf)
	prev := float32(-1)
	for pos := 0; pos <= len(s); {
		x, _ := l.Caret(pos)
		if x <= prev {
			t.Errorf("caret x not increasing at byte %d: %v after %v", pos, x, prev)
		}
		prev = x
		if got := l.HitTest(x+0.3, l.LineHeight/2); got != pos {
			t.Errorf("HitTest at caret x of byte %d returned %d", pos, got)
		}
		if pos == len(s) {
			break
		}
		_, n := utf8.DecodeRuneInString(s[pos:])
		pos += n
	}
}

func TestArabicIsRightToLeft(t *testing.T) {
	s := "مرحبا"
	l := lay(s, inf)
	if len(l.Lines) != 1 || len(l.Lines[0].Runs) != 1 || !l.Lines[0].Runs[0].RTL {
		t.Fatalf("want one RTL run, got %+v", l.Lines)
	}
	start, _ := l.Caret(0)
	end, _ := l.Caret(len(s))
	if start <= end {
		t.Errorf("RTL caret at start x=%v should be right of the end x=%v", start, end)
	}
}

func TestMixedDirectionRunsInVisualOrder(t *testing.T) {
	l := lay("abc مرحبا def", inf)
	runs := l.Lines[0].Runs
	if len(runs) < 3 {
		t.Fatalf("want at least 3 runs (LTR, RTL, LTR), got %d", len(runs))
	}
	for i := 1; i < len(runs); i++ {
		if runs[i].X < runs[i-1].X {
			t.Errorf("runs not in visual order")
		}
	}
	if runs[0].RTL || !runs[1].RTL {
		t.Errorf("unexpected directions: %v %v", runs[0].RTL, runs[1].RTL)
	}
}

func TestNewlinesAndEmptyLines(t *testing.T) {
	s := "a\n\nb"
	l := lay(s, inf)
	if len(l.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(l.Lines))
	}
	for pos, want := range map[int]int{0: 0, 1: 0, 2: 1, 3: 2, 4: 2} {
		if got := l.LineOf(pos); got != want {
			t.Errorf("LineOf(%d) = %d, want %d", pos, got, want)
		}
	}
	if l.HitTest(50, l.LineHeight*1.5) != 2 {
		t.Error("clicking the empty line should put the caret on it")
	}
	if l.Height != 3*l.LineHeight {
		t.Errorf("height %v, want 3 lines", l.Height)
	}
}

func TestTruncationAddsEllipsis(t *testing.T) {
	l := testSys.Layout("a fairly long sentence that cannot fit", Style{Size: 16, NoWrap: true, MaxLines: 1}, 100)
	if !l.Truncated || len(l.Lines) != 1 {
		t.Fatalf("truncated=%v lines=%d", l.Truncated, len(l.Lines))
	}
	if l.Width > 100.5 {
		t.Errorf("truncated width %v exceeds 100", l.Width)
	}
	runs := l.Lines[0].Runs
	if !runs[len(runs)-1].ellipsis {
		t.Error("last run is not the ellipsis")
	}
}

func TestGraphemesAndWords(t *testing.T) {
	s := "éx👍🏽!"
	if n := testSys.NextGrapheme(s, 0); n != 3 {
		t.Errorf("e + combining accent should be one grapheme: next=%d", n)
	}
	thumb := strings.Index(s, "👍")
	if n := testSys.NextGrapheme(s, thumb); n != thumb+8 {
		t.Errorf("thumbs up with skin tone should be one grapheme: next=%d want %d", n, thumb+8)
	}
	if p := testSys.PrevGrapheme(s, len(s)-1); p != thumb {
		t.Errorf("PrevGrapheme from '!' = %d, want %d", p, thumb)
	}
	w := "hello, big world"
	if a, b := WordAt(w, 9); w[a:b] != "big" {
		t.Errorf("WordAt = %q", w[a:b])
	}
	if NextWord(w, 0) != 5 || PrevWord(w, len(w)) != 11 {
		t.Errorf("NextWord/PrevWord = %d/%d", NextWord(w, 0), PrevWord(w, len(w)))
	}
}

func TestComplexScriptShapingUsesFallback(t *testing.T) {
	s := "नमस्ते" // Devanagari: conjuncts make fewer glyphs than runes
	l := lay(s, inf)
	glyphs := 0
	var name string
	for _, r := range l.Lines[0].Runs {
		glyphs += len(r.Glyphs)
		if len(r.Glyphs) > 0 && r.Glyphs[0].font != nil {
			name = r.Glyphs[0].font.Name
		}
	}
	if !strings.Contains(strings.ToLower(name), "nirmala") {
		t.Skipf("Devanagari not in a Nirmala fallback here (got %q)", name)
	}
	if glyphs >= utf8.RuneCountInString(s) {
		t.Errorf("%d glyphs for %d runes; expected shaping to form conjuncts", glyphs, utf8.RuneCountInString(s))
	}
}

func TestSelectionRects(t *testing.T) {
	s := "one two\nthree"
	l := lay(s, inf)
	rects := l.SelectionRects(4, 11, nil) // "two\nthr"
	if len(rects) != 2 {
		t.Fatalf("got %d rects, want 2 (one per line)", len(rects))
	}
	if rects[0].Y != 0 || rects[1].Y != l.LineHeight {
		t.Errorf("rect rows %v %v", rects[0].Y, rects[1].Y)
	}
}

func BenchmarkLayoutParagraph(b *testing.B) {
	s := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)
	sys := NewSystem()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		// A different width each time defeats the cache: this measures shaping + wrapping.
		sys.build(s, Style{Size: 15}, nil, 300+float32(i)*0.01)
	}
}

// BenchmarkEditLongDocument types one character per iteration into the
// middle of a ~11,000-character, 50-paragraph document and re-lays it out.
func BenchmarkEditLongDocument(b *testing.B) {
	para := strings.Repeat("Notes are mostly prose with the occasional list. ", 4)
	paras := make([]string, 50)
	for i := range paras {
		paras[i] = para
	}
	sys := NewSystem()
	sys.Layout(strings.Join(paras, "\n"), Style{Size: 15}, 600) // warm the paragraph cache
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		paras[25] = para + strings.Repeat("x", i%40+1)
		sys.Layout(strings.Join(paras, "\n"), Style{Size: 15}, 600)
		if i%100 == 0 {
			sys.EndFrame()
		}
	}
}

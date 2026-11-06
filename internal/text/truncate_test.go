package text

import (
	"slices"
	"strings"
	"testing"
)

// shown returns the text a one-line layout draws, with "…" for its ellipsis.
func shown(l *Layout) string {
	ln := l.Lines[0]
	var b strings.Builder
	for _, r := range ln.Runs {
		if r.ellipsis {
			b.WriteString("…")
			continue
		}
		b.WriteString(string(l.runes[r.RuneStart:r.RuneEnd]))
	}
	return b.String()
}

func oneLine(s string, tr Truncation, w float32) *Layout {
	return testSys.Layout(s, Style{Size: 16, NoWrap: true, MaxLines: 1, Truncation: tr}, w)
}

const path = "/home/alex/projects/jamb/examples/git/views/app.go"

// A middle truncation keeps both ends of the text, a head truncation its end,
// and both fit the width.
func TestTruncateMiddleAndHead(t *testing.T) {
	for _, w := range []float32{60, 120, 200, 300} {
		m := oneLine(path, TruncateMiddle, w)
		got := shown(m)
		if len(m.Lines) != 1 || !m.Truncated || m.Text != path {
			t.Fatalf("width %v: lines=%d truncated=%v text=%q", w, len(m.Lines), m.Truncated, m.Text)
		}
		if m.Width > w {
			t.Errorf("width %v: middle line is %v wide", w, m.Width)
		}
		head, tail, ok := strings.Cut(got, "…")
		if !ok || !strings.HasPrefix(path, head) || !strings.HasSuffix(path, tail) || tail == "" {
			t.Errorf("width %v: middle shows %q", w, got)
		}
		if w >= 120 && (head == "" || len(head) < len(tail)/3 || len(tail) < len(head)/3) {
			t.Errorf("width %v: middle is lopsided: %q", w, got)
		}

		h := oneLine(path, TruncateHead, w)
		got = shown(h)
		if h.Width > w || !strings.HasPrefix(got, "…") || !strings.HasSuffix(path, got[len("…"):]) {
			t.Errorf("width %v: head shows %q, %v wide", w, got, h.Width)
		}
	}
	// A wider box keeps more of the text.
	if a, b := shown(oneLine(path, TruncateMiddle, 150)), shown(oneLine(path, TruncateMiddle, 250)); len(a) >= len(b) {
		t.Errorf("a wider box kept no more: %q then %q", a, b)
	}
}

// A text that fits is laid out as it is, with no ellipsis.
func TestTruncateMiddleLeavesFittingTextAlone(t *testing.T) {
	plain := lay("main.go", inf)
	for _, tr := range []Truncation{TruncateMiddle, TruncateHead} {
		l := oneLine("main.go", tr, plain.Width+1)
		if l.Truncated || shown(l) != "main.go" || l.Width != plain.Width {
			t.Errorf("truncation %d changed a fitting text: %q, %v wide (want %v)", tr, shown(l), l.Width, plain.Width)
		}
	}
	if l := oneLine("", TruncateMiddle, 10); len(l.Lines) != 1 || l.Truncated {
		t.Errorf("empty text: %+v", l.Lines)
	}
}

// The cuts fall between grapheme clusters: a family emoji or a letter with a
// combining accent is kept or left out whole.
func TestTruncateMiddleCutsBetweenGraphemes(t *testing.T) {
	s := strings.Repeat("👨‍👩‍👧é", 12)
	stops := testSys.GraphemeStops(s)
	for _, tr := range []Truncation{TruncateMiddle, TruncateHead} {
		for w := float32(30); w < 200; w += 7 {
			l := oneLine(s, tr, w)
			ln := l.Lines[0]
			for _, r := range []int{ln.hideStart, ln.hideEnd} {
				if !slices.Contains(stops, l.byteAt(r)) {
					t.Fatalf("truncation %d at width %v cuts inside a grapheme at rune %d", tr, w, r)
				}
			}
		}
	}
}

// Hit testing never lands inside the hidden middle: the ellipsis's two sides
// are the end of the head and the start of the tail.
func TestTruncateMiddleHitTest(t *testing.T) {
	l := oneLine(path, TruncateMiddle, 160)
	ln := l.Lines[0]
	lo, hi := l.byteAt(ln.hideStart), l.byteAt(ln.hideEnd)
	for x := float32(-5); x < l.Width+5; x++ {
		p := l.HitTest(x, l.LineHeight/2)
		if p > lo && p < hi {
			t.Fatalf("HitTest(%v) = %d, inside the hidden %d..%d", x, p, lo, hi)
		}
	}
	// Selecting across the ellipsis covers it.
	rects := l.SelectionRects(0, len(path), nil)
	if len(rects) != 1 || rects[0].X > 0.5 || rects[0].W < l.Width-0.5 {
		t.Errorf("select all of a middle-truncated line: %+v, line %v wide", rects, l.Width)
	}
}

// Rich text keeps its spans on both sides of the cut.
func TestTruncateMiddleKeepsSpans(t *testing.T) {
	spans := []Span{{Start: 0, End: 5, Weight: Bold}, {Start: len(path) - 6, End: len(path), Italic: true}}
	l := testSys.LayoutSpans(path, Style{Size: 16, NoWrap: true, MaxLines: 1, Truncation: TruncateMiddle}, spans, 160)
	runs := l.Lines[0].Runs
	if runs[0].Span != 0 || runs[len(runs)-1].Span != 1 {
		t.Errorf("spans of the first and last runs: %d and %d, want 0 and 1", runs[0].Span, runs[len(runs)-1].Span)
	}
}

// Line breaks are shown as spaces on the one line.
func TestTruncateMiddleOneLineForManyParagraphs(t *testing.T) {
	l := oneLine("first line\nsecond line\nthird line", TruncateMiddle, 120)
	if len(l.Lines) != 1 || l.Height != l.LineHeight {
		t.Fatalf("got %d lines, %v tall", len(l.Lines), l.Height)
	}
	if got := shown(l); !strings.HasPrefix(got, "first") || !strings.HasSuffix(got, "line") {
		t.Errorf("shows %q", got)
	}
}

// MaxLines cuts a word too wide for the box between graphemes and measures at
// most its lines tall.
func TestMaxLinesClampsHeightAndCutsGraphemes(t *testing.T) {
	s := strings.Repeat("é", 80) + " and more words after it"
	stops := testSys.GraphemeStops(s)
	for w := float32(40); w < 160; w += 9 {
		l := testSys.Layout(s, Style{Size: 16, MaxLines: 2}, w)
		if len(l.Lines) > 2 || l.Height > 2*l.LineHeight+0.01 || !l.Truncated {
			t.Fatalf("width %v: %d lines, %v tall, truncated=%v", w, len(l.Lines), l.Height, l.Truncated)
		}
		for i := range l.Lines {
			if _, end := l.LineBounds(i); !slices.Contains(stops, end) {
				t.Errorf("width %v: line %d ends inside a grapheme at byte %d", w, i, end)
			}
		}
	}
}

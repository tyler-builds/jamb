package text

import (
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/scene"
	"golang.org/x/image/font/gofont/goregular"
)

func richStyle() Style { return Style{Size: 16} }

// runeOffset converts a byte offset into a rune index of s.
func runeOffset(s string, b int) int { return utf8.RuneCountInString(s[:b]) }

func TestBoldSpanShapesWithBoldFace(t *testing.T) {
	s := "one two three"
	plain := testSys.Layout(s, richStyle(), inf)
	rich := testSys.LayoutSpans(s, richStyle(), []Span{{Start: 4, End: 7, Weight: Bold}}, inf)
	bold := testSys.Registry().Font(FamilySans, Bold, false)
	seen := 0
	for _, r := range rich.Lines[0].Runs {
		if r.Span != 0 {
			continue
		}
		for _, g := range r.Glyphs {
			seen++
			if g.font != bold {
				t.Fatalf("glyph in the bold span uses a different font than the bold face")
			}
		}
	}
	if seen == 0 {
		t.Fatal("no glyphs carry span 0")
	}
	if rich.Width <= plain.Width {
		t.Errorf("bold span should widen the text: rich %v, plain %v", rich.Width, plain.Width)
	}
}

func TestItalicSpanUsesItalicFace(t *testing.T) {
	s := "plain slanted plain"
	l := testSys.LayoutSpans(s, richStyle(), []Span{{Start: 6, End: 13, Italic: true}}, inf)
	italic := testSys.Registry().Font(FamilySans, Regular, true)
	upright := testSys.Registry().Font(FamilySans, Regular, false)
	if italic == upright {
		t.Fatal("the sans family should have a distinct italic face")
	}
	var n int
	for _, r := range l.Lines[0].Runs {
		for _, g := range r.Glyphs {
			if r.Span == 0 {
				n++
				if g.font != italic {
					t.Fatalf("italic span glyph uses the upright face")
				}
			} else if g.font != upright {
				t.Fatalf("text outside the span uses the italic face")
			}
		}
	}
	if n == 0 {
		t.Fatal("no italic glyphs")
	}
}

func TestFamilyWithoutItalicUsesUpright(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("Upright Only", Regular, goregular.TTF); err != nil {
		t.Fatal(err)
	}
	if r.Font("Upright Only", Regular, true) != r.Font("Upright Only", Regular, false) {
		t.Error("a family with no italic face should use its upright face")
	}
}

func TestColorOnlySpansWrapLikePlainText(t *testing.T) {
	s := "the quick brown fox jumps over the lazy dog again and again and again"
	st := Style{Size: 16}
	plain := testSys.Layout(s, st, 150)
	rich := testSys.LayoutSpans(s, st, []Span{{Start: 4, End: 9}, {Start: 20, End: 25}}, 150)
	if len(rich.Lines) != len(plain.Lines) {
		t.Fatalf("rich wraps to %d lines, plain to %d", len(rich.Lines), len(plain.Lines))
	}
	for i := range plain.Lines {
		a1, b1 := plain.LineBounds(i)
		a2, b2 := rich.LineBounds(i)
		if a1 != a2 || b1 != b2 {
			t.Errorf("line %d: rich [%d,%d) plain [%d,%d)", i, a2, b2, a1, b1)
		}
	}
}

func TestSpanLayoutsAreCached(t *testing.T) {
	s := "cache me"
	bold := []Span{{Start: 0, End: 5, Weight: Bold}}
	a := testSys.LayoutSpans(s, richStyle(), bold, 200)
	b := testSys.LayoutSpans(s, richStyle(), []Span{{Start: 0, End: 5, Weight: Bold}}, 200)
	if a != b {
		t.Fatal("same text, style, spans and width should share one layout")
	}
	c := testSys.LayoutSpans(s, richStyle(), []Span{{Start: 0, End: 5, Weight: Regular}}, 200)
	if c == a || c.Width == a.Width {
		t.Errorf("changing a span's weight should change the layout (width %v and %v)", a.Width, c.Width)
	}
}

func TestRunSpanIndicesWithBidi(t *testing.T) {
	s := "abc مرحبا def"
	spans := []Span{{Start: 0, End: 3, Weight: Bold}, {Start: strings.Index(s, "def"), End: len(s), Italic: true}}
	l := testSys.LayoutSpans(s, richStyle(), spans, inf)
	defStart := runeOffset(s, spans[1].Start)
	var sawDef, sawArabic bool
	for _, ln := range l.Lines {
		for _, r := range ln.Runs {
			switch {
			case r.RTL:
				sawArabic = true
				if r.Span != -1 {
					t.Errorf("Arabic run has span %d, want -1", r.Span)
				}
			case r.RuneStart >= defStart:
				sawDef = true
				if r.Span != 1 {
					t.Errorf("Latin run after the Arabic has span %d, want 1", r.Span)
				}
			case r.RuneStart < defStart && r.RuneEnd <= runeOffset(s, 3):
				if r.Span != 0 {
					t.Errorf("leading Latin run has span %d, want 0", r.Span)
				}
			}
		}
	}
	if !sawArabic || !sawDef {
		t.Fatalf("expected Arabic and trailing Latin runs, arabic=%v def=%v", sawArabic, sawDef)
	}
}

func TestTruncationWithSpansKeepsEllipsisStyle(t *testing.T) {
	s := strings.Repeat("alpha beta gamma ", 12)
	st := Style{Size: 16, MaxLines: 2}
	l := testSys.LayoutSpans(s, st, []Span{{Start: 0, End: 40, Weight: Bold}, {Start: 60, End: 90, Italic: true}}, 160)
	if len(l.Lines) != 2 || !l.Truncated {
		t.Fatalf("want 2 truncated lines, got %d (truncated %v)", len(l.Lines), l.Truncated)
	}
	runs := l.Lines[1].Runs
	var ell, last *Run
	for i := range runs {
		r := &runs[i]
		if r.ellipsis {
			ell = r
		} else if last == nil || r.RuneEnd > last.RuneEnd {
			last = r
		}
	}
	if ell == nil || last == nil {
		t.Fatal("last line has no ellipsis")
	}
	if ell.Span != last.Span {
		t.Errorf("ellipsis span %d should match the run it follows (span %d)", ell.Span, last.Span)
	}
}

func TestSpanAtFindsRunUnderPoint(t *testing.T) {
	s := "hello world"
	l := testSys.LayoutSpans(s, richStyle(), []Span{{Start: 6, End: 11, Weight: Bold}}, inf)
	x, _ := l.Caret(7)
	if got := l.SpanAt(x+2, l.LineHeight/2); got != 0 {
		t.Errorf("SpanAt inside the bold word = %d, want 0", got)
	}
	if got := l.SpanAt(1, l.LineHeight/2); got != -1 {
		t.Errorf("SpanAt on plain text = %d, want -1", got)
	}
	if got := l.SpanAt(l.Width+50, l.LineHeight/2); got != -1 {
		t.Errorf("SpanAt past the line end = %d, want -1", got)
	}
	if got := l.SpanAt(x, l.LineHeight*3); got != -1 {
		t.Errorf("SpanAt below the text = %d, want -1", got)
	}
}

func TestDrawRichPaintsEachSpan(t *testing.T) {
	var sc scene.Scene
	l := testSys.LayoutSpans("ab cd", richStyle(), []Span{{Start: 3, End: 5}}, inf)
	red := scene.Color{R: 1, A: 1}
	blue := scene.Color{B: 1, A: 1}
	yellow := scene.Color{R: 1, G: 1, A: 1}
	testSys.DrawRich(&sc, l, 0, 0, 0, 1, 0, func(span int) RunPaint {
		if span == 0 {
			return RunPaint{Color: blue, Background: yellow, Underline: true}
		}
		return RunPaint{Color: red}
	})
	var bg, underline int
	for _, q := range sc.Quads {
		if q.Background == yellow {
			bg++
			if q.Bounds.W <= 0 || q.Bounds.H <= 0 {
				t.Errorf("background quad is empty: %+v", q.Bounds)
			}
		}
		if q.Background == blue {
			underline++
			if q.Bounds.H != 1 {
				t.Errorf("underline thickness %v, want 1 device pixel at 16px", q.Bounds.H)
			}
			if q.Bounds.Y <= l.Lines[0].Baseline {
				t.Errorf("underline at %v should be below the baseline %v", q.Bounds.Y, l.Lines[0].Baseline)
			}
		}
	}
	if bg != 1 || underline != 1 {
		t.Errorf("got %d background and %d underline quads, want 1 each", bg, underline)
	}
	var blueGlyphs, redGlyphs int
	for _, g := range sc.Glyphs {
		switch g.Color {
		case blue:
			blueGlyphs++
		case red:
			redGlyphs++
		}
	}
	if blueGlyphs != 2 || redGlyphs != 2 {
		t.Errorf("glyph colors: blue %d red %d, want 2 and 2", blueGlyphs, redGlyphs)
	}
}

// visualClusters returns the cluster (rune index) of every glyph of l in
// visual order, line by line.
func visualClusters(l *Layout) []int {
	var out []int
	for _, ln := range l.Lines {
		for _, r := range ln.Runs {
			for _, g := range r.Glyphs {
				out = append(out, g.Cluster)
			}
		}
	}
	return out
}

// wordOrder lists the words of l in visual order, left to right, each
// reported once per stretch of glyphs. It says which word is leftmost.
func wordOrder(l *Layout) []int {
	rs := []rune(l.Text)
	word := make([]int, len(rs))
	w := 0
	for i, r := range rs {
		word[i] = w
		if unicode.IsSpace(r) {
			w++
		}
	}
	var out []int
	for _, c := range visualClusters(l) {
		if id := word[c]; len(out) == 0 || out[len(out)-1] != id {
			out = append(out, id)
		}
	}
	return out
}

// runDirs lists the run directions of l in visual order, collapsing runs of
// the same direction (a span can split a run without changing its order).
func runDirs(l *Layout) []bool {
	var out []bool
	for _, ln := range l.Lines {
		for _, r := range ln.Runs {
			if len(out) == 0 || out[len(out)-1] != r.RTL {
				out = append(out, r.RTL)
			}
		}
	}
	return out
}

// A paint-only span must not change the visual order of right-to-left text,
// even when it starts or ends on a space or covers a whole RTL run.
func TestSpansKeepBidiVisualOrder(t *testing.T) {
	cases := []struct{ name, s, sub string }{
		{"Arabic word in an Arabic sentence", "مرحبا بالعالم الجميل", "بالعالم"},
		{"Arabic inside Latin", "hello مرحبا بالعالم الجميل world", "بالعالم"},
		{"Latin inside Arabic", "مرحبا hello بالعالم الجميل", "hello"},
		{"span starts on a space", "مرحبا بالعالم الجميل", " بالعالم"},
		{"span ends on a space", "مرحبا بالعالم الجميل", "مرحبا "},
		{"span covers a whole RTL run", "hello مرحبا بالعالم الجميل world", "مرحبا بالعالم الجميل"},
	}
	st := Style{Size: 14}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			i := strings.Index(c.s, c.sub)
			span := Span{Start: i, End: i + len(c.sub)}
			plain := testSys.Layout(c.s, st, 1000)
			if !slices.Contains(runDirs(plain), true) {
				t.Fatalf("the plain layout of %q has no right-to-left run", c.s)
			}
			rich := testSys.LayoutSpans(c.s, st, []Span{span}, 1000)
			if !slices.Equal(visualClusters(rich), visualClusters(plain)) {
				t.Errorf("visual glyph order differs: rich %v, plain %v", visualClusters(rich), visualClusters(plain))
			}
			if !slices.Equal(wordOrder(rich), wordOrder(plain)) {
				t.Errorf("word order differs: rich %v, plain %v", wordOrder(rich), wordOrder(plain))
			}
			if !slices.Equal(runDirs(rich), runDirs(plain)) {
				t.Errorf("run directions differ: rich %v, plain %v", runDirs(rich), runDirs(plain))
			}
			if rich.Width != plain.Width {
				t.Errorf("width %v, want %v", rich.Width, plain.Width)
			}
			// A bold span changes the glyphs but not the order of the words.
			bold := testSys.LayoutSpans(c.s, st, []Span{{Start: span.Start, End: span.End, Weight: Bold}}, 1000)
			if !slices.Equal(wordOrder(bold), wordOrder(plain)) {
				t.Errorf("bold span: word order %v, want %v", wordOrder(bold), wordOrder(plain))
			}
			if !slices.Equal(runDirs(bold), runDirs(plain)) {
				t.Errorf("bold span: run directions %v, want %v", runDirs(bold), runDirs(plain))
			}
		})
	}
}

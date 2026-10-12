package text

import (
	"strings"
	"testing"
)

// minOf returns the min-content width of s in the style st.
func minOf(t *testing.T, s string, st Style) float32 {
	t.Helper()
	w, wraps := testSys.MinContentWidth(s, st, nil)
	if !wraps {
		t.Fatalf("MinContentWidth(%q) reports no wrapping", s)
	}
	return w
}

func TestMinContentIsTheWidestWord(t *testing.T) {
	for _, size := range []float32{12, 16, 27} {
		st := Style{Size: size}
		for _, s := range []string{"18 open tasks", "the quick brown fox", "Ünïcödé wörds here"} {
			want := float32(0)
			for _, w := range strings.Fields(s) {
				want = max(want, testSys.Layout(w, st, inf).Width)
			}
			if got := minOf(t, s, st); absf(got-want) > 0.01 {
				t.Errorf("size %v, %q: min-content %v, want the widest word %v", size, s, got, want)
			}
		}
	}
}

// A hyphen is a break opportunity, so "note-taking" has the runs "note-" and "taking".
func TestMinContentBreaksAfterHyphen(t *testing.T) {
	st := Style{Size: 16}
	want := max(testSys.Layout("note-", st, inf).Width, testSys.Layout("taking", st, inf).Width)
	if got := minOf(t, "note-taking", st); absf(got-want) > 0.01 {
		t.Errorf("min-content of note-taking = %v, want %v", got, want)
	}
}

// A layout at exactly the min-content width keeps every word whole: no line
// holds a part of a word.
func TestLayoutAtMinContentKeepsWordsWhole(t *testing.T) {
	const s = "18 open tasks in the Projects page of a longer programme"
	for _, size := range []float32{12, 19.5, 27} {
		st := Style{Size: size}
		words := map[string]bool{}
		for _, w := range strings.Fields(s) {
			words[w] = true
		}
		w := minOf(t, s, st)
		l := testSys.Layout(s, st, w)
		for i := range l.Lines {
			a, b := l.LineBounds(i)
			line := strings.TrimSpace(l.Text[a:b])
			for _, word := range strings.Fields(line) {
				if !words[word] {
					t.Errorf("size %v: line %d holds %q, which is not a whole word", size, i, word)
				}
			}
			if l.Lines[i].Width > w+0.01 {
				t.Errorf("size %v: line %d is %v wide, over the min-content width %v", size, i, l.Lines[i].Width, w)
			}
		}
	}
}

func TestMinContentOfNoWrapIsTheFullWidth(t *testing.T) {
	st := Style{Size: 16, NoWrap: true}
	const s = "Updated 3 days ago in one line"
	got, wraps := testSys.MinContentWidth(s, st, nil)
	if wraps {
		t.Error("a NoWrap text reports that it wraps")
	}
	if want := testSys.Layout(s, Style{Size: 16}, inf).Width; absf(got-want) > 0.01 {
		t.Errorf("NoWrap min-content = %v, want the full width %v", got, want)
	}
}

// MaxLines limits the lines, not the words: the longest word is still the minimum.
func TestMinContentIgnoresMaxLines(t *testing.T) {
	const s = "a short phrase with a considerably longer word"
	plain := minOf(t, s, Style{Size: 16})
	clamped := minOf(t, s, Style{Size: 16, MaxLines: 1})
	if plain != clamped {
		t.Errorf("MaxLines changed the min-content width: %v, want %v", clamped, plain)
	}
}

// NoWrap with MaxLines wraps to those lines, so it has a min-content width below its full width.
func TestMinContentOfNoWrapWithMaxLinesWraps(t *testing.T) {
	const s = "a short phrase with a considerably longer word"
	st := Style{Size: 16, NoWrap: true, MaxLines: 2}
	got, wraps := testSys.MinContentWidth(s, st, nil)
	if !wraps {
		t.Fatal("NoWrap with MaxLines reports that it does not wrap")
	}
	if full := testSys.Layout(s, Style{Size: 16}, inf).Width; got >= full {
		t.Errorf("min-content %v is not below the full width %v", got, full)
	}
}

func TestMinContentOfEmptyTextIsZero(t *testing.T) {
	if got := minOf(t, "", Style{Size: 16}); got != 0 {
		t.Errorf("min-content of empty text = %v, want 0", got)
	}
}

// Spans shape their words in their own font, so a bold span can be the widest word.
func TestMinContentUsesSpanWeights(t *testing.T) {
	const s = "open tasks"
	st := Style{Size: 16}
	plain := minOf(t, s, st)
	spans := []Span{{Start: 0, End: 4, Weight: Bold}}
	bold, wraps := testSys.MinContentWidth(s, st, spans)
	if !wraps || bold <= plain {
		t.Errorf("bold span min-content %v (wraps %v), want more than the plain %v", bold, wraps, plain)
	}
}

func absf(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

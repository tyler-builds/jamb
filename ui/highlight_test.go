package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/scene"
)

// Match offsets index the original string, even where lowering changes the
// byte length ("İ" is 2 bytes and lowers to "i"; the Kelvin sign is 3 bytes and
// lowers to "k").
func TestMatchRangesAreValidInTheOriginal(t *testing.T) {
	cases := []struct {
		s, q string
		want []string // the matched substrings of s
	}{
		{"Kelvin notes", "kelvin", []string{"Kelvin"}},
		{"İstanbul", "istanbul", []string{"İstanbul"}},
		{"Ünïcode café, CAFÉ", "café", []string{"café", "CAFÉ"}},
		{"Straße and STRASSE, straße", "straße", []string{"Straße", "straße"}},
		{"plain lang text", "LANG", []string{"lang"}},
		{"nothing here", "", nil},
	}
	for _, c := range cases {
		var got []string
		for _, r := range matchRanges(c.s, c.q) {
			if r[0] < 0 || r[1] > len(c.s) || r[0] >= r[1] {
				t.Fatalf("%q/%q: range %v is outside the text", c.s, c.q, r)
			}
			got = append(got, c.s[r[0]:r[1]])
		}
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("matchRanges(%q, %q) = %q, want %q", c.s, c.q, got, c.want)
		}
	}
}

// HighlightMatches on a plain text styles each match, and a blank query
// leaves the text plain.
func TestHighlightMatchesStylesEachMatch(t *testing.T) {
	h := Highlight{Background: RGB(10, 20, 30)}
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Text("Lang and LANG").HighlightMatches("lang", h).Key("t")).
			Child(Text("no change").HighlightMatches("  ", h).Key("blank"))
	}, 400, 300, 1)
	// "Lang and LANG": the matches are bytes [0,4) and [9,13).
	n := richNode(t, rt, "t")
	for _, off := range []int{0, 3, 9, 12} {
		if p := paintAt(t, n, off); p.hl.Background != h.Background {
			t.Errorf("byte %d: background %v, want the match color", off, p.hl.Background)
		}
	}
	for _, off := range []int{4, 5} {
		if p := paintAt(t, n, off); p.hl.Background == h.Background {
			t.Errorf("byte %d (\"and\") is highlighted, want only the matches", off)
		}
	}
	if b := richNodeOrNil(rt, "blank"); b != nil {
		t.Error("a blank query made the text rich")
	}
}

// richNodeOrNil returns the rich text node with key, or nil.
func richNodeOrNil(rt *Runtime, key string) *node {
	for _, n := range rt.byID {
		if n.key == key && n.rich != nil {
			return n
		}
	}
	return nil
}

// Matches in a Rich text are found in its joined text.
func TestHighlightMatchesInRichText(t *testing.T) {
	h := Highlight{Background: RGB(10, 20, 30)}
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Rich(Seg("Alex").Bold(), Seg(" pushed Lang")).
			HighlightMatches("lang", h).Key("r"))
	}, 400, 300, 1)
	n := richNode(t, rt, "r")
	if p := paintAt(t, n, 13); p.hl.Background != h.Background {
		t.Errorf("the match in the second segment has background %v, want %v", p.hl.Background, h.Background)
	}
	if p := paintAt(t, n, 1); p.hl.Weight != FontBold || p.hl.Background == h.Background {
		t.Errorf("the first segment = %+v, want bold and no match color", p.hl)
	}
}

// Searching for an Arabic word highlights it and leaves the visual order of
// the line's glyphs unchanged.
func TestHighlightMatchesKeepsArabicOrder(t *testing.T) {
	const line = "Arabic (right to left): مرحبا بالعالم — النص العربي يتصل"
	const q = "بالعالم"
	h := Highlight{Background: RGB(10, 20, 30)}
	plain, plainQuads := glyphOrder(t, func() Element { return Text(line) }, h)
	hl, hlQuads := glyphOrder(t, func() Element { return Text(line).HighlightMatches(q, h) }, h)
	if plainQuads != 0 || hlQuads != 1 {
		t.Errorf("highlight quads: plain %d, search %d; want 0 and 1", plainQuads, hlQuads)
	}
	if !slices.Equal(plain, hl) {
		t.Errorf("the highlighted line draws its glyphs in a different order than the plain line")
	}
}

// glyphOrder paints el under a fresh runtime and returns the glyph sprites in
// paint order (visual order of the text) and the number of quads in the
// highlight's background color.
func glyphOrder(t *testing.T, el func() Element, h Highlight) ([]scene.Rect, int) {
	t.Helper()
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(el())
	}, 640, 120, 1)
	sc := paintNow(rt)
	var src []scene.Rect
	for _, g := range sc.Glyphs {
		src = append(src, g.Src)
	}
	quads := 0
	for _, q := range sc.Quads {
		if q.Background == scene.Color(h.Background) {
			quads++
		}
	}
	return src, quads
}

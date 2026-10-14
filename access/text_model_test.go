package access

import (
	"fmt"
	"slices"
	"testing"
	"unicode"
)

// stringModel is a second TextModel, for the tests: an ASCII text in which each byte is a point
// and each character is 8 units wide, cut into lines of width bytes within each paragraph, as
// testContent cuts them. Unlike TextContent it holds no table: it scans the text each time it is
// asked, so the two implementations share only the rules of the text, not their code.
type stringModel struct {
	s     string
	width int
}

// layout returns the starts of the lines, the paragraphs and the word units of the text.
func (m *stringModel) layout() (lines, paras, words []int) {
	s, n := m.s, len(m.s)
	paras = []int{0}
	for p := 0; p < n; p++ {
		if s[p] == '\n' {
			paras = append(paras, p+1)
		}
	}
	for i, ps := range paras {
		pe := n // the line break that ends the paragraph, or the end of the text
		if i+1 < len(paras) {
			pe = paras[i+1] - 1
		}
		for q := ps; q == ps || q < pe; q += m.width {
			lines = append(lines, q)
		}
	}
	for p := 0; p < n; p++ {
		if p == 0 {
			words = append(words, 0)
			continue
		}
		prev, cur := s[p-1], s[p]
		isBreak := func(b byte) bool { return b == '\n' }
		isSpace := func(b byte) bool { return b == ' ' || b == '\t' }
		isWord := func(b byte) bool { return unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b)) || b == '_' }
		switch {
		case isBreak(cur) || isBreak(prev):
			words = append(words, p)
		case isSpace(cur):
		case isSpace(prev) || isWord(cur) != isWord(prev):
			words = append(words, p)
		}
	}
	return lines, paras, words
}

// holder returns the last start at or before p.
func holder(starts []int, p int) int {
	h := 0
	for _, s := range starts {
		if s <= p {
			h = s
		}
	}
	return h
}

// after returns the first start after p, or n when there is none.
func after(starts []int, p, n int) int {
	for _, s := range starts {
		if s > p {
			return s
		}
	}
	return n
}

// before returns the last start before p, or 0 when there is none.
func before(starts []int, p int) int {
	b := 0
	for _, s := range starts {
		if s < p {
			b = s
		}
	}
	return b
}

func (m *stringModel) Len() int { return len(m.s) }

func (m *stringModel) PointOf(b int) int { return min(max(b, 0), len(m.s)) }

func (m *stringModel) ByteOf(p int) int { return min(max(p, 0), len(m.s)) }

func (m *stringModel) Span(u TextUnit, p int) (int, int) {
	n := len(m.s)
	lines, paras, words := m.layout()
	switch u {
	case UnitCharacter:
		return p, p + 1
	case UnitLine:
		return holder(lines, p), after(lines, p, n)
	case UnitParagraph:
		return holder(paras, p), after(paras, p, n)
	case UnitWord:
		return holder(words, p), after(words, p, n)
	}
	return 0, n
}

func (m *stringModel) Next(u TextUnit, p int) int {
	n := len(m.s)
	if p >= n {
		return n
	}
	lines, paras, words := m.layout()
	switch u {
	case UnitCharacter:
		return p + 1
	case UnitLine:
		return after(lines, p, n)
	case UnitParagraph:
		return after(paras, p, n)
	case UnitWord:
		return after(words, p, n)
	}
	return n
}

func (m *stringModel) Prev(u TextUnit, p int) int {
	if p <= 0 {
		return 0
	}
	lines, paras, words := m.layout()
	switch u {
	case UnitCharacter:
		return p - 1
	case UnitLine:
		return before(lines, p)
	case UnitParagraph:
		return before(paras, p)
	case UnitWord:
		return before(words, p)
	}
	return 0
}

func (m *stringModel) Line(p int) (TextLine, int) {
	lines, _, _ := m.layout()
	start := holder(lines, p)
	end := after(lines, p, len(m.s))
	row := slices.Index(lines, start)
	// Right is the x after the last point of the line that does not follow a space. The last
	// line also owns the end of the text.
	var right float32
	last := end
	if row == len(lines)-1 {
		last = end + 1
	}
	for q := start; q < last; q++ {
		if q == 0 || m.s[q-1] != ' ' {
			right = max(right, m.CaretX(q))
		}
	}
	return TextLine{Start: start, Top: float32(row) * 16, Height: 16, Right: right}, end
}

func (m *stringModel) CaretX(p int) float32 {
	lines, _, _ := m.layout()
	return float32(p-holder(lines, p)) * 8
}

func (m *stringModel) Nearest(x, y float32) int {
	lines, _, _ := m.layout()
	li := 0
	for li+1 < len(lines) && float32(li+1)*16 <= y {
		li++
	}
	start, end := lines[li], len(m.s)+1
	if li+1 < len(lines) {
		end = lines[li+1]
	}
	best, bestD := start, float32(-1)
	for p := start; p < end; p++ {
		d := m.CaretX(p) - x
		if d < 0 {
			d = -d
		}
		if bestD < 0 || d < bestD {
			best, bestD = p, d
		}
	}
	return best
}

func (m *stringModel) Read(from, to, limit int) string {
	return UTF16Prefix(m.s[m.ByteOf(from):m.ByteOf(to)], limit)
}

func (m *stringModel) SameText(o TextModel) bool {
	if om, ok := o.(*stringModel); ok {
		return om.s == m.s
	}
	return o != nil && o.Read(0, o.Len(), -1) == m.s
}

// The units, ranges and geometry of a text answer the same from both models.
func TestModelsAgreeOnUnitsRangesAndRects(t *testing.T) {
	texts := []string{"", "a", "foo bar", "foo   bar, baz", "aaaa bbbb\ncc", "one two\n\nthree", "ab\ncd\nef\n"}
	for _, s := range texts {
		for _, width := range []int{3, 80} {
			c := testContent(s, width)
			m := &stringModel{s: s, width: width}
			clip := Rect{W: 1000, H: 1000}
			ci := &TextInfo{Content: c, Scale: 1, Clip: clip}
			mi := &TextInfo{Content: m, Scale: 1, Clip: clip}
			n := c.Len()
			for a := 0; a <= n; a++ {
				for b := a; b <= n; b++ {
					cr := TextRange{Info: ci, Start: a, End: b}
					mr := TextRange{Info: mi, Start: a, End: b}
					for u := UnitCharacter; u <= UnitDocument; u++ {
						at := fmt.Sprintf("%q at %d..%d, unit %d", s, a, b, u)
						if got, want := span(mr.Expand(u)), span(cr.Expand(u)); got != want {
							t.Errorf("expand %s = %v, content %v", at, got, want)
						}
						for _, k := range []int{-5, -2, -1, 1, 2, 5} {
							cm, cn := cr.Move(u, k)
							mm, mn := mr.Move(u, k)
							if span(mm) != span(cm) || mn != cn {
								t.Errorf("move %s by %d = %v moved %d, content %v moved %d", at, k, span(mm), mn, span(cm), cn)
							}
						}
						for _, start := range []bool{true, false} {
							for _, k := range []int{-2, -1, 1, 3} {
								cm, cn := cr.MoveEndpoint(start, u, k)
								mm, mn := mr.MoveEndpoint(start, u, k)
								if span(mm) != span(cm) || mn != cn {
									t.Errorf("move endpoint %s start %v by %d = %v moved %d, content %v moved %d",
										at, start, k, span(mm), mn, span(cm), cn)
								}
							}
						}
					}
					if got, want := mr.Rects(), cr.Rects(); !slices.Equal(got, want) {
						t.Errorf("rects of %s at %d..%d = %v, content %v", s, a, b, got, want)
					}
					for _, limit := range []int{-1, 0, 1, 3, 100} {
						if got, want := mr.GetText(limit), cr.GetText(limit); got != want {
							t.Errorf("GetText(%d) of %s at %d..%d = %q, content %q", limit, s, a, b, got, want)
						}
					}
					for _, needle := range []string{"a", "ab", "b", "bar", " ", "\n", "zz"} {
						for _, back := range []bool{false, true} {
							for _, fold := range []bool{false, true} {
								cf, cok := cr.Find(needle, back, fold)
								mf, mok := mr.Find(needle, back, fold)
								if mok != cok || span(mf) != span(cf) {
									t.Errorf("find %q back %v fold %v in %s at %d..%d = %v %v, content %v %v",
										needle, back, fold, s, a, b, span(mf), mok, span(cf), cok)
								}
							}
						}
					}
				}
			}
			for _, x := range []float32{-5, 0, 7, 16, 40, 200} {
				for _, y := range []float32{-5, 0, 8, 16, 33, 60} {
					if got, want := ci.PointAt(x, y), mi.PointAt(x, y); got != want {
						t.Errorf("PointAt(%v, %v) in %q width %d = %d, content %d", x, y, s, width, got, want)
					}
				}
			}
		}
	}
}

// A read with a limit takes whole UTF-16 code units, and never splits a surrogate pair.
func TestGetTextLimitCountsUTF16Units(t *testing.T) {
	// "a", a code point of two UTF-16 units, then "b": the points are the grapheme starts.
	c := &TextContent{Text: "a\U0001F600b", Points: []int{0, 1, 5, 6}}
	r := TextRange{Info: &TextInfo{Content: c}, Start: 0, End: c.Len()}
	cases := []struct {
		limit int
		want  string
	}{
		{-1, "a\U0001F600b"},
		{0, ""},
		{1, "a"},
		{2, "a"},
		{3, "a\U0001F600"},
		{4, "a\U0001F600b"},
	}
	for _, tc := range cases {
		if got := r.GetText(tc.limit); got != tc.want {
			t.Errorf("GetText(%d) = %q, want %q", tc.limit, got, tc.want)
		}
	}
	m := &stringModel{s: "abcdef", width: 80}
	mr := TextRange{Info: &TextInfo{Content: m}, Start: 1, End: 4}
	if got := mr.GetText(2); got != "bc" {
		t.Errorf("model GetText(2) of 1..4 = %q, want %q", got, "bc")
	}
}

// A text of another model that holds the same text is not a text change, and a model with
// other text is one.
func TestDiffComparesTextAcrossModels(t *testing.T) {
	mk := func(content TextModel) []*Node {
		return []*Node{
			{ID: RootID, Role: RoleWindow, Children: []ID{2}},
			{ID: 2, Role: RoleTextArea, Text: &TextInfo{Content: content, Anchor: 1, Caret: 1}},
		}
	}
	prev := NewTree(0, mk(&stringModel{s: "abc", width: 80}))
	same := NewTree(0, mk(testContent("abc", 80)))
	if u := Diff(prev, same); len(u.TextChanged) != 0 {
		t.Errorf("same text across models reported TextChanged %v", u.TextChanged)
	}
	changed := NewTree(0, mk(&stringModel{s: "abcd", width: 80}))
	if u := Diff(prev, changed); len(u.TextChanged) != 1 {
		t.Errorf("changed text reported TextChanged %v, want [2]", u.TextChanged)
	}
}

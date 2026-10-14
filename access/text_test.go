package access

import (
	"strings"
	"testing"
	"unicode"
)

// testContent builds the content of an ASCII text, with each byte one point and
// each character 8 units wide. The text is laid out in visual lines of width
// bytes, cut within each paragraph, and a line break ends the last line of its
// paragraph. Runs and paragraphs are marked the way the ui package marks them.
func testContent(s string, width int) *TextContent {
	c := &TextContent{Text: s}
	n := len(s)
	for i := 0; i <= n; i++ {
		c.Points = append(c.Points, i)
	}
	c.Paras = []int{0}
	for p := 0; p < n; p++ {
		if s[p] == '\n' {
			c.Paras = append(c.Paras, p+1)
		}
	}
	for ps := 0; ; {
		pe := ps
		for pe < n && s[pe] != '\n' {
			pe++
		}
		for q := ps; q == ps || q < pe; q += width {
			c.Lines = append(c.Lines, TextLine{Start: q, Height: 16})
		}
		if pe >= n {
			break
		}
		ps = pe + 1
	}
	for i := range c.Lines {
		c.Lines[i].Top = float32(i) * 16
	}
	c.X = make([]float32, n+1)
	for p := 0; p <= n; p++ {
		li := 0
		for i := range c.Lines {
			if c.Lines[i].Start <= p {
				li = i
			}
		}
		c.X[p] = float32(p-c.Lines[li].Start) * 8
		// Right leaves out the trailing spaces of the line, as TextLine describes: a point
		// after a space is not the end of the text.
		if p == 0 || s[p-1] != ' ' {
			c.Lines[li].Right = max(c.Lines[li].Right, c.X[p])
		}
	}
	c.Words = asciiWordStarts(s)
	return c
}

// asciiWordStarts is the Windows word-unit rule of text.WordStarts for ASCII text:
// a unit starts at a visible character after a space or a line break, at a line
// break, and where a word character meets a punctuation character. Spaces and tabs
// belong to the unit before them.
func asciiWordStarts(s string) []int {
	isWord := func(b byte) bool {
		return unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b)) || b == '_'
	}
	isBreak := func(b byte) bool { return b == '\n' || b == '\r' }
	isSpace := func(b byte) bool { return b == ' ' || b == '\t' }
	var starts []int
	for p := 0; p < len(s); p++ {
		if p == 0 {
			starts = append(starts, 0)
			continue
		}
		prev, cur := s[p-1], s[p]
		switch {
		case isBreak(cur) || isBreak(prev):
			starts = append(starts, p)
		case isSpace(cur):
		case isSpace(prev) || isWord(cur) != isWord(prev):
			starts = append(starts, p)
		}
	}
	return starts
}

// testWhole returns the whole range of a content, in a TextInfo of its own.
func testWhole(c *TextContent) TextRange {
	return (&TextInfo{Content: c}).Whole()
}

// span returns the start and end of r, as a pair, for comparison.
func span(r TextRange) [2]int { return [2]int{r.Start, r.End} }

func TestPointOfRoundsDownWithinAGrapheme(t *testing.T) {
	// "e" followed by a combining acute accent (two bytes), then "x".
	c := &TextContent{Text: "éx", Points: []int{0, 3, 4}}
	for _, tc := range []struct{ b, p int }{{0, 0}, {1, 0}, {2, 0}, {3, 1}, {4, 2}} {
		if got := c.PointOf(tc.b); got != tc.p {
			t.Errorf("PointOf(%d) = %d, want %d", tc.b, got, tc.p)
		}
	}
	if got := c.ByteOf(9); got != 4 {
		t.Errorf("ByteOf past the end = %d, want the end 4", got)
	}
}

func TestExpandCharacterAndWord(t *testing.T) {
	// "foo bar": the units are "foo " (0 to 4) and "bar" (4 to 7).
	c := testContent("foo bar", 80)
	whole := testWhole(c)
	cases := []struct {
		name       string
		start, end int
		unit       TextUnit
		want       [2]int
	}{
		{"degenerate in a word, character", 1, 1, UnitCharacter, [2]int{1, 2}},
		{"degenerate in a word, word", 1, 1, UnitWord, [2]int{0, 4}},
		{"degenerate at the start of a unit", 4, 4, UnitWord, [2]int{4, 7}},
		{"degenerate in the spaces after a word takes that unit", 3, 3, UnitWord, [2]int{0, 4}},
		{"degenerate at the end of the text", 7, 7, UnitWord, [2]int{4, 7}},
		{"range within a unit", 1, 2, UnitWord, [2]int{0, 4}},
		{"range over two units", 1, 5, UnitWord, [2]int{0, 7}},
		{"range ending at a unit boundary keeps it", 0, 4, UnitWord, [2]int{0, 4}},
		{"document is the whole text", 2, 3, UnitDocument, [2]int{0, 7}},
		{"page is the whole text", 2, 3, UnitPage, [2]int{0, 7}},
		{"format is the whole text", 2, 3, UnitFormat, [2]int{0, 7}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := TextRange{Info: whole.Info, Start: tc.start, End: tc.end}
			if got := span(r.Expand(tc.unit)); got != tc.want {
				t.Errorf("Expand(%d..%d, unit %d) = %v, want %v", tc.start, tc.end, tc.unit, got, tc.want)
			}
		})
	}
}

func TestExpandLineAndParagraph(t *testing.T) {
	// "aaaa bbbb\ncc" laid out 5 bytes to a line: "aaaa " and "bbbb\n", then the
	// paragraph's "cc". Points 0 to 12; the break is point 9.
	c := testContent("aaaa bbbb\ncc", 5)
	whole := testWhole(c)
	at := func(p int, u TextUnit) [2]int {
		return span(TextRange{Info: whole.Info, Start: p, End: p}.Expand(u))
	}
	cases := []struct {
		p    int
		u    TextUnit
		want [2]int
	}{
		{2, UnitLine, [2]int{0, 5}},
		{5, UnitLine, [2]int{5, 10}},   // a wrap at 5 belongs to the next line
		{9, UnitLine, [2]int{5, 10}},   // the break is the last point of its line
		{10, UnitLine, [2]int{10, 12}}, // the first point of the next paragraph
		{12, UnitParagraph, [2]int{10, 12}},
		{3, UnitParagraph, [2]int{0, 10}},
	}
	for _, tc := range cases {
		if got := at(tc.p, tc.u); got != tc.want {
			t.Errorf("unit %d at %d = %v, want %v", tc.u, tc.p, got, tc.want)
		}
	}
}

func TestEmptyTextHasNoUnits(t *testing.T) {
	c := testContent("", 10)
	if got := span(testWhole(c).Expand(UnitLine)); got != [2]int{0, 0} {
		t.Errorf("empty text expands to %v", got)
	}
	if _, moved := testWhole(c).Move(UnitCharacter, 1); moved != 0 {
		t.Errorf("empty text moved %d", moved)
	}
}

// TestMoveWordFollowsWindows checks the degenerate word movement against the
// Windows convention, worked out by hand from the units of "foo   bar, baz"
// (see text.WordStarts): Ctrl+Right stops at the start of the next unit, and
// Ctrl+Left at the start of the unit before the caret.
func TestMoveWordFollowsWindows(t *testing.T) {
	// "foo   bar, baz": the units are "foo   " (0 to 6), "bar" (6 to 9), ", " (9 to 11)
	// and "baz" (11 to 14).
	c := testContent("foo   bar, baz", 80)
	whole := testWhole(c)
	p := TextRange{Info: whole.Info, Start: 0, End: 0}
	for _, want := range []int{6, 9, 11, 14} { // starts of bar, ", ", baz, and the end
		var moved int
		p, moved = p.Move(UnitWord, 1)
		if moved != 1 || p.Start != want || !p.Degenerate() {
			t.Fatalf("forward: moved %d to %d, want 1 to %d", moved, p.Start, want)
		}
	}
	p = TextRange{Info: whole.Info, Start: 14, End: 14}
	for _, want := range []int{11, 9, 6, 0} { // starts of baz, ", ", bar and foo
		var moved int
		p, moved = p.Move(UnitWord, -1)
		if moved != -1 || p.Start != want || !p.Degenerate() {
			t.Fatalf("backward: moved %d to %d, want -1 to %d", moved, p.Start, want)
		}
	}
}

func TestMoveDegenerateAndCounts(t *testing.T) {
	// "ab\ncd\nef": lines start at 0, 3 and 6; the end is point 8.
	c := testContent("ab\ncd\nef", 80)
	r, moved := (TextRange{Info: testWhole(c).Info}).Move(UnitCharacter, 3)
	if moved != 3 || !r.Degenerate() || r.Start != 3 {
		t.Errorf("move 3 characters = %v moved %d, want 3..3 moved 3", span(r), moved)
	}
	r, moved = r.Move(UnitLine, 5)
	if moved != 2 || span(r) != [2]int{8, 8} {
		t.Errorf("move 5 lines forward = %v moved %d, want 8..8 moved 2", span(r), moved)
	}
	if r2, moved := r.Move(UnitCharacter, 1); moved != 0 || r2 != r {
		t.Errorf("move past the end moved %d and changed the range", moved)
	}
	r, moved = r.Move(UnitLine, -4)
	if moved != -3 || span(r) != [2]int{0, 0} {
		t.Errorf("move 4 lines back = %v moved %d, want 0..0 moved -3", span(r), moved)
	}
	if _, moved := r.Move(UnitCharacter, 0); moved != 0 {
		t.Errorf("zero count moved %d", moved)
	}
}

func TestMoveNonDegenerateIsWholeWord(t *testing.T) {
	// "one two three": two is 4-7, three is 8-13. A range inside "two" moves to
	// the start of "two", then one word on, so it spans "three" as a whole.
	c := testContent("one two three", 80)
	r := TextRange{Info: testWhole(c).Info, Start: 5, End: 6}
	got, moved := r.Move(UnitWord, 1)
	if moved != 1 || span(got) != [2]int{8, 13} {
		t.Errorf("move from inside a word = %v moved %d, want 8..13 moved 1", span(got), moved)
	}
	// Moving back from there lands on "two", and the range is "two" again.
	back, moved := got.Move(UnitWord, -1)
	if moved != -1 || span(back) != [2]int{4, 8} {
		t.Errorf("move back = %v moved %d, want 4..8 moved -1", span(back), moved)
	}
}

func TestMoveEndpointOrdersTheRange(t *testing.T) {
	c := testContent("abcdef", 80)
	r := TextRange{Info: testWhole(c).Info, Start: 2, End: 4}
	got, moved := r.MoveEndpoint(true, UnitCharacter, 3) // the start passes the end
	if moved != 3 || span(got) != [2]int{5, 5} {
		t.Errorf("start past the end = %v moved %d, want 5..5 moved 3", span(got), moved)
	}
	got, moved = r.MoveEndpoint(false, UnitCharacter, -3) // the end passes the start
	if moved != -3 || span(got) != [2]int{1, 1} {
		t.Errorf("end before the start = %v moved %d, want 1..1 moved -3", span(got), moved)
	}
	got, moved = r.MoveEndpoint(false, UnitCharacter, 2) // a plain extension
	if moved != 2 || span(got) != [2]int{2, 6} {
		t.Errorf("end extended = %v moved %d, want 2..6 moved 2", span(got), moved)
	}
}

func TestMoveEndpointByRangeAndCompare(t *testing.T) {
	c := testContent("abcdef", 80)
	info := testWhole(c).Info
	a := TextRange{Info: info, Start: 1, End: 2}
	b := TextRange{Info: info, Start: 4, End: 5}
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"start of a against start of b", a.CompareEndpoints(true, b, true), -1},
		{"end of a against start of b", a.CompareEndpoints(false, b, true), -1},
		{"start of b against end of b", b.CompareEndpoints(true, b, false), -1},
		{"end of b against start of a", b.CompareEndpoints(false, a, true), 1},
		{"same endpoint", a.CompareEndpoints(true, a, true), 0},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
	if got := a.MoveEndpointByRange(false, b, false); span(got) != [2]int{1, 5} {
		t.Errorf("end set to b's end = %v, want 1..5", span(got))
	}
	if got := a.MoveEndpointByRange(true, b, true); span(got) != [2]int{4, 4} {
		t.Errorf("start set past the end = %v, want 4..4", span(got))
	}
}

func TestFindTextForwardBackwardAndCase(t *testing.T) {
	// "Ab ab AB": the matches of "ab" are at 0, 3 and 6.
	c := testContent("Ab ab AB", 80)
	whole := testWhole(c)
	cases := []struct {
		name       string
		from, to   int
		needle     string
		backward   bool
		ignoreCase bool
		want       [2]int
		found      bool
	}{
		{"first match, ignoring case", 0, 8, "ab", false, true, [2]int{0, 2}, true},
		{"last match, ignoring case", 0, 8, "ab", true, true, [2]int{6, 8}, true},
		{"first match, case-sensitive", 0, 8, "ab", false, false, [2]int{3, 5}, true},
		{"last match, case-sensitive", 0, 8, "ab", true, false, [2]int{3, 5}, true},
		{"match inside a range", 1, 5, "ab", false, true, [2]int{3, 5}, true},
		{"match running past the range", 1, 4, "ab", false, true, [2]int{}, false},
		{"missing text", 0, 8, "xyz", false, true, [2]int{}, false},
		{"empty search", 0, 8, "", false, true, [2]int{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := TextRange{Info: whole.Info, Start: tc.from, End: tc.to}
			got, ok := r.Find(tc.needle, tc.backward, tc.ignoreCase)
			if ok != tc.found || (ok && span(got) != tc.want) {
				t.Errorf("Find(%q) = %v, %v; want %v, %v", tc.needle, span(got), ok, tc.want, tc.found)
			}
		})
	}
}

func TestRectsOnePerLineAndClipped(t *testing.T) {
	// "aaaa" and "bbbb" on two lines, the first ending in a break at point 4.
	c := testContent("aaaa\nbbbb", 80)
	info := &TextInfo{Content: c, OriginX: 100, OriginY: 50, Scale: 2, Clip: Rect{X: 0, Y: 0, W: 1000, H: 1000}}
	rs := TextRange{Info: info, Start: 2, End: 7}.Rects()
	want := []Rect{
		{X: 132, Y: 50, W: 32, H: 32}, // from x 16 to the line's right edge, 32 units
		{X: 100, Y: 82, W: 32, H: 32}, // from the start of the second line to x 16
	}
	if len(rs) != len(want) {
		t.Fatalf("got %d rects, want one per line: %v", len(rs), rs)
	}
	for i := range want {
		if rs[i] != want[i] {
			t.Errorf("rect %d = %v, want %v", i, rs[i], want[i])
		}
	}
	if got := (TextRange{Info: info, Start: 3, End: 3}).Rects(); got != nil {
		t.Errorf("degenerate range has rects: %v", got)
	}
	info.Clip = Rect{X: 0, Y: 0, W: 140, H: 60}
	rs = TextRange{Info: info, Start: 2, End: 7}.Rects()
	if len(rs) != 1 || rs[0] != (Rect{X: 132, Y: 50, W: 8, H: 10}) {
		t.Errorf("clipped rects = %v, want one 8 by 10 rect at the field's edge", rs)
	}
}

func TestPointAtNearestOnLine(t *testing.T) {
	// "abcd" and "ef" on two lines; the second line starts at point 5.
	c := testContent("abcd\nef", 80)
	info := &TextInfo{Content: c, Scale: 1}
	if got := info.PointAt(17, 5); got != 2 {
		t.Errorf("point at x 17 on the first line = %d, want 2", got)
	}
	if got := info.PointAt(3, 25); got != 5 {
		t.Errorf("point at the start of the second line = %d, want 5", got)
	}
	if got := info.PointAt(500, 500); got != c.Len() {
		t.Errorf("point below and right of the text = %d, want the end %d", got, c.Len())
	}
}

func TestDiffReportsTextChanges(t *testing.T) {
	mk := func(c *TextContent, caret int) []*Node {
		return []*Node{
			{ID: RootID, Role: RoleWindow, Children: []ID{2}},
			{ID: 2, Role: RoleTextField, Text: &TextInfo{Content: c, Anchor: caret, Caret: caret}},
		}
	}
	prev := NewTree(0, mk(testContent("abc", 80), 1))
	next := NewTree(0, mk(testContent("abcd", 80), 4))
	u := Diff(prev, next)
	if len(u.TextChanged) != 1 || u.TextChanged[0] != 2 {
		t.Errorf("TextChanged = %v, want [2]", u.TextChanged)
	}
	if len(u.TextSelection) != 1 || u.TextSelection[0] != 2 {
		t.Errorf("TextSelection = %v, want [2]", u.TextSelection)
	}
	// The same text in a content object of its own, with the same selection,
	// changes nothing that a client hears about.
	same := NewTree(0, mk(testContent("abc", 80), 1))
	u = Diff(prev, same)
	if len(u.TextChanged) != 0 || len(u.TextSelection) != 0 {
		t.Errorf("unchanged text reported TextChanged %v, TextSelection %v", u.TextChanged, u.TextSelection)
	}
}

func TestUTF16PrefixNeverSplitsASurrogatePair(t *testing.T) {
	s := "a\U0001F600b" // a, a code point of two UTF-16 units, then b
	cases := map[int]string{
		-1: s,
		0:  "",
		1:  "a",
		2:  "a",
		3:  "a\U0001F600",
		4:  "a\U0001F600b",
	}
	for max, want := range cases {
		if got := UTF16Prefix(s, max); got != want {
			t.Errorf("UTF16Prefix(max %d) = %q, want %q", max, got, want)
		}
	}
	if got := UTF16Prefix(strings.Repeat("x", 3), 2); got != "xx" {
		t.Errorf("ASCII prefix = %q", got)
	}
}

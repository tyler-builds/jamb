package codeedit

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// A line is cut after the last space or punctuation that fits, and a word longer than the width
// is cut between two characters.
func TestWrapLineBreaksAfterSpacesAndHardBreaksWords(t *testing.T) {
	cases := []struct {
		line string
		cols int
		want []int32
	}{
		{"short", 80, nil},
		{"hello world foo", 8, []int32{6, 12}},
		{"abcdefghij", 4, []int32{4, 8}},
		{"call(a,b)", 6, []int32{5}}, // after the opening bracket, the last punctuation that fits
		{"abcd\r", 4, nil},           // a carriage return at the end stays on the last segment
		{"abcde\r", 4, []int32{4}},
	}
	for _, c := range cases {
		got, _ := wrapLine(c.line, c.cols, 4, IndentNone)
		if !slices.Equal(got, c.want) {
			t.Errorf("wrapLine(%q, %d) = %v, want %v", c.line, c.cols, got, c.want)
		}
	}
}

// A tab and a wide character are never cut, and each segment fits the width, counting the tab
// stops from the start of the line.
func TestWrapLineNeverSplitsTabsOrWideCharacters(t *testing.T) {
	for _, line := range []string{"ab\tcd\tef", "世界世界x世", "a\t\t\tb", "x世\ty界z"} {
		for cols := 2; cols < 12; cols++ {
			breaks, _ := wrapLine(line, cols, 4, IndentNone)
			starts := append([]int32{0}, breaks...)
			for k, b := range starts {
				if !utf8.RuneStart(line[b]) {
					t.Fatalf("%q at %d cells: segment %d starts inside a character at %d", line, cols, k, b)
				}
				end := len(line)
				if k+1 < len(starts) {
					end = int(starts[k+1])
				}
				first, last := cellOfCol(line, int(b), 4), cellOfCol(line, end, 4)
				if last-first > cols && utf8.RuneCountInString(line[b:end]) > 1 {
					t.Errorf("%q at %d cells: segment %d takes %d cells", line, cols, k, last-first)
				}
			}
		}
	}
}

// The continuation indent is none, the line's own or one tab stop more, and it is dropped when it
// would leave fewer than eight cells of text.
func TestWrapIndentModes(t *testing.T) {
	line := "    foo bar baz qux quux corge grault garply"
	for _, c := range []struct {
		ind  WrappingIndent
		cols int
		want int
	}{
		{IndentNone, 20, 0},
		{IndentSame, 20, 4},
		{IndentOneLevel, 20, 8},
		{IndentOneLevel, 15, 0}, // 15 - 8 leaves 7 cells
		{IndentSame, 11, 0},     // 11 - 4 leaves 7 cells
	} {
		if _, got := wrapLine(line, c.cols, 4, c.ind); got != c.want {
			t.Errorf("indent %d at %d cells = %d, want %d", c.ind, c.cols, got, c.want)
		}
	}
	// The continuation segments are narrower by the indent.
	w := wrapSegments(line, 20, 4, IndentSame)
	for k := 1; k < w.segs(); k++ {
		b, cell := w.start(k)
		end := len(line)
		if k+1 < w.segs() {
			end, _ = w.start(k + 1)
		}
		if got := cellOfCol(line, end, 4) - cell + w.indentCells(); got > 20 {
			t.Errorf("segment %d (from %d) ends at cell %d of the view, past 20", k, b, got)
		}
	}
}

// cacheFor returns a wrap cache of d at cols cells that follows its edits.
func cacheFor(d *Document, cols int) *wrapCache {
	c := &wrapCache{}
	d.OnEdit(func(ch Change) { c.edit(d, ch) })
	c.build(d, wrapKey{cols: cols, tab: 4, indent: IndentSame})
	return c
}

// An edit wraps again only the rows it touched.
func TestWrapCacheRewrapsOnlyEditedRows(t *testing.T) {
	d := NewDocument(sourceText(600))
	c := cacheFor(d, 12)
	before := c.wrapped
	_ = d.Replace(d.LineStart(100), d.LineStart(100), "x")
	if got := c.wrapped - before; got != 1 {
		t.Errorf("typing on one row wrapped %d rows, want 1", got)
	}
	before = c.wrapped
	_ = d.Replace(d.LineStart(200), d.LineStart(200), "a\nb\n")
	if got := c.wrapped - before; got != 3 {
		t.Errorf("inserting two lines wrapped %d rows, want 3", got)
	}
}

// Random edits, undo and redo, and folds leave the cache as a fresh wrap of the text, and the index
// of the display rows as a count of the segments of the shown rows.
func TestWrapCacheMatchesAFreshBuild(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	words := []string{"a", "bb", "ccc dd", "\t", " ", "\n", "e,f", "世界", "long_identifier_here ", "\n\t\t"}
	d := NewDocument(sourceText(60))
	c := cacheFor(d, 14)
	for step := 0; step < 300; step++ {
		switch rng.Intn(6) {
		case 0:
			d.Undo()
		case 1:
			d.Redo()
		case 2:
			d.Fold(rng.Intn(d.LineCount()))
		case 3:
			d.Unfold(rng.Intn(d.LineCount()))
		default:
			// A multi-part edit, which the document reports one change at a time.
			var chs []Change
			at := 0
			for k := 0; k < 1+rng.Intn(3); k++ {
				from := d.clampBoundary(d.text.Snapshot(), at+rng.Intn(max(d.Len()-at, 1)))
				to := d.clampBoundary(d.text.Snapshot(), min(from+rng.Intn(8), d.Len()))
				if from < at || to < from {
					break
				}
				chs = append(chs, Change{From: from, To: to, Text: words[rng.Intn(len(words))]})
				at = to + 1
			}
			_ = d.Edit(chs...)
		}
		fresh := &wrapCache{}
		fresh.build(d, c.key)
		if !c.ok || len(c.rows) != len(fresh.rows) {
			t.Fatalf("step %d: the cache has %d rows (ok %v), a fresh wrap %d", step, len(c.rows), c.ok, len(fresh.rows))
		}
		for r := range fresh.rows {
			if !slices.Equal(c.rows[r], fresh.rows[r]) {
				t.Fatalf("step %d: row %d wraps as %v, a fresh wrap as %v", step, r, c.rows[r], fresh.rows[r])
			}
		}
		if !slices.Equal(c.index(), fresh.index()) {
			t.Fatalf("step %d: the prefix differs from a fresh one", step)
		}
		x := rowIndex{prefix: c.index(), hidden: d.hiddenRanges(), lines: d.LineCount()}
		want := 0
		for r := 0; r < d.LineCount(); r++ {
			if !d.hiddenRow(r) {
				if got := x.indexOf(r); got != want {
					t.Fatalf("step %d: row %d is display row %d, want %d", step, r, got, want)
				}
				if row, seg, ok := x.at(want); !ok || row != r || seg != 0 {
					t.Fatalf("step %d: display row %d is row %d segment %d (%v), want row %d", step, want, row, seg, ok, r)
				}
				want += c.rows[r].segs()
			}
		}
		if x.count() != want {
			t.Fatalf("step %d: %d display rows, want %d", step, x.count(), want)
		}
	}
}

// longLines returns n lines of words, each long enough to wrap in the harness's editor.
func longLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "line %d: the quick brown fox jumps over the lazy dog, and then it naps under a tree %d\n", i, i)
	}
	return b.String()
}

// wrapAt returns an option of the harness that wraps at a column, with no continuation indent.
func wrapAt(cols int) func(*EditorElement) *EditorElement {
	return func(e *EditorElement) *EditorElement {
		return e.SoftWrap(WrapColumn).WrapColumn(cols).WrappingIndent(IndentNone)
	}
}

// A view that wraps to its width does not scroll horizontally.
func TestWrapViewportDoesNotScrollHorizontally(t *testing.T) {
	h := newHarness(t, strings.Repeat("word ", 400), func(e *EditorElement) *EditorElement { return e.SoftWrap(WrapViewport) })
	vs := h.ed.view
	if !vs.wrapping() || vs.displayRows() < 5 {
		t.Fatalf("the long line takes %d display rows (wrapping %v), want it wrapped", vs.displayRows(), vs.wrapping())
	}
	r, _ := h.rt.Find("wrap")
	h.rt.Scroll(r.X+100, r.Y+100, 200, 0, true, input.Modifiers(0))
	h.frame()
	h.doc.SetCaret(h.doc.Len())
	h.key(input.KeyEnd, 0)
	h.frame()
	if vs.scrollX != 0 {
		t.Errorf("scrollX = %v, want 0 while the rows wrap to the view", vs.scrollX)
	}
}

// Up and Down move by segments inside a wrapped line, keeping the cell, and PageDown by a page of
// display rows.
func TestWrapVerticalMovesGoBySegments(t *testing.T) {
	h := newHarness(t, "aaaa bbbb cccc dddd\nxy", wrapAt(5))
	vs := h.ed.view
	h.doc.SetCaret(2)
	h.key(input.KeyDown, 0)
	if got := h.doc.Caret(); got != 7 {
		t.Fatalf("down from cell 2 of the first segment went to %d, want 7 on the second", got)
	}
	h.key(input.KeyDown, 0)
	h.key(input.KeyDown, 0)
	if got := h.doc.Caret(); got != 17 {
		t.Fatalf("two more downs went to %d, want 17 on the last segment", got)
	}
	h.key(input.KeyDown, 0)
	if got := h.doc.Caret(); got != len("aaaa bbbb cccc dddd\nxy") {
		t.Fatalf("down onto the short row went to %d, want its end", got)
	}
	h.key(input.KeyUp, 0)
	if got := h.doc.Caret(); got != 17 {
		t.Fatalf("up kept the goal cell at %d, want 17", got)
	}

	h = newHarness(t, longLines(200), wrapAt(20))
	vs = h.ed.view
	h.doc.SetCaret(0)
	h.frame()
	h.key(input.KeyPageDown, 0)
	if i, _ := vs.displayOf(h.doc.Caret(), false); i != vs.pageRows() {
		t.Errorf("page down went to display row %d, want a page of %d", i, vs.pageRows())
	}
	if h.doc.RowOf(h.doc.Caret()) >= vs.pageRows() {
		t.Errorf("page down went to row %d, which is a page of rows, not of display rows", h.doc.RowOf(h.doc.Caret()))
	}
}

// Home and End go to the segment's edges first, then to the line's.
func TestWrapHomeAndEndTakeTwoSteps(t *testing.T) {
	h := newHarness(t, "  aaaa bbbb cccc", wrapAt(7))
	vs := h.ed.view
	// The segments are "  aaaa ", "bbbb ", "cccc".
	h.doc.SetCaret(9)
	h.key(input.KeyHome, 0)
	if got := h.doc.Caret(); got != 7 {
		t.Fatalf("home went to %d, want the segment's start 7", got)
	}
	h.key(input.KeyHome, 0)
	if got := h.doc.Caret(); got != 2 {
		t.Fatalf("a second home went to %d, want the first non-blank character", got)
	}
	h.doc.SetCaret(8)
	h.key(input.KeyEnd, 0)
	if got := h.doc.Caret(); got != 12 || !vs.upstreamNow() {
		t.Fatalf("end went to %d (upstream %v), want the segment's end 12, shown upstream", got, vs.upstreamNow())
	}
	if i, _ := vs.displayOf(12, vs.upstreamNow()); i != 1 {
		t.Errorf("the caret at the segment's end is on display row %d, want 1", i)
	}
	h.key(input.KeyEnd, 0)
	if got := h.doc.Caret(); got != h.doc.Len() || vs.upstreamNow() {
		t.Fatalf("a second end went to %d (upstream %v), want the line's end", got, vs.upstreamNow())
	}
}

// A click on a continuation row places the caret in that segment, and a click past its end puts
// the caret at its end.
func TestWrapClickOnAContinuationRow(t *testing.T) {
	h := newHarness(t, "aaaa bbbb cccc", wrapAt(5))
	vs := h.ed.view
	half := vs.geo.charW / 2 // the left edge of a cell, which is the boundary before it
	x, y := h.lineXY(1, 2)
	h.click(x-half, y, 0, 1)
	if got := h.doc.Caret(); got != 7 {
		t.Fatalf("a click on cell 2 of the second segment put the caret at %d, want 7", got)
	}
	x, y = h.lineXY(1, 9)
	h.click(x-half, y, 0, 1)
	if got := h.doc.Caret(); got != 10 || !vs.upstreamNow() {
		t.Fatalf("a click past the second segment put the caret at %d (upstream %v), want 10 upstream", got, vs.upstreamNow())
	}
	if r := vs.caretRect(10); r.Y != vs.geo.lineH-vs.scrollY {
		t.Errorf("the caret is drawn at y %v, want the second display row", r.Y)
	}
}

// A match that crosses a break is on both display rows, and a match over two lines is on both.
func TestWrapMatchAcrossABreakIsOnTwoRows(t *testing.T) {
	h := newHarness(t, "aaaa bbbb cccc\ndd", wrapAt(5))
	vs := h.ed.view
	rowsWith := func(m Match) int {
		n := 0
		for i := 0; i < vs.displayRows(); i++ {
			dr, _ := vs.displayAt(i)
			if len(matchesOverlapping([]Match{m}, dr.from, dr.end(h.doc.LineEnd(dr.row)))) > 0 {
				n++
			}
		}
		return n
	}
	if got := rowsWith(Match{From: 2, To: 7}); got != 2 {
		t.Errorf("a match across a break is on %d display rows, want 2", got)
	}
	if got := rowsWith(Match{From: 12, To: 16}); got != 2 {
		t.Errorf("a match over a line feed is on %d display rows, want 2", got)
	}
}

// A fold under a wrapped header hides the rows under it, and the header keeps its segments.
func TestWrapFoldUnderAWrappedHeader(t *testing.T) {
	h := newHarness(t, "func header(alpha, beta) {\n\tx\n\ty\n}\n", wrapAt(10))
	vs := h.ed.view
	segs := vs.wrapOf(0).segs()
	if segs < 2 {
		t.Fatalf("the header takes %d segments, want it wrapped", segs)
	}
	before := vs.displayRows()
	h.doc.Fold(0)
	h.frame()
	if got := vs.displayRows(); got != before-2 {
		t.Fatalf("folding left %d display rows, want %d", got, before-2)
	}
	if dr, _ := vs.displayAt(segs); dr.row != 3 {
		t.Errorf("the display row after the header is row %d, want 3", dr.row)
	}
	if i := vs.rowIndex().indexOf(1); i != 0 {
		t.Errorf("a hidden row is display row %d, want the header's 0", i)
	}
}

// A composition on a wrapped row puts the candidate window on the segment of the caret.
func TestWrapCompositionOnAWrappedRow(t *testing.T) {
	h := newHarness(t, "aaaa bbbb cccc", wrapAt(5))
	vs := h.ed.view
	h.doc.SetCaret(7)
	h.frame()
	h.compose("xy", 2, 0, 2)
	r := vs.caretRect(h.doc.Caret())
	if r.Y != vs.geo.lineH-vs.scrollY {
		t.Errorf("the input method's caret is at y %v, want the second display row", r.Y)
	}
	if want := vs.geo.textX - vs.scrollX + 4*vs.geo.charW; r.X != want {
		t.Errorf("the input method's caret is at x %v, want %v after bb and the composition", r.X, want)
	}
}

// A width change wraps the rows again and keeps the row at the top of the view.
func TestWrapWidthChangeKeepsTheTopRow(t *testing.T) {
	h := newHarness(t, longLines(300), func(e *EditorElement) *EditorElement {
		return e.SoftWrap(WrapViewport).Width(ui.Full)
	})
	vs := h.ed.view
	vs.scroll(0, 40*vs.geo.lineH)
	h.frame()
	top, _ := vs.displayAt(int(vs.scrollY / vs.geo.lineH))
	cols := vs.wc.key.cols
	h.rt.SetViewport(500, 600, 1)
	h.frame()
	if vs.wc.key.cols == cols {
		t.Fatalf("the width change kept %d columns", cols)
	}
	now, _ := vs.displayAt(int(vs.scrollY / vs.geo.lineH))
	if now.row != top.row || now.from > top.from || now.to < top.from {
		t.Errorf("after the width change the top row is row %d [%d, %d), want the segment of %d on row %d",
			now.row, now.from, now.to, top.from, top.row)
	}
}

// The access model's lines are the segments, and its paragraphs the rows.
func TestWrapAccessLinesAreSegments(t *testing.T) {
	h := newHarness(t, "aaaa bbbb cccc\nnext", wrapAt(5))
	activateAccess(h)
	c := accessField(t, h).Text.Content
	ln, end := c.Line(6)
	if ln.Start != 5 || end != 10 {
		t.Errorf("the line of point 6 is [%d, %d), want the segment [5, 10)", ln.Start, end)
	}
	if first, _ := c.Line(0); ln.Top-first.Top != h.ed.view.geo.lineH {
		t.Errorf("the second segment is %v below the first, want a row", ln.Top-first.Top)
	}
	if a, b := c.Span(access.UnitParagraph, 6); a != 0 || b != 15 {
		t.Errorf("the paragraph of point 6 is [%d, %d), want the row [0, 15)", a, b)
	}
	if got := c.Prev(access.UnitLine, 10); got != 5 {
		t.Errorf("the line before point 10 starts at %d, want 5", got)
	}
	if got := c.Next(access.UnitLine, 0); got != 5 {
		t.Errorf("the line after point 0 starts at %d, want 5", got)
	}
}

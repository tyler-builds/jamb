package codeedit

import (
	"bytes"
	"slices"
	"strings"
	"unicode/utf8"
)

// Soft wrap cuts a row that is wider than the wrap width into segments, and the view shows each
// segment on a display row of its own (see display.go). The text does not change: the moves of
// the Document, its rows and its offsets are the same with wrap on, and only the view, the
// pointer and the vertical moves of the editor go by display rows. The editor draws on a grid of
// cells, so the wrap of a row is a function of its text, the width in cells, the tab size and
// the indent of the continuation segments, and each editor keeps its own wrap of the rows: two
// editors can show one Document at different widths.

// A WrapMode says where an editor wraps long lines (see EditorElement.SoftWrap).
type WrapMode uint8

const (
	// WrapNone does not wrap: a long line scrolls horizontally. It is the default.
	WrapNone WrapMode = iota
	// WrapViewport wraps lines at the width of the editor's text.
	WrapViewport
	// WrapColumn wraps lines at the column set with WrapColumn, whatever the width of the
	// editor; a narrower editor scrolls horizontally to the column.
	WrapColumn
	// WrapBounded wraps lines at the column set with WrapColumn, or at the width of the editor's
	// text when that is narrower.
	WrapBounded
)

// A WrappingIndent says where the continuation segments of a wrapped line start (see
// EditorElement.WrappingIndent).
type WrappingIndent uint8

const (
	// IndentNone starts the continuation segments at the left edge of the text.
	IndentNone WrappingIndent = iota
	// IndentSame starts them under the first non-blank character of the line. It is the default.
	IndentSame
	// IndentOneLevel starts them one tab stop past the first non-blank character of the line.
	IndentOneLevel
)

// defaultWrapColumn is the column of WrapColumn and WrapBounded when the editor sets none.
const defaultWrapColumn = 80

// minWrapCells is the fewest cells a segment is given. A continuation indent that would leave
// fewer is dropped, and a narrower view still wraps at this width.
const minWrapCells = 8

// SoftWrap sets where the editor wraps lines that are too long for it: not at all (WrapNone, the
// default), at the width of the editor (WrapViewport), at a column (WrapColumn), or at the
// narrower of the two (WrapBounded). A wrapped line is cut after the last space or punctuation
// that fits, or else between two characters, and each segment is drawn on a row of its own; the
// line numbers and the fold markers are on the first segment. Up, Down, PageUp and PageDown move
// by the rows shown, and Home and End go to the start and the end of the segment first, then of
// the line. The document's text and its line count do not change.
func (e *EditorElement) SoftWrap(m WrapMode) *EditorElement { e.wrap.mode = m; return e }

// WrapColumn sets the column, in cells, where WrapColumn and WrapBounded wrap lines. It is 80 by
// default, and a column below 1 is 1.
func (e *EditorElement) WrapColumn(n int) *EditorElement { e.wrap.col = max(n, 1); return e }

// WrappingIndent sets where the continuation segments of a wrapped line start: at the left edge
// (IndentNone), under the line's first non-blank character (IndentSame, the default), or one tab
// stop further (IndentOneLevel). An indent that would leave fewer than 8 cells of text is dropped.
func (e *EditorElement) WrappingIndent(i WrappingIndent) *EditorElement { e.wrap.indent = i; return e }

// wrapSettings is how an editor wraps: its mode, its column and its continuation indent.
type wrapSettings struct {
	mode   WrapMode
	col    int
	indent WrappingIndent
}

// cols returns the width in cells that the lines wrap at, for a text area textW wide with cells of
// charW. The view keeps one cell free at the end of a row for the caret.
func (w wrapSettings) cols(textW, charW float32) int {
	view := max(int(textW/charW)-1, minWrapCells)
	col := w.col
	if col <= 0 {
		col = defaultWrapColumn
	}
	switch w.mode {
	case WrapColumn:
		return col
	case WrapBounded:
		return min(view, col)
	}
	return view
}

// fitsView reports whether the wrap keeps every row inside the view, so that the view never
// scrolls horizontally.
func (w wrapSettings) fitsView() bool { return w.mode == WrapViewport || w.mode == WrapBounded }

// rowWrap is the wrap of one row: nil for a row of one segment, or else the indent of its
// continuation segments in cells, followed by the byte of the row and the cell where each segment
// after the first starts, in pairs.
type rowWrap []int32

// segs returns how many segments the row has.
func (w rowWrap) segs() int {
	if len(w) == 0 {
		return 1
	}
	return (len(w)-1)/2 + 1
}

// start returns the byte of the row and the cell where segment k starts.
func (w rowWrap) start(k int) (b, cell int) {
	if k == 0 {
		return 0, 0
	}
	return int(w[2*k-1]), int(w[2*k])
}

// indentCells returns the indent of the continuation segments, in cells.
func (w rowWrap) indentCells() int {
	if len(w) == 0 {
		return 0
	}
	return int(w[0])
}

// segmentOf returns the segment that holds the byte col of the row. A byte where a segment starts
// is in that segment, unless upstream puts it at the end of the one before.
func (w rowWrap) segmentOf(col int, upstream bool) int {
	k := 0
	for k+1 < w.segs() {
		b, _ := w.start(k + 1)
		if b > col || (b == col && upstream) {
			break
		}
		k++
	}
	return k
}

// wrapLine returns where line is cut to fit cols cells: the byte offsets where the segments after
// the first start, nil when the line fits, and the indent of those segments in cells. A cut goes
// after the last space or punctuation that fits, or else before the character that does not fit,
// so a tab or a wide character is never cut. A carriage return at the end of the line stays on
// its last segment.
func wrapLine(line string, cols, tab int, ind WrappingIndent) (breaks []int32, indentCells int) {
	w := wrapSegments(line, cols, tab, ind)
	for k := 1; k < w.segs(); k++ {
		b, _ := w.start(k)
		breaks = append(breaks, int32(b))
	}
	return breaks, wrapIndent(line, cols, tab, ind)
}

// wrapIndent returns the indent of the continuation segments of line, in cells: none, the
// line's own, or that and one tab stop. An indent that leaves fewer than minWrapCells cells is 0.
func wrapIndent(line string, cols, tab int, ind WrappingIndent) int {
	if ind == IndentNone {
		return 0
	}
	n := cellOfCol(line, len(line)-len(strings.TrimLeft(line, " \t")), tab)
	if ind == IndentOneLevel {
		n += tab
	}
	if cols-n < minWrapCells {
		return 0
	}
	return n
}

// wrapSegments returns the wrap of line at cols cells, as wrapLine describes it.
func wrapSegments(line string, cols, tab int, ind WrappingIndent) rowWrap {
	if len(line) <= cols && !containsTab(line) {
		return nil // one byte takes at most one cell, but for a tab
	}
	indent := wrapIndent(line, cols, tab, ind)
	var w rowWrap
	start, startCell, width := 0, 0, cols // the segment being filled, and the cells it may take
	brk, brkCell := -1, 0                 // the last place after a space or punctuation in it
	cell := 0
	for i := 0; i < len(line); {
		r, n := utf8.DecodeRuneInString(line[i:])
		next := advance(cell, r, tab)
		if next-startCell > width && i > start && !(r == '\r' && i+n == len(line)) {
			b, bc := i, cell
			if brk > start {
				b, bc = brk, brkCell
			}
			if w == nil {
				w = rowWrap{int32(indent)}
			}
			w = append(w, int32(b), int32(bc))
			start, startCell, width = b, bc, cols-indent
			brk = -1
			i, cell = b, bc // the characters after the cut go on the next segment
			continue
		}
		cell = next
		i += n
		if breaksAfter(r) {
			brk, brkCell = i, cell
		}
	}
	return w
}

// fitsCells reports whether the bytes of a line take no more than cols cells. A character other
// than a tab takes no more cells than it has bytes, and a tab no more than tab cells, which
// settles most lines at once; the others are measured for the bytes of ASCII and tabs. It reports
// false for any other byte there, so that wrapSegments measures the line.
func fitsCells(b []byte, cols, tab int) bool {
	if len(b) <= cols && bytes.IndexByte(b, '\t') < 0 {
		return true
	}
	if len(b)+(tab-1)*bytes.Count(b, tabByte) <= cols {
		return true
	}
	cell := 0
	for _, c := range b {
		switch {
		case c == '\t':
			cell += tab - cell%tab
		case c < utf8.RuneSelf:
			cell++
		default:
			return false
		}
		if cell > cols {
			return false
		}
	}
	return true
}

// tabByte is a tab, for bytes.Count.
var tabByte = []byte{'\t'}

// breaksAfter reports whether a segment can end after r: after white space or punctuation, which
// is anything but a letter, a digit or the underscore.
func breaksAfter(r rune) bool {
	if r < utf8.RuneSelf {
		return !(r == '_' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9')
	}
	return charClass(r) != 1
}

// wrapKey is what a wrap of the rows was made for.
type wrapKey struct {
	cols, tab int
	indent    WrappingIndent
}

// wrapCache is the wrap of every row of the document an editor shows, at one width. It follows
// the text through the document's edits, wrapping again only the rows an edit touched, and keeps
// the display rows before each row (the prefix) for the index of the view, which it brings up to
// date when it is read.
type wrapCache struct {
	key       wrapKey
	ok        bool
	rows      []rowWrap
	prefix    []int32 // prefix[r] is the display rows of the rows before r, up to prefix[dirtyFrom]
	dirtyFrom int
	lineCount int
	wrapped   int // how many rows were wrapped, which the tests read
}

// build wraps every row of d for key.
func (c *wrapCache) build(d *Document, key wrapKey) {
	n := d.LineCount()
	c.key, c.ok, c.lineCount, c.dirtyFrom = key, true, n, 0
	c.rows = slices.Grow(c.rows[:0], n) // the rows of the last width give their room
	// pending is the part of a line that an earlier chunk held.
	var pending []byte
	add := func(b []byte) {
		c.wrapped++
		if fitsCells(b, key.cols, key.tab) {
			c.rows = append(c.rows, nil)
			return
		}
		c.rows = append(c.rows, wrapSegments(string(b), key.cols, key.tab, key.indent))
	}
	for chunk := range d.text.Snapshot().Chunks(0) {
		for {
			i := bytes.IndexByte(chunk, '\n')
			if i < 0 {
				pending = append(pending, chunk...)
				break
			}
			if len(pending) > 0 {
				pending = append(pending, chunk[:i]...)
				add(pending)
				pending = pending[:0]
			} else {
				add(chunk[:i])
			}
			chunk = chunk[i+1:]
		}
	}
	add(pending) // the last line, which no line feed ends
}

// edit follows the change c, which the document has just made: the rows it replaced give way to
// the rows of its text, which are wrapped again. The document reports a multi-part edit one change
// at a time, so the change in the line count since the last call is the change's own.
func (c *wrapCache) edit(d *Document, ch Change) {
	if !c.ok {
		return
	}
	r0 := d.RowOf(ch.From)
	added := strings.Count(ch.Text, "\n")
	n := d.LineCount()
	removed := added - (n - c.lineCount)
	if removed < 0 || r0+removed >= len(c.rows) {
		c.ok = false // out of step with the text: the next paint wraps every row again
		return
	}
	fresh := make([]rowWrap, added+1)
	for i := range fresh {
		c.wrapped++
		fresh[i] = wrapSegments(d.Line(r0+i), c.key.cols, c.key.tab, c.key.indent)
	}
	c.rows = slices.Replace(c.rows, r0, r0+removed+1, fresh...)
	c.lineCount = n
	c.dirtyFrom = min(c.dirtyFrom, r0)
}

// index returns the prefix of the display rows, brought up to date from the first row an edit
// changed.
func (c *wrapCache) index() []int32 {
	if len(c.prefix) != c.lineCount+1 {
		if cap(c.prefix) < c.lineCount+1 {
			p := make([]int32, c.lineCount+1)
			copy(p, c.prefix[:min(c.dirtyFrom+1, len(c.prefix))])
			c.prefix = p
		} else {
			c.prefix = c.prefix[:c.lineCount+1]
		}
	}
	c.prefix[0] = 0
	for r := c.dirtyFrom; r < c.lineCount; r++ {
		c.prefix[r+1] = c.prefix[r] + int32(c.rows[r].segs())
	}
	c.dirtyFrom = c.lineCount
	return c.prefix
}

// moveWrapped runs the move m by display rows, for the moves that go by the rows shown, and
// reports whether m is one of them. Up, Down, PageUp and PageDown keep the cell of the view, in
// the segment, from one move to the next. Home goes to the start of the segment, then to the
// first non-blank character of the line, as on an unwrapped row, and End goes to the end of the
// segment, then to the end of the line. The primary caret at the end of a segment shows there
// (see setUpstream); the other carets go to the end of the line at once.
func (vs *viewState) moveWrapped(m Move, extend bool) bool {
	doc := vs.doc
	x := vs.rowIndex()
	primary, up := doc.Caret(), vs.upstreamNow()
	primUp := false
	switch m {
	case MoveUp, MoveDown, MovePageUp, MovePageDown:
		steps := 1
		if m == MovePageUp || m == MovePageDown {
			steps = vs.pageRows()
		}
		if m == MoveUp || m == MovePageUp {
			steps = -steps
		}
		n := x.count()
		doc.moveEach(extend, true, func(head, goal int) (int, int) {
			isPrim := head == primary
			i, cell := vs.displayOfIn(x, head, isPrim && up)
			if goal < 0 {
				goal = cell
			}
			t := min(max(i+steps, 0), n-1)
			if t == i {
				// No row to move to: the caret goes to the start or the end of the text.
				if steps < 0 {
					return 0, goal
				}
				return doc.Len(), goal
			}
			dr, _ := vs.rowAt(x, t)
			p, pu := vs.offsetOn(dr, goal)
			if isPrim {
				primUp = pu
			}
			return p, goal
		})
	case MoveLineStart:
		t := doc.text.Snapshot()
		doc.moveEach(extend, false, func(head, _ int) (int, int) {
			i, _ := vs.displayOfIn(x, head, head == primary && up)
			dr, _ := vs.rowAt(x, i)
			if dr.seg == 0 || head == dr.from {
				// The first segment toggles as an unwrapped row does, and a caret at the start of a
				// later segment goes on to the line's first non-blank character.
				return doc.moveHead(t, head, MoveLineStart, 0, -1), -1
			}
			return dr.from, -1
		})
	case MoveLineEnd:
		doc.moveEach(extend, false, func(head, _ int) (int, int) {
			isPrim := head == primary
			i, _ := vs.displayOfIn(x, head, isPrim && up)
			dr, _ := vs.rowAt(x, i)
			le := doc.LineEnd(dr.row)
			if !isPrim || dr.last(le) || (head == dr.to && up) {
				return le, -1
			}
			primUp = true
			return dr.to, -1
		})
	default:
		return false
	}
	vs.setUpstream(primUp)
	return true
}

// addCaretVertical adds a caret one display row above (dir < 0) or below (dir > 0) the first or
// the last caret, at the same cell of the view, as Document.AddCaretVertical does by rows when
// the view does not wrap.
func (vs *viewState) addCaretVertical(dir int) {
	doc := vs.doc
	if !vs.wrapping() {
		doc.AddCaretVertical(dir)
		return
	}
	base := doc.sels[0].Head
	if dir > 0 {
		base = doc.sels[len(doc.sels)-1].Head
	}
	x := vs.rowIndex()
	i, cell := vs.displayOfIn(x, base, false)
	dr, ok := vs.rowAt(x, i+dir)
	if !ok {
		return
	}
	p, _ := vs.offsetOn(dr, cell)
	sels := append(slices.Clone(doc.sels), Selection{p, p})
	doc.setSels(sels, len(sels)-1)
}

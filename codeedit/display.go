package codeedit

import (
	"sort"
	"unicode/utf8"
)

// The view lays the text out in display rows. A display row is a row of the document, or one
// segment of a row when lines wrap: each segment of a wrapped row takes a row of the view.
// Folded rows take none. The paint, the pointer, the scrolling and the access model all go
// through the display rows, so that they agree on where a byte is drawn.

// displayRow is one row of the view: the document row it shows, which segment of that row it is,
// the bytes [from, to) of the segment, the cell of the row where the segment starts, and the
// cells of indent that the segment is drawn after. A row that does not wrap is one segment, from
// its start to its end, with no indent.
type displayRow struct{ row, seg, from, to, startCell, indent int }

// rowIndex maps the rows of a document to display rows. prefix[r] is how many display rows the
// rows before r take, folds aside, or prefix is nil when each row takes one. hidden is the runs
// of rows the folds hide, which take no display rows, and lines is the number of rows.
type rowIndex struct {
	prefix []int32
	hidden []rowRange
	lines  int
}

// before returns the display rows of the rows before r, folds aside.
func (x rowIndex) before(r int) int {
	if x.prefix == nil {
		return r
	}
	return int(x.prefix[r])
}

// count returns how many display rows there are.
func (x rowIndex) count() int {
	n := x.before(x.lines)
	for _, h := range x.hidden {
		n -= x.before(h.to+1) - x.before(h.from)
	}
	return n
}

// indexOf returns the display index of the first segment of row. A hidden row has the index of
// the header of its fold, as Document.VisibleIndex has.
func (x rowIndex) indexOf(row int) int {
	i := x.before(row)
	for _, h := range x.hidden {
		switch {
		case row > h.to:
			i -= x.before(h.to+1) - x.before(h.from)
		case row >= h.from:
			return i - (x.before(row) - x.before(h.from-1))
		}
	}
	return i
}

// at returns the row and the segment of the i-th display row, and reports whether there is one.
func (x rowIndex) at(i int) (row, seg int, ok bool) {
	if i < 0 {
		return 0, 0, false
	}
	u := i // the index among the rows with the folds open
	for _, h := range x.hidden {
		if x.before(h.from) > u {
			break
		}
		u += x.before(h.to+1) - x.before(h.from)
	}
	if u >= x.before(x.lines) {
		return 0, 0, false
	}
	if x.prefix == nil {
		return u, 0, true
	}
	row = sort.Search(x.lines, func(r int) bool { return x.before(r+1) > u })
	return row, u - x.before(row), true
}

// rowIndex returns the index of the document's display rows, as the view lays them out now.
func (vs *viewState) rowIndex() rowIndex {
	return rowIndex{hidden: vs.doc.hiddenRanges(), lines: vs.doc.LineCount()}
}

// displayRows returns how many display rows the view has.
func (vs *viewState) displayRows() int { return vs.rowIndex().count() }

// displayAt returns the i-th display row, and reports whether there is one.
func (vs *viewState) displayAt(i int) (displayRow, bool) { return vs.rowAt(vs.rowIndex(), i) }

// rowAt returns the i-th display row of the index x.
func (vs *viewState) rowAt(x rowIndex, i int) (displayRow, bool) {
	row, seg, ok := x.at(i)
	if !ok {
		return displayRow{}, false
	}
	return displayRow{row: row, seg: seg, from: vs.doc.LineStart(row), to: vs.doc.LineEnd(row)}, true
}

// displayOf returns the display row that shows the byte offset off, and the cell of the view
// where off is drawn on it. upstream puts an offset at the edge of two segments at the end of
// the first one rather than at the start of the next.
func (vs *viewState) displayOf(off int, upstream bool) (i, cell int) {
	return vs.displayOfIn(vs.rowIndex(), off, upstream)
}

// upstreamNow reports whether the primary caret shows at the end of a segment rather than at
// the start of the next one. Rows do not wrap, so it is never.
func (vs *viewState) upstreamNow() bool { return false }

// displayOfIn is displayOf on the index x.
func (vs *viewState) displayOfIn(x rowIndex, off int, upstream bool) (i, cell int) {
	doc := vs.doc
	row := doc.RowOf(off)
	line := doc.Line(row)
	return x.indexOf(row), cellOfCol(line, off-doc.LineStart(row), vs.tab)
}

// cellOf returns the cell of the view where the byte offset off of the segment is drawn. line is
// the text of the segment's row, which starts at the byte ls.
func (dr displayRow) cellOf(line string, ls, off, tab int) int {
	return cellIn(line, dr.from-ls, dr.startCell, off-ls, tab) - dr.startCell + dr.indent
}

// offsetAt returns the byte offset of the segment nearest to the cell of the view. A cell in the
// indent is the start of the segment, and a cell past its end is its end.
func (dr displayRow) offsetAt(line string, ls, cell, tab int) int {
	return ls + colIn(line, dr.from-ls, dr.to-ls, dr.startCell, cell-dr.indent+dr.startCell, tab)
}

// endCell returns the cell of the view where the segment ends.
func (dr displayRow) endCell(line string, ls, tab int) int { return dr.cellOf(line, ls, dr.to, tab) }

// cellIn returns the cell of the byte col of line, counted from the cell start of the byte from:
// the cells of a segment are counted from its start, so a long line is not measured from its
// beginning for each segment.
func cellIn(line string, from, start, col, tab int) int {
	cell := start
	for _, r := range line[from:col] {
		cell = advance(cell, r, tab)
	}
	return cell
}

// colIn returns the byte offset of line nearest to cell, among the bytes [from, to) of a segment
// that starts at the cell start, as colOfCell does for a whole line. A cell past the end of the
// segment is its end.
func colIn(line string, from, to, start, cell, tab int) int {
	c := start
	for i, r := range line[from:to] {
		next := advance(c, r, tab)
		if cell < next {
			if cell-c < next-cell {
				return from + i
			}
			return from + i + utf8.RuneLen(r)
		}
		c = next
	}
	return to
}

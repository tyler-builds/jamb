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
	x := rowIndex{hidden: vs.doc.hiddenRanges(), lines: vs.doc.LineCount()}
	if vs.wrapping() {
		x.prefix = vs.wc.index()
	}
	return x
}

// wrapping reports whether the view wraps its rows: wrap is on, and the last paint wrapped the
// rows at its width. Before the first paint the rows do not wrap.
func (vs *viewState) wrapping() bool { return vs.wrap.mode != WrapNone && vs.wc.ok }

// fitsView reports whether the view wraps its rows to its width, so that it never scrolls
// horizontally.
func (vs *viewState) fitsView() bool { return vs.wrapping() && vs.wrap.fitsView() }

// wrapOf returns the wrap of row, which is nil when the view does not wrap.
func (vs *viewState) wrapOf(row int) rowWrap {
	if !vs.wrapping() || row >= len(vs.wc.rows) {
		return nil
	}
	return vs.wc.rows[row]
}

// syncWrap wraps the rows for a text textW wide with cells of charW, when the width in cells, the
// tab size or the indent changed, or turns the wrap off. The row at the top of the view stays at
// the top, so the view does not jump when its width changes.
func (vs *viewState) syncWrap(textW, charW, lineH float32) {
	key := wrapKey{cols: vs.wrap.cols(textW, charW), tab: vs.tab, indent: vs.wrap.indent}
	if vs.wrap.mode == WrapNone {
		if !vs.wc.ok {
			return
		}
	} else if vs.wc.ok && vs.wc.key == key {
		return
	}
	top := int(vs.scrollY / lineH)
	frac := vs.scrollY - float32(top)*lineH
	anchor := 0 // the first byte at the top of the view
	if dr, ok := vs.displayAt(top); ok {
		anchor = dr.from
	}
	if vs.wrap.mode == WrapNone {
		vs.wc = wrapCache{}
	} else {
		vs.wc.build(vs.doc, key)
	}
	i, _ := vs.displayOf(anchor, false)
	vs.scrollY = float32(i)*lineH + frac
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
	return vs.segment(row, seg), true
}

// segment returns the display row of segment seg of row.
func (vs *viewState) segment(row, seg int) displayRow {
	ls, le := vs.doc.LineStart(row), vs.doc.LineEnd(row)
	w := vs.wrapOf(row)
	seg = min(seg, w.segs()-1)
	b, cell := w.start(seg)
	dr := displayRow{row: row, seg: seg, from: ls + b, to: le, startCell: cell}
	if seg+1 < w.segs() {
		next, _ := w.start(seg + 1)
		dr.to = ls + next
	}
	if seg > 0 {
		dr.indent = w.indentCells()
	}
	return dr
}

// last reports whether the segment is the last of its row, which ends at le.
func (dr displayRow) last(le int) bool { return dr.to == le }

// displayOf returns the display row that shows the byte offset off, and the cell of the view
// where off is drawn on it. upstream puts an offset at the edge of two segments at the end of
// the first one rather than at the start of the next.
func (vs *viewState) displayOf(off int, upstream bool) (i, cell int) {
	return vs.displayOfIn(vs.rowIndex(), off, upstream)
}

// setUpstream sets the affinity of the primary caret where it is now: on, the caret shows at the
// end of its segment when it is at the edge of two (see displayOf). Any later change of the
// selections or the text clears it, since the document's version changes.
func (vs *viewState) setUpstream(on bool) {
	vs.upstream, vs.upVer = on, vs.doc.version
}

// upstreamNow reports whether the primary caret shows at the end of a segment rather than at
// the start of the next one: a move to the end of a segment or a click past it put it there, and
// nothing has moved it since.
func (vs *viewState) upstreamNow() bool {
	return vs.upstream && vs.upVer == vs.doc.version && vs.wrapping()
}

// displayOfIn is displayOf on the index x.
func (vs *viewState) displayOfIn(x rowIndex, off int, upstream bool) (i, cell int) {
	doc := vs.doc
	row := doc.RowOf(off)
	ls := doc.LineStart(row)
	dr := vs.segment(row, vs.wrapOf(row).segmentOf(off-ls, upstream))
	text := string(doc.textSlice(dr.from, dr.to)) // the segment, not the whole of a long row
	return x.indexOf(row) + dr.seg, dr.cellOf(text, dr.from, off, vs.tab)
}

// offsetOn returns the byte offset nearest to the cell of the view on the display row dr, and
// whether it is the end of a segment that is not its row's last, where the caret shows upstream.
func (vs *viewState) offsetOn(dr displayRow, cell int) (int, bool) {
	text := string(vs.doc.textSlice(dr.from, dr.to))
	p := dr.offsetAt(text, dr.from, cell, vs.tab)
	return p, p == dr.to && !dr.last(vs.doc.LineEnd(dr.row))
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

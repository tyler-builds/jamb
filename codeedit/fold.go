package codeedit

import (
	"slices"
	"strings"
)

// A fold hides the lines under a header line that are indented past it. It is
// kept as byte offsets, so that it follows the text through edits: start is the
// start of the header line, and end is the end of the last hidden line, before
// its line feed.
type fold struct {
	start, end int
}

// indentOf returns the width of the leading white space of line, where a tab
// counts as the tab size.
func indentOf(line string, tab int) int {
	w := 0
	for _, r := range line {
		switch r {
		case ' ':
			w++
		case '\t':
			w += tab
		default:
			return w
		}
	}
	return w
}

// blankLine reports whether line has no text but white space.
func blankLine(line string) bool { return strings.TrimSpace(line) == "" }

// foldEnd returns the last row that the fold of header row hides, which is row
// itself when the row has no lines under it that are indented past it. Blank lines
// do not end a fold, and blank lines after the last indented line are not hidden.
func (d *Document) foldEnd(row int, tab int) int {
	h := indentOf(d.Line(row), tab)
	last := row
	for r := row + 1; r < d.LineCount(); r++ {
		line := d.Line(r)
		if blankLine(line) {
			continue
		}
		if indentOf(line, tab) <= h {
			break
		}
		last = r
	}
	return last
}

// Fold hides the lines indented under row, and reports whether it folded any.
// The row must be visible and must have lines under it that are indented past
// it. Folds that the new one covers are removed. The width of a tab stop is the document's
// (see SetTabSize), for the comparison of indentation.
func (d *Document) Fold(row int) bool {
	if row < 0 || row+1 >= d.LineCount() || d.hiddenRow(row) {
		return false
	}
	last := d.foldEnd(row, d.tab)
	if last == row {
		return false
	}
	f := fold{start: d.LineStart(row), end: d.LineEnd(last)}
	d.folds = slices.DeleteFunc(d.folds, func(g fold) bool { return g.start < f.end && g.end > f.start })
	d.folds = append(d.folds, f)
	slices.SortFunc(d.folds, func(a, b fold) int { return a.start - b.start })
	d.clampFolds()
	d.changed()
	return true
}

// Unfold shows the lines hidden by the fold at row, or by the fold that hides row,
// and reports whether it unfolded any.
func (d *Document) Unfold(row int) bool {
	n := len(d.folds)
	d.folds = slices.DeleteFunc(d.folds, func(f fold) bool {
		h := d.RowOf(f.start)
		return h == row || (row > h && row <= d.RowOf(f.end))
	})
	if len(d.folds) == n {
		return false
	}
	d.changed()
	return true
}

// opensBlock reports whether row has a line under it that is indented past it, which is what a
// fold marker shows. Unlike FoldableRow it looks at the next non-blank line only, so that the
// paint can mark each shown row cheaply.
func (d *Document) opensBlock(row, tab int) bool {
	if row+1 >= d.LineCount() {
		return false
	}
	h := indentOf(d.Line(row), tab)
	for r := row + 1; r < d.LineCount(); r++ {
		line := d.Line(r)
		if blankLine(line) {
			continue
		}
		return indentOf(line, tab) > h
	}
	return false
}

// Folded reports whether row is the header of a fold, which hides lines under it.
func (d *Document) Folded(row int) bool {
	for _, f := range d.folds {
		if d.RowOf(f.start) == row {
			return true
		}
	}
	return false
}

// FoldableRow reports whether row has lines under it that are indented past it,
// and so can be folded. The width of a tab is the document's (see SetTabSize).
func (d *Document) FoldableRow(row int) bool {
	return row+1 < d.LineCount() && !d.hiddenRow(row) && d.foldEnd(row, d.tab) > row
}

// hiddenRow reports whether row is hidden by a fold.
func (d *Document) hiddenRow(row int) bool {
	for _, f := range d.folds {
		h := d.RowOf(f.start)
		if row > h && row <= d.RowOf(f.end) {
			return true
		}
	}
	return false
}

// rowRange is a run of hidden rows, from and to both inclusive.
type rowRange struct{ from, to int }

// hiddenRanges returns the runs of hidden rows, in order.
func (d *Document) hiddenRanges() []rowRange {
	out := make([]rowRange, 0, len(d.folds))
	for _, f := range d.folds {
		h := d.RowOf(f.start)
		out = append(out, rowRange{h + 1, d.RowOf(f.end)})
	}
	return out
}

// VisibleRows returns how many rows are shown: the lines, less the hidden ones.
func (d *Document) VisibleRows() int {
	n := d.LineCount()
	for _, r := range d.hiddenRanges() {
		n -= r.to - r.from + 1
	}
	return n
}

// RowAtVisible returns the row of the i-th shown row, zero-based, and reports whether
// there is one.
func (d *Document) RowAtVisible(i int) (int, bool) {
	row := i
	for _, r := range d.hiddenRanges() {
		if r.from <= row {
			row += r.to - r.from + 1
		}
	}
	return row, row < d.LineCount()
}

// VisibleIndex returns the position of row among the shown rows. A hidden row has
// the position of the row above it.
func (d *Document) VisibleIndex(row int) int {
	i := row
	for _, r := range d.hiddenRanges() {
		switch {
		case row >= r.to:
			i -= r.to - r.from + 1
		case row >= r.from:
			i -= row - r.from + 1
		}
	}
	return i
}

// nextRow returns the shown row after row, or -1 when row is the last one. The
// hidden rows under a folded header are skipped.
func (d *Document) nextRow(row int) int {
	r := row + 1
	for _, h := range d.hiddenRanges() {
		if r >= h.from && r <= h.to {
			r = h.to + 1
		}
	}
	if r >= d.LineCount() {
		return -1
	}
	return r
}

// prevRow returns the shown row before row, or -1 when row is the first one.
func (d *Document) prevRow(row int) int {
	r := row - 1
	for _, h := range d.hiddenRanges() {
		if r >= h.from && r <= h.to {
			r = h.from - 1
		}
	}
	if r < 0 {
		return -1
	}
	return r
}

// mapFolds moves the folds through the changes.
func mapFolds(folds []fold, changes []Change) []fold {
	out := folds[:0]
	for _, f := range folds {
		f.start = mapThrough(changes, f.start, false)
		f.end = mapThrough(changes, f.end, false)
		if f.start < f.end {
			out = append(out, f)
		}
	}
	return out
}

// clampFolds moves the selections that are in hidden lines to the end of the header
// line of the fold that hides them, and normalizes the selections again.
func (d *Document) clampFolds() {
	if len(d.folds) == 0 {
		return
	}
	sels := slices.Clone(d.sels)
	for i, s := range sels {
		sels[i] = Selection{Anchor: d.unhide(s.Anchor), Head: d.unhide(s.Head)}
	}
	d.sels, d.primary = d.normalize(sels, d.primary)
}

// unhide returns p moved to the end of its fold's header line when p is in hidden lines.
func (d *Document) unhide(p int) int {
	for _, f := range d.folds {
		head := d.LineEnd(d.RowOf(f.start))
		if p > head && p <= f.end {
			return head
		}
	}
	return p
}

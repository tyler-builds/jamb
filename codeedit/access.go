package codeedit

import (
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/go-text/typesetting/segmenter"
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/rope"
	"github.com/tyler-builds/jamb/internal/text"
)

// The Text pattern of an editor. The access tree is built on the UI thread, and the
// tree is read from any thread, so the pattern reads the editor through an editorModel:
// a copy of the text (a rope snapshot), of the hidden rows, of the display rows before each
// row when the lines wrap, and of the layout of the last paint. The model never reads the
// Document, which the UI thread keeps changing.
//
// The points of the model are byte offsets, as the positions of the document are. A
// unit is measured on the line around a point, never on the whole text: a line feed
// begins a grapheme cluster, a word unit and a line, so the lines are the windows that
// the units are measured in.

var _ access.TextModel = (*editorModel)(nil)

// editorModel is the Text pattern's model of an editor (see access.TextModel).
type editorModel struct {
	doc     *Document // the document it was made from; compared by identity only
	version uint64    // the document's version, which changes with the text, the selections and the folds
	textVer uint64    // the document's text version, for SameText
	t       rope.Text
	hidden  []rowRange // the runs of hidden rows, copied from the folds
	geo     geometry   // the layout of the last paint
	scrollX float32    // the scroll position of the last paint
	scrollY float32
	tab     int
	// wrapped is whether the rows wrap, at wrap; prefix is then a copy of the display rows before
	// each row (see rowIndex), and the model wraps a row again from its text when it needs its
	// segments.
	wrapped bool
	wrap    wrapKey
	prefix  []int32
}

// textModel returns the model of the editor's text and of its last paint. It is made
// again only when the document or the layout changed, so the access tree keeps the same
// content while the text is the same (see access.TextRange.Same).
func (vs *viewState) textModel() *editorModel {
	d := vs.doc
	wrapped := vs.wrapping()
	if m := vs.model; m != nil && m.doc == d && m.version == d.version && m.geo == vs.geo &&
		m.scrollX == vs.scrollX && m.scrollY == vs.scrollY && m.tab == vs.tab &&
		m.wrapped == wrapped && m.wrap == vs.wc.key {
		return m
	}
	vs.model = &editorModel{doc: d, version: d.version, textVer: d.textVer, t: d.text.Snapshot(),
		hidden: d.hiddenRanges(), geo: vs.geo, scrollX: vs.scrollX, scrollY: vs.scrollY, tab: vs.tab,
		wrapped: wrapped, wrap: vs.wc.key}
	if wrapped {
		// The prefix is copied only here, when assistive technology reads the text.
		vs.model.prefix = slices.Clone(vs.wc.index())
	}
	return vs.model
}

// selectText sets the primary selection to the range of the byte offsets anchor and caret, as
// assistive technology asks with the Text pattern. The offsets are the model's points, which are
// bytes. A read-only editor takes a selection too, since it is only a view of the text.
func (vs *viewState) selectText(anchor, caret int) {
	vs.doc.SetSelections(Selection{Anchor: anchor, Head: caret})
	vs.reveal = true
	vs.touch()
}

// revealText scrolls the primary caret into view, as assistive technology's ScrollIntoView asks.
func (vs *viewState) revealText() {
	vs.reveal = true
	vs.touch()
}

// textAccess returns the Text pattern state of the editor for the access tree: its model
// and its primary selection. readOnly is the editor's setting, and size its font size in
// logical pixels, which is 0.75 points to the pixel at the default text size.
func (vs *viewState) textAccess(readOnly bool, size float32) *access.TextInfo {
	m := vs.textModel()
	sel := vs.doc.Primary()
	return &access.TextInfo{
		Content:    m,
		Anchor:     m.PointOf(sel.Anchor),
		Caret:      m.PointOf(sel.Head),
		FontName:   vs.fontName,
		FontSize:   float64(size) * 0.75, // logical pixels to points, at 96 per inch
		FontWeight: 400,
		ReadOnly:   readOnly,
	}
}

// Len returns the end of the text, in bytes.
func (m *editorModel) Len() int { return m.t.Len() }

// PointOf returns the point at byte offset b. A point is a byte offset, so this only
// clamps b to the text.
func (m *editorModel) PointOf(b int) int { return min(max(b, 0), m.Len()) }

// ByteOf returns the byte offset of point p, clamped to the text.
func (m *editorModel) ByteOf(p int) int { return min(max(p, 0), m.Len()) }

// SameText reports whether o is a model of the same document and text version. A model of
// another kind is compared by its text, as access.TextContent does.
func (m *editorModel) SameText(o access.TextModel) bool {
	if om, ok := o.(*editorModel); ok {
		return om.doc == m.doc && om.textVer == m.textVer
	}
	return o != nil && o.Read(0, o.Len(), -1) == m.Read(0, m.Len(), -1)
}

// rowOf returns the row that holds byte offset b.
func (m *editorModel) rowOf(b int) int { return m.t.PointAt(b).Row }

// lineEnd returns the byte offset where row ends, after its line feed: the start of the
// next row, or the end of the text on the last row.
func (m *editorModel) lineEnd(row int) int {
	if row+1 < m.t.LineCount() {
		return m.t.LineStart(row + 1)
	}
	return m.t.Len()
}

// lineText returns the text of row, without its line feed.
func (m *editorModel) lineText(row int) string {
	return strings.TrimSuffix(string(m.t.Slice(m.t.LineStart(row), m.lineEnd(row))), "\n")
}

// window returns the text from the start of the line that holds the byte before p, to the
// end of the line that holds p, with its byte offsets ws and we. Both are line starts or the
// ends of the text, which are unit boundaries, so the units inside the window are the units
// of the whole text.
func (m *editorModel) window(p int) (ws, we int, s string) {
	ws = m.t.LineStart(m.rowOf(max(p-1, 0)))
	we = m.lineEnd(m.rowOf(p))
	return ws, we, string(m.t.Slice(ws, we))
}

// graphemes returns the grapheme cluster boundaries in the window of p, as byte offsets
// of the text, from the start of the window to its end.
func (m *editorModel) graphemes(p int) (stops []int, we int) {
	ws, we, s := m.window(p)
	var seg segmenter.Segmenter
	if err := seg.InitWithString(s); err == nil {
		it := seg.GraphemeIterator()
		for it.Next() {
			stops = append(stops, ws+it.Grapheme().OffsetInBytes)
		}
	}
	if len(stops) == 0 || stops[0] != ws {
		stops = append([]int{ws}, stops...)
	}
	return append(stops, we), we
}

// words returns the word unit starts in the window of p, as byte offsets of the text (see
// text.WordStarts), and the end of the window.
func (m *editorModel) words(p int) (starts []int, we int) {
	ws, we, s := m.window(p)
	for _, b := range text.WordStarts(s) {
		starts = append(starts, ws+b)
	}
	return starts, we
}

// Span returns the points [a, b) of the unit u that holds point p, with 0 <= p < Len().
func (m *editorModel) Span(u access.TextUnit, p int) (int, int) {
	switch u {
	case access.UnitCharacter:
		stops, _ := m.graphemes(p)
		i := sort.SearchInts(stops, p+1) // the first boundary after p
		return stops[max(i-1, 0)], stops[min(i, len(stops)-1)]
	case access.UnitLine:
		ln, end := m.Line(p)
		return ln.Start, end
	case access.UnitParagraph:
		row := m.rowOf(p)
		return m.t.LineStart(row), m.lineEnd(row)
	case access.UnitWord:
		starts, we := m.words(p)
		i := sort.SearchInts(starts, p+1)
		end := we
		if i < len(starts) {
			end = starts[i]
		}
		return starts[max(i-1, 0)], end
	}
	return 0, m.Len() // format, page and document
}

// Next returns the point one unit after p, or the end of the text when there is none.
func (m *editorModel) Next(u access.TextUnit, p int) int {
	n := m.Len()
	if p >= n {
		return n
	}
	switch u {
	case access.UnitCharacter:
		stops, _ := m.graphemes(p)
		return stops[min(sort.SearchInts(stops, p+1), len(stops)-1)]
	case access.UnitLine:
		_, end := m.Line(p)
		return end
	case access.UnitParagraph:
		return m.lineEnd(m.rowOf(p))
	case access.UnitWord:
		starts, we := m.words(p)
		if i := sort.SearchInts(starts, p+1); i < len(starts) {
			return starts[i]
		}
		return we
	}
	return n
}

// Prev returns the point one unit before p, or 0 when there is none.
func (m *editorModel) Prev(u access.TextUnit, p int) int {
	if p <= 0 {
		return 0
	}
	switch u {
	case access.UnitCharacter:
		stops, _ := m.graphemes(p)
		return stops[max(sort.SearchInts(stops, p)-1, 0)]
	case access.UnitLine:
		ln, _ := m.Line(p - 1)
		return ln.Start
	case access.UnitParagraph:
		return m.t.LineStart(m.rowOf(p - 1))
	case access.UnitWord:
		starts, _ := m.words(p)
		return starts[max(sort.SearchInts(starts, p)-1, 0)]
	}
	return 0
}

// Line returns the visual line that holds point p, and the point where it ends: a display row,
// which is a row, or one segment of a row when the lines wrap. A segment ends where the next one
// starts, and the last segment of a row ends after its line feed. A hidden row has no height,
// so it has no rectangle.
func (m *editorModel) Line(p int) (access.TextLine, int) {
	row, dr, line := m.segmentAt(p)
	ls := m.t.LineStart(row)
	ln := access.TextLine{
		Start: dr.from,
		Top:   float32(m.index().indexOf(row)+dr.seg)*m.geo.lineH - m.scrollY,
	}
	if !m.hiddenRow(row) {
		ln.Height = m.geo.lineH
	}
	text := strings.TrimRight(line[dr.from-ls:dr.to-ls], " 	")
	ln.Right = m.cellX(dr.cellOf(line, ls, dr.from+len(text), m.tab))
	end := dr.to
	if dr.last(ls + len(line)) {
		end = m.lineEnd(row)
	}
	return ln, end
}

// CaretX returns the x of point p in layout units, on its display row.
func (m *editorModel) CaretX(p int) float32 {
	row, dr, line := m.segmentAt(p)
	return m.cellX(dr.cellOf(line, m.t.LineStart(row), p, m.tab))
}

// cellX returns the x of a cell of a display row in layout units. The layout starts at the
// top-left of the canvas, so the text is at the gutter's edge less the horizontal scroll.
func (m *editorModel) cellX(cell int) float32 {
	return m.geo.textX - m.scrollX + float32(cell)*m.geo.charW
}

// Nearest returns the point nearest to the position (x, y) in layout units, as the pointer
// finds it (see viewState.hit). It is 0 before the first paint.
func (m *editorModel) Nearest(x, y float32) int {
	g := m.geo
	if !g.painted || g.lineH <= 0 || g.charW <= 0 {
		return 0
	}
	ix := m.index()
	row, seg, found := ix.at(max(int((y+m.scrollY)/g.lineH), 0))
	if !found {
		row, seg, _ = ix.at(max(ix.count()-1, 0))
	}
	line := m.lineText(row)
	dr := m.segment(row, line, m.wrapOf(line), seg)
	cell := int((x - g.textX + m.scrollX + g.charW/2) / g.charW)
	return dr.offsetAt(line, m.t.LineStart(row), max(cell, 0), m.tab)
}

// index returns the index of the display rows, on the copies of the folds and of the prefix.
func (m *editorModel) index() rowIndex {
	return rowIndex{prefix: m.prefix, hidden: m.hidden, lines: m.t.LineCount()}
}

// wrapOf returns the wrap of a row whose text is line. The model wraps the row again, as the view
// did, since it keeps no copy of the view's wrap of the rows.
func (m *editorModel) wrapOf(line string) rowWrap {
	if !m.wrapped {
		return nil
	}
	return wrapSegments(line, m.wrap.cols, m.wrap.tab, m.wrap.indent)
}

// segment returns the display row of segment seg of row, whose text is line and whose wrap is w.
func (m *editorModel) segment(row int, line string, w rowWrap, seg int) displayRow {
	ls := m.t.LineStart(row)
	seg = min(seg, w.segs()-1)
	b, cell := w.start(seg)
	dr := displayRow{row: row, seg: seg, from: ls + b, to: ls + len(line), startCell: cell}
	if seg+1 < w.segs() {
		next, _ := w.start(seg + 1)
		dr.to = ls + next
	}
	if seg > 0 {
		dr.indent = w.indentCells()
	}
	return dr
}

// segmentAt returns the row that holds point p, the display row of p on it, and the row's text.
func (m *editorModel) segmentAt(p int) (int, displayRow, string) {
	row := m.rowOf(p)
	line := m.lineText(row)
	w := m.wrapOf(line)
	return row, m.segment(row, line, w, w.segmentOf(p-m.t.LineStart(row), false)), line
}

// Read returns the text of the points [from, to), cut to at most limit UTF-16 code units. It
// reads only that range, and a negative limit means no limit.
func (m *editorModel) Read(from, to, limit int) string {
	a, b := m.ByteOf(from), max(m.ByteOf(to), m.ByteOf(from))
	if limit >= 0 {
		// A UTF-16 code unit takes at most three bytes of UTF-8, and a surrogate pair takes
		// four for two units, so four bytes per unit hold limit units with room to spare.
		b = min(b, a+4*limit)
	}
	s := m.t.Slice(a, b)
	// A character that the limit cuts is dropped, since UTF16Prefix takes whole characters.
	if n := len(s); n > 0 {
		i := n - 1
		for i > 0 && !utf8.RuneStart(s[i]) {
			i--
		}
		if !utf8.FullRune(s[i:]) {
			s = s[:i]
		}
	}
	return access.UTF16Prefix(string(s), limit)
}

// hiddenRow reports whether row is hidden by a fold, as Document.hiddenRow does.
func (m *editorModel) hiddenRow(row int) bool {
	for _, r := range m.hidden {
		if r.from <= row && row <= r.to {
			return true
		}
	}
	return false
}

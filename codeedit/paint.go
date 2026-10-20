package codeedit

import (
	"slices"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/ui"
)

// paintSettings is what one render of the editor gives the paint: the resolved look, the
// theme, and the options that change the drawing.
type paintSettings struct {
	st       Style
	t        *ui.Theme
	numbers  bool
	tab      int
	readOnly bool
	focused  bool
}

// capStyle is the theme's look of one capture of the highlighter. ok is false when the theme
// has no style for it, and the text keeps the editor's text color.
type capStyle struct {
	st ui.SyntaxStyle
	ok bool
}

// expandRun returns s with its tabs replaced by spaces, for s drawn from cell.
func expandRun(s string, cell, tab int) string {
	if !containsTab(s) {
		return s
	}
	var b []byte
	for _, r := range s {
		if r == '\t' {
			n := tab - cell%tab
			for i := 0; i < n; i++ {
				b = append(b, ' ')
			}
			cell += n
			continue
		}
		b = utf8.AppendRune(b, r)
		cell += runeCells(r)
	}
	return string(b)
}

func containsTab(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			return true
		}
	}
	return false
}

// paint draws the editor into the canvas of w by h: the display rows that are shown, from the
// scroll position, with the gutter, the selections, the matches, the text and the carets.
// It records the geometry for the pointer, and asks the highlighter for the rows shown.
func (vs *viewState) paint(p *ui.Painter, w, h float32, ps paintSettings) {
	doc := vs.doc
	st := ps.st
	ts := ui.TextFormat{FontSize: st.Text.FontSize, Mono: true}
	charW, lineH := p.MeasureText("0", ts)
	if charW <= 0 || lineH <= 0 {
		return
	}
	if vs.fontName == "" {
		// The face does not change while the editor is shown, so its name is read once.
		vs.fontName = p.FontName(ts)
	}
	tab := ps.tab
	digits := len(strconv.Itoa(doc.LineCount()))
	numW := float32(0)
	if ps.numbers {
		numW = float32(digits)*charW + 12
	}
	foldW := max(lineH*0.6, 10)
	foldX := numW
	textX := numW + foldW + 4
	textW := max(w-textX, 0)
	vs.syncWrap(textW, charW, lineH)
	x := vs.rowIndex()
	rows := x.count()

	// The scroll position: the caret is kept in view after a move, and the position stays in
	// the content. Rows wrapped to the view's width do not scroll horizontally.
	vs.geo = geometry{lineH: lineH, charW: charW, gutterW: textX, textX: textX, foldX: foldX, viewW: w, viewH: h, painted: true}
	if vs.reveal {
		vs.reveal = false
		vs.revealCaret(x, lineH, charW, h, textW)
	}
	maxY := max(float32(rows)*lineH-h, 0)
	maxX := max(float32(vs.maxCells)*charW+charW*2-textW, 0)
	if vs.fitsView() {
		maxX = 0
	}
	vs.scrollY = min(max(vs.scrollY, 0), maxY)
	vs.scrollX = min(max(vs.scrollX, 0), maxX)

	first := int(vs.scrollY / lineH)
	last := min(first+int(h/lineH)+2, rows)
	shown := make([]displayRow, 0, max(last-first, 0))
	for i := first; i < last; i++ {
		dr, ok := vs.rowAt(x, i)
		if !ok {
			break
		}
		shown = append(shown, dr)
	}
	vs.syncHighlighter(shown)

	carets := make(map[int]bool)
	for _, s := range doc.sels {
		carets[doc.RowOf(s.Head)] = true
	}
	var matches []Match
	if vs.bar == barFind || vs.bar == barReplace || vs.bar == barGoto {
		matches = vs.matches()
	}
	cur := -1
	if len(matches) > 0 {
		cur = vs.currentMatch(matches)
	}
	fg := st.Text.TextColor
	maxCells := 0
	tx := textX - vs.scrollX
	line, lineRow := "", -1 // the text of the row of the last segment, read once for its segments
	var text string         // the row with the composition, when it shows on the row
	var at int
	var composing bool
	var cw rowWrap // the wrap of text
	for k, dr := range shown {
		row := dr.row
		y := float32(first+k)*lineH - vs.scrollY
		ls, le := doc.LineStart(row), doc.LineEnd(row)
		if row != lineRow {
			line, lineRow = doc.Line(row), row
			text, at, composing = vs.composedRow(row, line)
			if composing {
				cw = vs.composedWrap(text)
			}
		}
		if composing {
			maxCells = max(maxCells, cellOfCol(text, len(text), tab))
		} else {
			maxCells = max(maxCells, dr.endCell(line, ls, tab))
		}
		if carets[row] && !st.ActiveLine.Background.IsZero() {
			p.FillRect(0, y, w, lineH, st.ActiveLine.Background)
		}
		for _, s := range doc.sels {
			from, to := s.Range()
			if from == to || !dr.covers(from, to, le) || st.Selection.Background.IsZero() {
				continue
			}
			x0, x1 := selectionCells(dr, line, ls, le, from, to, tab)
			p.FillRect(tx+float32(x0)*charW, y, float32(x1-x0)*charW, lineH, st.Selection.Background)
		}
		for _, k := range matchesOverlapping(matches, dr.from, dr.end(le)) {
			m := matches[k]
			bg := st.Match.Background
			if k == cur {
				bg = st.CurrentMatch.Background
			}
			x0 := dr.cellOf(line, ls, min(max(m.From, dr.from), dr.to), tab)
			x1 := dr.cellOf(line, ls, min(max(m.To, dr.from), dr.to), tab)
			if !bg.IsZero() {
				p.FillRect(tx+float32(x0)*charW, y, float32(x1-x0)*charW, lineH, bg)
			}
		}
		switch {
		case composing:
			a, b, sc, ind := composedSegment(cw, dr, len(text), vs.wrapOf(row).segs())
			vs.drawComposition(p, ts, text, at, a, b, sc, ind, tx, y, charW, lineH, tab, fg)
		case dr.from < dr.to:
			vs.drawSegment(p, ts, line, ls, dr, tx, y, charW, tab, fg)
		}
	}
	// The gutter covers the text that is scrolled under it, and the numbers and the fold
	// markers are drawn over it, on the first segment of each row.
	if !st.Gutter.Background.IsZero() {
		p.FillRect(0, 0, textX, h, st.Gutter.Background)
	}
	vs.maxCells = maxCells
	gutterFg := st.Gutter.TextColor
	nts := ui.TextFormat{FontSize: st.Gutter.FontSize, Mono: true, Color: gutterFg, Align: ui.End}
	for k, dr := range shown {
		if dr.seg != 0 {
			continue
		}
		row := dr.row
		y := float32(first+k)*lineH - vs.scrollY
		if ps.numbers {
			nts.Color = gutterFg
			if carets[row] && !ps.t.CodeEditor.ActiveLineNumber.IsZero() {
				nts.Color = ps.t.CodeEditor.ActiveLineNumber
			}
			p.Text(numW-6, y, strconv.Itoa(row+1), nts)
		}
		if doc.opensBlock(row, tab) {
			vs.drawFoldMarker(p, row, foldX, foldW, y, lineH, gutterFg)
		}
	}
	// The caret of each selection, and the brackets around the primary caret's bracket.
	if vs.restart {
		vs.activity, vs.restart = p.Now(), false
	}
	if ps.focused {
		for _, s := range doc.sels {
			r, vi := vs.caretRectIn(x, s.Head)
			if vi < first || vi >= last || !p.CaretVisible(vs.activity) {
				continue
			}
			p.FillRect(r.X, r.Y, 2, lineH, fg)
		}
	}
	vs.drawBrackets(p, x, tx, charW, lineH, first, last, st)
}

// covers reports whether the range [from, to) of a row that ends at le shows on the segment: it
// overlaps the segment, or on the last segment it reaches the line feed, where the break shows.
func (dr displayRow) covers(from, to, le int) bool {
	if dr.to == le {
		return to >= dr.from && from <= le+1
	}
	return to > dr.from && from < dr.to
}

// selectionCells returns the cells of the view where the selection [from, to) covers the segment
// dr of the row line, which runs from ls to le. A selection that goes past the line feed covers
// one more cell on the last segment, so the break shows as selected.
func selectionCells(dr displayRow, line string, ls, le, from, to, tab int) (int, int) {
	a := min(max(from, dr.from), dr.to)
	b := min(max(to, dr.from), dr.to)
	x0 := dr.cellOf(line, ls, a, tab)
	x1 := dr.cellOf(line, ls, b, tab)
	if to > le && dr.to == le {
		x1++
	}
	return x0, x1
}

// end returns the end of the bytes the segment shows: its end, or past the line feed of its row,
// which ends at le, on the last segment, where a selection or a match shows the break.
func (dr displayRow) end(le int) int {
	if dr.last(le) {
		return le + 1
	}
	return dr.to
}

// matchesOverlapping returns the indices of the matches that overlap the bytes [from, to), with
// an empty match at from or after it. A match that spans display rows is on each of them.
func matchesOverlapping(matches []Match, from, to int) []int {
	if len(matches) == 0 {
		return nil
	}
	// The matches are sorted and do not overlap, so their ends are sorted too.
	i := sort.Search(len(matches), func(i int) bool { return matches[i].To > from || matches[i].From >= from })
	var out []int
	for k := i; k < len(matches) && matches[k].From < to; k++ {
		out = append(out, k)
	}
	return out
}

// drawSegment draws the text of the segment dr of the row line, which starts at byte ls, at tx,
// with its rows at y. The spans of the highlighter color the runs; a run with no span takes fg.
func (vs *viewState) drawSegment(p *ui.Painter, ts ui.TextFormat, line string, ls int, dr displayRow, tx, y, charW float32, tab int, fg ui.Color) {
	ts.Color = fg
	a, b := dr.from-ls, dr.to-ls
	x0 := tx + float32(dr.indent)*charW // the x of the segment's first cell
	if vs.hl == nil || vs.spans.list == nil {
		p.Text(x0, y, expandRun(line[a:b], dr.startCell, tab), ts)
		return
	}
	idx := vs.spans.around(dr.from, dr.to)
	if len(idx) == 0 {
		p.Text(x0, y, expandRun(line[a:b], dr.startCell, tab), ts)
		return
	}
	// The pieces are cut at every span edge, and each takes the last span that covers it.
	cuts := []int{a, b}
	for _, k := range idx {
		s := vs.spans.list[k]
		cuts = append(cuts, min(max(s.Start-ls, a), b), min(max(s.End-ls, a), b))
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	cell, at := dr.startCell, a // the cell of the row at the byte at, counted from the segment's start
	for i := 0; i+1 < len(cuts); i++ {
		ca, cb := cuts[i], cuts[i+1]
		if ca == cb || !utf8.RuneStart(line[ca]) || (cb < len(line) && !utf8.RuneStart(line[cb])) {
			continue
		}
		cell, at = cellIn(line, at, cell, ca, tab), ca
		seg := vs.segmentStyle(ts, fg, idx, ls, ca, cb)
		p.Text(x0+float32(cell-dr.startCell)*charW, y, expandRun(line[ca:cb], cell, tab), seg)
	}
}

// drawComposition draws the bytes [a, b) of the row shown, which has the composition at byte at,
// in fg: the part of it on one display row, which starts at the cell sc of the row and is drawn
// after ind cells of indent. The composition is underlined with a thin line, and the clause being
// converted with a thick one. The row takes no syntax colors, since the spans of the highlighter
// do not follow the text inserted into it.
func (vs *viewState) drawComposition(p *ui.Painter, ts ui.TextFormat, shown string, at, a, b, sc, ind int, tx, y, charW, lineH float32, tab int, fg ui.Color) {
	ts.Color = fg
	x0 := tx + float32(ind)*charW
	p.Text(x0, y, expandRun(shown[a:b], sc, tab), ts)
	underline := func(from, to int, h float32) {
		from, to = min(max(from, a), b), min(max(to, a), b)
		if from >= to {
			return
		}
		u0 := x0 + float32(cellIn(shown, a, sc, from, tab)-sc)*charW
		u1 := x0 + float32(cellIn(shown, a, sc, to, tab)-sc)*charW
		p.FillRect(u0, y+lineH-h, u1-u0, h, fg)
	}
	c := vs.comp
	underline(at, at+len(c.Text), 1)
	if c.ClauseEnd > c.ClauseStart {
		underline(at+c.ClauseStart, at+c.ClauseEnd, 2)
	}
}

// segmentStyle returns the text style of the run [a, b) of the row that starts at byte ls, given
// the spans idx that cover the row. The capture of the last span that covers the run sets its
// color, weight and slant; a run with no style takes ts, with fg as its color.
func (vs *viewState) segmentStyle(ts ui.TextFormat, fg ui.Color, idx []int, ls, a, b int) ui.TextFormat {
	seg := ts
	seg.Color = fg
	for _, k := range idx {
		s := vs.spans.list[k]
		if s.Start-ls <= a && s.End-ls >= b && s.Capture >= 0 && s.Capture < len(vs.capStyle) {
			if c := vs.capStyle[s.Capture]; c.ok {
				seg.Color = c.st.Color
				seg.Italic = c.st.Italic
				if c.st.Bold {
					seg.Weight = ui.FontSemibold
				}
			}
		}
	}
	if seg.Color.IsZero() {
		seg.Color = fg
	}
	return seg
}

// drawFoldMarker draws the marker of a row that opens a block: a triangle pointing down when
// its lines are shown, and to the right when they are folded.
func (vs *viewState) drawFoldMarker(p *ui.Painter, row int, foldX, foldW, y, lineH float32, c ui.Color) {
	s := lineH * 0.22
	cx := foldX + foldW/2
	cy := y + lineH/2
	path := ui.NewPath()
	if vs.doc.Folded(row) {
		path.MoveTo(cx-s/2, cy-s).LineTo(cx+s, cy).LineTo(cx-s/2, cy+s).Close()
	} else {
		path.MoveTo(cx-s, cy-s/2).LineTo(cx+s, cy-s/2).LineTo(cx, cy+s).Close()
	}
	p.Fill(path, c)
}

// drawBrackets outlines the bracket at the primary caret and the one that pairs with it, when
// the caret is at a bracket and there is one caret. x is the index of the display rows.
func (vs *viewState) drawBrackets(p *ui.Painter, x rowIndex, tx, charW, lineH float32, first, last int, st Style) {
	doc := vs.doc
	if len(doc.sels) != 1 || st.CurrentMatch.Background.IsZero() {
		return
	}
	head := doc.Caret()
	q, ok := doc.MatchingBracket(head)
	if !ok {
		return
	}
	for _, at := range []int{head, q} {
		if at >= doc.LineEnd(doc.RowOf(at)) {
			continue
		}
		vi, cell := vs.displayOfIn(x, at, false)
		if vi < first || vi >= last {
			continue
		}
		y := float32(vi)*lineH - vs.scrollY
		p.FillRect(tx+float32(cell)*charW, y, charW, lineH, st.CurrentMatch.Background)
	}
}

// revealCaret scrolls the view so that the primary caret is in it, with a margin on the
// sides, as the caret moves. x is the index of the display rows.
func (vs *viewState) revealCaret(x rowIndex, lineH, charW, h, textW float32) {
	head := vs.doc.Caret()
	vi, cell := vs.displayOfIn(x, head, vs.upstreamNow())
	top := float32(vi) * lineH
	if top < vs.scrollY {
		vs.scrollY = top
	} else if top+lineH > vs.scrollY+h {
		vs.scrollY = top + lineH - h
	}
	if vs.fitsView() {
		return
	}
	cx := float32(cell) * charW
	if cx < vs.scrollX {
		vs.scrollX = max(cx-charW*4, 0)
	} else if cx+charW > vs.scrollX+textW {
		vs.scrollX = cx + charW*5 - textW
	}
}

// syncHighlighter tells the highlighter which bytes the display rows shown cover, when they
// changed, and refreshes the sorted spans and the capture styles when the highlighter's spans
// changed.
func (vs *viewState) syncHighlighter(shown []displayRow) {
	if vs.hl == nil {
		return
	}
	if len(shown) > 0 {
		lo, hi := shown[0].from, shown[len(shown)-1].to
		if !vs.vpSet || lo != vs.vpLo || hi != vs.vpHi {
			vs.vpLo, vs.vpHi, vs.vpSet = lo, hi, true
			_ = vs.hl.SetViewport(lo, hi) // the highlighter reports its own errors
		}
	}
	spans, ver := vs.hl.Spans()
	// The styles of the captures follow the theme too: a switch to the light theme recolors the
	// text without new spans.
	if th := vs.ctxTheme(); !vs.spans.ok || vs.spans.ver != ver || vs.capTheme != th {
		vs.spans.build(spans, ver)
		vs.capTheme = th
		styles, ok := captureStyles(vs.hl.CaptureNames(), th.Syntax)
		vs.capStyle = vs.capStyle[:0]
		for i := range styles {
			vs.capStyle = append(vs.capStyle, capStyle{st: styles[i], ok: ok[i]})
		}
	}
}

// ctxTheme returns the theme of the latest render, or the built-in dark theme before one.
func (vs *viewState) ctxTheme() *ui.Theme {
	if vs.ctx != nil {
		return vs.ctx.Theme()
	}
	return ui.DarkTheme()
}

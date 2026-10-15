package codeedit

import (
	"slices"
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

// paint draws the editor into the canvas of w by h: the rows that are shown, from the
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
	rows := doc.VisibleRows()

	// The scroll position: the caret is kept in view after a move, and the position stays in
	// the content.
	if vs.reveal {
		vs.reveal = false
		vs.revealCaret(lineH, charW, h, textW, tab)
	}
	maxY := max(float32(rows)*lineH-h, 0)
	maxX := max(float32(vs.maxCells)*charW+charW*2-textW, 0)
	vs.scrollY = min(max(vs.scrollY, 0), maxY)
	vs.scrollX = min(max(vs.scrollX, 0), maxX)
	vs.geo = geometry{lineH: lineH, charW: charW, gutterW: textX, textX: textX, foldX: foldX, viewW: w, viewH: h, painted: true}

	first := int(vs.scrollY / lineH)
	last := min(first+int(h/lineH)+2, rows)
	vs.syncHighlighter(first, last)

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
	for vi := first; vi < last; vi++ {
		row, ok := doc.RowAtVisible(vi)
		if !ok {
			break
		}
		y := float32(vi)*lineH - vs.scrollY
		ls, le := doc.LineStart(row), doc.LineEnd(row)
		line := doc.Line(row)
		shown, at, composing := vs.composedRow(row, line)
		maxCells = max(maxCells, cellOfCol(shown, len(shown), tab))
		if carets[row] && !st.ActiveLine.Background.IsZero() {
			p.FillRect(0, y, w, lineH, st.ActiveLine.Background)
		}
		for _, s := range doc.sels {
			from, to := s.Range()
			if from == to || to < ls || from > le+1 || st.Selection.Background.IsZero() {
				continue
			}
			x0, x1 := selectionSpan(line, ls, le, from, to, tab)
			p.FillRect(tx+float32(x0)*charW, y, float32(x1-x0)*charW, lineH, st.Selection.Background)
		}
		for _, k := range matchesOnRow(matches, ls, le) {
			m := matches[k]
			bg := st.Match.Background
			if k == cur {
				bg = st.CurrentMatch.Background
			}
			x0 := cellOfCol(line, max(m.From-ls, 0), tab)
			x1 := cellOfCol(line, min(m.To-ls, len(line)), tab)
			if !bg.IsZero() {
				p.FillRect(tx+float32(x0)*charW, y, float32(x1-x0)*charW, lineH, bg)
			}
		}
		switch {
		case composing:
			vs.drawComposition(p, ts, shown, at, tx, y, charW, lineH, tab, fg)
		case line != "":
			vs.drawLine(p, ts, line, ls, tx, y, charW, tab, fg)
		}
	}
	// The gutter covers the text that is scrolled under it, and the numbers and the fold
	// markers are drawn over it.
	if !st.Gutter.Background.IsZero() {
		p.FillRect(0, 0, textX, h, st.Gutter.Background)
	}
	vs.maxCells = maxCells
	gutterFg := st.Gutter.TextColor
	nts := ui.TextFormat{FontSize: st.Gutter.FontSize, Mono: true, Color: gutterFg, Align: ui.End}
	for vi := first; vi < last; vi++ {
		row, ok := doc.RowAtVisible(vi)
		if !ok {
			break
		}
		y := float32(vi)*lineH - vs.scrollY
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
			row := doc.RowOf(s.Head)
			vi := doc.VisibleIndex(row)
			if vi < first || vi >= last || !p.CaretVisible(vs.activity) {
				continue
			}
			line := doc.Line(row)
			cell := cellOfCol(line, s.Head-doc.LineStart(row), tab)
			if shown, at, ok := vs.composedRow(row, line); ok && s.Head == doc.Caret() {
				// The caret of the primary selection is at the input method's caret.
				cell = cellOfCol(shown, at+vs.comp.Cursor, tab)
			}
			x := tx + float32(cell)*charW
			cy := float32(vi)*lineH - vs.scrollY
			p.FillRect(x, cy, 2, lineH, fg)
		}
	}
	vs.drawBrackets(p, ts, tx, charW, lineH, first, last, tab, st)
}

// selectionSpan returns the cells where the selection [from, to) covers the row that runs
// from ls to le. A selection that goes past the line feed covers one more cell there, so the
// break shows as selected.
func selectionSpan(line string, ls, le, from, to, tab int) (int, int) {
	a := max(from-ls, 0)
	b := min(to-ls, len(line))
	x0 := cellOfCol(line, min(a, len(line)), tab)
	x1 := cellOfCol(line, b, tab)
	if to > le {
		x1++
	}
	return x0, x1
}

// matchesOnRow returns the indices of the matches that begin on the row from ls to le.
func matchesOnRow(matches []Match, ls, le int) []int {
	if len(matches) == 0 {
		return nil
	}
	i, _ := slices.BinarySearchFunc(matches, ls, func(m Match, t int) int { return m.From - t })
	var out []int
	for k := i; k < len(matches) && matches[k].From <= le; k++ {
		if matches[k].From >= ls {
			out = append(out, k)
		}
	}
	return out
}

// drawLine draws the text of the row line, which starts at byte ls, at tx, with its rows
// at y. The spans of the highlighter color the runs; a run with no span takes fg.
func (vs *viewState) drawLine(p *ui.Painter, ts ui.TextFormat, line string, ls int, tx, y, charW float32, tab int, fg ui.Color) {
	ts.Color = fg
	if vs.hl == nil || vs.spans.list == nil {
		p.Text(tx, y, expandRun(line, 0, tab), ts)
		return
	}
	le := ls + len(line)
	idx := vs.spans.around(ls, le)
	if len(idx) == 0 {
		p.Text(tx, y, expandRun(line, 0, tab), ts)
		return
	}
	// The segments are cut at every span edge, and each takes the last span that covers it.
	cuts := []int{0, len(line)}
	for _, k := range idx {
		s := vs.spans.list[k]
		cuts = append(cuts, min(max(s.Start-ls, 0), len(line)), min(max(s.End-ls, 0), len(line)))
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	for i := 0; i+1 < len(cuts); i++ {
		a, b := cuts[i], cuts[i+1]
		if a == b || !utf8.RuneStart(line[a]) || (b < len(line) && !utf8.RuneStart(line[b])) {
			continue
		}
		seg := vs.segmentStyle(ts, fg, idx, ls, a, b)
		cell := cellOfCol(line, a, tab)
		p.Text(tx+float32(cell)*charW, y, expandRun(line[a:b], cell, tab), seg)
	}
}

// drawComposition draws the row shown, which has the composition at byte at, in fg. The
// composition is underlined with a thin line, and the clause being converted with a thick one.
// The row takes no syntax colors, since the spans of the highlighter do not follow the text
// inserted into it.
func (vs *viewState) drawComposition(p *ui.Painter, ts ui.TextFormat, shown string, at int, tx, y, charW, lineH float32, tab int, fg ui.Color) {
	ts.Color = fg
	p.Text(tx, y, expandRun(shown, 0, tab), ts)
	underline := func(from, to int, h float32) {
		x0 := tx + float32(cellOfCol(shown, from, tab))*charW
		x1 := tx + float32(cellOfCol(shown, to, tab))*charW
		p.FillRect(x0, y+lineH-h, x1-x0, h, fg)
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
// the caret is at a bracket and there is one caret.
func (vs *viewState) drawBrackets(p *ui.Painter, ts ui.TextFormat, tx, charW, lineH float32, first, last, tab int, st Style) {
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
		row := doc.RowOf(at)
		vi := doc.VisibleIndex(row)
		if vi < first || vi >= last {
			continue
		}
		line := doc.Line(row)
		col := at - doc.LineStart(row)
		if col >= len(line) {
			continue
		}
		x := tx + float32(cellOfCol(line, col, tab))*charW
		y := float32(vi)*lineH - vs.scrollY
		p.FillRect(x, y, charW, lineH, st.CurrentMatch.Background)
	}
}

// revealCaret scrolls the view so that the primary caret is in it, with a margin on the
// sides, as the caret moves.
func (vs *viewState) revealCaret(lineH, charW, h, textW float32, tab int) {
	doc := vs.doc
	head := doc.Caret()
	row := doc.RowOf(head)
	top := float32(doc.VisibleIndex(row)) * lineH
	if top < vs.scrollY {
		vs.scrollY = top
	} else if top+lineH > vs.scrollY+h {
		vs.scrollY = top + lineH - h
	}
	line := doc.Line(row)
	cx := float32(cellOfCol(line, head-doc.LineStart(row), tab)) * charW
	if cx < vs.scrollX {
		vs.scrollX = max(cx-charW*4, 0)
	} else if cx+charW > vs.scrollX+textW {
		vs.scrollX = cx + charW*5 - textW
	}
}

// syncHighlighter tells the highlighter which bytes the rows from first to last show, when
// they changed, and refreshes the sorted spans and the capture styles when the highlighter's
// spans changed.
func (vs *viewState) syncHighlighter(first, last int) {
	if vs.hl == nil {
		return
	}
	doc := vs.doc
	if first < last {
		lo := doc.LineStart(mustRow(doc, first))
		hi := doc.LineEnd(mustRow(doc, last-1))
		if !vs.vpSet || lo != vs.vpLo || hi != vs.vpHi {
			vs.vpLo, vs.vpHi, vs.vpSet = lo, hi, true
			_ = vs.hl.SetViewport(lo, hi) // the highlighter reports its own errors
		}
	}
	spans, ver := vs.hl.Spans()
	if !vs.spans.ok || vs.spans.ver != ver {
		vs.spans.build(spans, ver)
		styles, ok := captureStyles(vs.hl.CaptureNames(), vs.ctxTheme().Syntax)
		vs.capStyle = vs.capStyle[:0]
		for i := range styles {
			vs.capStyle = append(vs.capStyle, capStyle{st: styles[i], ok: ok[i]})
		}
	}
}

// mustRow returns the row of the i-th shown row. Callers pass rows in range.
func mustRow(doc *Document, i int) int {
	row, _ := doc.RowAtVisible(i)
	return row
}

// ctxTheme returns the theme of the latest render, or the built-in dark theme before one.
func (vs *viewState) ctxTheme() *ui.Theme {
	if vs.ctx != nil {
		return vs.ctx.Theme()
	}
	return ui.DarkTheme()
}

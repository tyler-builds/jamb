package codeedit

import (
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/ui"
)

// An input method (IME) composes text, such as Chinese, Japanese or Korean,
// before it is typed. The editor shows the composition inline at the primary
// caret, underlined, and the document does not change until the composition is
// committed. A click in the editor, a key the editor handles, and focus moving
// away each commit it, as one edit. A confirmed result arrives as typed text,
// which replaces the composition. A read-only editor shows no composition.

// wireIME connects the canvas to the input method (see ui.DivElement.IME).
func (e *EditorElement) wireIME(canvas *ui.DivElement, vs *viewState) {
	canvas.IME(ui.IME{
		Update: func(c ui.Composition) {
			if e.readOnly {
				c = ui.Composition{}
			}
			vs.comp = c
			vs.touch()
		},
		Commit: func() {
			text := vs.comp.Text
			vs.comp = ui.Composition{}
			if text == "" || e.readOnly {
				vs.touch()
				return
			}
			vs.edit(func(d *Document) error { return d.TypeText(text) })
		},
		Caret: func() scene.Rect { return vs.caretRect(vs.doc.Caret()) },
	})
}

// commitComposition types the composition, if there is one, before the editor changes: a
// click, or a key it handles.
func (vs *viewState) commitComposition() {
	if vs.ctx != nil {
		vs.ctx.CommitComposition()
	}
}

// composedRow returns line, the text of row, with the composition inserted at the primary
// caret when the caret is on that row, and the byte offset of the caret in it. ok is false when
// no composition shows on the row, and then shown is line.
func (vs *viewState) composedRow(row int, line string) (shown string, at int, ok bool) {
	if vs.comp.Text == "" || !vs.focused {
		return line, 0, false
	}
	head := vs.doc.Caret()
	if vs.doc.RowOf(head) != row {
		return line, 0, false
	}
	at = head - vs.doc.LineStart(row)
	return line[:at] + vs.comp.Text + line[at:], at, true
}

// composedWrap returns the wrap of shown, the row with the composition, at the width the rows
// wrap at, or nil when the view does not wrap. The row is wrapped as it shows while it is composed.
func (vs *viewState) composedWrap(shown string) rowWrap {
	if !vs.wrapping() {
		return nil
	}
	k := vs.wc.key
	return wrapSegments(shown, k.cols, k.tab, k.indent)
}

// composedSegment returns the bytes [a, b) of the row with the composition, n bytes long and
// wrapped as cw, that the display row dr shows, with the cell of the row where they start and
// their indent. The row keeps the segs display rows it has without the composition until the
// composition is typed, so its last display row shows the rest of it.
func composedSegment(cw rowWrap, dr displayRow, n, segs int) (a, b, sc, ind int) {
	if dr.seg >= cw.segs() {
		return n, n, 0, 0
	}
	a, sc = cw.start(dr.seg)
	b = n
	if dr.seg+1 < cw.segs() && dr.seg+1 < segs {
		b, _ = cw.start(dr.seg + 1)
	}
	if dr.seg > 0 {
		ind = cw.indentCells()
	}
	return a, b, sc, ind
}

// caretRect returns the caret at the byte offset off in the canvas's pixels, as the geometry of
// the last paint places it. The primary caret is the input method's caret while a composition
// shows, so its rectangle is where the candidate window opens.
func (vs *viewState) caretRect(off int) scene.Rect {
	r, _ := vs.caretRectIn(vs.rowIndex(), off)
	return r
}

// caretRectIn is caretRect on the index x of the display rows, with the display row of the caret.
func (vs *viewState) caretRectIn(x rowIndex, off int) (scene.Rect, int) {
	g := vs.geo
	primary := off == vs.doc.Caret()
	vi, cell := vs.displayOfIn(x, off, primary && vs.upstreamNow())
	if primary {
		row := vs.doc.RowOf(off)
		if shown, at, ok := vs.composedRow(row, vs.doc.Line(row)); ok {
			col := at + vs.comp.Cursor
			cw := vs.composedWrap(shown)
			k := min(cw.segmentOf(col, false), vs.wrapOf(row).segs()-1)
			a, sc := cw.start(k)
			ind := 0
			if k > 0 {
				ind = cw.indentCells()
			}
			vi, cell = x.indexOf(row)+k, cellIn(shown, a, sc, col, vs.tab)-sc+ind
		}
	}
	px := g.textX - vs.scrollX + float32(cell)*g.charW
	py := float32(vi)*g.lineH - vs.scrollY
	return scene.Rect{X: px, Y: py, W: 2, H: g.lineH}, vi
}

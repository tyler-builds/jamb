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
			vi, cell = x.indexOf(row), cellOfCol(shown, at+vs.comp.Cursor, vs.tab)
		}
	}
	px := g.textX - vs.scrollX + float32(cell)*g.charW
	py := float32(vi)*g.lineH - vs.scrollY
	return scene.Rect{X: px, Y: py, W: 2, H: g.lineH}, vi
}

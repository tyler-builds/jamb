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
func (e *Editor) wireIME(canvas *ui.DivElement, vs *viewState) {
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
		Caret: vs.compositionCaret,
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

// compositionCaret returns the primary caret in the canvas's pixels, for the input method's
// candidate window. While a composition shows, it is at the input method's caret. It uses the
// geometry of the last paint.
func (vs *viewState) compositionCaret() scene.Rect {
	g := vs.geo
	head := vs.doc.Caret()
	row := vs.doc.RowOf(head)
	line := vs.doc.Line(row)
	text, col := line, head-vs.doc.LineStart(row)
	if shown, at, ok := vs.composedRow(row, line); ok {
		text, col = shown, at+vs.comp.Cursor
	}
	x := g.textX - vs.scrollX + float32(cellOfCol(text, col, vs.tab))*g.charW
	y := float32(vs.doc.VisibleIndex(row))*g.lineH - vs.scrollY
	return scene.Rect{X: x, Y: y, W: 2, H: g.lineH}
}

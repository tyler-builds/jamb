package views

import (
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

// The notes window's menu bar, and the menu of a note row. Items that have a
// key run an action, so the shortcut shown is the binding's.

// menuBarHeight is the height of the menu bar. The window's content starts below
// it, so the bar is placed over the top of the window's padding.
const menuBarHeight = 30

// menuBar is the bar above the window's content: File, Edit and View. It is
// absolutely positioned, so the content keeps the size of the window below it.
func (a *App) menuBar(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	return ui.Div().Absolute().Top(0).Left(0).Right(0).
		Row().Align(ui.Center).Height(ui.Px(menuBarHeight)).PaddingX(4).
		Background(t.Sidebar).BorderEdges(ui.Edges{Bottom: 1}, t.Border).
		Child(ui.MenuBar(
			ui.TopMenu{Label: "File", Items: func() []ui.MenuItem {
				return []ui.MenuItem{
					{Action: NewNote},
					{Label: "Import...", OnClick: func() { a.importNotes(ctx) }},
					{Label: "Export note...", Disabled: a.s.Find(a.s.Selected.Peek()) == nil,
						OnClick: func() { a.exportNote(ctx) }},
					ui.MenuSeparator(),
					{Label: "Exit", OnClick: ctx.Window().Close},
				}
			}},
			ui.TopMenu{Label: "Edit", Items: func() []ui.MenuItem {
				return []ui.MenuItem{
					{Action: ui.Undo},
					{Action: ui.Redo},
					ui.MenuSeparator(),
					{Action: ui.Cut},
					{Action: ui.Copy},
					{Action: ui.Paste},
					ui.MenuSeparator(),
					{Action: ui.SelectAll},
				}
			}},
			ui.TopMenu{Label: "View", Items: func() []ui.MenuItem {
				return []ui.MenuItem{
					{Action: ToggleTheme},
					ui.MenuSeparator(),
					{Action: ZoomIn},
					{Action: ZoomOut},
					{Action: ZoomReset},
					ui.MenuSeparator(),
					{Action: Palette},
				}
			}},
		))
}

// zoomStep moves the zoom to the next of ui.ZoomLevels above it (dir > 0) or
// below it (dir < 0). It is the step the framework's zoom keys take.
func zoomStep(ctx *ui.Context, dir int) {
	z := ctx.Zoom()
	for i, level := range ui.ZoomLevels {
		if dir > 0 && level > z+0.001 {
			ctx.SetZoom(level)
			return
		}
		if dir < 0 && level >= z-0.001 && i > 0 {
			ctx.SetZoom(ui.ZoomLevels[i-1])
			return
		}
	}
}

// noteMenu is the menu of a note's row: pinning the note and deleting it.
// Delete selects the note and asks for confirmation, as the editor's Delete does.
func (a *App) noteMenu(n *store.Note) []ui.MenuItem {
	pin := "Pin"
	if n.Pinned.Peek() {
		pin = "Unpin"
	}
	return []ui.MenuItem{
		{Label: pin, OnClick: func() { a.s.TogglePin(n) }},
		ui.MenuSeparator(),
		{Label: "Delete", OnClick: func() {
			a.s.Selected.Set(n.ID)
			a.confirm.Set(n.ID)
		}},
	}
}

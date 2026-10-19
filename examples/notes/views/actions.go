package views

import "github.com/tyler-builds/jamb/ui"

// The notes app's actions. Key bindings and the command palette run them, and
// App.Root handles the window-wide ones. The palette (ui.CommandPalette) has keys of
// its own.
var (
	NewNote     = ui.NewAction("notes.new", "New note")
	Search      = ui.NewAction("notes.search", "Search notes")
	PrevNote    = ui.NewAction("notes.prev", "Previous note")
	NextNote    = ui.NewAction("notes.next", "Next note")
	TogglePin   = ui.NewAction("notes.pin", "Pin or unpin note")
	ToggleTheme = ui.NewAction("notes.theme", "Toggle theme")
	// TogglePreview shows or hides the Markdown preview beside the editor.
	TogglePreview = ui.NewAction("notes.preview", "Toggle Markdown preview")
	Palette       = ui.NewAction("notes.palette", "Command palette").Hide()
	// The zoom keys are the framework's built-in ones; these actions name them so
	// the View menu can show them, and they step the zoom as those keys do.
	ZoomIn    = ui.NewAction("notes.zoom-in", "Zoom in")
	ZoomOut   = ui.NewAction("notes.zoom-out", "Zoom out")
	ZoomReset = ui.NewAction("notes.zoom-reset", "Reset zoom")
)

// Actions lists every action of the notes app, for app.Window.Actions, so a
// keymap file can name them.
func Actions() []*ui.Action {
	return []*ui.Action{
		NewNote, Search, PrevNote, NextNote, TogglePin, ToggleTheme, TogglePreview, Palette,
		ZoomIn, ZoomOut, ZoomReset,
	}
}

// Bindings returns the window's key bindings for app.Window.Bindings.
func Bindings() []ui.Binding {
	return []ui.Binding{
		ui.Bind("ctrl+n", NewNote),
		ui.Bind("ctrl+f", Search),
		ui.Bind("alt+up", PrevNote),
		ui.Bind("alt+down", NextNote),
		ui.Bind("ctrl+shift+p", Palette),
		ui.Bind("ctrl+k ctrl+t", ToggleTheme),
		ui.Bind("ctrl+shift+v", TogglePreview), // as in VS Code
		ui.Bind("ctrl+=", ZoomIn),
		ui.Bind("ctrl+-", ZoomOut),
		ui.Bind("ctrl+0", ZoomReset),
	}
}

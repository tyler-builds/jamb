package views

import "github.com/tyler-builds/jamb/ui"

// The notes app's actions. Key bindings and the command palette run them;
// App.Root handles the window-wide ones and the palette handles its own.
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

	// The palette's own actions. They are hidden, so the palette does not list them.
	PaletteUp    = ui.NewAction("palette.up", "Up").Hide()
	PaletteDown  = ui.NewAction("palette.down", "Down").Hide()
	PaletteRun   = ui.NewAction("palette.run", "Run").Hide()
	PaletteClose = ui.NewAction("palette.close", "Close").Hide()
)

// Actions lists every action of the notes app, for app.Window.Actions, so a
// keymap file can name them.
func Actions() []*ui.Action {
	return []*ui.Action{
		NewNote, Search, PrevNote, NextNote, TogglePin, ToggleTheme, TogglePreview, Palette,
		ZoomIn, ZoomOut, ZoomReset,
		PaletteUp, PaletteDown, PaletteRun, PaletteClose,
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
		// The palette's search field is an Editor inside the palette, so these
		// bindings must name both: "Palette > Editor" is deeper than the
		// field's own bindings for the same keys, as in Zed's "Picker > Editor".
		ui.BindIn("Palette > Editor", "up", PaletteUp),
		ui.BindIn("Palette > Editor", "down", PaletteDown),
		ui.BindIn("Palette > Editor", "enter", PaletteRun),
		ui.BindIn("Palette > Editor", "escape", PaletteClose),
	}
}

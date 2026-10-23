package views

import "github.com/tyler-builds/jamb/ui"

// The app's actions. Key bindings, the command palette and the menus run them,
// and App.Root handles them.
var (
	AddRepo     = ui.NewAction("git.add-repo", "Add repository…")
	Refresh     = ui.NewAction("git.refresh", "Refresh")
	Find        = ui.NewAction("git.find", "Find commits")
	NextMatch   = ui.NewAction("git.next-match", "Next match")
	PrevMatch   = ui.NewAction("git.prev-match", "Previous match")
	GoHead      = ui.NewAction("git.go-head", "Go to HEAD")
	CopyHash    = ui.NewAction("git.copy-hash", "Copy commit hash")
	ShowHistory = ui.NewAction("git.history", "Show history")
	ShowTags    = ui.NewAction("git.tags", "Show tags")
	CloseDiff   = ui.NewAction("git.close-diff", "Back to history")
	PrevRepo    = ui.NewAction("git.prev-repo", "Previous repository")
	NextRepo    = ui.NewAction("git.next-repo", "Next repository")
	ToggleTheme = ui.NewAction("git.theme", "Toggle light and dark theme")
	Palette     = ui.NewAction("git.palette", "Command palette").Hide()
)

// Actions lists every action, for app.Window.Actions.
func Actions() []*ui.Action {
	return []*ui.Action{AddRepo, Refresh, Find, NextMatch, PrevMatch, GoHead, CopyHash,
		ShowHistory, ShowTags, CloseDiff, PrevRepo, NextRepo, ToggleTheme, Palette}
}

// Bindings returns the window's key bindings, for app.Window.Bindings.
func Bindings() []ui.Binding {
	return []ui.Binding{
		ui.Bind("ctrl+o", AddRepo),
		ui.Bind("f5", Refresh),
		ui.Bind("ctrl+r", Refresh),
		ui.Bind("ctrl+f", Find),
		ui.Bind("f3", NextMatch),
		ui.Bind("shift+f3", PrevMatch),
		ui.Bind("ctrl+h", GoHead),
		ui.Bind("ctrl+shift+c", CopyHash),
		ui.Bind("ctrl+1", ShowHistory),
		ui.Bind("ctrl+2", ShowTags),
		ui.Bind("ctrl+pageup", PrevRepo),
		ui.Bind("ctrl+pagedown", NextRepo),
		ui.Bind("ctrl+k", Palette), // as in Linear
		ui.Bind("ctrl+shift+p", Palette),
		ui.BindIn("DiffView", "escape", CloseDiff),
	}
}

package views

import "github.com/tyler-builds/jamb/ui"

// The demo's keyboard shortcuts. App runs them for every page; the main window
// gets their bindings from Bindings.
var (
	GoDashboard  = ui.NewAction("nav.dashboard", "Go to Dashboard")
	GoProjects   = ui.NewAction("nav.projects", "Go to Projects")
	GoActivity   = ui.NewAction("nav.activity", "Go to Activity")
	GoComponents = ui.NewAction("nav.components", "Go to Components")
	GoSettings   = ui.NewAction("nav.settings", "Go to Settings")
	GoMarkdown   = ui.NewAction("nav.markdown", "Go to Markdown")
	FocusSearch  = ui.NewAction("search.focus", "Search projects")
	ToggleTheme  = ui.NewAction("theme.toggle", "Toggle theme")
)

// Bindings returns the main window's key bindings for app.Window.Bindings.
func Bindings() []ui.Binding {
	return []ui.Binding{
		ui.Bind("ctrl+1", GoDashboard),
		ui.Bind("ctrl+2", GoProjects),
		ui.Bind("ctrl+3", GoActivity),
		ui.Bind("ctrl+4", GoComponents),
		ui.Bind("ctrl+5", GoSettings),
		ui.Bind("ctrl+6", GoMarkdown),
		ui.Bind("ctrl+k", FocusSearch),
		ui.Bind("ctrl+shift+l", ToggleTheme),
	}
}

// shortcutActions are the actions listed on the Settings page, in the order
// they show there. The window's own list (AvailableActions) is not used: it
// only sees the actions on the focus path, which a page does not have while
// it renders.
var shortcutActions = []*ui.Action{GoDashboard, GoProjects, GoActivity, GoComponents, GoSettings, GoMarkdown, FocusSearch, ToggleTheme}

// tip is label with the keys of a, for a tooltip: "Projects (Ctrl+2)". It is
// just the label when a has no binding.
func tip(ctx *ui.Context, label string, a *ui.Action) string {
	if keys := ctx.KeysFor(a); keys != "" {
		return label + " (" + keys + ")"
	}
	return label
}

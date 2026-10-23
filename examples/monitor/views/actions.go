package views

import "github.com/tyler-builds/jamb/ui"

// The app's actions. Key bindings, the command palette and the menus run them,
// and App.Root handles them.
var (
	Find         = ui.NewAction("monitor.find", "Find processes")
	Pause        = ui.NewAction("monitor.pause", "Pause or resume sampling")
	EndProcess   = ui.NewAction("monitor.end", "End process…")
	CopyName     = ui.NewAction("monitor.copy-name", "Copy process name")
	ShowCPU      = ui.NewAction("monitor.show-cpu", "Show processor")
	ShowMemory   = ui.NewAction("monitor.show-memory", "Show memory")
	ShowNetwork  = ui.NewAction("monitor.show-network", "Show network")
	ShowIO       = ui.NewAction("monitor.show-io", "Show process I/O")
	ShowProcs    = ui.NewAction("monitor.show-processes", "Show processes")
	ShowDetails  = ui.NewAction("monitor.show-details", "Show process details")
	ShowAlerts   = ui.NewAction("monitor.show-alerts", "Show alerts")
	ResetLayout  = ui.NewAction("monitor.reset-layout", "Reset panel layout")
	ToggleTheme  = ui.NewAction("monitor.theme", "Toggle light and dark theme")
	Palette      = ui.NewAction("monitor.palette", "Command palette").Hide()
	ClearAlerts  = ui.NewAction("monitor.clear-alerts", "Clear alerts")
	FasterSample = ui.NewAction("monitor.faster", "Sample more often")
	SlowerSample = ui.NewAction("monitor.slower", "Sample less often")
)

// Actions lists every action, for app.Window.Actions.
func Actions() []*ui.Action {
	return []*ui.Action{Find, Pause, EndProcess, CopyName, ShowCPU, ShowMemory, ShowNetwork, ShowIO,
		ShowProcs, ShowDetails, ShowAlerts, ResetLayout, ToggleTheme, Palette, ClearAlerts,
		FasterSample, SlowerSample}
}

// Bindings returns the window's key bindings, for app.Window.Bindings. Delete and
// Ctrl+C act on the process table only, so text fields keep them.
func Bindings() []ui.Binding {
	return []ui.Binding{
		ui.Bind("ctrl+f", Find),
		ui.Bind("ctrl+p", Pause),
		ui.Bind("ctrl+1", ShowCPU),
		ui.Bind("ctrl+2", ShowMemory),
		ui.Bind("ctrl+3", ShowNetwork),
		ui.Bind("ctrl+4", ShowIO),
		ui.Bind("ctrl+5", ShowProcs),
		ui.Bind("ctrl+6", ShowDetails),
		ui.Bind("ctrl+7", ShowAlerts),
		ui.Bind("ctrl+shift+l", ToggleTheme),
		ui.Bind("ctrl+shift+p", Palette),
		ui.Bind("ctrl+k", Palette),
		ui.Bind("ctrl+]", SlowerSample),
		ui.Bind("ctrl+[", FasterSample),
		ui.BindIn("Processes", "delete", EndProcess),
		ui.BindIn("Processes", "ctrl+c", CopyName),
	}
}

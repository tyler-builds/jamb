// Package views is the monitor's UI, built only from public jamb APIs.
package views

import (
	"image"
	"slices"
	"strconv"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
	"github.com/tyler-builds/jamb/examples/monitor/store"
)

// Layout is what the app keeps across runs, and the hooks it gives the window.
type Layout struct {
	// Dock is the arrangement of the panels.
	Dock *ui.Signal[ui.DockLayout]
	// Order, Widths and Sort are the process table's columns and sort.
	Order  *ui.Signal[[]string]
	Widths *ui.Signal[map[string]float32]
	Sort   *ui.Signal[ui.SortState]
	// Notify shows an OS notification for each alert, and CloseToTray hides the
	// window to the notification area when it is closed.
	Notify, CloseToTray *ui.Signal[bool]
	// Theme is "light", "dark" or "" to follow Windows.
	Theme *ui.Signal[string]
	// Grouped groups the process table by program, as Task Manager does.
	Grouped *ui.Signal[bool]
	// OnChange runs after the user changes something the app keeps.
	OnChange func()
	// Floating opens the window of a floating panel group: app.OpenDockWindow. Nil
	// keeps every panel in the main window, as in the tests.
	Floating func(ui.DockWindow) ui.Window
	// FileIcon reads the icon of an executable: app.FileIcon. Nil shows a plain
	// window for every program, as in the tests.
	FileIcon func(path string, size int) (image.Image, error)
}

// Intervals are the times between samples the app offers.
var Intervals = []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 5 * time.Second}

// App is the window's UI state.
type App struct {
	s      *store.Store
	layout Layout
	// selected holds the key of the selected process, as the table's selection.
	selected *ui.Signal[[]string]
	filter   *ui.Signal[string]
	// expanded holds the keys of the open groups of the process table.
	expanded *ui.Signal[[]string]
	search   *ui.FocusHandle
	grid     *ui.DataGridHandle
	// ending is the process the user asked to end, while the dialog asks to confirm.
	ending  *ui.Signal[*sample.Process]
	palette *ui.Signal[bool]
	// span is how many seconds of history the charts show.
	span *ui.Signal[int]
	// icons holds the programs' icons.
	icons *iconCache
}

// New returns the UI state of the window, filling in the parts of layout left nil.
func New(s *store.Store, layout Layout) *App {
	if layout.Dock == nil {
		layout.Dock = ui.NewSignal(DefaultDock.Clone())
	}
	if layout.Order == nil {
		layout.Order = ui.NewSignal([]string(nil))
	}
	if layout.Widths == nil {
		layout.Widths = ui.NewSignal(map[string]float32(nil))
	}
	if layout.Sort == nil {
		layout.Sort = ui.NewSignal(DefaultSort)
	}
	if layout.Notify == nil {
		layout.Notify = ui.NewSignal(true)
	}
	if layout.CloseToTray == nil {
		layout.CloseToTray = ui.NewSignal(false)
	}
	if layout.Theme == nil {
		layout.Theme = ui.NewSignal("")
	}
	if layout.Grouped == nil {
		layout.Grouped = ui.NewSignal(true)
	}
	return &App{s: s, layout: layout, selected: ui.NewSignal([]string(nil)), filter: ui.NewSignal(""),
		expanded: ui.NewSignal(slices.Clone(sections)),
		search:   ui.NewFocusHandle(), grid: ui.NewDataGridHandle(), ending: ui.NewSignal[*sample.Process](nil),
		palette: ui.NewSignal(false), span: ui.NewSignal(60), icons: newIconCache(layout.FileIcon)}
}

// Root returns the window's render function.
func (a *App) Root() func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		return ui.Div().Column().Size(ui.Full, ui.Full).Background(t.Background).TextColor(t.Text).
			FontSize(t.FontSize).
			OnAction(Find, a.find).
			OnAction(Pause, a.togglePause).
			OnAction(EndProcess, a.askEnd).
			OnAction(CopyName, func() { a.copyName(ctx) }).
			OnAction(ShowCPU, func() { a.showPanel(panelCPU) }).
			OnAction(ShowMemory, func() { a.showPanel(panelMemory) }).
			OnAction(ShowNetwork, func() { a.showPanel(panelNetwork) }).
			OnAction(ShowIO, func() { a.showPanel(panelIO) }).
			OnAction(ShowProcs, func() { a.showPanel(panelProcs) }).
			OnAction(ShowDetails, func() { a.showPanel(panelDetails) }).
			OnAction(ShowAlerts, func() { a.showPanel(panelAlerts) }).
			OnAction(ResetLayout, a.resetLayout).
			OnAction(ToggleTheme, func() { a.toggleTheme(ctx) }).
			OnAction(Palette, func() { a.palette.Set(true) }).
			OnAction(ClearAlerts, func() { a.s.Alerts.Set(nil) }).
			OnAction(FasterSample, func() { a.stepInterval(-1) }).
			OnAction(SlowerSample, func() { a.stepInterval(1) }).
			Child(ui.Component(a.titleBar)).
			Child(ui.Component(a.overview)).
			Child(ui.Div().Grow(1).MinHeight(ui.Px(0)).PaddingX(8).PaddingBottom(8).
				Child(a.dock())).
			Child(a.statusBar()).
			Child(ui.Component(a.endDialog)).
			Child(ui.CommandPalette(a.palette).Placeholder("Type a command…")).
			Child(ui.Toaster().Key("toaster"))
	}
}

func (a *App) changed() {
	if a.layout.OnChange != nil {
		a.layout.OnChange()
	}
}

func (a *App) find() {
	a.showPanel(panelProcs)
	a.search.Focus()
}

func (a *App) togglePause() {
	a.s.Paused.Set(!a.s.Paused.Peek())
	if !a.s.Paused.Peek() {
		a.s.Wake()
	}
}

// stepInterval moves to the next longer (by 1) or shorter (by -1) interval.
func (a *App) stepInterval(by int) {
	i := slices.Index(Intervals, a.s.Interval.Peek())
	if i < 0 {
		i = 1
	}
	a.setInterval(Intervals[max(0, min(len(Intervals)-1, i+by))])
}

func (a *App) setInterval(d time.Duration) {
	if d == a.s.Interval.Peek() {
		return
	}
	a.s.Interval.Set(d)
	a.s.Wake()
	a.changed()
}

// showPanel shows the panel id, opening it again if it was closed, and selects
// its tab.
func (a *App) showPanel(id string) {
	l := a.layout.Dock.Peek()
	if !l.Has(id) {
		l = l.Show(id)
	}
	a.layout.Dock.Set(l.Focus(id))
	a.changed()
}

// ShowAlerts shows the alerts panel, for a click on an alert's notification.
func (a *App) ShowAlerts() { a.showPanel(panelAlerts) }

func (a *App) resetLayout() {
	a.layout.Dock.Set(DefaultDock.Clone())
	a.changed()
}

func (a *App) toggleTheme(ctx *ui.Context) {
	if ctx.Theme().Dark {
		a.layout.Theme.Set("light")
		ctx.SetTheme(ui.LightTheme())
	} else {
		a.layout.Theme.Set("dark")
		ctx.SetTheme(ui.DarkTheme())
	}
	a.changed()
}

// selectedProcess returns the selected process in the latest sample, and whether
// it is still running. It reads the signals, so a render that calls it follows
// both.
func (a *App) selectedProcess() (key string, p sample.Process, ok bool) {
	sel := a.selected.Get()
	if len(sel) == 0 {
		return "", sample.Process{}, false
	}
	p, ok = a.s.Process(sel[0])
	return sel[0], p, ok
}

// selectProcess selects the process with key and scrolls the table to it.
func (a *App) selectProcess(key string) {
	a.selected.Set([]string{key})
	a.grid.ScrollToRow(key)
}

func (a *App) copyName(ctx *ui.Context) {
	key, p, ok := a.peekSelected()
	name := p.Label()
	if !ok {
		g, found := findRow(groupRows(a.s.Latest.Peek().Procs), key)
		if !found {
			return
		}
		name = g.Label()
	}
	ctx.SetClipboard(name)
	ctx.Toast(ui.ToastSuccess, "Copied "+name)
}

// peekSelected is selectedProcess for handlers: it reads without subscribing.
func (a *App) peekSelected() (string, sample.Process, bool) {
	sel := a.selected.Peek()
	if len(sel) == 0 {
		return "", sample.Process{}, false
	}
	for _, p := range a.s.Latest.Peek().Procs {
		if p.Key == sel[0] {
			return sel[0], p, true
		}
	}
	return sel[0], sample.Process{}, false
}

// statusBar shows the counts of the latest sample, the interval and whether the
// sampling is paused or failing.
func (a *App) statusBar() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		snap := a.s.Latest.Get()
		start := []ui.Element{
			ui.Text(count(len(snap.Procs)) + " processes"),
			ui.Text(count(snap.Threads()) + " threads"),
			ui.Text(count(snap.Handles()) + " handles"),
		}
		var state ui.Element
		switch {
		case a.s.Err.Get() != "":
			state = ui.Div().Row().Align(ui.Center).Gap(6).
				Child(ui.Icon(icons.TriangleAlert).Size(13).Color(t.Warning)).
				Child(ui.Text("Sampling failed: " + a.s.Err.Get()).Truncate())
		case a.s.Paused.Get():
			state = ui.Div().Row().Align(ui.Center).Gap(6).
				Child(ui.Icon(icons.Pause).Size(13).Color(t.TextMuted)).Child(ui.Text("Paused"))
		default:
			state = ui.Text("Every " + interval(a.s.Interval.Get()))
		}
		end := []ui.Element{state}
		if !snap.Time.IsZero() {
			end = append(end, ui.Text("Updated "+ui.FormatTime(snap.Time)))
		}
		return ui.StatusBar().Label("Sampling status").Start(start...).End(end...)
	})
}

// endDialog asks before it ends a process. The process may have exited by the
// time the user confirms; the store's source checks it is still the same one.
func (a *App) endDialog(ctx *ui.Context) ui.Element {
	p := a.ending.Get()
	if p == nil {
		return ui.Div()
	}
	name, pid, created := p.Label(), p.PID, p.Created
	return ui.AlertDialog("End "+name+"?",
		"Ending a process can lose its unsaved work, and ending a system process can make Windows unstable.").
		Intent(ui.AlertDialogDestructive).
		Body(ui.Text("Process ID "+strconv.FormatUint(uint64(pid), 10)).FontSize(12).TextColor(ctx.Theme().TextMuted)).
		Confirm("End process", func() {
			a.ending.Set(nil)
			err := a.s.Source().End(pid, created)
			switch {
			case err == nil:
				ctx.Toast(ui.ToastSuccess, "Ended "+name)
			case err == sample.ErrGone:
				ctx.Toast(ui.ToastInfo, name+" had already exited")
			default:
				ctx.Toast(ui.ToastError, "Could not end "+name+": "+err.Error())
			}
		}).
		Cancel("", func() { a.ending.Set(nil) })
}

// askEnd asks to end the selected process.
func (a *App) askEnd() {
	if _, p, ok := a.peekSelected(); ok {
		a.ending.Set(&p)
	}
}

package views

import (
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// titleBar is the app's own title bar: the mark and name at the start, the process
// filter centered on the window, and the interval, Pause, the theme and the window
// buttons at the end.
func (a *App) titleBar(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	mark := ui.Div().Center().Size(ui.Px(22), ui.Px(22)).Radius(6).Shrink(0).
		BackgroundGradient(ui.LinearGradient(135, ui.Hex("#34d399"), ui.Hex("#0f766e")).InOklab()).
		Child(ui.Icon(icons.Activity).Size(13).Color(ui.Hex("#ffffff")))
	paused := a.s.Paused.Get()
	pauseIcon, pauseLabel := icons.Pause, "Pause sampling"
	if paused {
		pauseIcon, pauseLabel = icons.Play, "Resume sampling"
	}
	options := make([]ui.Option[time.Duration], len(Intervals))
	for i, d := range Intervals {
		options[i] = ui.Option[time.Duration]{Value: d, Label: "Every " + interval(d)}
	}
	return ui.TitleBar().Key("titlebar").Height(ui.Px(44)).
		Style(ui.Style{Background: t.Panel, BorderEdges: ui.Edges{Bottom: 1}, BorderColor: t.Border}).
		Child(ui.Div().Row().Align(ui.Center).Gap(10).PaddingLeft(14).MinWidth(ui.Px(0)).
			Child(mark).Child(ui.Text("Monitor").FontWeight(ui.FontSemibold))).
		Center(ui.Component(a.filterField)).
		Trailing(
			ui.Select(a.s.Interval, options...).Key("interval").Small().Label("Sampling interval").
				Width(ui.Px(116)).OnChange(func(time.Duration) { a.s.Wake(); a.changed() }),
			ui.IconButton(pauseIcon).Key("pause").Ghost().Small().Label(pauseLabel).
				Tooltip(pauseLabel+" (Ctrl+P)").AccessChecked(paused).OnClick(a.togglePause),
			ui.IconButton(themeIcon(t)).Ghost().Small().Label("Toggle theme").Tooltip("Light or dark theme").
				OnClick(func() { a.toggleTheme(ctx) }),
		)
}

func themeIcon(t *ui.Theme) *icons.Icon {
	if t.Dark {
		return icons.Sun
	}
	return icons.Moon
}

// filterField narrows the process table to the processes whose name or PID
// matches, and says how many match.
func (a *App) filterField(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	q := a.filter.Get()
	trailing := ui.Element(ui.Kbd("Ctrl+F"))
	if q != "" {
		n := 0
		for _, p := range a.s.Latest.Get().Procs {
			if matches(q, p) {
				n++
			}
		}
		trailing = ui.Text(count(n) + " found").FontSize(11.5).TextColor(t.TextSubtle)
	}
	return ui.TextInput(a.filter).Key("filter").Small().Label("Filter processes").
		Placeholder("Filter by name or PID").Icon(icons.Search).Clearable().
		FocusHandle(a.search).Trailing(trailing).
		Style(ui.Style{Width: ui.Px(300)})
}

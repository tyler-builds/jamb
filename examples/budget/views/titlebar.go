package views

import (
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

// titleBar is the app's own title bar: the name at the start, the month centered on
// the window with its arrows, and Undo, Redo and the theme at the end.
func (a *App) titleBar(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	mark := ui.Div().Center().Size(ui.Px(22), ui.Px(22)).Radius(6).Shrink(0).
		BackgroundGradient(ui.LinearGradient(135, ui.Hex("#a78bfa"), ui.Hex("#6d28d9")).InOklab()).
		Child(ui.Icon(icons.Wallet).Size(13).Color(ui.Hex("#ffffff")))
	undo, redo := a.s.UndoLabel.Get(), a.s.RedoLabel.Get()
	undoTip, redoTip := "Nothing to undo", "Nothing to redo"
	if undo != "" {
		undoTip = "Undo " + undo + " (Ctrl+Z)"
	}
	if redo != "" {
		redoTip = "Redo " + redo + " (Ctrl+Y)"
	}
	theme := icons.Moon
	if t.Dark {
		theme = icons.Sun
	}
	return ui.TitleBar().Key("titlebar").Height(ui.Px(44)).
		Style(ui.Style{Background: t.Panel, BorderEdges: ui.Edges{Bottom: 1}, BorderColor: t.Border}).
		Child(ui.Div().Row().Align(ui.Center).Gap(10).PaddingLeft(14).
			Child(mark).Child(ui.Text("Budget").FontWeight(ui.FontSemibold))).
		Center(ui.Component(a.monthSwitcher)).
		Trailing(
			ui.IconButton(icons.Undo2).Key("undo").Ghost().Small().Label("Undo").Tooltip(undoTip).
				Disabled(undo == "").OnClick(a.s.Undo),
			ui.IconButton(icons.Redo2).Key("redo").Ghost().Small().Label("Redo").Tooltip(redoTip).
				Disabled(redo == "").OnClick(a.s.Redo),
			ui.IconButton(theme).Ghost().Small().Label("Toggle theme").Tooltip("Light or dark theme").
				OnClick(func() { a.toggleTheme(ctx) }),
		)
}

// monthSwitcher shows the month the pages show, with the months before and after it,
// and a way back to this month.
func (a *App) monthSwitcher(ctx *ui.Context) ui.Element {
	m := a.month.Get()
	this := ledger.MonthOf(a.opts.Now())
	row := ui.Div().Row().Align(ui.Center).Gap(4).Key("month").
		Child(ui.IconButton(icons.ChevronLeft).Key("prev-month").Ghost().Small().Label("Previous month").
			Tooltip("Previous month (Alt+Left)").OnClick(func() { a.month.Set(a.month.Peek().Add(-1)) })).
		Child(ui.Text(ui.FormatMonthYear(m.Start())).Key("month-name").FontWeight(ui.FontMedium).
			Align(ui.Center).Width(ui.Px(140))).
		Child(ui.IconButton(icons.ChevronRight).Key("next-month").Ghost().Small().Label("Next month").
			Tooltip("Next month (Alt+Right)").OnClick(func() { a.month.Set(a.month.Peek().Add(1)) }))
	if m != this {
		row.Child(ui.Button("This month").Key("this-month").Ghost().Small().
			OnClick(func() { a.month.Set(this) }))
	}
	return row
}

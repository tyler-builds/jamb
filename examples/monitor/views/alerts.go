package views

import (
	"slices"
	"strconv"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/store"
)

// alertPanel holds the alert rules and the alerts they raised, newest first.
func (a *App) alertPanel(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	alerts := a.s.Alerts.Get()
	firing := a.s.Firing.Get()
	var log ui.Element
	if len(alerts) == 0 {
		log = ui.Empty("No alerts").Icon(icons.BellRing).
			Description("An alert appears here when a rule's limit is passed for its time.")
	} else {
		log = ui.ListOf(alerts, func(al store.Alert) string { return al.Time.String() + string(al.Kind) },
			func(_ int, al store.Alert) ui.Element { return alertRow(t, al) }).
			Key("alert-log").Label("Recent alerts").Grow(1)
	}
	return ui.Div().Column().Size(ui.Full, ui.Full).Padding(12).Gap(12).
		Child(a.ruleRow(ctx, store.AlertCPU, "Processor load", slices.Contains(firing, store.AlertCPU))).
		Child(a.ruleRow(ctx, store.AlertMemory, "Memory use", slices.Contains(firing, store.AlertMemory))).
		Child(ui.Div().Row().Align(ui.Center).Gap(16).Wrap().
			Child(ui.Switch("Show a Windows notification", a.layout.Notify).Key("notify").
				OnChange(func(bool) { a.changed() })).
			Child(ui.Text("Show a Windows notification").FontSize(12.5)).
			Child(ui.Switch("Close to the notification area", a.layout.CloseToTray).Key("close-to-tray").
				OnChange(func(bool) { a.changed() })).
			Child(ui.Text("Close to the notification area").FontSize(12.5))).
		Child(ui.Div().Row().Align(ui.Center).
			Child(ui.Text("Recent alerts").FontSize(12).FontWeight(ui.FontMedium).TextColor(t.TextMuted)).
			Child(ui.Spacer()).
			Child(ui.Button("Clear").Key("clear-alerts").Ghost().Small().Disabled(len(alerts) == 0).
				OnClick(func() { a.s.Alerts.Set(nil) }))).
		Child(ui.Div().Column().Grow(1).MinHeight(ui.Px(0)).Child(log))
}

// ruleRow is one rule: whether it is on, its limit and how long the value must
// stay above it. A rule whose limit is passed now is marked.
func (a *App) ruleRow(ctx *ui.Context, kind store.AlertKind, name string, firing bool) ui.Element {
	rule := a.rule(kind)
	on := ui.Local(ctx, string(kind)+".on", rule.On)
	limit := ui.Local(ctx, string(kind)+".limit", int(rule.Threshold))
	secs := ui.Local(ctx, string(kind)+".for", rule.For)
	save := func() {
		r := store.Rule{On: on.Peek(), Threshold: float64(limit.Peek()), For: secs.Peek()}
		rules := a.s.Rules.Peek()
		switch kind {
		case store.AlertCPU:
			rules.CPU = r
		case store.AlertMemory:
			rules.Memory = r
		}
		a.s.Rules.Set(rules)
		a.changed()
	}
	k := string(kind)
	row := ui.Div().Row().Align(ui.Center).Gap(8).Wrap().Key("rule-" + k).
		Child(ui.Switch(name+" alert", on).Key(k + "-on").OnChange(func(bool) { save() })).
		Child(ui.Text(name + " above").FontSize(12.5)).
		Child(ui.NumberInput(limit, 10, 100, 5).Key(k + "-limit").Small().Width(ui.Px(84)).
			Label(name + " limit, percent").Disabled(!on.Get()).OnChange(func(int) { save() })).
		Child(ui.Text("% for").FontSize(12.5)).
		Child(ui.NumberInput(secs, 1, 600, 1).Key(k + "-for").Small().Width(ui.Px(84)).
			Label(name + " time, seconds").Disabled(!on.Get()).OnChange(func(int) { save() })).
		Child(ui.Text("seconds").FontSize(12.5))
	if firing {
		row.Child(ui.BadgeText("Above its limit").Key(k + "-firing").Variant(ui.BadgeDanger))
	}
	return row
}

func (a *App) rule(kind store.AlertKind) store.Rule {
	rules := a.s.Rules.Peek()
	if kind == store.AlertMemory {
		return rules.Memory
	}
	return rules.CPU
}

// alertRow is one alert in the log.
func alertRow(t *ui.Theme, al store.Alert) ui.Element {
	ic := icons.Cpu
	if al.Kind == store.AlertMemory {
		ic = icons.MemoryStick
	}
	body := ui.Div().Column().Gap(2).Grow(1).MinWidth(ui.Px(0)).
		Child(ui.Text(al.Message).FontSize(12.5)).
		Child(ui.Div().Row().Gap(6).
			Child(ui.TimeAgo(al.Time)).
			Child(ui.Text("·").TextColor(t.TextSubtle)).
			Child(ui.Text("peak " + strconv.Itoa(int(al.Value)) + "%")))
	if al.Top != "" {
		body.Child(ui.Text("Busiest process: " + al.Top).FontSize(12).TextColor(t.TextMuted))
	}
	return ui.Div().Row().Gap(10).PaddingY(8).BorderEdges(ui.Edges{Bottom: 1}, t.Border).
		Child(ui.Icon(ic).Size(15).Color(t.Warning)).Child(body.FontSize(12).TextColor(t.Text))
}

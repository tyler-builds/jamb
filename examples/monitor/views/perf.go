package views

import (
	"strconv"

	"github.com/tyler-builds/jamb/charts"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/store"
)

// spanToggle picks how much history the charts show: the last minute or the last
// five. The charts of every panel share it.
func (a *App) spanToggle() ui.Element {
	return ui.ToggleGroup(a.span,
		ui.Option[int]{Value: 60, Label: "1 min"},
		ui.Option[int]{Value: store.HistoryLen, Label: "5 min"},
	).Key("span").Label("History shown")
}

// panelBody is the column a chart panel lays out in, with the span toggle and a
// line of readings at the top. It is at least as tall as the panel, so a chart
// that grows fills the panel's height; see scroll.
func panelBody(t *ui.Theme, toggle ui.Element, readings ...ui.Element) *ui.DivElement {
	head := ui.Div().Row().Align(ui.Center).Gap(16).Wrap()
	for _, r := range readings {
		head.Child(r)
	}
	return ui.Div().Column().Width(ui.Full).MinHeight(ui.Full).Padding(12).Gap(12).
		Child(ui.Div().Row().Align(ui.Center).Gap(12).
			Child(head.Grow(1)).Child(toggle))
}

// scroll shows a panel's body, scrolling when the panel is shorter than the
// smallest height of its charts.
func scroll(body *ui.DivElement) ui.Element {
	return ui.Div().Size(ui.Full, ui.Full).ScrollY().Child(body)
}

// reading is a value with its name above it, at the top of a panel.
func reading(t *ui.Theme, name, value string) ui.Element {
	return ui.Div().Column().Gap(1).
		Child(ui.Text(name).FontSize(11.5).TextColor(t.TextMuted)).
		Child(ui.Text(value).FontSize(15).FontWeight(ui.FontMedium))
}

func (a *App) cpuPanel(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	snap := a.s.Latest.Get()
	hist := lastSeconds(a.s.History.Get(), a.span.Get())
	type core struct {
		name string
		load float64
	}
	cores := make([]core, len(snap.Cores))
	for i, c := range snap.Cores {
		cores[i] = core{strconv.Itoa(i), c}
	}
	return scroll(panelBody(t, a.spanToggle(),
		reading(t, "Load", percent(snap.CPU)),
		reading(t, "Logical processors", count(len(snap.Cores))),
		reading(t, "Processes", count(len(snap.Procs))),
		reading(t, "Threads", count(snap.Threads())),
	).
		Child(charts.Area(charts.Series[store.Point]{Name: "Processor load", Data: hist, Time: pointTime, Y: pointCPU}).
			Key("cpu-history").YAxis(charts.Axis{Format: percent}.Min(0).Max(100)).
			Legend(charts.LegendNone).Format(percent).Height(ui.Auto).Grow(1).MinHeight(ui.Px(140))).
		Child(charts.Bar(charts.Series[core]{Name: "Load", Data: cores,
			Category: func(c core) string { return c.name }, Y: func(c core) float64 { return c.load }}).
			Key("cores").Title("Each logical processor").
			XAxis(charts.Axis{Label: "Processor"}).YAxis(charts.Axis{Format: percent}.Min(0).Max(100)).
			Legend(charts.LegendNone).Format(percent).Height(ui.Px(170))))
}

func (a *App) memoryPanel(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	mem := a.s.Latest.Get().Mem
	hist := lastSeconds(a.s.History.Get(), a.span.Get())
	bytes := func(v float64) string { return ui.FormatBytes(uint64(max(v, 0))) }
	return scroll(panelBody(t, a.spanToggle(),
		reading(t, "In use", ui.FormatBytes(mem.Used())+" ("+percent(mem.Percent())+")"),
		reading(t, "Available", ui.FormatBytes(mem.Available)),
		reading(t, "Installed", ui.FormatBytes(mem.Total)),
		reading(t, "Committed", ui.FormatBytes(mem.CommitTotal)+" of "+ui.FormatBytes(mem.CommitLimit)),
	).
		Child(charts.Area(charts.Series[store.Point]{Name: "Memory in use", Data: hist, Time: pointTime, Y: pointMemUsed}).
			Key("memory-history").YAxis(charts.Axis{Format: bytes}.Min(0).Max(float64(mem.Total))).
			Legend(charts.LegendNone).Format(bytes).Height(ui.Auto).Grow(1).MinHeight(ui.Px(140))))
}

func (a *App) networkPanel(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	snap := a.s.Latest.Get()
	hist := lastSeconds(a.s.History.Get(), a.span.Get())
	return scroll(panelBody(t, a.spanToggle(),
		reading(t, "Received", rate(snap.NetIn)),
		reading(t, "Sent", rate(snap.NetOut)),
	).
		Child(charts.Line(
			charts.Series[store.Point]{Name: "Received", Data: hist, Time: pointTime, Y: pointNetIn},
			charts.Series[store.Point]{Name: "Sent", Data: hist, Time: pointTime, Y: pointNetOut},
		).Key("network-history").YAxis(charts.Axis{Format: rate}.Min(0)).Format(rate).Height(ui.Auto).Grow(1).MinHeight(ui.Px(160))))
}

func (a *App) ioPanel(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	snap := a.s.Latest.Get()
	hist := lastSeconds(a.s.History.Get(), a.span.Get())
	return scroll(panelBody(t, a.spanToggle(),
		reading(t, "Read", rate(snap.IORead)),
		reading(t, "Written", rate(snap.IOWrite)),
	).
		Child(ui.Text("The reads and writes of every process, to files, devices and the network.").
			FontSize(12).TextColor(t.TextMuted)).
		Child(charts.Line(
			charts.Series[store.Point]{Name: "Read", Data: hist, Time: pointTime, Y: pointIORead},
			charts.Series[store.Point]{Name: "Written", Data: hist, Time: pointTime, Y: pointIOWrite},
		).Key("io-history").YAxis(charts.Axis{Format: rate}.Min(0)).Format(rate).Height(ui.Auto).Grow(1).MinHeight(ui.Px(160))))
}

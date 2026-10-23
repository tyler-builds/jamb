package views

import (
	"time"

	"github.com/tyler-builds/jamb/charts"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/store"
)

// overview is the row of tiles above the panels: the processor, the memory, the
// network and the processes' I/O, each with its value now and a sparkline of the
// last minute. A tile's button shows its panel.
func (a *App) overview(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	snap := a.s.Latest.Get()
	hist := lastSeconds(a.s.History.Get(), 60)
	mem := snap.Mem
	return ui.Div().Row().Gap(8).Padding(8).Key("overview").
		Child(a.tile(t, tileSpec{id: panelCPU, icon: icons.Cpu, label: "Processor",
			value: percent(snap.CPU), detail: count(len(snap.Cores)) + " logical processors",
			series: []charts.Series[store.Point]{{Name: "Processor", Data: hist, Time: pointTime, Y: pointCPU}},
			yMax:   100})).
		Child(a.tile(t, tileSpec{id: panelMemory, icon: icons.MemoryStick, label: "Memory",
			value:  percent(mem.Percent()),
			detail: ui.FormatBytes(mem.Used()) + " of " + ui.FormatBytes(mem.Total),
			series: []charts.Series[store.Point]{{Name: "Memory", Data: hist, Time: pointTime, Y: pointMemPercent}},
			yMax:   100})).
		Child(a.tile(t, tileSpec{id: panelNetwork, icon: icons.Network, label: "Network",
			value:  rate(snap.NetIn + snap.NetOut),
			detail: "In " + rate(snap.NetIn) + ", out " + rate(snap.NetOut),
			series: []charts.Series[store.Point]{
				{Name: "Received", Data: hist, Time: pointTime, Y: pointNetIn},
				{Name: "Sent", Data: hist, Time: pointTime, Y: pointNetOut}},
			format: rate})).
		Child(a.tile(t, tileSpec{id: panelIO, icon: icons.HardDrive, label: "Process I/O",
			value:  rate(snap.IORead + snap.IOWrite),
			detail: "Read " + rate(snap.IORead) + ", write " + rate(snap.IOWrite),
			series: []charts.Series[store.Point]{
				{Name: "Read", Data: hist, Time: pointTime, Y: pointIORead},
				{Name: "Written", Data: hist, Time: pointTime, Y: pointIOWrite}},
			format: rate}))
}

// tileSpec is what a tile of the overview shows.
type tileSpec struct {
	id, label     string
	icon          *icons.Icon
	value, detail string
	series        []charts.Series[store.Point]
	// yMax fixes the top of a percentage's sparkline; zero follows the data.
	yMax   float64
	format func(float64) string
}

func (a *App) tile(t *ui.Theme, s tileSpec) ui.Element {
	y := charts.Axis{Hidden: true}.Min(0)
	if s.yMax > 0 {
		y = y.Max(s.yMax)
	}
	spark := charts.Area(s.series...).Key("spark").
		XAxis(charts.Axis{Hidden: true}).YAxis(y).Legend(charts.LegendNone).Height(ui.Px(40))
	if s.format != nil {
		spark.Format(s.format)
	} else {
		spark.Format(percent)
	}
	id := s.id
	return ui.Div().Key("tile-"+s.id).Grow(1).Basis(ui.Px(0)).MinWidth(ui.Px(0)).Gap(2).
		PaddingX(12).PaddingY(10).Radius(8).Background(t.Surface).Border(1, t.Border).
		Child(ui.Div().Row().Align(ui.Center).Gap(6).
			Child(ui.Icon(s.icon).Size(14).Color(t.TextMuted)).
			Child(ui.Text(s.label).FontSize(12).TextColor(t.TextMuted).Truncate()).
			Child(ui.Spacer()).
			Child(ui.IconButton(icons.ArrowUpRight).Ghost().Small().Label("Show the " + s.label + " panel").
				OnClick(func() { a.showPanel(id) }))).
		Child(ui.Text(s.value).Key("value").FontSize(22).FontWeight(ui.FontSemibold)).
		Child(ui.Text(s.detail).FontSize(11.5).TextColor(t.TextSubtle).Truncate()).
		Child(ui.Div().PaddingTop(4).Child(spark))
}

// lastSeconds returns the points of the last n seconds of hist.
func lastSeconds(hist []store.Point, n int) []store.Point {
	if len(hist) == 0 {
		return nil
	}
	from := hist[len(hist)-1].Time.Add(-time.Duration(n) * time.Second)
	for i, p := range hist {
		if !p.Time.Before(from) {
			return hist[i:]
		}
	}
	return nil
}

// The readers of a point, for the series.
func pointTime(p store.Point) time.Time     { return p.Time }
func pointCPU(p store.Point) float64        { return p.CPU }
func pointMemPercent(p store.Point) float64 { return p.MemPercent }
func pointMemUsed(p store.Point) float64    { return float64(p.MemUsed) }
func pointNetIn(p store.Point) float64      { return p.NetIn }
func pointNetOut(p store.Point) float64     { return p.NetOut }
func pointIORead(p store.Point) float64     { return p.IORead }
func pointIOWrite(p store.Point) float64    { return p.IOWrite }

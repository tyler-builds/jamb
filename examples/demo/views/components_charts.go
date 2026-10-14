package views

import (
	"time"

	"github.com/tyler-builds/jamb/charts"
	"github.com/tyler-builds/jamb/ui"
)

// chartDay is a day of the sample visits: its name, its visits and its returning visitors.
type chartDay struct {
	name      string
	visits    float64
	returning float64
}

// chartWeek is the sample data of the charts: a week of visits, Monday to Sunday.
var chartWeek = []chartDay{
	{"Mon", 120, 40}, {"Tue", 180, 62}, {"Wed", 150, 51}, {"Thu", 240, 90},
	{"Fri", 210, 77}, {"Sat", 300, 118}, {"Sun", 280, 104},
}

// chartPrice is a day of sample prices, with the open, high, low and close of the day.
type chartPrice struct {
	day                    time.Time
	open, high, low, close float64
}

// chartPrices is a week of sample prices, in order, for the candlestick chart.
var chartPrices = []chartPrice{
	{time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), 40, 44, 38, 43},
	{time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), 43, 45, 41, 42},
	{time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC), 42, 42, 36, 37},
	{time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), 37, 40, 35, 39},
	{time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), 39, 47, 38, 46},
	{time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), 46, 49, 44, 45},
	{time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC), 45, 46, 41, 42},
}

// chartSources is the sample traffic by source, for the pie and donut charts.
var chartSources = []chartDay{
	{"Search", 420, 0}, {"Social", 260, 0}, {"Email", 150, 0}, {"Direct", 110, 0}, {"Referral", 60, 0},
}

// chartFlows is the sample traffic flow for the Sankey diagram: from where visitors arrive,
// to the pages they reach, and on to a signup or an exit.
var chartFlows = []charts.SankeyLink{
	{From: "Search", To: "Home", Value: 210}, {From: "Search", To: "Product", Value: 120},
	{From: "Social", To: "Home", Value: 150}, {From: "Social", To: "Blog", Value: 60},
	{From: "Email", To: "Product", Value: 90}, {From: "Direct", To: "Home", Value: 70},
	{From: "Home", To: "Signup", Value: 130}, {From: "Home", To: "Exit", Value: 300},
	{From: "Product", To: "Signup", Value: 140}, {From: "Product", To: "Exit", Value: 170},
	{From: "Blog", To: "Exit", Value: 60},
}

// chartsSection shows each chart type on sample data. A button turns the first chart into
// its table, for readers who need the exact values.
func chartsSection(ctx *ui.Context, t *ui.Theme) ui.Element {
	asTable := ui.Local(ctx, "charts.table", false)
	toggle := ui.Button("Show visits as a table").Ghost().AccessChecked(asTable.Get()).OnClick(func() {
		asTable.Set(!asTable.Peek())
	})
	var visits ui.Element
	if asTable.Get() {
		visits = sample(t, "Visits as a table, with the values of each series",
			ui.Div().Width(ui.Px(420)).Height(ui.Px(220)).Child(charts.DataTable(weekVisitsChart())))
	} else {
		visits = sample(t, "Line: visits and returning visitors by day, smoothed",
			ui.Div().Width(ui.Px(420)).Child(charts.Line(
				charts.Series[chartDay]{Name: "Visits", Data: chartWeek, Category: dayName, Y: visitsOf},
				charts.Series[chartDay]{Name: "Returning", Data: chartWeek, Category: dayName, Y: returningOf},
			).Title("Visits this week").Smooth().Height(ui.Px(220))))
	}
	return section(t, "Charts",
		sample(t, "Switch the first chart to its table", toggle),
		visits,
		sample(t, "Area: the two series stacked",
			ui.Div().Width(ui.Px(420)).Child(charts.Area(
				charts.Series[chartDay]{Name: "Visits", Data: chartWeek, Category: dayName, Y: visitsOf},
				charts.Series[chartDay]{Name: "Returning", Data: chartWeek, Category: dayName, Y: returningOf},
			).Stacked().Height(ui.Px(220)))),
		sample(t, "Bar: grouped, and stacked horizontally",
			ui.Div().Width(ui.Px(420)).Child(charts.Bar(
				charts.Series[chartDay]{Name: "Visits", Data: chartWeek, Category: dayName, Y: visitsOf},
				charts.Series[chartDay]{Name: "Returning", Data: chartWeek, Category: dayName, Y: returningOf},
			).Stacked().Horizontal().Height(ui.Px(220)))),
		sample(t, "Scatter: visits against returning visitors",
			ui.Div().Width(ui.Px(420)).Child(charts.Scatter(
				charts.Series[chartDay]{Name: "Days", Data: chartWeek,
					X: visitsOf, Y: returningOf},
			).XAxis(charts.Axis{Label: "Visits"}).YAxis(charts.Axis{Label: "Returning"}).Height(ui.Px(220)))),
		sample(t, "Pie and donut: visits by source",
			ui.Div().Row().Gap(16).Child(ui.Div().Width(ui.Px(200)).Child(charts.Pie(
				charts.Series[chartDay]{Name: "Sources", Data: chartSources, Category: dayName, Y: visitsOf},
			).Legend(charts.LegendNone).Height(ui.Px(200)))).Child(ui.Div().Width(ui.Px(200)).Child(charts.Donut(
				charts.Series[chartDay]{Name: "Sources", Data: chartSources, Category: dayName, Y: visitsOf},
			).Height(ui.Px(200))))),
		sample(t, "Radar: the week across its days",
			ui.Div().Width(ui.Px(300)).Child(charts.Radar(
				charts.Series[chartDay]{Name: "Visits", Data: chartWeek, Category: dayName, Y: visitsOf},
				charts.Series[chartDay]{Name: "Returning", Data: chartWeek, Category: dayName, Y: returningOf},
			).Height(ui.Px(260)))),
		sample(t, "Candlestick: prices of a week",
			ui.Div().Width(ui.Px(420)).Child(charts.Candlestick(charts.CandleSeries[chartPrice]{
				Name: "Price", Data: chartPrices, Time: func(p chartPrice) time.Time { return p.day },
				Open: func(p chartPrice) float64 { return p.open }, High: func(p chartPrice) float64 { return p.high },
				Low: func(p chartPrice) float64 { return p.low }, Close: func(p chartPrice) float64 { return p.close },
			}).Title("Price, 5 to 11 October").Height(ui.Px(220)))),
		sample(t, "Sankey: visitors from source to page to outcome",
			ui.Div().Width(ui.Px(520)).Child(charts.Sankey(chartFlows...).Height(ui.Px(260)))),
	)
}

// weekVisitsChart returns the visits of the sample week as a chart, for its table.
func weekVisitsChart() *charts.Chart {
	return charts.Line(charts.Series[chartDay]{Name: "Visits", Data: chartWeek, Category: dayName, Y: visitsOf},
		charts.Series[chartDay]{Name: "Returning", Data: chartWeek, Category: dayName, Y: returningOf})
}

// dayName, visitsOf and returningOf read the fields of a sample day for the series.
func dayName(d chartDay) string      { return d.name }
func visitsOf(d chartDay) float64    { return d.visits }
func returningOf(d chartDay) float64 { return d.returning }

package charts_test

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/charts"
	"github.com/tyler-builds/jamb/ui"
)

// Visit is one day's count, the data of the examples.
type Visit struct {
	Day   time.Time
	Count int
}

var visits = []Visit{
	{Day: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), Count: 120},
	{Day: time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), Count: 180},
}

func formatCount(v float64) string { return fmt.Sprintf("%.0f", v) }

// The chart of the package documentation, placed in a tree.
func Example() {
	view := ui.Div().Child(charts.Line(charts.Series[Visit]{
		Name: "Visits",
		Data: visits,
		Time: func(v Visit) time.Time { return v.Day },
		Y:    func(v Visit) float64 { return float64(v.Count) },
	}).Title("Visits this week").
		YAxis(charts.Axis{Label: "Visits"}.Min(0).Max(400)).
		Size(ui.Full, ui.Px(300)))
	_ = view
}

// A value axis fixed from 0 to 400, with a tick format.
func ExampleAxis() {
	chart := charts.Line(charts.Series[Visit]{
		Name: "Visits",
		Data: visits,
		Time: func(v Visit) time.Time { return v.Day },
		Y:    func(v Visit) float64 { return float64(v.Count) },
	})
	chart.YAxis(charts.Axis{Label: "Visits", Format: formatCount}.Min(0).Max(400))
}

// The data table of a chart, placed beside it.
func ExampleDataTable() {
	chart := charts.Line(charts.Series[Visit]{
		Name: "Visits",
		Data: visits,
		Time: func(v Visit) time.Time { return v.Day },
		Y:    func(v Visit) float64 { return float64(v.Count) },
	})
	view := ui.Div().Column().Child(chart).Child(charts.DataTable(chart))
	_ = view
}

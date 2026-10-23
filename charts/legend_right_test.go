package charts

import (
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// TestRightLegendStaysInsideTheChart checks that a legend beside a chart, with more
// items than fit its height, is laid out inside the chart's box, and that the plot
// takes the width the legend leaves.
func TestRightLegendStaysInsideTheChart(t *testing.T) {
	type slice struct {
		name string
		v    float64
	}
	var data []slice
	for i := range 14 {
		data = append(data, slice{"Category " + strconv.Itoa(i+1), float64(20 - i)})
	}
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Column().Child(ui.Div().Width(ui.Px(460)).Child(Donut(Series[slice]{Name: "Spending", Data: data,
			Category: func(s slice) string { return s.name }, Y: func(s slice) float64 { return s.v }}).
			Key("donut").Legend(LegendRight).Height(ui.Px(220))))
	}, uitest.Size(800, 600))
	box := u.Get("donut").Bounds()
	if box.W > 460.5 || box.H > 220.5 {
		t.Fatalf("the chart is %v, bigger than the 460 by 220 it was given", box)
	}
	u.Tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleButton && n.Name == "Category 1" {
			b := n.Bounds
			if b.X < box.X || b.X+b.W > box.X+box.W+0.5 || b.Y < box.Y || b.Y+b.H > box.Y+box.H+0.5 {
				t.Errorf("legend item %v is outside the chart %v", b, box)
			}
		}
		return true
	})
}

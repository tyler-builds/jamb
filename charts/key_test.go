package charts

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// TestChartKeyFindsTheChart checks that a chart's Key finds it in tests, while its
// enter animation runs and after it ends.
func TestChartKeyFindsTheChart(t *testing.T) {
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Column().Child(Line(visits()).Key("visits-chart").Height(ui.Px(120)))
	})
	for _, when := range []string{"first frame", "during the enter animation", "after it"} {
		switch when {
		case "during the enter animation":
			u.Frame()
		case "after it":
			u.Settle()
		}
		if !u.Has("visits-chart") {
			t.Errorf("the chart's key finds nothing at the %s", when)
		}
	}
}

package charts

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// TestTabStopFalseLeavesTheChartOutOfTheTabOrder checks that Tab goes past a chart
// with TabStop(false), that a chart is a Tab stop by default, and that the chart stays
// in the accessibility tree either way.
func TestTabStopFalseLeavesTheChartOutOfTheTabOrder(t *testing.T) {
	for _, stop := range []bool{true, false} {
		u := uitest.New(t, func(*ui.Context) ui.Element {
			return ui.Div().Column().
				Child(ui.Button("Before").Key("before")).
				Child(Line(visits()).Label("Weekly visits").TabStop(stop).Height(ui.Px(60))).
				Child(ui.Button("After").Key("after"))
		})
		u.Get("before").Focus()
		u.Press("tab")
		want := "Weekly visits"
		if !stop {
			want = "After"
		}
		if got := u.Focused().Name(); got != want {
			t.Errorf("TabStop(%t): Tab from Before focused %q, want %q", stop, got, want)
		}
		u.ByRole(access.RoleImage, "Weekly visits")
	}
}

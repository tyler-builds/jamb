package charts

import (
	"math"
	"reflect"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// sparkline returns a line of the visits and returns with both axes hidden and no legend,
// named so that the tests find its focus stop, and at the given height.
func sparkline(c **Chart, h float32) func(*ui.Context) ui.Element {
	return func(*ui.Context) ui.Element {
		ch := Line(visits(), returns()).XAxis(Axis{Hidden: true}).YAxis(Axis{Hidden: true}).
			Legend(LegendNone).Label("Weekly visits").Height(ui.Px(h))
		if c != nil {
			*c = ch
		}
		return ui.Div().Child(ch)
	}
}

// near reports whether a and b differ by less than half a pixel.
func near(a, b float64) bool { return math.Abs(a-b) < 0.5 }

// A chart with both axes hidden, no legend and no title plots over the whole canvas,
// inset only by the keyboard ring that the edge points need.
func TestHiddenAxesFillTheCanvas(t *testing.T) {
	var c *Chart
	newRig(sparkline(&c, 48))
	g := c.last
	if !g.drawn {
		t.Fatal("the sparkline was not drawn")
	}
	want := rect{x0: plotInset, y0: plotInset, x1: g.w - plotInset, y1: g.h - plotInset}
	if !near(g.plot.x0, want.x0) || !near(g.plot.y0, want.y0) || !near(g.plot.x1, want.x1) || !near(g.plot.y1, want.y1) {
		t.Errorf("the plot is %+v in a %v by %v canvas, want %+v", g.plot, g.w, g.h, want)
	}
}

// A hidden X axis drops the margin under the plot, and keeps the Y axis's margin at the left.
func TestHiddenXKeepsYMargin(t *testing.T) {
	var shown, hiddenX *Chart
	newRig(func(*ui.Context) ui.Element {
		shown = Line(visits()).Height(ui.Px(200))
		return ui.Div().Child(shown)
	})
	newRig(func(*ui.Context) ui.Element {
		hiddenX = Line(visits()).XAxis(Axis{Hidden: true}).Height(ui.Px(200))
		return ui.Div().Child(hiddenX)
	})
	if hiddenX.last.plot.x0 != shown.last.plot.x0 {
		t.Errorf("the value margin at the left is %v, want the visible axis's %v", hiddenX.last.plot.x0, shown.last.plot.x0)
	}
	if hiddenX.last.plot.x0 <= plotInset {
		t.Errorf("the value margin at the left is %v, which is no more than the inset", hiddenX.last.plot.x0)
	}
	if !near(hiddenX.last.plot.y1, hiddenX.last.h-plotInset) {
		t.Errorf("the plot ends at %v of a %v canvas, want the inset of %v", hiddenX.last.plot.y1, hiddenX.last.h, plotInset)
	}
	if shown.last.plot.y1 >= shown.last.h-plotInset {
		t.Errorf("the visible X axis reserves no margin below the plot")
	}
}

// A hidden Y axis drops the value margin at the left, and keeps the X axis's margin below.
// The first category's label still hangs past the left edge, so the plot starts at its
// half width, which is less than the value margin of a visible Y axis.
func TestHiddenYKeepsXMargin(t *testing.T) {
	var shown, hiddenY *Chart
	newRig(func(*ui.Context) ui.Element {
		shown = Line(visits()).Height(ui.Px(200))
		return ui.Div().Child(shown)
	})
	newRig(func(*ui.Context) ui.Element {
		hiddenY = Line(visits()).YAxis(Axis{Hidden: true}).Height(ui.Px(200))
		return ui.Div().Child(hiddenY)
	})
	if hiddenY.last.plot.x0 >= shown.last.plot.x0 {
		t.Errorf("the plot starts at %v, which is no less than the visible Y axis's %v", hiddenY.last.plot.x0, shown.last.plot.x0)
	}
	if hiddenY.last.plot.y1 >= hiddenY.last.h-plotInset {
		t.Errorf("the visible X axis reserves no margin below the plot")
	}
}

// GridAuto draws no grid on a hidden axis, and GridOn draws it whether or not the axis is hidden.
func TestHiddenAxisGrid(t *testing.T) {
	cases := []struct {
		name string
		a    Axis
		want bool
	}{
		{"auto, shown", Axis{}, true},
		{"auto, hidden", Axis{Hidden: true}, false},
		{"on, shown", Axis{Grid: GridOn}, true},
		{"on, hidden", Axis{Grid: GridOn, Hidden: true}, true},
		{"off, shown", Axis{Grid: GridOff}, false},
		{"off, hidden", Axis{Grid: GridOff, Hidden: true}, false},
	}
	for _, tc := range cases {
		c := Line(visits()).YAxis(tc.a)
		if got := c.valueGrid(); got != tc.want {
			t.Errorf("%s: the value grid is %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A chart 36 pixels high, with both axes hidden, lays out and draws, with a plot of positive size.
func TestThirtySixPixelSparklineDraws(t *testing.T) {
	var c *Chart
	newRig(sparkline(&c, 36))
	if !c.last.drawn {
		t.Fatal("the 36 pixel sparkline was not drawn")
	}
	if c.last.h != 36 {
		t.Errorf("the drawing is %v pixels high in a 36 pixel chart", c.last.h)
	}
	if c.last.plot.y1 <= c.last.plot.y0 || c.last.plot.x1 <= c.last.plot.x0 {
		t.Errorf("the plot of a 36 pixel sparkline is %+v", c.last.plot)
	}
}

// A sparkline with a fixed range, smooth, and an area fills and draws its marks, and the
// tooltip and the walk still read its points.
func TestSparklineVariantsDraw(t *testing.T) {
	var c *Chart
	newRig(func(*ui.Context) ui.Element {
		c = Area(visits()).XAxis(Axis{Hidden: true}).YAxis(Axis{Hidden: true}.Min(0).Max(100)).
			Legend(LegendNone).Smooth().Height(ui.Px(40))
		return ui.Div().Child(c)
	})
	if !c.last.drawn {
		t.Fatal("the area sparkline was not drawn")
	}
	if !near(c.last.val.at(100), c.last.plot.y0) || !near(c.last.val.at(0), c.last.plot.y1) {
		t.Errorf("the range 0 to 100 maps to %v and %v, want the plot's top and bottom %v and %v",
			c.last.val.at(100), c.last.val.at(0), c.last.plot.y0, c.last.plot.y1)
	}
}

// The keyboard walks a sparkline and announces its points, and its summary names it.
func TestSparklineKeyboardAndSummary(t *testing.T) {
	var c *Chart
	r := newRig(sparkline(&c, 48))
	if got, want := c.summary(en), "Line chart, 2 series, 7 points each, from Monday to Sunday; Visits from 120 to 300, Returns from 10 to 40"; got != want {
		t.Errorf("summary %q, want %q", got, want)
	}
	r.press(input.KeyTab)
	accesstest.Focused(t, r.rt.AccessTree(), access.RoleImage, "Weekly visits")
	r.press(input.KeyRight)
	r.press(input.KeyDown)
	want := []string{"Tuesday, Visits 180", "Tuesday, Returns 20"}
	if got := r.takeAnns(); !reflect.DeepEqual(got, want) {
		t.Errorf("announcements %q, want %q", got, want)
	}
}

// Hidden axes do nothing to a pie, whose plot has no axes to hide.
func TestHiddenAxesDoNothingToPie(t *testing.T) {
	var c *Chart
	newRig(func(*ui.Context) ui.Element {
		c = Pie(visits()).XAxis(Axis{Hidden: true}).YAxis(Axis{Hidden: true}).Height(ui.Px(200))
		return ui.Div().Child(c)
	})
	if !c.last.drawn || c.last.rad <= 0 {
		t.Errorf("the pie drew with radius %v, drawn %v", c.last.rad, c.last.drawn)
	}
}

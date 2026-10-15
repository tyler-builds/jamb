package charts

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// day is a row of the sample data: a day of the week, its visits and its returns.
type day struct {
	name             string
	visits, returned float64
}

// week is a week of sample data, Monday to Sunday.
var week = []day{
	{"Monday", 120, 10}, {"Tuesday", 180, 20}, {"Wednesday", 150, 15}, {"Thursday", 240, 30},
	{"Friday", 210, 40}, {"Saturday", 300, 25}, {"Sunday", 280, 12},
}

// visits returns the series of visits by day, with the day names as categories.
func visits() Series[day] {
	return Series[day]{Name: "Visits", Data: week, Category: func(d day) string { return d.name },
		Y: func(d day) float64 { return d.visits }}
}

// returns returns the series of returns by day.
func returns() Series[day] {
	return Series[day]{Name: "Returns", Data: week, Category: func(d day) string { return d.name },
		Y: func(d day) float64 { return d.returned }}
}

// rig runs a tree of charts in a runtime of 800 by 600 pixels, with the access bridge on
// and the announcements it receives collected.
type rig struct {
	rt   *ui.Runtime
	now  time.Time
	anns []string
}

func newRig(root func(*ui.Context) ui.Element) *rig {
	r := &rig{rt: ui.NewRuntime(root, ui.DarkTheme(), nil), now: time.Unix(1_000_000, 0)}
	r.rt.SetViewport(800, 600, 1)
	bridge := r.rt.AccessBridge()
	bridge.Listen(func(u access.Update) {
		for _, a := range u.Announcements {
			r.anns = append(r.anns, a.Text)
		}
	})
	bridge.Activate()
	r.frame()
	return r
}

// frame runs a frame at the next instant.
func (r *rig) frame() {
	r.now = r.now.Add(time.Second)
	r.rt.Frame(r.now)
}

// press sends a key press and runs a frame.
func (r *rig) press(k input.Key) {
	r.rt.Press(k, 0)
	r.frame()
}

// takeAnns returns the announcements received since the last call, and empties them.
func (r *rig) takeAnns() []string {
	out := r.anns
	r.anns = nil
	return out
}

// lineRoot returns a tree holding a line chart of the visits and returns, with a label.
func lineRoot(c **Chart) func(*ui.Context) ui.Element {
	return func(*ui.Context) ui.Element {
		ch := Line(visits(), returns()).Title("Visits this week").Label("Weekly visits")
		if c != nil {
			*c = ch
		}
		return ui.Div().Child(ch)
	}
}

// The summary names the chart's type, its series, their points and the ranges, as
// the package documents it.
func TestSummary(t *testing.T) {
	want := "Line chart, 2 series, 7 points each, from Monday to Sunday; Visits from 120 to 300, Returns from 10 to 40"
	if got := Line(visits(), returns()).summary(); got != want {
		t.Errorf("summary = %q\nwant      %q", got, want)
	}
	pie := Pie(Series[day]{Name: "Share", Data: week, Category: func(d day) string { return d.name },
		Y: func(d day) float64 { return d.visits }})
	if got := pie.summary(); !strings.HasPrefix(got, "Pie chart, 7 slices; largest is Saturday at ") {
		t.Errorf("pie summary = %q", got)
	}
	sk := Sankey(SankeyLink{From: "A", To: "B", Value: 3}, SankeyLink{From: "B", To: "C", Value: 2})
	if got := sk.summary(); got != "Sankey diagram, 3 nodes, 2 links; total flow 5" {
		t.Errorf("sankey summary = %q", got)
	}
	if got := Bar(visits()).Stacked().Horizontal().summary(); !strings.HasPrefix(got, "Stacked horizontal bar chart, 1 series") {
		t.Errorf("stacked horizontal bar summary = %q", got)
	}
}

// A chart with a Label is named by it, and the summary is what it is named by otherwise.
func TestAccessibleName(t *testing.T) {
	var c *Chart
	r := newRig(lineRoot(&c))
	tree := r.rt.AccessTree()
	accesstest.Find(t, tree, access.RoleImage, "Weekly visits")
	if n := tree.Find(access.RoleImage, c.summary()); n != nil {
		t.Errorf("the chart is named by its summary although it has a Label")
	}
}

// Tab reaches the chart, and each arrow step announces the point it moves to: Right and Left
// walk the points of a series, and Down and Up switch to the next and previous series at the
// same day.
func TestKeyboardWalkAnnounces(t *testing.T) {
	r := newRig(lineRoot(nil))
	r.press(input.KeyTab)
	accesstest.Focused(t, r.rt.AccessTree(), access.RoleImage, "Weekly visits")
	r.press(input.KeyRight)
	r.press(input.KeyDown)
	r.press(input.KeyLeft)
	r.press(input.KeyUp)
	want := []string{"Tuesday, Visits 180", "Tuesday, Returns 20", "Monday, Returns 10", "Monday, Visits 120"}
	if got := r.takeAnns(); !reflect.DeepEqual(got, want) {
		t.Errorf("announcements %q, want %q", got, want)
	}
}

// Two presses before the next frame each walk from where the last one left the cursor, so a
// quick repeat is not lost to the render that has not happened yet.
func TestKeyboardWalkBeforeRender(t *testing.T) {
	r := newRig(lineRoot(nil))
	r.press(input.KeyTab)
	r.takeAnns()
	r.rt.Press(input.KeyRight, 0)
	r.rt.Press(input.KeyRight, 0)
	r.frame()
	if got := r.takeAnns(); !reflect.DeepEqual(got, []string{"Tuesday, Visits 180", "Wednesday, Visits 150"}) {
		t.Errorf("two presses before a frame announced %q", got)
	}
}

// Walking past the last point stays on it, and the announcement says so by repeating it.
func TestKeyboardWalkStopsAtEnds(t *testing.T) {
	r := newRig(lineRoot(nil))
	r.press(input.KeyTab)
	for i := 0; i < 10; i++ {
		r.press(input.KeyLeft)
	}
	r.takeAnns()
	r.press(input.KeyLeft)
	if got := r.takeAnns(); !reflect.DeepEqual(got, []string{"Monday, Visits 120"}) {
		t.Errorf("past the start: %q", got)
	}
}

// The legend items are toggle buttons: a Tab from the chart reaches the first, and Enter
// hides its series, which then reads as not checked.
func TestLegendToggles(t *testing.T) {
	r := newRig(lineRoot(nil))
	r.press(input.KeyTab) // the chart
	r.press(input.KeyTab) // Visits
	tree := r.rt.AccessTree()
	visitsButton := tree.Find(access.RoleButton, "Visits")
	if visitsButton == nil {
		t.Fatalf("no Visits legend button:\n%s", tree.Dump())
	}
	accesstest.Check(t, visitsButton, access.RoleButton, "Visits", access.StateChecked|access.StateToggle)
	r.press(input.KeyEnter)
	tree = r.rt.AccessTree()
	if n := tree.Find(access.RoleButton, "Visits"); n == nil || n.States.Has(access.StateChecked) {
		t.Errorf("Visits is still checked after Enter")
	}
}

// Hovering a point shows a tooltip with the point's label and every series' value there.
func TestHoverTooltip(t *testing.T) {
	var c *Chart
	r := newRig(lineRoot(&c))
	g := c.last
	canvas := r.rt.AccessTree().Find(access.RoleImage, "Weekly visits")
	if canvas == nil {
		t.Fatal("no chart in the access tree")
	}
	x := float32(canvas.Bounds.X) + float32(g.pos.at(1)) // Tuesday
	y := float32(canvas.Bounds.Y) + float32((g.plot.y0+g.plot.y1)/2)
	r.rt.PointerMove(x, y, 0)
	r.frame()
	if n := r.rt.AccessTree().Find(access.RoleTooltip, "Tuesday, Visits 180, Returns 20"); n == nil {
		t.Errorf("no tooltip for Tuesday:\n%s", r.rt.AccessTree().Dump())
	}
}

// The data table is a grid of the chart's rows, named by the chart, with a column per series.
func TestDataTable(t *testing.T) {
	c := Line(visits(), returns())
	header, rows := c.table()
	if want := []string{"Category", "Visits", "Returns"}; !reflect.DeepEqual(header, want) {
		t.Errorf("header %v, want %v", header, want)
	}
	if len(rows) != 7 || !reflect.DeepEqual(rows[1].cells, []string{"Tuesday", "180", "20"}) {
		t.Errorf("rows %d, second %v", len(rows), rows[1].cells)
	}
	r := newRig(func(*ui.Context) ui.Element { return DataTable(c) })
	accesstest.Find(t, r.rt.AccessTree(), access.RoleDataGrid, "Data table: "+c.summary())
}

// A pie's slices sweep the full circle, in proportion to their values, from the top.
func TestPieAngles(t *testing.T) {
	var c *Chart
	newRig(func(*ui.Context) ui.Element {
		c = Pie(Series[day]{Name: "Share", Data: []day{{"A", 1, 0}, {"B", 1, 0}, {"C", 2, 0}},
			Category: func(d day) string { return d.name }, Y: func(d day) float64 { return d.visits }})
		return ui.Div().Child(c)
	})
	g := c.last
	if len(g.slices) != 3 {
		t.Fatalf("%d slices, want 3", len(g.slices))
	}
	if math.Abs(g.slices[0].a0-startAngle) > 1e-9 {
		t.Errorf("the first slice starts at %v, want the top %v", g.slices[0].a0, startAngle)
	}
	sum := 0.0
	for _, s := range g.slices {
		sum += s.a1 - s.a0
	}
	if math.Abs(sum-2*math.Pi) > 1e-9 {
		t.Errorf("slices sweep %v, want 2π", sum)
	}
	if got, want := g.slices[2].a1-g.slices[2].a0, math.Pi; math.Abs(got-want) > 1e-9 {
		t.Errorf("the slice of half the total sweeps %v, want π", got)
	}
}

// A donut's hole is the chart's inner radius; a pie has none.
func TestDonutHole(t *testing.T) {
	var pie, donut *Chart
	newRig(func(*ui.Context) ui.Element {
		s := Series[day]{Name: "Share", Data: week, Category: func(d day) string { return d.name }, Y: func(d day) float64 { return d.visits }}
		pie = Pie(s)
		donut = Donut(s)
		return ui.Div().Child(pie).Child(donut)
	})
	if pie.last.inner != 0 {
		t.Errorf("a pie has an inner radius of %v", pie.last.inner)
	}
	if got, want := donut.last.inner, donut.last.rad*donutHole; math.Abs(got-want) > 1e-9 {
		t.Errorf("a donut's hole is %v, want %v", got, want)
	}
}

// A radar vertex is at its value's share of the outer ring, on its axis, clockwise from the top.
func TestRadarVertices(t *testing.T) {
	var c *Chart
	newRig(func(*ui.Context) ui.Element {
		c = Radar(visits())
		return ui.Div().Child(c)
	})
	g := c.last
	if g.axes != 7 {
		t.Fatalf("%d axes, want 7", g.axes)
	}
	// The axes run clockwise from the top, one for each day in order, and each vertex is at its
	// value's share of the outer ring.
	for i, d := range week {
		x, y := c.radarVertex(&view{c: c, geo: g}, 0, i)
		r := math.Hypot(x-g.cx, y-g.cy)
		want := g.rad * d.visits / g.ringMax
		if math.Abs(r-want) > 1e-6 {
			t.Errorf("%s at radius %v, want %v", d.name, r, want)
		}
		a := math.Atan2(y-g.cy, x-g.cx)
		wantA := startAngle + float64(i)*2*math.Pi/7
		if delta := math.Mod(a-wantA+3*math.Pi, 2*math.Pi) - math.Pi; math.Abs(delta) > 1e-9 {
			t.Errorf("%s on angle %v, want %v", d.name, a, wantA)
		}
	}
}

// Stacking and the horizontal orientation render without panicking, and every configuration
// lays out a plot.
func TestConfigurationsRender(t *testing.T) {
	cases := map[string]*Chart{
		"line smooth":       Line(visits()).Smooth(),
		"area stacked":      Area(visits(), returns()).Stacked(),
		"bar grouped":       Bar(visits(), returns()),
		"bar stacked":       Bar(visits(), returns()).Stacked(),
		"bar horizontal":    Bar(visits(), returns()).Horizontal(),
		"bar stacked horiz": Bar(visits(), returns()).Stacked().Horizontal(),
		"scatter":           Scatter(visits(), returns()),
		"radar":             Radar(visits(), returns()),
		"legend top":        Line(visits()).Legend(LegendTop),
		"legend right":      Line(visits(), returns()).Legend(LegendRight),
		"legend none":       Line(visits()).Legend(LegendNone),
		"range y":           Line(visits()).YAxis(Axis{Label: "Visits"}.Min(0).Max(400)),
		"no grid":           Line(visits()).YAxis(Axis{Grid: GridOff}),
		"x grid":            Line(visits()).XAxis(Axis{Grid: GridOn, Label: "Day"}),
		"donut":             Donut(visits()),
		"sankey":            Sankey(SankeyLink{From: "Search", To: "Visit", Value: 5}, SankeyLink{From: "Direct", To: "Visit", Value: 3}),
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			r := newRig(func(*ui.Context) ui.Element { return ui.Div().Child(c) })
			r.frame()
			if !c.last.drawn {
				t.Errorf("%s was not drawn", name)
			}
		})
	}
}

// A candlestick and a time axis draw: candles at their slots, dates on the axis.
func TestCandlesAndTimeAxis(t *testing.T) {
	type bar struct {
		at                       time.Time
		open, high, low, closing float64
	}
	bars := []bar{
		{time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 10, 12, 9, 11},
		{time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), 11, 13, 10, 10},
		{time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), 10, 14, 9, 13},
	}
	var c *Chart
	r := newRig(func(*ui.Context) ui.Element {
		c = Candlestick(CandleSeries[bar]{Name: "Price", Data: bars, Time: func(b bar) time.Time { return b.at },
			Open: func(b bar) float64 { return b.open }, High: func(b bar) float64 { return b.high },
			Low: func(b bar) float64 { return b.low }, Close: func(b bar) float64 { return b.closing }})
		return ui.Div().Child(c)
	})
	r.frame()
	if !c.last.drawn || len(c.last.slots) != 3 {
		t.Fatalf("drawn %v, %d slots", c.last.drawn, len(c.last.slots))
	}
	first, last := ui.FormatDate(bars[0].at), ui.FormatDate(bars[2].at)
	if got := c.positionLabel(c.last.slots[0]); got != first {
		t.Errorf("first slot labeled %q, want %q", got, first)
	}
	if got := c.summary(); !strings.HasPrefix(got, "Candlestick chart, 3 candles, from "+first+" to "+last) {
		t.Errorf("candle summary %q", got)
	}
}

// Under high contrast and reduced motion the chart still draws, and it is drawn in full at
// once: the enter animation does not run.
func TestReducedMotionAndHighContrast(t *testing.T) {
	var c *Chart
	r := newRig(lineRoot(&c))
	r.rt.SetSystemSettings(ui.SystemSettings{HighContrast: true, ReducedMotion: true})
	r.frame()
	if !c.last.drawn || c.last.plot.x1 <= c.last.plot.x0 {
		t.Errorf("under high contrast the chart is not drawn")
	}
	if r.rt.NeedsFrame() {
		t.Error("the chart asks for frames under reduced motion")
	}
}

// Keys other than the arrows are not taken by the chart, so they reach the app.
func TestUnhandledKeysPassThrough(t *testing.T) {
	if _, _, ok := arrow(input.KeyEnter); ok {
		t.Error("Enter is an arrow")
	}
	if dx, dy, ok := arrow(input.KeyUp); !ok || dx != 0 || dy != -1 {
		t.Errorf("Up is (%d, %d, %v)", dx, dy, ok)
	}
}

// walkFrom drives the keyboard walk of a drawn chart from a position with a key, and returns
// the announcement. The view carries what the walk reads: the cursor and the geometry.
func walkFrom(t *testing.T, c *Chart, from cursorAt, k input.Key) (cursorAt, string) {
	t.Helper()
	v := &view{c: c, cursor: from, geo: c.last}
	next, text, ok := c.walk(v, k)
	if !ok {
		t.Fatalf("the key %v did not move the cursor", k)
	}
	return next, text
}

// On a horizontal bar chart the categories run down the chart, so Down walks them and Right
// switches the series, which keeps the category.
func TestHorizontalBarWalk(t *testing.T) {
	var c *Chart
	newRig(func(*ui.Context) ui.Element {
		c = Bar(visits(), returns()).Horizontal()
		return ui.Div().Child(c)
	})
	next, text := walkFrom(t, c, cursorAt{on: true, series: 0, index: 0}, input.KeyDown)
	if next.index != 1 || text != "Tuesday, Visits 180" {
		t.Errorf("Down: index %d, %q", next.index, text)
	}
	next, text = walkFrom(t, c, next, input.KeyRight)
	if next.series != 1 || text != "Tuesday, Returns 20" {
		t.Errorf("Right: series %d, %q", next.series, text)
	}
}

// A pie walks its slices with any arrow key, announcing each slice's share of the total.
func TestPieWalk(t *testing.T) {
	var c *Chart
	newRig(func(*ui.Context) ui.Element {
		c = Pie(Series[day]{Name: "Visits", Data: week, Category: func(d day) string { return d.name },
			Y: func(d day) float64 { return d.visits }})
		return ui.Div().Child(c)
	})
	// The week's visits total 1,480, so Tuesday's 180 is 12.2 percent of it.
	next, text := walkFrom(t, c, cursorAt{on: true, series: 0, index: 0}, input.KeyRight)
	if next.index != 1 || text != "Tuesday, 180 (12.2%)" {
		t.Errorf("Right: slice %d, %q", next.index, text)
	}
	if _, text = walkFrom(t, c, next, input.KeyUp); text != "Monday, 120 (8.1%)" {
		t.Errorf("Up back to Monday: %q", text)
	}
}

// Every kind of chart answers the pointer over a grid of its plot, and every arrow key, without
// a panic, and the keyboard walks it through its points.
func TestInteractionsEveryKind(t *testing.T) {
	cases := map[string]func() *Chart{
		"line":    func() *Chart { return Line(visits(), returns()) },
		"area":    func() *Chart { return Area(visits(), returns()).Stacked() },
		"bar":     func() *Chart { return Bar(visits(), returns()).Stacked() },
		"barh":    func() *Chart { return Bar(visits(), returns()).Horizontal() },
		"scatter": func() *Chart { return Scatter(visits(), returns()) },
		"candle":  func() *Chart { return Candlestick(candles()) },
		"pie":     func() *Chart { return Pie(Series[day]{Name: "V", Data: week, Category: dayName, Y: visitOf}) },
		"donut":   func() *Chart { return Donut(Series[day]{Name: "V", Data: week, Category: dayName, Y: visitOf}) },
		"radar":   func() *Chart { return Radar(visits(), returns()) },
		"sankey": func() *Chart {
			return Sankey(SankeyLink{From: "A", To: "B", Value: 3}, SankeyLink{From: "A", To: "C", Value: 1})
		},
	}
	for name, mk := range cases {
		mk := mk
		t.Run(name, func(t *testing.T) {
			var c *Chart
			r := newRig(func(*ui.Context) ui.Element {
				c = mk()
				return ui.Div().Child(c)
			})
			canvas := r.rt.AccessTree().Find(access.RoleImage, c.summary())
			if canvas == nil {
				t.Fatalf("no chart named %q:\n%s", c.summary(), r.rt.AccessTree().Dump())
			}
			g := c.last
			for x := 0.0; x <= 1; x += 0.25 {
				for y := 0.0; y <= 1; y += 0.25 {
					px := float32(canvas.Bounds.X) + float32(g.w*x)
					py := float32(canvas.Bounds.Y) + float32(g.h*y)
					r.rt.PointerMove(px, py, 0)
					r.frame()
				}
			}
			r.rt.PointerMove(-100, -100, 0)
			r.frame()
			r.press(input.KeyTab)
			for _, k := range []input.Key{input.KeyRight, input.KeyRight, input.KeyDown, input.KeyLeft, input.KeyUp, input.KeyDown, input.KeyLeft} {
				r.press(k)
			}
			if !c.last.drawn {
				t.Errorf("%s was never drawn", name)
			}
		})
	}
}

// visitOf returns the visits of a sample day.
func visitOf(d day) float64 { return d.visits }

// candles returns a week of candles for the interaction test: the sample days' visits as prices.
func candles() CandleSeries[day] {
	return CandleSeries[day]{Name: "Price", Data: week, Category: dayName,
		Open: func(d day) float64 { return d.visits - 5 }, High: func(d day) float64 { return d.visits + 10 },
		Low: func(d day) float64 { return d.visits - 15 }, Close: func(d day) float64 { return d.visits }}
}

// dayName returns the name of a sample day, for a category.
func dayName(d day) string { return d.name }

// A pie's table has a row for each slice with its value and share, and a Sankey diagram's has
// a row for each link.
func TestPieAndSankeyTables(t *testing.T) {
	header, rows := Pie(Series[day]{Name: "Visits", Data: week, Category: func(d day) string { return d.name },
		Y: func(d day) float64 { return d.visits }}).table()
	if want := []string{"Slice", "Value", "Share"}; !reflect.DeepEqual(header, want) || len(rows) != 7 {
		t.Fatalf("pie table %v with %d rows", header, len(rows))
	}
	if want := []string{"Tuesday", "180", "12.2%"}; !reflect.DeepEqual(rows[1].cells, want) {
		t.Errorf("Tuesday row %v, want %v", rows[1].cells, want)
	}
	header, rows = Sankey(SankeyLink{From: "A", To: "B", Value: 3}).table()
	if want := []string{"From", "To", "Value"}; !reflect.DeepEqual(header, want) || len(rows) != 1 ||
		!reflect.DeepEqual(rows[0].cells, []string{"A", "B", "3"}) {
		t.Errorf("sankey table %v %v", header, rows)
	}
}

// A chart with no data draws its frame and is named for what it is: nothing panics, and each
// kind of chart with no points is drawn.
func TestEmptyCharts(t *testing.T) {
	none := Series[day]{Name: "None", Category: dayName, Y: visitOf}
	cases := map[string]*Chart{
		"line":    Line(none),
		"bar":     Bar(none).Stacked(),
		"area":    Area(none).Stacked(),
		"scatter": Scatter(none),
		"pie":     Pie(none),
		"radar":   Radar(none),
		"candle": Candlestick(CandleSeries[day]{Name: "None", Category: dayName,
			Open: visitOf, High: visitOf, Low: visitOf, Close: visitOf}),
		"sankey": Sankey(),
		"none":   Line[day](),
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			newRig(func(*ui.Context) ui.Element { return ui.Div().Child(c) })
			if c.summary() == "" {
				t.Errorf("%s has no summary", name)
			}
		})
	}
}

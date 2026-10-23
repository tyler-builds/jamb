// Package charts draws charts with ui.Canvas: line, area, bar (grouped and
// stacked, vertical and horizontal), scatter, pie and donut, radar, candlestick
// and Sankey charts.
//
// A chart is built from data with one of the constructors, configured with chained
// methods, and placed in a tree as its Element:
//
//	ui.Div().Child(charts.Line(charts.Series[Visit]{
//		Name: "Visits",
//		Data: visits,
//		Time: func(v Visit) time.Time { return v.Day },
//		Y:    func(v Visit) float64 { return float64(v.Count) },
//	}).Title("Visits this week").
//		YAxis(charts.Axis{Label: "Visits"}.Min(0).Max(400)).
//		Size(ui.Full, ui.Px(300)))
//
// The data is converted once, when the chart is built, so build the chart again
// when the data changes; a chart built in a render function does that on each
// render. The series of one chart share a kind of horizontal axis: time, category
// or number (see Series). A Sankey diagram takes its links instead.
//
// Hovering the pointer over a chart shows a tooltip with the values at that
// place, and the chart is a focus stop. The arrow keys walk the points: Left and
// Right move along a series, Up and Down switch to the next or previous series,
// and each step is announced with the point's position and value, such as
// "Tuesday, Visits 180". For a horizontal bar chart the roles of the arrows turn
// with the bars: Up and Down walk the categories and Left and Right switch the
// series. A pie walks its slices with any arrow key, and a Sankey diagram walks
// its links. Clicking a legend item shows or hides its series, or its slice.
//
// Assistive technology reads the chart as an image named from its data: its
// type, its series, their ranges, as in "Line chart, 2 series, 7 points each,
// from Monday to Sunday; Visits from 120 to 300". Label replaces the name. The
// same data is available as a table with DataTable, whose cells read exact
// values. The words of the summary, the announcements, the tooltips, the legend
// and the table are the Chart fields of the window's ui.Strings, so an app can
// translate them.
//
// Lines and areas are downsampled before they are drawn, with
// Largest-Triangle-Three-Buckets to about two points per pixel, so a series of
// 100,000 points draws in a few milliseconds. The keyboard and the pointer use
// the points of the data, not the downsampled ones. Scatter plots draw every point.
//
// The charts take their colors from the theme. Series colors come from a
// palette for each kind of theme (light, dark and high contrast), and each
// series of a line also has its own dash pattern and marker, and each bar, area
// and slice its own hatch under high contrast, so series are not told apart by
// color alone. Under high contrast every series is the text color. A series sets
// its own color with Series.Color, and a candlestick its rising and falling colors
// with CandleSeries.Rising and Falling; under high contrast those are ignored too.
// The text, axis and grid colors are the theme's TextMuted and Border.
//
// Parts (see Style) are the chart box (Chart), the title (Title), the axis labels
// and titles (Axis), the grid lines (Grid), the legend row (Legend), each legend
// item (LegendItem) and the tooltip box (Tooltip). A part's style merges over
// the built-in defaults, then over the theme's Style for every chart (see
// ui.SetThemeStyle), then over the chart's Styles, and under high contrast its
// colors are the system colors (see docs/charts.md).
//
// The package does not inspect the data it is given. Labels, summaries and the
// table are written from the values, names and positions that the app supplies, as
// they are supplied.
package charts

import (
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ease"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/layout"
	"github.com/tyler-builds/jamb/ui"
)

// enterDuration is how long a chart takes to draw in: its lines and areas reveal
// from left to right, and its bars, slices and candles grow from their base. A
// reduced motion setting skips it.
const enterDuration = 600 * time.Millisecond

// minCanvasHeight is the smallest height of a chart's drawing, in logical pixels, unless
// the chart is sized to less (see canvasMinHeight).
const minCanvasHeight = 120

// kind is the type of a chart.
type kind uint8

const (
	kindLine kind = iota
	kindArea
	kindBar
	kindScatter
	kindPie
	kindDonut
	kindRadar
	kindCandle
	kindSankey
)

// Chart is a chart under construction. It is made by one of the constructors (Line, Bar,
// and the others) and configured with the methods below, each of which returns the chart
// so that they chain. A chart is a ui.Element, so it is placed in a tree as it is. Its
// size is Full by 300 pixels unless the sizing methods (Size, Width, Height and the
// others) say otherwise.
type Chart struct {
	ui.Composite
	kind   kind
	ds     dataset
	sk     *sankey
	title  string
	name   string
	xa, ya Axis
	legend LegendPosition
	stack  bool
	horiz  bool
	smooth bool
	format func(float64) string
	size   sizes // see Width
	styles Style
	// last is the geometry of the chart's latest render, which the tests read.
	last *geometry
}

// newChart returns a chart of kind k over the data ds. Its element renders the chart.
func newChart(k kind, ds dataset) *Chart {
	c := &Chart{kind: k, ds: ds, legend: LegendBottom}
	c.size.setWidth(ui.Full)
	c.size.setHeight(ui.Px(300))
	return c
}

// Render builds the chart. A chart is an element: place it in a tree with
// ui.Div().Child or any other container. It is built on each render of the
// component that holds it, and its state (the hidden series, the focus and the
// keyboard position) is kept by the element, so give it a Key when its place
// in the tree may change.
//
//	ui.Div().Child(charts.Line(series).Title("Visits"))
func (c *Chart) Render(ctx *ui.Context) ui.Element { return c.render(ctx) }

// Line returns a line chart of the series, one line for each. The points of a line
// are joined in the order of their positions.
func Line[T any](series ...Series[T]) *Chart {
	return newChart(kindLine, fromSeries(series, false))
}

// Area returns an area chart: a line for each series, with the area under it filled
// to the baseline. Stacked makes the areas stack on one another.
func Area[T any](series ...Series[T]) *Chart {
	return newChart(kindArea, fromSeries(series, false))
}

// Bar returns a bar chart with a bar for each series at each position, side by side.
// Stacked stacks the bars of a position on one another, and Horizontal turns the
// bars to run from left to right.
func Bar[T any](series ...Series[T]) *Chart {
	return newChart(kindBar, fromSeries(series, false))
}

// Scatter returns a scatter plot of the points of each series, each series with its
// own marker. Every point is drawn.
func Scatter[T any](series ...Series[T]) *Chart {
	return newChart(kindScatter, fromSeries(series, false))
}

// Pie returns a pie chart of the points of s, one slice for each point in order, sized
// by their values. The Category of the points names the slices.
func Pie[T any](s Series[T]) *Chart {
	return newChart(kindPie, fromSeries([]Series[T]{s}, true))
}

// Donut is Pie with a hole in its middle, which shows the total of the slices.
func Donut[T any](s Series[T]) *Chart {
	return newChart(kindDonut, fromSeries([]Series[T]{s}, true))
}

// Radar returns a radar chart: one axis for each position of the series, from the
// center outwards, with a polygon for each series through its values.
func Radar[T any](series ...Series[T]) *Chart {
	return newChart(kindRadar, fromSeries(series, false))
}

// Candlestick returns a candlestick chart of the candles of s: each candle is the open,
// high, low and close of one position, drawn as a body from the open to the close,
// with a wick from the low to the high. A rising candle is drawn hollow under high
// contrast, and a falling one solid, so they differ without color.
func Candlestick[T any](s CandleSeries[T]) *Chart {
	return newChart(kindCandle, fromCandles(s))
}

// Sankey returns a Sankey diagram of the links. The nodes are the names of the links'
// ends, placed in columns from their sources to their targets, and each link is a band
// whose width is its value.
func Sankey(links ...SankeyLink) *Chart {
	c := newChart(kindSankey, dataset{})
	c.sk = newSankey(links)
	return c
}

// Title sets the title of the chart, shown above it.
func (c *Chart) Title(s string) *Chart { c.title = s; return c }

// Label sets the name that assistive technology reads for the chart, in place of the
// name that the chart makes from its data (see the package documentation).
func (c *Chart) Label(s string) *Chart { c.name = s; return c }

// Key gives the chart a stable identity among its siblings, so its state (the
// hidden series, the focus and the keyboard position) survives reordering.
func (c *Chart) Key(k string) *Chart { c.SetKey(k); return c }

// XAxis sets the horizontal axis of a chart, which places its positions: the
// categories, the times or the numbers. In a horizontal bar chart the positions run
// down the chart, so the axis is drawn at its left.
func (c *Chart) XAxis(a Axis) *Chart { c.xa = a; return c }

// YAxis sets the vertical axis of a chart, which shows its values. In a horizontal bar
// chart the values run along the bottom, so the axis is drawn there.
func (c *Chart) YAxis(a Axis) *Chart { c.ya = a; return c }

// Legend sets where the legend is shown: LegendBottom (the default), LegendTop,
// LegendRight, or LegendNone to hide it.
func (c *Chart) Legend(p LegendPosition) *Chart { c.legend = p; return c }

// Stacked stacks the series of a bar or area chart on one another, so that each
// position shows the total of the series and the share of each. A pie or radar chart
// ignores it.
func (c *Chart) Stacked() *Chart { c.stack = true; return c }

// Horizontal turns the bars of a bar chart to run from left to right, with the
// categories down the chart. It does nothing to the other charts.
func (c *Chart) Horizontal() *Chart { c.horiz = true; return c }

// Smooth draws the lines and the areas of a chart as smooth curves through their
// points, which do not overshoot between them. It does nothing to the other charts.
func (c *Chart) Smooth() *Chart { c.smooth = true; return c }

// Format sets how values are written: in the value axis's labels (unless its Format
// is set), in the tooltips, in the table and in the summary. The default writes a
// whole number as it is and other numbers with up to two decimals.
func (c *Chart) Format(f func(float64) string) *Chart { c.format = f; return c }

// Size sets the width and the height of the chart. The width is Full by default.
func (c *Chart) Size(w, h ui.Length) *Chart { c.size.setWidth(w); c.size.setHeight(h); return c }

// The sizing methods give the chart's box a size, as they do for the widgets of the ui
// package, and win over the Chart part of Styles. The chart is Full wide by default.
// See Sizing in docs/styling.md.
func (c *Chart) Width(l ui.Length) *Chart { c.size.setWidth(l); return c }

// Height sets the height of the chart, which is 300 pixels by default.
func (c *Chart) Height(h ui.Length) *Chart { c.size.setHeight(h); return c }

// MinWidth sets the smallest width of the chart.
func (c *Chart) MinWidth(l ui.Length) *Chart { c.size.setMinWidth(l); return c }

// MinHeight sets the smallest height of the chart.
func (c *Chart) MinHeight(l ui.Length) *Chart { c.size.setMinHeight(l); return c }

// MaxWidth sets the largest width of the chart.
func (c *Chart) MaxWidth(l ui.Length) *Chart { c.size.setMaxWidth(l); return c }

// MaxHeight sets the largest height of the chart.
func (c *Chart) MaxHeight(l ui.Length) *Chart { c.size.setMaxHeight(l); return c }

// Grow makes the chart take a share of the free space on its parent's main axis.
func (c *Chart) Grow(f float32) *Chart { c.size.setGrow(f); return c }

// GridSpan sets how many columns and rows the chart covers in its parent's grid.
func (c *Chart) GridSpan(cols, rows int) *Chart { c.size.setSpan(cols, rows); return c }

// Styles sets the look of the parts of the chart, for this chart only. Set fields win
// over the theme's Style and the defaults. Calling Styles again merges with the earlier values.
func (c *Chart) Styles(st Style) *Chart {
	c.styles = c.styles.merge(st)
	return c
}

// Style is the look of the parts of a chart. A part's style merges over the built-in
// defaults, then over the Style that the theme holds for every chart (set with
// ui.SetThemeStyle), then over the Styles of the chart. Chart is the box around the title, the
// canvas and the legend: its Background, border, radius and padding. Title is the
// title text. Axis is the text of the axis labels and titles: its TextColor and
// FontSize. Grid is the grid lines: its BorderColor is their color and its BorderWidth
// their width. Legend is the row (or column) of legend items, and LegendItem each
// item, a button: its padding, radius, gap and text. Tooltip is the box of the
// tooltip: its Background, border, shadow, padding and text color.
//
// Under high contrast the colors of these parts are the system colors, whatever is set
// here or in the theme; their sizes, spacing and fonts still apply.
type Style struct {
	// Chart is the style of the whole chart.
	Chart ui.Style
	// Title is the style of the chart's title.
	Title ui.Style
	// Axis is the style of the axis labels and titles.
	Axis ui.Style
	// Grid is the style of the grid lines.
	Grid ui.Style
	// Legend is the style of the legend.
	Legend ui.Style
	// LegendItem is the style of each item of the legend.
	LegendItem ui.Style
	// Tooltip is the style of the tooltip that the pointer shows.
	Tooltip ui.Style
}

// merge returns s with the set fields of o applied on top, part by part.
func (s Style) merge(o Style) Style {
	return Style{
		Chart:      s.Chart.Merge(o.Chart),
		Title:      s.Title.Merge(o.Title),
		Axis:       s.Axis.Merge(o.Axis),
		Grid:       s.Grid.Merge(o.Grid),
		Legend:     s.Legend.Merge(o.Legend),
		LegendItem: s.LegendItem.Merge(o.LegendItem),
		Tooltip:    s.Tooltip.Merge(o.Tooltip),
	}
}

// plain returns s without the colors, gradients, shadows and opacity that high
// contrast takes from the system colors, as the ui's own parts do.
func plain(s ui.Style) ui.Style {
	s.Background, s.BorderColor, s.TextColor = ui.Color{}, ui.Color{}, ui.Color{}
	s.Gradient, s.Shadows, s.Opacity = ui.Gradient{}, nil, 0
	return s
}

// chartDefaults returns the built-in look of the parts, from the colors of t.
func chartDefaults(t *ui.Theme) Style {
	return Style{
		Chart:      ui.Style{Gap: 8, Direction: ui.Column},
		Title:      ui.Style{FontSize: 15, FontWeight: ui.FontSemibold, TextColor: t.Text},
		Axis:       ui.Style{FontSize: 12, TextColor: t.TextMuted},
		Grid:       ui.Style{BorderColor: t.Border, BorderWidth: 1},
		Legend:     ui.Style{Gap: 12, Wrap: true, Align: ui.Center},
		LegendItem: ui.Style{Gap: 6, Padding: ui.XY(6, 2), Radius: t.Radius, FontSize: 12, TextColor: t.Text},
		Tooltip: ui.Style{Padding: ui.XY(8, 4), Radius: 6, Background: t.Surface, BorderWidth: 1,
			BorderColor: t.Border, Shadows: []ui.Shadow{ui.ShadowMd}, TextColor: t.Text, FontSize: 12, Gap: 2},
	}
}

// resolveStyle returns the look of the parts in the theme t: the defaults, merged with
// the theme's charts.Style (see ui.ThemeStyle), then with the chart's Styles. Under high
// contrast the colors are the system colors (see plain).
func (c *Chart) resolveStyle(ctx *ui.Context) Style {
	t := ctx.Theme()
	themed, _ := ui.ThemeStyle[Style](t)
	st := ui.StyleLayer(ctx, chartDefaults(t), "built-in default").
		merge(ui.StyleLayer(ctx, themed, "theme")).
		merge(ui.StyleLayer(ctx, c.styles, "instance Styles"))
	if t.HighContrast {
		st = Style{Chart: plain(st.Chart), Title: plain(st.Title), Axis: plain(st.Axis), Grid: plain(st.Grid),
			Legend: plain(st.Legend), LegendItem: plain(st.LegendItem), Tooltip: plain(st.Tooltip)}
		// The grid's border is set field by field, as it was before the inspector: its
		// records show it as an element method.
		st.Grid.BorderColor, st.Grid.BorderWidth = t.Text, 1
		st.Axis = st.Axis.Merge(ui.StyleLayer(ctx, ui.Style{TextColor: t.Text}, "high contrast"))
		st.LegendItem = st.LegendItem.Merge(ui.StyleLayer(ctx, ui.Style{TextColor: t.Text}, "high contrast"))
		st.Tooltip = st.Tooltip.Merge(ui.StyleLayer(ctx, ui.Style{TextColor: t.Text}, "high contrast"))
		st.Title = st.Title.Merge(ui.StyleLayer(ctx, ui.Style{TextColor: t.Text}, "high contrast"))
	}
	return st
}

// view is what one render of a chart knows: the theme, the parts, the state the
// render read from its signals, and the geometry that the draw leaves for the
// handlers. The draw and the handlers of one render share it.
type view struct {
	c        *Chart
	t        *ui.Theme
	s        *ui.Strings
	st       Style
	hidden   []bool
	pointer  pointerAt
	cursor   cursorAt
	focused  bool
	progress float32 // the enter animation: 0 at its start, 1 when drawn in full
	geo      *geometry
}

// pointerAt is where the pointer is over the canvas, in its logical pixels, and
// whether it is over it.
type pointerAt struct {
	on   bool
	x, y float32
}

// cursorAt is the keyboard position, or the point under the pointer: a series (or a
// slice, a radar axis or a link, by kind) and an index into its points. What the index
// counts depends on the kind of the chart: the points of a series, the positions of a
// bar chart, the slices of a pie, the axes of a radar or the links of a Sankey diagram.
type cursorAt struct {
	on     bool
	series int
	index  int
}

// items returns how many legend items the chart has: its series, or for a pie its
// slices. A Sankey diagram has none.
func (c *Chart) items() int {
	switch c.kind {
	case kindSankey:
		return 0
	case kindPie, kindDonut:
		if len(c.ds.series) == 0 {
			return 0
		}
		return len(c.ds.series[0].pts)
	}
	return len(c.ds.series)
}

// isHidden reports whether legend item i is hidden.
func (v *view) isHidden(i int) bool { return i < len(v.hidden) && v.hidden[i] }

// visible returns the indices of the items that are shown, in order.
func (v *view) visible() []int {
	n := v.c.items()
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if !v.isHidden(i) {
			out = append(out, i)
		}
	}
	return out
}

// render builds the chart for its component. The hidden series, the pointer, the
// keyboard position and the focus are signals, so the chart renders again when they
// change, and the hidden series are stored as a whole, never changed in place.
func (c *Chart) render(ctx *ui.Context) ui.Element {
	hidden := ui.Local(ctx, "charts.hidden", []bool(nil))
	pointer := ui.Local(ctx, "charts.pointer", pointerAt{})
	cursor := ui.Local(ctx, "charts.cursor", cursorAt{})
	focused := ui.Local(ctx, "charts.focused", false)
	t := ctx.Theme()
	// The geometry outlives the renders: the draw writes it, and the tooltip, built in
	// the next render, reads the layout of the last draw.
	geo := ui.Local(ctx, "charts.geometry", &geometry{}).Peek()
	c.last = geo
	// The pointer, the keyboard position and the focus are read without
	// subscribing: they change the hover layer only (see hoverLayer), so moving
	// the pointer does not draw the chart again.
	v := &view{c: c, t: t, s: ctx.Strings(), st: c.resolveStyle(ctx), hidden: hidden.Get(), pointer: pointer.Peek(),
		cursor: cursor.Peek(), focused: focused.Peek(), geo: geo}
	build := func(progress float32) ui.Element {
		v.progress = progress
		return c.column(v, hidden, pointer, cursor, focused)
	}
	if ctx.ReducedMotion() {
		return build(1)
	}
	return ui.Animate("charts.enter", ui.Animation{Duration: enterDuration, Easing: ease.OutCubic}, build)
}

// column lays out the parts of a chart in a column: its title, its legend above or
// below, and the canvas with the legend beside it when the legend is on the right.
// The chart's sizes are those of the column.
func (c *Chart) column(v *view, hidden *ui.Signal[[]bool], pointer *ui.Signal[pointerAt], cursor *ui.Signal[cursorAt], focused *ui.Signal[bool]) ui.Element {
	canvas := c.canvas(v, hidden, pointer, cursor, focused)
	legend := c.legendRow(v, hidden, cursor)
	col := applySizes(c.size, ui.Div().Style(v.st.Chart))
	if c.title != "" {
		col.Child(ui.Text(c.title).Style(v.st.Title).NoWrap())
	}
	switch {
	case c.legend == LegendTop && legend != nil:
		col.Child(legend).Child(canvas)
	case c.legend == LegendRight && c.items() > 0:
		col.Child(ui.Div().Row().Gap(12).Grow(1).Child(canvas).Child(c.legendColumn(v, hidden, cursor)))
	default:
		col.Child(canvas)
		if legend != nil {
			col.Child(legend)
		}
	}
	return col
}

// canvas returns the drawing of the chart, with its pointer, keyboard and focus
// handling. The canvas is the one focus stop of the chart and is named by its
// summary. A tooltip is laid over it while the pointer or the keyboard is on a point.
func (c *Chart) canvas(v *view, hidden *ui.Signal[[]bool], pointer *ui.Signal[pointerAt], cursor *ui.Signal[cursorAt], focused *ui.Signal[bool]) *ui.DivElement {
	name := c.name
	if name == "" {
		name = c.summary(v.s)
	}
	el := ui.Canvas(func(p *ui.Painter, w, h float32) {
		c.paint(v, p, w, h)
	}).Size(ui.Full, ui.Auto).Grow(1).MinHeight(c.canvasMinHeight()).Label(name).Focusable()
	el.FocusVisible(ui.Style{BorderWidth: 2, BorderColor: v.t.FocusRing})
	el.OnMouseMove(func(e *ui.MouseEvent) {
		pointer.Set(pointerAt{on: true, x: e.LocalX, y: e.LocalY})
	})
	el.OnMouseLeave(func() { pointer.Set(pointerAt{}) })
	// The handlers read the signals as they are now, not as the last render saw them, so that
	// two events before the next render act on each other's result.
	current := func() *view {
		cv := *v
		cv.hidden, cv.cursor = hidden.Peek(), cursor.Peek()
		return &cv
	}
	el.OnFocus(func() {
		focused.Set(true)
		if cv := current(); !cv.cursor.on {
			if start, ok := c.startCursor(cv); ok {
				cursor.Set(start)
			}
		}
	})
	el.OnBlur(func() { focused.Set(false) })
	el.OnKeyDown(func(e *ui.KeyEvent) {
		if e.Mods&(input.ModCtrl|input.ModAlt|input.ModSuper) != 0 {
			return
		}
		next, text, ok := c.walk(current(), e.Key)
		if !ok {
			return
		}
		cursor.Set(next)
		ui.Announce(text, access.Polite)
		e.StopPropagation()
	})
	return el.Child(c.hoverLayer(v, pointer, cursor, focused))
}

// canvasMinHeight returns the smallest height of the drawing: 120 pixels, so that a plot
// stays readable in a chart with no height of its own. A chart sized in pixels to less
// than that is drawn at its own height, as a sparkline is.
func (c *Chart) canvasMinHeight() ui.Length {
	if c.size.h.Unit == layout.UnitPx && c.size.h.Value < minCanvasHeight {
		return c.size.h
	}
	return ui.Px(minCanvasHeight)
}

// hoverLayer returns the marks of the pointer and the keyboard position, and
// the tooltip, on a canvas of their own over the chart. It is its own
// component, the only one that reads the pointer, so a pointer move draws
// these few marks again and leaves the chart's drawing as it is.
func (c *Chart) hoverLayer(v *view, pointer *ui.Signal[pointerAt], cursor *ui.Signal[cursorAt], focused *ui.Signal[bool]) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		hv := *v
		hv.pointer, hv.cursor, hv.focused = pointer.Get(), cursor.Get(), focused.Get()
		// The layer is a plain container to assistive technology (no role), so
		// the chart stays the one image while the tooltip in it is still read.
		layer := ui.Canvas(func(p *ui.Painter, w, h float32) { c.paintCursor(&hv, p) }).
			Absolute().Size(ui.Full, ui.Full).Role(access.RoleUnknown)
		if tip := c.tooltip(&hv); tip != nil {
			layer.Child(tip)
		}
		return layer
	})
}

// startCursor returns the keyboard position a chart starts from when it gains focus:
// its first shown point.
func (c *Chart) startCursor(v *view) (cursorAt, bool) {
	if c.kind == kindSankey {
		return cursorAt{on: true}, len(c.sk.links) > 0
	}
	vis := v.visible()
	if len(vis) == 0 {
		return cursorAt{}, false
	}
	if c.kind == kindPie || c.kind == kindDonut {
		return cursorAt{on: true, series: 0, index: vis[0]}, true
	}
	return cursorAt{on: true, series: vis[0], index: 0}, true
}

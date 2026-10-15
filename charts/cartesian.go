package charts

import (
	"math"
	"strconv"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// Spacing and sizes of the parts of a cartesian chart, in logical pixels.
const (
	axisGap     = 8   // between a plot and its labels
	labelGap    = 12  // between the labels of two neighboring ticks
	edgePad     = 8   // between the canvas and a label that hangs over its edge
	markSize    = 4   // the radius of a scatter point's marker
	lineWidth   = 2   // the width of a line or an area's outline
	markerLimit = 60  // the most points a line has and still gets a marker at each
	barShare    = 0.8 // the share of a slot that a group of bars fills
	candleShare = 0.6 // the share of a slot that a candle's body fills
	areaFill    = 0.2 // the opacity of an area's fill over the surface
	maxTicks    = 14  // the most ticks an axis aims for by default, on the position axis
	maxValTicks = 10  // the most ticks the value axis aims for by default
	minPlot     = 10  // the smallest plot that is drawn, in pixels
	valueMargin = 10  // between the value labels and the plot on the left
	titleGap    = 6   // between an axis title and the labels beside it
	defaultFont = 12  // the size of the axis labels when the part sets none
	probeText   = "0" // the text whose height is a label's line height
)

// vertical reports whether the positions of the chart run along the x axis. Every
// chart does but a horizontal bar chart, whose categories run down the chart.
func (c *Chart) vertical() bool { return !(c.kind == kindBar && c.horiz) }

// cartModel is the data of the visible series of a cartesian chart, prepared for its
// layout and its draw: the slots of a band chart, the value of each visible series at
// each slot, the extents of the bars and stacks, and the domains of the positions and
// of the values before their ticks are chosen.
type cartModel struct {
	vis    []int
	band   bool
	slots  []float64
	vals   [][]float64 // a bar chart: the value of each visible series at each slot
	lo, hi [][]float64 // a bar chart and a stacked area: each visible series' extent
	plo    float64     // the domain of the positions
	phi    float64
	vlo    float64 // the domain of the values
	vhi    float64
}

// model prepares the visible series of the chart for the layout and the draw.
func (c *Chart) model(v *view) cartModel {
	ds := &c.ds
	m := cartModel{vis: v.visible(), band: c.kind == kindBar || c.kind == kindCandle}
	stacked := c.stack && (c.kind == kindBar || c.kind == kindArea)
	if m.band || stacked {
		m.slots = ds.slots(m.vis)
	}
	if c.kind == kindBar || stacked {
		m.vals = make([][]float64, len(m.vis))
		for j, k := range m.vis {
			m.vals[j] = slotValues(ds.series[k].pts, m.slots)
		}
	}
	switch {
	case stacked:
		m.lo, m.hi = stackValues(m.vals)
	case c.kind == kindBar:
		m.lo = make([][]float64, len(m.vis))
		m.hi = m.vals
		for j := range m.vis {
			m.lo[j] = make([]float64, len(m.slots))
		}
	}

	// The domain of the values covers the extents of the marks, and zero for the
	// charts that fill to a baseline.
	first := true
	include := func(a, b float64) {
		if first {
			m.vlo, m.vhi, first = a, b, false
			return
		}
		m.vlo, m.vhi = min(m.vlo, a), max(m.vhi, b)
	}
	if m.lo != nil {
		for j := range m.lo {
			for i := range m.lo[j] {
				include(m.lo[j][i], m.hi[j][i])
			}
		}
	} else {
		for _, k := range m.vis {
			if s := &ds.series[k]; len(s.pts) > 0 {
				include(s.lo, s.hi)
			}
		}
	}
	if c.kind == kindArea || c.kind == kindBar {
		include(0, 0)
	}
	if first {
		m.vlo, m.vhi = 0, 1
	}

	switch {
	case m.band:
		m.plo, m.phi = -0.5, float64(len(m.slots))-0.5
	default:
		if lo, hi, ok := ds.bounds(m.vis); ok {
			m.plo, m.phi = lo, hi
		} else {
			m.plo, m.phi = 0, 1
		}
	}
	return m
}

// cartLayout is the layout of a cartesian chart: its plot area, the scales that map
// positions and values into it, and the ticks and labels of its two axes.
type cartLayout struct {
	vert      bool
	plot      rect
	pos, val  scale
	posTicks  []float64
	posLabels []string
	valTicks  []float64
	valLabels []string
	valLo     float64 // the domain of the value axis: its ticks' nice bounds, or its Range
	valHi     float64
}

// axisFont returns the size of the axis labels, from the Axis part.
func axisFont(st Style) float32 {
	if st.Axis.FontSize > 0 {
		return st.Axis.FontSize
	}
	return defaultFont
}

// valueLabel returns the text of a value: the value axis's Format, else the chart's.
func (c *Chart) valueLabel(x float64) string {
	switch {
	case c.ya.Format != nil:
		return c.ya.Format(x)
	case c.format != nil:
		return c.format(x)
	}
	return formatNumber(x)
}

// positionLabel returns the text of a position: a category, a time or a number, for the
// labels of the position axis, the tooltips and the summary. A time shows the date, and
// the time of day when it has one.
func (c *Chart) positionLabel(x float64) string {
	switch c.ds.x {
	case xCategory:
		if i := int(x); i >= 0 && i < len(c.ds.cats) {
			return c.ds.cats[i]
		}
		return strconv.Itoa(int(x))
	case xTime:
		if c.xa.Format != nil {
			return c.xa.Format(x)
		}
		return formatTime(x, c.timeZone())
	}
	if c.xa.Format != nil {
		return c.xa.Format(x)
	}
	return formatNumber(x)
}

// timeZone returns the time zone of the chart's times, UTC when there are none.
func (c *Chart) timeZone() *time.Location {
	if c.ds.loc == nil {
		return time.UTC
	}
	return c.ds.loc
}

// xLabel returns the label of the position of a point.
func (c *Chart) xLabel(p point) string { return c.positionLabel(p.x) }

// positionTicks returns about n ticks of the position axis, with their labels. On a
// band chart a tick is a slot, labeled with the slot's position. On a category axis
// with more categories than ticks, every few categories are labeled.
func (c *Chart) positionTicks(m cartModel, n int) (ticks []float64, labels []string) {
	if m.band {
		count := len(m.slots)
		stride := spacing(count, n)
		for i := 0; i < count; i += stride {
			ticks = append(ticks, float64(i))
			labels = append(labels, c.positionLabel(m.slots[i]))
		}
		return ticks, labels
	}
	if c.ds.x == xCategory {
		first, last := int(math.Ceil(m.plo)), int(math.Floor(m.phi))
		stride := spacing(last-first+1, n)
		for i := first; i <= last; i += stride {
			ticks = append(ticks, float64(i))
			labels = append(labels, c.positionLabel(float64(i)))
		}
		return ticks, labels
	}
	if c.ds.x == xTime {
		var st timeStep
		ticks, st = timeTicks(m.plo, m.phi, n, c.timeZone())
		for _, t := range ticks {
			if c.xa.Format != nil {
				labels = append(labels, c.xa.Format(t))
			} else {
				labels = append(labels, formatTick(time.Unix(int64(t), 0).In(c.timeZone()), st))
			}
		}
		return ticks, labels
	}
	ticks, _, _ = linearTicks(m.plo, m.phi, n)
	for _, t := range ticks {
		labels = append(labels, c.positionLabel(t))
	}
	return ticks, labels
}

// spacing returns the stride that shows about n of count items, every stride-th of them,
// starting with the first. It is at least one.
func spacing(count, n int) int {
	if n <= 1 {
		return max(count, 1)
	}
	if count <= n {
		return 1
	}
	return (count - 1 + n - 2) / (n - 1)
}

// valueTicks returns about n ticks of the value axis, and the domain that they span:
// the ticks' nice bounds, or the ends that the axis's Min and Max fix.
func (c *Chart) valueTicks(m cartModel, n int) (ticks []float64, lo, hi float64) {
	vlo, vhi := m.vlo, m.vhi
	if c.ya.hasMin {
		vlo = c.ya.min
	}
	if c.ya.hasMax {
		vhi = c.ya.max
	}
	ticks, lo, hi = linearTicks(vlo, vhi, n)
	if c.ya.hasMin {
		lo = c.ya.min
	}
	if c.ya.hasMax {
		hi = c.ya.max
	}
	return ticks, lo, hi
}

// measureLabels returns the width of the widest label, the height of a line of text,
// and the width of each label, at the size of the axis labels.
func measureLabels(p *ui.Painter, labels []string, size float32) (widest, lineH float32, widths []float32) {
	st := ui.TextFormat{FontSize: size}
	widths = make([]float32, len(labels))
	for i, s := range labels {
		w, h := p.MeasureText(s, st)
		widths[i] = w
		widest = max(widest, w)
		lineH = max(lineH, h)
	}
	if lineH == 0 {
		_, lineH = p.MeasureText(probeText, st)
	}
	return widest, lineH, widths
}

// axisScale returns the scale of an axis over the plot. A horizontal axis runs from the
// plot's left edge to its right. A vertical one runs from the bottom edge to the top when
// it is reversed (a value axis, whose values grow upwards), and else from the top down.
func axisScale(lo, hi float64, plot rect, horizontal, reverse bool) scale {
	if horizontal {
		return newScale(lo, hi, plot.x0, plot.x1)
	}
	if reverse {
		return newScale(lo, hi, plot.y1, plot.y0)
	}
	return newScale(lo, hi, plot.y0, plot.y1)
}

// layout sizes the plot of a chart drawn in a canvas of w by h logical pixels. The
// labels are measured with the painter, so the margins hold them, and the ticks suit
// the plot they are drawn in. The first pass sizes the labels from an estimated plot and
// the second sizes the plot from the labels of the first, which is enough for the
// labels that a plot holds.
func (c *Chart) layout(p *ui.Painter, w, h float64, m cartModel, st Style) cartLayout {
	L := cartLayout{vert: c.vertical()}
	size := axisFont(st)
	_, lineH, _ := measureLabels(p, []string{probeText}, size)
	title := float64(lineH) + titleGap
	plot := rect{x0: 56, y0: 8, x1: w - 8, y1: h - 36}
	for pass := 0; pass < 2; pass++ {
		// The axis lengths: the value axis along the value direction, the position
		// axis along the other.
		valLen, posLen := plot.y1-plot.y0, plot.x1-plot.x0
		if !L.vert {
			valLen, posLen = plot.x1-plot.x0, plot.y1-plot.y0
		}

		nPos := c.xa.Ticks
		if nPos == 0 {
			nPos = clampInt(int(posLen/90), 2, maxTicks)
			if !L.vert {
				nPos = clampInt(int(posLen/(float64(lineH)*2.2)), 2, 40)
			}
		}
		L.pos = axisScale(m.plo, m.phi, plot, L.vert, false)
		var posWidths []float32
		var posWide, posH float32
		for n := nPos; ; n-- {
			L.posTicks, L.posLabels = c.positionTicks(m, n)
			posWide, posH, posWidths = measureLabels(p, L.posLabels, size)
			if n <= 1 || c.positionFits(L, posWidths, posH) {
				break
			}
		}

		nVal := c.ya.Ticks
		if nVal == 0 {
			nVal = clampInt(int(valLen/(float64(lineH)*2.6)), 2, maxValTicks)
			if !L.vert {
				nVal = clampInt(int(valLen/80), 2, maxValTicks)
			}
		}
		var valWide, valH float32
		var valWidths []float32
		for n := nVal; ; n-- {
			L.valTicks, L.valLo, L.valHi = c.valueTicks(m, n)
			L.valLabels = L.valLabels[:0]
			for _, t := range L.valTicks {
				L.valLabels = append(L.valLabels, c.valueLabel(t))
			}
			valWide, valH, valWidths = measureLabels(p, L.valLabels, size)
			L.val = axisScale(L.valLo, L.valHi, plot, !L.vert, L.vert)
			if n <= 2 || c.valueFits(L, valWide, valH) {
				break
			}
		}

		// The margins hold the outer labels, the value labels on the value side, and the
		// titles beside the axes that have them.
		var left, right, top, bottom float64
		if L.vert {
			left = float64(valWide) + valueMargin
			bottom = float64(posH) + axisGap
			top = math.Max(float64(valH)/2, edgePad)
			right = math.Max(edgePad, lastOf(posWidths)/2)
			left = math.Max(left, firstOf(posWidths)/2)
			if c.ya.Label != "" {
				top += title
			}
			if c.xa.Label != "" {
				bottom += title
			}
		} else {
			left = float64(posWide) + valueMargin
			bottom = float64(valH) + axisGap
			top = math.Max(float64(posH)/2, edgePad)
			right = math.Max(edgePad, lastOf(valWidths)/2)
			left = math.Max(left, firstOf(valWidths)/2)
			if c.xa.Label != "" {
				top += title
			}
			if c.ya.Label != "" {
				bottom += title
			}
		}
		plot = rect{x0: left, y0: top, x1: w - right, y1: h - bottom}
	}
	L.plot = plot
	L.pos = axisScale(m.plo, m.phi, plot, L.vert, false)
	L.val = axisScale(L.valLo, L.valHi, plot, !L.vert, L.vert)
	return L
}

// firstOf returns the first of the widths, or zero when there are none.
func firstOf(ws []float32) float64 {
	if len(ws) == 0 {
		return 0
	}
	return float64(ws[0])
}

// lastOf returns the last of the widths, or zero when there are none.
func lastOf(ws []float32) float64 {
	if len(ws) == 0 {
		return 0
	}
	return float64(ws[len(ws)-1])
}

// positionFits reports whether the labels of the position axis do not overlap their
// neighbors at their pixel positions. Along a horizontal axis they need their widths
// and a gap; along a vertical one, the height of a line of text and a smaller gap.
func (c *Chart) positionFits(L cartLayout, widths []float32, lineH float32) bool {
	for i := 1; i < len(L.posTicks); i++ {
		gap := math.Abs(L.pos.at(L.posTicks[i]) - L.pos.at(L.posTicks[i-1]))
		if L.vert {
			if gap < float64(widths[i-1]+widths[i])/2+labelGap {
				return false
			}
		} else if gap < float64(lineH)+labelGap/2 {
			return false
		}
	}
	return true
}

// valueFits reports whether the labels of the value axis do not overlap. A vertical
// axis stacks its labels, which need the height of a line between them; a horizontal
// one needs room for the widest label.
func (c *Chart) valueFits(L cartLayout, widest, lineH float32) bool {
	if len(L.valTicks) < 2 {
		return true
	}
	gap := math.Abs(L.val.at(L.valTicks[1]) - L.val.at(L.valTicks[0]))
	if L.vert {
		return gap >= float64(lineH)+labelGap/2
	}
	return gap >= float64(widest)+labelGap
}

// clampInt returns v limited to lo through hi.
func clampInt(v, lo, hi int) int { return min(max(v, lo), hi) }

// paintCartesian draws a line, area, bar, scatter or candlestick chart into its canvas
// of w by h logical pixels, and leaves its geometry for the handlers and the cursor.
func (c *Chart) paintCartesian(v *view, p *ui.Painter, w, h float32) {
	m := c.model(v)
	L := c.layout(p, float64(w), float64(h), m, v.st)
	g := v.geo
	*g = geometry{w: float64(w), h: float64(h), plot: L.plot, vert: L.vert, pos: L.pos, val: L.val,
		vis: m.vis, band: m.band, slots: m.slots, stackLo: m.lo, stackHi: m.hi, drawn: true}
	if L.plot.x1-L.plot.x0 < minPlot || L.plot.y1-L.plot.y0 < minPlot {
		return
	}
	c.paintGrid(v, p, L)
	c.paintMarks(v, p, g, m)
	c.paintAxes(v, p, L)
}

// paintGrid draws the grid lines: the value axis's by default, and the position axis's
// where its Grid is GridOn.
func (c *Chart) paintGrid(v *view, p *ui.Painter, L cartLayout) {
	width := float32(v.st.Grid.BorderWidth)
	if width == 0 {
		width = 1
	}
	plot := L.plot
	path := ui.NewPath()
	if c.ya.Grid != GridOff {
		for _, t := range L.valTicks {
			a := L.val.at(t)
			if L.vert {
				path.MoveTo(float32(plot.x0), float32(a)).LineTo(float32(plot.x1), float32(a))
			} else {
				path.MoveTo(float32(a), float32(plot.y0)).LineTo(float32(a), float32(plot.y1))
			}
		}
	}
	if c.xa.Grid == GridOn {
		for _, t := range L.posTicks {
			a := L.pos.at(t)
			if L.vert {
				path.MoveTo(float32(a), float32(plot.y0)).LineTo(float32(a), float32(plot.y1))
			} else {
				path.MoveTo(float32(plot.x0), float32(a)).LineTo(float32(plot.x1), float32(a))
			}
		}
	}
	p.Stroke(path, ui.StrokeStyle{Width: width, Color: v.st.Grid.BorderColor})
}

// paintAxes draws the axis lines, the tick labels and the axis titles. The value axis
// is at the left of a vertical chart and the bottom of a horizontal one; the position
// axis is at the bottom of a vertical chart and the left of a horizontal one.
func (c *Chart) paintAxes(v *view, p *ui.Painter, L cartLayout) {
	size := axisFont(v.st)
	text := v.st.Axis.TextColor
	plot := L.plot
	line := ui.StrokeStyle{Width: 1, Color: v.t.BorderStrong}
	x0, x1, y0, y1 := float32(plot.x0), float32(plot.x1), float32(plot.y0), float32(plot.y1)
	if L.vert {
		p.Stroke(ui.NewPath().MoveTo(x0, y0).LineTo(x0, y1), line)
		p.Stroke(ui.NewPath().MoveTo(x0, y1).LineTo(x1, y1), line)
	} else {
		p.Stroke(ui.NewPath().MoveTo(x0, y1).LineTo(x1, y1), line)
		p.Stroke(ui.NewPath().MoveTo(x0, y0).LineTo(x0, y1), line)
	}
	for i, t := range L.valTicks {
		if L.vert {
			p.Text(x0-axisGap, float32(L.val.at(t)), L.valLabels[i],
				ui.TextFormat{FontSize: size, Color: text, Align: ui.End, VAlign: ui.Center})
		} else {
			p.Text(float32(L.val.at(t)), y1+axisGap, L.valLabels[i],
				ui.TextFormat{FontSize: size, Color: text, Align: ui.Center, VAlign: ui.Start})
		}
	}
	for i, t := range L.posTicks {
		if L.vert {
			p.Text(float32(L.pos.at(t)), y1+axisGap, L.posLabels[i],
				ui.TextFormat{FontSize: size, Color: text, Align: ui.Center, VAlign: ui.Start})
		} else {
			p.Text(x0-axisGap, float32(L.pos.at(t)), L.posLabels[i],
				ui.TextFormat{FontSize: size, Color: text, Align: ui.End, VAlign: ui.Center})
		}
	}

	h := float32(v.geo.h)
	valueTitle, posTitle := c.ya.Label, c.xa.Label
	if L.vert {
		if valueTitle != "" {
			p.Text(x0, 2, valueTitle, ui.TextFormat{FontSize: size, Color: text, Align: ui.Start, VAlign: ui.Start})
		}
		if posTitle != "" {
			p.Text((x0+x1)/2, h-2, posTitle, ui.TextFormat{FontSize: size, Color: text, Align: ui.Center, VAlign: ui.End})
		}
	} else {
		if valueTitle != "" {
			p.Text((x0+x1)/2, h-2, valueTitle, ui.TextFormat{FontSize: size, Color: text, Align: ui.Center, VAlign: ui.End})
		}
		if posTitle != "" {
			p.Text(2, 2, posTitle, ui.TextFormat{FontSize: size, Color: text, Align: ui.Start, VAlign: ui.Start})
		}
	}
}

// paintMarks draws the marks of the visible series for the kind of the chart, at the
// enter animation's progress.
func (c *Chart) paintMarks(v *view, p *ui.Painter, g *geometry, m cartModel) {
	switch c.kind {
	case kindLine:
		for _, k := range m.vis {
			c.paintLine(v, p, g, k)
		}
	case kindArea:
		c.paintArea(v, p, g, m)
	case kindBar:
		c.paintBars(v, p, g, m)
	case kindScatter:
		for _, k := range m.vis {
			l := c.lookOf(v.t, k)
			for _, pt := range c.ds.series[k].pts {
				x, y := g.xy(pt.x, pt.y)
				drawMarker(p, l.marker, float32(x), float32(y), markSize, l.color)
			}
		}
	case kindCandle:
		c.paintCandles(v, p, g, m)
	}
}

// reveal limits the drawing of a line or an area to the part of the plot that the enter
// animation has reached: from the left in a vertical chart, and from the top in a
// horizontal one. It reports whether it set a clip, which the caller restores.
func reveal(p *ui.Painter, g *geometry, progress float32) bool {
	if progress >= 1 {
		return false
	}
	p.Save()
	if g.vert {
		x1 := g.plot.x0 + (g.plot.x1-g.plot.x0)*float64(progress)
		p.Clip(ui.NewPath().Rect(float32(g.plot.x0), 0, float32(x1-g.plot.x0)+1, float32(g.h)))
	} else {
		y1 := g.plot.y0 + (g.plot.y1-g.plot.y0)*float64(progress)
		p.Clip(ui.NewPath().Rect(0, float32(g.plot.y0), float32(g.w), float32(y1-g.plot.y0)+1))
	}
	return true
}

// thinned returns the coordinates of the points of series k to draw at this plot: all of
// them when there are about two per pixel or fewer, else their Largest-Triangle-Three-
// Buckets reduction to two per pixel of the plot's length.
func (c *Chart) thinned(g *geometry, k int) (xs, ys []float64) {
	s := &c.ds.series[k]
	threshold := int(2 * math.Abs(g.plot.x1-g.plot.x0))
	if !g.vert {
		threshold = int(2 * math.Abs(g.plot.y1-g.plot.y0))
	}
	if len(s.xs) <= threshold || threshold < 3 {
		return s.xs, s.ys
	}
	idx := lttb(s.xs, s.ys, threshold)
	xs, ys = make([]float64, len(idx)), make([]float64, len(idx))
	for j, i := range idx {
		xs[j], ys[j] = s.xs[i], s.ys[i]
	}
	return xs, ys
}

// linePath returns the outline through the points xs and ys: straight segments, or a
// smooth curve when the chart is Smooth.
func (c *Chart) linePath(g *geometry, xs, ys []float64) *ui.Path {
	px := make([]float64, len(xs))
	py := make([]float64, len(ys))
	for i := range xs {
		px[i], py[i] = g.xy(xs[i], ys[i])
	}
	if c.smooth {
		return smoothPath(px, py)
	}
	path := ui.NewPath()
	for i := range px {
		if i == 0 {
			path.MoveTo(float32(px[i]), float32(py[i]))
		} else {
			path.LineTo(float32(px[i]), float32(py[i]))
		}
	}
	return path
}

// paintLine draws the line of series k, with a marker at each point when the line is
// short enough for them to read.
func (c *Chart) paintLine(v *view, p *ui.Painter, g *geometry, k int) {
	l := c.lookOf(v.t, k)
	xs, ys := c.thinned(g, k)
	clipped := reveal(p, g, v.progress)
	// A round join is an arc at every vertex, which costs the rasterizer more than the
	// line's width repays on a long line, so a long line takes bevel joins.
	st := ui.StrokeStyle{Width: lineWidth, Color: l.color, Dash: l.dash, Join: ui.JoinBevel}
	if len(xs) <= markerLimit {
		st.Join, st.Cap = ui.JoinRound, ui.CapRound
	}
	p.Stroke(c.linePath(g, xs, ys), st)
	if len(xs) <= markerLimit {
		for i := range xs {
			x, y := g.xy(xs[i], ys[i])
			drawMarker(p, l.marker, float32(x), float32(y), 3, l.color)
		}
	}
	if clipped {
		p.Restore()
	}
}

// areaPath returns the closed outline of an area over the points xs and ys: the line,
// then down to the baseline at the value zero, and back along it to the start.
func (c *Chart) areaPath(g *geometry, xs, ys []float64) *ui.Path {
	area := c.linePath(g, xs, ys)
	if len(xs) == 0 {
		return area
	}
	base := g.val.at(0)
	lx, ly := g.xy(xs[len(xs)-1], ys[len(ys)-1])
	fx, fy := g.xy(xs[0], ys[0])
	if g.vert {
		area.LineTo(float32(lx), float32(base)).LineTo(float32(fx), float32(base))
	} else {
		area.LineTo(float32(base), float32(ly)).LineTo(float32(base), float32(fy))
	}
	return area.Close()
}

// paintArea draws the areas of the visible series. Unstacked, each fills from its line
// to the baseline, and its line is drawn over the fill. Stacked, each fills the band
// between its extent and the one below it.
func (c *Chart) paintArea(v *view, p *ui.Painter, g *geometry, m cartModel) {
	clipped := reveal(p, g, v.progress)
	if c.stack {
		c.paintStackedArea(v, p, g, m)
	} else {
		for _, k := range m.vis {
			l := c.lookOf(v.t, k)
			xs, ys := c.thinned(g, k)
			c.fillArea(v, p, g, c.areaPath(g, xs, ys), l)
			p.Stroke(c.linePath(g, xs, ys), ui.StrokeStyle{Width: lineWidth, Color: l.color, Dash: l.dash, Join: ui.JoinRound})
		}
	}
	if clipped {
		p.Restore()
	}
}

// paintStackedArea draws the stacked areas. The positions are thinned once, for all the
// series, by the total of the stack, so that every series keeps the same positions.
func (c *Chart) paintStackedArea(v *view, p *ui.Painter, g *geometry, m cartModel) {
	if len(m.vis) == 0 {
		return
	}
	totals := m.hi[len(m.hi)-1]
	idx := lttb(m.slots, totals, clampInt(int(2*math.Abs(g.plot.x1-g.plot.x0)), 3, max(3, len(m.slots))))
	for j, k := range m.vis {
		l := c.lookOf(v.t, k)
		xs := make([]float64, len(idx))
		top := make([]float64, len(idx))
		bottom := make([]float64, len(idx))
		for n, i := range idx {
			xs[n], top[n], bottom[n] = m.slots[i], m.hi[j][i], m.lo[j][i]
		}
		// The lower edge runs back along the extent below, from the last point to the first.
		area := c.linePath(g, xs, top)
		for n := len(idx) - 1; n >= 0; n-- {
			x, y := g.xy(xs[n], bottom[n])
			area.LineTo(float32(x), float32(y))
		}
		c.fillArea(v, p, g, area.Close(), l)
		p.Stroke(c.linePath(g, xs, top), ui.StrokeStyle{Width: lineWidth, Color: l.color, Dash: l.dash, Join: ui.JoinRound})
	}
}

// fillArea fills an area with its series' color at areaFill over the surface. Under high
// contrast it is the series' hatch over the surface, outlined in the text color.
func (c *Chart) fillArea(v *view, p *ui.Painter, g *geometry, area *ui.Path, l look) {
	if v.t.HighContrast {
		fillHatched(p, v.t, area, l, g.plot)
		return
	}
	p.Fill(area, l.color.Alpha(areaFill))
}

// fillHatched fills a shape with a series' pattern under high contrast: solid in the
// text color, or the hatch over the surface, outlined in the text color.
func fillHatched(p *ui.Painter, t *ui.Theme, shape *ui.Path, l look, b rect) {
	if l.hatch == hatchSolid {
		p.Fill(shape, t.Text)
		return
	}
	p.Fill(shape, t.Surface)
	hatchFill(p, shape, l.hatch, t.Text, float32(b.x0), float32(b.y0), float32(b.x1), float32(b.y1))
	p.Stroke(shape, ui.StrokeStyle{Width: 1, Color: t.Text})
}

// barRect returns the rectangle of a bar that spans the positions a0 to a1, in pixels,
// and the values from the base to the value's pixels lo and hi. progress scales the bar's
// length from the base, for the enter animation.
func barRect(g *geometry, a0, a1, base, lo, hi float64, progress float32) *ui.Path {
	lo = base + (lo-base)*float64(progress)
	hi = base + (hi-base)*float64(progress)
	top, bottom := math.Min(lo, hi), math.Max(lo, hi)
	if g.vert {
		return ui.NewPath().Rect(float32(a0), float32(top), float32(a1-a0), float32(bottom-top))
	}
	return ui.NewPath().Rect(float32(top), float32(a0), float32(bottom-top), float32(a1-a0))
}

// paintBars draws the bars of a bar chart: a group of bars at each slot, side by side,
// or when stacked a single bar of the stack's extents.
func (c *Chart) paintBars(v *view, p *ui.Painter, g *geometry, m cartModel) {
	if len(m.vis) == 0 {
		return
	}
	sw := g.slotWidth()
	base := g.val.at(0)
	group := sw * barShare
	for j, k := range m.vis {
		l := c.lookOf(v.t, k)
		bw := group / float64(len(m.vis))
		off := -group/2 + float64(j)*bw
		if c.stack {
			bw, off = group, -group/2
		}
		for i := range m.slots {
			lo, hi := 0.0, m.vals[j][i]
			if c.stack {
				lo, hi = m.lo[j][i], m.hi[j][i]
			}
			if lo == hi {
				continue
			}
			center := g.pos.at(float64(i)) + off
			bar := barRect(g, center, center+bw, base, g.val.at(lo), g.val.at(hi), v.progress)
			if v.t.HighContrast {
				fillHatched(p, v.t, bar, l, g.plot)
				continue
			}
			p.Fill(bar, l.color)
		}
	}
}

// paintCandles draws the candles at the slots: a wick from the low to the high, and a body
// from the open to the close, in the success color when the candle rises and the danger
// color when it falls, unless the CandleSeries sets its own (see candleColor). Under high
// contrast a rising body is hollow and a falling one solid.
func (c *Chart) paintCandles(v *view, p *ui.Painter, g *geometry, m cartModel) {
	if len(c.ds.series) == 0 {
		return
	}
	pts := c.ds.series[0].pts
	bw := g.slotWidth() * candleShare
	for i, s := range m.slots {
		j, ok := findX(pts, s)
		if !ok {
			continue
		}
		cd := pts[j]
		up := cd.close >= cd.open
		color := c.candleColor(v.t, up)
		center := g.pos.at(float64(i))
		hx, hy := g.xy(float64(i), cd.high)
		lx, ly := g.xy(float64(i), cd.low)
		p.Stroke(ui.NewPath().MoveTo(float32(hx), float32(hy)).LineTo(float32(lx), float32(ly)),
			ui.StrokeStyle{Width: 1, Color: color})
		// The body runs along the value axis, from the open to the close.
		valuePx := func(val float64) float64 {
			x, y := g.xy(float64(i), val)
			if g.vert {
				return y
			}
			return x
		}
		open, closed := valuePx(cd.open), valuePx(cd.close)
		body := barRect(g, center-bw/2, center+bw/2, open, open, closed, 1)
		switch {
		case v.t.HighContrast && up:
			p.Fill(body, v.t.Surface)
			p.Stroke(body, ui.StrokeStyle{Width: 1, Color: v.t.Text})
		default:
			p.Fill(body, color)
		}
	}
}

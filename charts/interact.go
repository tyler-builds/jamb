package charts

import (
	"math"
	"strconv"
	"strings"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// focusRingWidth is the width of the keyboard position's ring, and hoverWidth that of
// the outline of a hovered pie slice or Sankey link.
const (
	focusRingWidth = 2
	hoverWidth     = 2
	focusRadius    = 6
	crosshairDash  = 3
	slotHighlight  = 0.08 // the opacity of the highlight of a bar's slot, over the text
)

// paint draws a chart into its canvas for its kind, then the marks of the pointer and the
// keyboard position over it.
func (c *Chart) paint(v *view, p *ui.Painter, w, h float32) {
	switch c.kind {
	case kindPie, kindDonut:
		c.paintPie(v, p, w, h)
	case kindRadar:
		c.paintRadar(v, p, w, h)
	case kindSankey:
		c.paintSankey(v, p, w, h)
	default:
		c.paintCartesian(v, p, w, h)
	}
	c.paintCursor(v, p)
}

// arrow returns the direction of an arrow key: -1 or +1 along x and along y, and whether
// the key is an arrow at all.
func arrow(k input.Key) (dx, dy int, ok bool) {
	switch k {
	case input.KeyLeft:
		return -1, 0, true
	case input.KeyRight:
		return 1, 0, true
	case input.KeyUp:
		return 0, -1, true
	case input.KeyDown:
		return 0, 1, true
	}
	return 0, 0, false
}

// walk returns the keyboard position that an arrow key moves to, the text that the move
// announces, and whether the key moves the position at all. A cartesian chart walks its
// points along the axis of its positions and switches series along the other; a
// horizontal bar chart walks its categories up and down, since they run down the chart.
// A pie walks its slices in either direction, a radar chart its axes along x and its series
// along y, and a Sankey diagram its links.
func (c *Chart) walk(v *view, k input.Key) (cursorAt, string, bool) {
	dx, dy, ok := arrow(k)
	if !ok {
		return cursorAt{}, "", false
	}
	cur := v.cursor
	if !cur.on {
		if cur, ok = c.startCursor(v); !ok {
			return cursorAt{}, "", false
		}
	}
	vis := v.visible()
	next := cur
	switch c.kind {
	case kindSankey:
		step := dx + dy
		next.index = clampInt(cur.index+step, 0, len(c.sk.links)-1)
	case kindPie, kindDonut:
		step := dx + dy
		next.index = neighbor(vis, cur.index, step)
	case kindRadar:
		if dx != 0 {
			next.index = clampInt(cur.index+dx, 0, max(v.geo.axes-1, 0))
		} else {
			next = c.switchSeries(v, cur, vis, dy)
		}
	default:
		walkDelta, switchDelta := dx, dy
		if !c.vertical() {
			walkDelta, switchDelta = dy, dx
		}
		if walkDelta != 0 {
			next.index = c.walkIndex(v, cur, walkDelta)
		} else {
			next = c.switchSeries(v, cur, vis, switchDelta)
		}
	}
	return next, c.describe(v, next), true
}

// neighbor returns the item after (or before, for a negative step) the item x in the
// list vis, clamped to the ends of the list. A value not in the list is the first item.
func neighbor(vis []int, x, step int) int {
	if len(vis) == 0 {
		return x
	}
	pos := 0
	for i, v := range vis {
		if v == x {
			pos = i
		}
	}
	return vis[clampInt(pos+step, 0, len(vis)-1)]
}

// walkIndex returns the point of the cursor's series, or the slot of a band chart, that
// a step of d moves to. It stops at the ends.
func (c *Chart) walkIndex(v *view, cur cursorAt, d int) int {
	n := len(c.ds.series[cur.series].pts)
	if c.kind == kindBar || c.kind == kindCandle {
		n = len(v.geo.slots)
	}
	return clampInt(cur.index+d, 0, max(n-1, 0))
}

// switchSeries returns the position on the next or previous visible series from the
// cursor, at the same place along the axis of its positions: the same slot on a band
// chart, and the point nearest in x on the others. A radar chart keeps its axis.
func (c *Chart) switchSeries(v *view, cur cursorAt, vis []int, d int) cursorAt {
	if len(vis) == 0 || c.kind == kindCandle {
		return cur
	}
	next := cur
	next.series = neighbor(vis, cur.series, d)
	if c.kind == kindRadar || c.kind == kindBar {
		return next
	}
	if pt, ok := c.pointAt(v, cur); ok {
		pts := c.ds.series[next.series].pts
		if len(pts) > 0 {
			next.index = nearestX(pts, pt.x)
		}
	}
	return next
}

// pointAt returns the point of the cursor: the point of its series at its index, or on a
// band chart the point of its series at its slot. It is false when the series has none.
func (c *Chart) pointAt(v *view, cur cursorAt) (point, bool) {
	if cur.series < 0 || cur.series >= len(c.ds.series) {
		return point{}, false
	}
	pts := c.ds.series[cur.series].pts
	if c.kind == kindBar || c.kind == kindCandle {
		if cur.index < 0 || cur.index >= len(v.geo.slots) {
			return point{}, false
		}
		j, ok := findX(pts, v.geo.slots[cur.index])
		if !ok {
			return point{}, false
		}
		return pts[j], true
	}
	if cur.index < 0 || cur.index >= len(pts) {
		return point{}, false
	}
	return pts[cur.index], true
}

// valueText returns the text of a value in a tooltip, an announcement or the table: the
// chart's Format, else the default number format.
func (c *Chart) valueText(y float64) string {
	if c.format != nil {
		return c.format(y)
	}
	return formatNumber(y)
}

// describe returns the text announced for a position: its place and its value, as in
// "Tuesday, Visits 180".
func (c *Chart) describe(v *view, cur cursorAt) string {
	switch c.kind {
	case kindSankey:
		if cur.index < 0 || cur.index >= len(c.sk.links) {
			return ""
		}
		e := c.sk.links[cur.index]
		return c.sk.nodes[e.from].name + " to " + c.sk.nodes[e.to].name + ", " + c.valueText(e.value)
	case kindPie, kindDonut:
		pt := c.ds.series[0].pts[cur.index]
		return c.xLabel(pt) + ", " + c.valueText(pt.y) + " (" + c.share(pt.y, v.visible()) + ")"
	case kindCandle:
		pt, ok := c.pointAt(v, cur)
		if !ok {
			return ""
		}
		return c.xLabel(pt) + ", open " + c.valueText(pt.open) + ", high " + c.valueText(pt.high) +
			", low " + c.valueText(pt.low) + ", close " + c.valueText(pt.close)
	case kindRadar:
		if cur.index < 0 || cur.index >= len(v.geo.slots) {
			return ""
		}
		label := c.positionLabel(v.geo.slots[cur.index])
		val := slotValue(c.ds.series[cur.series].pts, v.geo.slots, cur.index)
		return label + ", " + c.itemName(cur.series) + " " + c.valueText(val)
	}
	label := c.positionLabel(c.cursorX(v, cur))
	pt, ok := c.pointAt(v, cur)
	if !ok {
		return label + ", " + c.itemName(cur.series) + " no value"
	}
	return c.xLabel(pt) + ", " + c.itemName(cur.series) + " " + c.valueText(pt.y)
}

// cursorX returns the position of the cursor on a cartesian chart: the slot of a band
// chart, and the point's position otherwise.
func (c *Chart) cursorX(v *view, cur cursorAt) float64 {
	if c.kind == kindBar && cur.index >= 0 && cur.index < len(v.geo.slots) {
		return v.geo.slots[cur.index]
	}
	if pt, ok := c.pointAt(v, cur); ok {
		return pt.x
	}
	return 0
}

// share returns the share of the total of the visible slices that a value y is, as a
// percentage with at most one decimal, such as "40%".
func (c *Chart) share(y float64, vis []int) string {
	total := 0.0
	pts := c.ds.series[0].pts
	for _, i := range vis {
		total += math.Max(pts[i].y, 0)
	}
	if total <= 0 {
		return "0%"
	}
	s := strconv.FormatFloat(100*math.Max(y, 0)/total, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0") + "%"
}

// tipRow is a line of a tooltip: a color swatch and its text.
type tipRow struct {
	color ui.Color
	text  string
}

// tipContent returns the header and the rows of the tooltip of a position: the position's
// label, and each visible series' value at it.
func (c *Chart) tipContent(v *view, cur cursorAt) (string, []tipRow) {
	switch c.kind {
	case kindSankey:
		e := c.sk.links[cur.index]
		return c.sk.nodes[e.from].name + " to " + c.sk.nodes[e.to].name,
			[]tipRow{{v.t.Text, c.valueText(e.value)}}
	case kindPie, kindDonut:
		pt := c.ds.series[0].pts[cur.index]
		return c.xLabel(pt), []tipRow{{seriesLook(v.t, cur.index).color, c.valueText(pt.y) + " (" + c.share(pt.y, v.visible()) + ")"}}
	case kindCandle:
		pt, _ := c.pointAt(v, cur)
		return c.xLabel(pt), []tipRow{
			{v.t.Text, "Open " + c.valueText(pt.open)}, {v.t.Text, "High " + c.valueText(pt.high)},
			{v.t.Text, "Low " + c.valueText(pt.low)}, {v.t.Text, "Close " + c.valueText(pt.close)}}
	case kindRadar:
		if cur.index < 0 || cur.index >= len(v.geo.slots) {
			return "", nil
		}
		var rows []tipRow
		for _, k := range v.visible() {
			val := slotValue(c.ds.series[k].pts, v.geo.slots, cur.index)
			rows = append(rows, tipRow{seriesLook(v.t, k).color, c.itemName(k) + " " + c.valueText(val)})
		}
		return c.positionLabel(v.geo.slots[cur.index]), rows
	}
	var header string
	if c.kind == kindBar {
		if cur.index < 0 || cur.index >= len(v.geo.slots) {
			return "", nil
		}
		header = c.positionLabel(v.geo.slots[cur.index])
	} else if pt, ok := c.pointAt(v, cur); ok {
		header = c.xLabel(pt)
	}
	var rows []tipRow
	for _, k := range v.visible() {
		cur2 := cursorAt{on: true, series: k, index: cur.index}
		if c.kind != kindBar {
			x := c.cursorX(v, cur)
			pts := c.ds.series[k].pts
			if len(pts) == 0 {
				continue
			}
			cur2.index = nearestX(pts, x)
		}
		pt, ok := c.pointAt(v, cur2)
		if !ok {
			continue
		}
		rows = append(rows, tipRow{seriesLook(v.t, k).color, c.itemName(k) + " " + c.valueText(pt.y)})
	}
	return header, rows
}

// cursorXY returns the pixel position of the cursor's mark: the point on a line, the top
// of a bar, the middle of a slice, the vertex of a radar chart or the middle of a link.
func (c *Chart) cursorXY(v *view, cur cursorAt) (x, y float64, ok bool) {
	g := v.geo
	if !g.drawn {
		return 0, 0, false
	}
	switch c.kind {
	case kindPie, kindDonut:
		for _, s := range g.slices {
			if s.index == cur.index {
				mid := (s.a0 + s.a1) / 2
				r := (g.rad + g.inner) / 2
				return g.cx + r*math.Cos(mid), g.cy + r*math.Sin(mid), true
			}
		}
		return 0, 0, false
	case kindRadar:
		x, y = c.radarVertex(v, cur.series, cur.index)
		return x, y, true
	case kindSankey:
		if cur.index < 0 || cur.index >= len(g.links) {
			return 0, 0, false
		}
		poly := g.links[cur.index].poly
		var sx, sy float64
		for i := 0; i+1 < len(poly); i += 2 {
			sx, sy = sx+poly[i], sy+poly[i+1]
		}
		n := float64(len(poly) / 2)
		return sx / n, sy / n, true
	case kindBar:
		if cur.index < 0 || cur.index >= len(g.slots) {
			return 0, 0, false
		}
		pt, found := c.pointAt(v, cur)
		val := 0.0
		if found {
			val = pt.y
		}
		x, y = g.xy(float64(cur.index), val)
		return x, y, true
	}
	pt, found := c.pointAt(v, cur)
	if !found {
		return 0, 0, false
	}
	x, y = g.xy(pt.x, pt.y)
	return x, y, true
}

// hoverMark returns the mark under the pointer: the slot, the series whose point is nearest,
// the slice, the vertex's axis, or the link. It is false when the pointer is over no mark.
func (c *Chart) hoverMark(v *view) (cursorAt, bool) {
	g := v.geo
	if !g.drawn || !v.pointer.on {
		return cursorAt{}, false
	}
	x, y := float64(v.pointer.x), float64(v.pointer.y)
	switch c.kind {
	case kindPie, kindDonut:
		r := math.Hypot(x-g.cx, y-g.cy)
		if r < g.inner || r > g.rad {
			return cursorAt{}, false
		}
		a := math.Atan2(y-g.cy, x-g.cx)
		for _, s := range g.slices {
			if inArc(a, s.a0, s.a1) {
				return cursorAt{on: true, series: 0, index: s.index}, true
			}
		}
		return cursorAt{}, false
	case kindRadar:
		return c.radarHover(v, x, y)
	case kindSankey:
		for i := len(g.links) - 1; i >= 0; i-- {
			if pointInPoly(g.links[i].poly, x, y) {
				return cursorAt{on: true, index: i}, true
			}
		}
		return cursorAt{}, false
	}
	if !g.plot.contains(x, y) {
		return cursorAt{}, false
	}
	return c.cartesianHover(v, x, y)
}

// cartesianHover returns the point under the pointer on a cartesian chart: on a band chart
// the slot nearest the pointer and the series whose bar it is or is nearest; otherwise the
// series whose nearest point is nearest in pixels.
func (c *Chart) cartesianHover(v *view, x, y float64) (cursorAt, bool) {
	g := v.geo
	pos, val := g.data(x, y)
	if g.band {
		if len(g.slots) == 0 {
			return cursorAt{}, false
		}
		slot := clampInt(int(math.Round(pos)), 0, len(g.slots)-1)
		best, bestD := -1, math.Inf(1)
		for j, k := range g.vis {
			pt, ok := c.pointAt(v, cursorAt{on: true, series: k, index: slot})
			if !ok {
				continue
			}
			d := math.Abs(val - pt.y)
			if c.stack && j < len(g.stackLo) {
				lo, hi := g.stackLo[j][slot], g.stackHi[j][slot]
				if val >= math.Min(lo, hi) && val <= math.Max(lo, hi) {
					d = 0
				}
			}
			if d < bestD {
				best, bestD = k, d
			}
		}
		if best < 0 {
			best = g.vis[0]
		}
		return cursorAt{on: true, series: best, index: slot}, true
	}
	best := cursorAt{}
	bestD := math.Inf(1)
	for _, k := range g.vis {
		pts := c.ds.series[k].pts
		if len(pts) == 0 {
			continue
		}
		i := nearestX(pts, pos)
		px, py := g.xy(pts[i].x, pts[i].y)
		if d := math.Hypot(px-x, py-y); d < bestD {
			best, bestD = cursorAt{on: true, series: k, index: i}, d
		}
	}
	return best, best.on
}

// radarHover returns the axis under the pointer on a radar chart, and the series whose
// vertex on that axis is nearest. It is false outside the outer ring.
func (c *Chart) radarHover(v *view, x, y float64) (cursorAt, bool) {
	g := v.geo
	if g.axes < 2 || math.Hypot(x-g.cx, y-g.cy) > g.rad+radarLabelRoom/2 {
		return cursorAt{}, false
	}
	a := math.Atan2(y-g.cy, x-g.cx) - startAngle
	step := 2 * math.Pi / float64(g.axes)
	axis := int(math.Round(a/step)) % g.axes
	if axis < 0 {
		axis += g.axes
	}
	best := cursorAt{}
	bestD := math.Inf(1)
	for _, k := range g.vis {
		vx, vy := c.radarVertex(v, k, axis)
		if d := math.Hypot(vx-x, vy-y); d < bestD {
			best, bestD = cursorAt{on: true, series: k, index: axis}, d
		}
	}
	return best, best.on
}

// paintCursor draws the pointer's mark, a highlight of the point under it, and the keyboard
// position's ring, when the chart is focused.
func (c *Chart) paintCursor(v *view, p *ui.Painter) {
	g := v.geo
	if !g.drawn {
		return
	}
	if cur, ok := c.hoverMark(v); ok {
		c.paintHover(v, p, cur)
	}
	if v.focused && v.cursor.on {
		c.paintFocus(v, p, v.cursor)
	}
}

// paintHover highlights the mark under the pointer: a slot's band on a band chart, a
// crosshair on a line, scatter or area, an outline on a pie slice or a Sankey link, and a
// ring on a radar vertex.
func (c *Chart) paintHover(v *view, p *ui.Painter, cur cursorAt) {
	g := v.geo
	switch c.kind {
	case kindPie, kindDonut:
		for _, s := range g.slices {
			if s.index == cur.index {
				shape := slicePath(g.cx, g.cy, g.rad, g.inner, s.a0, s.a1)
				p.Stroke(shape, ui.StrokeStyle{Width: hoverWidth, Color: v.t.Text})
			}
		}
	case kindSankey:
		if cur.index >= 0 && cur.index < len(g.links) {
			p.Stroke(polyPath(g.links[cur.index].poly), ui.StrokeStyle{Width: hoverWidth, Color: v.t.Text})
		}
	case kindRadar:
		if x, y, ok := c.cursorXY(v, cur); ok {
			drawRing(p, x, y, 4, v.t.Text, 1)
		}
	default:
		if g.band {
			if cur.index >= 0 && cur.index < len(g.slots) {
				sw := g.slotWidth()
				center := g.pos.at(float64(cur.index))
				band := ui.NewPath()
				if g.vert {
					band.Rect(float32(center-sw/2), float32(g.plot.y0), float32(sw), float32(g.plot.y1-g.plot.y0))
				} else {
					band.Rect(float32(g.plot.x0), float32(center-sw/2), float32(g.plot.x1-g.plot.x0), float32(sw))
				}
				p.Fill(band, v.t.Text.Alpha(slotHighlight))
			}
			return
		}
		if x, y, ok := c.cursorXY(v, cur); ok {
			line := ui.NewPath()
			if g.vert {
				line.MoveTo(float32(x), float32(g.plot.y0)).LineTo(float32(x), float32(g.plot.y1))
			} else {
				line.MoveTo(float32(g.plot.x0), float32(y)).LineTo(float32(g.plot.x1), float32(y))
			}
			p.Stroke(line, ui.StrokeStyle{Width: 1, Color: v.t.TextMuted, Dash: []float32{crosshairDash, crosshairDash}})
		}
	}
}

// paintFocus draws the keyboard position's ring: round on a point, slice or vertex, and an
// outline on a Sankey link or a bar's slot. It is the visible focus of the chart's walk.
func (c *Chart) paintFocus(v *view, p *ui.Painter, cur cursorAt) {
	g := v.geo
	if c.kind == kindSankey {
		if cur.index >= 0 && cur.index < len(g.links) {
			p.Stroke(polyPath(g.links[cur.index].poly), ui.StrokeStyle{Width: focusRingWidth, Color: v.t.FocusRing})
		}
		return
	}
	if (c.kind == kindPie || c.kind == kindDonut) && cur.index >= 0 {
		for _, s := range g.slices {
			if s.index == cur.index {
				shape := slicePath(g.cx, g.cy, g.rad, g.inner, s.a0, s.a1)
				p.Stroke(shape, ui.StrokeStyle{Width: focusRingWidth, Color: v.t.FocusRing})
			}
		}
		return
	}
	if x, y, ok := c.cursorXY(v, cur); ok {
		drawRing(p, x, y, focusRadius, v.t.FocusRing, focusRingWidth)
	}
}

// drawRing strokes a ring of radius r about x, y.
func drawRing(p *ui.Painter, x, y, r float64, color ui.Color, width float32) {
	p.Stroke(ui.NewPath().Circle(float32(x), float32(y), float32(r)), ui.StrokeStyle{Width: width, Color: color})
}

// tooltip returns the tooltip of the mark that the pointer or the keyboard is on: a box
// laid at the pointer, or at the keyboard position, with the position's label and its
// values. It is nil when there is no such mark, or the chart is not yet drawn.
func (c *Chart) tooltip(v *view) ui.Element {
	if !v.geo.drawn {
		return nil
	}
	var cur cursorAt
	var x, y float64
	switch {
	case v.pointer.on:
		mark, ok := c.hoverMark(v)
		if !ok {
			return nil
		}
		cur, x, y = mark, float64(v.pointer.x), float64(v.pointer.y)
	case v.focused && v.cursor.on:
		px, py, ok := c.cursorXY(v, v.cursor)
		if !ok {
			return nil
		}
		cur, x, y = v.cursor, px, py
	default:
		return nil
	}
	header, rows := c.tipContent(v, cur)
	return tooltipAt(v, x, y, header, rows)
}

// tooltipAt returns a tooltip box with its header and rows, placed below the point x, y. The
// box is an overlay on a one-pixel anchor at the point, so it is kept inside the window.
func tooltipAt(v *view, x, y float64, header string, rows []tipRow) ui.Element {
	box := ui.Div().Role(access.RoleTooltip).Column().Style(v.st.Tooltip)
	names := []string{header}
	box.Child(ui.Text(header).NoWrap().Semibold())
	for _, r := range rows {
		line := ui.Div().Row().Align(ui.Center).Gap(6)
		line.Child(ui.Div().Style(ui.Style{Width: ui.Px(8), Height: ui.Px(8), NoShrink: true, Radius: 4,
			Background: r.color}))
		line.Child(ui.Text(r.text).NoWrap())
		box.Child(line)
		names = append(names, r.text)
	}
	box.Label(strings.Join(names, ", "))
	box.Overlay(ui.BelowStart).OverlayGap(8)
	return ui.Div().Absolute().Left(float32(x)).Top(float32(y)).Size(ui.Px(1), ui.Px(1)).Child(box)
}

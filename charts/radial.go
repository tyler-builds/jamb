package charts

import (
	"math"

	"github.com/tyler-builds/jamb/ui"
)

// donutHole is the inner radius of a donut, as a share of the pie's outer radius.
const donutHole = 0.6

// radarLabelRoom is the space between a radar chart's outer ring and its axis labels.
const radarLabelRoom = 34

// startAngle is where the first slice of a pie, and the first axis of a radar chart,
// starts: at the top, which is -π/2 in the canvas's coordinates.
const startAngle = -math.Pi / 2

// paintPie draws a pie or a donut chart: one slice for each visible point, in order,
// clockwise from the top, sized by the points' values. Each slice has its series' color
// (its point's, by its index) and a separator in the surface color; under high contrast
// it is hatched and outlined. A donut shows the total in its hole.
func (c *Chart) paintPie(v *view, p *ui.Painter, w, h float32) {
	g := v.geo
	*g = geometry{w: float64(w), h: float64(h), vis: v.visible(), drawn: true}
	if len(c.ds.series) == 0 {
		return
	}
	pts := c.ds.series[0].pts
	total := 0.0
	for _, i := range g.vis {
		total += math.Max(pts[i].y, 0)
	}
	g.cx, g.cy = float64(w)/2, float64(h)/2
	g.rad = math.Max(math.Min(float64(w), float64(h))/2-8, 4)
	if c.kind == kindDonut {
		g.inner = g.rad * donutHole
	}
	r := g.rad * float64(v.progress)
	ri := g.inner * float64(v.progress)
	a := startAngle
	box := rect{x0: g.cx - g.rad, y0: g.cy - g.rad, x1: g.cx + g.rad, y1: g.cy + g.rad}
	for _, i := range g.vis {
		if pts[i].y <= 0 || total <= 0 {
			continue
		}
		b := a + 2*math.Pi*pts[i].y/total
		g.slices = append(g.slices, sliceGeo{a0: a, a1: b, index: i})
		shape := slicePath(g.cx, g.cy, r, ri, a, b)
		l := seriesLook(v.t, i)
		if v.t.HighContrast {
			fillHatched(p, v.t, shape, l, box)
		} else {
			p.Fill(shape, l.color)
			p.Stroke(shape, ui.StrokeStyle{Width: 2, Color: v.t.Surface, Join: ui.JoinRound})
		}
		a = b
	}
	if c.kind == kindDonut && total > 0 && v.progress >= 1 {
		p.Text(float32(g.cx), float32(g.cy), c.valueLabel(total), ui.TextFormat{
			FontSize: axisFont(v.st) + 4, Color: v.t.Text, Align: ui.Center, VAlign: ui.Center})
	}
}

// slicePath returns the outline of the slice from the angle a0 to a1 about cx, cy: an
// arc at the radius r, and for a donut the arc back at the inner radius ri. A slice
// with no inner radius is a wedge from the center.
func slicePath(cx, cy, r, ri, a0, a1 float64) *ui.Path {
	path := ui.NewPath()
	if ri <= 0 {
		path.MoveTo(float32(cx), float32(cy))
		path.LineTo(float32(cx+r*math.Cos(a0)), float32(cy+r*math.Sin(a0)))
		path.Arc(float32(cx), float32(cy), float32(r), float32(a0), float32(a1))
		return path.Close()
	}
	path.MoveTo(float32(cx+r*math.Cos(a0)), float32(cy+r*math.Sin(a0)))
	path.Arc(float32(cx), float32(cy), float32(r), float32(a0), float32(a1))
	path.LineTo(float32(cx+ri*math.Cos(a1)), float32(cy+ri*math.Sin(a1)))
	path.Arc(float32(cx), float32(cy), float32(ri), float32(a1), float32(a0))
	return path.Close()
}

// inArc reports whether the angle a lies in the arc from a0 to a1, clockwise, in
// radians.
func inArc(a, a0, a1 float64) bool {
	d := math.Mod(a-a0, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return d <= a1-a0
}

// paintRadar draws a radar chart: a ring of gridlines, a spoke for each axis (one for
// each position of the series), and a polygon for each visible series through its values.
// The values are scaled to the largest of them, rounded up to a nice tick.
func (c *Chart) paintRadar(v *view, p *ui.Painter, w, h float32) {
	g := v.geo
	vis := v.visible()
	*g = geometry{w: float64(w), h: float64(h), vis: vis, drawn: true}
	slots := c.ds.slots(vis)
	g.slots = slots
	g.axes = len(slots)
	g.cx, g.cy = float64(w)/2, float64(h)/2
	g.rad = math.Max(math.Min(float64(w), float64(h))/2-radarLabelRoom, 8)
	if len(slots) < 2 {
		return
	}
	vmax := 0.0
	for _, k := range vis {
		vmax = math.Max(vmax, c.ds.series[k].hi)
	}
	_, _, top := linearTicks(0, vmax, 4)
	g.ringMax = top
	if top <= 0 {
		top = 1
	}
	axisAngle := func(i int) float64 { return startAngle + float64(i)*2*math.Pi/float64(len(slots)) }

	grid := v.st.Grid.BorderColor
	for ring := 1; ring <= 4; ring++ {
		r := g.rad * float64(ring) / 4
		path := ui.NewPath()
		for i := range slots {
			x, y := g.cx+r*math.Cos(axisAngle(i)), g.cy+r*math.Sin(axisAngle(i))
			if i == 0 {
				path.MoveTo(float32(x), float32(y))
			} else {
				path.LineTo(float32(x), float32(y))
			}
		}
		p.Stroke(path.Close(), ui.StrokeStyle{Width: 1, Color: grid})
	}
	spokes := ui.NewPath()
	for i := range slots {
		a := axisAngle(i)
		spokes.MoveTo(float32(g.cx), float32(g.cy)).LineTo(float32(g.cx+g.rad*math.Cos(a)), float32(g.cy+g.rad*math.Sin(a)))
	}
	p.Stroke(spokes, ui.StrokeStyle{Width: 1, Color: grid})
	size := axisFont(v.st)
	for i, s := range slots {
		a := axisAngle(i)
		lx := g.cx + (g.rad+10)*math.Cos(a)
		ly := g.cy + (g.rad+10)*math.Sin(a)
		align, valign := ui.Center, ui.Center
		switch cs := math.Cos(a); {
		case cs > 0.2:
			align = ui.Start
		case cs < -0.2:
			align = ui.End
		}
		switch sn := math.Sin(a); {
		case sn > 0.2:
			valign = ui.Start
		case sn < -0.2:
			valign = ui.End
		}
		p.Text(float32(lx), float32(ly), c.positionLabel(s), ui.TextFormat{FontSize: size,
			Color: v.st.Axis.TextColor, Align: align, VAlign: valign})
	}

	for _, k := range vis {
		l := seriesLook(v.t, k)
		vals := slotValues(c.ds.series[k].pts, slots)
		path := ui.NewPath()
		for i, val := range vals {
			r := g.rad * float64(v.progress) * math.Max(val, 0) / top
			x, y := g.cx+r*math.Cos(axisAngle(i)), g.cy+r*math.Sin(axisAngle(i))
			if i == 0 {
				path.MoveTo(float32(x), float32(y))
			} else {
				path.LineTo(float32(x), float32(y))
			}
		}
		path.Close()
		if v.t.HighContrast {
			fillHatched(p, v.t, path, l, rect{g.cx - g.rad, g.cy - g.rad, g.cx + g.rad, g.cy + g.rad})
		} else {
			p.Fill(path, l.color.Alpha(0.18))
		}
		p.Stroke(path, ui.StrokeStyle{Width: lineWidth, Color: l.color, Dash: l.dash, Join: ui.JoinRound})
	}
}

// radarVertex returns the pixel position of series k's value at the axis i of a radar
// chart, as the draw places it.
func (c *Chart) radarVertex(v *view, k, i int) (x, y float64) {
	g := v.geo
	val := slotValue(c.ds.series[k].pts, g.slots, i)
	top := g.ringMax
	if top <= 0 {
		top = 1
	}
	r := g.rad * math.Max(val, 0) / top
	a := startAngle + float64(i)*2*math.Pi/float64(len(g.slots))
	return g.cx + r*math.Cos(a), g.cy + r*math.Sin(a)
}

// slotValue returns the value of the points at the slot i of slots, or zero when the
// series has no point there.
func slotValue(pts []point, slots []float64, i int) float64 {
	if i < 0 || i >= len(slots) {
		return 0
	}
	if j, ok := findX(pts, slots[i]); ok {
		return pts[j].y
	}
	return 0
}

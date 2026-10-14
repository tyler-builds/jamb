package charts

import "math"

// geometry is the layout of a chart's last draw, in its canvas's logical pixels.
// The draw leaves it for the chart's handlers, which map the pointer to the points
// and slices that the draw placed.
type geometry struct {
	w, h float64
	// plot is the area inside the axes, where the marks are drawn.
	plot rect
	// vert is set when the positions run along the x axis, as in every chart but a
	// horizontal bar chart, where they run down the y axis.
	vert bool
	// pos maps positions to pixels along the position axis, and val maps values
	// to pixels along the value axis.
	pos, val scale
	// vis is the visible series, or for a pie the visible slices, by index.
	vis []int
	// band is set when each position is a slot of a bar or candlestick chart, and
	// slots holds the positions of the slots of a band chart and of a radar chart.
	band  bool
	slots []float64
	// stackLo and stackHi are the extents of each visible series at each slot, for a
	// stacked bar chart, so that the hit test finds the bar under the pointer.
	stackLo, stackHi [][]float64
	// slices and the center and radii are a pie's; cx, cy, rad and inner are also a
	// radar's center and outer radius. ringMax is the value at a radar chart's outer
	// ring, and axes is its number of axes.
	slices     []sliceGeo
	cx, cy     float64
	rad, inner float64
	axes       int
	ringMax    float64
	// links are a Sankey diagram's link outlines.
	links []linkGeo
	// drawn is set by the first draw, so that a chart not yet drawn has no marks to hit.
	drawn bool
}

// rect is a rectangle in logical pixels, from its top-left corner x0, y0 to its
// bottom-right corner x1, y1.
type rect struct{ x0, y0, x1, y1 float64 }

// contains reports whether the point x, y is inside r.
func (r rect) contains(x, y float64) bool {
	return x >= r.x0 && x <= r.x1 && y >= r.y0 && y <= r.y1
}

// sliceGeo is the angle range of a pie slice, in radians from the top clockwise, and
// the index of the slice's point.
type sliceGeo struct {
	a0, a1 float64
	index  int
}

// linkGeo is the outline of a Sankey link: its polygon in pixels, and the index of
// the link.
type linkGeo struct {
	poly  []float64 // x0, y0, x1, y1, ... around the band
	index int
}

// xy maps a position and a value to pixels.
func (g *geometry) xy(position, value float64) (x, y float64) {
	if g.vert {
		return g.pos.at(position), g.val.at(value)
	}
	return g.val.at(value), g.pos.at(position)
}

// data maps pixels to a position and a value: the inverse of xy.
func (g *geometry) data(x, y float64) (position, value float64) {
	if g.vert {
		return g.pos.value(x), g.val.value(y)
	}
	return g.pos.value(y), g.val.value(x)
}

// slotWidth returns the width in pixels of one slot of a band chart.
func (g *geometry) slotWidth() float64 {
	if len(g.slots) == 0 {
		return 0
	}
	return math.Abs(g.pos.at(1) - g.pos.at(0))
}

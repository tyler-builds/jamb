package charts

import "github.com/tyler-builds/jamb/ui"

// lightColors are the series colors on a light surface, in order. Each is at
// least 3:1 against the surface's white, as the test of the palette checks.
var lightColors = []ui.Color{
	ui.Hex("#2563eb"), ui.Hex("#dc2626"), ui.Hex("#059669"), ui.Hex("#b45309"),
	ui.Hex("#7c3aed"), ui.Hex("#db2777"), ui.Hex("#0e7490"), ui.Hex("#57534e"),
}

// darkColors are the series colors on the dark surface, in order, each at least
// 3:1 against it.
var darkColors = []ui.Color{
	ui.Hex("#60a5fa"), ui.Hex("#f87171"), ui.Hex("#34d399"), ui.Hex("#fbbf24"),
	ui.Hex("#a78bfa"), ui.Hex("#f472b6"), ui.Hex("#22d3ee"), ui.Hex("#a1a1aa"),
}

// dashes are the dash patterns of the series' lines, in order. The first is solid.
var dashes = [][]float32{nil, {6, 3}, {2, 3}, {8, 3, 2, 3}, {12, 4}, {3, 3, 9, 3}}

// marker is the shape drawn at each point of a series' line.
type marker uint8

const (
	markCircle marker = iota
	markSquare
	markTriangle
	markDiamond
	markPlus
	markCross
	markerKinds
)

// hatch is the pattern that fills a bar, an area or a slice under high contrast,
// where colors are the system colors and the pattern tells the series apart.
// hatchSolid is a solid fill.
type hatch uint8

const (
	hatchSolid hatch = iota
	hatchForward
	hatchBack
	hatchHorizontal
	hatchVertical
	hatchCross
)

// look is how one series is drawn: its color, its line's dash pattern, its
// marker and its hatch. Series differ in more than color, so they are told apart
// without it.
type look struct {
	color  ui.Color
	dash   []float32
	marker marker
	hatch  hatch
}

// seriesLook returns the look of series i in the theme t. Under high contrast every
// series is the text color, and the dash, marker and hatch tell them apart.
func seriesLook(t *ui.Theme, i int) look {
	l := look{
		dash:   dashes[i%len(dashes)],
		marker: marker(i % int(markerKinds)),
		hatch:  hatch(i % int(hatchCross+1)),
	}
	switch {
	case t.HighContrast:
		l.color = t.Text
	case t.Dark:
		l.color = darkColors[i%len(darkColors)]
	default:
		l.color = lightColors[i%len(lightColors)]
	}
	return l
}

// lookOf returns the look of legend item i of the chart in the theme t: the palette's
// (see seriesLook), with the series' Color in place of the palette's color when the
// app sets one. Under high contrast the app's colors are ignored, and the slices of a
// pie and the nodes of a Sankey diagram always take the palette. The other series keep
// the palette colors of their places, whichever series set their own.
func (c *Chart) lookOf(t *ui.Theme, i int) look {
	l := seriesLook(t, i)
	if t.HighContrast || c.kind == kindPie || c.kind == kindDonut || c.kind == kindSankey || i >= len(c.ds.series) {
		return l
	}
	if color := c.ds.series[i].color; !color.IsZero() {
		l.color = color
	}
	return l
}

// candleColor returns the color of a candle in the theme t: the CandleSeries' Rising or
// Falling color when the app sets it, else the theme's Success or Danger. Under high
// contrast every candle is the text color, and its fill tells rising from falling.
func (c *Chart) candleColor(t *ui.Theme, up bool) ui.Color {
	switch {
	case t.HighContrast:
		return t.Text
	case up && !c.ds.rising.IsZero():
		return c.ds.rising
	case up:
		return t.Success
	case !c.ds.falling.IsZero():
		return c.ds.falling
	}
	return t.Danger
}

// drawMarker draws the marker m of radius r about x, y in color c. Plus and cross
// are strokes, so they are drawn with their weight; the others are filled.
func drawMarker(p *ui.Painter, m marker, x, y, r float32, c ui.Color) {
	switch m {
	case markSquare:
		p.FillRect(x-r, y-r, 2*r, 2*r, c)
	case markTriangle:
		p.Fill(ui.NewPath().MoveTo(x, y-r).LineTo(x+r, y+r).LineTo(x-r, y+r).Close(), c)
	case markDiamond:
		p.Fill(ui.NewPath().MoveTo(x, y-r).LineTo(x+r, y).LineTo(x, y+r).LineTo(x-r, y).Close(), c)
	case markPlus:
		p.Stroke(ui.NewPath().MoveTo(x-r, y).LineTo(x+r, y).MoveTo(x, y-r).LineTo(x, y+r), ui.StrokeStyle{Width: 2, Color: c})
	case markCross:
		p.Stroke(ui.NewPath().MoveTo(x-r, y-r).LineTo(x+r, y+r).MoveTo(x+r, y-r).LineTo(x-r, y+r), ui.StrokeStyle{Width: 2, Color: c})
	default:
		p.Fill(ui.NewPath().Circle(x, y, r), c)
	}
}

// hatchSpacing is the distance between the lines of a hatch, in logical pixels.
const hatchSpacing = 6

// hatchFill fills the shape with the pattern h in the color c, inside the box x0,
// y0 to x1, y1, which the shape lies in. A solid pattern fills the shape with c.
func hatchFill(p *ui.Painter, shape *ui.Path, h hatch, c ui.Color, x0, y0, x1, y1 float32) {
	if h == hatchSolid {
		p.Fill(shape, c)
		return
	}
	p.Save()
	p.Clip(shape)
	lines := ui.NewPath()
	height := y1 - y0
	if h == hatchForward || h == hatchCross {
		// Lines that run up to the right, from the bottom edge to the top edge.
		for x := x0 - height; x <= x1; x += hatchSpacing {
			lines.MoveTo(x, y1).LineTo(x+height, y0)
		}
	}
	if h == hatchBack || h == hatchCross {
		// Lines that run up to the left.
		for x := x0; x <= x1+height; x += hatchSpacing {
			lines.MoveTo(x, y1).LineTo(x-height, y0)
		}
	}
	if h == hatchHorizontal || h == hatchCross {
		for y := y0; y <= y1; y += hatchSpacing {
			lines.MoveTo(x0, y).LineTo(x1, y)
		}
	}
	if h == hatchVertical || h == hatchCross {
		for x := x0; x <= x1; x += hatchSpacing {
			lines.MoveTo(x, y0).LineTo(x, y1)
		}
	}
	p.Stroke(lines, ui.StrokeStyle{Width: 1, Color: c})
	p.Restore()
}

// Package icons provides a small set of line icons defined in Go on a 24×24
// grid (2-unit strokes, round caps and joins — the familiar "Lucide" look)
// and a rasterizer that turns them into anti-aliased coverage masks.
//
// This is deliberately not an SVG engine: an icon is a list of polylines,
// circles and filled polygons built with a tiny typed API.
package icons

import (
	"image"
	"math"
	"sync"
)

// Icon is a vector icon on a 24×24 grid.
type Icon struct {
	// Name is the name of the icon in its icon set.
	Name    string
	strokes [][]pt // open or closed polylines, stroked
	fills   [][]pt // closed polygons, filled
	// StrokeWidth in grid units; default 2.
	StrokeWidth float32

	// build makes the shapes; it runs once, on the first Rasterize, so that an
	// app pays for the icons it draws and not for the whole set at start.
	build func(b *builder)
	once  sync.Once
}

type pt struct{ x, y float32 }

// builder accumulates the shapes of one icon.
type builder struct {
	ic  *Icon
	cur []pt
}

// makeShapes builds the shapes of the icon, once. A zero Icon has none.
func (ic *Icon) makeShapes() {
	ic.once.Do(func() {
		if ic.build != nil {
			b := &builder{ic: ic}
			ic.build(b)
			b.end()
		}
		ic.build = nil
	})
}

func (b *builder) end() {
	if len(b.cur) > 1 {
		b.ic.strokes = append(b.ic.strokes, b.cur)
	}
	b.cur = nil
}

// M starts a new polyline.
func (b *builder) M(x, y float32) *builder { b.end(); b.cur = []pt{{x, y}}; return b }

// L adds a line segment.
func (b *builder) L(x, y float32) *builder { b.cur = append(b.cur, pt{x, y}); return b }

// Z closes the current polyline.
func (b *builder) Z() *builder {
	if len(b.cur) > 0 {
		b.cur = append(b.cur, b.cur[0])
	}
	b.end()
	return b
}

// Line is a single segment.
func (b *builder) Line(x0, y0, x1, y1 float32) *builder { return b.M(x0, y0).L(x1, y1) }

// Arc appends an arc around (cx, cy) from angle a0 to a1 (degrees, 0 = +x,
// clockwise because y points down), continuing the current polyline.
func (b *builder) Arc(cx, cy, r, a0, a1 float32) *builder {
	steps := int(math.Ceil(float64(abs(a1-a0)) / 10))
	steps = max(steps, 2)
	for i := 0; i <= steps; i++ {
		a := float64(a0+(a1-a0)*float32(i)/float32(steps)) * math.Pi / 180
		b.cur = append(b.cur, pt{cx + r*float32(math.Cos(a)), cy + r*float32(math.Sin(a))})
	}
	return b
}

// Circle strokes a full circle.
func (b *builder) Circle(cx, cy, r float32) *builder {
	b.end()
	b.Arc(cx, cy, r, 0, 360)
	b.end()
	return b
}

// Dot fills a small disc.
func (b *builder) Dot(cx, cy, r float32) *builder {
	b.end()
	var poly []pt
	for i := 0; i < 24; i++ {
		a := float64(i) / 24 * 2 * math.Pi
		poly = append(poly, pt{cx + r*float32(math.Cos(a)), cy + r*float32(math.Sin(a))})
	}
	b.ic.fills = append(b.ic.fills, poly)
	return b
}

// RoundRect strokes a rounded rectangle.
func (b *builder) RoundRect(x, y, w, h, r float32) *builder {
	b.end()
	b.cur = nil
	b.Arc(x+w-r, y+r, r, -90, 0)
	b.Arc(x+w-r, y+h-r, r, 0, 90)
	b.Arc(x+r, y+h-r, r, 90, 180)
	b.Arc(x+r, y+r, r, 180, 270)
	return b.Z()
}

// Poly fills a closed polygon.
func (b *builder) Poly(coords ...float32) *builder {
	b.end()
	var poly []pt
	for i := 0; i+1 < len(coords); i += 2 {
		poly = append(poly, pt{coords[i], coords[i+1]})
	}
	b.ic.fills = append(b.ic.fills, poly)
	return b
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// Rasterize draws the icon into dst, scaled to fill dst's bounds. Coverage is
// computed analytically from the distance to each stroke, which gives clean
// round caps and joins at any size.
func (ic *Icon) Rasterize(dst *image.Alpha) {
	ic.makeShapes()
	b := dst.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := float32(w) / 24
	half := ic.StrokeWidth * scale / 2
	// Very small icons look washed out with exact coverage; keep strokes at
	// least about one device pixel wide.
	half = max(half, 0.55)
	type seg struct{ ax, ay, bx, by float32 }
	var segs []seg
	minX, minY, maxX, maxY := float32(1e9), float32(1e9), float32(-1e9), float32(-1e9)
	for _, s := range ic.strokes {
		for i := 0; i+1 < len(s); i++ {
			a, c := s[i], s[i+1]
			sg := seg{a.x * scale, a.y * scale, c.x * scale, c.y * scale}
			segs = append(segs, sg)
			minX, maxX = min(minX, sg.ax, sg.bx), max(maxX, sg.ax, sg.bx)
			minY, maxY = min(minY, sg.ay, sg.by), max(maxY, sg.ay, sg.by)
		}
	}
	fills := make([][]pt, len(ic.fills))
	for i, f := range ic.fills {
		for _, p := range f {
			fills[i] = append(fills[i], pt{p.x * scale, p.y * scale})
		}
	}
	for py := 0; py < h; py++ {
		y := float32(py) + 0.5
		for px := 0; px < w; px++ {
			x := float32(px) + 0.5
			cov := float32(0)
			if len(segs) > 0 && x >= minX-half-1 && x <= maxX+half+1 && y >= minY-half-1 && y <= maxY+half+1 {
				d := float32(1e9)
				for _, s := range segs {
					d = min(d, segDist(x, y, s.ax, s.ay, s.bx, s.by))
				}
				cov = clamp01(half + 0.5 - d)
			}
			for _, f := range fills {
				cov = max(cov, polyCoverage(x, y, f))
			}
			if cov > 0 {
				dst.Pix[py*dst.Stride+px] = uint8(cov*255 + 0.5)
			}
		}
	}
}

func clamp01(v float32) float32 { return min(max(v, 0), 1) }

func segDist(px, py, ax, ay, bx, by float32) float32 {
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	t := float32(0)
	if l2 > 0 {
		t = clamp01(((px-ax)*dx + (py-ay)*dy) / l2)
	}
	ex, ey := px-(ax+t*dx), py-(ay+t*dy)
	return float32(math.Sqrt(float64(ex*ex + ey*ey)))
}

func polyCoverage(x, y float32, poly []pt) float32 {
	inside := false
	d := float32(1e9)
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a.y > y) != (b.y > y) && x < (b.x-a.x)*(y-a.y)/(b.y-a.y)+a.x {
			inside = !inside
		}
		d = min(d, segDist(x, y, a.x, a.y, b.x, b.y))
	}
	if inside {
		return clamp01(0.5 + d)
	}
	return clamp01(0.5 - d)
}

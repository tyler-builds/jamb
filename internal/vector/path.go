// Package vector draws paths into pixels on the CPU: fills with the nonzero
// or even-odd rule, strokes with joins, caps and dashes, solid colors and
// linear and radial gradients, clipping and group opacity. The ui package's
// Canvas and SVG are drawn with it, into an image the renderer shows.
//
// Coordinates are float32 pixels, y down. Colors are straight (not
// premultiplied) R, G, B, A from 0 to 1; the image drawn into is
// premultiplied.
package vector

import "math"

// Point is a position.
type Point struct{ X, Y float32 }

func (p Point) add(q Point) Point             { return Point{p.X + q.X, p.Y + q.Y} }
func (p Point) sub(q Point) Point             { return Point{p.X - q.X, p.Y - q.Y} }
func (p Point) mul(s float32) Point           { return Point{p.X * s, p.Y * s} }
func (p Point) dot(q Point) float32           { return p.X*q.X + p.Y*q.Y }
func (p Point) cross(q Point) float32         { return p.X*q.Y - p.Y*q.X }
func (p Point) len() float32                  { return float32(math.Hypot(float64(p.X), float64(p.Y))) }
func (p Point) lerp(q Point, t float32) Point { return p.add(q.sub(p).mul(t)) }

type verb uint8

const (
	verbMove verb = iota
	verbLine
	verbQuad
	verbCubic
	verbClose
)

// Path is a sequence of subpaths made of lines and Bézier curves. The zero
// value is an empty path.
type Path struct {
	verbs      []verb
	pts        []Point
	start, cur Point
	hasCur     bool
}

// Empty reports whether the path has nothing to draw.
func (p *Path) Empty() bool { return len(p.verbs) == 0 }

// Current returns the end of the path so far, and false before a MoveTo.
func (p *Path) Current() (Point, bool) { return p.cur, p.hasCur }

// MoveTo starts a new subpath at x, y.
func (p *Path) MoveTo(x, y float32) {
	p.verbs = append(p.verbs, verbMove)
	p.pts = append(p.pts, Point{x, y})
	p.start, p.cur, p.hasCur = Point{x, y}, Point{x, y}, true
}

// ensure starts a subpath at x, y when there is no current point, as a line
// or curve drawn first does in the HTML canvas.
func (p *Path) ensure(x, y float32) {
	if !p.hasCur {
		p.MoveTo(x, y)
	}
}

// LineTo adds a straight line to x, y.
func (p *Path) LineTo(x, y float32) {
	if !p.hasCur {
		p.MoveTo(x, y)
		return
	}
	p.verbs = append(p.verbs, verbLine)
	p.pts = append(p.pts, Point{x, y})
	p.cur = Point{x, y}
}

// QuadTo adds a quadratic Bézier curve through the control point cx, cy to
// x, y.
func (p *Path) QuadTo(cx, cy, x, y float32) {
	p.ensure(cx, cy)
	p.verbs = append(p.verbs, verbQuad)
	p.pts = append(p.pts, Point{cx, cy}, Point{x, y})
	p.cur = Point{x, y}
}

// CubicTo adds a cubic Bézier curve with the control points c1 and c2 to
// x, y.
func (p *Path) CubicTo(c1x, c1y, c2x, c2y, x, y float32) {
	p.ensure(c1x, c1y)
	p.verbs = append(p.verbs, verbCubic)
	p.pts = append(p.pts, Point{c1x, c1y}, Point{c2x, c2y}, Point{x, y})
	p.cur = Point{x, y}
}

// Close closes the subpath with a line back to its start.
func (p *Path) Close() {
	if !p.hasCur || (len(p.verbs) > 0 && p.verbs[len(p.verbs)-1] == verbClose) {
		return
	}
	p.verbs = append(p.verbs, verbClose)
	p.cur = p.start
}

// Rect adds a closed rectangle.
func (p *Path) Rect(x, y, w, h float32) {
	p.MoveTo(x, y)
	p.LineTo(x+w, y)
	p.LineTo(x+w, y+h)
	p.LineTo(x, y+h)
	p.Close()
}

// kappa is the distance of the control points of a cubic quarter circle of
// radius 1 from its ends.
const kappa = 0.5522847498

// RoundRect adds a closed rectangle whose corners are quarter ellipses of
// radii rx, ry, which are limited to half the sides.
func (p *Path) RoundRect(x, y, w, h, rx, ry float32) {
	rx, ry = min(max(rx, 0), w/2), min(max(ry, 0), h/2)
	if rx == 0 || ry == 0 {
		p.Rect(x, y, w, h)
		return
	}
	kx, ky := rx*kappa, ry*kappa
	p.MoveTo(x+rx, y)
	p.LineTo(x+w-rx, y)
	p.CubicTo(x+w-rx+kx, y, x+w, y+ry-ky, x+w, y+ry)
	p.LineTo(x+w, y+h-ry)
	p.CubicTo(x+w, y+h-ry+ky, x+w-rx+kx, y+h, x+w-rx, y+h)
	p.LineTo(x+rx, y+h)
	p.CubicTo(x+rx-kx, y+h, x, y+h-ry+ky, x, y+h-ry)
	p.LineTo(x, y+ry)
	p.CubicTo(x, y+ry-ky, x+rx-kx, y, x+rx, y)
	p.Close()
}

// Ellipse adds a closed ellipse centered on cx, cy.
func (p *Path) Ellipse(cx, cy, rx, ry float32) {
	kx, ky := rx*kappa, ry*kappa
	p.MoveTo(cx+rx, cy)
	p.CubicTo(cx+rx, cy+ky, cx+kx, cy+ry, cx, cy+ry)
	p.CubicTo(cx-kx, cy+ry, cx-rx, cy+ky, cx-rx, cy)
	p.CubicTo(cx-rx, cy-ky, cx-kx, cy-ry, cx, cy-ry)
	p.CubicTo(cx+kx, cy-ry, cx+rx, cy-ky, cx+rx, cy)
	p.Close()
}

// Arc adds an arc of the circle centered on cx, cy, of radius r, from angle
// a0 to a1 in radians (0 is to the right, angles grow clockwise on screen),
// counterclockwise when ccw is set. As in the HTML canvas, a line joins the
// current point to the arc's start, and a sweep of a full turn or more draws
// the whole circle.
func (p *Path) Arc(cx, cy, r, a0, a1 float32, ccw bool) {
	sweep := float64(a1 - a0)
	const tau = 2 * math.Pi
	switch {
	case !ccw && sweep >= tau, ccw && sweep <= -tau:
		sweep = tau
		if ccw {
			sweep = -tau
		}
	case !ccw:
		sweep = math.Mod(sweep, tau)
		if sweep < 0 {
			sweep += tau
		}
	default:
		sweep = math.Mod(sweep, tau)
		if sweep > 0 {
			sweep -= tau
		}
	}
	start := Point{cx + r*float32(math.Cos(float64(a0))), cy + r*float32(math.Sin(float64(a0)))}
	if p.hasCur {
		p.LineTo(start.X, start.Y)
	} else {
		p.MoveTo(start.X, start.Y)
	}
	p.arcCubics(Point{cx, cy}, r, r, 0, float64(a0), sweep)
}

// arcCubics adds an elliptical arc, centered on c with radii rx, ry rotated by
// rot, from angle a0 by sweep (radians), as cubics of at most a quarter turn.
func (p *Path) arcCubics(c Point, rx, ry float32, rot, a0, sweep float64) {
	n := int(math.Ceil(math.Abs(sweep) / (math.Pi / 2)))
	if n == 0 {
		return
	}
	step := sweep / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	sr, cr := math.Sincos(rot)
	at := func(a float64) (Point, Point) {
		s, co := math.Sincos(a)
		// The point, and the derivative, of the unrotated ellipse; then rotated.
		x, y := float64(rx)*co, float64(ry)*s
		dx, dy := -float64(rx)*s, float64(ry)*co
		return Point{c.X + float32(x*cr-y*sr), c.Y + float32(x*sr+y*cr)},
			Point{float32(dx*cr - dy*sr), float32(dx*sr + dy*cr)}
	}
	a := a0
	p0, d0 := at(a)
	for range n {
		p1, d1 := at(a + step)
		p.CubicTo(p0.X+float32(k)*d0.X, p0.Y+float32(k)*d0.Y, p1.X-float32(k)*d1.X, p1.Y-float32(k)*d1.Y, p1.X, p1.Y)
		p0, d0 = p1, d1
		a += step
	}
}

// ArcTo adds an elliptical arc to x, y, as the SVG path command A does: radii
// rx, ry, the ellipse rotated by rotation degrees, and the flags that pick
// one of the four arcs that join the points.
func (p *Path) ArcTo(rx, ry, rotation float32, large, sweep bool, x, y float32) {
	p.ensure(x, y)
	p0 := p.cur
	if p0 == (Point{x, y}) {
		return
	}
	rx, ry = float32(math.Abs(float64(rx))), float32(math.Abs(float64(ry)))
	if rx == 0 || ry == 0 {
		p.LineTo(x, y)
		return
	}
	// The center parameterization of SVG's implementation notes (F.6.5).
	phi := float64(rotation) * math.Pi / 180
	sp, cp := math.Sincos(phi)
	dx, dy := float64(p0.X-x)/2, float64(p0.Y-y)/2
	x1 := cp*dx + sp*dy
	y1 := -sp*dx + cp*dy
	frx, fry := float64(rx), float64(ry)
	if l := x1*x1/(frx*frx) + y1*y1/(fry*fry); l > 1 {
		s := math.Sqrt(l)
		frx, fry = frx*s, fry*s
	}
	num := frx*frx*fry*fry - frx*frx*y1*y1 - fry*fry*x1*x1
	den := frx*frx*y1*y1 + fry*fry*x1*x1
	co := 0.0
	if den > 0 && num > 0 {
		co = math.Sqrt(num / den)
	}
	if large == sweep {
		co = -co
	}
	cx1, cy1 := co*frx*y1/fry, -co*fry*x1/frx
	cx := cp*cx1 - sp*cy1 + float64(p0.X+x)/2
	cy := sp*cx1 + cp*cy1 + float64(p0.Y+y)/2
	angle := func(ux, uy, vx, vy float64) float64 {
		a := math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
		return a
	}
	a0 := angle(1, 0, (x1-cx1)/frx, (y1-cy1)/fry)
	d := angle((x1-cx1)/frx, (y1-cy1)/fry, (-x1-cx1)/frx, (-y1-cy1)/fry)
	if !sweep && d > 0 {
		d -= 2 * math.Pi
	} else if sweep && d < 0 {
		d += 2 * math.Pi
	}
	p.arcCubics(Point{float32(cx), float32(cy)}, float32(frx), float32(fry), phi, a0, d)
	p.cur = Point{x, y} // exactly, whatever the rounding of the last cubic
	p.pts[len(p.pts)-1] = p.cur
}

// Bounds returns the box of the path's points, control points included, and
// false for an empty path.
func (p *Path) Bounds() (x0, y0, x1, y1 float32, ok bool) {
	if len(p.pts) == 0 {
		return 0, 0, 0, 0, false
	}
	x0, y0, x1, y1 = p.pts[0].X, p.pts[0].Y, p.pts[0].X, p.pts[0].Y
	for _, pt := range p.pts[1:] {
		x0, y0, x1, y1 = min(x0, pt.X), min(y0, pt.Y), max(x1, pt.X), max(y1, pt.Y)
	}
	return x0, y0, x1, y1, true
}

// Transform returns the path with every point transformed by m.
func (p *Path) Transform(m Matrix) *Path {
	q := &Path{verbs: append([]verb(nil), p.verbs...), pts: make([]Point, len(p.pts)), hasCur: p.hasCur}
	for i, pt := range p.pts {
		q.pts[i] = m.Apply(pt)
	}
	q.start, q.cur = m.Apply(p.start), m.Apply(p.cur)
	return q
}

// Append adds the subpaths of o to p.
func (p *Path) Append(o *Path) {
	p.verbs = append(p.verbs, o.verbs...)
	p.pts = append(p.pts, o.pts...)
	if o.hasCur {
		p.start, p.cur, p.hasCur = o.start, o.cur, true
	}
}

// Matrix is a 2D affine transform: a point x, y becomes
// A*x + C*y + E, B*x + D*y + F (the SVG and canvas matrix(a b c d e f)).
type Matrix struct{ A, B, C, D, E, F float32 }

// Identity is the transform that changes nothing.
var Identity = Matrix{A: 1, D: 1}

// Translate, Scale and Rotate return the transforms that move by x, y, scale
// by sx, sy and rotate by angle radians (clockwise on screen).
func Translate(x, y float32) Matrix { return Matrix{A: 1, D: 1, E: x, F: y} }
func Scale(sx, sy float32) Matrix   { return Matrix{A: sx, D: sy} }
func Rotate(angle float32) Matrix {
	s, c := math.Sincos(float64(angle))
	return Matrix{A: float32(c), B: float32(s), C: float32(-s), D: float32(c)}
}

// Skew returns the transform that skews x by ax and y by ay radians.
func Skew(ax, ay float32) Matrix {
	return Matrix{A: 1, B: float32(math.Tan(float64(ay))), C: float32(math.Tan(float64(ax))), D: 1}
}

// Then returns the transform that applies m, then n.
func (m Matrix) Then(n Matrix) Matrix {
	return Matrix{
		A: n.A*m.A + n.C*m.B, B: n.B*m.A + n.D*m.B,
		C: n.A*m.C + n.C*m.D, D: n.B*m.C + n.D*m.D,
		E: n.A*m.E + n.C*m.F + n.E, F: n.B*m.E + n.D*m.F + n.F,
	}
}

// Apply transforms p.
func (m Matrix) Apply(p Point) Point {
	return Point{m.A*p.X + m.C*p.Y + m.E, m.B*p.X + m.D*p.Y + m.F}
}

// Invert returns the inverse transform, and false when m flattens the plane.
func (m Matrix) Invert() (Matrix, bool) {
	det := m.A*m.D - m.B*m.C
	if det == 0 || math.IsNaN(float64(det)) {
		return Matrix{}, false
	}
	return Matrix{
		A: m.D / det, B: -m.B / det, C: -m.C / det, D: m.A / det,
		E: (m.C*m.F - m.D*m.E) / det, F: (m.B*m.E - m.A*m.F) / det,
	}, true
}

// scale is how much m enlarges lengths on average: the square root of its
// determinant's magnitude.
func (m Matrix) scale() float32 {
	return float32(math.Sqrt(math.Abs(float64(m.A*m.D - m.B*m.C))))
}

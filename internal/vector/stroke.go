package vector

import "math"

// Cap is the shape of the ends of an open stroke.
type Cap uint8

const (
	CapButt   Cap = iota // the stroke stops at the end
	CapRound             // a half circle past the end
	CapSquare            // a half square past the end
)

// Join is the shape of a stroke's corners.
type Join uint8

const (
	JoinMiter Join = iota // sharp, cut to a bevel past the miter limit
	JoinRound
	JoinBevel
)

// Stroke describes how a path's outline is drawn.
type Stroke struct {
	Width float32
	Cap   Cap
	Join  Join
	// MiterLimit is the longest miter, as a multiple of the width, before a
	// miter join becomes a bevel; 0 means 4, as in SVG.
	MiterLimit float32
	// Dash alternates the lengths of dashes and gaps; empty draws a solid line.
	// DashOffset is how far into the pattern the stroke starts.
	Dash       []float32
	DashOffset float32
}

// strokePolygons returns polygons whose nonzero union is the stroke of lines
// with s. Every polygon winds the same way, so overlaps add up rather than
// cancel. tol is the flattening tolerance, in the same units.
func strokePolygons(lines []polyline, s Stroke, tol float32) [][]Point {
	hw := s.Width / 2
	if !(hw > 0) {
		return nil
	}
	if len(s.Dash) > 0 {
		lines = dash(lines, s.Dash, s.DashOffset)
	}
	limit := s.MiterLimit
	if limit <= 0 {
		limit = 4
	}
	var out [][]Point
	add := func(poly ...Point) {
		if area(poly) < 0 {
			for i, j := 0, len(poly)-1; i < j; i, j = i+1, j-1 {
				poly[i], poly[j] = poly[j], poly[i]
			}
		}
		out = append(out, poly)
	}
	for _, pl := range lines {
		pts := dedupe(pl.pts, pl.closed)
		if len(pts) == 1 {
			// A zero-length subpath draws only its caps.
			switch s.Cap {
			case CapRound:
				add(circle(pts[0], hw, tol)...)
			case CapSquare:
				p := pts[0]
				add(Point{p.X - hw, p.Y - hw}, Point{p.X + hw, p.Y - hw}, Point{p.X + hw, p.Y + hw}, Point{p.X - hw, p.Y + hw})
			}
			continue
		}
		n := len(pts)
		segs := n - 1
		if pl.closed {
			segs = n
		}
		for i := range segs {
			a, b := pts[i], pts[(i+1)%n]
			nv := normal(a, b).mul(hw)
			add(a.add(nv), b.add(nv), b.sub(nv), a.sub(nv))
		}
		// Joins at the inner points, and at the start of a closed subpath.
		for i := range n {
			if !pl.closed && (i == 0 || i == n-1) {
				continue
			}
			prev, cur, next := pts[(i-1+n)%n], pts[i], pts[(i+1)%n]
			join(add, prev, cur, next, hw, s.Join, limit, tol)
		}
		if !pl.closed {
			capEnd(add, pts[1], pts[0], hw, s.Cap, tol)
			capEnd(add, pts[n-2], pts[n-1], hw, s.Cap, tol)
		}
	}
	return out
}

// dedupe drops points equal to the one before them (and, for a closed
// subpath, a last point equal to the first).
func dedupe(pts []Point, closed bool) []Point {
	out := pts[:0:0]
	for _, p := range pts {
		if len(out) == 0 || p.sub(out[len(out)-1]).len() > 1e-5 {
			out = append(out, p)
		}
	}
	if closed && len(out) > 1 && out[0].sub(out[len(out)-1]).len() <= 1e-5 {
		out = out[:len(out)-1]
	}
	return out
}

// normal returns the unit normal of the line from a to b, to its left on
// screen.
func normal(a, b Point) Point {
	d := b.sub(a)
	l := d.len()
	if l == 0 {
		return Point{}
	}
	return Point{d.Y / l, -d.X / l}
}

// join adds the corner at cur between the lines prev-cur and cur-next.
func join(add func(...Point), prev, cur, next Point, hw float32, j Join, limit, tol float32) {
	n0, n1 := normal(prev, cur), normal(cur, next)
	turn := cur.sub(prev).cross(next.sub(cur))
	if math.Abs(float64(turn)) < 1e-9 && n0.dot(n1) > 0 {
		return // straight on
	}
	// The outer side is the one the corner turns away from: the left side
	// (the side normal gives) for a clockwise turn on screen.
	if turn < 0 {
		n0, n1 = n0.mul(-1), n1.mul(-1)
	}
	p0, p1 := cur.add(n0.mul(hw)), cur.add(n1.mul(hw))
	switch j {
	case JoinRound:
		a0 := math.Atan2(float64(n0.Y), float64(n0.X))
		a1 := math.Atan2(float64(n1.Y), float64(n1.X))
		d := a1 - a0
		for d > math.Pi {
			d -= 2 * math.Pi
		}
		for d < -math.Pi {
			d += 2 * math.Pi
		}
		k := max(int(math.Ceil(math.Abs(d)/arcStep(hw, tol))), 1)
		poly := []Point{cur, p0}
		r := hw * areaScale(math.Abs(d)/float64(k))
		for i := 1; i < k; i++ {
			s, c := math.Sincos(a0 + d*float64(i)/float64(k))
			poly = append(poly, Point{cur.X + r*float32(c), cur.Y + r*float32(s)})
		}
		add(append(poly, p1)...)
	case JoinMiter:
		// The miter's tip is where the two outer edges meet.
		cosHalf := float32(math.Sqrt(float64(max((1+n0.dot(n1))/2, 0))))
		if cosHalf > 1e-6 && 1/cosHalf <= limit {
			mid := n0.add(n1)
			tip := cur.add(mid.mul(hw / (cosHalf * mid.len())))
			add(cur, p0, tip, p1)
			return
		}
		add(cur, p0, p1)
	default:
		add(cur, p0, p1)
	}
}

// capEnd adds the cap at the end b of the line from a to b.
func capEnd(add func(...Point), a, b Point, hw float32, c Cap, tol float32) {
	d := b.sub(a)
	l := d.len()
	if l == 0 {
		return
	}
	d = d.mul(hw / l)
	nv := Point{d.Y, -d.X}
	switch c {
	case CapSquare:
		add(b.add(nv), b.add(nv).add(d), b.sub(nv).add(d), b.sub(nv))
	case CapRound:
		a0 := math.Atan2(float64(nv.Y), float64(nv.X))
		k := max(int(math.Ceil(math.Pi/arcStep(hw, tol))), 2)
		poly := []Point{b.add(nv)}
		r := hw * areaScale(math.Pi/float64(k))
		for i := 1; i < k; i++ {
			s, co := math.Sincos(a0 + math.Pi*float64(i)/float64(k)) // through the direction d
			poly = append(poly, Point{b.X + r*float32(co), b.Y + r*float32(s)})
		}
		add(append(poly, b.sub(nv))...)
	}
}

// arcStep is the largest angle, in radians, whose chord stays within tol of a
// circle of radius r.
func arcStep(r, tol float32) float64 {
	if r <= tol {
		return math.Pi / 2
	}
	return max(2*math.Acos(1-float64(tol/r)), math.Pi/128)
}

// areaScale is how much to enlarge the radius of a polygon whose vertices are
// step radians apart on a circle so that its area is the circle's: the chords
// cut inside the circle, and the vertices then lie a little outside it.
func areaScale(step float64) float32 {
	if step <= 0 || step >= math.Pi {
		return 1
	}
	return float32(math.Sqrt(step / math.Sin(step)))
}

// circle returns a polygon close to the circle of radius r about c.
func circle(c Point, r, tol float32) []Point {
	k := max(int(math.Ceil(2*math.Pi/arcStep(r, tol))), 8)
	pts := make([]Point, k)
	r *= areaScale(2 * math.Pi / float64(k))
	for i := range k {
		s, co := math.Sincos(2 * math.Pi * float64(i) / float64(k))
		pts[i] = Point{c.X + r*float32(co), c.Y + r*float32(s)}
	}
	return pts
}

// area returns the signed area of a polygon: positive when it winds clockwise
// on screen (y down).
func area(poly []Point) float32 {
	var a float32
	for i := range poly {
		a += poly[i].cross(poly[(i+1)%len(poly)])
	}
	return a / 2
}

// dash splits lines into the dashes of pattern, starting offset into it.
func dash(lines []polyline, pattern []float32, offset float32) []polyline {
	var total float32
	for _, d := range pattern {
		if d < 0 {
			return lines
		}
		total += d
	}
	if total <= 0 {
		return lines
	}
	if len(pattern)%2 == 1 {
		pattern = append(pattern, pattern...) // SVG repeats an odd list
		total *= 2
	}
	var out []polyline
	for _, pl := range lines {
		pts := pl.pts
		if pl.closed && len(pts) > 0 {
			pts = append(append([]Point(nil), pts...), pts[0])
		}
		// Find where in the pattern the subpath starts.
		idx, left := 0, pattern[0]
		off := float32(math.Mod(float64(offset), float64(total)))
		if off < 0 {
			off += total
		}
		for off > 0 {
			if off < left {
				left -= off
				break
			}
			off -= left
			idx = (idx + 1) % len(pattern)
			left = pattern[idx]
		}
		on := idx%2 == 0
		var cur []Point
		if on && len(pts) > 0 {
			cur = []Point{pts[0]}
		}
		for i := 0; i+1 < len(pts); i++ {
			a, b := pts[i], pts[i+1]
			seg := b.sub(a).len()
			pos := float32(0)
			for seg-pos > left {
				pos += left
				p := a.lerp(b, pos/seg)
				if on {
					cur = append(cur, p)
					out = append(out, polyline{pts: cur})
					cur = nil
				} else {
					cur = []Point{p}
				}
				on = !on
				idx = (idx + 1) % len(pattern)
				left = pattern[idx]
			}
			left -= seg - pos
			if on {
				cur = append(cur, b)
			}
		}
		if on && len(cur) > 1 {
			out = append(out, polyline{pts: cur})
		}
	}
	return out
}

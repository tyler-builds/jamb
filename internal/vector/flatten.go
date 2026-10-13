package vector

import "math"

// polyline is a subpath flattened to straight lines.
type polyline struct {
	pts    []Point
	closed bool
}

// flatten turns p into polylines, with curves split into lines that stay
// within tol of them.
func flatten(p *Path, tol float32) []polyline {
	var out []polyline
	var cur polyline
	end := func() {
		if len(cur.pts) > 0 {
			out = append(out, cur)
		}
		cur = polyline{}
	}
	i := 0
	var last Point
	for _, v := range p.verbs {
		switch v {
		case verbMove:
			end()
			last = p.pts[i]
			cur.pts = append(cur.pts, last)
			i++
		case verbLine:
			last = p.pts[i]
			cur.pts = append(cur.pts, last)
			i++
		case verbQuad:
			c, e := p.pts[i], p.pts[i+1]
			n := segments(last.sub(c.mul(2)).add(e).len(), tol, 4)
			for k := 1; k <= n; k++ {
				t := float32(k) / float32(n)
				a, b := last.lerp(c, t), c.lerp(e, t)
				cur.pts = append(cur.pts, a.lerp(b, t))
			}
			last = e
			i += 2
		case verbCubic:
			c1, c2, e := p.pts[i], p.pts[i+1], p.pts[i+2]
			dd := max(last.sub(c1.mul(2)).add(c2).len(), c1.sub(c2.mul(2)).add(e).len())
			n := segments(dd*6, tol, 8)
			for k := 1; k <= n; k++ {
				t := float32(k) / float32(n)
				a, b, c := last.lerp(c1, t), c1.lerp(c2, t), c2.lerp(e, t)
				ab, bc := a.lerp(b, t), b.lerp(c, t)
				cur.pts = append(cur.pts, ab.lerp(bc, t))
			}
			last = e
			i += 3
		case verbClose:
			cur.closed = true
			if len(cur.pts) > 0 {
				last = cur.pts[0]
			}
			end()
			// A segment after Close continues from the subpath's start.
			cur.pts = append(cur.pts, last)
		}
	}
	end()
	// Drop the lone starting points that Close left behind.
	j := 0
	for _, pl := range out {
		if len(pl.pts) > 1 || pl.closed {
			out[j] = pl
			j++
		}
	}
	return out[:j]
}

// segments returns how many lines approximate a curve whose second
// difference is dd within tol (Wang's formula, with div the degree's
// constant), from 1 to 512.
func segments(dd, tol, div float32) int {
	if tol <= 0 {
		tol = 0.25
	}
	n := int(math.Ceil(math.Sqrt(float64(dd / (div * tol)))))
	return min(max(n, 1), 512)
}

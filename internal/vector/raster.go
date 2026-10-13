package vector

import (
	"math"
	"slices"
)

// FillRule decides which parts of a self-overlapping path are inside.
type FillRule uint8

const (
	// NonZero fills every point the path winds around (SVG's default).
	NonZero FillRule = iota
	// EvenOdd fills points that an odd number of the path's edges surround,
	// so overlaps and nested subpaths make holes.
	EvenOdd
)

// subsamples is how many scanlines each row of pixels is sampled at. Across a
// scanline coverage is exact, so 16 gives smooth edges at every angle.
const subsamples = 16

type edge struct {
	x0, y0, x1, y1 float32 // y0 < y1
	dxdy           float32
	dir            int8 // +1 when the polygon goes down, -1 when up
}

type crossing struct {
	x   float32
	dir int8
}

// rasterizer computes the coverage of polygons, reusing its buffers.
type rasterizer struct {
	edges     []edge
	active    []int
	crossings []crossing
	frac      []float32 // per pixel: the partial coverage of span ends
	run       []float32 // per pixel boundary: where full coverage starts and stops
}

// fill sets cov (w*h values from 0 to 1) to the coverage of polys under rule,
// within the rows y0..y1 it returns; rows outside are left alone, at 0 when
// cov was zeroed.
func (r *rasterizer) fill(polys [][]Point, rule FillRule, w, h int, cov []float32) (y0, y1 int) {
	r.edges = r.edges[:0]
	minY, maxY := float32(math.Inf(1)), float32(math.Inf(-1))
	for _, poly := range polys {
		for i := range poly {
			a, b := poly[i], poly[(i+1)%len(poly)]
			if a.Y == b.Y || isBad(a) || isBad(b) {
				continue
			}
			e := edge{x0: a.X, y0: a.Y, x1: b.X, y1: b.Y, dir: 1}
			if a.Y > b.Y {
				e = edge{x0: b.X, y0: b.Y, x1: a.X, y1: a.Y, dir: -1}
			}
			e.dxdy = (e.x1 - e.x0) / (e.y1 - e.y0)
			r.edges = append(r.edges, e)
			minY, maxY = min(minY, e.y0), max(maxY, e.y1)
		}
	}
	if len(r.edges) == 0 {
		return 0, 0
	}
	slices.SortFunc(r.edges, func(a, b edge) int {
		switch {
		case a.y0 < b.y0:
			return -1
		case a.y0 > b.y0:
			return 1
		}
		return 0
	})
	y0 = max(int(math.Floor(float64(minY))), 0)
	y1 = min(int(math.Ceil(float64(maxY))), h)
	if y0 >= y1 {
		return 0, 0
	}
	if cap(r.frac) < w+2 {
		r.frac = make([]float32, w+2)
		r.run = make([]float32, w+2)
	}
	frac, run := r.frac[:w+2], r.run[:w+2]
	r.active = r.active[:0]
	next := 0
	const weight = 1.0 / subsamples
	for y := y0; y < y1; y++ {
		clear(frac)
		clear(run)
		touched := false
		for s := range subsamples {
			sy := float32(y) + (float32(s)+0.5)/subsamples
			for next < len(r.edges) && r.edges[next].y0 <= sy {
				r.active = append(r.active, next)
				next++
			}
			r.crossings = r.crossings[:0]
			j := 0
			for _, i := range r.active {
				e := &r.edges[i]
				if e.y1 <= sy {
					continue // ended above this scanline
				}
				r.active[j] = i
				j++
				if e.y0 <= sy {
					r.crossings = append(r.crossings, crossing{e.x0 + (sy-e.y0)*e.dxdy, e.dir})
				}
			}
			r.active = r.active[:j]
			if len(r.crossings) < 2 {
				continue
			}
			slices.SortFunc(r.crossings, func(a, b crossing) int {
				switch {
				case a.x < b.x:
					return -1
				case a.x > b.x:
					return 1
				}
				return 0
			})
			wind := 0
			var start float32
			for _, c := range r.crossings {
				was := inside(wind, rule)
				wind += int(c.dir)
				now := inside(wind, rule)
				if !was && now {
					start = c.x
				} else if was && !now {
					span(frac, run, start, c.x, w, weight)
					touched = true
				}
			}
		}
		if !touched {
			continue
		}
		row := cov[y*w : (y+1)*w]
		acc := float32(0)
		for x := range w {
			acc += run[x]
			row[x] = min(max(acc+frac[x], 0), 1)
		}
	}
	return y0, y1
}

func inside(wind int, rule FillRule) bool {
	if rule == EvenOdd {
		return wind&1 != 0
	}
	return wind != 0
}

func isBad(p Point) bool {
	return math.IsNaN(float64(p.X)) || math.IsNaN(float64(p.Y)) ||
		math.IsInf(float64(p.X), 0) || math.IsInf(float64(p.Y), 0)
}

// span adds the coverage of a scanline from x0 to x1, weighted by wt: the
// partly covered pixels at its ends to frac, and the fully covered ones
// between as a step in run.
func span(frac, run []float32, x0, x1 float32, w int, wt float32) {
	x0, x1 = max(x0, 0), min(x1, float32(w))
	if x0 >= x1 {
		return
	}
	i0, i1 := int(x0), int(x1)
	if i0 == i1 {
		frac[i0] += (x1 - x0) * wt
		return
	}
	frac[i0] += (float32(i0+1) - x0) * wt
	run[i0+1] += wt
	run[i1] -= wt
	if i1 < w {
		frac[i1] += (x1 - float32(i1)) * wt
	}
}

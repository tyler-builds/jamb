package vector

import (
	"cmp"
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
	edges []edge
	// sorted and counts are the buffers of sortEdges.
	sorted []edge
	counts []int32
	// active holds the edges that the current scanline crosses, as their indexes in
	// edges, and xs where each crosses it, in the same order: from left to right.
	// From one scanline to the next the edges move little, so the order is kept
	// with an insertion sort that has almost nothing to do.
	active    []int32
	xs        []float32
	crossings []crossing // used by fillReference in the tests
	frac      []float32  // per pixel: the partial coverage of span ends
	run       []float32  // per pixel boundary: where full coverage starts and stops
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
	r.sortEdges(h)
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
	r.active, r.xs = r.active[:0], r.xs[:0]
	next := 0
	const weight = 1.0 / subsamples
	for y := y0; y < y1; y++ {
		clear(frac)
		clear(run)
		touched := false
		for s := range subsamples {
			sy := float32(y) + (float32(s)+0.5)/subsamples
			for next < len(r.edges) && r.edges[next].y0 <= sy {
				r.active = append(r.active, int32(next))
				r.xs = append(r.xs, 0)
				next++
			}
			// Drop the edges that ended above this scanline, and place the others on it.
			j := 0
			for _, i := range r.active {
				e := &r.edges[i]
				if e.y1 <= sy {
					continue
				}
				r.active[j] = i
				r.xs[j] = e.x0 + (sy-e.y0)*e.dxdy
				j++
			}
			r.active, r.xs = r.active[:j], r.xs[:j]
			if j < 2 {
				continue
			}
			sortActive(r.active, r.xs)
			wind := 0
			var start float32
			for k, i := range r.active {
				was := inside(wind, rule)
				wind += int(r.edges[i].dir)
				now := inside(wind, rule)
				if !was && now {
					start = r.xs[k]
				} else if was && !now {
					span(frac, run, start, r.xs[k], w, weight)
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

// sortEdges sorts r.edges by y0, as the scanlines take them up. It counts the edges
// of each row of pixels, places them by row, and sorts each row's few with an
// insertion sort: linear in the edges, where a comparison sort of them was a
// tenth of the time of a chart's stroke. Edges above the first row or below the
// last share the row they are clamped to.
func (r *rasterizer) sortEdges(h int) {
	if cap(r.counts) < h+2 {
		r.counts = make([]int32, h+2)
	}
	counts := r.counts[:h+2]
	clear(counts)
	row := func(e *edge) int { return min(max(int(e.y0), -1), h) + 1 }
	for i := range r.edges {
		counts[row(&r.edges[i])]++
	}
	start := int32(0)
	for i, n := range counts {
		counts[i] = start
		start += n
	}
	if cap(r.sorted) < len(r.edges) {
		r.sorted = make([]edge, len(r.edges))
	}
	sorted := r.sorted[:len(r.edges)]
	for i := range r.edges {
		k := row(&r.edges[i])
		sorted[counts[k]] = r.edges[i]
		counts[k]++
	}
	// Each row's edges now end where the next row's start; sort within each. A row
	// with many edges, such as the first when a path lies mostly above the canvas,
	// takes a comparison sort, so no row costs the square of its edges.
	from := 0
	for _, end := range counts {
		if int(end)-from > 32 {
			slices.SortFunc(sorted[from:end], func(a, b edge) int { return cmp.Compare(a.y0, b.y0) })
			from = int(end)
			continue
		}
		for i := from + 1; i < int(end); i++ {
			e := sorted[i]
			j := i
			for j > from && sorted[j-1].y0 > e.y0 {
				sorted[j] = sorted[j-1]
				j--
			}
			sorted[j] = e
		}
		from = int(end)
	}
	r.edges, r.sorted = sorted, r.edges
}

// sortActive sorts the active edges by xs, keeping the two slices in step. They
// come mostly in order from the scanline before, which an insertion sort takes in
// close to one pass.
func sortActive(active []int32, xs []float32) {
	for i := 1; i < len(xs); i++ {
		x, e := xs[i], active[i]
		j := i
		for j > 0 && xs[j-1] > x {
			xs[j], active[j] = xs[j-1], active[j-1]
			j--
		}
		xs[j], active[j] = x, e
	}
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

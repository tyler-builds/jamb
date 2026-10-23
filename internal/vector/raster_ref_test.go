package vector

import (
	"math"
	"math/rand"
	"slices"
	"testing"
)

// fillReference is fill as it was before the active edges were kept in order: it
// sorts the crossings of each scanline afresh. The tests compare fill with it.
//
// fill sets cov (w*h values from 0 to 1) to the coverage of polys under rule,
// within the rows y0..y1 it returns; rows outside are left alone, at 0 when
// cov was zeroed.
func (r *rasterizer) fillReference(polys [][]Point, rule FillRule, w, h int, cov []float32) (y0, y1 int) {
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
	var act []int
	next := 0
	const weight = 1.0 / subsamples
	for y := y0; y < y1; y++ {
		clear(frac)
		clear(run)
		touched := false
		for s := range subsamples {
			sy := float32(y) + (float32(s)+0.5)/subsamples
			for next < len(r.edges) && r.edges[next].y0 <= sy {
				act = append(act, next)
				next++
			}
			r.crossings = r.crossings[:0]
			j := 0
			for _, i := range act {
				e := &r.edges[i]
				if e.y1 <= sy {
					continue // ended above this scanline
				}
				act[j] = i
				j++
				if e.y0 <= sy {
					r.crossings = append(r.crossings, crossing{e.x0 + (sy-e.y0)*e.dxdy, e.dir})
				}
			}
			act = act[:j]
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

// TestFillMatchesTheReference fills random polygons, chart strokes and areas, and
// shapes with holes, under both rules, and checks that fill gives the coverage the
// reference gives, to within float rounding.
func TestFillMatchesTheReference(t *testing.T) {
	const w, h = 160, 90
	rng := rand.New(rand.NewSource(11))
	var cases [][][]Point
	for range 60 {
		var polys [][]Point
		for range 1 + rng.Intn(4) {
			poly := make([]Point, 3+rng.Intn(12))
			for i := range poly {
				poly[i] = Point{rng.Float32()*(w+20) - 10, rng.Float32()*(h+20) - 10}
			}
			polys = append(polys, poly)
		}
		cases = append(cases, polys)
	}
	c := NewCanvas(w, h)
	for i, line := range []*Path{chartPath(80, w, h, 4), chartPath(400, w, h, 5)} {
		for _, join := range []Join{JoinRound, JoinBevel, JoinMiter} {
			cases = append(cases, c.polys(line, &Stroke{Width: 1 + float32(i), Join: join}))
		}
	}
	cases = append(cases, c.polys(areaPath(120, w, h, 6), nil))
	// A tall stroke whose edges lie mostly above and below the canvas, so that the
	// first and last rows of the edge sort hold many edges.
	tall := c.polys(chartPath(400, w, h*6, 7), &Stroke{Width: 2, Join: JoinRound})
	for _, poly := range tall {
		for i := range poly {
			poly[i].Y -= h * 2.5
		}
	}
	cases = append(cases, tall)
	// A square with a square hole, both ways round, and touching and coincident edges.
	cases = append(cases, [][]Point{
		{{10, 10}, {80, 10}, {80, 80}, {10, 80}},
		{{30, 30}, {30, 60}, {60, 60}, {60, 30}},
		{{30, 30}, {60, 30}, {60, 60}, {30, 60}},
		{{80, 10}, {120, 10}, {120, 40}, {80, 40}},
		{{10.5, 10}, {80.5, 10}, {80.5, 20}, {10.5, 20}},
	})
	var r, ref rasterizer
	for n, polys := range cases {
		for _, rule := range []FillRule{NonZero, EvenOdd} {
			got, want := make([]float32, w*h), make([]float32, w*h)
			gy0, gy1 := r.fill(polys, rule, w, h, got)
			wy0, wy1 := ref.fillReference(polys, rule, w, h, want)
			if gy0 != wy0 || gy1 != wy1 {
				t.Fatalf("case %d, rule %d: rows %d..%d, want %d..%d", n, rule, gy0, gy1, wy0, wy1)
			}
			for i := range got {
				if d := math.Abs(float64(got[i] - want[i])); d > 1e-4 {
					t.Fatalf("case %d, rule %d: pixel (%d, %d) is %v, want %v", n, rule, i%w, i/w, got[i], want[i])
				}
			}
		}
	}
}

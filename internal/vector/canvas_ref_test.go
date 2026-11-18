package vector

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// fillFullWidth is fill as it was before it kept to the pixels a shape covers:
// it works out each row the shape crosses across the whole canvas. The tests
// check that fill and the canvas give the same bytes as it does.
func (r *rasterizer) fillFullWidth(polys [][]Point, rule FillRule, w, h int, cov []float32) (y0, y1 int) {
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
	frac, run := make([]float32, w+2), make([]float32, w+2)
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

// refCanvas draws as Canvas did before its fills kept to the pixels a shape
// covers and its clips to their bounds: each fill and each clip worked out
// across the whole canvas, and each clip was a mask of the whole canvas. It
// keeps its transforms, opacity and layers in a Canvas that never clips, and
// its clips beside that Canvas's states.
type refCanvas struct {
	c     *Canvas
	clips [][]float32 // the clip of each of c's states; nil when none
	r     rasterizer
}

func newRefCanvas(w, h int) *refCanvas {
	return &refCanvas{c: NewCanvas(w, h), clips: [][]float32{nil}}
}

func (rc *refCanvas) Save() {
	rc.c.Save()
	rc.clips = append(rc.clips, rc.clips[len(rc.clips)-1])
}

func (rc *refCanvas) Restore() {
	if len(rc.clips) > 1 {
		rc.clips = rc.clips[:len(rc.clips)-1]
	}
	rc.c.Restore()
}

func (rc *refCanvas) Clip(p *Path, rule FillRule) {
	w, h := rc.c.img.Rect.Dx(), rc.c.img.Rect.Dy()
	mask := make([]float32, w*h)
	rc.r.fillFullWidth(rc.c.polys(p, nil), rule, w, h, mask)
	if old := rc.clips[len(rc.clips)-1]; old != nil {
		for i := range mask {
			mask[i] *= old[i]
		}
	}
	rc.clips[len(rc.clips)-1] = mask
}

func (rc *refCanvas) Fill(p *Path, paint Paint, rule FillRule) {
	rc.draw(rc.c.polys(p, nil), paint, rule)
}

func (rc *refCanvas) Stroke(p *Path, paint Paint, s Stroke) {
	rc.draw(rc.c.polys(p, &s), paint, NonZero)
}

func (rc *refCanvas) draw(polys [][]Point, paint Paint, rule FillRule) {
	st := rc.c.top()
	clip := rc.clips[len(rc.clips)-1]
	var sh shader
	if !sh.init(paint, st.m, st.alpha) || len(polys) == 0 {
		return
	}
	dst := rc.c.target()
	w, h := dst.Rect.Dx(), dst.Rect.Dy()
	cov := make([]float32, w*h)
	y0, y1 := rc.r.fillFullWidth(polys, rule, w, h, cov)
	solid := sh.grad == nil
	opaque := solid && sh.solid[3] == 255
	var px [4]uint8
	for k := range 4 {
		px[k] = uint8(min(sh.solid[k]+0.5, 255))
	}
	for y := y0; y < y1; y++ {
		row := cov[y*w : (y+1)*w]
		for x, cv := range row {
			if cv == 0 {
				continue
			}
			if clip != nil {
				cv *= clip[y*w+x]
				if cv == 0 {
					continue
				}
			}
			if opaque && cv == 1 {
				copy(dst.Pix[y*dst.Stride+x*4:y*dst.Stride+x*4+4], px[:])
				continue
			}
			src := sh.solid
			if !solid {
				src = sh.at(float32(x)+0.5, float32(y)+0.5)
			}
			sa := src[3] * cv
			if sa <= 0 {
				continue
			}
			p := dst.Pix[y*dst.Stride+x*4 : y*dst.Stride+x*4+4]
			keep := 1 - sa/255
			for k := range 4 {
				p[k] = uint8(min(src[k]*cv+float32(p[k])*keep+0.5, 255))
			}
		}
	}
}

// drawer is what the scenes below draw with: a Canvas, or the refCanvas.
type drawer interface {
	Save()
	Restore()
	Clip(p *Path, rule FillRule)
	Fill(p *Path, paint Paint, rule FillRule)
	Stroke(p *Path, paint Paint, s Stroke)
}

// canvasOf returns the Canvas that keeps a drawer's transforms, opacity and
// layers.
func canvasOf(d drawer) *Canvas {
	if rc, ok := d.(*refCanvas); ok {
		return rc.c
	}
	return d.(*Canvas)
}

// refScene is one drawing, made once by each of Canvas and refCanvas.
type refScene struct {
	name string
	w, h int
	draw func(d drawer)
}

func rectPath(x, y, w, h float32) *Path {
	p := &Path{}
	p.Rect(x, y, w, h)
	return p
}

var (
	refRed   = Paint{Color: Color{R: 0.9, G: 0.1, B: 0.1, A: 1}}
	refBlue  = Paint{Color: Color{R: 0.1, G: 0.2, B: 0.9, A: 0.6}}
	refGreen = Paint{Color: Color{R: 0.1, G: 0.8, B: 0.3, A: 1}}
	refGrad  = Paint{Gradient: &Gradient{Kind: Linear, X0: 0, Y0: 0, X1: 60, Y1: 30,
		Stops: []Stop{{0, Color{1, 0, 0, 1}}, {1, Color{0, 0, 1, 0.5}}}, Matrix: Identity}}
)

// refScenes covers what the fills and clips have to get right: edges between
// pixels, shapes partly and wholly off the canvas, and clips inside clips.
func refScenes() []refScene {
	scenes := []refScene{
		{"fractional rects", 64, 40, func(d drawer) {
			for i := range 30 {
				f := float32(i)
				d.Fill(rectPath(f*2.13-3.7, f*1.31-2.2, 5.37+f*0.11, 3.29+f*0.07), refBlue, NonZero)
				d.Fill(rectPath(f*1.9+0.5, 39.5-f*1.27, 0.3+f*0.05, 0.4), refRed, EvenOdd)
			}
		}},
		{"rects on and off the canvas", 50, 30, func(d drawer) {
			for _, r := range [][4]float32{
				{-10, -10, 15, 15}, {45.5, 25.25, 20, 20}, {-5, 10, 70, 3.5}, {20, -50, 4.75, 200},
				{60, 5, 10, 10}, {-30, 5, 10, 10}, {5, 40, 10, 10}, {5, -20, 10, 10},
				{-1e6, 12, 2e6, 1}, {10, 10, 0, 10}, {10, 10, 10, 0}, {30, 20, -7.5, -6.25},
				{0, 0, 50, 30},
			} {
				d.Fill(rectPath(r[0], r[1], r[2], r[3]), refBlue, NonZero)
			}
		}},
		{"anti-aliased shapes", 80, 60, func(d drawer) {
			var p Path
			p.Ellipse(30.3, 25.7, 22.4, 14.1)
			d.Fill(&p, refGrad, NonZero)
			var q Path
			q.RoundRect(45.5, 30.25, 40, 40, 7, 7)
			d.Fill(&q, refBlue, NonZero)
			d.Stroke(chartPath(60, 80, 60, 3), refRed, Stroke{Width: 1.5, Join: JoinRound})
			c := canvasOf(d)
			c.Save()
			c.Transform(Rotate(0.3))
			d.Fill(rectPath(20, -5, 30, 12), refGreen, NonZero)
			c.Restore()
		}},
		{"nested clips", 60, 50, func(d drawer) {
			d.Save()
			var circle Path
			circle.Ellipse(30, 25, 22.5, 18.3)
			d.Clip(&circle, NonZero)
			d.Fill(rectPath(0, 0, 60, 50), refBlue, NonZero)
			d.Save()
			d.Clip(rectPath(10.5, 5.25, 30.75, 60), NonZero)
			d.Fill(rectPath(-5, 10, 80, 20), refRed, NonZero)
			d.Save()
			d.Clip(rectPath(70, 0, 10, 10), NonZero) // wholly outside the clip above
			d.Fill(rectPath(0, 0, 60, 50), refGreen, NonZero)
			d.Restore()
			d.Stroke(chartPath(40, 60, 50, 9), refGreen, Stroke{Width: 2})
			d.Clip(rectPath(20.5, 20.5, 5, 5), NonZero) // twice in one state
			d.Fill(rectPath(0, 0, 60, 50), refGrad, NonZero)
			d.Restore()
			d.Fill(rectPath(40.4, 30.6, 30, 30), refGreen, NonZero)
			d.Restore()
			d.Fill(rectPath(-2.5, 40.5, 20, 20), refRed, NonZero)
		}},
		{"clips off the canvas", 40, 30, func(d drawer) {
			d.Save()
			d.Clip(rectPath(-20, -20, 10, 10), NonZero)
			d.Fill(rectPath(0, 0, 40, 30), refRed, NonZero)
			d.Restore()
			d.Save()
			d.Clip(rectPath(-20.5, 10.5, 45, 100), NonZero)
			d.Fill(rectPath(0, 0, 40, 30), refBlue, NonZero)
			d.Restore()
			// A clip in the first state, with no Save, that a later clip narrows.
			d.Clip(rectPath(5.5, 5.5, 30, 20), EvenOdd)
			d.Fill(rectPath(0, 0, 40, 30), refGreen, NonZero)
			d.Clip(rectPath(30, 0, 30, 30), NonZero)
			d.Fill(rectPath(0, 0, 40, 30), refRed, NonZero)
		}},
		{"clips with layers and opacity", 50, 40, func(d drawer) {
			c := canvasOf(d)
			d.Save()
			d.Clip(rectPath(4.5, 4.5, 40, 30), NonZero)
			c.PushLayer(0.7)
			c.SetAlpha(0.8)
			d.Fill(rectPath(0, 0, 30, 30), refRed, NonZero)
			d.Save()
			c.Transform(Scale(1.5, 1.25))
			var e Path
			e.Ellipse(20, 15, 12, 9)
			d.Clip(&e, NonZero)
			d.Fill(rectPath(0, 0, 50, 40), refGrad, NonZero)
			d.Restore()
			c.PopLayer()
			d.Restore()
		}},
	}
	// Random rectangles, polygons and clips, nested at random.
	for seed := range int64(12) {
		scenes = append(scenes, refScene{fmt.Sprintf("random %d", seed), 70, 45, func(d drawer) {
			rng := rand.New(rand.NewSource(seed))
			coord := func(n float32) float32 { return rng.Float32()*(n+30) - 15 }
			paints := []Paint{refRed, refBlue, refGreen, refGrad}
			depth := 0
			for range 40 {
				var p *Path
				if rng.Intn(2) == 0 {
					p = rectPath(coord(70), coord(45), rng.Float32()*40-5, rng.Float32()*30-5)
				} else {
					p = &Path{}
					p.MoveTo(coord(70), coord(45))
					for range 2 + rng.Intn(6) {
						p.LineTo(coord(70), coord(45))
					}
					p.Close()
				}
				rule := FillRule(rng.Intn(2))
				switch k := rng.Intn(10); {
				case k < 2:
					d.Save()
					depth++
					d.Clip(p, rule)
				case k < 3 && depth > 0:
					d.Restore()
					depth--
				case k < 4:
					d.Stroke(p, paints[rng.Intn(len(paints))], Stroke{Width: 0.5 + rng.Float32()*3})
				default:
					d.Fill(p, paints[rng.Intn(len(paints))], rule)
				}
			}
		}})
	}
	return scenes
}

// TestCanvasMatchesTheReference draws each scene with Canvas and with the full
// width reference, and checks that they give the same bytes. One Canvas draws
// them all, so a buffer or mask that one scene leaves dirty shows in the next.
func TestCanvasMatchesTheReference(t *testing.T) {
	c := NewCanvas(1, 1)
	for _, sc := range refScenes() {
		for range 2 {
			c.Reset(sc.w, sc.h)
			sc.draw(c)
			ref := newRefCanvas(sc.w, sc.h)
			sc.draw(ref)
			got, want := c.Image(), ref.c.Image()
			if bytes.Equal(got.Pix, want.Pix) {
				continue
			}
			for i := range got.Pix {
				if got.Pix[i] != want.Pix[i] {
					px := i / 4
					t.Fatalf("%s: pixel (%d, %d) is %v, want %v", sc.name, px%sc.w, px/sc.w,
						got.Pix[px*4:px*4+4], want.Pix[px*4:px*4+4])
				}
			}
		}
	}
	for i, v := range c.cov {
		if v != 0 {
			t.Fatalf("coverage %d left at %v", i, v)
		}
	}
}

// TestFillMatchesFullWidth checks that fill, by its rectangle path and its
// general one, gives exactly the coverage of the full width reference, within
// the rows it is limited to, and sets no pixel outside the spans it reports.
func TestFillMatchesFullWidth(t *testing.T) {
	const w, h = 40, 30
	rng := rand.New(rand.NewSource(5))
	var r, ref rasterizer
	got, want := make([]float32, w*h), make([]float32, w*h)
	for n := range 600 {
		x, y := rng.Float32()*60-10, rng.Float32()*50-10
		rw, rh := rng.Float32()*30-3, rng.Float32()*25-3
		switch n % 4 {
		case 0: // on whole pixels
			x, y, rw, rh = float32(int(x)), float32(int(y)), float32(int(rw)), float32(int(rh))
		case 1: // on 32nds of pixels, so some edges lie on scanlines
			snap := func(v float32) float32 { return float32(math.Round(float64(v)*32)) / 32 }
			x, y, rw, rh = snap(x), snap(y), snap(rw), snap(rh)
		}
		var polys [][]Point
		switch n % 3 {
		case 0:
			polys = [][]Point{{{x, y}, {x + rw, y}, {x + rw, y + rh}, {x, y + rh}}}
		case 1:
			polys = [][]Point{{{x, y}, {x, y + rh}, {x + rw, y + rh}, {x + rw, y}}}
		default:
			poly := make([]Point, 3+rng.Intn(6))
			for i := range poly {
				poly[i] = Point{rng.Float32()*60 - 10, rng.Float32()*50 - 10}
			}
			polys = [][]Point{poly}
		}
		if _, _, ok := (&rasterizer{}).fillRect(polys, w, 0, h, make([]float32, w*h)); ok != (n%3 != 2) {
			t.Fatalf("case %d: fillRect took %v: %v", n, ok, polys)
		}
		top, bottom := 0, h
		if n%5 < 2 {
			top = rng.Intn(h)
			bottom = top + rng.Intn(h-top+1)
		}
		for _, rule := range []FillRule{NonZero, EvenOdd} {
			clear(got)
			clear(want)
			y0, y1 := r.fill(polys, rule, w, h, top, bottom, got)
			ref.fillFullWidth(polys, rule, w, h, want)
			for y := range h {
				var sp rowSpan
				if y >= y0 && y < y1 {
					sp = r.rows[y-y0]
				}
				for x := range w {
					g, wv := got[y*w+x], want[y*w+x]
					if y < top || y >= bottom {
						wv = 0
					}
					if g != wv {
						t.Fatalf("case %d, rule %d, rows %d..%d: pixel (%d, %d) is %v, want %v",
							n, rule, top, bottom, x, y, g, wv)
					}
					if g != 0 && (x < int(sp.x0) || x >= int(sp.x1)) {
						t.Fatalf("case %d: pixel (%d, %d) set outside its row's span %v", n, x, y, sp)
					}
				}
			}
		}
	}
}

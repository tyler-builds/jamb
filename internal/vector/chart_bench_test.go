package vector

import (
	"math/rand"
	"testing"
)

// chartPath is a jagged line of n points across a w by h plot, as a live chart
// of a processor's load draws it.
func chartPath(n int, w, h float32, seed int64) *Path {
	rng := rand.New(rand.NewSource(seed))
	p := &Path{}
	y := h / 2
	for i := range n {
		y = min(max(y+float32(rng.NormFloat64())*h/12, 4), h-4)
		x := float32(i) * (w - 1) / float32(n-1)
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	return p
}

// areaPath closes chartPath down to the bottom of the plot, as an area chart fills it.
func areaPath(n int, w, h float32, seed int64) *Path {
	p := chartPath(n, w, h, seed)
	p.LineTo(w-1, h)
	p.LineTo(0, h)
	p.Close()
	return p
}

var chartPaint = Paint{Color: Color{R: 0.3, G: 0.5, B: 0.9, A: 1}}

// BenchmarkChartStroke strokes a 300-point line, 2 pixels wide with round joins,
// on an 800 by 220 canvas: a chart of five minutes of samples.
func BenchmarkChartStroke(b *testing.B) {
	c := NewCanvas(800, 220)
	p := chartPath(300, 800, 220, 1)
	b.ReportAllocs()
	for b.Loop() {
		c.Stroke(p, chartPaint, Stroke{Width: 2, Join: JoinRound})
	}
}

// BenchmarkChartStrokeDense strokes a 1,600-point line, two points per pixel, as a
// long series downsampled to the plot draws it.
func BenchmarkChartStrokeDense(b *testing.B) {
	c := NewCanvas(800, 220)
	p := chartPath(1600, 800, 220, 2)
	b.ReportAllocs()
	for b.Loop() {
		c.Stroke(p, chartPaint, Stroke{Width: 2, Join: JoinRound})
	}
}

// BenchmarkChartArea fills the area under a 300-point line.
func BenchmarkChartArea(b *testing.B) {
	c := NewCanvas(800, 220)
	p := areaPath(300, 800, 220, 3)
	paint := Paint{Color: Color{R: 0.3, G: 0.5, B: 0.9, A: 0.3}}
	b.ReportAllocs()
	for b.Loop() {
		c.Fill(p, paint, NonZero)
	}
}

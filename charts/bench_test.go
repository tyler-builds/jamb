package charts

import (
	"math/rand"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// walk100k returns 100,000 values of a random walk, as the data of a long series.
func walk100k() []float64 {
	rng := rand.New(rand.NewSource(7))
	v := make([]float64, 100000)
	y := 0.0
	for i := range v {
		y += rng.NormFloat64()
		v[i] = y
	}
	return v
}

// benchChart runs a chart in an 800 by 600 runtime with reduced motion, so each frame draws
// the chart once. Each iteration moves the pointer, which re-renders the chart and draws it
// again, with the layout, the downsampling and the hover, as a live hover does.
func benchChart(b *testing.B, c *Chart) {
	b.Helper()
	rt := ui.NewRuntime(func(*ui.Context) ui.Element { return ui.Div().Child(c) }, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.SetSystemSettings(ui.SystemSettings{ReducedMotion: true})
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	plot := c.last.plot
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := float32(plot.x0) + float32(i%int(plot.x1-plot.x0))
		rt.PointerMove(x, float32(plot.y0)+40, 0)
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
}

// BenchmarkLine100k draws a line of 100,000 points, reduced to about two points per pixel of
// the plot, per frame.
func BenchmarkLine100k(b *testing.B) {
	c := Line(Series[float64]{Name: "Walk", Data: walk100k(), Y: func(v float64) float64 { return v }})
	benchChart(b, c)
}

// BenchmarkArea100k draws an area of 100,000 points per frame.
func BenchmarkArea100k(b *testing.B) {
	c := Area(Series[float64]{Name: "Walk", Data: walk100k(), Y: func(v float64) float64 { return v }})
	benchChart(b, c)
}

// TestLargeLineFrameIsDrawn checks that a line of 100,000 points is drawn, and reduced: the
// geometry of the draw holds the plot, and the points it drew are downsampled to about two per
// pixel, which the benchmarks time.
func TestLargeLineFrameIsDrawn(t *testing.T) {
	c := Line(Series[float64]{Name: "Walk", Data: walk100k(), Y: func(v float64) float64 { return v }})
	newRig(func(*ui.Context) ui.Element { return ui.Div().Child(c) })
	g := c.last
	if !g.drawn || g.plot.x1 <= g.plot.x0 {
		t.Fatalf("the 100,000-point line was not drawn: %+v", g.plot)
	}
	xs, _ := c.thinned(g, 0)
	if limit := int(2*(g.plot.x1-g.plot.x0)) + 1; len(xs) > limit || len(xs) < 3 {
		t.Errorf("the line draws %d points, want at most about %d", len(xs), limit)
	}
}

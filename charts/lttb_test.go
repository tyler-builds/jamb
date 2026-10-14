package charts

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// series returns n points at x = 0 .. n-1 with values from f.
func series(n int, f func(i int) float64) (xs, ys []float64) {
	xs, ys = make([]float64, n), make([]float64, n)
	for i := range xs {
		xs[i], ys[i] = float64(i), f(i)
	}
	return xs, ys
}

// LTTB keeps the first and the last point, exactly the threshold number of points, in order.
func TestLTTBShape(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	xs, ys := series(10000, func(int) float64 { return rng.Float64() })
	idx := lttb(xs, ys, 200)
	if len(idx) != 200 {
		t.Fatalf("kept %d points, want 200", len(idx))
	}
	if idx[0] != 0 || idx[len(idx)-1] != len(xs)-1 {
		t.Errorf("ends are %d and %d, want 0 and %d", idx[0], idx[len(idx)-1], len(xs)-1)
	}
	for i := 1; i < len(idx); i++ {
		if idx[i] <= idx[i-1] {
			t.Fatalf("indices not increasing at %d: %d then %d", i, idx[i-1], idx[i])
		}
	}
}

// A peak survives the reduction: the largest triangle is at the spike, in its bucket.
func TestLTTBKeepsSpikes(t *testing.T) {
	xs, ys := series(1000, func(i int) float64 {
		if i == 437 {
			return 100
		}
		return 1
	})
	idx := lttb(xs, ys, 50)
	found := false
	for _, i := range idx {
		found = found || i == 437
	}
	if !found {
		t.Errorf("the spike at 437 is not among the %d kept points", len(idx))
	}
}

// A threshold at or above the count keeps every point, and a threshold below three
// keeps them too, since a triangle needs three.
func TestLTTBIdentity(t *testing.T) {
	xs, ys := series(12, func(i int) float64 { return float64(i * i) })
	for _, th := range []int{12, 50, 2, 0} {
		if idx := lttb(xs, ys, th); len(idx) != 12 {
			t.Errorf("threshold %d kept %d points, want all 12", th, len(idx))
		}
	}
}

// Reducing 100,000 points to about two per pixel of an 800-pixel plot is fast enough for
// each hover to redraw the chart. The bound is generous, so that a loaded machine does not
// fail it.
func TestLTTBLargeIsFast(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	xs, ys := series(100000, func(int) float64 { return rng.NormFloat64() })
	start := time.Now()
	idx := lttb(xs, ys, 1600)
	if took := time.Since(start); took > 200*time.Millisecond {
		t.Errorf("lttb of 100,000 points took %v", took)
	}
	if len(idx) != 1600 || math.IsNaN(ys[idx[5]]) {
		t.Errorf("kept %d points", len(idx))
	}
}

// BenchmarkLTTB reduces 100,000 points to 1,600, two per pixel of an 800-pixel plot.
func BenchmarkLTTB(b *testing.B) {
	rng := rand.New(rand.NewSource(3))
	xs, ys := series(100000, func(int) float64 { return rng.NormFloat64() })
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lttb(xs, ys, 1600)
	}
}

package charts

import (
	"math"

	"github.com/tyler-builds/jamb/ui"
)

// smoothPath returns a smooth curve through the points px and py, in pixels, with
// cubic Bézier segments. The tangents are those of the monotone cubic interpolation of
// Fritsch and Carlson, so the curve never overshoots a point into the next segment's
// range: a peak stays at its point. px must be ascending.
func smoothPath(px, py []float64) *ui.Path {
	path := ui.NewPath()
	n := len(px)
	if n == 0 {
		return path
	}
	path.MoveTo(float32(px[0]), float32(py[0]))
	if n == 1 {
		return path
	}
	// The slopes of the segments, and the tangents at the points.
	d := make([]float64, n-1)
	for k := range d {
		if dx := px[k+1] - px[k]; dx != 0 {
			d[k] = (py[k+1] - py[k]) / dx
		}
	}
	m := make([]float64, n)
	m[0], m[n-1] = d[0], d[n-2]
	for k := 1; k < n-1; k++ {
		if d[k-1]*d[k] > 0 {
			m[k] = (d[k-1] + d[k]) / 2
		}
	}
	for k := range d {
		if d[k] == 0 {
			m[k], m[k+1] = 0, 0
			continue
		}
		a, b := m[k]/d[k], m[k+1]/d[k]
		if s := a*a + b*b; s > 9 {
			tau := 3 / math.Sqrt(s)
			m[k], m[k+1] = tau*a*d[k], tau*b*d[k]
		}
	}
	for k := 0; k < n-1; k++ {
		third := (px[k+1] - px[k]) / 3
		path.CubicTo(float32(px[k]+third), float32(py[k]+m[k]*third),
			float32(px[k+1]-third), float32(py[k+1]-m[k+1]*third),
			float32(px[k+1]), float32(py[k+1]))
	}
	return path
}

package charts

import "math"

// lttb returns the indices of the points that Largest-Triangle-Three-Buckets
// keeps when it reduces the points xs and ys to threshold points. The first and
// the last point are always kept. Between them the points are split into
// buckets, and each bucket keeps the point that forms the largest triangle with
// the point kept before it and the average of the bucket after it, so peaks and
// troughs survive the reduction. xs must be ascending. It runs in linear time.
// A threshold of at least the number of points keeps them all.
func lttb(xs, ys []float64, threshold int) []int {
	n := len(xs)
	if threshold >= n || threshold < 3 {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		return idx
	}
	out := make([]int, 0, threshold)
	out = append(out, 0)
	every := float64(n-2) / float64(threshold-2)
	a := 0
	for i := 0; i < threshold-2; i++ {
		// The average of the bucket after this one, which the triangle's third
		// corner is taken from.
		avgStart := int(math.Floor(float64(i+1)*every)) + 1
		avgEnd := min(int(math.Floor(float64(i+2)*every))+1, n)
		if avgEnd <= avgStart {
			avgEnd = avgStart + 1
		}
		var avgX, avgY float64
		for j := avgStart; j < avgEnd; j++ {
			avgX += xs[j]
			avgY += ys[j]
		}
		avgX /= float64(avgEnd - avgStart)
		avgY /= float64(avgEnd - avgStart)

		rangeStart := int(math.Floor(float64(i)*every)) + 1
		rangeEnd := int(math.Floor(float64(i+1)*every)) + 1
		pax, pay := xs[a], ys[a]
		next, maxArea := rangeStart, -1.0
		for j := rangeStart; j < rangeEnd; j++ {
			area := math.Abs((pax-avgX)*(ys[j]-pay)-(pax-xs[j])*(avgY-pay)) * 0.5
			if area > maxArea {
				maxArea, next = area, j
			}
		}
		out = append(out, next)
		a = next
	}
	return append(out, n-1)
}

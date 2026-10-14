package charts

// stackValues returns the extent of each series in a stack, slot by slot. vals[k][i]
// is the value of series k at slot i, and a missing value is 0. A series' extent at
// a slot runs from lo[k][i] to hi[k][i]: positive values stack upwards from zero,
// negative values stack downwards from zero, and each series starts where the
// stack of its sign below it ends.
func stackValues(vals [][]float64) (lo, hi [][]float64) {
	lo, hi = make([][]float64, len(vals)), make([][]float64, len(vals))
	if len(vals) == 0 {
		return lo, hi
	}
	n := len(vals[0])
	pos, neg := make([]float64, n), make([]float64, n)
	for k, row := range vals {
		lo[k], hi[k] = make([]float64, n), make([]float64, n)
		for i := 0; i < n; i++ {
			v := row[i]
			if v >= 0 {
				lo[k][i], hi[k][i] = pos[i], pos[i]+v
				pos[i] += v
			} else {
				lo[k][i], hi[k][i] = neg[i]+v, neg[i]
				neg[i] += v
			}
		}
	}
	return lo, hi
}

// slotValues returns the values of a series at the slots xs (ascending), with 0
// for a slot the series has no point at. pts is sorted by x.
func slotValues(pts []point, xs []float64) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		if j, ok := findX(pts, x); ok {
			out[i] = pts[j].y
		}
	}
	return out
}

// findX returns the index of a point of pts, sorted by x, whose x is x exactly.
func findX(pts []point, x float64) (int, bool) {
	lo, hi := 0, len(pts)
	for lo < hi {
		m := (lo + hi) / 2
		if pts[m].x < x {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(pts) && pts[lo].x == x {
		return lo, true
	}
	return 0, false
}

// nearestX returns the index of the point of pts, sorted by x, whose x is nearest
// to x. pts must not be empty.
func nearestX(pts []point, x float64) int {
	lo, hi := 0, len(pts)
	for lo < hi {
		m := (lo + hi) / 2
		if pts[m].x < x {
			lo = m + 1
		} else {
			hi = m
		}
	}
	switch {
	case lo == 0:
		return 0
	case lo == len(pts):
		return len(pts) - 1
	case x-pts[lo-1].x <= pts[lo].x-x:
		return lo - 1
	}
	return lo
}

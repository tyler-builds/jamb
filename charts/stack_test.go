package charts

import (
	"reflect"
	"testing"
)

// Stacks run from zero upwards for positive values and downwards for negative ones, each
// series starting where the stack of its sign below it ends.
func TestStackValues(t *testing.T) {
	lo, hi := stackValues([][]float64{{1, 2}, {3, 4}, {-1, 0}, {2, -5}})
	// Slot 0: 1, then 3 stacked on it (4), then -1 down from zero, then 2 on the 4 (6).
	// Slot 1: 2, then 4 (6), then 0 on the 6, then -5 down from zero.
	wantLo := [][]float64{{0, 0}, {1, 2}, {-1, 6}, {4, -5}}
	wantHi := [][]float64{{1, 2}, {4, 6}, {0, 6}, {6, 0}}
	if !reflect.DeepEqual(lo, wantLo) || !reflect.DeepEqual(hi, wantHi) {
		t.Errorf("stackValues: lo %v hi %v, want lo %v hi %v", lo, hi, wantLo, wantHi)
	}
	// The top of the positive stack is the sum of the positive values.
	if top := hi[1][1]; top != 6 {
		t.Errorf("top of the second slot's stack = %v, want 6", top)
	}
}

// A stack of no series has no extents.
func TestStackValuesEmpty(t *testing.T) {
	lo, hi := stackValues(nil)
	if len(lo) != 0 || len(hi) != 0 {
		t.Errorf("empty stack: %v %v", lo, hi)
	}
}

// A series' values at the slots are its points' values, with zero where it has no point.
func TestSlotValuesAndSearch(t *testing.T) {
	pts := []point{{x: 1, y: 10}, {x: 3, y: 30}, {x: 5, y: 50}}
	if got := slotValues(pts, []float64{0, 1, 2, 3, 5}); !reflect.DeepEqual(got, []float64{0, 10, 0, 30, 50}) {
		t.Errorf("slotValues = %v", got)
	}
	if _, ok := findX(pts, 2); ok {
		t.Error("findX found a point at 2")
	}
	if i := nearestX(pts, 2.4); i != 1 {
		t.Errorf("nearestX(2.4) = %d, want 1", i)
	}
	if i := nearestX(pts, 2.6); i != 1 {
		t.Errorf("nearestX(2.6) = %d, want 1", i)
	}
	if i := nearestX(pts, -9); i != 0 {
		t.Errorf("nearestX before the start = %d", i)
	}
	if i := nearestX(pts, 99); i != 2 {
		t.Errorf("nearestX after the end = %d", i)
	}
}

package charts

import "testing"

// Min and Max fix either end of the value axis, and the other end follows the data.
func TestAxisMinAndMaxFixEitherEnd(t *testing.T) {
	m := cartModel{vlo: 0, vhi: 95}
	check := func(name string, a Axis, wantLo, wantHi float64, fixedLo, fixedHi bool) {
		t.Helper()
		c := &Chart{ya: a}
		ticks, lo, hi := c.valueTicks(m, 5)
		if fixedLo && lo != wantLo {
			t.Errorf("%s: the axis starts at %v, want %v", name, lo, wantLo)
		}
		if !fixedLo && lo > 0 {
			t.Errorf("%s: the axis starts at %v, want the data's start, at or below 0", name, lo)
		}
		if fixedHi && hi != wantHi {
			t.Errorf("%s: the axis ends at %v, want %v", name, hi, wantHi)
		}
		if !fixedHi && hi < 95 {
			t.Errorf("%s: the axis ends at %v, want the data's end, at or above 95", name, hi)
		}
		for _, v := range ticks {
			if v < lo || v > hi {
				t.Errorf("%s: tick %v is outside the axis, %v to %v", name, v, lo, hi)
			}
		}
	}
	check("both", Axis{}.Min(10).Max(60), 10, 60, true, true)
	check("min alone", Axis{}.Min(10), 10, 0, true, false)
	check("max alone", Axis{}.Max(60), 0, 60, false, true)
}

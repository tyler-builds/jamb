package charts

import (
	"math"
	"reflect"
	"testing"
	"time"
)

// Nice steps are 1, 2 or 5 times a power of ten, chosen for about n ticks.
func TestNiceStep(t *testing.T) {
	cases := []struct {
		span float64
		n    int
		want float64
	}{
		{97, 5, 20},  // 97/4 = 24.25 is a 2 x 10 step
		{1, 5, 0.2},  // 0.25 is a 2 x 0.1 step
		{100, 6, 20}, // 100/5 = 20
		{10, 3, 5},   // 10/2 = 5 is a 5 x 1 step
		{3, 3, 2},    // 3/2 = 1.5 is at the boundary to a 2 step
		{2.9, 3, 1},  // 2.9/2 = 1.45 is a 1 step
		{7, 2, 10},   // 7/1 = 7 is at the boundary to a 10 step
	}
	for _, c := range cases {
		if got := niceStep(c.span, c.n); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("niceStep(%v, %d) = %v, want %v", c.span, c.n, got, c.want)
		}
	}
}

// Linear ticks start and end on the step, and the ticks lie inside the span.
func TestLinearTicks(t *testing.T) {
	// The ticks lie inside the span; the bounds are the first and last steps around it.
	ticks, lo, hi := linearTicks(0, 97, 5)
	if want := []float64{0, 20, 40, 60, 80}; !reflect.DeepEqual(ticks, want) || lo != 0 || hi != 100 {
		t.Errorf("linearTicks(0, 97) = %v bounds %v to %v, want %v bounds 0 to 100", ticks, lo, hi, want)
	}
	ticks, lo, hi = linearTicks(0.13, 0.87, 5)
	want := []float64{0.2, 0.4, 0.6, 0.8}
	if len(ticks) != len(want) || lo != 0 || math.Abs(hi-1) > 1e-12 {
		t.Fatalf("linearTicks(0.13, 0.87) = %v bounds %v to %v", ticks, lo, hi)
	}
	for i := range want {
		if math.Abs(ticks[i]-want[i]) > 1e-9 {
			t.Errorf("tick %d = %v, want %v", i, ticks[i], want[i])
		}
	}
	// A span of no width still has ticks around its value.
	ticks, lo, hi = linearTicks(5, 5, 5)
	if len(ticks) == 0 || lo > 5 || hi < 5 {
		t.Errorf("linearTicks(5, 5) = %v bounds %v to %v", ticks, lo, hi)
	}
}

// Time ticks fall on whole days, months and years of the location, inside the span.
func TestTimeTicks(t *testing.T) {
	lo := float64(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC).Unix())
	hi := float64(time.Date(2026, time.October, 8, 0, 0, 0, 0, time.UTC).Unix())
	ticks, st := timeTicks(lo, hi, 8, time.UTC)
	if st.unit != unitDay || st.n != 1 || len(ticks) != 8 {
		t.Fatalf("a week of ticks: step %+v, %d ticks", st, len(ticks))
	}
	if got := formatTime(ticks[0], time.UTC); got != "2026-10-01" {
		t.Errorf("first tick %s, want 2026-10-01", got)
	}

	lo = float64(time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC).Unix())
	hi = float64(time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC).Unix())
	ticks, st = timeTicks(lo, hi, 6, time.UTC)
	if st.unit != unitMonth || st.n != 3 {
		t.Fatalf("a year of ticks: step %+v", st)
	}
	var got []string
	for _, v := range ticks {
		got = append(got, time.Unix(int64(v), 0).UTC().Format("2006-01-02"))
	}
	if want := []string{"2026-04-01", "2026-07-01", "2026-10-01"}; !reflect.DeepEqual(got, want) {
		t.Errorf("quarterly ticks %v, want %v", got, want)
	}
}

// A linear scale maps its domain to its range, and back.
func TestScaleMapping(t *testing.T) {
	s := newScale(0, 10, 0, 100)
	if s.at(5) != 50 || s.value(50) != 5 {
		t.Errorf("at(5) = %v, value(50) = %v", s.at(5), s.value(50))
	}
	// A value axis runs backwards: its larger values are higher, at smaller y.
	v := newScale(0, 10, 200, 0)
	if v.at(10) != 0 || v.at(0) != 200 {
		t.Errorf("reversed scale maps 10 to %v and 0 to %v", v.at(10), v.at(0))
	}
	if flat := newScale(4, 4, 0, 100); flat.at(4) != 50 {
		t.Errorf("a domain of no width maps its value to %v, want the middle", flat.at(4))
	}
}

// The default number labels are whole numbers as they are, else two decimals with the zeros
// taken off.
func TestFormatNumber(t *testing.T) {
	cases := map[float64]string{120: "120", 0.5: "0.5", 1.2345: "1.23", 2.10: "2.1", -0.001: "0", -3: "-3", 1e15: "1000000000000000"}
	for v, want := range cases {
		if got := formatNumber(v); got != want {
			t.Errorf("formatNumber(%v) = %q, want %q", v, got, want)
		}
	}
}

// Time labels show the date alone at midnight, and the time of day when there is one.
func TestFormatTime(t *testing.T) {
	midnight := float64(time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC).Unix())
	if got := formatTime(midnight, time.UTC); got != "2026-10-13" {
		t.Errorf("midnight = %q", got)
	}
	noon := midnight + 12*3600 + 30*60
	if got := formatTime(noon, time.UTC); got != "2026-10-13 12:30" {
		t.Errorf("12:30 = %q", got)
	}
	if got := formatTime(noon+7, time.UTC); got != "2026-10-13 12:30:07" {
		t.Errorf("with seconds = %q", got)
	}
}

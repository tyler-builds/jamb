package charts

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// scale maps data values to pixels linearly. The domain d0 to d1 maps to the
// range r0 to r1, which may run backwards, as the vertical axis does: its
// pixels grow downwards.
type scale struct {
	d0, d1 float64
	r0, r1 float64
}

// newScale returns the scale from the domain d0 to d1 to the range r0 to r1. A
// domain of no width is widened by one on each side, so its values map to the
// middle of the range.
func newScale(d0, d1, r0, r1 float64) scale {
	if d1 == d0 {
		d0, d1 = d0-1, d1+1
	}
	return scale{d0: d0, d1: d1, r0: r0, r1: r1}
}

// at returns the pixel of the value v.
func (s scale) at(v float64) float64 {
	return s.r0 + (v-s.d0)/(s.d1-s.d0)*(s.r1-s.r0)
}

// value returns the value at the pixel px: the inverse of at.
func (s scale) value(px float64) float64 {
	return s.d0 + (px-s.r0)/(s.r1-s.r0)*(s.d1-s.d0)
}

// niceStep returns a step of 1, 2 or 5 times a power of ten that gives about n
// ticks across a span: the step nearest to span/(n-1) in that sequence.
func niceStep(span float64, n int) float64 {
	if n < 2 {
		n = 2
	}
	raw := span / float64(n-1)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	switch r := raw / mag; {
	case r < 1.5:
		return mag
	case r < 3:
		return 2 * mag
	case r < 7:
		return 5 * mag
	}
	return 10 * mag
}

// linearTicks returns about n nice ticks across lo to hi, the ticks inside it,
// and the bounds nlo and nhi that the first and last ticks reach. An axis whose
// domain is nlo to nhi starts and ends on a tick. A span of no width is padded
// so that the ticks still have a step.
func linearTicks(lo, hi float64, n int) (ticks []float64, nlo, nhi float64) {
	if hi-lo <= 0 {
		pad := math.Max(math.Abs(lo)*0.1, 1)
		lo, hi = lo-pad, hi+pad
	}
	step := niceStep(hi-lo, n)
	nlo = math.Floor(lo/step) * step
	nhi = math.Ceil(hi/step) * step
	kmin := math.Ceil(lo/step - 1e-9)
	kmax := math.Floor(hi/step + 1e-9)
	for k := kmin; k <= kmax; k++ {
		ticks = append(ticks, k*step)
	}
	return ticks, nlo, nhi
}

// timeUnit is the unit of a time step: seconds, days, months or years.
type timeUnit uint8

const (
	unitSecond timeUnit = iota + 1
	unitDay
	unitMonth
	unitYear
)

// timeStep is a step of time ticks: n of the unit, such as 6 hours (n of
// seconds) or 2 months.
type timeStep struct {
	unit timeUnit
	n    int
}

// timeSteps are the steps that time ticks take, from the shortest.
var timeSteps = []timeStep{
	{unitSecond, 1}, {unitSecond, 2}, {unitSecond, 5}, {unitSecond, 10}, {unitSecond, 15}, {unitSecond, 30},
	{unitSecond, 60}, {unitSecond, 120}, {unitSecond, 300}, {unitSecond, 900}, {unitSecond, 1800},
	{unitSecond, 3600}, {unitSecond, 7200}, {unitSecond, 10800}, {unitSecond, 21600}, {unitSecond, 43200},
	{unitDay, 1}, {unitDay, 2}, {unitDay, 7}, {unitMonth, 1}, {unitMonth, 2}, {unitMonth, 3}, {unitMonth, 6},
	{unitYear, 1}, {unitYear, 2}, {unitYear, 5}, {unitYear, 10}, {unitYear, 20}, {unitYear, 50}, {unitYear, 100},
}

// seconds returns the approximate length of the step in seconds.
func (s timeStep) seconds() float64 {
	switch s.unit {
	case unitDay:
		return float64(s.n) * 86400
	case unitMonth:
		return float64(s.n) * 2629800
	case unitYear:
		return float64(s.n) * 31557600
	}
	return float64(s.n)
}

// pickTimeStep returns the shortest time step that gives at most n ticks
// across span seconds.
func pickTimeStep(span float64, n int) timeStep {
	for _, s := range timeSteps {
		if span/s.seconds() <= float64(n-1) {
			return s
		}
	}
	return timeSteps[len(timeSteps)-1]
}

// timeTicks returns the ticks of a time axis from lo to hi, in Unix seconds,
// about n of them, and the step they are spaced by. Ticks fall on whole
// seconds, days, months or years of loc, so a day tick is at midnight.
func timeTicks(lo, hi float64, n int, loc *time.Location) ([]float64, timeStep) {
	st := pickTimeStep(hi-lo, n)
	var ticks []float64
	at := func(t time.Time) {
		if v := float64(t.Unix()); v >= lo && v <= hi {
			ticks = append(ticks, v)
		}
	}
	start := time.Unix(int64(math.Floor(lo)), 0).In(loc)
	switch st.unit {
	case unitSecond:
		day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
		step := float64(st.n)
		first := float64(day.Unix()) + math.Ceil((lo-float64(day.Unix()))/step)*step
		for v := first; v <= hi; v += step {
			ticks = append(ticks, v)
		}
	case unitDay:
		t := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
		for ; float64(t.Unix()) < lo; t = t.AddDate(0, 0, st.n) {
		}
		for ; float64(t.Unix()) <= hi; t = t.AddDate(0, 0, st.n) {
			at(t)
		}
	case unitMonth:
		m := int(start.Year())*12 + int(start.Month()) - 1
		m -= m % st.n
		t := time.Date(m/12, time.Month(m%12+1), 1, 0, 0, 0, 0, loc)
		for ; float64(t.Unix()) < lo; t = t.AddDate(0, st.n, 0) {
		}
		for ; float64(t.Unix()) <= hi; t = t.AddDate(0, st.n, 0) {
			at(t)
		}
	case unitYear:
		y := start.Year()
		y -= y % st.n
		t := time.Date(y, 1, 1, 0, 0, 0, 0, loc)
		for ; float64(t.Unix()) < lo; t = t.AddDate(st.n, 0, 0) {
		}
		for ; float64(t.Unix()) <= hi; t = t.AddDate(st.n, 0, 0) {
			at(t)
		}
	}
	return ticks, st
}

// timeTickFormat is the layout of a time tick's label for a step: the time of
// day for steps shorter than a day, the date for days, the month and the year.
func timeTickFormat(st timeStep) string {
	switch st.unit {
	case unitSecond:
		if st.n < 60 {
			return "15:04:05"
		}
		return "15:04"
	case unitDay:
		return "2006-01-02"
	case unitMonth:
		return "2006-01"
	}
	return "2006"
}

// formatTime returns the label of the time at the Unix seconds v: the date
// when it is midnight, else the date and the time to the minute, or to the
// second when it has seconds.
func formatTime(v float64, loc *time.Location) string {
	t := time.Unix(int64(math.Floor(v)), 0).In(loc)
	switch {
	case t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0:
		return t.Format("2006-01-02")
	case t.Second() == 0:
		return t.Format("2006-01-02 15:04")
	}
	return t.Format("2006-01-02 15:04:05")
}

// formatNumber returns the default label of a number: a whole number as it is,
// else up to two decimals with the trailing zeros taken off.
func formatNumber(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 0) {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	s := strconv.FormatFloat(v, 'f', 2, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	if s == "-0" {
		return "0"
	}
	return s
}

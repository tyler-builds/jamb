package ui

import (
	"math"
	"testing"
)

// Helpers for the layout tables: a sized pane of a pixel or a percentage size, and a fill pane, with
// no limits unless a case sets them.

func pxPane(v float32) paneIn {
	return paneIn{sized: true, size: Px(v), hi: math.Inf(1)}
}

func pctPane(v float32) paneIn {
	return paneIn{sized: true, size: Percent(v), hi: math.Inf(1)}
}

func fillPane() paneIn {
	return paneIn{weight: 1, hi: math.Inf(1)}
}

// layoutCase is one case of a resolvePanes table. shown is nil when it is the same as base.
type layoutCase struct {
	name  string
	panes []paneIn
	avail float64
	base  []float64
	shown []float64
}

// TestResolvePanesTable checks the sizes of panes in a split, by px, percentage and fill, over and under
// the length, and with limits.
func TestResolvePanesTable(t *testing.T) {
	hidden := pxPane(100)
	hidden.hidden = true
	cases := []layoutCase{
		{name: "px and fill", panes: []paneIn{pxPane(200), fillPane()}, avail: 491, base: []float64{200, 291}},
		{name: "percent and fill", panes: []paneIn{pctPane(40), fillPane()}, avail: 491, base: []float64{196.4, 294.6}},
		{name: "three panes: px, percent, fill", panes: []paneIn{pxPane(100), pctPane(20), fillPane()}, avail: 482,
			base: []float64{100, 96.4, 285.6}},
		{name: "two fill panes share equally", panes: []paneIn{fillPane(), fillPane()}, avail: 400, base: []float64{200, 200}},
		{name: "fill panes share by weight", panes: []paneIn{weighted(1), weighted(3)}, avail: 400, base: []float64{100, 300}},
		{name: "fill clamped to its most", panes: []paneIn{limited(fillPane(), 0, 150), fillPane()}, avail: 400,
			base: []float64{150, 250}},
		{name: "fill clamped to its least", panes: []paneIn{limited(fillPane(), 300, math.Inf(1)), fillPane()}, avail: 400,
			base: []float64{300, 100}},
		{name: "sized clamped to its most", panes: []paneIn{limited(pxPane(300), 0, 200), fillPane()}, avail: 400,
			base: []float64{200, 200}},
		{name: "sized clamped to its least", panes: []paneIn{limited(pxPane(100), 150, math.Inf(1)), fillPane()}, avail: 400,
			base: []float64{150, 250}},
		{name: "percent clamped to its most", panes: []paneIn{limited(pctPane(60), 0, 196.4), fillPane()}, avail: 491,
			base: []float64{196.4, 294.6}},
		{name: "least wins over a most below it", panes: []paneIn{limited(pxPane(250), 300, 300), fillPane()}, avail: 491,
			base: []float64{300, 191}},
		{name: "under: the last pane takes the rest for display", panes: []paneIn{pxPane(100), pctPane(10), pxPane(50), pxPane(60)},
			avail: 400, base: []float64{100, 40, 50, 60}, shown: []float64{100, 40, 50, 210}},
		{name: "under: a single sized pane takes the length", panes: []paneIn{pxPane(200)}, avail: 491,
			base: []float64{200}, shown: []float64{491}},
		{name: "under: a fill at its most leaves the rest to the last pane", panes: []paneIn{limited(fillPane(), 0, 100)}, avail: 400,
			base: []float64{100}, shown: []float64{400}},
		{name: "over: fill panes go to their least first", panes: []paneIn{pxPane(300), pxPane(300), limited(fillPane(), 100, math.Inf(1))},
			avail: 400, base: []float64{150, 150, 100}},
		{name: "over: sized panes shrink toward their least", panes: []paneIn{limited(pxPane(300), 200, math.Inf(1)), pxPane(300)},
			avail: 400, base: []float64{250, 150}},
		{name: "over: content that does not fit is clipped", panes: []paneIn{limited(pxPane(300), 300, math.Inf(1)), limited(pxPane(300), 300, math.Inf(1))},
			avail: 400, base: []float64{300, 300}},
		{name: "over: no length at all", panes: []paneIn{pxPane(100), fillPane()}, avail: 0, base: []float64{0, 0}},
		{name: "a collapsed pane takes no space", panes: []paneIn{hidden, fillPane()}, avail: 400, base: []float64{0, 400}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base, shown := resolvePanes(c.panes, c.avail)
			checkSizes(t, "base", base, c.base)
			want := c.shown
			if want == nil {
				want = c.base
			}
			checkSizes(t, "shown", shown, want)
		})
	}
}

// weighted returns a fill pane with the weight w.
func weighted(w float64) paneIn {
	p := fillPane()
	p.weight = w
	return p
}

// limited returns p with the least lo and the most hi.
func limited(p paneIn, lo, hi float64) paneIn {
	p.lo, p.hi = lo, hi
	return p
}

// checkSizes fails the test when got differs from want by more than half a pixel in any pane.
func checkSizes(t *testing.T, what string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d sizes, want %d (%v)", what, len(got), len(want), want)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 0.5 {
			t.Fatalf("%s: %v, want %v", what, got, want)
		}
	}
}

// TestPaneLimitsPixelsAndPercent checks the limits of a pane in both units, against a length of avail
// pixels, and that the least wins when the two contradict each other.
func TestPaneLimitsPixelsAndPercent(t *testing.T) {
	cases := []struct {
		name           string
		min, max       Length
		noMax          bool
		wantLo, wantHi float64
	}{
		{name: "pixels", min: Px(120), max: Px(320), wantLo: 120, wantHi: 320},
		{name: "percent", min: Percent(20), max: Percent(40), wantLo: 98.2, wantHi: 196.4},
		{name: "no most", min: Px(0), noMax: true, wantLo: 0, wantHi: math.Inf(1)},
		{name: "least wins", min: Px(300), max: Px(200), wantLo: 300, wantHi: 300},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := Pane(Div()).Min(c.min)
			if !c.noMax {
				p.Max(c.max)
			}
			lo, hi := paneLimits(p, 491)
			if math.Abs(lo-c.wantLo) > 0.01 || !sameLimit(hi, c.wantHi) {
				t.Fatalf("limits %v, %v; want %v, %v", lo, hi, c.wantLo, c.wantHi)
			}
		})
	}
}

// sameLimit reports whether a and b are the same limit: equal within 0.01, or both infinite.
func sameLimit(a, b float64) bool {
	if math.IsInf(a, 1) || math.IsInf(b, 1) {
		return math.IsInf(a, 1) && math.IsInf(b, 1)
	}
	return math.Abs(a-b) <= 0.01
}

// TestPaneLengthKindsPanic checks that a size or a limit that is not a pixel or a percentage length
// panics, as a programming error, and so does a split whose panes share a key.
func TestPaneLengthKindsPanic(t *testing.T) {
	cases := map[string]func(){
		"Size(Auto)": func() { Pane(Div()).Size(Auto) },
		"Size(Fr)":   func() { Pane(Div()).Size(Fr(1)) },
		"Min(Auto)":  func() { Pane(Div()).Min(Auto) },
		"Min(Fr)":    func() { Pane(Div()).Min(Fr(1)) },
		"Max(Auto)":  func() { Pane(Div()).Max(Auto) },
		"Max(Fr)":    func() { Pane(Div()).Max(Fr(1)) },
		"nil signal": func() { Pane(Div()).SizeSignal(nil) },
		"nil pane":   func() { Split(nil) },
		"same keys":  func() { Split(Pane(Div()).Key("a"), Pane(Div()).Key("a")) },
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s did not panic", name)
				}
			}()
			f()
		})
	}
}

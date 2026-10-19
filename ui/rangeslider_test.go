package ui

import (
	"math"
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// rangeRig is a range slider named Price, from 0 to 100 in steps of 5, 300 px
// wide, holding low and high; build adds options.
func rangeRig(low, high float64, build func(*RangeSliderElement) *RangeSliderElement) (*accessRig, *Signal[float64], *Signal[float64], *[][2]float64) {
	lo, hi := NewSignal(low), NewSignal(high)
	changes := &[][2]float64{}
	r := newAccessRig(func(ctx *Context) Element {
		rs := RangeSlider(lo, hi, 0, 100, 5).Label("Price").Key("price").Width(Px(300)).
			OnChange(func(l, h float64) { *changes = append(*changes, [2]float64{l, h}) })
		if build != nil {
			rs = build(rs)
		}
		return Div().Column().Child(rs)
	})
	return r, lo, hi, changes
}

// thumbNodes returns the low and the high thumb.
func thumbNodes(t *testing.T, r *accessRig) (*node, *node) {
	t.Helper()
	low := accessNode(t, r, access.RoleSlider, "Minimum")
	high := accessNode(t, r, access.RoleSlider, "Maximum")
	return low, high
}

// Assistive technology sees a group named by the label that holds two sliders,
// Minimum and Maximum, each with its range and its value text.
func TestRangeSliderAccess(t *testing.T) {
	r, _, _, _ := rangeRig(20, 80, func(s *RangeSliderElement) *RangeSliderElement {
		return s.Format(func(v float64) string { return "$" + strconv.FormatFloat(v, 'f', -1, 64) })
	})
	tr := r.tree()
	group := accesstest.Find(t, tr, access.RoleGroup, "Price")
	kids := accesstest.Children(tr, group)
	if len(kids) != 2 {
		t.Fatalf("the group holds %q, want the two thumbs", kids)
	}
	for _, c := range []struct {
		name  string
		value float64
		text  string
	}{{"Minimum", 20, "$20"}, {"Maximum", 80, "$80"}} {
		n := accesstest.Find(t, tr, access.RoleSlider, c.name)
		if n.Range == nil || n.Range.Value != c.value || n.Range.Min != 0 || n.Range.Max != 100 || n.Range.SmallChange != 5 {
			t.Errorf("%s: range %+v, want %v in 0 to 100 by 5", c.name, n.Range, c.value)
		}
		if n.Value != c.text {
			t.Errorf("%s: value text %q, want %q", c.name, n.Value, c.text)
		}
		if !n.States.Has(access.StateFocusable) {
			t.Errorf("%s is not a Tab stop", c.name)
		}
	}
}

// Each thumb takes the keys of a slider, and stops at the other thumb.
func TestRangeSliderKeys(t *testing.T) {
	r, lo, hi, changes := rangeRig(20, 80, nil)
	low, high := thumbNodes(t, r)
	focusNode(r, low)
	r.press(input.KeyRight, 0)
	if lo.Get() != 25 {
		t.Errorf("Right moved the low value to %v, want 25", lo.Get())
	}
	if got := *changes; len(got) != 1 || got[0] != [2]float64{25, 80} {
		t.Errorf("OnChange ran with %v, want 25 and 80", got)
	}
	r.press(input.KeyPageUp, 0)
	if lo.Get() != 35 {
		t.Errorf("Page Up moved the low value to %v, want 35", lo.Get())
	}
	r.press(input.KeyEnd, 0)
	if lo.Get() != 80 {
		t.Errorf("End moved the low value to %v, want the high value 80", lo.Get())
	}
	r.press(input.KeyRight, 0)
	if lo.Get() != 80 {
		t.Errorf("Right past the high thumb moved the low value to %v", lo.Get())
	}
	r.press(input.KeyHome, 0)
	if lo.Get() != 0 {
		t.Errorf("Home moved the low value to %v, want 0", lo.Get())
	}

	_, high = thumbNodes(t, r)
	focusNode(r, high)
	r.press(input.KeyEnd, 0)
	r.press(input.KeyDown, 0)
	if hi.Get() != 95 {
		t.Errorf("End and Down moved the high value to %v, want 95", hi.Get())
	}
	r.press(input.KeyHome, 0)
	if hi.Get() != 0 {
		t.Errorf("Home moved the high value to %v, want the low value 0", hi.Get())
	}
}

// MinGap keeps the thumbs apart.
func TestRangeSliderMinGap(t *testing.T) {
	r, lo, hi, _ := rangeRig(20, 80, func(s *RangeSliderElement) *RangeSliderElement { return s.MinGap(10) })
	low, _ := thumbNodes(t, r)
	focusNode(r, low)
	r.press(input.KeyEnd, 0)
	if lo.Get() != 70 {
		t.Errorf("End moved the low value to %v, want 70, 10 below the high value", lo.Get())
	}
	_, high := thumbNodes(t, r)
	focusNode(r, high)
	r.press(input.KeyHome, 0)
	if hi.Get() != 80 {
		t.Errorf("Home moved the high value to %v, want it to stay at 80", hi.Get())
	}
	// Assistive technology cannot cross the thumbs either.
	n := accesstest.Find(t, r.tree(), access.RoleSlider, "Maximum")
	r.bridge.Do(n.ID, access.Request{Action: access.ActionSetRangeValue, X: 10})
	r.frame()
	if hi.Get() != 80 {
		t.Errorf("setting the high value to 10 gave %v, want 80", hi.Get())
	}
}

// A press on the track moves the nearer thumb there and focuses it, and a drag
// moves it on. The fill runs from the center of one thumb to the center of the
// other.
func TestRangeSliderPointer(t *testing.T) {
	r, lo, hi, _ := rangeRig(20, 80, nil)
	track := partNode(t, r.rt, "RangeSlider.Track", "")
	rc := r.rt.nodeRect(track)
	y := rc.Y + rc.H/2

	// The fill's ends are the thumbs' centers.
	low, high := thumbNodes(t, r)
	fill := partNode(t, r.rt, "RangeSlider.Fill", "")
	fr, lr, hr := r.rt.nodeRect(fill), r.rt.nodeRect(low), r.rt.nodeRect(high)
	if d := math.Abs(float64(fr.X - (lr.X + lr.W/2))); d > 0.5 {
		t.Errorf("the fill starts %v px from the low thumb's center", d)
	}
	if d := math.Abs(float64(fr.X + fr.W - (hr.X + hr.W/2))); d > 0.5 {
		t.Errorf("the fill ends %v px from the high thumb's center", d)
	}

	// Near the right end: the high thumb moves there and takes focus.
	r.clickAt(rc.X+rc.W-2, y)
	if hi.Get() != 100 || lo.Get() != 20 {
		t.Errorf("a press at the right end gave %v to %v, want 20 to 100", lo.Get(), hi.Get())
	}
	if _, high = thumbNodes(t, r); r.rt.focusID != high.id {
		t.Error("the press did not focus the high thumb")
	}
	// Near the left end, then a drag to the middle.
	r.rt.PointerMove(rc.X+2, y, 0)
	r.rt.PointerDown(rc.X+2, y, input.ButtonLeft, 1, 0)
	r.frame()
	if lo.Get() != 0 {
		t.Errorf("a press at the left end gave the low value %v, want 0", lo.Get())
	}
	r.rt.PointerMove(rc.X+rc.W/2, y, 0)
	r.frame()
	r.rt.PointerUp(rc.X+rc.W/2, y, input.ButtonLeft, 0)
	r.frame()
	if got := lo.Get(); got < 45 || got > 55 {
		t.Errorf("a drag to the middle gave the low value %v, want about 50", got)
	}
	if low, _ = thumbNodes(t, r); r.rt.focusID != low.id {
		t.Error("the drag did not focus the low thumb")
	}
}

// A disabled range slider takes no focus and no input.
func TestRangeSliderDisabled(t *testing.T) {
	r, lo, _, _ := rangeRig(20, 80, func(s *RangeSliderElement) *RangeSliderElement { return s.Disabled(true) })
	tr := r.tree()
	for _, name := range []string{"Minimum", "Maximum"} {
		n := accesstest.Find(t, tr, access.RoleSlider, name)
		if !n.States.Has(access.StateDisabled) || n.States.Has(access.StateFocusable) {
			t.Errorf("%s of a disabled range slider has states %v", name, n.States)
		}
	}
	rc := r.rt.nodeRect(partNode(t, r.rt, "RangeSlider.Track", ""))
	r.clickAt(rc.X+2, rc.Y+rc.H/2)
	if lo.Get() != 20 {
		t.Errorf("a press on a disabled range slider changed the low value to %v", lo.Get())
	}
}

// partRangeSlider is a range slider with the instance style st.
func partRangeSlider(st RangeSliderStyle) Element {
	return Div().Column().Child(RangeSlider(NewSignal(20.0), NewSignal(80.0), 0, 100, 5).Label("Price").Key("price").Styles(st))
}

func testRangeSliderParts(t *testing.T, applied map[string]bool) {
	type rangeCase struct {
		part  string
		set   func(*RangeSliderStyle, Style)
		drawn string
		text  bool
		act   func(*testing.T, *accessRig)
	}
	firstThumb := func(t *testing.T, r *accessRig) *node { return accessNode(t, r, access.RoleSlider, "Minimum") }
	cases := []rangeCase{
		{part: "RangeSlider.Track", set: func(s *RangeSliderStyle, m Style) { s.Track = m }, drawn: "RangeSlider.Track"},
		{part: "RangeSlider.Rail", set: func(s *RangeSliderStyle, m Style) { s.Rail = m }, drawn: "RangeSlider.Rail"},
		{part: "RangeSlider.Fill", set: func(s *RangeSliderStyle, m Style) { s.Fill = m }, drawn: "RangeSlider.Fill"},
		{part: "RangeSlider.Thumb", set: func(s *RangeSliderStyle, m Style) { s.Thumb = m }, drawn: "RangeSlider.Thumb"},
		{part: "RangeSlider.ThumbHover", set: func(s *RangeSliderStyle, m Style) { s.ThumbHover = m }, drawn: "RangeSlider.Thumb",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, firstThumb(t, r)) }},
		{part: "RangeSlider.ThumbPressed", set: func(s *RangeSliderStyle, m Style) { s.ThumbPressed = m }, drawn: "RangeSlider.Thumb",
			act: func(t *testing.T, r *accessRig) { pressNode(r, firstThumb(t, r)) }},
		{part: "RangeSlider.ThumbFocusVisible", set: func(s *RangeSliderStyle, m Style) { s.ThumbFocusVisible = m }, drawn: "RangeSlider.Thumb",
			act: func(t *testing.T, r *accessRig) { focusNode(r, firstThumb(t, r)) }},
		{part: "RangeSlider.ValueText", set: func(s *RangeSliderStyle, m Style) { s.ValueText = m }, drawn: "RangeSlider.ValueText", text: true},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st RangeSliderStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return partRangeSlider(st) })
			if c.act != nil {
				c.act(t, r)
			}
			var n *node
			if c.drawn == "RangeSlider.Thumb" {
				n = firstThumb(t, r)
			} else {
				n = partNode(t, r.rt, c.drawn, "")
			}
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

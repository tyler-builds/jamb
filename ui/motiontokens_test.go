package ui

import (
	"testing"
	"time"
)

// The motion tokens default to the built-in timings, and a theme's Enter
// changes how long a menu takes to appear.
func TestMotionTokens(t *testing.T) {
	var zero MotionTokens
	if zero.hover() != 110*time.Millisecond || zero.enter(140*time.Millisecond) != 140*time.Millisecond ||
		zero.exit(80*time.Millisecond) != 80*time.Millisecond {
		t.Errorf("zero tokens give %v, %v, %v; want the defaults 110ms, 140ms, 80ms",
			zero.hover(), zero.enter(140*time.Millisecond), zero.exit(80*time.Millisecond))
	}
	opacityAfter := func(theme *Theme, d time.Duration) float32 {
		t.Helper()
		r := dropdownRig(func() []MenuItem { return sampleMenu(new([]string)) }, false)
		r.rt.SetTheme(theme)
		r.frame()
		x, y := r.center(t, "trigger")
		r.click(x, y)
		for elapsed := time.Duration(0); elapsed < d; elapsed += 16 * time.Millisecond {
			r.now = r.now.Add(16 * time.Millisecond)
			r.frame()
		}
		s, ok := r.rt.StyleOf("jamb.menu")
		if !ok {
			t.Fatal("the menu did not open")
		}
		return s.Opacity
	}
	if o := opacityAfter(DarkTheme(), 160*time.Millisecond); o != 1 {
		t.Errorf("with the default tokens the menu has opacity %v after 160 ms, want 1 (140 ms entrance)", o)
	}
	slow := DarkTheme()
	slow.Motion.Enter = 400 * time.Millisecond
	if o := opacityAfter(slow, 160*time.Millisecond); o >= 0.9 {
		t.Errorf("with Motion.Enter 400 ms the menu has opacity %v after 160 ms, want it still entering", o)
	}
	if o := opacityAfter(slow, 416*time.Millisecond); o != 1 {
		t.Errorf("with Motion.Enter 400 ms the menu has opacity %v after 416 ms, want 1", o)
	}
}

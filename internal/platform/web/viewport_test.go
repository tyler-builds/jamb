//go:build js

package web

import (
	"syscall/js"
	"testing"
)

// The canvas covers what shows of the page, within the safe area: it ends at
// the keyboard, follows the page when the browser pans it, and stays out of
// the insets of the display.
func TestFitBox(t *testing.T) {
	notch := insets{top: 47, bottom: 34}
	for _, c := range []struct {
		name    string
		visible pageBox
		safe    insets
		want    pageBox
	}{
		{"the whole page", pageBox{0, 0, 390, 844}, insets{}, pageBox{0, 0, 390, 844}},
		{"the keyboard is up", pageBox{0, 0, 390, 508}, insets{}, pageBox{0, 0, 390, 508}},
		{"the browser panned the page up", pageBox{0, 120, 390, 508}, insets{}, pageBox{0, 120, 390, 508}},
		{"a notch and a home indicator", pageBox{0, 0, 390, 844}, notch, pageBox{0, 47, 390, 763}},
		// The keyboard covers the home indicator, so its inset takes nothing more.
		{"the keyboard is up over the home indicator", pageBox{0, 0, 390, 508}, notch, pageBox{0, 47, 390, 461}},
		{"a notch at the side", pageBox{0, 0, 844, 390}, insets{left: 47, right: 47, bottom: 21},
			pageBox{47, 0, 750, 369}},
		{"nothing shows", pageBox{0, 0, 0, 0}, notch, pageBox{0, 47, 1, 1}},
	} {
		pageW, pageH := 390.0, 844.0
		if c.visible.width > pageW {
			pageW, pageH = pageH, pageW
		}
		if got := fitBox(c.visible, pageW, pageH, c.safe); got != c.want {
			t.Errorf("%s: the canvas is at %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCSSPx(t *testing.T) {
	for s, want := range map[string]float64{"47px": 47, "20.5px": 20.5, "0px": 0, "": 0, "auto": 0, "-3px": 0, "1.2.3px": 0} {
		if got := cssPx(js.ValueOf(s)); got != want {
			t.Errorf("cssPx(%q) = %v, want %v", s, got, want)
		}
	}
	if got := cssPx(js.Undefined()); got != 0 {
		t.Errorf("cssPx of no value = %v", got)
	}
}

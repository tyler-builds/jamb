//go:build windows

package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/internal/platform"
)

// menuHandler is a Handler that reports whether its window shows a menu bar.
// Only HasMenuBar is used by the tests; the embedded nil Handler is never
// called.
type menuHandler struct {
	platform.Handler
	bar bool
}

func (h menuHandler) HasMenuBar() bool { return h.bar }

// A lone Alt press is taken by a window's menu bar only when the window has
// one. A handler without the optional interface keeps the system menu.
func TestLoneAltKeepsSystemMenuWithoutMenuBar(t *testing.T) {
	cases := []struct {
		name string
		h    platform.Handler
		want bool
	}{
		{"menu bar", menuHandler{bar: true}, true},
		{"no menu bar", menuHandler{bar: false}, false},
		{"handler without a menu bar interface", struct{ platform.Handler }{}, false},
	}
	for _, c := range cases {
		if got := showsMenuBar(c.h); got != c.want {
			t.Errorf("%s: showsMenuBar = %v, want %v", c.name, got, c.want)
		}
	}
}

// A WM_PAINT repaints unless the window showed a frame at its current size with
// no animation running. A frame that was not shown, such as one drawn while the
// window was covered or hidden, leaves no size to match, so the window repaints
// once it is uncovered or shown.
func TestRepaintNeeded(t *testing.T) {
	cases := []struct {
		name                          string
		shownW, shownH, width, height int
		frameWanted                   bool
		want                          bool
	}{
		{"shown at this size", 800, 600, 800, 600, false, false},
		{"shown at another size", 800, 600, 820, 600, false, true},
		{"not shown", 0, 0, 800, 600, false, true},
		{"animating", 800, 600, 800, 600, true, true},
		{"no client area", 0, 0, 0, 0, false, true},
		{"shown, then minimized", 800, 600, 0, 0, false, true},
	}
	for _, c := range cases {
		if got := repaintNeeded(c.shownW, c.shownH, c.width, c.height, c.frameWanted); got != c.want {
			t.Errorf("%s: repaintNeeded = %v, want %v", c.name, got, c.want)
		}
	}
}

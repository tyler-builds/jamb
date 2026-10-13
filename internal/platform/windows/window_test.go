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

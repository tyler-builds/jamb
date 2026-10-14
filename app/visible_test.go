package app

import "testing"

// TestDrawWanted checks the frame decision for the window states: shown, hidden,
// minimized (which the platform reports as not visible, with no client area) and
// shown with no client area.
func TestDrawWanted(t *testing.T) {
	for name, tc := range map[string]struct {
		visible       bool
		width, height int
		want          bool
	}{
		"shown":                {true, 1024, 720, true},
		"hidden":               {false, 1024, 720, false},
		"minimized":            {false, 0, 0, false},
		"shown without a size": {true, 0, 0, false},
		"shown, zero width":    {true, 0, 720, false},
	} {
		if got := drawWanted(tc.visible, tc.width, tc.height); got != tc.want {
			t.Errorf("%s: drawWanted = %t; want %t", name, got, tc.want)
		}
	}
}

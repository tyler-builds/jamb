package app

import (
	"testing"

	"github.com/tyler-builds/jamb/internal/scene"
)

// TestDrawWanted checks the frame decision for the window states: shown, hidden,
// minimized (which the platform reports as not visible, with no client area) and
// shown with no client area. A hidden window draws its first frame, before it is
// shown, and nothing after that.
func TestDrawWanted(t *testing.T) {
	for name, tc := range map[string]struct {
		visible, drawn bool
		width, height  int
		want           bool
	}{
		"shown":                         {true, true, 1024, 720, true},
		"shown, first frame":            {true, false, 1024, 720, true},
		"hidden":                        {false, true, 1024, 720, false},
		"hidden, first frame":           {false, false, 1024, 720, true},
		"hidden, first frame, no size":  {false, false, 0, 0, false},
		"minimized":                     {false, true, 0, 0, false},
		"shown without a size":          {true, true, 0, 0, false},
		"shown, zero width":             {true, true, 0, 720, false},
		"hidden, first frame, zero hgt": {false, false, 1024, 0, false},
	} {
		if got := drawWanted(tc.visible, tc.drawn, tc.width, tc.height); got != tc.want {
			t.Errorf("%s: drawWanted = %t; want %t", name, got, tc.want)
		}
	}
}

// The first frame's scene is used only for the size it was built for: a window
// that has been resized since needs a new one, and a missing scene is built.
func TestPreSceneFits(t *testing.T) {
	built := &scene.Scene{}
	for name, tc := range map[string]struct {
		pre           *scene.Scene
		preW, preH    int
		width, height int
		want          bool
	}{
		"same size":     {built, 800, 600, 800, 600, true},
		"resized":       {built, 780, 600, 800, 600, false},
		"nothing built": {nil, 800, 600, 800, 600, false},
	} {
		if got := preSceneFits(tc.pre, tc.preW, tc.preH, tc.width, tc.height); got != tc.want {
			t.Errorf("%s: preSceneFits = %t, want %t", name, got, tc.want)
		}
	}
}

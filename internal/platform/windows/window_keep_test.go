package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/internal/win32"
)

// TestKeepOnScreenMovesAWindowInFromTheEdges checks the corner that keepOnScreen gives a
// window: a window that fits is kept where it is, one that sticks out of an edge is moved in,
// and one larger than the work area keeps its top-left corner.
func TestKeepOnScreenMovesAWindowInFromTheEdges(t *testing.T) {
	work := win32.RECT{Left: 0, Top: 0, Right: 1920, Bottom: 1040}
	cases := []struct {
		name         string
		x, y, w, h   int32
		wantX, wantY int32
	}{
		{"fits", 100, 200, 480, 360, 100, 200},
		{"past the right edge", 1800, 200, 480, 360, 1440, 200},
		{"past the bottom edge", 100, 900, 480, 360, 100, 680},
		{"above and left of the area", -50, -20, 480, 360, 0, 0},
		{"larger than the area", 1500, 900, 2400, 1200, 0, 0},
	}
	for _, c := range cases {
		x, y := keepOnScreen(c.x, c.y, c.w, c.h, work)
		if x != c.wantX || y != c.wantY {
			t.Errorf("%s: got (%d, %d), want (%d, %d)", c.name, x, y, c.wantX, c.wantY)
		}
	}
}

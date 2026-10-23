package views

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// The details pane resizes from its divider: from the 1 px bar itself and from
// a few pixels either side of it, over the edges of the panes.
func TestDetailPaneResizes(t *testing.T) {
	u, _, _ := open(t)
	rt := u.Runtime()
	for _, off := range []float32{0, -3, 3} {
		b := u.ByRole(access.RoleSeparator, "Resize details").Bounds()
		before := u.ByRole(access.RoleGroup, "Commit details").Bounds().W
		x, y := b.X+b.W/2+off, b.Y+b.H/2
		rt.PointerMove(x, y, 0)
		rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
		u.Frame()
		rt.PointerMove(x-60, y, 0)
		u.Frame()
		rt.PointerUp(x-60, y, input.ButtonLeft, 0)
		u.Frame()
		if after := u.ByRole(access.RoleGroup, "Commit details").Bounds().W; after != before+60 {
			t.Errorf("a drag at %v px from the bar took the details from %v to %v px, want %v", off, before, after, before+60)
		}
	}
}

package views

import "testing"

// Each row's graph canvas must be as tall as the row, so its lines meet the
// next row's (at zero height nothing is drawn at all).
func TestGraphFillsTheRow(t *testing.T) {
	u, _, r := open(t)
	entries := r.Entries.Peek()
	for i, e := range entries {
		g := u.Get("graph-" + e.Hash).Bounds()
		if g.H != 36 || g.W <= 0 {
			t.Errorf("%s: graph is %vx%v, want 36 px high", e.Short(), g.W, g.H)
		}
		if i+1 < len(entries) {
			if next := u.Get("graph-" + entries[i+1].Hash).Bounds(); next.Y != g.Y+g.H {
				t.Errorf("%s: graph ends at %v, the next starts at %v", e.Short(), g.Y+g.H, next.Y)
			}
		}
	}
}

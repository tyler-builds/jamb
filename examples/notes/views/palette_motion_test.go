package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
)

// paletteQuads returns the solid quads (background and border, not the shadow)
// of the palette panel, which is 520 wide, or a little narrower while it is
// scaled in or out.
func paletteQuads(r *rig) []scene.Quad {
	var out []scene.Quad
	for _, q := range frameScene(r.rt, time.Now()).Quads {
		if q.Blur == 0 && q.Bounds.W > 480 && q.Bounds.W <= 520 {
			out = append(out, q)
		}
	}
	return out
}

// waitForMotion renders frames until done reports true, and fails the test if
// that does not happen within the deadline. The motion runs on the runtime's
// clock, so the test waits for the state it checks rather than for a fixed time:
// a fixed wait is too short on a loaded machine.
func waitForMotion(t *testing.T, r *rig, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not happen within 10 s", what)
		}
		time.Sleep(5 * time.Millisecond)
		r.frame()
	}
}

// The palette fades in and rises into place over about 140 ms, and when it
// closes it stays drawn for about 90 ms without being in the tree.
func TestPaletteEnterAndExit(t *testing.T) {
	r := newRig(t)
	openPaletteKey(r)
	qs := paletteQuads(r)
	if len(qs) == 0 {
		t.Fatal("no palette quads on the first frame")
	}
	for _, q := range qs {
		if q.Background.A >= 0.5 {
			t.Fatalf("palette quad alpha %v on its first frame, want a faded-out start", q.Background.A)
		}
	}
	top, _ := r.rt.Find("palette")
	opaque := func() bool {
		for _, q := range paletteQuads(r) {
			if q.Background.A < 0.99 {
				return false
			}
		}
		return len(paletteQuads(r)) > 0
	}
	waitForMotion(t, r, "the palette became fully opaque", func() bool {
		settled, ok := r.rt.Find("palette")
		return ok && opaque() && settled.Y-top.Y >= 5
	})
	settled, _ := r.rt.Find("palette")
	if d := settled.Y - top.Y; d < 5 || d > 7 {
		t.Errorf("palette moved %v from its first frame to its resting place, want about 6", d)
	}

	r.key(input.KeyEscape, 0)
	if _, ok := r.rt.Find("palette"); ok {
		t.Error("the palette is still in the tree after it was closed")
	}
	if len(paletteQuads(r)) == 0 {
		t.Error("the closing palette is not drawn during its exit")
	}
	waitForMotion(t, r, "the palette's exit ended", func() bool { return len(paletteQuads(r)) == 0 })
}

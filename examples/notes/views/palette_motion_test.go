package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/scene"
)

// paletteQuads returns the solid quads (background and border, not the shadow)
// of the palette panel, which is 520 wide, or a little narrower while it is
// scaled in or out.
func paletteQuads(r *rig) []scene.Quad {
	var out []scene.Quad
	for _, q := range r.rt.Frame(time.Now()).Quads {
		if q.Blur == 0 && q.Bounds.W > 480 && q.Bounds.W <= 520 {
			out = append(out, q)
		}
	}
	return out
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
	deadline := time.Now().Add(220 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		r.frame()
	}
	for _, q := range paletteQuads(r) {
		if q.Background.A < 0.99 {
			t.Fatalf("palette quad alpha %v after 220 ms, want fully opaque", q.Background.A)
		}
	}
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
	time.Sleep(120 * time.Millisecond)
	r.frame()
	if len(paletteQuads(r)) != 0 {
		t.Error("the palette is still drawn after its exit ended")
	}
}

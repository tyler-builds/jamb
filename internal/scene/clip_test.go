package scene

import (
	"fmt"
	"slices"
	"testing"
)

// recorder logs the drawing calls Replay makes, in order.
type recorder struct{ calls []string }

func (r *recorder) DrawQuads(qs []Quad) { r.calls = append(r.calls, fmt.Sprintf("quads %d", len(qs))) }
func (r *recorder) DrawGlyphs(g GlyphRun) {
	r.calls = append(r.calls, fmt.Sprintf("glyphs %d", len(g.Glyphs)))
}
func (r *recorder) DrawImages(ims []ImageDraw) {
	r.calls = append(r.calls, fmt.Sprintf("images %d", len(ims)))
}
func (r *recorder) PushClip(c Rect) { r.calls = append(r.calls, "clip") }
func (r *recorder) PushRoundedClip(c Rect, rd Corners) {
	r.calls = append(r.calls, fmt.Sprintf("rounded %v", rd))
}
func (r *recorder) PopClip() { r.calls = append(r.calls, "pop") }

// A rounded clip keeps its corners, and a transform scales them with the
// rectangle. A plain clip carries no corners.
func TestRoundedClipIsScaledAndReplayed(t *testing.T) {
	var s Scene
	s.Reset()
	s.PushClip(Rect{1, 2, 3, 4})
	s.PopClip()
	s.PushTransform(Transform{Scale: 2, DX: 10, DY: 20})
	s.PushRoundedClip(Rect{1, 2, 3, 4}, Corners{TL: 1, TR: 2, BR: 3, BL: 4})
	s.PopClip()
	s.PopTransform()

	if op := s.Ops[0]; op.Radii != (Corners{}) {
		t.Errorf("plain clip has radii %+v", op.Radii)
	}
	want := Corners{TL: 2, TR: 4, BR: 6, BL: 8}
	if op := s.Ops[2]; op.Clip != (Rect{12, 24, 6, 8}) || op.Radii != want {
		t.Errorf("rounded clip %+v radii %+v, want rect {12 24 6 8} radii %+v", op.Clip, op.Radii, want)
	}

	var rec recorder
	s.Replay(&rec)
	got := []string{"clip", "pop", "rounded {2 4 6 8}", "pop"}
	if !slices.Equal(rec.calls, got) {
		t.Errorf("replay calls %v, want %v", rec.calls, got)
	}
}

// BorderSides is scaled like BorderWidth, side by side.
func TestBorderSidesScaleWithTransform(t *testing.T) {
	var s Scene
	s.Reset()
	s.PushTransform(Transform{Scale: 2})
	s.AddQuad(Quad{Bounds: Rect{W: 10, H: 10}, BorderSides: Sides{Top: 1, Right: 2, Bottom: 3, Left: 0}})
	s.PopTransform()
	if got, want := s.Quads[0].BorderSides, (Sides{Top: 2, Right: 4, Bottom: 6, Left: 0}); got != want {
		t.Errorf("border sides %+v, want %+v", got, want)
	}
}

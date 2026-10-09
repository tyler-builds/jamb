package scene

import "testing"

// Every primitive, and every clip, is placed by the transform in effect when it
// is added, so a scaled subtree is drawn scaled.
func TestTransformAppliesToEveryPrimitive(t *testing.T) {
	tex := NewTexture(4, 4, FormatAlpha8)
	var s Scene
	s.Reset()
	s.PushTransform(Transform{Scale: 2, DX: 10, DY: 20})
	s.AddQuad(Quad{Bounds: Rect{1, 2, 3, 4}, BorderWidth: 1, Blur: 1, Radii: Uniform(1)})
	s.AddGlyph(tex, Glyph{Bounds: Rect{1, 2, 3, 4}})
	s.AddImage(ImageDraw{Bounds: Rect{1, 2, 3, 4}, Radii: Uniform(1)})
	s.PushClip(Rect{1, 2, 3, 4})
	s.PopClip()
	s.PopTransform()

	want := Rect{12, 24, 6, 8}
	if q := s.Quads[0]; q.Bounds != want || q.BorderWidth != 2 || q.Blur != 2 || q.Radii.TL != 2 {
		t.Errorf("quad %+v, want bounds %+v, border 2, blur 2, radius 2", q, want)
	}
	if g := s.Glyphs[0]; g.Bounds != want {
		t.Errorf("glyph bounds %+v, want %+v", g.Bounds, want)
	}
	if im := s.Images[0]; im.Bounds != want || im.Radii.TL != 2 {
		t.Errorf("image %+v, want bounds %+v and radius 2", im, want)
	}
	var clip Rect
	for _, op := range s.Ops {
		if op.Kind == OpPushClip {
			clip = op.Clip
		}
	}
	if clip != want {
		t.Errorf("clip %+v, want %+v", clip, want)
	}
}

// Nested transforms compose: the inner one acts first, then the outer one.
func TestNestedTransformsCompose(t *testing.T) {
	var s Scene
	s.Reset()
	s.PushTransform(Transform{Scale: 2})
	s.PushTransform(Transform{Scale: 0.5, DX: 1})
	s.AddQuad(Quad{Bounds: Rect{X: 1, W: 1, H: 1}})
	s.PopTransform()
	s.PopTransform()
	// p -> p*0.5+1 (inner), then *2 (outer): 1 -> 1.5 -> 3.
	if got := s.Quads[0].Bounds.X; got != 3 {
		t.Errorf("nested quad at X %v, want 3", got)
	}
}

// A gradient quad keeps its colors, angle and stops; a scale transform moves
// and resizes it but leaves the angle and stop positions alone.
func TestGradientQuadFieldsSurviveTransform(t *testing.T) {
	var s Scene
	s.Reset()
	s.PushTransform(Transform{Scale: 2})
	s.AddQuad(Quad{
		Bounds:     Rect{X: 1, Y: 1, W: 3, H: 4},
		Background: Color{1, 0, 0, 1},
		Gradient:   true,
		Color2:     Color{0, 0, 1, 0.5},
		Angle:      135,
		Stop0:      0.25,
		Stop1:      0.75,
		Oklab:      true,
	})
	s.PopTransform()
	q := s.Quads[0]
	if q.Bounds != (Rect{X: 2, Y: 2, W: 6, H: 8}) {
		t.Errorf("bounds %+v, want {2 2 6 8}", q.Bounds)
	}
	if !q.Gradient || q.Color2 != (Color{0, 0, 1, 0.5}) || q.Angle != 135 || q.Stop0 != 0.25 || q.Stop1 != 0.75 || !q.Oklab {
		t.Errorf("gradient fields lost or changed: %+v", q)
	}
}

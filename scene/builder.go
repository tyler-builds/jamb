package scene

// OpKind identifies a scene command.
type OpKind uint8

const (
	OpQuads OpKind = iota
	OpGlyphs
	OpImages
	OpPushClip
	OpPopClip
)

// Op is one command in paint order. Consecutive primitives of the same kind
// are merged into a single Op so they reach the GPU as one instanced draw.
type Op struct {
	Kind       OpKind
	Start, End int      // range in the matching primitive slice
	Texture    *Texture // glyph batches only
	Clip       Rect     // OpPushClip only
	Radii      Corners  // OpPushClip only: a rounded clip when not zero
}

// Transform is a uniform scale followed by a translation:
// p' = p*Scale + (DX, DY).
type Transform struct {
	DX, DY, Scale float32
}

func (t Transform) apply(r Rect) Rect {
	return Rect{r.X*t.Scale + t.DX, r.Y*t.Scale + t.DY, r.W * t.Scale, r.H * t.Scale}
}

// Scene is an ordered list of primitives. Build it with the Push*/Add*
// methods, then hand it to Replay.
type Scene struct {
	Clear  Color
	Ops    []Op
	Quads  []Quad
	Glyphs []Glyph
	Images []ImageDraw

	transforms []Transform
	clipDepth  int
}

// Reset empties the scene while keeping its allocated capacity.
func (s *Scene) Reset() {
	s.Ops = s.Ops[:0]
	s.Quads = s.Quads[:0]
	s.Glyphs = s.Glyphs[:0]
	s.Images = s.Images[:0]
	s.transforms = s.transforms[:0]
	s.clipDepth = 0
}

func (s *Scene) xf() (Transform, bool) {
	if n := len(s.transforms); n > 0 {
		return s.transforms[n-1], true
	}
	return Transform{Scale: 1}, false
}

// PushTransform composes t with the current transform for everything added
// until the matching PopTransform. Transforms are applied on the CPU while
// building, so renderers only ever see final pixel coordinates.
func (s *Scene) PushTransform(t Transform) {
	if t.Scale == 0 {
		t.Scale = 1
	}
	cur, _ := s.xf()
	s.transforms = append(s.transforms, Transform{
		DX:    t.DX*cur.Scale + cur.DX,
		DY:    t.DY*cur.Scale + cur.DY,
		Scale: t.Scale * cur.Scale,
	})
}

// PopTransform restores the previous transform.
func (s *Scene) PopTransform() { s.transforms = s.transforms[:len(s.transforms)-1] }

// PushClip restricts drawing to r (intersected with any enclosing clip).
func (s *Scene) PushClip(r Rect) {
	if t, ok := s.xf(); ok {
		r = t.apply(r)
	}
	s.clipDepth++
	s.Ops = append(s.Ops, Op{Kind: OpPushClip, Clip: r})
}

// PushRoundedClip restricts drawing to r with its corners rounded by radii,
// anti-aliased. Like PushClip it is intersected with the enclosing clip, but
// the rounding applies to r itself, so an outer clip that cuts r does not
// round the cut. A renderer keeps the rounding of the innermost rounded clip
// only; an enclosing rounded clip is applied as a plain rectangle.
func (s *Scene) PushRoundedClip(r Rect, radii Corners) {
	if t, ok := s.xf(); ok {
		r = t.apply(r)
		radii = Corners{radii.TL * t.Scale, radii.TR * t.Scale, radii.BR * t.Scale, radii.BL * t.Scale}
	}
	s.clipDepth++
	s.Ops = append(s.Ops, Op{Kind: OpPushClip, Clip: r, Radii: radii})
}

// PopClip ends the innermost clip.
func (s *Scene) PopClip() {
	s.clipDepth--
	s.Ops = append(s.Ops, Op{Kind: OpPopClip})
}

func (s *Scene) extend(kind OpKind, start int, tex *Texture) {
	if n := len(s.Ops); n > 0 {
		last := &s.Ops[n-1]
		if last.Kind == kind && last.Texture == tex && last.End == start {
			last.End++
			return
		}
	}
	s.Ops = append(s.Ops, Op{Kind: kind, Start: start, End: start + 1, Texture: tex})
}

// AddQuad appends a quad (or shadow).
func (s *Scene) AddQuad(q Quad) {
	if t, ok := s.xf(); ok {
		q.Bounds = t.apply(q.Bounds)
		q.BorderWidth *= t.Scale
		q.BorderSides = Sides{q.BorderSides.Top * t.Scale, q.BorderSides.Right * t.Scale, q.BorderSides.Bottom * t.Scale, q.BorderSides.Left * t.Scale}
		q.Blur *= t.Scale
		q.Radii = Corners{q.Radii.TL * t.Scale, q.Radii.TR * t.Scale, q.Radii.BR * t.Scale, q.Radii.BL * t.Scale}
	}
	s.Quads = append(s.Quads, q)
	s.extend(OpQuads, len(s.Quads)-1, nil)
}

// AddGlyph appends a mask sprite from tex.
func (s *Scene) AddGlyph(tex *Texture, g Glyph) {
	if t, ok := s.xf(); ok {
		g.Bounds = t.apply(g.Bounds)
	}
	s.Glyphs = append(s.Glyphs, g)
	s.extend(OpGlyphs, len(s.Glyphs)-1, tex)
}

// AddImage appends an image draw.
func (s *Scene) AddImage(im ImageDraw) {
	if t, ok := s.xf(); ok {
		im.Bounds = t.apply(im.Bounds)
		im.Radii = Corners{im.Radii.TL * t.Scale, im.Radii.TR * t.Scale, im.Radii.BR * t.Scale, im.Radii.BL * t.Scale}
	}
	s.Images = append(s.Images, im)
	s.extend(OpImages, len(s.Images)-1, nil)
}

// Drawer is the subset of a renderer that Replay needs. render.Renderer
// satisfies it.
type Drawer interface {
	DrawQuads([]Quad)
	DrawGlyphs(GlyphRun)
	DrawImages([]ImageDraw)
	PushClip(Rect)
	PushRoundedClip(Rect, Corners)
	PopClip()
}

// Replay issues the scene's commands to d in order.
func (s *Scene) Replay(d Drawer) {
	for _, op := range s.Ops {
		switch op.Kind {
		case OpQuads:
			d.DrawQuads(s.Quads[op.Start:op.End])
		case OpGlyphs:
			d.DrawGlyphs(GlyphRun{Texture: op.Texture, Glyphs: s.Glyphs[op.Start:op.End]})
		case OpImages:
			d.DrawImages(s.Images[op.Start:op.End])
		case OpPushClip:
			if op.Radii == (Corners{}) {
				d.PushClip(op.Clip)
			} else {
				d.PushRoundedClip(op.Clip, op.Radii)
			}
		case OpPopClip:
			d.PopClip()
		}
	}
}

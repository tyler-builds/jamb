package ui

import (
	"math"

	"github.com/tyler-builds/jamb/layout"
	"github.com/tyler-builds/jamb/scene"
	"github.com/tyler-builds/jamb/text"
)

func (rt *Runtime) paint(dt float32) {
	sc := &rt.scene
	sc.Reset()
	sc.Clear = rt.theme.Background.scene()
	view := scene.Rect{W: rt.viewW, H: rt.viewH}
	rt.overlays = rt.overlays[:0]
	rt.paintGen++
	// The focused element and its ancestors are painted even when they are off
	// screen: the caret, the input method and revealCaret need them.
	for n := rt.byID[rt.focusID]; n != nil; n = n.parent {
		n.keepGen = rt.paintGen
	}
	rt.paintNode(rt.tree, 0, 0, view, 1, rt.theme.Text, dt)
	rt.paintGhosts(dt)
	rt.paintOverlays(view, dt)
	rt.paintDrag(view, dt)
	rt.revealTabFocus()
}

// inkVisible reports whether a child laid out at (ox+X, oy+Y) in its parent
// can draw into clip. One pixel of slack covers antialiased edges.
func inkVisible(l *layout.Node, ox, oy float32, clip scene.Rect) bool {
	x0, y0 := ox+l.X+l.InkX0, oy+l.Y+l.InkY0
	x1, y1 := ox+l.X+l.InkX1, oy+l.Y+l.InkY1
	return x1 >= clip.X-1 && x0 <= clip.Right()+1 && y1 >= clip.Y-1 && y0 <= clip.Bottom()+1
}

func round(v float32) float32 { return float32(math.Round(float64(v))) }

// px converts a logical rect to physical pixels, snapping edges to the pixel
// grid so borders and edges stay crisp at fractional scales.
func (rt *Runtime) px(r scene.Rect) scene.Rect {
	s := rt.scale
	x0, y0 := round(r.X*s), round(r.Y*s)
	x1, y1 := round((r.X+r.W)*s), round((r.Y+r.H)*s)
	return scene.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// borderSides converts border widths to physical pixels. A side with a width
// is at least one pixel, so it stays visible at any scale.
func (rt *Runtime) borderSides(e Edges) scene.Sides {
	side := func(v float32) float32 {
		if v <= 0 {
			return 0
		}
		return max(1, round(v*rt.scale))
	}
	return scene.Sides{Top: side(e.Top), Right: side(e.Right), Bottom: side(e.Bottom), Left: side(e.Left)}
}

// innerCorners returns the radii of a border's inner edge. Each corner loses
// the average of its two sides' widths: exact for uniform borders, as in CSS,
// and the same rule the shader uses for the inner edge.
func innerCorners(c Corners, b Edges) Corners {
	return Corners{
		TopLeft:     max(c.TopLeft-(b.Top+b.Left)/2, 0),
		TopRight:    max(c.TopRight-(b.Top+b.Right)/2, 0),
		BottomRight: max(c.BottomRight-(b.Bottom+b.Right)/2, 0),
		BottomLeft:  max(c.BottomLeft-(b.Bottom+b.Left)/2, 0),
	}
}

func (rt *Runtime) corners(c Corners) scene.Corners {
	s := rt.scale
	return scene.Corners{TL: c.TopLeft * s, TR: c.TopRight * s, BR: c.BottomRight * s, BL: c.BottomLeft * s}
}

func (rt *Runtime) paintNode(n *node, ox, oy float32, clip scene.Rect, opacity float32, fg Color, dt float32) {
	sc := &rt.scene
	s := &n.style
	bx, by := ox+n.lay.X, oy+n.lay.Y
	n.paintGen = rt.paintGen
	n.painted = n.painted[:0]
	rt.stats.NodesPainted++
	if n.flags&flagExit != 0 {
		// Kept so the element can be drawn as a ghost after it is removed.
		n.baseX, n.baseY, n.inOpacity, n.inFg = bx, by, opacity, fg
	}
	if !s.TextColor.IsZero() {
		fg = s.TextColor
	}
	v := rt.visual(n, fg, dt)
	x, y := bx+v.offX, by+v.offY
	w, h := n.lay.W, n.lay.H
	n.abs = scene.Rect{X: x, Y: y, W: w, H: h}
	// The hit box is the layout box moved by the element's own offset, not by
	// its hover, focus or press styles, transitions or motion. Those move the
	// paint only, so a hover that lifts an element cannot move the pointer off
	// it, end the hover, and bring the element back under the pointer.
	own := &n.style
	if n.div != nil {
		own = &n.div.style
	}
	n.hit = scene.Rect{X: bx + own.OffsetX, Y: by + own.OffsetY, W: w, H: h}
	n.clip = clip
	opacity *= v.opacity
	bg, border, fg := v.bg, v.border, v.fg
	if v.scale != 1 {
		// Scale about the element's center. Transforms are in device pixels and
		// compose with the transform of any enclosing element, so the element's
		// own drawing and its children's are all scaled.
		k := v.scale
		cx, cy := (x+w/2)*rt.scale, (y+h/2)*rt.scale
		sc.PushTransform(scene.Transform{Scale: k, DX: cx * (1 - k), DY: cy * (1 - k)})
		defer sc.PopTransform()
		// The children are drawn scaled, which the culling test (made in
		// unscaled coordinates) does not model, so none of them is culled.
		rt.unculled++
		defer func() { rt.unculled-- }()
	}

	visible := !n.abs.Intersect(clip).Empty()
	switch n.kind {
	case kindDiv, kindEditor:
		radii := rt.corners(s.radii())
		for _, sh := range s.Shadows {
			if rt.sys.HighContrast && (sh.Blur != 0 || sh.X != 0 || sh.Y != 0 || sh.Spread <= 0) {
				// High contrast draws no shadows. A crisp ring, as a focus ring
				// is, stays.
				continue
			}
			if !visible && sh.Blur == 0 {
				continue
			}
			r := scene.Rect{X: x + sh.X - sh.Spread, Y: y + sh.Y - sh.Spread, W: w + 2*sh.Spread, H: h + 2*sh.Spread}
			sr := radii
			if sh.Spread != 0 {
				d := sh.Spread * rt.scale
				sr = scene.Corners{TL: max(sr.TL+d, 0), TR: max(sr.TR+d, 0), BR: max(sr.BR+d, 0), BL: max(sr.BL+d, 0)}
			}
			if sh.Blur == 0 && sh.X == 0 && sh.Y == 0 && sh.Spread > 0 {
				// A crisp spread without offset is a ring around the box, as a
				// focus ring. It is drawn as a border of the expanded box, so it
				// stays hollow over a transparent element; a filled shape would
				// cover the element's own background, or the element if it has none.
				sc.AddQuad(scene.Quad{
					Bounds:      rt.px(r),
					BorderWidth: max(1, round(sh.Spread*rt.scale)),
					BorderColor: sh.Color.Alpha(opacity).scene(),
					Radii:       sr,
				})
				continue
			}
			sc.AddQuad(scene.Quad{
				Bounds:     rt.px(r),
				Background: sh.Color.Alpha(opacity).scene(),
				Radii:      sr,
				Blur:       max(sh.Blur*rt.scale/2, 0.5),
			})
		}
		be := s.borderEdges()
		grad := s.Gradient
		if rt.sys.HighContrast && !grad.isZero() {
			// High contrast draws solid colors: a gradient becomes its first color.
			bg, grad = grad.From, Gradient{}
		}
		if visible && (bg.A > 0 || !grad.isZero() || (!be.isZero() && border.A > 0)) {
			q := scene.Quad{
				Bounds:     rt.px(n.abs),
				Background: bg.Alpha(opacity).scene(),
				Radii:      radii,
			}
			if !grad.isZero() {
				// A gradient replaces the eased Background. Its colors are read
				// from the style, so they snap rather than transition.
				q.Background = grad.From.Alpha(opacity).scene()
				q.Gradient, q.Color2 = true, grad.To.Alpha(opacity).scene()
				q.Angle = grad.Angle
				q.Stop0, q.Stop1 = grad.stops()
				q.Oklab = grad.Oklab
			}
			if !be.isZero() && border.A > 0 {
				q.BorderSides = rt.borderSides(be)
				q.BorderColor = border.Alpha(opacity).scene()
			}
			sc.AddQuad(q)
		}
	case kindText:
		if visible {
			rt.paintText(n, x, y, w, fg, opacity)
		}
	}
	// A focused editor is painted even when scrolled out of view, so that typing
	// can scroll its caret back into view (revealCaret).
	if n.kind == kindEditor && (visible || rt.focusID == n.id) {
		rt.paintEditor(n, x, y, w, fg, opacity)
	}
	switch n.kind {
	case kindIcon:
		if visible {
			c := fg
			if !n.icon.color.IsZero() && !rt.sys.HighContrast {
				// High contrast draws icons in the text color, as Microsoft's guidance asks.
				c = n.icon.color
			}
			rt.paintIcon(n, x, y, c.Alpha(opacity))
		}
	case kindImage:
		if visible {
			rt.paintImage(n, opacity)
		}
	}

	if len(n.order) == 0 {
		return
	}
	clips := s.OverflowX != OverflowVisible || s.OverflowY != OverflowVisible
	childClip := clip
	if clips {
		// The children are clipped to the padding box, inside the border. A
		// rounded container clips them to its rounded shape there.
		be := s.borderEdges()
		inner := scene.Rect{X: x + be.Left, Y: y + be.Top, W: w - be.Left - be.Right, H: h - be.Top - be.Bottom}
		childClip = clip.Intersect(inner)
		if r := s.radii(); r.isZero() {
			sc.PushClip(rt.px(childClip))
		} else {
			sc.PushRoundedClip(rt.px(inner), rt.corners(innerCorners(r, be)))
		}
	}
	var sx, sy float32
	if n.st != nil && (s.OverflowX == OverflowScroll || s.OverflowY == OverflowScroll) {
		sx, sy = rt.scrollStep(n, dt)
	}
	for _, c := range n.order {
		if c.flags&flagOverlay != 0 {
			// Painted later, on top of everything and unclipped. n.abs and
			// clip are the anchor's.
			if anchorVisible(n.abs, clip) {
				rt.overlays = append(rt.overlays, overlayItem{n: c, anchor: n.abs, opacity: opacity, fg: fg, ghost: rt.inGhost})
			}
			continue
		}
		// A child whose drawing cannot reach the clip is skipped with its whole
		// subtree. Never skipped: the focus path, and anything painted inside a
		// ghost or a scaled node. Click-outside listeners don't need painting
		// (they are registered when built), and an element that was not drawn
		// has no ghost to fade out when it is removed.
		if rt.unculled == 0 && !rt.inGhost && c.keepGen != rt.paintGen &&
			!inkVisible(&c.lay, x-sx, y-sy, childClip) {
			continue
		}
		rt.paintNode(c, x-sx, y-sy, childClip, opacity, fg, dt)
		n.painted = append(n.painted, c)
	}
	if n.st != nil && (s.OverflowX == OverflowScroll || s.OverflowY == OverflowScroll) {
		rt.paintScrollbars(n, opacity)
	}
	if n.flags&flagList != 0 && !rt.inGhost {
		rt.checkList(n, sy)
	}
	if clips {
		sc.PopClip()
	}
}

// paintText draws the text of node n. fg is the inherited text color; rich
// text resolves its own colors against it.
func (rt *Runtime) paintText(n *node, x, y, w float32, fg Color, opacity float32) {
	s := &n.style
	contentW := max(w-s.Padding.Left-s.Padding.Right, 0)
	align := 0
	switch n.ts.align {
	case Center:
		align = 1
	case End:
		align = 2
	}
	if n.rich != nil {
		l := rt.text.LayoutSpans(n.textEl.text, n.textStyle(), n.rich.spans, contentW+0.01)
		n.rich.lay = l
		rt.text.DrawRich(&rt.scene, l, x+s.Padding.Left, y+s.Padding.Top, contentW, rt.scale, align, func(span int) text.RunPaint {
			return rt.runPaint(n, span, fg, opacity)
		})
		return
	}
	l := rt.text.Layout(n.textEl.text, n.textStyle(), contentW+0.01)
	rt.text.Draw(&rt.scene, l, x+s.Padding.Left, y+s.Padding.Top, contentW, rt.scale, align, fg.Alpha(opacity).scene())
}

func (rt *Runtime) paintIcon(n *node, x, y float32, c Color) {
	size := int(round(n.icon.size * rt.scale))
	if size <= 0 || n.icon.icon == nil {
		return
	}
	ic := n.icon.icon
	tex, src, ok := rt.text.Mask(text.MaskKey{Name: ic.Name, W: size, H: size}, ic.Rasterize)
	if !ok {
		return
	}
	rt.scene.AddGlyph(tex, scene.Glyph{
		Bounds: scene.Rect{X: round(x * rt.scale), Y: round(y * rt.scale), W: float32(size), H: float32(size)},
		Src:    src,
		Color:  c.scene(),
	})
}

func (rt *Runtime) paintImage(n *node, opacity float32) {
	img := n.img
	if img.src == nil {
		return
	}
	dst := n.abs
	iw, ih := float32(img.src.width), float32(img.src.height)
	srcR := scene.Rect{W: iw, H: ih}
	switch img.fit {
	case FitCover:
		scale := max(dst.W/iw, dst.H/ih)
		cw, ch := dst.W/scale, dst.H/scale
		srcR = scene.Rect{X: (iw - cw) / 2, Y: (ih - ch) / 2, W: cw, H: ch}
	case FitContain:
		scale := min(dst.W/iw, dst.H/ih)
		dw, dh := iw*scale, ih*scale
		dst = scene.Rect{X: dst.X + (dst.W-dw)/2, Y: dst.Y + (dst.H-dh)/2, W: dw, H: dh}
	}
	if img.style.Opacity > 0 {
		opacity *= img.style.Opacity
	}
	rt.scene.AddImage(scene.ImageDraw{
		Bounds:  rt.px(dst),
		Texture: img.src.tex,
		Src:     srcR,
		Radii:   rt.corners(img.style.radii()),
		Opacity: opacity,
	})
}

// scrollStep clamps and animates a scroll container's offset.
func (rt *Runtime) scrollStep(n *node, dt float32) (float32, float32) {
	st := n.st
	maxX := max(n.lay.ContentW-n.lay.W, 0)
	maxY := max(n.lay.ContentH-n.lay.H, 0)
	st.targetX = min(max(st.targetX, 0), maxX)
	st.targetY = min(max(st.targetY, 0), maxY)
	// If the scroll range shrank (window resized, content removed), clamp at
	// once: easing toward the new limit would slide the content around.
	st.scrollX = min(st.scrollX, maxX)
	st.scrollY = min(st.scrollY, maxY)
	k := 1 - float32(math.Exp(float64(-dt*20)))
	step := func(cur, target float32) float32 {
		if rt.sys.ReducedMotion {
			return target // with reduced motion a scroll jumps to its target
		}
		if d := target - cur; d > 0.5 || d < -0.5 {
			rt.animating = true
			return cur + d*k
		}
		return target
	}
	ox, oy := st.scrollX, st.scrollY
	st.scrollX = step(st.scrollX, st.targetX)
	st.scrollY = step(st.scrollY, st.targetY)
	if st.scrollX != st.targetX || st.scrollY != st.targetY {
		rt.scrolled = true
	}
	if rt.sys.ReducedMotion && (st.scrollX != ox || st.scrollY != oy) {
		rt.scrolled = true // the content jumped, so the hover under the pointer is updated
	}
	return st.scrollX, st.scrollY
}

// visual is the paint-only part of a node for one frame: its colors, its
// opacity, offset and scale, after transitions and any entrance or exit.
type visual struct {
	bg, border, fg Color
	opacity        float32 // multiplies the inherited opacity; 1 = none
	offX, offY     float32
	scale          float32 // 1 = none
}

// visual returns the values n is painted with. fg is the text color inherited
// from its parent, after the element's own TextColor.
func (rt *Runtime) visual(n *node, fg Color, dt float32) visual {
	s := &n.style
	v := visual{bg: s.Background, border: s.BorderColor, fg: fg, opacity: 1, offX: s.OffsetX, offY: s.OffsetY, scale: 1}
	if s.Opacity > 0 {
		v.opacity = s.Opacity
	}
	if s.Scale > 0 {
		v.scale = s.Scale
	}
	if n.st != nil {
		if s.Transition > 0 {
			v = rt.animate(n.st, v, float32(s.Transition.Seconds()), dt)
		}
		rt.motion(n, &v)
	}
	return v
}

// animate eases the node's displayed values toward their targets. The first
// paint shows the targets at once.
func (rt *Runtime) animate(st *nodeState, to visual, dur, dt float32) visual {
	if !st.animReady || rt.sys.ReducedMotion {
		// With reduced motion a transition jumps to its target.
		st.shown, st.animReady = to, true
		return to
	}
	k := 1 - float32(math.Exp(float64(-dt/(dur/3))))
	cur := &st.shown
	cur.bg = rt.approach(cur.bg, to.bg, k)
	cur.border = rt.approach(cur.border, to.border, k)
	cur.fg = rt.approach(cur.fg, to.fg, k)
	cur.opacity = rt.approachF(cur.opacity, to.opacity, k)
	cur.offX = rt.approachF(cur.offX, to.offX, k)
	cur.offY = rt.approachF(cur.offY, to.offY, k)
	cur.scale = rt.approachF(cur.scale, to.scale, k)
	return *cur
}

// approachF moves cur a fraction k of the way to target, and asks for another
// frame until it arrives.
func (rt *Runtime) approachF(cur, target, k float32) float32 {
	if math.Abs(float64(target-cur)) < 1e-3 {
		return target
	}
	rt.animating = true
	return cur + (target-cur)*k
}

func (rt *Runtime) approach(cur, target Color, k float32) Color {
	// Fade to/from transparent without passing through black.
	if target.A == 0 {
		target = Color{cur.R, cur.G, cur.B, 0}
	}
	if cur.A == 0 {
		cur = Color{target.R, target.G, target.B, 0}
	}
	if cur.near(target) {
		return target
	}
	rt.animating = true
	return cur.Mix(target, k)
}

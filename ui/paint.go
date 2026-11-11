package ui

import (
	"image"
	"math"
	"slices"

	"github.com/tyler-builds/jamb/internal/layout"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
)

func (rt *Runtime) paint(dt float32) {
	sc := &rt.scene
	sc.Reset()
	sc.Clear = rt.theme.Background.scene()
	view := scene.Rect{W: rt.viewW, H: rt.viewH}
	rt.overlays = rt.overlays[:0]
	rt.paintGen++
	rt.reflowStart = false
	// The focused element and its ancestors are painted even when they are off
	// screen: the caret, the input method and revealCaret need them.
	for n := rt.byID[rt.focusID]; n != nil; n = n.parent {
		n.keepGen = rt.paintGen
	}
	if mr := rt.match; mr != nil {
		mr.flights, mr.done = mr.flights[:0], 0
		if len(mr.in) > 0 || len(mr.out) > 0 {
			rt.pairMatched()
		}
	}
	if rt.tree.flags&flagNoDisplay == 0 {
		rt.paintNode(rt.tree, 0, 0, view, 1, rt.theme.Text, dt)
	}
	rt.paintGhosts(dt)
	rt.paintFlights(dt) // above the rest of the window (see Matched)
	rt.paintOverlays(view, dt)
	rt.paintFlights(dt) // those inside overlays
	rt.paintDrag(view, dt)
	if len(rt.held) > 0 {
		// Held content whose box did not draw it has gone (see reflowSpec).
		rt.held = slices.DeleteFunc(rt.held, func(h heldNode) bool { return h.gen != rt.paintGen })
	}
	if rt.insp != nil {
		inspecting.Load().paint(rt, view)
	}
	rt.revealTabFocus()
	rt.revealRowsNow()
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
	if n.painted == nil && len(n.order) > 0 {
		// The first paint of the node lists its children: room for all of them.
		n.painted = make([]*node, 0, len(n.order))
	} else {
		n.painted = n.painted[:0]
	}
	rt.stats.NodesPainted++
	if n.flags&flagExit != 0 && n.st != nil {
		// Kept so the element can be drawn as a ghost after it is removed.
		n.st.baseX, n.st.baseY = bx, by
	}
	n.inOpacity, n.inFg = opacity, fg // also read by StyleOf
	if !s.TextColor.IsZero() {
		fg = s.TextColor
	}
	v := rt.visual(n, fg, dt)
	var lx, ly float32 // where a layout spring draws the element, from its laid-out place
	w, h := n.lay.W, n.lay.H
	var box *layoutSpringState // the element's layout spring, which draws its box at the size it has reached
	var prevAcc reflowMotion
	var prevBox *layoutSpringState
	var fl *matchState // the flight that draws the element, while it flies (see Matched)
	if n.flags&(flagLayoutSpring|flagHeld|flagMatched) != 0 && n.st != nil && n.st.lay != nil {
		ls := n.st.lay
		if n.flags&flagHeld != 0 && ls.reveal {
			opacity *= rt.revealHeld(ls)
		}
		if n.flags&flagMatched != 0 {
			fl = rt.flight(n, ox, oy, v.offX, v.offY, dt)
		}
		if fl != nil {
			// Its box is the flight's frame. Its children are drawn in the
			// flight's scale, not in the motion of a layout spring.
			lx, ly = ls.x-n.lay.X, ls.y-n.lay.Y
			v.offX += lx
			v.offY += ly
			w, h = ls.w, ls.h
		} else if n.flags&flagLayoutSpring != 0 {
			lx, ly = rt.layoutSpring(n, dt)
			v.offX += lx
			v.offY += ly
			w, h, box = ls.w, ls.h, ls
			// The children are painted in the element's motion (restored at
			// the end, with no defer, which nodes without a spring would pay).
			prevAcc, prevBox = rt.reflowAcc, rt.reflowBox
			if lx != 0 || ly != 0 || ls.moving {
				rt.enterReflow(lx, ly, ls)
			}
			rt.reflowBox = ls
		}
	}
	x, y := bx+v.offX, by+v.offY
	n.abs = scene.Rect{X: x, Y: y, W: w, H: h}
	// The hit box is the layout box moved by the element's own offset and its
	// layout spring, not by its hover, focus or press styles, transitions or
	// motion. Those move the paint only, so a hover that lifts an element
	// cannot move the pointer off it, end the hover, and bring the element back
	// under the pointer. A layout spring moves the element itself, as Offset
	// does: what is clicked is what is under the pointer.
	own := &n.style
	if n.div != nil {
		own = &n.div.style
	}
	n.hit = scene.Rect{X: bx + own.OffsetX + lx, Y: by + own.OffsetY + ly, W: w, H: h}
	n.clip = clip
	opacity *= v.opacity
	bg, border, fg := v.bg, v.border, v.fg
	if n.flags&flagMatched != 0 {
		// A flight's opacity, fill and corner radius go from its match's to
		// its own; how it is drawn is what a match of it flies from.
		opacity, bg = rt.seeMatched(n, fl, opacity, bg)
	}
	n.opacity, n.scale = opacity, v.scale
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
		if fl != nil {
			radii = rt.corners(fl.radius)
		}
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
		if visible && n.div != nil && n.div.fillFromParent && n.parent != nil {
			// An opaque fill under the element: the base color, then the fill its
			// parent is drawn with, so that a pinned cell hides what scrolls under it
			// and still shows the state of its row (see DataGrid).
			for _, c := range [2]Color{n.div.fillBase, n.parent.shownBG()} {
				if c.A > 0 {
					sc.AddQuad(scene.Quad{Bounds: rt.px(n.abs), Background: c.Alpha(opacity).scene(), Radii: radii})
				}
			}
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
		if fl != nil {
			// From here on, the element's box is its content's, at its final
			// layout, which the flight scales to cover its frame (see
			// enterFlight). A flight is a div's.
			x, y, clip = rt.enterFlight(n, fl, n.abs, clip)
			w, h = n.lay.W, n.lay.H
		}
		// What the element draws inside its box, a canvas, text or an image, is
		// drawn at its laid-out size: a layout spring moves it, and draws the
		// box at the size it has reached, but never scales or rewraps it.
		if visible && n.div != nil && n.div.canvas != nil {
			rt.paintCanvas(n, x, y, n.lay.W, n.lay.H, fg, opacity)
		}
		if visible && n.div != nil && n.div.grid != nil {
			rt.paintCellGrid(n, x, y, n.lay.W, n.lay.H, fg, opacity)
		}
	case kindText:
		if visible {
			rt.paintText(n, x, y, n.lay.W, fg, opacity)
		}
	}
	// A focused editor is painted even when scrolled out of view, so that typing
	// can scroll its caret back into view (revealCaret).
	if n.kind == kindEditor && (visible || rt.focusID == n.id) {
		rt.paintEditor(n, x, y, n.lay.W, fg, opacity)
	}
	switch n.kind {
	case kindIcon:
		if visible {
			c := fg
			if !n.icon.color.IsZero() && !rt.sys.HighContrast {
				// High contrast draws icons in the text color, as Microsoft's guidance asks.
				c = n.icon.color
			}
			rt.paintIcon(n, x, y, c.Alpha(opacity), opacity)
		}
	case kindImage:
		if visible {
			rt.paintImage(n, scene.Rect{X: x, Y: y, W: n.lay.W, H: n.lay.H}, opacity)
		}
	}

	if len(n.order) == 0 && (box == nil || len(rt.held) == 0) {
		if box != nil {
			rt.reflowAcc, rt.reflowBox = prevAcc, prevBox
		}
		if fl != nil {
			rt.leaveFlight()
		}
		return
	}
	// A box whose widget clips it while its size moves (see reflowSpec) clips
	// its children to the size it is drawn at, so they are revealed and covered
	// by its edge.
	// A flight clips its content to its frame (see enterFlight), which the
	// content, scaled to cover it, always overflows: the element's own clip
	// would cut nothing more, and would replace the frame's rounded corners.
	clips := (s.OverflowX != OverflowVisible || s.OverflowY != OverflowVisible ||
		box != nil && box.clip && box.moving) && fl == nil
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
		if n.div != nil && n.div.vlist != nil && n.div.vlist.before != nil && !rt.inGhost {
			n.div.vlist.before(n)
		}
		if n.div != nil && n.div.vlist != nil && n.div.vlist.snap {
			rt.reflowSnapIn++
			defer func() { rt.reflowSnapIn-- }()
		}
		sx, sy = rt.scrollStep(n, dt)
		// The content moves by whole device pixels. Each element inside rounds
		// its own position to the pixel grid (text at its baseline, an icon at its
		// top), so a fractional offset moved some of them a pixel more than the
		// others: in a list that scrolls by a pixel or two, the icons moved and
		// the labels beside them did not.
		sx = rt.snapScroll(sx, max(n.lay.ContentW-n.lay.W, 0))
		sy = rt.snapScroll(sy, max(n.lay.ContentH-n.lay.H, 0))
		if n.div != nil && n.div.vlist != nil && n.div.vlist.sticky && len(n.children) > 1 {
			// The header is the child after the rows. It is moved to the top of
			// the viewport before the children are painted.
			n.children[1].style.OffsetY = sy
		}
		if n.div != nil && n.div.vlist != nil && n.div.vlist.pin != nil && !rt.inGhost {
			n.div.vlist.pin(n, sx)
		}
	}
	if n.div != nil && n.div.extent != nil && !rt.inGhost {
		rt.reportExtent(n)
	}
	if box != nil {
		box.ox, box.oy = x-sx, y-sy
	}
	for _, c := range n.order {
		if c.flags&(flagNoDisplay|flagOverlay|flagReplaced|flagMatched) != 0 {
			if c.flags&(flagNoDisplay|flagReplaced) != 0 {
				// Nothing of an element hidden by Display is drawn, or hit, nor
				// of one whose match flies in its place.
				continue
			}
			if c.flags&flagOverlay != 0 {
				// Painted later, on top of everything and unclipped. n.abs and
				// clip are the anchor's.
				if anchorVisible(n.abs, clip) {
					rt.overlays = append(rt.overlays, overlayItem{n: c, anchor: n.abs, opacity: opacity, fg: fg, ghost: rt.inGhost})
				}
				continue
			}
			if rt.deferFlight(c, x-sx, y-sy, childClip, opacity, fg) {
				// It flies: drawn after the rest of the window.
				n.painted = append(n.painted, c)
				continue
			}
		}
		// A child whose drawing cannot reach the clip is skipped with its whole
		// subtree. Never skipped: the focus path, and anything painted inside a
		// ghost or a scaled node. Click-outside listeners don't need painting
		// (they are registered when built), and an element that was not drawn
		// has no ghost to fade out when it is removed.
		if rt.unculled == 0 && !rt.inGhost && c.keepGen != rt.paintGen &&
			!inkVisible(&c.lay, x-sx, y-sy, childClip) &&
			(c.flags&flagLayoutSpring == 0 || !springInk(c, x-sx, y-sy, childClip)) {
			continue
		}
		rt.paintNode(c, x-sx, y-sy, childClip, opacity, fg, dt)
		n.painted = append(n.painted, c)
	}
	if box != nil && len(rt.held) > 0 {
		rt.paintHeld(n, box, x-sx, y-sy, childClip, dt)
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
	if fl != nil {
		rt.leaveFlight()
	}
	if box != nil {
		rt.reflowAcc, rt.reflowBox = prevAcc, prevBox
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
	tx, ty := x+s.Padding.Left, y+s.Padding.Top
	if n.textPaint == nil {
		n.textPaint = rt.newPaintedText()
	}
	if n.rich != nil {
		l := n.textPaint.layout(rt, n, contentW+0.01)
		n.rich.lay = l
		rects := rt.textSelectionRects(n, l, tx, ty, opacity)
		rt.text.DrawRichCached(&rt.scene, l, &n.textPaint.glyphs, tx, ty, contentW, rt.scale, align, func(span int) text.RunPaint {
			return rt.runPaint(n, span, fg, opacity)
		})
		rt.redrawSelected(rects, func(c Color) { rt.text.Draw(&rt.scene, l, tx, ty, contentW, rt.scale, align, c.Alpha(opacity).scene()) })
		if rt.focusID == n.id && rt.keyboardModality {
			rt.paintLinkRing(n, l, tx, ty, opacity)
		}
		return
	}
	l := n.textPaint.layout(rt, n, contentW+0.01)
	rects := rt.textSelectionRects(n, l, tx, ty, opacity)
	rt.text.DrawCached(&rt.scene, l, &n.textPaint.glyphs, tx, ty, contentW, rt.scale, align, fg.Alpha(opacity).scene())
	rt.redrawSelected(rects, func(c Color) { rt.text.Draw(&rt.scene, l, tx, ty, contentW, rt.scale, align, c.Alpha(opacity).scene()) })
}

// paintedText is what the last paint of a text node drew: the layout, found for
// the text, style, spans and width, and the atlas entries of its glyphs.
type paintedText struct {
	text   string
	style  text.Style
	spans  []text.Span
	width  float32
	ref    text.LayoutRef
	glyphs text.DrawCache
}

// layout returns the layout of n's text at width, as rt.text.LayoutSpans does. It
// takes the one of the last paint when the text, the style, the spans and the width
// are the same, which saves the lookup of the text system's cache, whose key holds
// the whole text, on each frame that paints the node again.
func (p *paintedText) layout(rt *Runtime, n *node, width float32) *text.Layout {
	st := n.textStyle()
	var spans []text.Span
	if n.rich != nil {
		spans = n.rich.spans
	}
	if p.ref.Layout() != nil && p.text == n.textEl.text && p.style == st && p.width == width && sameSpans(p.spans, spans) {
		if l, ok := rt.text.Reuse(p.ref); ok {
			return l
		}
	}
	p.text, p.style, p.spans, p.width = n.textEl.text, st, spans, width
	p.ref = rt.text.LayoutRef(n.textEl.text, st, spans, width)
	return p.ref.Layout()
}

// maxRecycledTexts bounds the paintedTexts kept for new text nodes, and
// maxRecycledGlyphs the glyphs one may keep room for, so that the pool holds
// little memory after a large view goes.
const (
	maxRecycledTexts  = 1024
	maxRecycledGlyphs = 256
)

// newPaintedText returns an empty paintedText for a text node, one a retired
// node had when there is one, so that scrolling new rows in does not allocate one
// and its buffers for each text.
func (rt *Runtime) newPaintedText() *paintedText {
	if k := len(rt.textPaints); k > 0 {
		p := rt.textPaints[k-1]
		rt.textPaints[k-1] = nil
		rt.textPaints = rt.textPaints[:k-1]
		return p
	}
	return &paintedText{}
}

// recyclePaintedText empties p, the paintedText of a retired node, and keeps it
// for newPaintedText.
func (rt *Runtime) recyclePaintedText(p *paintedText) {
	if len(rt.textPaints) >= maxRecycledTexts || p.glyphs.Room() > maxRecycledGlyphs {
		return
	}
	p.glyphs.Reset()
	*p = paintedText{glyphs: p.glyphs}
	rt.textPaints = append(rt.textPaints, p)
}

// sameSpans reports whether a and b are the same slice of spans. Spans equal in
// another slice are taken as different, which costs a lookup.
func sameSpans(a, b []text.Span) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// paintLinkRing draws the focus ring around the link of rich text n that has
// keyboard focus, if it has, laid out as l at x, y: a ring around each piece of
// the link's text (all of its segments), one per line it covers.
func (rt *Runtime) paintLinkRing(n *node, l *text.Layout, x, y, opacity float32) {
	if rt.focusedLink() == nil {
		return
	}
	start, end := 0, 0
	first := rt.focusLink - 1
	for i, g := range n.rich.segs[:linkEnd(n.rich.segs, first)] {
		if i < first {
			start += len(g.text)
		}
		end += len(g.text)
	}
	ring := rt.theme.FocusRing.Alpha(opacity).scene()
	w := max(1, round(2*rt.scale))
	rt.selRects = l.SelectionRects(start, end, rt.selRects[:0])
	for _, r := range rt.selRects {
		li := 0
		if l.LineHeight > 0 {
			li = min(max(int(r.Y/l.LineHeight+0.5), 0), len(l.Lines)-1)
		}
		b := rt.px(scene.Rect{X: x + n.lineOffset(l, li) + r.X - 2, Y: y + r.Y, W: r.W + 4, H: r.H})
		rt.scene.AddQuad(scene.Quad{Bounds: b, BorderColor: ring, BorderWidth: w, Radii: scene.Uniform(3 * rt.scale)})
	}
}

func (rt *Runtime) paintIcon(n *node, x, y float32, c Color, opacity float32) {
	size := int(round(n.icon.size * rt.scale))
	if size <= 0 || n.icon.icon == nil {
		return
	}
	ic := n.icon.icon
	if ic.Colored() && !rt.sys.HighContrast {
		// An icon that keeps its colors is an image of them. High contrast draws
		// it in the text color, as every other icon.
		k := text.MaskKey{Name: ic.Name, W: size, H: size}
		tex := rt.colorIcons[k]
		if tex == nil {
			m := image.NewRGBA(image.Rect(0, 0, size, size))
			ic.RasterizeRGBA(m)
			tex = scene.NewTexture(size, size, scene.FormatRGBA8)
			copy(tex.Pix, m.Pix)
			if rt.colorIcons == nil {
				rt.colorIcons = map[text.MaskKey]*scene.Texture{}
			}
			rt.colorIcons[k] = tex
		}
		rt.scene.AddImage(scene.ImageDraw{
			Bounds:  scene.Rect{X: round(x * rt.scale), Y: round(y * rt.scale), W: float32(size), H: float32(size)},
			Texture: tex,
			Opacity: opacity,
		})
		return
	}
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

// paintImage draws image n into dst, its laid-out box where it is drawn.
func (rt *Runtime) paintImage(n *node, dst scene.Rect, opacity float32) {
	img := n.img
	if img.src == nil {
		return
	}
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
	bounds := rt.px(dst)
	tex, srcR := rt.fitImage(n, rt.imageTexture(n), srcR, bounds)
	rt.scene.AddImage(scene.ImageDraw{
		Bounds:  bounds,
		Texture: tex,
		Src:     srcR,
		Radii:   rt.corners(img.style.radii()),
		Opacity: opacity,
	})
}

// extentWatch is set on an element by the widget that drives its scrolling, a tab
// strip: paint reports to fn the element's horizontal scroll target x, its range maxX
// (how far its content runs past its width, whether it scrolls or not) and its width
// w, when one of them changes. fn may set signals: the frame is built and painted
// again before it ends, so a change it makes is never shown a frame late. scrollBy
// moves the element's scroll target.
type extentWatch struct {
	fn   func(x, maxX, w float32)
	rt   *Runtime
	id   uint64 // the element's node, from the last paint
	last [3]float32
	seen bool
}

// reportExtent calls n's extentWatch when n's offset, range or width has changed
// since the last report.
func (rt *Runtime) reportExtent(n *node) {
	w := n.div.extent
	w.rt, w.id = rt, n.id
	var x float32
	if n.st != nil && isScroll(n) {
		x = n.st.targetX
	}
	v := [3]float32{x, max(n.lay.ContentW-n.lay.W, 0), n.lay.W}
	if w.seen && v == w.last {
		return
	}
	w.seen, w.last = true, v
	if w.fn != nil {
		w.fn(v[0], v[1], v[2])
		rt.listStale = true // what fn changed is built before the frame ends
	}
}

// scrollBy moves the scroll target of the watched element by dx, clamped to its
// range, and reports whether it moved.
func (w *extentWatch) scrollBy(dx float32) bool {
	if w.rt == nil {
		return false
	}
	n := w.rt.byID[w.id]
	if n == nil || n.st == nil || n.style.OverflowX != OverflowScroll {
		return false
	}
	maxX := max(n.lay.ContentW-n.lay.W, 0)
	x := min(max(n.st.targetX+dx, 0), maxX)
	if x == n.st.targetX {
		return false
	}
	n.st.targetX = x
	w.rt.needPaint = true
	w.rt.scrolled = true
	w.rt.requestFrame()
	return true
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
		if s.Spring.Duration > 0 {
			v = rt.spring(n.st, v, s, dt)
		} else if s.Transition > 0 {
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
	st.vel = [4]float32{} // a spring that becomes a transition forgets its velocity
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

// spring moves the node's displayed opacity, offset and scale toward their
// targets as the style's spring, from where they are and at the velocity they
// have. Colors cannot overshoot, so they ease as a transition does. The first
// paint shows the targets at once.
func (rt *Runtime) spring(st *nodeState, to visual, s *Style, dt float32) visual {
	if !st.animReady || rt.sys.ReducedMotion {
		// With reduced motion a spring jumps to its target and stops.
		st.shown, st.animReady, st.vel = to, true, [4]float32{}
		return to
	}
	dur := s.Spring.Duration
	if s.Transition > 0 {
		dur = s.Transition
	}
	k := 1 - float32(math.Exp(-float64(dt)/(dur.Seconds()/3)))
	cur := &st.shown
	cur.bg = rt.approach(cur.bg, to.bg, k)
	cur.border = rt.approach(cur.border, to.border, k)
	cur.fg = rt.approach(cur.fg, to.fg, k)
	omega, zeta := s.Spring.params()
	cur.opacity, st.vel[0] = rt.springF(cur.opacity, st.vel[0], to.opacity, 1e-3, dt, omega, zeta)
	cur.offX, st.vel[1] = rt.springF(cur.offX, st.vel[1], to.offX, 0.05, dt, omega, zeta)
	cur.offY, st.vel[2] = rt.springF(cur.offY, st.vel[2], to.offY, 0.05, dt, omega, zeta)
	cur.scale, st.vel[3] = rt.springF(cur.scale, st.vel[3], to.scale, 1e-3, dt, omega, zeta)
	// Opacity stays in 0..1 and scale at 0 or more; a value that reaches a
	// limit stops there.
	if cur.opacity < 0 || cur.opacity > 1 {
		cur.opacity, st.vel[0] = min(max(cur.opacity, 0), 1), 0
	}
	if cur.scale < 0 {
		cur.scale, st.vel[3] = 0, 0
	}
	return *cur
}

// springF moves one value as a spring for dt, and asks for another frame
// until it rests: within eps of its target and nearly still.
func (rt *Runtime) springF(x, v, target, eps, dt float32, omega, zeta float64) (float32, float32) {
	if x == target && v == 0 {
		return x, 0
	}
	x, v = springStep(x, v, target, dt, omega, zeta)
	if abs32(x-target) < eps && abs32(v) < eps*float32(omega) {
		return target, 0
	}
	rt.animating = true
	return x, v
}

// springStep advances a spring at x with velocity v toward target over dt
// seconds. It is the closed-form solution of the damped oscillator, so it is
// exact for any step: the motion does not depend on the frame rate.
func springStep(x, v, target, dt float32, omega, zeta float64) (float32, float32) {
	y0, v0, t := float64(x-target), float64(v), float64(dt)
	var y, vel float64
	if zeta >= 1 {
		// Critically damped: y = e^(-ωt) (y0 + (v0 + ω y0) t).
		e := math.Exp(-omega * t)
		b := v0 + omega*y0
		y = e * (y0 + b*t)
		vel = e * (v0 - omega*b*t)
	} else {
		// Underdamped: an oscillation at the damped frequency in a decaying envelope.
		a := zeta * omega
		wd := omega * math.Sqrt(1-zeta*zeta)
		e := math.Exp(-a * t)
		c, s := math.Cos(wd*t), math.Sin(wd*t)
		y = e * (y0*c + (v0+a*y0)/wd*s)
		vel = e * (v0*c - (a*v0+omega*omega*y0)/wd*s)
	}
	return target + float32(y), float32(vel)
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

// imageTexture returns the texture an image node shows now. An animated GIF
// shows the frame for the time since it was first drawn, and asks for a frame
// when the next one is due. Only drawn images ask, so a GIF that is off screen,
// or in a window that does not draw, costs nothing. Under reduced motion it
// shows its first frame.
func (rt *Runtime) imageTexture(n *node) *scene.Texture {
	src := n.img.src
	if !src.Animated() || rt.sys.ReducedMotion {
		return src.tex
	}
	st := rt.state(n.id)
	if st.imageStart.IsZero() || rt.lastFrame.Before(st.imageStart) {
		st.imageStart = rt.lastFrame // first drawn now, or the clock stepped back
	}
	tex, next := src.frameAt(rt.lastFrame.Sub(st.imageStart))
	if next > 0 {
		due := rt.lastFrame.Add(next)
		if rt.imageDue.IsZero() || due.Before(rt.imageDue) {
			rt.imageDue = due
		}
		rt.wakeAt(due)
	}
	return tex
}

// snapScroll rounds a scroll offset to whole device pixels. An offset at the end
// of its range, end, rounds up, so the end of the content stays in view.
func (rt *Runtime) snapScroll(v, end float32) float32 {
	if rt.scale <= 0 || v == 0 {
		return v
	}
	d := float64(v * rt.scale)
	if v >= end-0.001 {
		return float32(math.Ceil(d-0.001)) / rt.scale
	}
	return float32(math.Round(d)) / rt.scale
}

// shownBG returns the background n was last painted with, after its
// transition or spring, for a child that fills from it (see fillFromParent).
// visual eases the background only for an element with a Transition or a
// Spring, so the others were painted with their style's.
func (n *node) shownBG() Color {
	if st := n.st; st != nil && st.animReady && (n.style.Transition > 0 || n.style.Spring.Duration > 0) {
		return st.shown.bg
	}
	return n.style.Background
}

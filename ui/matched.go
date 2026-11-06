package ui

import (
	"fmt"
	"slices"
	"time"

	"github.com/tyler-builds/jamb/internal/scene"
)

// Shared-element motion (DivElement.Matched) moves an element from one place
// in the window to another when a build removes it from the first and adds it
// to the second: a thumbnail that opens as the large picture, a card that
// becomes the page it opens. The two are different elements, laid out where
// they are. They share an id, and the new one starts where the old one was
// drawn, with its velocity, opacity, corner radius and fill, and springs to
// its own place and size: a flight.
//
// A flight is drawn after the rest of the window, so that what enters and
// exits around it (the grid the thumbnail leaves, the page it opens in) never
// hides it, and inside the rectangular clips of where it is going, a scroll
// viewport's included. The element keeps its final layout: its content is
// scaled uniformly to cover the moving frame, centered, and clipped to it,
// so a picture zooms without distortion and nothing is laid out again while
// it flies. The element's own box (fill, border, shadows) is drawn at the
// frame, with a corner radius between the old one's and its own.

// matchLook is how an element with a Matched id was drawn: where, in the
// window, after the scale of its ancestors, how fast it moved, and its
// opacity, corner radius and fill.
type matchLook struct {
	rect           scene.Rect
	vx, vy, vw, vh float32
	opacity        float32
	radius         Corners
	bg             Color
}

// matchState is the shared-element motion of an element with a Matched id.
// Its place and size while it flies are those of its layoutSpringState: in its
// parent's layout box, as for a layout spring.
type matchState struct {
	id     string
	spring Spring // the element's own layout spring, or the theme's Reflow
	// seen is how the last paint that drew the element drew it, at paint seenGen.
	seen    matchLook
	seenGen uint32
	// pending is set when the element has just replaced its match: its flight
	// starts at its next paint, from from.
	pending bool
	from    matchLook
	flying  bool
	// t is how far the opacity, corner radius and fill have gone from from to
	// the element's own, as a spring from 0 to 1, and vt its velocity.
	t, vt float32
	// radius and bg are the corner radius and fill the flight is drawn with.
	radius Corners
	bg     Color
	// inScale and inOpacity are the scale and opacity of the ancestors of the
	// element at the last paint of the flight, and still reports that they did
	// not change since the paint before: a flight lands once they hold still,
	// so that it does not pop into an ancestor that still moves or fades.
	inScale, inOpacity float32
	still              bool
}

// matchRuntime is the shared-element motion of a window (see matchState).
type matchRuntime struct {
	// in is the elements with a Matched id that the builds since the last
	// paint added; out, by id, those they removed after the last paint drew
	// them. Paint pairs them (see pairMatched).
	in  []*node
	out map[string]matchDeparture
	// last is, for each id, the element the latest paint drew with it, last
	// in tree order: the one that is matched when it leaves.
	last map[string]matchMark
	// flights are the flights the paint under way draws after the rest of the
	// window, the first done of them drawn already; flying is above zero
	// while one is drawn.
	flights []flightItem
	done    int
	flying  int
}

// matchDeparture is an element with a Matched id that a build removed, and
// how the last paint drew it.
type matchDeparture struct {
	n    *node
	look matchLook
}

// matchMark is the element a paint drew last with an id; warned is set once
// two elements were drawn with it in one paint.
type matchMark struct {
	n      *node
	gen    uint32
	warned bool
}

// flightItem is a flight that paint draws after the rest of the window: its
// element, its parent's content origin and the clip of where it is going, and
// what its parent passed down.
type flightItem struct {
	n       *node
	ox, oy  float32
	clip    scene.Rect
	opacity float32
	fg      Color
	ghost   bool
}

// Matched gives the element a shared-element id. When a build removes an
// element with the id and adds another one with it, anywhere in the same
// window, the new element starts where the old one was drawn, with its
// velocity, opacity, corner radius and fill, and springs to its own place and
// size: a thumbnail grows into the picture it opens, and shrinks back when
// the picture closes. The spring is the element's LayoutSpring when it has
// one, and the theme's Motion.Reflow otherwise.
//
// While it flies, the element keeps its final layout: its content is scaled
// uniformly to cover the moving frame and clipped to it, so it never
// distorts or rewraps, and its own box is drawn at the frame. It is drawn
// above the rest of the window, so an ancestor that fades or scales as it
// enters or exits does not hide it, but inside the rectangular clips of where
// it is going. The old element does not play its Exit, and the new one does
// not play its Enter: the flight replaces them. A change that comes
// mid-flight starts from where the element is drawn, with its velocity.
//
// Ids are per window. When two elements drawn at once share an id, the last
// in tree order is matched, and debug builds warn. With reduced motion
// nothing flies: the old element exits and the new one enters, in place.
// Hit testing, Find and assistive technology follow the element where it is
// drawn. A text, an icon or an image flies inside a div with the id.
//
//	tile := func(id string) *ui.DivElement {
//	    return ui.Div().Key(id).Matched(id).Radius(12).ClipContent().Child(ui.Image(photos[id]).Fit(ui.FitCover))
//	}
func (d *DivElement) Matched(id string) *DivElement {
	if d.reflow == nil {
		d.reflow = &reflowSpec{}
	}
	d.reflow.matched = id
	return d
}

// matchedID returns the Matched id of the element n was built from. Only a
// div has one: a text, an icon or an image flies inside a div with the id,
// so that the elements built most often stay in their size classes.
func (n *node) matchedID() string {
	if n.div != nil && n.div.reflow != nil {
		return n.div.reflow.matched
	}
	return ""
}

// setMatched readies node n, just built with a Matched id (flagMatched), for
// paint. A new element is listed for pairing with one that left (see
// pairMatched).
func (rt *Runtime) setMatched(n *node) {
	id := n.matchedID()
	if id == "" || n.flags&flagOverlay != 0 {
		// An overlay is placed by its anchor, not by the layout: it has no
		// place to fly to.
		n.flags &^= flagMatched
		return
	}
	if n.st == nil {
		n.st = rt.state(n.id)
	}
	ls := n.st.lay
	if ls == nil {
		ls = &layoutSpringState{}
		n.st.lay = ls
	}
	m := ls.match
	if m == nil || n.fresh {
		m = &matchState{}
		ls.match = m
		if n.flags&flagLayoutSpring == 0 {
			ls.ready = false
		}
	}
	m.id = id
	m.spring = rt.theme.Motion.ReflowSpring()
	if n.flags&flagLayoutSpring != 0 {
		m.spring = ls.spring
	}
	if rt.match == nil {
		rt.match = &matchRuntime{}
	}
	if n.fresh && !rt.sys.ReducedMotion {
		rt.match.in = append(rt.match.in, n)
	}
}

// departMatched records how n, an element with a Matched id that is leaving
// the tree, was drawn, so that an element with its id added in the same build
// flies from there. Only the element the last paint drew last with the id is
// recorded, and not with reduced motion.
func (rt *Runtime) departMatched(n *node) {
	mr := rt.match
	if mr == nil || rt.sys.ReducedMotion || n.st == nil || n.st.lay == nil || n.st.lay.match == nil {
		return
	}
	m := n.st.lay.match
	if mk, ok := mr.last[m.id]; !ok || mk.n != n {
		return // not the element drawn last with its id
	}
	delete(mr.last, m.id)
	if n.paintGen != rt.paintGen || m.seenGen != rt.paintGen {
		return // not drawn by the last paint: there is nothing to fly from
	}
	if mr.out == nil {
		mr.out = map[string]matchDeparture{}
	}
	mr.out[m.id] = matchDeparture{n: n, look: m.seen}
}

// pairMatched pairs the elements with a Matched id that the builds since the
// last paint added with those they removed. A pair's new element flies from
// where the old one was drawn, and plays no Enter; the old one, and anything
// in it, plays no Exit.
func (rt *Runtime) pairMatched() {
	mr := rt.match
	for i, n := range mr.in {
		if rt.byID[n.id] != n || n.st == nil || n.st.lay == nil || n.st.lay.match == nil {
			continue // gone again, or rebuilt
		}
		m := n.st.lay.match
		later := slices.IndexFunc(mr.in[i+1:], func(o *node) bool {
			return rt.byID[o.id] == o && o.st != nil && o.st.lay != nil && o.st.lay.match != nil && o.st.lay.match.id == m.id
		})
		if later >= 0 {
			// Two elements with one id came at once: the last in tree order
			// is matched.
			rt.warnMatched(m.id)
			continue
		}
		d, ok := mr.out[m.id]
		if !ok {
			continue
		}
		m.pending, m.from = true, d.look
		n.st.enterAt = time.Time{} // the flight is its entrance
		rt.replaceMatched(d.n)
	}
	clear(mr.in)
	mr.in = mr.in[:0]
	clear(mr.out)
}

// replaceMatched marks old, an element that left and whose match flies from
// where it was, as replaced: nothing of it is drawn, even inside an exiting
// ancestor, and neither it nor anything in it plays its Exit.
func (rt *Runtime) replaceMatched(old *node) {
	old.flags |= flagReplaced
	now := rt.now()
	rt.ghosts = slices.DeleteFunc(rt.ghosts, func(g *node) bool {
		for p := g; p != nil; p = p.parent {
			if p == old {
				if a := &g.div.exit.a; a.OnDone != nil {
					// Removed without its exit, the element has finished leaving at once.
					rt.done(now, func() func() { return a.OnDone })
				}
				return true
			}
		}
		return false
	})
}

// warnMatched warns, once per id while an element with it is drawn, that two
// elements drawn or added at once share the Matched id id.
func (rt *Runtime) warnMatched(id string) {
	mr := rt.match
	if mr.last == nil {
		mr.last = map[string]matchMark{}
	}
	mk := mr.last[id]
	if mk.warned {
		return
	}
	mk.warned = true
	mr.last[id] = mk
	if rt.warn != nil {
		rt.warn(fmt.Sprintf("two elements share the Matched id %q; ids are per window, and only the last "+
			"in tree order is matched", id))
	}
}

// seeMatched returns the opacity and fill n, an element with a Matched id
// (flagMatched), is drawn with: its own, op and bg, or while it flies (fl),
// between its match's and its own. It records how this paint draws n: its box
// n.abs, after the scale of its ancestors, its opacity, fill and corner
// radius. It warns when another element was drawn with the same id in this
// paint.
func (rt *Runtime) seeMatched(n *node, fl *matchState, op float32, bg Color) (float32, Color) {
	if fl != nil {
		op, bg = fl.look(op, bg, n.style.radii())
	}
	ls := n.st.lay // setMatched made it
	m := ls.match
	if m == nil || rt.inGhost {
		return op, bg
	}
	opacity := op
	r, k := n.abs, float32(1)
	if xf := rt.scene.Current(); xf.Scale != 1 || xf.DX != 0 || xf.DY != 0 {
		// Under an ancestor's scale: where it is seen, in logical pixels.
		s := rt.scale
		k = xf.Scale
		r = scene.Rect{X: (r.X*s*k + xf.DX) / s, Y: (r.Y*s*k + xf.DY) / s, W: r.W * k, H: r.H * k}
	}
	look := matchLook{rect: r, opacity: opacity, bg: bg, radius: n.style.radii()}
	if fl != nil {
		look.radius = fl.radius
	}
	look.radius = Corners{look.radius.TopLeft * k, look.radius.TopRight * k, look.radius.BottomRight * k, look.radius.BottomLeft * k}
	if fl != nil || n.flags&flagLayoutSpring != 0 && ls.moving {
		look.vx, look.vy, look.vw, look.vh = ls.vx*k, ls.vy*k, ls.vw*k, ls.vh*k
	}
	m.seen, m.seenGen = look, rt.paintGen
	mr := rt.match
	if mk := mr.last[m.id]; mk.gen == rt.paintGen && mk.n != n {
		rt.warnMatched(m.id) // another element was drawn with the id in this paint
	}
	mk := mr.last[m.id] // made by warnMatched, or there already
	if mr.last == nil {
		mr.last = map[string]matchMark{}
	}
	mk.n, mk.gen = n, rt.paintGen
	mr.last[m.id] = mk
	return op, bg
}

// deferFlight queues child c of the element being painted to be drawn after
// the rest of the window, when it flies, and reports whether it did. ox and oy
// are the parent's content origin, clip the clip the child is drawn in, and
// opacity and fg what the parent passes down. A flight inside a flight is
// drawn with it.
func (rt *Runtime) deferFlight(c *node, ox, oy float32, clip scene.Rect, opacity float32, fg Color) bool {
	mr := rt.match
	if mr == nil || mr.flying > 0 || c.st == nil || c.st.lay == nil {
		return false
	}
	m := c.st.lay.match
	if m == nil || !m.pending && !m.flying {
		return false
	}
	s := rt.scene.Current().Scale
	m.still = m.inScale == s && m.inOpacity == opacity
	m.inScale, m.inOpacity = s, opacity
	mr.flights = append(mr.flights, flightItem{n: c, ox: ox, oy: oy, clip: clip, opacity: opacity, fg: fg, ghost: rt.inGhost})
	return true
}

// paintFlights draws the flights queued since it last ran, each inside the
// rectangular clip of where it is going and outside its ancestors' opacity
// and scale (see deferFlight).
func (rt *Runtime) paintFlights(dt float32) {
	mr := rt.match
	if mr == nil {
		return
	}
	sc := &rt.scene
	for ; mr.done < len(mr.flights); mr.done++ {
		f := mr.flights[mr.done] // a copy: painting may append to the slice
		sc.PushClip(rt.px(f.clip))
		prev := rt.inGhost
		rt.inGhost = f.ghost
		mr.flying++
		rt.paintNode(f.n, f.ox, f.oy, f.clip, f.opacity, f.fg, dt)
		mr.flying--
		rt.inGhost = prev
		sc.PopClip()
	}
}

// flight steps the flight of n, an element with a Matched id, for this paint,
// and returns its state while it flies, or nil. ox and oy are its parent's
// content origin, and offX and offY its own offset this paint. Its place and
// size are left in n.st.lay, in its parent's layout box as for a layout
// spring. A flight starts where its match was seen; it lands once its spring
// rests and its ancestors hold still, and at once on a resize, with reduced
// motion, or when it was not drawn by the paint before.
func (rt *Runtime) flight(n *node, ox, oy, offX, offY, dt float32) *matchState {
	ls := n.st.lay
	m := ls.match
	if m == nil {
		return nil
	}
	tx, ty, tw, th := n.lay.X, n.lay.Y, n.lay.W, n.lay.H
	if m.pending {
		m.pending = false
		if rt.sys.ReducedMotion || rt.reflowSnap {
			return nil
		}
		// It starts where its match was drawn, in its parent's layout box, and
		// moves on the next frame, as a layout spring does.
		f := &m.from
		ls.x, ls.y = f.rect.X-ox-offX, f.rect.Y-oy-offY
		ls.w, ls.h = f.rect.W, f.rect.H
		ls.vx, ls.vy, ls.vw, ls.vh = f.vx, f.vy, f.vw, f.vh
		ls.tw, ls.th, ls.fw, ls.fh = tw, th, tw, th
		ls.ready, ls.moving, ls.gen = true, true, rt.paintGen
		m.t, m.vt, m.flying, m.still = 0, 0, true, true
		rt.reflowStart = true
		return m
	}
	if !m.flying {
		return nil
	}
	if rt.sys.ReducedMotion || rt.reflowSnap || ls.gen+1 < rt.paintGen {
		m.land(ls, n)
		return nil
	}
	ls.gen = rt.paintGen
	omega, zeta := m.spring.params()
	ls.x, ls.vx = rt.springF(ls.x, ls.vx, tx, 0.05, dt, omega, zeta)
	ls.y, ls.vy = rt.springF(ls.y, ls.vy, ty, 0.05, dt, omega, zeta)
	ls.w, ls.vw = rt.springF(ls.w, ls.vw, tw, 0.05, dt, omega, zeta)
	ls.h, ls.vh = rt.springF(ls.h, ls.vh, th, 0.05, dt, omega, zeta)
	m.t, m.vt = rt.springF(m.t, m.vt, 1, 0.001, dt, omega, zeta)
	if ls.w < 0 {
		ls.w, ls.vw = 0, 0
	}
	if ls.h < 0 {
		ls.h, ls.vh = 0, 0
	}
	moving := ls.x != tx || ls.y != ty || ls.w != tw || ls.h != th || m.t != 1 ||
		ls.vx != 0 || ls.vy != 0 || ls.vw != 0 || ls.vh != 0 || m.vt != 0
	if !moving && m.still {
		m.land(ls, n)
		return nil
	}
	rt.animating = true
	return m
}

// land ends the flight of n: from this paint it is drawn in place, in the
// tree.
func (m *matchState) land(ls *layoutSpringState, n *node) {
	m.flying, m.pending = false, false
	ls.x, ls.y, ls.w, ls.h = n.lay.X, n.lay.Y, n.lay.W, n.lay.H
	ls.vx, ls.vy, ls.vw, ls.vh = 0, 0, 0, 0
	ls.tw, ls.th, ls.fw, ls.fh = n.lay.W, n.lay.H, n.lay.W, n.lay.H
	ls.moving = false
}

// look returns the opacity and fill the flight m draws its element with,
// between its match's and op and bg, its own, and sets the corner radius it
// draws with, between its match's and own.
func (m *matchState) look(op float32, bg Color, own Corners) (float32, Color) {
	p := min(max(m.t, 0), 1)
	f := &m.from
	m.radius = Corners{
		TopLeft:     lerpF(f.radius.TopLeft, own.TopLeft, p),
		TopRight:    lerpF(f.radius.TopRight, own.TopRight, p),
		BottomRight: lerpF(f.radius.BottomRight, own.BottomRight, p),
		BottomLeft:  lerpF(f.radius.BottomLeft, own.BottomLeft, p),
	}
	m.bg = f.bg.Mix(bg, p)
	return lerpF(f.opacity, op, p), m.bg
}

// coverScale returns the uniform scale that makes a box w × h cover a frame
// fw × fh: the larger of the two ratios. A box with no area is not scaled.
func coverScale(fw, fh, w, h float32) float32 {
	if w <= 0 || h <= 0 {
		return 1
	}
	return max(fw/w, fh/h, 1e-3)
}

// enterFlight starts drawing the content of n, flying in frame (see
// matchState): it clips to the frame, rounded as the flight draws it, and
// scales the content's final layout uniformly to cover the frame, about its
// center. It returns the content's origin and clip, in the content's own
// (unscaled) coordinates. leaveFlight ends it.
func (rt *Runtime) enterFlight(n *node, fl *matchState, frame, clip scene.Rect) (cx, cy float32, inner scene.Rect) {
	sc := &rt.scene
	if fl.radius.isZero() {
		sc.PushClip(rt.px(frame))
	} else {
		sc.PushRoundedClip(rt.px(frame), rt.corners(fl.radius))
	}
	w, h := n.lay.W, n.lay.H
	k := coverScale(frame.W, frame.H, w, h)
	mx, my := frame.X+frame.W/2, frame.Y+frame.H/2
	sc.PushTransform(scene.Transform{Scale: k, DX: mx * rt.scale * (1 - k), DY: my * rt.scale * (1 - k)})
	// The content is drawn scaled, which culling does not model.
	rt.unculled++
	c := clip.Intersect(frame)
	inner = scene.Rect{X: mx + (c.X-mx)/k, Y: my + (c.Y-my)/k, W: c.W / k, H: c.H / k}
	return mx - w/2, my - h/2, inner
}

// leaveFlight ends what enterFlight started.
func (rt *Runtime) leaveFlight() {
	rt.unculled--
	rt.scene.PopTransform()
	rt.scene.PopClip()
}

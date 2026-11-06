package ui

import (
	"fmt"
	"slices"

	"github.com/tyler-builds/jamb/internal/scene"
)

// A layout spring (DivElement.LayoutSpring, ChildrenLayoutSpring) moves an
// element to a new place in the layout, and takes it to a new size, as a
// spring. The layout itself is not animated: it puts the element at its new
// place and size at once, and paint draws the element where the spring has
// brought it, adding the difference to its offset as Offset is added, and
// drawing its own box (background, border, shadows) at the size the spring has
// reached. Nothing is laid out again while it moves.
//
// The place is the element's laid-out position in its parent's layout box. A
// parent that moves carries its children with it, so they do not spring, and a
// scroll moves the parent's content, not the children's places in it, so it
// never springs. The children of an element whose size springs keep their own
// layout and move with its drawn origin; they are clipped to its drawn box
// only when it clips them (Overflow, or a widget's reflowSpec).

// layoutSpringState is where a layout spring draws its element: its place in
// the parent's layout box and its size, which follow the laid-out ones as a
// spring.
type layoutSpringState struct {
	spring Spring
	x, y   float32 // the place drawn, in the parent's layout box
	vx, vy float32 // its velocity, logical pixels per second
	w, h   float32 // the size drawn
	vw, vh float32 // its velocity
	tw, th float32 // the laid-out size, at the last paint
	// fw and fh are the size drawn when the motion began: held content fades
	// out (or in) as the size goes from them to tw, th (see remaining).
	fw, fh float32
	// ox and oy are the origin of the element's content at the last paint, in
	// the window: what held content is placed from (see hold).
	ox, oy  float32
	gen     uint32 // the paint that last drew it
	ready   bool   // x, y, w and h hold a place and a size; false until the element is first drawn
	moving  bool   // the spring moves: away from its target, or not still
	warned  bool   // the warning for an element with no key has been given
	boxOnly bool   // only the size springs, the place snaps (a widget's box; see reflowSpec)
	clip    bool   // the children are clipped to the drawn box while the size moves (see reflowSpec)
	quiet   bool   // set by a widget, which keys what needs keys: no warning
	// reveal is set on held content (flagHeld) that has just appeared: it
	// fades in as the box that holds it grows (see revealHeld).
	reveal bool
}

// reflowSpec is how a widget's element takes part in a change of size that a
// layout spring animates. A widget that opens and closes (Collapsible,
// Sidebar) gives its box a spring (spring; boxOnly when its place should
// snap, as a lone section's does when what is above it changes) and has it
// clip what it holds while its size moves (clip), so the content is revealed
// and covered by the box's edge.
//
// What the widget adds and removes is held content (held). When it leaves the
// tree, or is hidden with Display, while the nearest ancestor with a layout
// spring (the box) shrinks, it is kept, like the ghost of an Exit: the box
// draws it at its last place in the box, under the box's clip, fading out as
// the box reaches its new size, and drops it when the box comes to rest. Held
// content that appears while its box grows fades in the same way. Nothing in
// it can be hit, focused or read by assistive technology once it has left.
type reflowSpec struct {
	spring  Spring
	boxOnly bool
	clip    bool
	held    bool
}

// heldNode is held content that has left the tree and is drawn by its box
// while the box shrinks (see reflowSpec).
type heldNode struct {
	n       *node
	box     uint64  // the ID of the box that draws it
	x, y    float32 // its place in the box's content, before its own offset
	opacity float32 // the opacity and text color it inherited when it left
	fg      Color
	gen     uint32 // the paint that last drew it; one that did not is dropped
}

// springBox gives d a layout spring set by the widget that builds it, sp, used
// when the app sets none, and clips its children to its drawn box while its
// size moves. boxOnly springs only its size.
func (d *DivElement) springBox(sp Spring, boxOnly bool) *DivElement {
	if d.reflow == nil {
		d.reflow = &reflowSpec{}
	}
	d.reflow.spring, d.reflow.boxOnly, d.reflow.clip = sp, boxOnly, true
	return d
}

// heldContent marks d as held content (see reflowSpec).
func (d *DivElement) heldContent() *DivElement {
	if d.reflow == nil {
		d.reflow = &reflowSpec{}
	}
	d.reflow.held = true
	return d
}

// setLayoutSpring gives node n, just built under parent, its layout spring:
// its own, its parent's ChildrenLayoutSpring, or the one its widget set (see
// reflowSpec). A node without one pays nothing more at paint than a flag check.
func (rt *Runtime) setLayoutSpring(n, parent *node) {
	sp := n.style.LayoutSpring
	if sp.Duration <= 0 && parent != nil && parent.div != nil {
		sp = parent.div.kidsSpring
	}
	var spec *reflowSpec
	if n.div != nil {
		spec = n.div.reflow
	}
	widget := false
	if sp.Duration <= 0 && spec != nil {
		sp, widget = spec.spring, true
	}
	if sp.Duration <= 0 || n.flags&flagOverlay != 0 {
		// An overlay is placed by its anchor, not by the layout, so it has no
		// place among its siblings to spring from.
		n.flags &^= flagLayoutSpring
		return
	}
	n.flags |= flagLayoutSpring
	if n.st == nil {
		n.st = rt.state(n.id)
	}
	ls := n.st.lay
	if ls == nil {
		ls = &layoutSpringState{}
		n.st.lay = ls
	}
	ls.spring = sp
	ls.boxOnly = widget && spec.boxOnly
	ls.quiet = widget
	ls.clip = spec != nil && spec.clip
	if n.fresh {
		// A new element appears at its place; its Enter, if any, moves it in.
		ls.ready, ls.moving, ls.warned = false, false, false
	}
}

// setHeld readies node n, held content (see reflowSpec), for paint. shown
// reports that it is shown (not hidden by Display), and appearing that it was
// not shown before this build: it is new, or was hidden.
func (rt *Runtime) setHeld(n *node, shown, appearing bool) {
	if n.st == nil {
		n.st = rt.state(n.id)
	}
	if n.st.lay == nil {
		n.st.lay = &layoutSpringState{}
	}
	if appearing {
		n.st.lay.reveal = true
	}
	if shown && len(rt.held) > 0 {
		rt.dropHeld(n.id) // back before its box came to rest: it is drawn once, live
	}
}

// layoutSpring steps n's layout spring for this paint, and returns how far
// the place it is drawn at is from its laid-out place. The size it is drawn
// at is left in n.st.lay.w and h. dt is the time since the last frame.
func (rt *Runtime) layoutSpring(n *node, dt float32) (dx, dy float32) {
	ls := n.st.lay
	tx, ty, tw, th := n.lay.X, n.lay.Y, n.lay.W, n.lay.H
	if ls.moving && (tw != ls.tw || th != ls.th) {
		// A new size while it moves: what held content fades against starts
		// from where it is now.
		ls.fw, ls.fh = ls.w, ls.h
	}
	ls.tw, ls.th = tw, th
	if !ls.ready && !ls.warned && !ls.quiet && n.key == "" && rt.warn != nil {
		ls.warned = true
		rt.warn(fmt.Sprintf("an element with a layout spring (%s) has no key; give it a Key, "+
			"or it takes the identity, and the place, of whichever sibling is built where it was, "+
			"and a reorder moves nothing", n.describe()))
	}
	// It snaps to its place and size when it was not drawn by the paint before
	// (it is new, was off screen, or hidden), when the window's size, zoom or
	// text scale changed, with reduced motion, and in a UniformList whose row
	// height has just changed.
	if !ls.ready || ls.gen+1 < rt.paintGen || rt.sys.ReducedMotion || rt.reflowSnap || rt.reflowSnapIn > 0 {
		// Its place is where the layout puts it in the window. Inside ancestors
		// that spring, that is not its place in its parent: it is drawn where
		// it will be, and moves in its parent against the parent's motion
		// (with the opposite velocity), so it holds still while they glide to
		// it. A toast added to a stack that grows from the bottom of the window
		// appears in its place, rather than below the window. With no ancestor
		// moving, acc is zero and it is simply at its place. An element whose
		// place snaps (boxOnly) is at its place in its parent.
		acc := rt.reflowAcc
		if ls.boxOnly {
			acc = reflowMotion{}
		}
		ls.x, ls.y, ls.vx, ls.vy = tx-acc.dx, ty-acc.dy, -acc.vx, -acc.vy
		ls.w, ls.h, ls.vw, ls.vh, ls.fw, ls.fh = tw, th, 0, 0, tw, th
		ls.gen, ls.ready, ls.moving = rt.paintGen, true, acc != reflowMotion{}
		rt.reflowStart = rt.reflowStart || ls.moving
		return ls.x - tx, ls.y - ty
	}
	ls.gen = rt.paintGen
	if ls.boxOnly {
		ls.x, ls.y, ls.vx, ls.vy = tx, ty, 0, 0
	}
	if !ls.moving {
		if ls.x == tx && ls.y == ty && ls.w == tw && ls.h == th {
			return 0, 0
		}
		// The layout has just moved or resized the element. Its motion starts
		// now, from where it is drawn: this paint draws it there, and the
		// frames after it step it. Stepping it here by the time since the last
		// frame, which can be long for a UI that was idle, would jump.
		ls.moving = true
		ls.fw, ls.fh = ls.w, ls.h
		rt.reflowStart = true
		return ls.x - tx, ls.y - ty
	}
	omega, zeta := ls.spring.params()
	if !ls.boxOnly {
		ls.x, ls.vx = rt.springF(ls.x, ls.vx, tx, 0.05, dt, omega, zeta)
		ls.y, ls.vy = rt.springF(ls.y, ls.vy, ty, 0.05, dt, omega, zeta)
	}
	ls.w, ls.vw = rt.springF(ls.w, ls.vw, tw, 0.05, dt, omega, zeta)
	ls.h, ls.vh = rt.springF(ls.h, ls.vh, th, 0.05, dt, omega, zeta)
	// A size that bounces past zero stops there.
	if ls.w < 0 {
		ls.w, ls.vw = 0, 0
	}
	if ls.h < 0 {
		ls.h, ls.vh = 0, 0
	}
	ls.moving = ls.x != tx || ls.y != ty || ls.w != tw || ls.h != th ||
		ls.vx != 0 || ls.vy != 0 || ls.vw != 0 || ls.vh != 0
	return ls.x - tx, ls.y - ty
}

// remaining returns how much of its change of size the box ls has still to
// make: 1 when it is drawn at the size it had when its motion began, 0 at its
// laid-out size or at rest. Held content fades with it (see reflowSpec). The
// axis that changes more decides.
func (ls *layoutSpringState) remaining() float32 {
	if !ls.moving {
		return 0
	}
	d, cur := ls.fw-ls.tw, ls.w-ls.tw
	if dh := ls.fh - ls.th; abs32(dh) > abs32(d) {
		d, cur = dh, ls.h-ls.th
	}
	if abs32(d) < 0.5 {
		return 0
	}
	return min(max(cur/d, 0), 1)
}

// revealHeld returns the opacity held content ls, which has just appeared, is
// drawn at: the share of its change of size that the box it is drawn in has
// made. Once the box comes to rest it is drawn as it is.
func (rt *Runtime) revealHeld(ls *layoutSpringState) float32 {
	r := float32(0)
	if b := rt.reflowBox; b != nil {
		r = b.remaining()
	}
	if r <= 0 {
		ls.reveal = false
		return 1
	}
	return 1 - r
}

// hold keeps n, held content that has just left the tree or been hidden (see
// reflowSpec), so that the nearest ancestor with a layout spring draws it while
// it shrinks. It reports whether n is held: it was drawn by the last paint,
// inside a box that is still in the tree, and motion is not reduced.
func (rt *Runtime) hold(n *node) bool {
	if rt.sys.ReducedMotion || n.paintGen != rt.paintGen || n.div == nil {
		return false
	}
	for p := n.parent; p != nil; p = p.parent {
		st := rt.states[p.id]
		if st == nil || st.lay == nil || st.lay.spring.Duration <= 0 {
			continue
		}

		if st.lay.gen != rt.paintGen || rt.byID[p.id] == nil {
			return false // the box was not drawn, or is leaving too
		}
		// Its place in the box's content: where it was laid out, which is its
		// hit box without its own offset.
		own := &n.div.style
		rt.held = append(rt.held, heldNode{n: n, box: p.id,
			x: n.hit.X - own.OffsetX - st.lay.ox, y: n.hit.Y - own.OffsetY - st.lay.oy,
			opacity: n.inOpacity, fg: n.inFg})
		return true
	}
	return false
}

// paintHeld draws the held content of box n (see reflowSpec), whose layout
// spring is ls, from its content's origin ox, oy, under clip. It fades as the
// box reaches its new size; at rest it is not drawn, and the paint drops it.
func (rt *Runtime) paintHeld(n *node, ls *layoutSpringState, ox, oy float32, clip scene.Rect, dt float32) {
	r := ls.remaining()
	if r <= 0 {
		return
	}
	prev := rt.inGhost
	rt.inGhost = true
	for i := range rt.held {
		h := &rt.held[i]
		if h.box != n.id {
			continue
		}
		h.gen = rt.paintGen
		g := h.n
		rt.paintNode(g, ox+h.x-g.lay.X, oy+h.y-g.lay.Y, clip, h.opacity*r, h.fg, dt)
	}
	rt.inGhost = prev
}

// dropHeld forgets the held content with ID id.
func (rt *Runtime) dropHeld(id uint64) {
	rt.held = slices.DeleteFunc(rt.held, func(h heldNode) bool { return h.n.id == id })
}

// reflowMotion is how far, and how fast, the layout springs of the ancestors
// of the element being painted move it from where the layout puts it.
type reflowMotion struct{ dx, dy, vx, vy float32 }

// enterReflow adds the motion of a layout spring that moves its element by
// dx, dy at velocity ls to the motion its children are painted in. The caller
// restores the motion after them.
func (rt *Runtime) enterReflow(dx, dy float32, ls *layoutSpringState) {
	prev := rt.reflowAcc
	rt.reflowAcc = reflowMotion{prev.dx + dx, prev.dy + dy, prev.vx + ls.vx, prev.vy + ls.vy}
}

// springInk reports whether child c, which has a layout spring (flagLayoutSpring)
// and whose laid-out ink is outside clip, is drawn inside clip where its spring has
// it: a child moving out of view, or shrinking, is still drawn until it has gone.
// ox and oy are the parent's content origin, as inkVisible takes them.
func springInk(c *node, ox, oy float32, clip scene.Rect) bool {
	if c.st == nil || c.st.lay == nil || !c.st.lay.ready {
		return false
	}
	ls, l := c.st.lay, &c.lay
	// Drawn larger than it is laid out, its box reaches past its ink.
	x0, y0 := ox+ls.x+l.InkX0, oy+ls.y+l.InkY0
	x1, y1 := ox+ls.x+l.InkX1+max(ls.w-l.W, 0), oy+ls.y+l.InkY1+max(ls.h-l.H, 0)
	return x1 >= clip.X-1 && x0 <= clip.Right()+1 && y1 >= clip.Y-1 && y0 <= clip.Bottom()+1
}

// layoutSpringOf returns the layout spring an element sets in its own style,
// for a list that moves the element's row in its place (see UniformList).
func layoutSpringOf(e Element) Spring {
	if d, ok := e.(*DivElement); ok && d != nil {
		return d.style.LayoutSpring
	}
	return Spring{}
}

// describe names n for a warning: its kind, and its text when it has some.
func (n *node) describe() string {
	switch n.kind {
	case kindText:
		if n.textEl != nil {
			t := []rune(n.textEl.text)
			if len(t) > 24 {
				t = append(t[:24], '…') // whole characters, never a cut UTF-8 sequence
			}
			return fmt.Sprintf("text %q", string(t))
		}
		return "text"
	case kindIcon:
		return "icon"
	case kindImage:
		return "image"
	case kindEditor:
		return "text field"
	}
	if n.div != nil && n.div.part != "" {
		return n.div.part
	}
	return "div"
}

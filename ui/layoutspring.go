package ui

import (
	"fmt"

	"github.com/tyler-builds/jamb/internal/scene"
)

// A layout spring (DivElement.LayoutSpring, ChildrenLayoutSpring) moves an
// element to a new place in the layout as a spring. The layout itself is not
// animated: it puts the element at its new place at once, and paint draws the
// element where the spring has brought it, adding the difference to its offset
// as Offset is added. Nothing is laid out again while it moves.
//
// The place is the element's laid-out position in its parent's layout box. A
// parent that moves carries its children with it, so they do not spring, and a
// scroll moves the parent's content, not the children's places in it, so it
// never springs.

// layoutSpringState is where a layout spring draws its element: its place in
// the parent's layout box, which follows the laid-out place as a spring.
type layoutSpringState struct {
	spring Spring
	x, y   float32 // the place drawn, in the parent's layout box
	vx, vy float32 // its velocity, logical pixels per second
	gen    uint32  // the paint that last drew it
	ready  bool    // x and y hold a place; false until the element is first drawn
	moving bool    // the spring moves: away from its target, or not still
	warned bool    // the warning for an element with no key has been given
}

// setLayoutSpring gives node n, just built under parent, its layout spring:
// its own, or its parent's ChildrenLayoutSpring. A node without one pays
// nothing more at paint than a flag check.
func (rt *Runtime) setLayoutSpring(n, parent *node) {
	sp := n.style.LayoutSpring
	if sp.Duration <= 0 && parent != nil && parent.div != nil {
		sp = parent.div.kidsSpring
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
	if n.fresh {
		// A new element appears at its place; its Enter, if any, moves it in.
		ls.ready, ls.moving, ls.warned = false, false, false
	}
}

// layoutSpring steps n's layout spring for this paint, and returns how far
// the place it is drawn at is from its laid-out place. dt is the time since
// the last frame.
func (rt *Runtime) layoutSpring(n *node, dt float32) (dx, dy float32) {
	ls := n.st.lay
	tx, ty := n.lay.X, n.lay.Y
	if !ls.ready && !ls.warned && n.key == "" && rt.warn != nil {
		ls.warned = true
		rt.warn(fmt.Sprintf("an element with a layout spring (%s) has no key; give it a Key, "+
			"or it takes the identity, and the place, of whichever sibling is built where it was, "+
			"and a reorder moves nothing", n.describe()))
	}
	// It snaps to its place when it was not drawn by the paint before (it is
	// new, was off screen, or hidden), when the window's size, zoom or text
	// scale changed, with reduced motion, and in a UniformList whose row height
	// has just changed.
	if !ls.ready || ls.gen+1 < rt.paintGen || rt.sys.ReducedMotion || rt.reflowSnap || rt.reflowSnapIn > 0 {
		// Its place is where the layout puts it in the window. Inside ancestors
		// that spring, that is not its place in its parent: it is drawn where
		// it will be, and moves in its parent against the parent's motion
		// (with the opposite velocity), so it holds still while they glide to
		// it. A toast added to a stack that grows from the bottom of the window
		// appears in its place, rather than below the window. With no ancestor
		// moving, acc is zero and it is simply at its place.
		acc := rt.reflowAcc
		ls.x, ls.y, ls.vx, ls.vy = tx-acc.dx, ty-acc.dy, -acc.vx, -acc.vy
		ls.gen, ls.ready, ls.moving = rt.paintGen, true, acc != reflowMotion{}
		rt.animating = rt.animating || ls.moving
		return ls.x - tx, ls.y - ty
	}
	ls.gen = rt.paintGen
	if !ls.moving {
		if ls.x == tx && ls.y == ty {
			return 0, 0
		}
		// The layout has just moved the element. Its motion starts now, from
		// where it is drawn: this paint draws it there, and the frames after
		// it step it. Stepping it here by the time since the last frame, which
		// can be long for a UI that was idle, would jump.
		ls.moving = true
		rt.animating = true
		return ls.x - tx, ls.y - ty
	}
	omega, zeta := ls.spring.params()
	ls.x, ls.vx = rt.springF(ls.x, ls.vx, tx, 0.05, dt, omega, zeta)
	ls.y, ls.vy = rt.springF(ls.y, ls.vy, ty, 0.05, dt, omega, zeta)
	ls.moving = ls.x != tx || ls.y != ty || ls.vx != 0 || ls.vy != 0
	return ls.x - tx, ls.y - ty
}

// reflowMotion is how far, and how fast, the layout springs of the ancestors
// of the element being painted move it from where the layout puts it.
type reflowMotion struct{ dx, dy, vx, vy float32 }

// enterReflow adds the motion of a layout spring that moves its element by
// dx, dy at velocity ls to the motion its children are painted in, and
// returns the motion to restore after them.
func (rt *Runtime) enterReflow(dx, dy float32, ls *layoutSpringState) reflowMotion {
	prev := rt.reflowAcc
	rt.reflowAcc = reflowMotion{prev.dx + dx, prev.dy + dy, prev.vx + ls.vx, prev.vy + ls.vy}
	return prev
}

// springInk reports whether child c, which has a layout spring (flagLayoutSpring)
// and whose laid-out ink is outside clip, is drawn inside clip where its spring has
// it: a child moving out of view is still drawn until it has gone. ox and oy
// are the parent's content origin, as inkVisible takes them.
func springInk(c *node, ox, oy float32, clip scene.Rect) bool {
	if c.st == nil || c.st.lay == nil || !c.st.lay.ready {
		return false
	}
	ls := c.st.lay
	return inkVisible(&c.lay, ox+ls.x-c.lay.X, oy+ls.y-c.lay.Y, clip)
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
			t := n.textEl.text
			if len(t) > 24 {
				t = t[:24] + "…"
			}
			return fmt.Sprintf("text %q", t)
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

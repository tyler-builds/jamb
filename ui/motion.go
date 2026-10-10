package ui

import (
	"slices"
	"time"
)

// Motion is a paint-only state an element enters from or exits to: an opacity,
// an offset and a scale. Build one with Fade, Slide and Grow, and combine them
// with the methods of the same names:
//
//	ui.Fade().Slide(0, -6).Grow(0.98)
type Motion struct {
	fade  bool
	x, y  float32
	scale float32 // 0 = unset (1)
}

// Fade is a Motion that starts or ends fully transparent.
func Fade() Motion { return Motion{}.Fade() }

// Slide is a Motion that starts or ends offset by (x, y) logical pixels.
func Slide(x, y float32) Motion { return Motion{}.Slide(x, y) }

// Grow is a Motion that starts or ends scaled by s about the element's
// center: 0.96 starts a little smaller and grows into place.
func Grow(s float32) Motion { return Motion{}.Grow(s) }

// Fade also makes the motion start or end transparent.
func (m Motion) Fade() Motion { m.fade = true; return m }

// Slide also offsets the motion by (x, y).
func (m Motion) Slide(x, y float32) Motion { m.x, m.y = x, y; return m }

// Grow also scales the motion by s.
func (m Motion) Grow(s float32) Motion { m.scale = s; return m }

// pose returns the paint values of m at t: t = 0 is m itself and t = 1 is the
// element's normal look, where every value is neutral.
func (m Motion) pose(t float32) (opacity, x, y, scale float32) {
	opacity = 1
	if m.fade {
		opacity = 0
	}
	s := m.scale
	if s == 0 {
		s = 1
	}
	return lerpF(opacity, 1, t), m.x * (1 - t), m.y * (1 - t), lerpF(s, 1, t)
}

func lerpF(a, b, t float32) float32 { return a + (b-a)*t }

// motionSpec is an entrance or an exit: the Motion and the timing of it.
type motionSpec struct {
	m Motion
	a Animation
}

// progress returns the linear progress (0..1) of a motion that began at from,
// and whether it is still running.
func (s *motionSpec) progress(now, from time.Time) (float32, bool) {
	d := s.a.Duration
	if d <= 0 {
		return 1, false
	}
	p := float32(now.Sub(from).Seconds() / d.Seconds())
	if p >= 1 {
		return 1, false
	}
	return max(p, 0), true
}

// Enter makes the element animate into place when it first appears: it starts
// with the look of m and moves to its normal look over a. Elements that were
// already there do not enter again.
//
//	ui.Div().Enter(ui.Fade().Slide(0, -6), ui.Animation{Duration: 140 * time.Millisecond})
func (d *DivElement) Enter(m Motion, a Animation) *DivElement {
	d.enter = &motionSpec{m, a}
	return d
}

// Exit keeps the element drawn while it animates to the look of m over a, after
// it has been removed from the tree. The exiting element leaves the layout at
// once, so its siblings move straight away; while it exits it is drawn above
// its former siblings, and it cannot be hit, focused or found by Find.
func (d *DivElement) Exit(m Motion, a Animation) *DivElement {
	d.exit = &motionSpec{m, a}
	return d
}

// motion applies the entrance or exit of n to v. An entrance runs from the
// Motion to the normal look; an exit runs from the normal look to the Motion.
func (rt *Runtime) motion(n *node, v *visual) {
	st := n.st
	if n.div == nil || st == nil {
		return
	}
	now := rt.now()
	if sp := n.div.enter; sp != nil && !st.enterAt.IsZero() && !rt.sys.ReducedMotion {
		p, running := sp.progress(now, st.enterAt)
		rt.animating = rt.animating || running
		op, x, y, sc := sp.m.pose(sp.a.easing()(p))
		v.opacity *= op
		v.offX += x
		v.offY += y
		v.scale *= sc
	}
	if sp := n.div.exit; sp != nil && !st.leaveAt.IsZero() && !rt.sys.ReducedMotion {
		p, running := sp.progress(now, st.leaveAt)
		rt.animating = rt.animating || running
		// The exit runs the pose backwards: eased progress 0 is the normal look.
		op, x, y, sc := sp.m.pose(1 - sp.a.easing()(p))
		v.opacity *= op
		v.offX += x
		v.offY += y
		v.scale *= sc
	}
}

// paintGhosts paints the elements that left the tree and are still exiting.
// Those that are not overlays are painted now, after the tree; overlays are
// queued after the other overlays. Exits that have ended are dropped.
func (rt *Runtime) paintGhosts(dt float32) {
	now := rt.now()
	rt.ghosts = slices.DeleteFunc(rt.ghosts, func(n *node) bool {
		return now.Sub(n.st.leaveAt) >= n.div.exit.a.Duration
	})
	for _, n := range rt.ghosts {
		if n.div.overlay != 0 {
			rt.overlays = append(rt.overlays, overlayItem{n: n, opacity: n.inOpacity, fg: n.inFg, ghost: true, exiting: true})
		} else {
			rt.paintGhost(n, dt)
		}
	}
}

// paintGhost paints an exiting element at its last position. Whatever it
// adds to the overlays is marked as ghost too, so nothing in it can be hit.
func (rt *Runtime) paintGhost(n *node, dt float32) {
	prev := rt.inGhost
	rt.inGhost = true
	rt.paintNode(n, n.baseX-n.lay.X, n.baseY-n.lay.Y, n.clip, n.inOpacity, n.inFg, dt)
	rt.inGhost = prev
}

// dropGhost forgets an exiting element that has the same ID as an element that
// is appearing again, so the two are not drawn together.
func (rt *Runtime) dropGhost(id uint64) {
	rt.ghosts = slices.DeleteFunc(rt.ghosts, func(n *node) bool { return n.id == id })
}

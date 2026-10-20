package ui

import (
	"cmp"
	"math"
	"slices"
	"time"
)

// Motion is a paint-only state an element enters from or exits to, or passes
// through in keyframes (see Play): an opacity, an offset and a scale. Build one
// with Fade, Slide and Grow, and combine them with the methods of the same
// names and Opacity:
//
//	ui.Fade().Slide(0, -6).Grow(0.98)
type Motion struct {
	fade  bool    // the opacity is set, to alpha
	alpha float32 // the opacity when fade is set: 0 for Fade
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
func (m Motion) Fade() Motion { m.fade, m.alpha = true, 0; return m }

// Opacity also gives the motion the opacity o, from 0 (transparent, as Fade)
// to 1 (opaque), so a keyframe can stop at a partial opacity.
func (m Motion) Opacity(o float32) Motion { m.fade, m.alpha = true, min(max(o, 0), 1); return m }

// Slide also offsets the motion by (x, y).
func (m Motion) Slide(x, y float32) Motion { m.x, m.y = x, y; return m }

// Grow also scales the motion by s.
func (m Motion) Grow(s float32) Motion { m.scale = s; return m }

// pose returns the paint values of m at t: t = 0 is m itself and t = 1 is the
// element's normal look, where every value is neutral.
func (m Motion) pose(t float32) (opacity, x, y, scale float32) {
	opacity = 1
	if m.fade {
		opacity = m.alpha
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
// and whether it is still running. A motion in its Delay is at 0 and not
// running, and wait is when it starts.
func (s *motionSpec) progress(now, from time.Time) (p float32, running bool, wait time.Time) {
	if from = from.Add(max(s.a.Delay, 0)); now.Before(from) {
		return 0, false, from
	}
	d := s.a.Duration
	if d <= 0 {
		return 1, false, time.Time{}
	}
	p = float32(now.Sub(from).Seconds() / d.Seconds())
	if p >= 1 {
		return 1, false, time.Time{}
	}
	return max(p, 0), true, time.Time{}
}

// waitFor records that a motion the paint drew starts at t, after its delay.
// The window is woken then, and until then the UI has not settled, though it
// asks for no frames.
func (rt *Runtime) waitFor(t time.Time) {
	if rt.motionDue.IsZero() || t.Before(rt.motionDue) {
		rt.motionDue = t
	}
	rt.wakeAt(t)
}

// staggerKey groups the elements that enter together with the same Stagger
// under one parent.
type staggerKey struct {
	parent  uint64
	stagger time.Duration
}

// staggered returns how much later than now an element that enters under
// parent with Stagger s starts: s for each element of its group that entered
// before it in this build, up to 16. Reduced motion has no stagger.
func (rt *Runtime) staggered(parent *node, s time.Duration) time.Duration {
	if s <= 0 || rt.sys.ReducedMotion {
		return 0
	}
	if rt.staggers == nil || rt.staggerGen != rt.gen {
		rt.staggers, rt.staggerGen = map[staggerKey]int{}, rt.gen
	}
	k := staggerKey{stagger: s}
	if parent != nil {
		k.parent = parent.id
	}
	i := rt.staggers[k]
	rt.staggers[k] = i + 1
	return time.Duration(min(i, 16)) * s
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

// Keyframe is one stop of keyframed motion (see Play): at progress At, from 0
// (the start) to 1 (the end), the element has the look of Motion. Easing shapes
// the segment that arrives at this stop; nil uses the Animation's easing.
type Keyframe struct {
	At     float32
	Motion Motion
	Easing func(t float32) float32
}

// playSpec is keyframed motion: the key it plays for, its stops in order of At
// from a stop at 0 to one at 1, and its timing.
type playSpec struct {
	key    any
	frames []Keyframe
	a      Animation
}

// Play runs the element through keyframes over a: at each stop's At it has
// the look of the stop's Motion, and between stops it moves from one look to
// the next. A missing stop at 0 or at 1 is the element's normal look, so a
// single stop at 0.5 rises from and returns to the element as it is. It plays
// when the element first appears and again whenever key changes; key must be
// comparable, as a map key is. With Repeat it loops for as long as it is drawn.
//
//	ui.Div().Play("pulse", []ui.Keyframe{{At: 0.5, Motion: ui.Grow(1.06).Opacity(0.8)}},
//	    ui.Animation{Duration: 600 * time.Millisecond, Repeat: true})
//
// Like Enter, it is paint-only: it costs no layout, and an element that is not
// drawn (scrolled or clipped out of view, or hidden by Display) asks for no
// frames. The progress is time based, so when it is drawn again it shows the
// progress of that moment.
func (d *DivElement) Play(key any, frames []Keyframe, a Animation) *DivElement {
	fs := make([]Keyframe, 0, len(frames)+2)
	for _, f := range frames {
		f.At = min(max(f.At, 0), 1)
		fs = append(fs, f)
	}
	slices.SortStableFunc(fs, func(a, b Keyframe) int { return cmp.Compare(a.At, b.At) })
	if len(fs) == 0 || fs[0].At > 0 {
		fs = slices.Insert(fs, 0, Keyframe{})
	}
	if fs[len(fs)-1].At < 1 {
		fs = append(fs, Keyframe{At: 1})
	}
	d.play = &playSpec{key: key, frames: fs, a: a}
	return d
}

// pose returns the paint values of the keyframes at linear progress p: the
// looks of the stops around p, mixed by the eased progress between them.
func (sp *playSpec) pose(p float32) (opacity, x, y, scale float32) {
	fs := sp.frames
	i := 1
	for i < len(fs)-1 && p > fs[i].At {
		i++
	}
	from, to := &fs[i-1], &fs[i]
	t := float32(1)
	if span := to.At - from.At; span > 0 {
		t = min(max((p-from.At)/span, 0), 1)
	}
	ease := to.Easing
	if ease == nil {
		ease = sp.a.easing()
	}
	e := ease(t)
	o0, x0, y0, s0 := from.Motion.pose(0)
	o1, x1, y1, s1 := to.Motion.pose(0)
	return lerpF(o0, o1, e), lerpF(x0, x1, e), lerpF(y0, y1, e), lerpF(s0, s1, e)
}

// playProgress returns the linear progress of keyframes that began at from,
// and asks for the next frame while they run; in their delay they are at 0 and
// wait. With reduced motion, which has no delays, keyframes that end show
// their end, and repeating ones hold their first frame.
func (rt *Runtime) playProgress(sp *playSpec, now, from time.Time) float32 {
	a := &sp.a
	if rt.sys.ReducedMotion {
		if a.looping() {
			return 0
		}
		return 1
	}
	if from = from.Add(max(a.Delay, 0)); now.Before(from) {
		rt.waitFor(from)
		return 0
	}
	d := a.Duration
	if d <= 0 {
		return 1
	}
	p := float32(now.Sub(from).Seconds() / d.Seconds())
	if a.looping() {
		rt.looping = true
		p = max(p, 0)
		return p - float32(math.Floor(float64(p)))
	}
	if p >= 1 {
		return 1
	}
	rt.animating = true
	return max(p, 0)
}

// motion applies the entrance, exit and keyframes of n to v. An entrance runs
// from the Motion to the normal look; an exit runs from the normal look to the
// Motion.
func (rt *Runtime) motion(n *node, v *visual) {
	st := n.st
	if n.div == nil || st == nil {
		return
	}
	now := rt.now()
	if sp := n.div.enter; sp != nil && !st.enterAt.IsZero() && !rt.sys.ReducedMotion {
		p, running, wait := sp.progress(now, st.enterAt)
		rt.animating = rt.animating || running
		if !wait.IsZero() {
			rt.waitFor(wait)
		}
		op, x, y, sc := sp.m.pose(sp.a.easing()(p))
		v.opacity *= op
		v.offX += x
		v.offY += y
		v.scale *= sc
	}
	if sp := n.div.exit; sp != nil && !st.leaveAt.IsZero() && !rt.sys.ReducedMotion {
		p, running, wait := sp.progress(now, st.leaveAt)
		rt.animating = rt.animating || running
		if !wait.IsZero() {
			rt.waitFor(wait)
		}
		// The exit runs the pose backwards: eased progress 0 is the normal look.
		op, x, y, sc := sp.m.pose(1 - sp.a.easing()(p))
		v.opacity *= op
		v.offX += x
		v.offY += y
		v.scale *= sc
	}
	if sp := n.div.play; sp != nil && !st.playAt.IsZero() {
		op, x, y, sc := sp.pose(rt.playProgress(sp, now, st.playAt))
		v.opacity *= op
		v.offX += x
		v.offY += y
		v.scale *= sc
	}
}

// paintGhosts paints the elements that left the tree and are still exiting.
// Those that are not overlays are painted now, after the tree; overlays are
// queued after the other overlays. Exits that have ended are dropped, and
// their OnDone runs at the next frame.
func (rt *Runtime) paintGhosts(dt float32) {
	now := rt.now()
	rt.ghosts = slices.DeleteFunc(rt.ghosts, func(n *node) bool {
		a := &n.div.exit.a
		if now.Sub(n.st.leaveAt) < max(a.Delay, 0)+a.Duration {
			return false
		}
		if a.OnDone != nil {
			rt.done(now, func() func() { return a.OnDone })
		}
		return true
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

package ui

import (
	"math"
	"time"
)

// Animation configures timed motion. Easing maps progress 0..1; nil means
// ease-out (cubic). Use the functions in github.com/tyler-builds/jamb/ease or your own.
type Animation struct {
	// Duration is how long the animation runs.
	Duration time.Duration
	// Easing maps the linear progress of the animation to its eased progress, such as ease.OutCubic.
	Easing func(t float32) float32
	Repeat bool // Animate and Play: loop forever
	// Delay is how long the animation waits before it starts. Meanwhile it
	// shows its first frame (an entrance its Motion, keyframes their first
	// stop) and asks for no frames: the window is woken when it is due. A
	// delayed entrance is invisible when its Motion fades, but it can be hit.
	Delay time.Duration
	// Stagger, on an Enter, delays the elements that enter together, in one
	// render, under one parent with the same Stagger: each starts Stagger after
	// the one before it, in the order they were built, up to 16 Staggers. An
	// element that enters later, as a row appended to a list, enters at once.
	Stagger time.Duration
	// OnDone runs once when the animation ends by itself: Delay plus Duration
	// after it started, whether or not it is drawn then. It runs on the UI
	// thread at the next frame, never while rendering. It does not run for an
	// animation that is restarted (its key changed) or removed first, nor for
	// a repeating one. An Exit's runs when the element has finished exiting,
	// or at the next frame when it is removed without its exit (off screen, or
	// with reduced motion). In a Timeline a step's OnDone runs when the step
	// ends. With reduced motion it runs at the next frame.
	OnDone func()
}

// Spring is a damped spring that moves an element's opacity, offset and scale
// toward their targets (see DivElement.Spring). Unlike a Transition it has a
// velocity: a target that changes mid-flight bends the motion rather than
// restarting it, and with a bounce it overshoots and swings back. The zero
// Spring is unset.
type Spring struct {
	// Duration is the perceived length of the response: the spring's natural
	// period, which sets its stiffness. A critically damped spring is close to
	// its target after one Duration.
	Duration time.Duration
	// Bounce is how much the spring overshoots: 0 is critically damped and
	// never passes its target, and larger values swing more. It is clamped to
	// [0, 0.9]; the damping ratio is 1-Bounce.
	Bounce float32
}

// params returns the spring's angular frequency (per second) and damping
// ratio: a unit mass with stiffness omega² and damping 2·zeta·omega.
func (s Spring) params() (omega, zeta float64) {
	return 2 * math.Pi / s.Duration.Seconds(), 1 - float64(s.bounce())
}

// bounce returns Bounce clamped to the range the spring accepts.
func (s Spring) bounce() float32 { return min(max(s.Bounce, 0), 0.9) }

// Animate re-renders fn every frame with eased progress t (0..1) until the
// animation ends (or forever with Repeat), so any property can be animated.
// It restarts when key changes. Layout properties (Width, Padding...) cost a
// re-layout each frame; Opacity/Offset/Scale are cheaper but fine here too.
//
//	ui.Animate("bar", ui.Animation{Duration: 600 * time.Millisecond, Easing: ease.OutCubic},
//	    func(t float32) ui.Element {
//	        return ui.Div().Width(ui.Percent(100 * t)).Height(ui.Px(6)).Background(accent)
//	    })
func Animate(key string, a Animation, fn func(t float32) Element) *AnimateElement {
	return &AnimateElement{key: key, anim: a, fn: fn}
}

// AnimateElement is the element made by Animate. It is an expander: it
// realizes the element fn returns for the current progress.
type AnimateElement struct {
	key  string
	anim Animation
	fn   func(t float32) Element
	tl   *Timeline // set by AnimateTimeline, which renders tfn
	tfn  func(p []float32) Element
}

func (*AnimateElement) isElement() {}

// animState is an Animate instance's progress, kept across its renders.
type animState struct {
	key   string
	start time.Time
	el    *AnimateElement // the latest render's element, whose OnDone runs
	wait  *timer          // wakes the instance when its delay ends
	done  []*timer        // run the OnDone of the animation or of its steps
}

// stop cancels the timers of an animation that restarts or is removed. It is
// safe on nil.
func (as *animState) stop() {
	if as == nil {
		return
	}
	as.wait.cancel()
	for _, tm := range as.done {
		tm.cancel()
	}
	as.wait, as.done = nil, nil
}

// delay returns how long a waits before anything moves: its Delay, or for a
// timeline the start of its first step.
func (a *AnimateElement) delay() time.Duration {
	if a.tl == nil {
		return max(a.anim.Delay, 0)
	}
	if len(a.tl.steps) == 0 {
		return 0
	}
	first := a.tl.steps[0].start
	for _, s := range a.tl.steps[1:] {
		first = min(first, s.start)
	}
	return first
}

// schedule registers the OnDone of an animation that starts now: the
// animation's, or each of its timeline's steps', at their natural end.
func (a *AnimateElement) schedule(rt *Runtime, inst *componentInstance, as *animState) {
	if a.anim.Repeat {
		return
	}
	at := func(end time.Duration, pick func(*AnimateElement) func()) {
		as.done = append(as.done, rt.done(as.start.Add(end), func() func() {
			if inst.disposed {
				return nil
			}
			return pick(as.el)
		}))
	}
	if a.tl == nil {
		if a.anim.OnDone != nil {
			at(max(a.anim.Delay, 0)+max(a.anim.Duration, 0), func(e *AnimateElement) func() { return e.anim.OnDone })
		}
		return
	}
	for i, s := range a.tl.steps {
		if s.a.OnDone != nil {
			at(s.start+max(s.a.Duration, 0), func(e *AnimateElement) func() {
				if e.tl == nil || i >= len(e.tl.steps) {
					return nil
				}
				return e.tl.steps[i].a.OnDone
			})
		}
	}
}

// expand renders fn for the progress now. An animation that is still running
// registers its instance, so the next Frame renders it again; one that has
// ended renders its last value and then nothing more. Whether the instance
// keeps asking for frames is decided after the paint (see settleAnims), since
// an instance the paint does not draw has no need of them.
func (a *AnimateElement) expand(ctx *Context) Element {
	if a.fn == nil && a.tfn == nil {
		return nil
	}
	rt := ctx.rt
	st := ctx.state()
	now := rt.now()
	if st.anim == nil || st.anim.key != a.key {
		if st.anim == nil {
			// The timers of the animation go with the instance.
			ctx.OnDispose(func() { st.anim.stop() })
		}
		st.anim.stop()
		st.anim = &animState{key: a.key, start: now}
		a.schedule(rt, ctx.inst, st.anim)
	}
	as := st.anim
	as.el = a
	if due := as.start.Add(a.delay()); now.Before(due) && !rt.sys.ReducedMotion {
		// In its delay the animation shows its first frame and asks for no
		// frames; a timer renders it again when it is due.
		if as.wait == nil {
			inst := ctx.inst
			as.wait = rt.at(due, func() {
				if !inst.disposed {
					inst.invalidate()
				}
			})
			as.wait.motion = true
		}
		return a.render(0)
	}
	// p counts durations from the end of the delay: 1 is the end. A zero
	// duration ends at once.
	p := float32(1)
	if d := a.anim.Duration; d > 0 {
		p = float32(now.Sub(as.start.Add(max(a.anim.Delay, 0))).Seconds() / d.Seconds())
	}
	running := p < 1
	if a.anim.looping() {
		running = true
		p -= float32(math.Floor(float64(p)))
	}
	p = min(max(p, 0), 1)
	if rt.sys.ReducedMotion {
		// With reduced motion a finite animation jumps to its end, and a
		// repeating one stops at its first frame, so nothing asks for a frame.
		running = false
		p = 1
		if a.anim.looping() {
			p = 0
		}
	}
	if running {
		rt.animInsts = append(rt.animInsts, ctx.inst)
	}
	return a.render(p)
}

// render returns the element of a at linear progress p.
func (a *AnimateElement) render(p float32) Element {
	if a.tl != nil {
		if a.tfn == nil {
			return nil
		}
		// The timeline's own steps ease their progress; p is the linear
		// progress through the whole of it.
		elapsed := a.anim.Duration
		if p < 1 {
			elapsed = time.Duration(float64(p) * float64(a.anim.Duration))
		}
		ps := make([]float32, len(a.tl.steps))
		a.tl.progress(elapsed, ps)
		return a.tfn(ps)
	}
	return a.fn(a.anim.easing()(p))
}

// looping reports whether the animation repeats forever.
func (a Animation) looping() bool { return a.Repeat && a.Duration > 0 }

// settleAnims decides which running Animate instances keep the frames coming,
// once the frame's paint is done. An instance whose element that paint drew
// keeps going. One it did not draw (scrolled or clipped out of view, or hidden
// by Display) asks for no frame and waits in pausedAnims. Its progress is
// time based, so when a later paint draws it again it is rendered at the
// progress of that moment and runs on from there, with no jump. Without a paint
// this frame nothing can be told apart, so every instance keeps going.
func (rt *Runtime) settleAnims(painted bool) {
	for pass := 0; ; pass++ {
		kept := rt.animInsts[:0]
		for _, inst := range rt.animInsts {
			if inst.disposed {
				continue
			}
			if painted && !rt.drawn(inst) {
				// An instance the paint did not draw gets one more render before it
				// pauses. A reused instance whose parent was rebuilt keeps its old
				// root, which is no longer in the tree; rendering it again attaches
				// the new one. Without this grace the animation would never be drawn.
				if _, paused := rt.pausedAnims[inst]; paused {
					continue
				}
				if _, graced := rt.graceAnims[inst]; graced {
					delete(rt.graceAnims, inst)
					rt.pauseAnim(inst)
					continue
				}
				rt.addGrace(inst)
			} else {
				if len(rt.pausedAnims) > 0 {
					delete(rt.pausedAnims, inst)
				}
				if len(rt.graceAnims) > 0 {
					delete(rt.graceAnims, inst)
				}
			}
			kept = append(kept, inst)
			if a, ok := inst.el.(*AnimateElement); ok && a.anim.looping() {
				rt.looping = true
			} else {
				rt.animating = true
			}
		}
		clear(rt.animInsts[len(kept):])
		rt.animInsts = kept
		// A paused instance that is drawn now is rendered again, and the frame
		// is painted again so the new progress shows. Its render registers it
		// for the next pass. The passes are bounded: each one resumes only the
		// instances that a paint has just drawn.
		if !painted || pass == 2 || !rt.resumeAnims() {
			return
		}
		if rt.needLayout {
			rt.layout()
		}
		rt.paintFrame(0)
	}
}

// drawn reports whether the last paint drew inst's element.
func (rt *Runtime) drawn(inst *componentInstance) bool {
	return inst.root != nil && inst.root.paintGen == rt.paintGen
}

// addGrace records a running instance that the paint did not draw, for its one
// more render (see settleAnims).
func (rt *Runtime) addGrace(inst *componentInstance) {
	if rt.graceAnims == nil {
		rt.graceAnims = map[*componentInstance]struct{}{}
	}
	rt.graceAnims[inst] = struct{}{}
}

// pauseAnim records a running instance that the paint did not draw.
func (rt *Runtime) pauseAnim(inst *componentInstance) {
	if rt.pausedAnims == nil {
		rt.pausedAnims = map[*componentInstance]struct{}{}
	}
	rt.pausedAnims[inst] = struct{}{}
}

// resumeAnims renders again the paused instances that the last paint drew, so
// that they show their progress at this moment. It reports whether any did.
func (rt *Runtime) resumeAnims() bool {
	for inst := range rt.graceAnims {
		if inst.disposed {
			delete(rt.graceAnims, inst)
		}
	}
	var resume []*componentInstance
	for inst := range rt.pausedAnims {
		if inst.disposed {
			delete(rt.pausedAnims, inst)
			continue
		}
		if rt.drawn(inst) {
			delete(rt.pausedAnims, inst)
			resume = append(resume, inst)
		}
	}
	if len(resume) > 0 {
		// A build pass of its own, as rebuildDirty runs: the instances render
		// under a new generation, so each claims its identity afresh.
		rt.gen++
		for _, inst := range resume {
			rt.rerender(inst)
		}
	}
	return len(resume) > 0
}

// easing returns the easing function of a, the default when none is set.
func (a Animation) easing() func(float32) float32 {
	if a.Easing != nil {
		return a.Easing
	}
	return func(t float32) float32 {
		u := 1 - t
		return 1 - u*u*u
	}
}

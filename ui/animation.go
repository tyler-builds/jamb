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
	Repeat bool // Animate only: loop forever
}

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
}

func (*AnimateElement) isElement() {}

// animState is an Animate instance's progress, kept across its renders.
type animState struct {
	key   string
	start time.Time
}

// expand renders fn for the progress now. An animation that is still running
// registers its instance, so the next Frame renders it again; one that has
// ended renders its last value and then nothing more. Whether the instance
// keeps asking for frames is decided after the paint (see settleAnims), since
// an instance the paint does not draw has no need of them.
func (a *AnimateElement) expand(ctx *Context) Element {
	if a.fn == nil {
		return nil
	}
	rt := ctx.rt
	st := ctx.state()
	now := rt.now()
	if st.anim == nil || st.anim.key != a.key {
		st.anim = &animState{key: a.key, start: now}
	}
	// p counts durations: 1 is the end. A zero duration ends at once.
	p := float32(1)
	if d := a.anim.Duration; d > 0 {
		p = float32(now.Sub(st.anim.start).Seconds() / d.Seconds())
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

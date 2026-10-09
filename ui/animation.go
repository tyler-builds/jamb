package ui

import (
	"math"
	"time"
)

// Animation configures timed motion. Easing maps progress 0..1; nil means
// ease-out (cubic). Use the functions in github.com/tyler-builds/jamb/ease or your own.
type Animation struct {
	Duration time.Duration
	Easing   func(t float32) float32
	Repeat   bool // Animate only: loop forever
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
// ended renders its last value and then nothing more.
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
	looping := a.anim.Repeat && a.anim.Duration > 0
	if looping {
		running = true
		p -= float32(math.Floor(float64(p)))
	}
	p = min(max(p, 0), 1)
	if running {
		if looping {
			rt.looping = true
		} else {
			rt.animating = true
		}
		rt.animInsts = append(rt.animInsts, ctx.inst)
	}
	return a.fn(a.anim.easing()(p))
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

package ui

import (
	"slices"
	"time"
)

// Timeline arranges animations in time, for AnimateTimeline: steps that run
// one after another (Sequence), timelines that run together (Parallel), one
// timeline after another (Then), and a timeline that loops (Repeat). Its
// steps are the Animations it was built from, in the order they were given;
// each has a progress of its own.
//
//	tl := ui.Sequence(fadeIn, slide).Then(ui.Parallel(ui.Sequence(pop), ui.Sequence(glow)))
type Timeline struct {
	steps  []timelineStep
	length time.Duration
	repeat bool
}

// timelineStep is one Animation of a timeline and when it starts, from the
// timeline's start.
type timelineStep struct {
	start time.Duration
	a     Animation
}

// Sequence is a timeline of the steps one after another: each starts when the
// one before it ends, after its own Delay. A step's Easing shapes its own
// progress, and its OnDone runs when it ends; its Repeat and Stagger are not
// used.
func Sequence(steps ...Animation) Timeline {
	var t Timeline
	for _, a := range steps {
		start := t.length + max(a.Delay, 0)
		t.steps = append(t.steps, timelineStep{start: start, a: a})
		t.length = start + max(a.Duration, 0)
	}
	return t
}

// Parallel is a timeline of the parts all starting together. It lasts as long
// as the longest of them.
func Parallel(parts ...Timeline) Timeline {
	var t Timeline
	for _, p := range parts {
		t.steps = append(t.steps, p.steps...)
		t.length = max(t.length, p.length)
	}
	return t
}

// Then is the timeline t followed by next: next starts when t ends.
func (t Timeline) Then(next Timeline) Timeline {
	out := Timeline{steps: slices.Clone(t.steps), length: t.length + next.length}
	for _, s := range next.steps {
		s.start += t.length
		out.steps = append(out.steps, s)
	}
	return out
}

// Repeat is the timeline looping forever, when AnimateTimeline plays it. A
// timeline made from it with Then or Parallel plays it once.
func (t Timeline) Repeat() Timeline {
	t.repeat = true
	return t
}

// Duration returns how long the timeline lasts: until its last step ends.
func (t Timeline) Duration() time.Duration { return t.length }

// progress sets p[i] to the eased progress of step i at elapsed time from the
// timeline's start: 0 before the step starts and 1 once it has ended.
func (t *Timeline) progress(elapsed time.Duration, p []float32) {
	for i, s := range t.steps {
		lp := float32(0)
		if d := s.a.Duration; d > 0 {
			lp = min(max(float32((elapsed-s.start).Seconds()/d.Seconds()), 0), 1)
		} else if elapsed >= s.start {
			lp = 1
		}
		p[i] = s.a.easing()(lp)
	}
}

// AnimateTimeline re-renders fn every frame with the progress of each step of
// t, as Animate does with one: p[i] is the eased progress (0..1) of the i-th
// Animation that t was built from, in the order they were given. It restarts
// when key changes, and stops asking for frames when t ends, after rendering
// fn once more with every step at 1; a repeating timeline loops.
//
//	tl := ui.Sequence(
//	    ui.Animation{Duration: 200 * time.Millisecond},
//	    ui.Animation{Duration: 300 * time.Millisecond, Easing: ease.Spring(0.3)},
//	)
//	ui.AnimateTimeline("intro", tl, func(p []float32) ui.Element {
//	    return ui.Div().Opacity(p[0]).Width(ui.Px(100 + 100*p[1]))
//	})
func AnimateTimeline(key string, t Timeline, fn func(p []float32) Element) *AnimateElement {
	return &AnimateElement{key: key, anim: Animation{Duration: t.length, Repeat: t.repeat}, tl: &t, tfn: fn}
}

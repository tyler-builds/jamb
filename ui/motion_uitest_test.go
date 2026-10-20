package ui_test

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ease"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// springBox is a box keyed "box" offset by x with the spring sp.
func springBox(x *ui.Signal[float32], sp ui.Spring) func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		return ui.Div().Row().Child(ui.Div().Key("box").Size(ui.Px(20), ui.Px(20)).
			Offset(x.Get(), 0).Spring(sp))
	}
}

// A spring retargeted mid-flight keeps its velocity: the box goes on moving
// the way it was going for a moment, without a jump, then comes back.
func TestSpringRetargetKeepsVelocity(t *testing.T) {
	x := ui.NewSignal[float32](0)
	u := uitest.New(t, springBox(x, ui.Spring{Duration: 300 * time.Millisecond}))
	x.Set(100)
	u.Advance(48 * time.Millisecond)
	x1 := u.Get("box").Bounds().X
	u.Advance(16 * time.Millisecond)
	x2 := u.Get("box").Bounds().X
	if !(x2 > x1 && x1 > 0 && x2 < 100) {
		t.Fatalf("moving toward 100: X %v then %v, want rising between 0 and 100", x1, x2)
	}
	x.Set(0)
	u.Advance(16 * time.Millisecond)
	x3 := u.Get("box").Bounds().X
	if x3 <= x2 {
		t.Errorf("one frame after the retarget X = %v, want past %v (the velocity is kept)", x3, x2)
	}
	if x3-x2 > 2*(x2-x1) {
		t.Errorf("one frame after the retarget X jumped from %v to %v", x2, x3)
	}
	u.Settle()
	if got := u.Get("box").Bounds().X; got != 0 {
		t.Errorf("settled at X %v, want 0", got)
	}
}

// A spring comes to rest within a bounded time and then asks for no frames.
// With a bounce it passes its target on the way.
func TestSpringSettles(t *testing.T) {
	x := ui.NewSignal[float32](0)
	u := uitest.New(t, springBox(x, ui.Spring{Duration: 300 * time.Millisecond, Bounce: 0.5}))
	x.Set(100)
	start := u.Now()
	var peak float32
	for !u.Runtime().Settled() {
		u.Advance(16 * time.Millisecond)
		peak = max(peak, u.Get("box").Bounds().X)
		if u.Now().Sub(start) > 3*time.Second {
			t.Fatal("the spring did not settle within 3 seconds")
		}
	}
	if peak <= 101 {
		t.Errorf("a spring with a bounce peaked at %v, want past 100", peak)
	}
	if got := u.Get("box").Bounds().X; got != 100 {
		t.Errorf("settled at X %v, want 100", got)
	}
	u.Frame()
	if u.Runtime().NeedsFrame() || u.Runtime().Stats().Painted {
		t.Error("a settled spring still asks for frames or paints")
	}
}

// A spring's opacity and scale stay in range: opacity in 0..1 and scale at
// 0 or more, however far it swings.
func TestSpringClampsOpacityAndScale(t *testing.T) {
	on := ui.NewSignal(false)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		b := ui.Div().Key("box").Size(ui.Px(20), ui.Px(20)).Spring(ui.Spring{Duration: 300 * time.Millisecond, Bounce: 0.9})
		if on.Get() {
			b.Opacity(1).Scale(1)
		} else {
			b.Opacity(0.05).Scale(0.05)
		}
		return ui.Div().Row().Child(b)
	})
	on.Set(true)
	for i := 0; i < 60; i++ {
		u.Advance(16 * time.Millisecond)
		s := u.Get("box").Style()
		if s.Opacity < 0 || s.Opacity > 1 || s.Scale < 0 {
			t.Fatalf("frame %d: opacity %v, scale %v", i, s.Opacity, s.Scale)
		}
	}
	on.Set(false)
	for i := 0; i < 60; i++ {
		u.Advance(16 * time.Millisecond)
		s := u.Get("box").Style()
		if s.Opacity < 0 || s.Opacity > 1 || s.Scale < 0 {
			t.Fatalf("frame %d back: opacity %v, scale %v", i, s.Opacity, s.Scale)
		}
	}
}

// With reduced motion a spring jumps to its target and asks for no frames.
func TestSpringReducedMotion(t *testing.T) {
	x := ui.NewSignal[float32](0)
	u := uitest.New(t, springBox(x, ui.Spring{Duration: 300 * time.Millisecond, Bounce: 0.5}),
		uitest.Settings(ui.SystemSettings{ReducedMotion: true}))
	x.Set(100)
	u.Frame()
	if got := u.Get("box").Bounds().X; got != 100 {
		t.Errorf("under reduced motion X = %v, want 100 at once", got)
	}
	if u.Runtime().NeedsFrame() {
		t.Error("a spring under reduced motion asks for frames")
	}
}

// near reports whether a and b differ by less than 1e-3.
func near(a, b float32) bool { return a-b < 1e-3 && b-a < 1e-3 }

// pulseBox is a box keyed "box" that plays keyframes under key: from
// transparent, through half opacity and scale 1.2 at the middle, to its normal
// look, over 320 ms with linear segments.
func pulseBox(key *ui.Signal[string], repeat bool) func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		return ui.Div().Row().Child(ui.Div().Key("box").Size(ui.Px(20), ui.Px(20)).Background(ui.Hex("#ff0000")).
			Play(key.Get(), []ui.Keyframe{
				{At: 0, Motion: ui.Fade()},
				{At: 0.5, Motion: ui.Grow(1.2).Opacity(0.5)},
			}, ui.Animation{Duration: 320 * time.Millisecond, Easing: ease.Linear, Repeat: repeat}))
	}
}

// Keyframes show each stop's look at its time, move between stops, end at the
// element's normal look (the missing stop at 1) and then ask for no frames.
func TestPlayStops(t *testing.T) {
	key := ui.NewSignal("a")
	u := uitest.New(t, pulseBox(key, false))
	check := func(when string, opacity, scale float32) {
		t.Helper()
		s := u.Get("box").Style()
		if !near(s.Opacity, opacity) || !near(s.Scale, scale) {
			t.Errorf("%s: opacity %v, scale %v; want %v, %v", when, s.Opacity, s.Scale, opacity, scale)
		}
	}
	check("at the start", 0, 1)
	u.Advance(80 * time.Millisecond)
	check("a quarter in", 0.25, 1.1)
	u.Advance(80 * time.Millisecond)
	check("at the middle stop", 0.5, 1.2)
	u.Advance(80 * time.Millisecond)
	check("three quarters in", 0.75, 1.1)
	u.Advance(80 * time.Millisecond)
	check("at the end", 1, 1)
	u.Frame()
	if u.Runtime().NeedsFrame() {
		t.Error("keyframes that ended still ask for frames")
	}
	// A new key plays them again from the start.
	key.Set("b")
	u.Frame()
	check("after the key changed", 0, 1)
	if !u.Runtime().NeedsFrame() {
		t.Error("restarted keyframes do not ask for frames")
	}
	u.Settle()
	check("settled", 1, 1)
}

// Repeating keyframes loop, keep asking for frames and count as settled.
func TestPlayRepeat(t *testing.T) {
	u := uitest.New(t, pulseBox(ui.NewSignal("a"), true))
	u.Advance(320*time.Millisecond + 160*time.Millisecond)
	if s := u.Get("box").Style(); !near(s.Opacity, 0.5) {
		t.Errorf("halfway through the second loop opacity = %v, want 0.5", s.Opacity)
	}
	if !u.Runtime().NeedsFrame() || !u.Runtime().Settled() {
		t.Errorf("repeating: NeedsFrame=%v Settled=%v; want true, true", u.Runtime().NeedsFrame(), u.Runtime().Settled())
	}
}

// With reduced motion finite keyframes show their end at once, and repeating
// ones hold their first frame; neither asks for frames.
func TestPlayReducedMotion(t *testing.T) {
	reduced := uitest.Settings(ui.SystemSettings{ReducedMotion: true})
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Row().Child(ui.Div().Key("box").Size(ui.Px(20), ui.Px(20)).
			Play("k", []ui.Keyframe{{At: 1, Motion: ui.Motion{}.Opacity(0.4)}}, ui.Animation{Duration: time.Second}))
	}, reduced)
	if s := u.Get("box").Style(); !near(s.Opacity, 0.4) || u.Runtime().NeedsFrame() {
		t.Errorf("finite under reduced motion: opacity %v, NeedsFrame %v; want 0.4, false", s.Opacity, u.Runtime().NeedsFrame())
	}
	loop := uitest.New(t, pulseBox(ui.NewSignal("a"), true), reduced)
	loop.Advance(100 * time.Millisecond)
	if s := loop.Get("box").Style(); !near(s.Opacity, 0) || loop.Runtime().NeedsFrame() {
		t.Errorf("repeating under reduced motion: opacity %v, NeedsFrame %v; want 0, false", s.Opacity, loop.Runtime().NeedsFrame())
	}
}

// AnimateTimeline renders each step's progress: a sequence's steps one after
// another, and every step at 1 once it ends, after which it asks for no frames.
func TestAnimateTimeline(t *testing.T) {
	var got []float32
	tl := ui.Sequence(
		ui.Animation{Duration: 96 * time.Millisecond, Easing: ease.Linear},
		ui.Animation{Duration: 192 * time.Millisecond, Easing: ease.Linear},
	)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.AnimateTimeline("intro", tl, func(p []float32) ui.Element {
			got = append(got[:0], p...)
			return ui.Div().Key("box").Size(ui.Px(20), ui.Px(20))
		})
	})
	check := func(when string, want ...float32) {
		t.Helper()
		if len(got) != len(want) || !near(got[0], want[0]) || !near(got[1], want[1]) {
			t.Errorf("%s: progress %v, want %v", when, got, want)
		}
	}
	check("at the start", 0, 0)
	u.Advance(48 * time.Millisecond)
	check("halfway through the first step", 0.5, 0)
	u.Advance(48 * time.Millisecond)
	check("at the first step's end", 1, 0)
	u.Advance(96 * time.Millisecond)
	check("halfway through the second step", 1, 0.5)
	u.Advance(96 * time.Millisecond)
	check("at the end", 1, 1)
	u.Frame()
	if u.Runtime().NeedsFrame() {
		t.Error("a timeline that ended still asks for frames")
	}
}

// A repeating timeline wraps: past its end it starts again.
func TestAnimateTimelineRepeat(t *testing.T) {
	var got []float32
	tl := ui.Parallel(
		ui.Sequence(ui.Animation{Duration: 160 * time.Millisecond, Easing: ease.Linear}),
		ui.Sequence(ui.Animation{Duration: 320 * time.Millisecond, Easing: ease.Linear}),
	).Repeat()
	if tl.Duration() != 320*time.Millisecond {
		t.Fatalf("parallel timeline lasts %v, want 320ms", tl.Duration())
	}
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.AnimateTimeline("loop", tl, func(p []float32) ui.Element {
			got = append(got[:0], p...)
			return ui.Div().Key("box").Size(ui.Px(20), ui.Px(20))
		})
	})
	u.Advance(320*time.Millisecond + 80*time.Millisecond)
	if !near(got[0], 0.5) || !near(got[1], 0.25) {
		t.Errorf("80 ms into the second loop: progress %v, want [0.5 0.25]", got)
	}
	if !u.Runtime().NeedsFrame() || !u.Runtime().Settled() {
		t.Error("a repeating timeline should keep asking for frames and count as settled")
	}
}

// With reduced motion a finite timeline jumps to its end and a repeating one
// holds its first frame.
func TestAnimateTimelineReducedMotion(t *testing.T) {
	reduced := uitest.Settings(ui.SystemSettings{ReducedMotion: true})
	for _, repeat := range []bool{false, true} {
		var got []float32
		tl := ui.Sequence(ui.Animation{Duration: 100 * time.Millisecond}, ui.Animation{Duration: 100 * time.Millisecond})
		if repeat {
			tl = tl.Repeat()
		}
		u := uitest.New(t, func(ctx *ui.Context) ui.Element {
			return ui.AnimateTimeline("t", tl, func(p []float32) ui.Element {
				got = append(got[:0], p...)
				return ui.Div()
			})
		}, reduced)
		want := float32(1)
		if repeat {
			want = 0
		}
		if got[0] != want || got[1] != want || u.Runtime().NeedsFrame() {
			t.Errorf("repeat=%v under reduced motion: progress %v, NeedsFrame %v; want all %v, false", repeat, got, u.Runtime().NeedsFrame(), want)
		}
	}
}

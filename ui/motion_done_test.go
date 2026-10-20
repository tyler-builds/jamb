package ui_test

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// expectDone checks how many times an OnDone has run.
func expectDone(t *testing.T, when string, got *int, want int) {
	t.Helper()
	if *got != want {
		t.Errorf("%s: OnDone ran %d times, want %d", when, *got, want)
	}
}

// An Animate's OnDone runs once, at its end, and not again.
func TestAnimateOnDone(t *testing.T) {
	var done int
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Animate("a", ui.Animation{Duration: 96 * time.Millisecond, OnDone: func() { done++ }},
			func(p float32) ui.Element { return ui.Div().Size(ui.Px(10), ui.Px(10)) })
	})
	u.Advance(80 * time.Millisecond)
	expectDone(t, "before the end", &done, 0)
	u.Advance(16 * time.Millisecond)
	expectDone(t, "at the end", &done, 1)
	u.Advance(200 * time.Millisecond)
	expectDone(t, "after the end", &done, 1)
}

// An OnDone runs at its time for an animation that is out of view, which
// asks for no frames meanwhile.
func TestOnDoneOffscreen(t *testing.T) {
	var done int
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Height(ui.Px(100)).ScrollY().
			Child(ui.Div().Height(ui.Px(1000)).Shrink(0)).
			Child(ui.Animate("a", ui.Animation{Duration: 200 * time.Millisecond, OnDone: func() { done++ }},
				func(p float32) ui.Element { return ui.Div().Key("dot").Size(ui.Px(10), ui.Px(10)) }))
	})
	u.Advance(48 * time.Millisecond)
	if u.Runtime().NeedsFrame() {
		t.Error("an animation out of view asks for frames")
	}
	if u.Runtime().Settled() {
		t.Error("an animation with an OnDone still to run counts as settled")
	}
	u.Settle()
	expectDone(t, "out of view, after its end", &done, 1)
}

// A key change restarts an Animate: the first run's OnDone does not run, and
// the second's runs at its own end.
func TestOnDoneNotAfterKeyChange(t *testing.T) {
	var done int
	key := ui.NewSignal("a")
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Animate(key.Get(), ui.Animation{Duration: 96 * time.Millisecond, OnDone: func() { done++ }},
			func(p float32) ui.Element { return ui.Div().Size(ui.Px(10), ui.Px(10)) })
	})
	u.Advance(48 * time.Millisecond)
	key.Set("b")
	u.Advance(64 * time.Millisecond)
	expectDone(t, "at the first run's end", &done, 0)
	u.Advance(48 * time.Millisecond)
	expectDone(t, "at the second run's end", &done, 1)
}

// An entrance's OnDone runs at its end, and not when the element is removed
// first.
func TestEnterOnDone(t *testing.T) {
	var done int
	on := ui.NewSignal(true)
	root := func(ctx *ui.Context) ui.Element {
		return ui.Div().Row().Child(ui.If(on.Get(), fadeBox("box",
			ui.Animation{Duration: 96 * time.Millisecond, Delay: 32 * time.Millisecond, OnDone: func() { done++ }})))
	}
	u := uitest.New(t, root)
	u.Advance(112 * time.Millisecond)
	expectDone(t, "before the end", &done, 0)
	u.Advance(16 * time.Millisecond)
	expectDone(t, "after the delay and duration", &done, 1)

	done = 0
	on.Set(false)
	u.Frame()
	on.Set(true)
	u.Advance(48 * time.Millisecond)
	on.Set(false)
	u.Advance(time.Second)
	expectDone(t, "removed during the entrance", &done, 0)
}

// An Exit's OnDone runs when the element has finished exiting, and not when
// it is shown again during its exit.
func TestExitOnDone(t *testing.T) {
	var done int
	on := ui.NewSignal(true)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Row().Child(ui.If(on.Get(), ui.Div().Key("box").Size(ui.Px(20), ui.Px(20)).
			Background(ui.Hex("#ff0000")).
			Exit(ui.Fade(), ui.Animation{Duration: 96 * time.Millisecond, OnDone: func() { done++ }})))
	})
	on.Set(false)
	u.Frame()
	u.Advance(80 * time.Millisecond)
	expectDone(t, "during the exit", &done, 0)
	u.Settle()
	expectDone(t, "after the exit", &done, 1)

	done = 0
	on.Set(true)
	u.Settle()
	on.Set(false)
	u.Advance(32 * time.Millisecond)
	on.Set(true)
	u.Advance(time.Second)
	expectDone(t, "shown again during the exit", &done, 0)
}

// Under reduced motion an Exit leaves at once, and its OnDone runs at the
// next frame.
func TestExitOnDoneReducedMotion(t *testing.T) {
	var done int
	on := ui.NewSignal(true)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Row().Child(ui.If(on.Get(), ui.Div().Key("box").Size(ui.Px(20), ui.Px(20)).
			Exit(ui.Fade(), ui.Animation{Duration: time.Second, OnDone: func() { done++ }})))
	}, uitest.Settings(ui.SystemSettings{ReducedMotion: true}))
	on.Set(false)
	u.Frame()
	u.Advance(16 * time.Millisecond)
	expectDone(t, "the frame after the removal", &done, 1)
}

// Keyframes' OnDone runs once at their end; repeating keyframes never run it.
func TestPlayOnDone(t *testing.T) {
	var once, loop int
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		frames := []ui.Keyframe{{At: 0.5, Motion: ui.Grow(1.1)}}
		return ui.Div().Row().
			Child(ui.Div().Key("a").Size(ui.Px(10), ui.Px(10)).
				Play("k", frames, ui.Animation{Duration: 96 * time.Millisecond, OnDone: func() { once++ }})).
			Child(ui.Div().Key("b").Size(ui.Px(10), ui.Px(10)).
				Play("k", frames, ui.Animation{Duration: 96 * time.Millisecond, Repeat: true, OnDone: func() { loop++ }}))
	})
	u.Advance(80 * time.Millisecond)
	expectDone(t, "before the end", &once, 0)
	u.Advance(16 * time.Millisecond)
	expectDone(t, "at the end", &once, 1)
	u.Advance(time.Second)
	expectDone(t, "a second later", &once, 1)
	expectDone(t, "repeating keyframes", &loop, 0)
}

// Each step of a timeline runs its OnDone when the step ends.
func TestTimelineStepOnDone(t *testing.T) {
	var first, second int
	tl := ui.Sequence(
		ui.Animation{Duration: 48 * time.Millisecond, OnDone: func() { first++ }},
		ui.Animation{Duration: 48 * time.Millisecond, Delay: 32 * time.Millisecond, OnDone: func() { second++ }},
	)
	if tl.Duration() != 128*time.Millisecond {
		t.Fatalf("the timeline lasts %v, want 128ms (the delay included)", tl.Duration())
	}
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.AnimateTimeline("t", tl, func(p []float32) ui.Element { return ui.Div() })
	})
	u.Advance(48 * time.Millisecond)
	expectDone(t, "the first step at its end", &first, 1)
	expectDone(t, "the second step before it starts", &second, 0)
	u.Advance(64 * time.Millisecond)
	expectDone(t, "the second step before its end", &second, 0)
	u.Advance(16 * time.Millisecond)
	expectDone(t, "the second step at its end", &second, 1)
}

package ui

import (
	"math"
	"testing"
	"time"
)

const pausedAnimDuration = 400 * time.Millisecond

// animColumn is a 300 px scrolling column of 40 rows, each 50 px high. Row 30,
// 1,500 px down, holds the element that el returns, so it starts out of view.
func animColumn(el func() Element) func(*Context) Element {
	return func(ctx *Context) Element {
		list := Div().Key("list").Height(Full).ScrollY()
		for i := 0; i < 40; i++ {
			row := Div().Keyf("row%d", i).Height(Px(50)).Shrink(0)
			if i == 30 {
				row.Child(el())
			}
			list.Child(row)
		}
		return list
	}
}

// spinningDot is a repeating animation keyed "dot". It counts its renders and
// records the last progress it was given.
func spinningDot(renders *int, last *float32) Element {
	return Animate("spin", Animation{Duration: pausedAnimDuration, Repeat: true, Easing: linear}, func(p float32) Element {
		*renders++
		*last = p
		return Div().Key("dot").Size(Px(10), Px(10)).Background(Hex("#ffffff"))
	})
}

// stepFrames advances the clock by step and renders n frames.
func stepFrames(r *overlayRig, n int, step time.Duration) {
	for i := 0; i < n; i++ {
		r.now = r.now.Add(step)
		r.frame()
	}
}

// A repeating animation whose element is out of view asks for no frames and
// is not rendered again, however long it stays there.
func TestOffscreenRepeatingAnimationRequestsNoFrames(t *testing.T) {
	var renders int
	var last float32
	r := animRig(animColumn(func() Element { return spinningDot(&renders, &last) }))
	if paintedKey(t, r.rt, "dot") {
		t.Fatal("the animation is out of view but was painted")
	}
	// It asks for one more frame, then none.
	r.settle(t, 16*time.Millisecond)
	before := renders
	stepFrames(r, 60, 16*time.Millisecond)
	if r.rt.NeedsFrame() {
		t.Fatal("an off-screen repeating animation asks for frames after a second")
	}
	if renders != before {
		t.Errorf("the off-screen animation rendered %d times, want 0", renders-before)
	}
}

// Scrolling the animation into view resumes it at the progress its time
// implies, not at the progress it had when it left view, and it runs on from
// there. Scrolling it away again stops it.
func TestScrollingAnimationIntoViewResumesAtItsProgress(t *testing.T) {
	var renders int
	var last float32 = -1
	r := animRig(animColumn(func() Element { return spinningDot(&renders, &last) }))
	start := r.now // the animation starts when it is first built
	// progressAt is the progress a repeating animation shows at a time: the
	// fraction of a duration elapsed since it started.
	progressAt := func(at time.Time) float32 {
		d := pausedAnimDuration.Seconds()
		return float32(math.Mod(at.Sub(start).Seconds()/d, 1))
	}
	stepFrames(r, 10, 16*time.Millisecond)
	// One second passes out of view, with no frames: the animation resumes at
	// the progress its time implies, not the one it had when it left view.
	r.now = r.now.Add(time.Second)
	r.rt.Scroll(100, 100, 0, 1500, true, 0)
	r.frame()
	if !r.rt.NeedsFrame() {
		t.Fatal("scrolled into view, the repeating animation does not ask for frames")
	}
	if !paintedKey(t, r.rt, "dot") {
		t.Fatal("scrolled into view, the animation was not painted")
	}
	if want := progressAt(r.now); math.Abs(float64(last-want)) > 1e-3 {
		t.Errorf("progress on resume = %v, want %v (time based, no jump)", last, want)
	}
	// 100 ms later the progress has moved on from the resumed value.
	r.now = r.now.Add(100 * time.Millisecond)
	r.frame()
	if want := progressAt(r.now); math.Abs(float64(last-want)) > 1e-3 {
		t.Errorf("progress 100 ms after resume = %v, want %v", last, want)
	}
	// Scrolled away again: it stops asking for frames.
	r.rt.Scroll(100, 100, 0, -1500, true, 0)
	r.settle(t, 16*time.Millisecond)
	before := renders
	stepFrames(r, 30, 16*time.Millisecond)
	if renders != before {
		t.Errorf("the animation rendered %d times while out of view again", renders-before)
	}
}

// A finite animation out of view also asks for no frames. When it is shown
// after its end it renders its final value.
func TestOffscreenFiniteAnimationShowsItsEndWhenVisible(t *testing.T) {
	var last float32 = -1
	r := animRig(animColumn(func() Element {
		return Animate("fade", Animation{Duration: pausedAnimDuration, Easing: linear}, func(p float32) Element {
			last = p
			return Div().Key("dot").Size(Px(10), Px(10)).Background(Hex("#ffffff"))
		})
	}))
	stepFrames(r, 30, 20*time.Millisecond)
	if r.rt.NeedsFrame() {
		t.Fatal("an off-screen finite animation asks for frames")
	}
	r.now = r.now.Add(time.Second)
	r.rt.Scroll(100, 100, 0, 1500, true, 0)
	r.frame()
	if last != 1 {
		t.Errorf("progress on resume = %v, want 1", last)
	}
	if r.rt.NeedsFrame() {
		t.Error("the finished animation still asks for frames once visible")
	}
}

// The indeterminate progress bar and the progress ring are repeating
// animations, so out of view they ask for no frames and in view they run.
func TestOffscreenProgressIndicatorsAreIdle(t *testing.T) {
	indicators := map[string]func() Element{
		"ring": func() Element { return ProgressRing().Key("ring").Label("Loading") },
		"bar":  func() Element { return ProgressBar(0).Indeterminate().Key("bar").Label("Loading") },
	}
	for name, el := range indicators {
		t.Run(name, func(t *testing.T) {
			r := animRig(animColumn(el))
			var warnings []string
			r.rt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
			r.settle(t, 16*time.Millisecond)
			stepFrames(r, 30, 16*time.Millisecond)
			if r.rt.NeedsFrame() {
				t.Fatalf("an off-screen %s asks for frames after half a second", name)
			}
			r.rt.Scroll(100, 100, 0, 1500, true, 0)
			r.frame()
			if !r.rt.NeedsFrame() {
				t.Fatalf("a %s scrolled into view does not ask for frames", name)
			}
			// Resuming renders the indicator again; its identities must hold.
			if len(warnings) > 0 {
				t.Fatalf("resuming the %s warned: %v", name, warnings)
			}
		})
	}
}

// When the element around a running animation is rebuilt and the animation
// itself is not, it must still be drawn and keep running: the animation is
// reused, and only the rebuilt parent places it. This is how a page switch
// reuses the progress bars of a page the user returns to.
func TestRebuiltParentKeepsRunningAnimationDrawn(t *testing.T) {
	label := NewSignal("first")
	var renders int
	var last float32
	spin := Animate("spin", Animation{Duration: pausedAnimDuration, Repeat: true, Easing: linear}, func(p float32) Element {
		renders++
		last = p
		return Div().Key("dot").Size(Px(10), Px(10)).Background(Hex("#ffffff"))
	})
	r := animRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Text(label.Get()).Key("label")).
			Child(Div().Key("row").Row().Child(spin))
	})
	stepFrames(r, 3, 16*time.Millisecond)
	if !paintedKey(t, r.rt, "dot") {
		t.Fatal("the animation is not drawn on screen")
	}
	label.Set("second")
	stepFrames(r, 5, 16*time.Millisecond)
	if !paintedKey(t, r.rt, "dot") {
		t.Fatal("after its parent was rebuilt, the animation is not drawn")
	}
	before := renders
	stepFrames(r, 5, 16*time.Millisecond)
	if renders == before {
		t.Fatal("after its parent was rebuilt, the animation stopped running")
	}
	_ = last
}

// The same, with the parent a component of its own: its signal changes, it is
// rendered again, and the animation it holds is reused.
func TestComponentRerenderKeepsRunningAnimationDrawn(t *testing.T) {
	page := NewSignal(1)
	var renders int
	spin := Animate("spin", Animation{Duration: pausedAnimDuration, Repeat: true, Easing: linear}, func(p float32) Element {
		renders++
		return Div().Key("dot").Size(Px(10), Px(10)).Background(Hex("#ffffff"))
	})
	r := animRig(func(ctx *Context) Element {
		return Div().Column().Child(Component(func(ctx *Context) Element {
			return Div().Key("row").Row().Child(Text(map[int]string{1: "one", 2: "two"}[page.Get()]).Key("label")).Child(spin)
		}))
	})
	stepFrames(r, 3, 16*time.Millisecond)
	page.Set(2)
	stepFrames(r, 5, 16*time.Millisecond)
	if !paintedKey(t, r.rt, "dot") {
		t.Fatal("after its component re-rendered, the animation is not drawn")
	}
	before := renders
	stepFrames(r, 5, 16*time.Millisecond)
	if renders == before {
		t.Fatal("after its component re-rendered, the animation stopped running")
	}
}

package ui_test

import (
	"testing"
	"time"

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

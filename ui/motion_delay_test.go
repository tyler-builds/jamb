package ui_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ease"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// fadeBox is a red box keyed key that enters with a over a fade.
func fadeBox(key string, a ui.Animation) *ui.DivElement {
	return ui.Div().Key(key).Size(ui.Px(20), ui.Px(20)).Background(ui.Hex("#ff0000")).Enter(ui.Fade(), a)
}

// A delayed entrance shows its Motion while it waits and asks for no frames
// (and paints nothing) until it is due; the UI has not settled meanwhile.
func TestEnterDelay(t *testing.T) {
	on := ui.NewSignal(false)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Row().Child(ui.If(on.Get(), fadeBox("box",
			ui.Animation{Duration: 96 * time.Millisecond, Delay: 160 * time.Millisecond, Easing: ease.Linear})))
	})
	on.Set(true)
	u.Frame()
	for i := 0; i < 10; i++ {
		if s := u.Get("box").Style(); s.Opacity != 0 {
			t.Fatalf("frame %d of the delay: opacity %v, want 0", i, s.Opacity)
		}
		if u.Runtime().NeedsFrame() {
			t.Fatalf("frame %d of the delay asks for another frame", i)
		}
		if u.Runtime().Settled() {
			t.Fatalf("frame %d of the delay counts as settled", i)
		}
		u.Advance(16 * time.Millisecond)
		if i < 9 && u.Runtime().Stats().Painted {
			t.Fatalf("frame %d of the delay painted", i+1)
		}
	}
	// 160 ms: the delay has ended and the entrance runs.
	if !u.Runtime().NeedsFrame() {
		t.Fatal("the entrance does not run when its delay ends")
	}
	u.Advance(48 * time.Millisecond)
	if s := u.Get("box").Style(); !near(s.Opacity, 0.5) {
		t.Errorf("48 ms after the delay opacity = %v, want 0.5", s.Opacity)
	}
	u.Settle()
	if s := u.Get("box").Style(); s.Opacity != 1 {
		t.Errorf("settled opacity = %v, want 1", s.Opacity)
	}
}

// Settle waits through a delay: one call shows the entrance at its end.
func TestSettleCoversDelay(t *testing.T) {
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Row().Child(fadeBox("box", ui.Animation{Duration: 100 * time.Millisecond, Delay: time.Second}))
	})
	start := u.Now()
	u.Settle()
	if s := u.Get("box").Style(); s.Opacity != 1 {
		t.Errorf("settled opacity = %v, want 1", s.Opacity)
	}
	if d := u.Now().Sub(start); d < 1100*time.Millisecond {
		t.Errorf("Settle returned after %v, before the delayed entrance ended", d)
	}
}

// A delayed Animate renders its first frame and nothing more until it is due,
// then runs; delayed keyframes show their first stop meanwhile.
func TestAnimateAndPlayDelay(t *testing.T) {
	var renders int
	var last float32 = -1
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Row().
			Child(ui.Animate("a", ui.Animation{Duration: 96 * time.Millisecond, Delay: 96 * time.Millisecond, Easing: ease.Linear},
				func(p float32) ui.Element {
					renders++
					last = p
					return ui.Div().Size(ui.Px(10), ui.Px(10))
				})).
			Child(ui.Div().Key("box").Size(ui.Px(20), ui.Px(20)).Play("k",
				[]ui.Keyframe{{At: 0, Motion: ui.Motion{}.Opacity(0.2)}},
				ui.Animation{Duration: 96 * time.Millisecond, Delay: 96 * time.Millisecond, Easing: ease.Linear}))
	})
	if renders != 1 || last != 0 {
		t.Fatalf("first frame: %d renders at %v, want 1 at 0", renders, last)
	}
	if s := u.Get("box").Style(); !near(s.Opacity, 0.2) {
		t.Errorf("delayed keyframes show opacity %v, want their first stop 0.2", s.Opacity)
	}
	if u.Runtime().NeedsFrame() || u.Runtime().Settled() {
		t.Errorf("in the delay: NeedsFrame %v, Settled %v; want false, false", u.Runtime().NeedsFrame(), u.Runtime().Settled())
	}
	u.Advance(80 * time.Millisecond)
	if renders != 1 {
		t.Errorf("the delayed animation rendered %d times in its delay, want 1", renders)
	}
	u.Advance(64 * time.Millisecond) // 144 ms: 48 ms after the delay
	if !near(last, 0.5) {
		t.Errorf("48 ms after the delay progress = %v, want 0.5", last)
	}
	if s := u.Get("box").Style(); !near(s.Opacity, 0.6) {
		t.Errorf("48 ms after the delay keyframes show opacity %v, want 0.6", s.Opacity)
	}
	u.Settle()
	if last != 1 {
		t.Errorf("settled progress = %v, want 1", last)
	}
}

// staggerRows is a column of n rows keyed row<i> that enter with a fade, each
// stagger after the one before.
func staggerRows(n *ui.Signal[int], stagger time.Duration) func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		col := ui.Div().Column()
		for i := 0; i < n.Get(); i++ {
			col.Child(ui.Div().Keyf("row%d", i).Size(ui.Px(20), ui.Px(4)).Background(ui.Hex("#ff0000")).
				Enter(ui.Fade(), ui.Animation{Duration: 96 * time.Millisecond, Stagger: stagger, Easing: ease.Linear}))
		}
		return col
	}
}

// Rows that enter together are staggered in build order; a row appended later
// enters at once.
func TestStagger(t *testing.T) {
	n := ui.NewSignal(0)
	u := uitest.New(t, staggerRows(n, 32*time.Millisecond))
	n.Set(5)
	u.Frame()
	u.Advance(80 * time.Millisecond)
	for i, want := range []float32{80.0 / 96, 48.0 / 96, 16.0 / 96, 0, 0} {
		if s := u.Get(fmt.Sprintf("row%d", i)).Style(); !near(s.Opacity, want) {
			t.Errorf("80 ms in, row %d has opacity %v, want %v", i, s.Opacity, want)
		}
	}
	u.Settle()
	n.Set(6)
	u.Frame()
	u.Advance(48 * time.Millisecond)
	if s := u.Get("row5").Style(); !near(s.Opacity, 0.5) {
		t.Errorf("a row appended later has opacity %v 48 ms in, want 0.5 (no stagger)", s.Opacity)
	}
}

// A stagger counts up to 16: the 17th row and those after it start together.
func TestStaggerCap(t *testing.T) {
	n := ui.NewSignal(0)
	u := uitest.New(t, staggerRows(n, 16*time.Millisecond))
	n.Set(20)
	u.Frame()
	u.Advance(16*16*time.Millisecond + 48*time.Millisecond)
	for _, i := range []int{16, 17, 19} {
		if s := u.Get(fmt.Sprintf("row%d", i)).Style(); !near(s.Opacity, 0.5) {
			t.Errorf("row %d has opacity %v, want 0.5 (the stagger stops at 16)", i, s.Opacity)
		}
	}
}

// Stagger and Delay are ignored under reduced motion, and an OnDone runs at
// the next frame.
func TestDelayStaggerReducedMotion(t *testing.T) {
	n := ui.NewSignal(0)
	var done int
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		col := ui.Div().Column()
		for i := 0; i < n.Get(); i++ {
			col.Child(ui.Div().Keyf("row%d", i).Size(ui.Px(20), ui.Px(4)).
				Enter(ui.Fade(), ui.Animation{Duration: time.Second, Delay: time.Second, Stagger: time.Second, OnDone: func() { done++ }}))
		}
		return col
	}, uitest.Settings(ui.SystemSettings{ReducedMotion: true}))
	n.Set(3)
	u.Frame()
	for i := 0; i < 3; i++ {
		if s := u.Get(fmt.Sprintf("row%d", i)).Style(); s.Opacity != 1 {
			t.Errorf("row %d under reduced motion has opacity %v, want 1", i, s.Opacity)
		}
	}
	if u.Runtime().NeedsFrame() {
		t.Error("delayed entrances under reduced motion ask for frames")
	}
	u.Advance(16 * time.Millisecond)
	if done != 3 {
		t.Errorf("OnDone ran %d times by the next frame, want 3", done)
	}
}

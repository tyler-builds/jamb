package ui

import (
	"math"
	"testing"
	"time"
)

// A delayed exit keeps the element's normal look while it waits, asking for
// no frames, then fades; the ghost goes after the delay and the duration.
func TestExitDelay(t *testing.T) {
	on := NewSignal(true)
	r := animRig(func(ctx *Context) Element {
		return Div().Row().Child(If(on.Get(), Div().Key("panel").Width(Px(100)).Height(Px(20)).Background(Color{1, 0, 0, 1}).
			Exit(Fade(), Animation{Duration: 96 * time.Millisecond, Delay: 96 * time.Millisecond, Easing: linear})))
	})
	on.Set(false)
	r.frame()
	if r.rt.NeedsFrame() || r.rt.Settled() {
		t.Errorf("in the exit's delay: NeedsFrame %v, Settled %v; want false, false", r.rt.NeedsFrame(), r.rt.Settled())
	}
	r.now = r.now.Add(80 * time.Millisecond)
	r.frame()
	if q, ok := redQuad(r.rt, 100); !ok || q.Background.A != 1 {
		t.Errorf("in the exit's delay the ghost is drawn with %+v, want opacity 1", q.Background)
	}
	r.now = r.now.Add(64 * time.Millisecond) // 48 ms into the exit
	r.frame()
	if q, ok := redQuad(r.rt, 100); !ok || math.Abs(float64(q.Background.A-0.5)) > 1e-3 {
		t.Errorf("48 ms into the exit the ghost is drawn with %+v, want opacity 0.5", q.Background)
	}
	r.settle(t, 16*time.Millisecond)
	if _, ok := quadOf(r.rt, 100); ok {
		t.Error("the ghost is still drawn after its delay and exit")
	}
	if len(r.rt.ghosts) != 0 {
		t.Errorf("%d ghosts after the exit, want 0", len(r.rt.ghosts))
	}
}

// A delayed motion that is not drawn records no wake-up, so it neither asks
// for frames nor keeps the UI from settling.
func TestOffscreenDelayIsIdle(t *testing.T) {
	r := animRig(animColumn(func() Element {
		return Div().Key("dot").Size(Px(10), Px(10)).
			Enter(Fade(), Animation{Duration: 100 * time.Millisecond, Delay: time.Second})
	}))
	r.frame()
	if r.rt.NeedsFrame() || !r.rt.Settled() {
		t.Errorf("an off-screen delayed entrance: NeedsFrame %v, Settled %v; want false, true", r.rt.NeedsFrame(), r.rt.Settled())
	}
}

package ui

import (
	"math"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/scene"
)

func linear(t float32) float32 { return t }

// animRig renders root with a clock the test advances by hand.
func animRig(root func(*Context) Element) *overlayRig {
	return newOverlayRig(root, 400, 300)
}

// The progress reaches 0, halfway and 1 at the start, half and end of the
// duration, and the element is rendered once per frame, not in a loop.
func TestAnimateProgress(t *testing.T) {
	const d = 400 * time.Millisecond
	var ts []float32
	r := animRig(func(ctx *Context) Element {
		return Animate("fade", Animation{Duration: d, Easing: linear}, func(t float32) Element {
			ts = append(ts, t)
			return Div().Row().Child(Div().Key("box").Width(Px(100 + 100*t)).Height(Px(10)))
		})
	})
	step := func(by time.Duration) {
		before := len(ts)
		r.now = r.now.Add(by)
		r.frame()
		if n := len(ts) - before; n > 1 {
			t.Fatalf("%d renders in one frame, want at most 1", n)
		}
	}
	if !r.rt.NeedsFrame() {
		t.Fatal("a running animation does not ask for a frame")
	}
	step(d / 2)
	step(d / 2)
	step(0)
	want := []float32{0, 0.5, 1}
	if len(ts) < len(want) {
		t.Fatalf("progress %v, want at least %v", ts, want)
	}
	for i, w := range want {
		if math.Abs(float64(ts[i]-w)) > 1e-4 {
			t.Errorf("progress %v, want %v", ts, want)
			break
		}
	}
	if w := r.find(t, "box").W; math.Abs(float64(w-200)) > 1e-3 {
		t.Errorf("box width at the end %v, want 200", w)
	}
	if r.rt.NeedsFrame() {
		t.Error("the animation ended but the runtime still asks for frames")
	}
	n := len(ts)
	step(time.Second)
	step(time.Second)
	if len(ts) != n {
		t.Errorf("render function ran %d more times after the end", len(ts)-n)
	}
}

// Repeat loops: after one duration plus half of another, progress is halfway
// through the second, and it never stops asking for frames.
func TestAnimateRepeatWraps(t *testing.T) {
	const d = 200 * time.Millisecond
	var last float32
	r := animRig(func(ctx *Context) Element {
		return Animate("spin", Animation{Duration: d, Easing: linear, Repeat: true}, func(t float32) Element {
			last = t
			return Div().Key("spin").Width(Px(10 + 10*t)).Height(Px(10))
		})
	})
	r.now = r.now.Add(d + d/2)
	r.frame()
	if math.Abs(float64(last-0.5)) > 1e-3 {
		t.Errorf("progress after 1.5 durations = %v, want 0.5", last)
	}
	for i := 0; i < 5; i++ {
		r.now = r.now.Add(73 * time.Millisecond)
		r.frame()
		if !r.rt.NeedsFrame() {
			t.Fatal("a repeating animation stopped asking for frames")
		}
		if last < 0 || last > 1 {
			t.Fatalf("progress %v is outside 0..1", last)
		}
	}
}

// A repeating animation keeps asking for frames but counts as settled, so a
// wait for the UI to come to rest ends. A finite one does not, until it ends.
func TestAnimateSettled(t *testing.T) {
	const d = 200 * time.Millisecond
	loop := animRig(func(ctx *Context) Element {
		return Animate("spin", Animation{Duration: d, Repeat: true}, func(t float32) Element {
			return Div().Width(Px(10 + 10*t)).Height(Px(10))
		})
	})
	loop.now = loop.now.Add(d / 2)
	loop.frame()
	if !loop.rt.NeedsFrame() || !loop.rt.Settled() {
		t.Errorf("repeating: NeedsFrame=%v Settled=%v; want true, true", loop.rt.NeedsFrame(), loop.rt.Settled())
	}

	once := animRig(func(ctx *Context) Element {
		return Animate("fade", Animation{Duration: d}, func(t float32) Element {
			return Div().Width(Px(10 + 10*t)).Height(Px(10))
		})
	})
	once.now = once.now.Add(d / 2)
	once.frame()
	if once.rt.Settled() {
		t.Error("a running finite animation counts as settled")
	}
	once.now = once.now.Add(d)
	once.frame()
	once.frame()
	if !once.rt.Settled() || once.rt.NeedsFrame() {
		t.Errorf("ended: Settled=%v NeedsFrame=%v; want true, false", once.rt.Settled(), once.rt.NeedsFrame())
	}
}

// A new key starts the animation again, from 0.
func TestAnimateKeyRestarts(t *testing.T) {
	const d = 200 * time.Millisecond
	var last float32
	key := NewSignal("first")
	r := animRig(func(ctx *Context) Element {
		k := key.Get()
		return Animate(k, Animation{Duration: d, Easing: linear}, func(t float32) Element {
			last = t
			return Div().Key("box").Height(Px(10))
		})
	})
	r.now = r.now.Add(d)
	r.frame()
	if last != 1 {
		t.Fatalf("progress at the end of the first key = %v, want 1", last)
	}
	key.Set("second")
	r.frame()
	if last != 0 {
		t.Errorf("progress after the key changed = %v, want 0", last)
	}
	r.now = r.now.Add(d / 2)
	r.frame()
	if math.Abs(float64(last-0.5)) > 1e-3 {
		t.Errorf("progress halfway through the new key = %v, want 0.5", last)
	}
}

// redQuad returns the first quad whose width is w and whose color is red.
func redQuad(rt *Runtime, w float32) (scene.Quad, bool) {
	for _, q := range rt.scene.Quads {
		if q.Bounds.W == w && q.Background.R == 1 && q.Background.A > 0 {
			return q, true
		}
	}
	return scene.Quad{}, false
}

// quadOf returns the first quad whose width is w, of any color.
func quadOf(rt *Runtime, w float32) (scene.Quad, bool) {
	for _, q := range rt.scene.Quads {
		if q.Bounds.W == w {
			return q, true
		}
	}
	return scene.Quad{}, false
}

// hits reports whether the element with key is under (x, y).
func hits(rt *Runtime, key string, x, y float32) bool {
	for _, n := range rt.hitTest(x, y) {
		if n.key == key {
			return true
		}
	}
	return false
}

// settle advances the clock by step until the runtime stops asking for frames.
func (r *overlayRig) settle(t *testing.T, step time.Duration) {
	t.Helper()
	for i := 0; i < 1000 && r.rt.NeedsFrame(); i++ {
		r.now = r.now.Add(step)
		r.frame()
	}
	if r.rt.NeedsFrame() {
		t.Fatal("the runtime never went idle")
	}
}

func redBox(on bool) *DivElement {
	b := Div().Key("box").Width(Px(100)).Height(Px(20)).Background(Color{1, 0, 0, 1}).Transition(100 * time.Millisecond)
	if on {
		b.Offset(50, 0).Opacity(0.5)
	}
	return b
}

// A transition eases offset and opacity over frames.
func TestTransitionEases(t *testing.T) {
	on := NewSignal(false)
	r := animRig(func(ctx *Context) Element {
		return Div().Row().Child(redBox(on.Get()))
	})
	if x := r.find(t, "box").X; x != 0 {
		t.Fatalf("box starts at %v, want 0", x)
	}
	on.Set(true)
	r.now = r.now.Add(16 * time.Millisecond)
	r.frame()
	if x := r.find(t, "box").X; x <= 0 || x >= 50 {
		t.Errorf("box at %v after one frame of the transition, want between 0 and 50", x)
	}
	if q, ok := redQuad(r.rt, 100); !ok || q.Background.A <= 0.5 || q.Background.A >= 1 {
		t.Errorf("box opacity after one frame: %+v, want between 0.5 and 1", q.Background)
	}
	r.settle(t, 16*time.Millisecond)
	if x := r.find(t, "box").X; math.Abs(float64(x-50)) > 1e-3 {
		t.Errorf("box at %v after the transition, want 50", x)
	}
	if q, ok := redQuad(r.rt, 100); !ok || math.Abs(float64(q.Background.A-0.5)) > 1e-3 {
		t.Errorf("box opacity after the transition: %+v, want 0.5", q.Background)
	}
}

// The first paint shows the target values at once, with no transition from
// the defaults.
func TestFirstPaintSnaps(t *testing.T) {
	r := animRig(func(ctx *Context) Element {
		return Div().Row().Child(redBox(true))
	})
	if x := r.find(t, "box").X; x != 50 {
		t.Errorf("box at %v on the first paint, want 50 with no transition", x)
	}
	if q, ok := redQuad(r.rt, 100); !ok || q.Background.A != 0.5 {
		t.Errorf("first paint opacity %+v, want 0.5", q.Background)
	}
}

// Offset moves the element for hit testing and Find; Scale does not.
func TestOffsetHitsScaleDoesNot(t *testing.T) {
	r := animRig(func(ctx *Context) Element {
		return Div().Row().Child(Div().Key("moved").Width(Px(100)).Height(Px(20)).Offset(200, 0)).
			Child(Div().Key("big").Width(Px(100)).Height(Px(20)).Scale(2))
	})
	if !hits(r.rt, "moved", 250, 10) {
		t.Error("an offset element is not hit where it is drawn")
	}
	if hits(r.rt, "moved", 10, 10) {
		t.Error("an offset element is still hit where it was laid out")
	}
	if x := r.find(t, "moved").X; x != 200 {
		t.Errorf("Find of an offset element gives X %v, want 200", x)
	}
	// "big" is laid out at x 100..200. Scaled about its center it covers
	// 50..250, but only its laid-out box is hit.
	if !hits(r.rt, "big", 110, 10) {
		t.Error("a scaled element is not hit in its laid-out box")
	}
	if hits(r.rt, "big", 230, 10) {
		t.Error("a scaled element is hit outside its laid-out box")
	}
}

// An element with Enter appears from its motion and settles in place.
func TestEnterFromMotion(t *testing.T) {
	on := NewSignal(false)
	r := animRig(func(ctx *Context) Element {
		if !on.Get() {
			return Div().Row()
		}
		return Div().Row().Child(Div().Key("panel").Width(Px(100)).Height(Px(20)).Background(Color{1, 0, 0, 1}).
			Enter(Fade().Slide(0, -6), Animation{Duration: 200 * time.Millisecond}))
	})
	on.Set(true)
	r.frame()
	if y := r.find(t, "panel").Y; math.Abs(float64(y+6)) > 1e-3 {
		t.Errorf("panel at Y %v on its first frame, want -6 (the motion's offset)", y)
	}
	if q, ok := redQuad(r.rt, 100); ok {
		t.Errorf("panel on its first frame is drawn with opacity %v, want 0", q.Background.A)
	}
	r.settle(t, 20*time.Millisecond)
	if y := r.find(t, "panel").Y; y != 0 {
		t.Errorf("panel settled at Y %v, want 0", y)
	}
	if q, ok := redQuad(r.rt, 100); !ok || q.Background.A != 1 {
		t.Errorf("panel settled with opacity %+v, want 1", q.Background)
	}
}

// An element with Exit keeps drawing while it leaves, but is not in the tree:
// it cannot be hit or found, and once its exit ends it is gone.
func TestExitGhost(t *testing.T) {
	on := NewSignal(true)
	r := animRig(func(ctx *Context) Element {
		if !on.Get() {
			return Div().Row()
		}
		return Div().Row().Child(Div().Key("panel").Width(Px(100)).Height(Px(20)).Background(Color{1, 0, 0, 1}).
			Exit(Fade(), Animation{Duration: 100 * time.Millisecond}))
	})
	if !hits(r.rt, "panel", 10, 10) {
		t.Fatal("panel is not hit while shown")
	}
	on.Set(false)
	r.frame()
	if _, ok := r.rt.Find("panel"); ok {
		t.Error("an exiting element is still found")
	}
	if hits(r.rt, "panel", 10, 10) {
		t.Error("an exiting element is still hit")
	}
	if _, ok := redQuad(r.rt, 100); !ok {
		t.Fatal("an exiting element is not drawn on its first exit frame")
	}
	r.now = r.now.Add(40 * time.Millisecond)
	r.frame()
	if q, ok := redQuad(r.rt, 100); !ok || q.Background.A <= 0 || q.Background.A >= 1 {
		t.Errorf("exiting element at 40ms has opacity %+v, want between 0 and 1", q.Background)
	}
	if hits(r.rt, "panel", 10, 10) {
		t.Error("an exiting element is hit during its exit")
	}
	r.settle(t, 20*time.Millisecond)
	if _, ok := quadOf(r.rt, 100); ok {
		t.Error("the exited element is still drawn after its exit ended")
	}
	r.frame()
	if r.rt.Stats().Painted {
		t.Error("an idle frame after the exit paints")
	}
}

// Showing an element again while it exits drops the ghost, so it is drawn once.
func TestExitDroppedWhenShownAgain(t *testing.T) {
	on := NewSignal(true)
	r := animRig(func(ctx *Context) Element {
		if !on.Get() {
			return Div().Row()
		}
		return Div().Row().Child(Div().Key("panel").Width(Px(100)).Height(Px(20)).Background(Color{1, 0, 0, 1}).
			Exit(Fade(), Animation{Duration: 100 * time.Millisecond}))
	})
	on.Set(false)
	r.frame()
	r.now = r.now.Add(30 * time.Millisecond)
	on.Set(true)
	r.frame()
	n := 0
	for _, q := range r.rt.scene.Quads {
		if q.Bounds.W == 100 {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d quads for the element while it re-appears, want 1", n)
	}
	if q, ok := redQuad(r.rt, 100); !ok || q.Background.A != 1 {
		t.Errorf("re-shown element has opacity %+v, want 1 (it is shown, not exiting)", q.Background)
	}
}

// Quads, glyphs and the clip of a scaled element are scaled about its center.
func TestScaleAboutCenter(t *testing.T) {
	r := animRig(func(ctx *Context) Element {
		return Div().Row().Child(Div().Key("sc").Width(Px(100)).Height(Px(50)).Background(Color{1, 0, 0, 1}).
			Scale(0.5).ClipContent().Child(Text("hi")))
	})
	q, ok := redQuad(r.rt, 50)
	if !ok {
		t.Fatalf("no quad of the scaled width 50; quads %+v", r.rt.scene.Quads)
	}
	want := scene.Rect{X: 25, Y: 12.5, W: 50, H: 25}
	if q.Bounds != want {
		t.Errorf("scaled quad at %+v, want %+v (centered on 50,25)", q.Bounds, want)
	}
	var clip *scene.Rect
	for _, op := range r.rt.scene.Ops {
		if op.Kind == scene.OpPushClip {
			c := op.Clip
			clip = &c
		}
	}
	if clip == nil {
		t.Fatal("no clip for a clipped scaled element")
	}
	if clip.W > 50 || clip.H > 25 || clip.X < 25 || clip.Y < 12.5 {
		t.Errorf("clip %+v is not scaled about the center (within 25,12.5 50x25)", *clip)
	}
	if len(r.rt.scene.Glyphs) == 0 {
		t.Fatal("no glyphs for the scaled text")
	}
	for _, g := range r.rt.scene.Glyphs {
		if g.Bounds.X < 25 || g.Bounds.Right() > 75 || g.Bounds.Y < 12.5 || g.Bounds.Bottom() > 37.5 {
			t.Errorf("glyph %+v lies outside the scaled box", g.Bounds)
		}
	}
}

// After every animation has ended, frames do no paint work.
func TestIdleAfterAnimations(t *testing.T) {
	on := NewSignal(false)
	r := animRig(func(ctx *Context) Element {
		return Div().Row().Child(redBox(on.Get())).
			Child(Animate("pulse", Animation{Duration: 200 * time.Millisecond}, func(t float32) Element {
				return Div().Key("pulse").Width(Px(10 + 10*t)).Height(Px(10))
			}))
	})
	on.Set(true)
	r.frame()
	r.settle(t, 16*time.Millisecond)
	for i := 0; i < 2; i++ {
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
		if r.rt.NeedsFrame() || r.rt.Stats().Painted {
			t.Fatalf("frame %d after the animations ended did paint work", i)
		}
	}
}

// Removing a focused field clears focus, so a field inside an exiting element
// draws no caret and does not claim the input-method position while its ghost
// fades out.
func TestExitingFieldIsNotFocused(t *testing.T) {
	show := NewSignal(true)
	h := NewFocusHandle()
	v := NewSignal("hello")
	r := newKeyRig(func(ctx *Context) Element {
		return Div().Child(If(show.Get(), Div().Exit(Fade(), Animation{Duration: 100 * time.Millisecond}).
			Child(TextInput(v).FocusHandle(h))))
	})
	h.Focus()
	r.frame()
	if !r.rt.hasTextInput {
		t.Fatal("the focused field did not report its caret")
	}
	show.Set(false)
	r.frame()
	if len(r.rt.ghosts) != 1 {
		t.Fatalf("%d ghosts, want the exiting element", len(r.rt.ghosts))
	}
	if r.rt.hasTextInput {
		t.Error("the exiting field still reports a caret for the input method")
	}
}

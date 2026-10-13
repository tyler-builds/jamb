package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// The runtime has no platform or GPU dependencies, so UI behavior can be
// tested headlessly: build a tree, run frames, inject input, inspect results.

func newTestRuntime(root func(*Context) Element, w, h, scale float32) *Runtime {
	rt := NewRuntime(root, DarkTheme, nil)
	rt.SetViewport(w, h, scale)
	rt.Frame(time.Now())
	return rt
}

func TestSignalRerendersOnlySubscribedComponent(t *testing.T) {
	a, b := NewSignal(0), NewSignal(0)
	runsA, runsB, runsRoot := 0, 0, 0
	rt := newTestRuntime(func(ctx *Context) Element {
		runsRoot++
		return Div().
			Child(Component(func(ctx *Context) Element { runsA++; return Textf("a=%d", a.Get()) })).
			Child(Component(func(ctx *Context) Element { runsB++; return Textf("b=%d", b.Get()) }))
	}, 400, 300, 1)
	if runsRoot != 1 || runsA != 1 || runsB != 1 {
		t.Fatalf("initial renders root=%d a=%d b=%d", runsRoot, runsA, runsB)
	}
	a.Set(5)
	if !rt.NeedsFrame() {
		t.Fatal("setting a signal did not request a frame")
	}
	rt.Frame(time.Now())
	if runsRoot != 1 || runsA != 2 || runsB != 1 {
		t.Fatalf("after a.Set: root=%d a=%d b=%d; want only a re-rendered", runsRoot, runsA, runsB)
	}
	rt.Frame(time.Now())
	if runsA != 2 || rt.NeedsFrame() {
		t.Fatal("idle frame did work")
	}
}

func TestClickDispatchAndHover(t *testing.T) {
	clicks, outer := 0, 0
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).OnClick(func() { outer++ }).
			Child(Div().Size(Px(100), Px(40)).OnClick(func() { clicks++ }).
				Hover(Style{Background: Hex("#ff0000")}))
	}, 400, 300, 1)
	rt.PointerMove(50, 30, 0)
	rt.Frame(time.Now())
	if rt.Cursor() != input.CursorPointer {
		t.Errorf("cursor over clickable = %v, want pointer", rt.Cursor())
	}
	inner := rt.tree.children[0]
	if inner.style.Background != Hex("#ff0000") {
		t.Error("hover style not applied")
	}
	rt.PointerDown(50, 30, input.ButtonLeft, 1, 0)
	rt.PointerUp(50, 30, input.ButtonLeft, 0)
	if clicks != 1 || outer != 0 {
		t.Errorf("clicks inner=%d outer=%d; want the deepest handler only", clicks, outer)
	}
	// Press inside, release outside: no click.
	rt.PointerDown(50, 30, input.ButtonLeft, 1, 0)
	rt.PointerUp(300, 250, input.ButtonLeft, 0)
	if clicks != 1 {
		t.Error("release outside the element still clicked")
	}
}

func TestScrollClampsToContent(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div().Height(Px(100)).ScrollY()
		for i := 0; i < 10; i++ {
			list.Child(Div().Height(Px(50)).Shrink(0))
		}
		return list
	}, 200, 100, 1)
	rt.Scroll(50, 50, 0, 10000, true, 0)
	rt.Frame(time.Now())
	st := rt.tree.st
	if st == nil || st.scrollY != 400 {
		t.Fatalf("scrollY = %v, want 400 (500 content - 100 viewport)", st.scrollY)
	}
	if got := rt.tree.children[9].abs.Y; got != 50 {
		t.Errorf("last row painted at y=%v, want 50", got)
	}
}

func TestScaleProducesPhysicalPixels(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(Div().Size(Px(100), Px(40)).Background(Hex("#ffffff")).Radius(4))
	}, 400, 300, 1.5)
	sc := &rt.scene
	if len(sc.Quads) != 1 {
		t.Fatalf("got %d quads, want 1", len(sc.Quads))
	}
	q := sc.Quads[0]
	if q.Bounds.X != 15 || q.Bounds.Y != 15 || q.Bounds.W != 150 || q.Bounds.H != 60 || q.Radii.TL != 6 {
		t.Errorf("quad at scale 1.5 = %+v radii %v; want 15,15 150x60 radius 6", q.Bounds, q.Radii)
	}
}

func TestTextInputEditing(t *testing.T) {
	v := NewSignal("")
	submitted := ""
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Child(TextInput(v).OnSubmit(func(s string) { submitted = s }))
	}, 400, 100, 1)
	rt.PointerDown(50, 25, input.ButtonLeft, 1, 0)
	rt.PointerUp(50, 25, input.ButtonLeft, 0)
	rt.Frame(time.Now())
	for _, r := range "helo" {
		rt.TextInput(string(r))
	}
	rt.KeyDown(input.KeyLeft, 0, false)
	rt.TextInput("l")
	rt.KeyDown(input.KeyEnter, 0, false)
	if v.Peek() != "hello" || submitted != "hello" {
		t.Errorf("value %q submitted %q, want hello", v.Peek(), submitted)
	}
}

func BenchmarkFrameRebuild(b *testing.B) {
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div().Gap(8).Padding(16).ScrollY()
		for i := 0; i < 50; i++ {
			list.Child(Div().Row().Gap(12).Padding(12).Radius(8).Background(Hex("#18181b")).
				Hover(Style{Background: Hex("#27272a")}).
				Child(Div().Size(Px(32), Px(32)).Radius(8).Background(Hex("#6366f1"))).
				Child(Div().Grow(1).Gap(4).
					Child(Textf("Row %d", i).Semibold()).
					Child(Text("A description that is long enough to wrap at narrow widths.").FontSize(13))))
		}
		return list
	}, 1000, 800, 1)
	b.ReportAllocs()
	for b.Loop() {
		rt.InvalidateAll()
		rt.Frame(time.Now())
	}
	b.ReportMetric(float64(rt.Stats().Nodes), "nodes")
}

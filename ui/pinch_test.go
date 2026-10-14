package ui

import "testing"

// pinchTree is a window with a 20 px margin, a 100 by 40 px element inside it
// at (20, 20) with a handler that counts the gestures it receives, and an outer
// element around it with its own counter. The center (70, 40) is inside the
// inner element.
func pinchTree(inner, outer func(*PinchEvent)) func(*Context) Element {
	return func(ctx *Context) Element {
		return Div().Padding(20).OnPinch(outer).
			Child(Div().Key("inner").Size(Px(100), Px(40)).OnPinch(inner))
	}
}

// TestPinchDeepestHandlerWins checks that the element under the center gets the
// gesture, and that the element around it gets it only when the inner one does
// not stop it.
func TestPinchDeepestHandlerWins(t *testing.T) {
	var inner, outer int
	stop := false
	rt := newTestRuntime(pinchTree(func(e *PinchEvent) {
		inner++
		if stop {
			e.StopPropagation()
		}
	}, func(e *PinchEvent) { outer++ }), 300, 200, 1)
	frame(rt)

	rt.Pinch(70, 40, GestureUpdate, 1.05, 0, 0, false, 0)
	if inner != 1 || outer != 1 {
		t.Errorf("unstopped: inner %d, outer %d; want 1 and 1 (the outer element hears what the inner does not stop)", inner, outer)
	}

	stop = true
	rt.Pinch(70, 40, GestureUpdate, 1.05, 0, 0, false, 0)
	if inner != 2 || outer != 1 {
		t.Errorf("stopped: inner %d, outer %d; want 2 and 1 (StopPropagation keeps the outer element from hearing it)", inner, outer)
	}
}

// TestPinchLocalCoordinates checks that the handler sees the center relative to
// the element it runs on, and the change of the gesture.
func TestPinchLocalCoordinates(t *testing.T) {
	var got PinchEvent
	rt := newTestRuntime(pinchTree(func(e *PinchEvent) { got = *e }, func(*PinchEvent) {}), 300, 200, 1)
	frame(rt)
	rt.Pinch(70, 40, GestureUpdate, 1.5, 3, -2, false, 0)
	if got.LocalX != 50 || got.LocalY != 20 {
		t.Errorf("local center = (%v, %v), want (50, 20)", got.LocalX, got.LocalY)
	}
	if got.X != 70 || got.Y != 40 || got.Scale != 1.5 || got.PanX != 3 || got.PanY != -2 || got.Phase != GestureUpdate {
		t.Errorf("event = %+v; the window center, scale, pan and phase should arrive unchanged", got)
	}
}

// TestPinchZoomsWhenUnhandled checks that a pinch no element handles zooms the
// window in the same steps as Ctrl+= and Ctrl+-.
func TestPinchZoomsWhenUnhandled(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element { return Div().Padding(20) }, 300, 200, 1)
	frame(rt)

	rt.Pinch(50, 50, GestureBegin, 1, 0, 0, false, 0)
	rt.Pinch(50, 50, GestureUpdate, 1.25, 0, 0, false, 0)
	if rt.Zoom() != 1.25 {
		t.Errorf("a pinch of 1.25 zoomed to %v, want 1.25 (two steps)", rt.Zoom())
	}
	rt.Pinch(50, 50, GestureEnd, 1, 0, 0, false, 0)

	rt.Pinch(50, 50, GestureBegin, 1, 0, 0, false, 0)
	rt.Pinch(50, 50, GestureUpdate, 0.8, 0, 0, false, 0)
	if rt.Zoom() != 1 {
		t.Errorf("a pinch of 0.8 from 1.25 zoomed to %v, want 1 (two steps out)", rt.Zoom())
	}
	rt.Pinch(50, 50, GestureEnd, 1, 0, 0, false, 0)
}

// TestPinchAccumulatesAcrossUpdates checks that a pinch too small for one step
// adds up over its updates, and that a new gesture starts from nothing.
func TestPinchAccumulatesAcrossUpdates(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element { return Div().Padding(20) }, 300, 200, 1)
	frame(rt)

	rt.Pinch(50, 50, GestureBegin, 1, 0, 0, false, 0)
	rt.Pinch(50, 50, GestureUpdate, 1.05, 0, 0, false, 0)
	rt.Pinch(50, 50, GestureUpdate, 1.05, 0, 0, false, 0) // 1.1025 in all: one step
	if rt.Zoom() != 1.1 {
		t.Fatalf("two small updates zoomed to %v, want 1.1", rt.Zoom())
	}
	rt.Pinch(50, 50, GestureEnd, 1, 0, 0, false, 0)

	rt.Pinch(50, 50, GestureBegin, 1, 0, 0, false, 0)
	rt.Pinch(50, 50, GestureUpdate, 1.05, 0, 0, false, 0)
	rt.Pinch(50, 50, GestureEnd, 1, 0, 0, false, 0)
	if rt.Zoom() != 1.1 {
		t.Errorf("a new gesture carried a part step over: zoom %v, want 1.1", rt.Zoom())
	}
}

// TestPinchNoZoomWithoutShortcuts checks that an unhandled pinch leaves the zoom
// alone when the zoom shortcuts are off, as Ctrl+wheel does.
func TestPinchNoZoomWithoutShortcuts(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element { return Div().Padding(20) }, 300, 200, 1)
	frame(rt)
	rt.SetZoomShortcuts(false)
	rt.Pinch(50, 50, GestureBegin, 1, 0, 0, false, 0)
	rt.Pinch(50, 50, GestureUpdate, 2, 0, 0, false, 0)
	rt.Pinch(50, 50, GestureEnd, 1, 0, 0, false, 0)
	if rt.Zoom() != 1 {
		t.Errorf("zoom %v with the shortcuts off, want 1", rt.Zoom())
	}
}

// TestPinchReducedMotionLeavesOutInertia checks that under reduced motion the
// change of an inertia event is not applied, while the gesture still reaches the
// handler, and that an ordinary change is applied.
func TestPinchReducedMotionLeavesOutInertia(t *testing.T) {
	var phases []GesturePhase
	var scales []float32
	rt := newTestRuntime(pinchTree(func(e *PinchEvent) {
		phases = append(phases, e.Phase)
		scales = append(scales, e.Scale)
	}, func(*PinchEvent) {}), 300, 200, 1)
	frame(rt)
	rt.SetSystemSettings(SystemSettings{ReducedMotion: true})

	rt.Pinch(70, 40, GestureUpdate, 2, 0, 0, true, 0) // inertia: left out
	rt.Pinch(70, 40, GestureEnd, 1, 0, 0, true, 0)
	if len(scales) != 2 || scales[0] != 1 || phases[1] != GestureEnd {
		t.Errorf("inertia under reduced motion: scales %v, phases %v; want the change left out and the end delivered", scales, phases)
	}

	rt.Pinch(70, 40, GestureUpdate, 2, 0, 0, false, 0) // not inertia: applied
	if scales[2] != 2 {
		t.Errorf("a change without inertia under reduced motion: scale %v, want 2", scales[2])
	}
}

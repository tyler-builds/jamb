package ui

import "testing"

// sharedOf reports whether the shared state of the layout is still held.
func sharedOf(layout *Signal[DockLayout]) bool {
	dockShares.Lock()
	defer dockShares.Unlock()
	_, ok := dockShares.byLayout[layout]
	return ok
}

// TestHiddenDockReleasesItsPanels checks that a dock whose elements are all unmounted disposes
// its panels once, and drops its shared state, and that a dock shown again starts with new panels.
func TestHiddenDockReleasesItsPanels(t *testing.T) {
	disposed := 0
	text, label := NewSignal(""), NewSignal("first")
	sig := NewSignal(dockSplitLayout())
	show := NewSignal(true)
	r := newAccessRig(func(ctx *Context) Element {
		return Component(func(ctx *Context) Element {
			c := Div().Column().Size(Full, Full)
			if show.Get() {
				c.Child(Dock(sig, statefulPanels(&disposed, text, label)).Key("dock"))
			}
			return c
		})
	})
	r.frame()
	r.clickKey(t, "notes-bump")
	if !sharedOf(sig) {
		t.Fatal("the shared state is missing while the dock is shown")
	}

	show.Set(false)
	r.frame()
	r.frame()
	if disposed != 1 {
		t.Errorf("OnDispose ran %d times after the dock was hidden, want 1", disposed)
	}
	if sharedOf(sig) {
		t.Error("the shared state of a hidden dock is still held")
	}

	show.Set(true)
	r.frame()
	r.frame()
	if disposed != 1 {
		t.Errorf("OnDispose ran %d times after the dock was shown again, want still 1", disposed)
	}
	if !sharedOf(sig) {
		t.Error("the shared state is missing after the dock was shown again")
	}
}

// TestRecreatedDockInOneFrameKeepsItsState checks that a dock whose element is unmounted and
// mounted again within one frame keeps its panels and state, since the release waits for the
// frame to end.
func TestRecreatedDockInOneFrameKeepsItsState(t *testing.T) {
	disposed := 0
	text, label := NewSignal(""), NewSignal("first")
	sig := NewSignal(dockSplitLayout())
	tick := NewSignal(0)
	r := newAccessRig(func(ctx *Context) Element {
		return Component(func(ctx *Context) Element {
			_ = tick.Get()
			// A new key each time: the old dock is unmounted, and the new one mounted, in one build.
			key := "dock"
			if tick.Peek()%2 == 1 {
				key = "dock2"
			}
			return Div().Column().Size(Full, Full).Child(Dock(sig, statefulPanels(&disposed, text, label)).Key(key))
		})
	})
	r.frame()
	r.clickKey(t, "notes-bump")
	r.clickKey(t, "notes-bump")
	for i := 1; i <= 3; i++ {
		tick.Set(i)
		r.frame()
		r.frame()
		if disposed != 0 {
			t.Fatalf("tick %d: OnDispose ran %d times, want none", i, disposed)
		}
		if got := keyText(t, r.rt, "notes-count"); got != "count=2" {
			t.Fatalf("tick %d: %q, want the counter kept at 2", i, got)
		}
	}
}

// TestFloatingWindowKeepsTheSharedStateWhileTheDockIsShown checks that closing a floating window
// does not release the state while the main dock is shown, and that closing everything releases
// it: the panels are disposed once and the state is dropped.
func TestFloatingWindowKeepsTheSharedStateWhileTheDockIsShown(t *testing.T) {
	disposed := 0
	text, label := NewSignal(""), NewSignal("first")
	sig := NewSignal(dockSplitLayout())
	show := NewSignal(true)
	var opened []DockWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		return &dockTestWindow{x: w.X, y: w.Y, w: w.Width, h: w.Height}
	}
	r := newAccessRig(func(ctx *Context) Element {
		return Component(func(ctx *Context) Element {
			c := Div().Column().Size(Full, Full)
			if show.Get() {
				c.Child(Dock(sig, statefulPanels(&disposed, text, label)).Key("dock").Floating(open))
			}
			return c
		})
	})
	r.frame()
	sig.Set(sig.Peek().Float("log", 10, 10, 300, 200))
	r.frame()
	r.frame()
	if len(opened) != 1 {
		t.Fatalf("%d windows opened, want 1", len(opened))
	}

	opened[0].OnClosed()
	r.frame()
	if !sharedOf(sig) {
		t.Fatal("closing a floating window released the state of a dock that is shown")
	}

	sig.Set(sig.Peek().Float("log", 10, 10, 300, 200))
	r.frame()
	r.frame()
	show.Set(false)
	r.frame()
	r.frame()
	// Hiding the dock disposes the panels in its own groups, as the component's disposal does.
	// The floating panel is in its window, so the state stays until the window closes.
	if disposed != 1 {
		t.Fatalf("OnDispose ran %d times after the dock was hidden, want 1", disposed)
	}
	if !sharedOf(sig) {
		t.Fatal("hiding the dock released the state while its floating window is open")
	}

	opened[len(opened)-1].OnClosed()
	r.frame()
	r.frame()
	if sharedOf(sig) {
		t.Error("the shared state is held after every dock and window closed")
	}
	if disposed != 1 {
		t.Errorf("OnDispose ran %d times after every dock and window closed, want 1", disposed)
	}
}

// TestReleasingTheSharedStateFlushesTheDebouncedOnChange checks that a layout change saved late,
// such as a floating window moved, is saved when the state is released, and that its timer stops.
func TestReleasingTheSharedStateFlushesTheDebouncedOnChange(t *testing.T) {
	disposed := 0
	text, label := NewSignal(""), NewSignal("first")
	sig := NewSignal(dockSplitLayout())
	show := NewSignal(true)
	var saved []DockLayout
	var opened []DockWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		return &dockTestWindow{x: w.X, y: w.Y, w: w.Width, h: w.Height}
	}
	r := newAccessRig(func(ctx *Context) Element {
		return Component(func(ctx *Context) Element {
			c := Div().Column().Size(Full, Full)
			if show.Get() {
				c.Child(Dock(sig, statefulPanels(&disposed, text, label)).Key("dock").Floating(open).
					OnChange(func(l DockLayout) { saved = append(saved, l) }))
			}
			return c
		})
	})
	r.frame()
	sig.Set(sig.Peek().Float("log", 10, 10, 300, 200))
	r.frame()
	r.frame()
	savedBefore := len(saved)

	opened[0].OnGeometry(120, 130, 340, 260) // a late change: it waits for the layout to be still
	r.frame()
	if len(saved) != savedBefore {
		t.Fatal("the late change was saved at once")
	}
	if len(r.rt.timers) == 0 {
		t.Fatal("no timer waits for the late change")
	}

	// Docking the group back closes its window without a save of its own, so the late change
	// is still waiting when the dock is hidden.
	sig.Set(sig.Peek().Dock("log"))
	r.frame()
	opened[0].OnClosed()
	r.frame()
	if len(saved) != savedBefore {
		t.Fatal("the docking back saved the late change, so the release is not tested")
	}

	show.Set(false)
	r.frame()
	r.frame()

	if len(saved) == savedBefore {
		t.Fatal("the late change was not saved when the state was released")
	}
	if len(r.rt.timers) != 0 {
		t.Errorf("%d timers are still scheduled after the release, want none", len(r.rt.timers))
	}
	if sharedOf(sig) {
		t.Error("the shared state is held after release")
	}
	if disposed != 1 {
		t.Errorf("OnDispose ran %d times, want 1", disposed)
	}
}

package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// quietPast moves the rig's clock past the quiet period of a save and runs a frame.
func quietPast(r *accessRig, d time.Duration) {
	r.now = r.now.Add(d)
	r.frame()
}

// TestDockSplitterStepsSaveOnceTheLayoutIsStill checks that the keyboard moves of a divider
// change the layout at once, but OnChange runs once, when the layout has been still for the
// quiet period.
func TestDockSplitterStepsSaveOnceTheLayoutIsStill(t *testing.T) {
	saves := 0
	sig := NewSignal(dockSplitLayout())
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").
			OnChange(func(DockLayout) { saves++ }))
	})
	r.frame()
	r.focusAccess(t, access.RoleSeparator, "Resize panes")
	before := sig.Peek().Root.Sizes[0]
	r.press(input.KeyRight, 0)
	r.press(input.KeyRight, 0)
	if sig.Peek().Root.Sizes[0] == before {
		t.Fatalf("the divider did not move the layout")
	}
	if saves != 0 {
		t.Fatalf("OnChange ran %d times during the steps, want none yet", saves)
	}
	quietPast(r, 300*time.Millisecond)
	if saves != 0 {
		t.Fatalf("OnChange ran %d times before the quiet period ended, want none", saves)
	}
	quietPast(r, 300*time.Millisecond)
	if saves != 1 {
		t.Fatalf("OnChange ran %d times after the quiet period, want 1", saves)
	}
	quietPast(r, 2*time.Second)
	if saves != 1 {
		t.Errorf("OnChange ran %d times in all, want one for the run of steps", saves)
	}
}

// TestDockSplitterDragSavesAtTheRelease checks that the end of a pointer drag of a divider
// saves at once, without waiting for the quiet period.
func TestDockSplitterDragSavesAtTheRelease(t *testing.T) {
	saves := 0
	sig := NewSignal(dockSplitLayout())
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").
			OnChange(func(DockLayout) { saves++ }))
	})
	r.frame()
	handles := partNodes(r.rt, "Split.Handle")
	if len(handles) == 0 {
		t.Fatalf("no divider in the dock")
	}
	dragDivider(r, handles[0], 40, 0)
	if saves != 1 {
		t.Errorf("OnChange ran %d times at the end of the drag, want 1 at once", saves)
	}
	quietPast(r, 2*time.Second)
	if saves != 1 {
		t.Errorf("OnChange ran %d times in all, want 1", saves)
	}
}

// TestDockWindowMovesSaveOnceTheWindowIsStill checks that a window's moves and resizes take
// the layout's place at once, and that OnChange runs once after the window has been still,
// and at once when the window closes.
func TestDockWindowMovesSaveOnceTheWindowIsStill(t *testing.T) {
	var opened []DockWindow
	var wins []*dockTestWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		f := &dockTestWindow{x: w.X, y: w.Y, w: w.Width, h: w.Height}
		wins = append(wins, f)
		return f
	}
	saves := 0
	sig := NewSignal(DockFloatPanel(dockSplitLayout(), "tree", 30, 40, 300, 200))
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").
			Floating(open).OnChange(func(DockLayout) { saves++ }))
	})
	r.frame()
	r.frame()
	if len(opened) != 1 {
		t.Fatalf("%d windows opened, want one", len(opened))
	}
	w := opened[0]
	w.OnGeometry(50, 60, 320, 210)
	quietPast(r, 100*time.Millisecond)
	w.OnGeometry(70, 80, 340, 220)
	quietPast(r, 100*time.Millisecond)
	if f := floatByID(sig.Peek(), "float-1"); f == nil || f.X != 70 || f.Y != 80 || f.Width != 340 {
		t.Fatalf("the layout did not take the window's place at once: %s", layoutText(sig.Peek()))
	}
	if saves != 0 {
		t.Fatalf("OnChange ran %d times while the window moved, want none", saves)
	}
	quietPast(r, 600*time.Millisecond)
	if saves != 1 {
		t.Fatalf("OnChange ran %d times after the window was still, want 1", saves)
	}

	// Closing the window saves at once, with the place it closed at.
	w.OnGeometry(90, 100, 360, 230)
	wins[0].x, wins[0].y, wins[0].w, wins[0].h = 110, 120, 380, 240
	w.OnClose()
	if saves != 2 {
		t.Fatalf("OnChange ran %d times after the window closed, want 2", saves)
	}
	if f := floatByID(sig.Peek(), "float-1"); f == nil || f.X != 110 || f.Height != 240 {
		t.Errorf("the window's place at its close was not recorded: %s", layoutText(sig.Peek()))
	}
}

// TestDockPendingSaveRunsWhenTheWindowFlushes checks that a save that is waiting runs when
// the window's timers are flushed, as they are when the window closes.
func TestDockPendingSaveRunsWhenTheWindowFlushes(t *testing.T) {
	saves := 0
	sig := NewSignal(dockSplitLayout())
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").
			OnChange(func(DockLayout) { saves++ }))
	})
	r.frame()
	r.focusAccess(t, access.RoleSeparator, "Resize panes")
	r.press(input.KeyRight, 0)
	if saves != 0 {
		t.Fatalf("OnChange ran %d times before the flush, want none", saves)
	}
	(*runtimeHost)(r.rt).FlushTimers()
	if saves != 1 {
		t.Errorf("OnChange ran %d times after the flush, want 1", saves)
	}
	quietPast(r, 2*time.Second)
	if saves != 1 {
		t.Errorf("OnChange ran again after the flush: %d times", saves)
	}
}

// TestRuntimeAtRunsTheTimerAtItsFrame checks the timers that the dock saves with: a timer
// runs at the first frame at or after its time, once, and a canceled one never runs.
func TestRuntimeAtRunsTheTimerAtItsFrame(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return Div() })
	ran, canceled := 0, 0
	start := r.now
	r.rt.at(start.Add(100*time.Millisecond), func() { ran++ })
	tm := r.rt.at(start.Add(100*time.Millisecond), func() { canceled++ })
	tm.cancel()
	r.now = start.Add(50 * time.Millisecond)
	r.frame()
	if ran != 0 {
		t.Fatalf("the timer ran %d times before its time", ran)
	}
	r.now = start.Add(150 * time.Millisecond)
	r.frame()
	r.frame()
	if ran != 1 || canceled != 0 {
		t.Errorf("ran %d times and the canceled one %d times, want 1 and 0", ran, canceled)
	}
}

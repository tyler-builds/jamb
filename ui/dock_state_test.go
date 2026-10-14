package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// statefulPanels returns the panels of the state tests. The notes panel keeps what a panel
// may hold: a component with a Local counter that its button bumps, a text field bound to
// text, a scroll container, a label that shows label, and a component whose disposal is
// counted in disposed. The other panels are testDockPanels.
func statefulPanels(disposed *int, text, label *Signal[string]) map[string]DockPanel {
	panels := testDockPanels()
	panels["notes"] = DockPanel{Title: "Notes", Render: func() Element {
		return Component(func(ctx *Context) Element {
			count := Local(ctx, "count", 0)
			if seen := Local(ctx, "seen", false); !seen.Peek() {
				seen.Set(true)
				ctx.OnDispose(func() { *disposed++ })
			}
			return Div().Column().
				Child(Div().Key("notes-count").Child(Textf("count=%d", count.Get()))).
				Child(Div().Key("notes-bump").Child(Button("Bump").OnClick(func() { count.Set(count.Peek() + 1) }))).
				Child(Div().Key("notes-text").Child(TextInput(text))).
				Child(Div().Key("notes-label").Child(Text(label.Get()))).
				Child(Div().Key("notes-scroll").Height(Px(40)).ScrollY().Child(Div().Key("notes-tall").Height(Px(400))))
		})
	}}
	return panels
}

// treeNode returns the node with the key in rt's tree, or fails the test.
func treeNode(t *testing.T, rt *Runtime, key string) *node {
	t.Helper()
	n := rt.findKey(rt.tree, key)
	if n == nil {
		t.Fatalf("no element with key %q in the tree", key)
	}
	return n
}

// keyText returns the text under the element with the key in rt's tree.
func keyText(t *testing.T, rt *Runtime, key string) string {
	t.Helper()
	return nodeText(treeNode(t, rt, key))
}

// scrollOffset returns the vertical scroll offset of the scroll container with the key.
func scrollOffset(t *testing.T, rt *Runtime, key string) float32 {
	t.Helper()
	n := treeNode(t, rt, key)
	if n.st == nil {
		t.Fatalf("scroll container %q has no state", key)
	}
	return n.st.scrollY
}

// setScrollOffset moves the scroll container with the key to y.
func setScrollOffset(t *testing.T, rt *Runtime, key string, y float32) {
	t.Helper()
	n := treeNode(t, rt, key)
	if n.st == nil {
		t.Fatalf("scroll container %q has no state", key)
	}
	n.st.scrollY, n.st.targetY = y, y
}

// clickTree presses and releases the left button on the focusable element under the key,
// as the tree has it now. The runtime's index by ID can keep an element that an earlier build
// made, so the form helpers, which read that index, are not used here.
func clickTree(t *testing.T, r *accessRig, key string) {
	t.Helper()
	n := r.rt.findKey(r.rt.tree, key)
	if n == nil {
		t.Fatalf("no element with key %q", key)
	}
	if f := firstFocusable(n); f != nil {
		n = f
	}
	rc := r.rt.nodeRect(n)
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
}

// focusedIn reports whether the focus of rt is on the focusable element under the key.
func focusedIn(rt *Runtime, key string) bool {
	n := rt.findKey(rt.tree, key)
	if n == nil {
		return false
	}
	if f := firstFocusable(n); f != nil {
		n = f
	}
	return rt.focusID == n.id
}

// dockStateCheck is what a panel must still hold after a move: the count, the typed text, the
// label, the scroll offset, and no disposal.
type dockStateCheck struct {
	t        *testing.T
	disposed *int
	text     *Signal[string]
}

func (c dockStateCheck) verify(step string, rt *Runtime) {
	c.t.Helper()
	if *c.disposed != 0 {
		c.t.Fatalf("%s: OnDispose ran %d times, want none", step, *c.disposed)
	}
	if got := keyText(c.t, rt, "notes-count"); got != "count=2" {
		c.t.Errorf("%s: %q, want the counter kept at 2", step, got)
	}
	if got := c.text.Peek(); got != "hello" {
		c.t.Errorf("%s: typed text %q, want hello", step, got)
	}
	if got := scrollOffset(c.t, rt, "notes-scroll"); got != 30 {
		c.t.Errorf("%s: scroll offset %v, want 30", step, got)
	}
}

// TestPanelKeepsItsStateAcrossMoves checks that a panel keeps its component state, its
// widgets' state, its focus and its disposal count through every dock operation that moves
// it: between groups, split, edge drop, floating, docking back, a layout reset, and a drag of
// its tab, and that closing it disposes it once.
func TestPanelKeepsItsStateAcrossMoves(t *testing.T) {
	disposed := 0
	text, label := NewSignal(""), NewSignal("first")
	sig := NewSignal(dockSplitLayout())
	var opened []DockWindow
	var wins []*dockTestWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		f := &dockTestWindow{x: w.X, y: w.Y, w: w.Width, h: w.Height}
		wins = append(wins, f)
		return f
	}
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, statefulPanels(&disposed, text, label)).
			Key("dock").Floating(open))
	})
	r.frame()

	// Build the state: two bumps, typed text, a scroll offset, and the focus in the text field.
	clickTree(t, r, "notes-bump")
	clickTree(t, r, "notes-bump")
	clickTree(t, r, "notes-text")
	r.typeText("hello")
	setScrollOffset(t, r.rt, "notes-scroll", 30)
	check := dockStateCheck{t: t, disposed: &disposed, text: text}
	check.verify("before the moves", r.rt)

	sig.Set(DockMove(sig.Peek(), "notes", "tree", DockCenter))
	r.frame()
	check.verify("moved to another group", r.rt)
	if !focusedIn(r.rt, "notes-text") {
		t.Errorf("the text field lost the focus when its panel moved")
	}

	sig.Set(DockMove(sig.Peek(), "notes", "tree", DockRight))
	r.frame()
	check.verify("split beside a group", r.rt)

	sig.Set(DockMove(sig.Peek(), "notes", "", DockBottom))
	r.frame()
	check.verify("dropped on the bottom edge", r.rt)

	// A real drag: the tab is released on the center of another group.
	dragHover(t, r, "dock/tab:notes", "dock/group:tree")
	x, y := center(t, r.rt, "dock/group:tree")
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
	check.verify("dragged onto a group", r.rt)

	// Floating: the panel moves into a window of its own, which renders it in its own runtime.
	sig.Set(DockFloatPanel(sig.Peek(), "notes", 30, 40, 300, 200))
	r.frame()
	r.frame()
	if len(opened) != 1 {
		t.Fatalf("%d windows opened, want one for the floated panel", len(opened))
	}
	wr := newTestRuntime(opened[0].Root, 300, 200, 1)
	check.verify("floated into a window", wr)

	// A signal changed after the move renders the panel in its new window.
	label.Set("second")
	wr.Frame(wr.now().Add(time.Millisecond * 16))
	if got := keyText(t, wr, "notes-label"); got != "second" {
		t.Errorf("label in the floating window %q, want second", got)
	}

	// Docking back closes the window, and the panel returns with its state.
	sig.Set(DockBack(sig.Peek(), "notes"))
	r.frame()
	r.frame()
	if wins[0].closed != 1 {
		t.Errorf("the window of the docked back panel closed %d times, want 1", wins[0].closed)
	}
	check.verify("docked back", r.rt)
	if got := keyText(t, r.rt, "notes-label"); got != "second" {
		t.Errorf("label %q after docking back, want second", got)
	}

	// A layout reset that keeps the panel: it moves back to its default place.
	sig.Set(DockReset(dockSplitLayout()))
	r.frame()
	check.verify("layout reset", r.rt)

	// Closing the panel disposes it, once.
	sig.Set(DockClose(sig.Peek(), "notes"))
	r.frame()
	r.frame()
	if disposed != 1 {
		t.Errorf("OnDispose ran %d times after the panel closed, want 1", disposed)
	}
}

// TestRecreatedDockKeepsItsPanels checks that a dock its parent builds again, for the same
// layout, takes over the panels that the earlier dock showed, so that they keep their state
// and are not disposed. The demo's dock is built this way, inside a component that re-renders.
func TestRecreatedDockKeepsItsPanels(t *testing.T) {
	disposed := 0
	text, label := NewSignal(""), NewSignal("first")
	sig := NewSignal(dockSplitLayout())
	tick := NewSignal(0)
	r := newAccessRig(func(ctx *Context) Element {
		return Component(func(ctx *Context) Element {
			_ = tick.Get()
			return Div().Column().Size(Full, Full).Child(Dock(sig, statefulPanels(&disposed, text, label)).Key("dock"))
		})
	})
	r.frame()
	clickTree(t, r, "notes-bump")
	clickTree(t, r, "notes-bump")
	clickTree(t, r, "notes-text")
	r.typeText("hello")
	setScrollOffset(t, r.rt, "notes-scroll", 30)
	check := dockStateCheck{t: t, disposed: &disposed, text: text}

	tick.Set(1)
	r.frame()
	r.frame()
	check.verify("the dock was built again", r.rt)

	sig.Set(DockClose(sig.Peek(), "notes"))
	r.frame()
	r.frame()
	if disposed != 1 {
		t.Errorf("OnDispose ran %d times after the panel closed, want 1", disposed)
	}
}

// TestPanelOpenedInAnotherRuntimeKeepsItsFocusAndInput checks that the focus of a panel that
// floats while focused is taken by its window, and that typing there goes to the field.
func TestPanelOpenedInAnotherRuntimeKeepsItsFocusAndInput(t *testing.T) {
	disposed := 0
	text, label := NewSignal(""), NewSignal("a")
	sig := NewSignal(dockSplitLayout())
	var opened []DockWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		return &dockTestWindow{x: w.X, y: w.Y, w: w.Width, h: w.Height}
	}
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, statefulPanels(&disposed, text, label)).
			Key("dock").Floating(open))
	})
	r.frame()
	clickTree(t, r, "notes-text")
	r.typeText("ab")
	sig.Set(DockFloatPanel(sig.Peek(), "notes", 30, 40, 300, 200))
	r.frame()
	r.frame()
	wr := newTestRuntime(opened[0].Root, 300, 200, 1)
	if focusedIn(r.rt, "notes-text") {
		t.Errorf("the main window still focuses the panel's field after it floated")
	}
	if !focusedIn(wr, "notes-text") {
		t.Errorf("the floating window does not have the focus the panel had")
	}
	wr.TextInput("c")
	wr.Frame(wr.now().Add(time.Millisecond * 16))
	if got := text.Peek(); got != "abc" {
		t.Errorf("typed text %q in the floating window, want abc", got)
	}
	if disposed != 0 {
		t.Errorf("OnDispose ran %d times, want none", disposed)
	}
}

// TestDockDropOutsideFloatsThePanelAtThePointer checks that a tab released where no dock
// takes it floats its panel in a window with its top-left corner by the pointer, so the tab
// ends under it.
func TestDockDropOutsideFloatsThePanelAtThePointer(t *testing.T) {
	var opened []DockWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		return &dockTestWindow{x: w.X, y: w.Y, w: w.Width, h: w.Height}
	}
	sig := NewSignal(dockSplitLayout())
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").Floating(open))
	})
	r.frame()
	dragHover(t, r, "dock/tab:tree", "dock/group:notes")
	// The pointer is released past the window's right edge, where no group is.
	r.rt.PointerUp(700, 50, input.ButtonLeft, 0)
	r.frame()
	r.frame()
	if len(opened) != 1 {
		t.Fatalf("%d windows opened, want one for the released panel", len(opened))
	}
	w := opened[0]
	if w.Title != "Explorer" || w.X != 700-dockGrabX || w.Y != 50-dockGrabY || w.Width != dockFloatWidth || w.Height != dockFloatHeight {
		t.Errorf("window %+v, want Explorer at the pointer, offset by the grab", w)
	}
	if !floatingHas(sig.Peek(), "tree") {
		t.Errorf("the released panel does not float: %s", layoutText(sig.Peek()))
	}
}

// TestDockDropOutsideOnACancelledDragDoesNotFloat checks that Escape ends a drag without
// floating its panel.
func TestDockDropOutsideOnACancelledDragDoesNotFloat(t *testing.T) {
	var opened []DockWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		return &dockTestWindow{}
	}
	sig := NewSignal(dockSplitLayout())
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").Floating(open))
	})
	r.frame()
	dragHover(t, r, "dock/tab:tree", "dock/group:notes")
	r.rt.KeyDown(input.KeyEscape, 0, false)
	r.frame()
	if len(opened) != 0 || floatingHas(sig.Peek(), "tree") {
		t.Errorf("a cancelled drag floated the panel: %s", layoutText(sig.Peek()))
	}
}

// TestDockDropOnAGroupOfAnotherWindowMovesThePanelThere checks that a tab released over a
// group of a floating window moves its panel into that group.
func TestDockDropOnAGroupOfAnotherWindowMovesThePanelThere(t *testing.T) {
	var opened []DockWindow
	var wins []*dockTestWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		f := &dockTestWindow{x: w.X, y: w.Y, w: w.Width, h: w.Height}
		wins = append(wins, f)
		return f
	}
	// The floating window lies beyond the main window's right edge, which is 500 pixels wide.
	sig := NewSignal(DockFloatPanel(dockSplitLayout(), "log", 600, 40, 300, 200))
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").Floating(open))
	})
	r.frame()
	r.frame()
	if len(opened) != 1 {
		t.Fatalf("%d windows opened, want one", len(opened))
	}
	wr := newTestRuntime(opened[0].Root, 300, 200, 1)
	wr.SetWindow(wins[0]) // the window's place on the screen, which a release is matched against

	// The tab of notes is dragged out of the main window and released over the floating
	// window's group, at its screen position.
	dragHover(t, r, "dock/tab:notes", "dock/group:tree")
	r.rt.PointerUp(650, 60, input.ButtonLeft, 0)
	r.frame()
	wr.Frame(wr.now().Add(time.Millisecond * 16))
	f := sig.Peek()
	fl := floatByID(f, "float-1")
	if fl == nil || !containsString(fl.Group.Panels, "notes") {
		t.Fatalf("the notes panel did not move into the floating group: %s", layoutText(f))
	}
	if groupIn(&f.Root, "notes") != nil {
		t.Errorf("the notes panel is still in the main dock: %s", layoutText(f))
	}
	if n := len(wins); n != 1 {
		t.Errorf("%d windows, want the one floating window", n)
	}
}

// floatingHas reports whether a floating group of l holds the panel id.
func floatingHas(l DockLayout, id string) bool { return l.floating(id) }

// containsString reports whether s holds v.
func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

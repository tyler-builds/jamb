package ui

import (
	"math"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// testDockPanels returns three panels: Notes and Output, which can be closed, and Explorer.
// Each panel's content is a text, so a test can find it.
func testDockPanels() map[string]DockPanel {
	return map[string]DockPanel{
		"notes": {Title: "Notes", Render: func() Element { return Text("Notes body") }},
		"log":   {Title: "Output", Render: func() Element { return Text("Output body") }, Closable: true},
		"tree":  {Title: "Explorer", Render: func() Element { return Text("Explorer body") }},
	}
}

// dockRig shows a dock of layout, with the panels of testDockPanels and the instance
// style st, in a window of 500 by 400 pixels. The dock has the key "dock". It returns the
// rig and the layout signal.
func dockRig(layout DockLayout, st DockStyle) (*accessRig, *Signal[DockLayout]) {
	sig := NewSignal(layout)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").Styles(st))
	})
	return r, sig
}

// dockSplitLayout is a row of the group [notes log] with the active notes, and the group
// [tree] beside it.
func dockSplitLayout() DockLayout {
	return DockLayout{Root: sized(false, []float64{0.5, 0.5}, grp("notes", "notes", "log"), grp("tree", "tree"))}
}

// accessNodes returns the nodes of the accessibility tree with the role.
func accessNodes(r *accessRig, role access.Role) []*access.Node {
	var out []*access.Node
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == role {
			out = append(out, n)
		}
		return true
	})
	return out
}

// accessNamed returns the node with the role and the name, or fails the test.
func accessNamed(t *testing.T, r *accessRig, role access.Role, name string) *access.Node {
	t.Helper()
	for _, n := range accessNodes(r, role) {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no %v named %q", role, name)
	return nil
}

// dragHover presses on the element from, drags past the threshold and moves over the
// element to, without releasing, so the drop preview is up.
func dragHover(t *testing.T, r *accessRig, from, to string) {
	t.Helper()
	x0, y0 := center(t, r.rt, from)
	x1, y1 := center(t, r.rt, to)
	r.rt.PointerMove(x0, y0, 0)
	r.frame()
	r.rt.PointerDown(x0, y0, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(x0, y0+dragThreshold, 0)
	r.frame()
	r.rt.PointerMove(x1, y1, 0)
	r.frame()
}

func TestDockRendersGroupsSplitsAndTabs(t *testing.T) {
	r, _ := dockRig(dockSplitLayout(), DockStyle{})
	if lists := accessNodes(r, access.RoleTabList); len(lists) != 2 {
		t.Errorf("%d tab lists, want one per group", len(lists))
	}
	tabs := accessNodes(r, access.RoleTab)
	if len(tabs) != 3 {
		t.Errorf("%d tabs, want one per panel in the layout", len(tabs))
	}
	if n := accessNamed(t, r, access.RoleTab, "Notes"); n.States&access.StateSelected == 0 {
		t.Errorf("the active tab Notes is not selected")
	}
	if n := accessNamed(t, r, access.RoleTab, "Output"); n.States&access.StateSelected != 0 {
		t.Errorf("the inactive tab Output is selected")
	}
	accessNamed(t, r, access.RoleTabPanel, "Explorer")
	sep := accessNodes(r, access.RoleSeparator)
	if len(sep) != 1 || sep[0].Name != "Resize panes" {
		t.Fatalf("separators %v, want one divider named Resize panes", sep)
	}
	// The panes are percentage panes, so the divider's value is the first pane's share in percent.
	if rg := sep[0].Range; rg == nil || math.Abs(rg.Value-50) > 0.01 {
		t.Errorf("divider value %v, want the share 50 percent", rg)
	}
}

// TestDockHidesUnknownPanels checks that the panels a layout names but the dock does not
// have are dropped, and that a panel the layout does not hold is shown by DockShow.
func TestDockHidesUnknownPanels(t *testing.T) {
	l := DockLayout{Root: grp("notes", "notes", "ghost")}
	r, sig := dockRig(l, DockStyle{})
	if n := len(accessNodes(r, access.RoleTab)); n != 1 {
		t.Errorf("%d tabs, want only the known panel", n)
	}
	sig.Set(DockShow(sig.Peek(), "tree"))
	r.frame()
	accessNamed(t, r, access.RoleTab, "Explorer")
}

// TestDockPanelKeepsItsStateWhileItsTabIsNotActive checks that a panel is kept built while
// another tab of its group is active, so its state survives the switch.
func TestDockPanelKeepsItsStateWhileItsTabIsNotActive(t *testing.T) {
	var count *Signal[int]
	panels := testDockPanels()
	panels["notes"] = DockPanel{Title: "Notes", Render: func() Element {
		return Component(func(ctx *Context) Element {
			count = Local(ctx, "count", 0)
			return Text("Notes body")
		})
	}}
	sig := NewSignal(DockLayout{Root: grp("notes", "notes", "log")})
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, panels).Key("dock"))
	})
	count.Set(2)
	sig.Set(DockFocus(sig.Peek(), "log"))
	r.frame()
	sig.Set(DockFocus(sig.Peek(), "notes"))
	r.frame()
	if got := count.Peek(); got != 2 {
		t.Errorf("state of Notes after switching tabs: %d, want 2", got)
	}
}

func TestDockKeyboardMovesAPanelToTheNeighbour(t *testing.T) {
	r, sig := dockRig(dockSplitLayout(), DockStyle{})
	focusNode(r, keyNode(t, r.rt, "dock/tab:log"))
	r.rt.KeyDown(input.KeyRight, input.ModCtrl|input.ModShift, false)
	r.frame()
	if got, want := dockText(sig.Peek().Root), "H{0.50 [notes*], 0.50 [tree log*]}"; got != want {
		t.Errorf("after Ctrl+Shift+Right: %s, want %s", got, want)
	}
}

func TestDockCtrlTabCyclesPanelsOfAGroup(t *testing.T) {
	r, sig := dockRig(dockSplitLayout(), DockStyle{})
	focusNode(r, keyNode(t, r.rt, "dock/tab:notes"))
	r.rt.KeyDown(input.KeyTab, input.ModCtrl, false)
	r.frame()
	if got, want := dockText(sig.Peek().Root), "H{0.50 [notes log*], 0.50 [tree*]}"; got != want {
		t.Errorf("after Ctrl+Tab: %s, want %s", got, want)
	}
}

func TestDockCtrlF6CyclesGroups(t *testing.T) {
	r, _ := dockRig(dockSplitLayout(), DockStyle{})
	focusNode(r, keyNode(t, r.rt, "dock/tab:notes"))
	r.rt.KeyDown(input.KeyF6, input.ModCtrl, false)
	r.frame()
	if r.rt.focusID != keyNode(t, r.rt, "dock/tab:tree").id {
		t.Errorf("Ctrl+F6 did not move focus to the next group's active tab")
	}
}

// TestDockTabMenuDisablesWhatTheLayoutCannotDo checks the tab's menu: a move with no group
// on that side is disabled, and so is Float when the dock has no way to open a window.
func TestDockTabMenuDisablesWhatTheLayoutCannotDo(t *testing.T) {
	r, _ := dockRig(dockSplitLayout(), DockStyle{})
	items := keyNode(t, r.rt, "dock/tab:log").div.contextMenu()
	byLabel := map[string]MenuItem{}
	for _, it := range items {
		byLabel[it.Label] = it
	}
	if byLabel["Move right"].Disabled {
		t.Errorf("Move right is disabled, though the tree group lies on the right")
	}
	if byLabel["Move left"].Disabled != true {
		t.Errorf("Move left is enabled with no group on the left")
	}
	if byLabel["Float"].Disabled != true {
		t.Errorf("Float is enabled without Floating")
	}
	if _, ok := byLabel["Close panel"]; !ok {
		t.Errorf("a closable panel has no Close panel item")
	}
}

func TestDockDragDropMovesPanelToGroupCenter(t *testing.T) {
	r, sig := dockRig(dockSplitLayout(), DockStyle{})
	dragBetween(t, r.rt, "dock/tab:log", "dock/group:tree")
	if got, want := dockText(sig.Peek().Root), "H{0.50 [notes*], 0.50 [tree log*]}"; got != want {
		t.Errorf("after a drop on the center: %s, want %s", got, want)
	}
}

// TestDockDragDropToLeftEdgeMakesAGroupAtTheDock checks that a drop within dockEdgeBand of
// the dock's left edge makes a group along that edge, not beside the group under the
// pointer.
func TestDockDragDropToLeftEdgeMakesAGroupAtTheDock(t *testing.T) {
	r, sig := dockRig(dockSplitLayout(), DockStyle{})
	root, ok := r.rt.Find("dock")
	if !ok {
		t.Fatal("no dock")
	}
	x0, y0 := center(t, r.rt, "dock/tab:log")
	x1, y1 := root.X+4, root.Y+root.H/2
	r.rt.PointerMove(x0, y0, 0)
	r.frame()
	r.rt.PointerDown(x0, y0, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(x0, y0+dragThreshold, 0)
	r.frame()
	r.rt.PointerMove(x1, y1, 0)
	r.frame()
	r.rt.PointerUp(x1, y1, input.ButtonLeft, 0)
	r.frame()
	if got, want := dockText(sig.Peek().Root), "H{0.33 [log*], 0.33 [notes*], 0.33 [tree*]}"; got != want {
		t.Errorf("after a drop on the left edge: %s, want %s", got, want)
	}
}

// TestDockDragPreviewShowsTheZone checks that a drag over a group shows the preview of the
// zone the pointer is in, as the drop would place it.
func TestDockDragPreviewShowsTheZone(t *testing.T) {
	r, _ := dockRig(dockSplitLayout(), DockStyle{})
	dragHover(t, r, "dock/tab:log", "dock/group:tree")
	if got := len(partNodes(r.rt, "Dock.Preview")); got != 1 {
		t.Errorf("%d previews while over a group, want one", got)
	}
}

// TestDockDragPreviewIsNotShownForANoOpDrop checks that a panel dragged over its own alone
// group, where dropping it changes nothing, shows no preview.
func TestDockDragPreviewIsNotShownForANoOpDrop(t *testing.T) {
	r, _ := dockRig(DockLayout{Root: grp("notes", "notes")}, DockStyle{})
	dragHover(t, r, "dock/tab:notes", "dock/group:notes")
	if got := len(partNodes(r.rt, "Dock.Preview")); got != 0 {
		t.Errorf("%d previews for a drop that changes nothing, want none", got)
	}
}

// dockTestWindow is the ui.Window of a floating group in the tests. Only the methods the dock
// calls are implemented; the rest would panic if called.
type dockTestWindow struct {
	Window
	closed     int
	x, y, w, h int
}

func (f *dockTestWindow) Close()               { f.closed++ }
func (f *dockTestWindow) Closed() bool         { return f.closed > 0 }
func (f *dockTestWindow) Position() (int, int) { return f.x, f.y }
func (f *dockTestWindow) Size() (int, int)     { return f.w, f.h }

// TestDockFloatingGroupOpensAndClosesItsWindow checks that the dock asks its opener for a
// window for each floating group, closes it when the group docks back, and that a window
// the user closes records its place and docks its panels back.
func TestDockFloatingGroupOpensAndClosesItsWindow(t *testing.T) {
	var opened []DockWindow
	var wins []*dockTestWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		f := &dockTestWindow{x: w.X, y: w.Y, w: w.Width, h: w.Height}
		wins = append(wins, f)
		return f
	}
	changes := 0
	sig := NewSignal(DockFloatPanel(dockSplitLayout(), "tree", 30, 40, 300, 200))
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").
			Floating(open).OnChange(func(DockLayout) { changes++ }))
	})
	r.frame()
	if len(opened) != 1 {
		t.Fatalf("%d windows opened, want one for the floating group", len(opened))
	}
	w := opened[0]
	if w.Title != "Explorer" || w.X != 30 || w.Y != 40 || w.Width != 300 || w.Height != 200 {
		t.Errorf("window %+v, want the title and place of the floating group", w)
	}

	// The window shows its group, in a runtime of its own.
	wr := newTestRuntime(w.Root, 300, 200, 1)
	if _, ok := wr.Find("dock/float:float-1/tab:tree"); !ok {
		t.Errorf("the floating window does not show the group's tab")
	}

	// The user closes the window: its place is recorded and the panel docks back.
	wins[0].x, wins[0].y, wins[0].w, wins[0].h = 55, 66, 333, 222
	w.OnClose()
	w.OnClosed()
	r.frame()
	if got, want := layoutText(sig.Peek()), "H{0.50 [notes* log], 0.50 [tree*]}"; got != want {
		t.Errorf("after the window closed: %s, want %s", got, want)
	}
	if changes == 0 {
		t.Errorf("OnChange did not run for the docked back panel")
	}

	// A group that docks back closes its window.
	sig.Set(DockFloatPanel(sig.Peek(), "log", 10, 10, 200, 100))
	r.frame()
	if len(opened) != 2 {
		t.Fatalf("%d windows opened, want one more for the floated panel", len(opened))
	}
	sig.Set(DockBack(sig.Peek(), "log"))
	r.frame()
	if wins[1].closed != 1 {
		t.Errorf("the window of a group that docked back was closed %d times, want 1", wins[1].closed)
	}
}

// TestDockWithoutOpenerShowsNoFloatingGroup checks that a dock with no way to open a window
// shows only its docked panels, so a floating group does not appear as a tab.
func TestDockWithoutOpenerShowsNoFloatingGroup(t *testing.T) {
	l := DockFloatPanel(dockSplitLayout(), "tree", 30, 40, 300, 200)
	r, _ := dockRig(l, DockStyle{})
	if n := len(accessNodes(r, access.RoleTab)); n != 2 {
		t.Errorf("%d tabs, want the two docked panels only", n)
	}
}

// testDockParts checks that each part of DockStyle reaches the node that draws it.
func testDockParts(t *testing.T, applied map[string]bool) {
	t.Helper()
	cases := []struct {
		part string
		set  func(*DockStyle, Style)
		key  string
		act  func(t *testing.T, r *accessRig)
	}{
		{part: "Dock.Group", set: func(s *DockStyle, m Style) { s.Group = m }},
		{part: "Dock.Strip", set: func(s *DockStyle, m Style) { s.Strip = m }},
		{part: "Dock.Tab", set: func(s *DockStyle, m Style) { s.Tab = m }, key: "dock/tab:notes"},
		{part: "Dock.TabHover", set: func(s *DockStyle, m Style) { s.TabHover = m }, key: "dock/tab:log",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "dock/tab:log")) }},
		{part: "Dock.TabSelected", set: func(s *DockStyle, m Style) { s.TabSelected = m }, key: "dock/tab:notes"},
		{part: "Dock.TabFocusVisible", set: func(s *DockStyle, m Style) { s.TabFocusVisible = m }, key: "dock/tab:notes",
			act: func(t *testing.T, r *accessRig) { focusNode(r, keyNode(t, r.rt, "dock/tab:notes")) }},
		{part: "Dock.Indicator", set: func(s *DockStyle, m Style) { s.Indicator = m }},
		{part: "Dock.Panel", set: func(s *DockStyle, m Style) { s.Panel = m }, key: "dock/panel:notes"},
		{part: "Dock.Preview", set: func(s *DockStyle, m Style) { s.Preview = m },
			act: func(t *testing.T, r *accessRig) { dragHover(t, r, "dock/tab:log", "dock/group:notes") }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st DockStyle
			c.set(&st, Style{Background: markBG})
			r, _ := dockRig(DockLayout{Root: grp("notes", "notes", "log")}, st)
			if c.act != nil {
				c.act(t, r)
			}
			if c.part == "Dock.Indicator" {
				// Only the active tab's line takes the style; the others are kept transparent.
				found := false
				for _, n := range partNodes(r.rt, "Dock.Indicator") {
					found = found || n.style.Background == markBG
				}
				if !found {
					t.Errorf("no indicator is painted with the style")
				}
				return
			}
			key := c.key
			if c.part == "Dock.Group" || c.part == "Dock.Strip" {
				key = ""
			}
			wantBackground(t, partNode(t, r.rt, drawnBy(c.part), key), c.part, markBG)
		})
	}
}

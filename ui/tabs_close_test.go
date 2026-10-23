package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// closeTabs is a tab list of Alpha (closable), Beta (not closable), Gamma (closable)
// and Delta (closable, disabled). The values the app is asked to close are recorded
// in closed; onClose gives the tab list an OnClose, which records them and drops the
// tab from the list the app keeps, as the app would.
func closeTabs(sel *Signal[string], closed *[]string, onClose bool) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		tabs := []Tab[string]{
			{Value: "a", Label: "Alpha", Closable: true, Panel: func() Element { return Text("Alpha panel") }},
			{Value: "b", Label: "Beta", Panel: func() Element { return Text("Beta panel") }},
			{Value: "c", Label: "Gamma", Closable: true, Panel: func() Element { return Button("Gamma action") }},
			{Value: "d", Label: "Delta", Closable: true, Disabled: true},
		}
		tl := Tabs(sel, tabs...).Key("t")
		if onClose {
			tl.OnClose(func(v string) { *closed = append(*closed, v) })
		}
		return Div().Column().Padding(20).Child(tl)
	})
}

// closeButton returns the element of the close button named name.
func closeButton(t *testing.T, r *accessRig, name string) *node {
	t.Helper()
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.acc != nil && n.div.acc.role == access.RoleButton && n.div.acc.name == name {
			return n
		}
	}
	t.Fatalf("no close button named %q", name)
	return nil
}

// clickNode presses and releases the left button at the center of n.
func clickNode(r *accessRig, n *node) {
	rc := r.rt.nodeRect(n)
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
}

func TestTabsCloseButtonsAreNamedInTheTabItems(t *testing.T) {
	sel := NewSignal("a")
	var closed []string
	r := closeTabs(sel, &closed, true)
	for _, name := range []string{"Close Alpha", "Close Gamma"} {
		if n := closeButton(t, r, name); n == nil {
			t.Fatalf("no %s", name)
		}
	}
	// A disabled and a plain tab have none.
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.acc != nil && n.div.acc.role == access.RoleButton &&
			(n.div.acc.name == "Close Beta" || n.div.acc.name == "Close Delta") {
			t.Fatalf("%q has a close button, want none", n.div.acc.name)
		}
	}
	// The close button is not a Tab stop: Tab from the selected tab leaves the strip.
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyTab, 0)
	if r.focusedAccess(t, access.RoleButton, "Close Alpha") {
		t.Fatalf("Tab reached the close button of Alpha")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsWithoutOnCloseShowNoCloseButtons(t *testing.T) {
	sel := NewSignal("a")
	var closed []string
	r := closeTabs(sel, &closed, false)
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.acc != nil && n.div.acc.role == access.RoleButton && n.div.acc.name == "Close Alpha" {
			t.Fatalf("a close button without OnClose")
		}
	}
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyW, input.ModCtrl)
	r.press(input.KeyDelete, 0)
	if len(closed) != 0 {
		t.Fatalf("closed %v without OnClose", closed)
	}
}

func TestTabsCloseButtonClosesWithoutSelecting(t *testing.T) {
	sel := NewSignal("a")
	var closed []string
	r := closeTabs(sel, &closed, true)
	clickNode(r, closeButton(t, r, "Close Gamma"))
	if !slices.Equal(closed, []string{"c"}) {
		t.Fatalf("OnClose saw %v, want [c]", closed)
	}
	if sel.Peek() != "a" {
		t.Fatalf("clicking the close button selected %q", sel.Peek())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsMiddleClickClosesTheTab(t *testing.T) {
	sel := NewSignal("a")
	var closed []string
	r := closeTabs(sel, &closed, true)
	rc := r.rt.nodeRect(r.keyed(t, "t/tab:c"))
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonMiddle, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonMiddle, 0)
	r.frame()
	if !slices.Equal(closed, []string{"c"}) {
		t.Fatalf("OnClose saw %v after a middle click on Gamma, want [c]", closed)
	}
	if sel.Peek() != "a" {
		t.Fatalf("a middle click selected %q", sel.Peek())
	}
	// A middle click on a tab that is not closable does nothing.
	rc = r.rt.nodeRect(r.keyed(t, "t/tab:b"))
	x, y = rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerDown(x, y, input.ButtonMiddle, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonMiddle, 0)
	r.frame()
	if len(closed) != 1 {
		t.Fatalf("closed %v after a middle click on Beta, want no more", closed)
	}
}

func TestTabsKeysCloseTheFocusedTab(t *testing.T) {
	sel := NewSignal("a")
	var closed []string
	r := closeTabs(sel, &closed, true)
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyW, input.ModCtrl)
	if !slices.Equal(closed, []string{"a"}) {
		t.Fatalf("Ctrl+W closed %v, want [a]", closed)
	}
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyF4, input.ModCtrl)
	r.press(input.KeyDelete, 0)
	if !slices.Equal(closed, []string{"a", "a", "a"}) {
		t.Fatalf("Ctrl+F4 and Delete closed %v, want a again for each", closed)
	}
	// Ctrl+W on a tab that cannot be closed does nothing, and so does Delete.
	sel.Set("b")
	r.frame()
	r.focusKey(t, "t/tab:b")
	r.press(input.KeyW, input.ModCtrl)
	r.press(input.KeyDelete, 0)
	if len(closed) != 3 {
		t.Fatalf("closed %v from the tab that cannot close", closed)
	}
	if sel.Peek() != "b" {
		t.Fatalf("keys selected %q", sel.Peek())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsCtrlWClosesSelectedTabFromThePanel(t *testing.T) {
	sel := NewSignal("c")
	var closed []string
	r := closeTabs(sel, &closed, true)
	r.focusKey(t, "t/tab:c")
	r.press(input.KeyTab, 0) // from the tab list into the panel
	r.press(input.KeyW, input.ModCtrl)
	if !slices.Equal(closed, []string{"c"}) {
		t.Fatalf("Ctrl+W in the panel closed %v, want [c]", closed)
	}
}

// appTabs is a closable tab list whose app keeps the open tabs in a signal, as an
// app does: OnClose drops the tab, and selects the one that took its place when the
// selected tab was closed.
func appTabs(open *Signal[[]string], sel *Signal[string]) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		var tabs []Tab[string]
		for _, v := range open.Get() {
			tabs = append(tabs, Tab[string]{Value: v, Label: "Tab " + v, Closable: true, Panel: func() Element { return Text(v) }})
		}
		tl := Tabs(sel, tabs...).Key("t").Reorderable().OnClose(func(v string) {
			list := slices.Clone(open.Get())
			i := slices.Index(list, v)
			list = slices.Delete(list, i, i+1)
			open.Set(list)
			if sel.Peek() == v && len(list) > 0 {
				sel.Set(list[min(i, len(list)-1)])
			}
		})
		return Div().Column().Padding(20).Child(tl)
	})
}

func TestTabsAppDropsTheClosedTabAndFocusGoesToTheNextSelection(t *testing.T) {
	open := NewSignal([]string{"a", "b", "c"})
	sel := NewSignal("b")
	r := appTabs(open, sel)
	r.focusKey(t, "t/tab:b")
	r.press(input.KeyW, input.ModCtrl)
	if got := open.Peek(); !slices.Equal(got, []string{"a", "c"}) {
		t.Fatalf("open tabs %v after closing b, want [a c]", got)
	}
	if sel.Peek() != "c" {
		t.Fatalf("selection %q after closing b, want the tab that took its place: c", sel.Peek())
	}
	if got := r.focusedKey(); got != "t/tab:c" {
		t.Fatalf("focus on %q after the close, want the selected tab t/tab:c", got)
	}
	// Closing the last tab selects the one before it.
	r.focusKey(t, "t/tab:c")
	r.press(input.KeyDelete, 0)
	if sel.Peek() != "a" || !slices.Equal(open.Peek(), []string{"a"}) {
		t.Fatalf("selection %q, open %v after closing the last tab", sel.Peek(), open.Peek())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsCloseFollowsReordering(t *testing.T) {
	open := NewSignal([]string{"a", "b", "c"})
	sel := NewSignal("a")
	r := appTabs(open, sel)
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyRight, input.ModCtrl|input.ModShift) // the tabs are now b, a, c
	if got := tabNames(r); got != "Tab b,Tab a,Tab c" {
		t.Fatalf("tabs are %s after the move, want Tab b,Tab a,Tab c", got)
	}
	r.press(input.KeyW, input.ModCtrl) // Alpha's place in the strip is now the second
	if got := open.Peek(); !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("open tabs %v after closing the moved a, want [b c]", got)
	}
	// The app closed a, the first tab it holds, so the tab that took its place is b.
	if sel.Peek() != "b" {
		t.Fatalf("selection %q after closing the moved a, want b", sel.Peek())
	}
}

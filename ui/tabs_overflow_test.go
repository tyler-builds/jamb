package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// manyTabs are eight tabs, too wide together for a 300 px window.
func manyTabs() []Tab[string] {
	var tabs []Tab[string]
	for _, name := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel"} {
		v := strings.ToLower(name[:1])
		tabs = append(tabs, Tab[string]{Value: v, Label: name, Panel: func() Element {
			return Button(name + " action").Key("action")
		}})
	}
	return tabs
}

// overflowRig is a 300 px window with the eight tabs, made into a tab list by
// build, and the tabs selected at sel.
func overflowRig(sel *Signal[string], build func(*TabsElement[string]) Element) *accessRig {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(build(Tabs(sel, manyTabs()...).Key("t").Label("Sections")))
	})
	r.rt.SetViewport(300, 400, 1)
	settle(r)
	return r
}

// settle runs frames until the strip's scroll has eased to its target.
func settle(r *accessRig) {
	for range 40 {
		r.frame()
	}
}

// stripNode is the strip of the one tab list in the rig.
func stripNode(t *testing.T, r *accessRig) *node {
	t.Helper()
	return partNode(t, r.rt, "Tabs.List", "")
}

// shownInStrip fails unless the tab with key is drawn inside the strip.
func shownInStrip(t *testing.T, r *accessRig, key string) {
	t.Helper()
	strip := r.rt.nodeRect(stripNode(t, r))
	tab, ok := r.rt.Find(key)
	if !ok {
		t.Fatalf("no tab %q", key)
	}
	if tab.X < strip.X-0.5 || tab.Right() > strip.Right()+0.5 {
		t.Fatalf("tab %q at %v..%v is not inside the strip at %v..%v", key, tab.X, tab.Right(), strip.X, strip.Right())
	}
}

// Tabs that fit build the strip as before: no bar, no buttons, no scrolling.
func TestTabsThatFitAreUnchanged(t *testing.T) {
	sel := NewSignal("a")
	r := overflowRig(sel, func(tb *TabsElement[string]) Element { return tb })
	r.rt.SetViewport(900, 400, 1)
	settle(r)
	if n := partNodes(r.rt, "Tabs.Bar"); len(n) != 0 {
		t.Fatalf("tabs that fit are in a bar")
	}
	strip := stripNode(t, r)
	if strip.style.OverflowX != OverflowVisible || strip.parent.key != "t" {
		t.Fatalf("the strip of tabs that fit scrolls (%v) or is not in the group", strip.style.OverflowX)
	}
	if _, ok := r.rt.Find("back"); ok {
		t.Fatal("tabs that fit show scroll buttons")
	}
}

// Overflowing tabs scroll: End scrolls the last tab into the strip, the scroll
// buttons are shown, disabled at their end, and Tab does not reach them.
func TestTabsOverflowScrollsToTheSelectedTab(t *testing.T) {
	sel := NewSignal("a")
	r := overflowRig(sel, func(tb *TabsElement[string]) Element { return tb })
	if stripNode(t, r).style.OverflowX != OverflowScroll {
		t.Fatal("the strip of overflowing tabs does not scroll")
	}
	if !r.stateOf(t, access.RoleButton, "Scroll tabs back").States.Has(access.StateDisabled) {
		t.Fatal("the back button is enabled at the start of the strip")
	}
	if r.stateOf(t, access.RoleButton, "Scroll tabs forward").States.Has(access.StateDisabled) {
		t.Fatal("the forward button is disabled at the start of the strip")
	}
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyEnd, 0)
	settle(r)
	if sel.Get() != "h" {
		t.Fatalf("End selected %q", sel.Get())
	}
	shownInStrip(t, r, "t/tab:h")
	if r.stateOf(t, access.RoleButton, "Scroll tabs back").States.Has(access.StateDisabled) {
		t.Fatal("the back button is disabled at the end of the strip")
	}
	if !r.stateOf(t, access.RoleButton, "Scroll tabs forward").States.Has(access.StateDisabled) {
		t.Fatal("the forward button is enabled at the end of the strip")
	}
	// Tab leaves the strip for the panel, past the buttons.
	r.press(input.KeyTab, 0)
	if got := r.focusedKey(); got != "action" {
		t.Fatalf("Tab from the strip went to %q, want the panel's button", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// The scroll buttons move the strip, and keep the focus where it is.
func TestTabsScrollButtonsMoveTheStrip(t *testing.T) {
	sel := NewSignal("a")
	r := overflowRig(sel, func(tb *TabsElement[string]) Element { return tb })
	r.focusKey(t, "t/tab:a")
	clickFound(t, r, "forward")
	settle(r)
	if x := stripNode(t, r).st.scrollX; x <= 0 {
		t.Fatalf("the forward button left the strip at %v", x)
	}
	if got := r.focusedKey(); got != "t/tab:a" {
		t.Fatalf("a press on a scroll button moved the focus to %q", got)
	}
	clickFound(t, r, "back")
	settle(r)
	if x := stripNode(t, r).st.scrollX; x != 0 {
		t.Fatalf("the back button left the strip at %v", x)
	}
}

// A vertical wheel over the strip scrolls it sideways, and is the page's at the end.
func TestTabsWheelScrollsTheStripSideways(t *testing.T) {
	sel := NewSignal("a")
	r := overflowRig(sel, func(tb *TabsElement[string]) Element { return tb })
	rc := r.rt.nodeRect(stripNode(t, r))
	r.rt.Scroll(rc.X+20, rc.Y+rc.H/2, 0, 120, true, 0)
	settle(r)
	if x := stripNode(t, r).st.scrollX; x != 120 {
		t.Fatalf("a wheel of 120 moved the strip to %v", x)
	}
	r.rt.Scroll(rc.X+20, rc.Y+rc.H/2, 0, 10000, true, 0)
	settle(r)
	r.rt.Scroll(rc.X+20, rc.Y+rc.H/2, 0, -10000, true, 0)
	settle(r)
	if x := stripNode(t, r).st.scrollX; x != 0 {
		t.Fatalf("a wheel back left the strip at %v", x)
	}
}

// A selection made by code is scrolled to.
func TestTabsSelectionByCodeIsRevealed(t *testing.T) {
	sel := NewSignal("a")
	r := overflowRig(sel, func(tb *TabsElement[string]) Element { return tb })
	sel.Set("g")
	settle(r)
	shownInStrip(t, r, "t/tab:g")
}

// The overflow menu lists every tab; choosing one selects, reveals and focuses it.
func TestTabsOverflowMenuChoosesATab(t *testing.T) {
	sel := NewSignal("a")
	var changed []string
	r := overflowRig(sel, func(tb *TabsElement[string]) Element {
		return tb.OverflowMenu().OnChange(func(v string) { changed = append(changed, v) })
	})
	r.focusAccess(t, access.RoleButton, "All tabs")
	r.press(input.KeyEnter, 0)
	tree := r.tree()
	for _, name := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel"} {
		accesstest.Find(t, tree, access.RoleMenuItemRadio, name)
	}
	if !accesstest.Find(t, tree, access.RoleMenuItemRadio, "Alpha").States.Has(access.StateChecked) {
		t.Fatal("the selected tab is not checked in the menu")
	}
	clickFound(t, r, "jamb.menu.0.7")
	settle(r)
	if sel.Get() != "h" || len(changed) != 1 || changed[0] != "h" {
		t.Fatalf("choosing Hotel selected %q, OnChange saw %v", sel.Get(), changed)
	}
	if got := r.focusedKey(); got != "t/tab:h" {
		t.Fatalf("focus is on %q, want the chosen tab", got)
	}
	shownInStrip(t, r, "t/tab:h")
}

// A capped tab cuts its label short, and keeps the whole label as its name.
func TestTabsTabMaxWidthTruncates(t *testing.T) {
	sel := NewSignal("a")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Tabs(sel,
			Tab[string]{Value: "a", Label: "A label far too long for the cap"},
			Tab[string]{Value: "b", Label: "Short"},
		).Key("t").TabMaxWidth(Px(80)))
	})
	if w := r.nodeByKey(t, "t/tab:a").lay.W; w > 80 {
		t.Fatalf("the capped tab is %v wide", w)
	}
	tab := r.stateOf(t, access.RoleTab, "A label far too long for the cap")
	if tab.Description != "" {
		t.Fatalf("the capped tab is described as %q", tab.Description)
	}
}

// Prefix and Suffix sit in the bar beside the strip, and Tab visits them in order.
func TestTabsPrefixAndSuffix(t *testing.T) {
	sel := NewSignal("a")
	r := overflowRig(sel, func(tb *TabsElement[string]) Element {
		return tb.Prefix(Button("Before").Key("before")).Suffix(Button("After").Key("after")).OverflowMenu()
	})
	bar := partNode(t, r.rt, "Tabs.Bar", "")
	if stripNode(t, r).parent != bar {
		t.Fatal("the strip is not in the bar")
	}
	r.focusKey(t, "before")
	var order []string
	for range 4 {
		r.press(input.KeyTab, 0)
		order = append(order, r.focusedKey())
	}
	if want := "t/tab:a all after action"; strings.Join(order, " ") != want {
		t.Fatalf("Tab visits %v, want %s", order, want)
	}
}

// An open popup closes when the wheel scrolls the strip.
func TestTabsPopupClosesWhenTheStripScrolls(t *testing.T) {
	sel := NewSignal("a")
	r := newAccessRig(func(ctx *Context) Element {
		tabs := manyTabs()
		tabs[0].Popup = func(close func()) Element { return Button("Rename").Key("rename") }
		return Div().Column().Child(Tabs(sel, tabs...).Key("t"))
	})
	r.rt.SetViewport(300, 400, 1)
	settle(r)
	r.clickKey(t, "t/tab:a")
	if _, ok := r.rt.Find("rename"); !ok {
		t.Fatal("the popup did not open")
	}
	rc := r.rt.nodeRect(stripNode(t, r))
	r.rt.Scroll(rc.X+20, rc.Y+rc.H/2, 0, 120, true, 0)
	settle(r)
	if _, ok := r.rt.Find("rename"); ok {
		t.Fatal("the popup stayed open while the strip scrolled")
	}
}

// Under high contrast the scroll buttons are drawn in the system's text color.
func TestTabsScrollButtonsUnderHighContrast(t *testing.T) {
	th := HighContrastTheme(HighContrastBlack(), DarkTheme())
	sel := NewSignal("a")
	r := newThemedRig(th, func(ctx *Context) Element {
		return Div().Column().Child(Tabs(sel, manyTabs()...).Key("t"))
	})
	r.rt.SetViewport(300, 400, 1)
	settle(r)
	n := r.nodeByKey(t, "forward")
	if c := effectiveTextColor(n); c != th.TextMuted && c != th.Text {
		t.Fatalf("the forward button is drawn in %v", c)
	}
	if n.lay.W == 0 {
		t.Fatal("the forward button is not laid out")
	}
}

// A segmented strip keeps the width of its tabs until it reaches the window's,
// and then scrolls as the underline strip does.
func TestTabsSegmentedOverflowScrolls(t *testing.T) {
	sel := NewSignal("a")
	r := overflowRig(sel, func(tb *TabsElement[string]) Element { return tb.Variant(TabsSegmented) })
	strip := stripNode(t, r)
	if strip.style.OverflowX != OverflowScroll || strip.lay.W > 300 {
		t.Fatalf("the segmented strip is %v wide and scrolls: %v", strip.lay.W, strip.style.OverflowX == OverflowScroll)
	}
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyEnd, 0)
	settle(r)
	shownInStrip(t, r, "t/tab:h")
	r.rt.SetViewport(900, 400, 1)
	settle(r)
	if strip := stripNode(t, r); strip.style.OverflowX == OverflowScroll || strip.lay.W >= 900 {
		t.Fatalf("the segmented strip that fits is %v wide and scrolls: %v", strip.lay.W, strip.style.OverflowX == OverflowScroll)
	}
}

// A focused tab keeps the focus when the window narrows and the scroll buttons
// appear, and when it widens and they go.
func TestTabsFocusSurvivesTheScrollButtons(t *testing.T) {
	sel := NewSignal("c")
	r := overflowRig(sel, func(tb *TabsElement[string]) Element { return tb })
	r.rt.SetViewport(900, 400, 1)
	settle(r)
	r.focusKey(t, "t/tab:c")
	for _, w := range []float32{300, 900} {
		r.rt.SetViewport(w, 400, 1)
		settle(r)
		if got := r.focusedKey(); got != "t/tab:c" {
			t.Fatalf("at %v px the focus is on %q, want the selected tab", w, got)
		}
	}
}

// The scroll buttons, once shown, stay while the tabs would not fit in the room
// they give back, and go once they would; at each width the strip settles on one
// of the two, frame after frame, without flickering between them.
func TestTabsScrollButtonsDoNotFlicker(t *testing.T) {
	sel := NewSignal("a")
	r := overflowRig(sel, func(tb *TabsElement[string]) Element { return tb })
	r.rt.SetViewport(900, 400, 1)
	settle(r)
	last := r.nodeByKey(t, "t/tab:h")
	content := last.lay.X + last.lay.W
	shown := func() bool { _, ok := r.rt.Find("forward"); return ok }
	steady := func(w float32, want bool) {
		t.Helper()
		r.rt.SetViewport(w, 400, 1)
		for range 10 {
			r.frame()
			if shown() != want {
				t.Fatalf("at %v px the scroll buttons are shown: %v, want %v", w, shown(), want)
			}
		}
	}
	steady(content+2, false)
	steady(content-2, true)
	steady(content-40, true)
	steady(content+2, false)
	steady(content-1, true)
}

package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// popupTabs is a tab list whose Alpha tab has a Popup with two buttons, the
// second of which closes it, and whose Gamma tab has none. reselected records
// the values OnReselect saw.
func popupTabs(sel *Signal[string], reselected *[]string) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Tabs(sel,
			Tab[string]{Value: "a", Label: "Alpha", Panel: func() Element { return Text("Alpha panel") },
				Popup: func(close func()) Element {
					return Div().Column().Children(
						Button("Rename").Key("rename"),
						Button("Done").Key("done").OnClick(close),
					)
				}},
			Tab[string]{Value: "c", Label: "Gamma", Panel: func() Element { return Text("Gamma panel") }},
		).Key("t").OnReselect(func(v string) { *reselected = append(*reselected, v) }))
	})
}

func popupOpen(r *accessRig) bool {
	_, ok := r.rt.Find("rename")
	return ok
}

// A click on the selected tab is a reselect, and opens its popup; a second
// click closes it. A click on another tab only selects it.
func TestTabsReselectOpensPopupOnSecondClick(t *testing.T) {
	sel := NewSignal("a")
	var re []string
	r := popupTabs(sel, &re)
	r.clickKey(t, "t/tab:c")
	if len(re) != 0 || popupOpen(r) {
		t.Fatalf("selecting another tab reselected %v or opened a popup", re)
	}
	r.clickKey(t, "t/tab:a")
	if len(re) != 0 || popupOpen(r) {
		t.Fatalf("selecting Alpha reselected %v or opened its popup", re)
	}
	r.clickKey(t, "t/tab:a")
	if len(re) != 1 || re[0] != "a" {
		t.Fatalf("OnReselect saw %v, want [a]", re)
	}
	if !popupOpen(r) {
		t.Fatal("clicking the selected tab did not open its popup")
	}
	tab := accesstest.Find(t, r.tree(), access.RoleTab, "Alpha")
	if !tab.States.Has(access.StateExpandable) || !tab.States.Has(access.StateExpanded) {
		t.Fatalf("the tab with an open popup has states [%s]", tab.States)
	}
	accesstest.Find(t, r.tree(), access.RoleDialog, "Alpha")
	if got := r.focusedKey(); got != "rename" {
		t.Fatalf("focus is on %q, want the popup's first button", got)
	}
	r.clickKey(t, "t/tab:a")
	if popupOpen(r) {
		t.Fatal("a second click on the tab did not close its popup")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// A tab without a Popup still reports reselects and is not expandable.
func TestTabsReselectWithoutPopup(t *testing.T) {
	sel := NewSignal("c")
	var re []string
	r := popupTabs(sel, &re)
	r.clickKey(t, "t/tab:c")
	r.focusKey(t, "t/tab:c")
	r.press(input.KeyEnter, 0)
	r.press(input.KeyDown, 0)
	if len(re) != 2 || re[0] != "c" || re[1] != "c" {
		t.Fatalf("OnReselect saw %v, want [c c] from the click and Enter", re)
	}
	if popupOpen(r) {
		t.Fatal("a popup opened on a tab without one")
	}
	if accesstest.Find(t, r.tree(), access.RoleTab, "Gamma").States.Has(access.StateExpandable) {
		t.Fatal("a tab without a popup is reported expandable")
	}
}

// Down, Shift+F10 and the Menu key open the popup from the focused tab; Escape
// and the content's close func close it and return focus to the tab. Keys in
// the popup do not move between tabs.
func TestTabsPopupKeyboard(t *testing.T) {
	sel := NewSignal("a")
	var re []string
	r := popupTabs(sel, &re)
	for _, k := range []struct {
		key  input.Key
		mods input.Modifiers
	}{{input.KeyDown, 0}, {input.KeyF10, input.ModShift}, {input.KeyMenu, 0}} {
		r.focusKey(t, "t/tab:a")
		r.press(k.key, k.mods)
		if !popupOpen(r) {
			t.Fatalf("key %v+%v did not open the popup", k.mods, k.key)
		}
		r.press(input.KeyEscape, 0)
		if popupOpen(r) {
			t.Fatal("Escape did not close the popup")
		}
		if got := r.focusedKey(); got != "t/tab:a" {
			t.Fatalf("focus is on %q after Escape, want the tab", got)
		}
	}
	if len(re) != 0 {
		t.Fatalf("opening from the keyboard counted as a reselect: %v", re)
	}

	r.focusKey(t, "t/tab:a")
	r.press(input.KeyDown, 0)
	r.press(input.KeyRight, 0)
	r.press(input.KeyTab, input.ModCtrl)
	if sel.Peek() != "a" || !popupOpen(r) {
		t.Fatalf("keys in the popup selected %q (popup open: %v), want a with the popup open", sel.Peek(), popupOpen(r))
	}
	r.clickKey(t, "done")
	if popupOpen(r) {
		t.Fatal("close did not close the popup")
	}
	if got := r.focusedKey(); got != "t/tab:a" {
		t.Fatalf("focus is on %q after close, want the tab", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// Expand and Collapse from assistive technology open and close the popup
// without a reselect, and a selection changed in code closes it.
func TestTabsPopupAccessAndSelectionChange(t *testing.T) {
	sel := NewSignal("a")
	var re []string
	r := popupTabs(sel, &re)
	tab := accesstest.Find(t, r.tree(), access.RoleTab, "Alpha")
	r.bridge.Perform(tab.ID, access.ActionExpand)
	r.frame()
	if !popupOpen(r) {
		t.Fatal("Expand did not open the popup")
	}
	r.bridge.Perform(tab.ID, access.ActionCollapse)
	r.frame()
	if popupOpen(r) {
		t.Fatal("Collapse did not close the popup")
	}
	if len(re) != 0 {
		t.Fatalf("Expand or Collapse counted as a reselect: %v", re)
	}
	r.bridge.Perform(tab.ID, access.ActionExpand)
	r.frame()
	sel.Set("c")
	r.frame()
	sel.Set("a")
	r.frame()
	if popupOpen(r) {
		t.Fatal("the popup is open again after the selection left its tab and came back")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// TestClickedDockTabShowsKeyboardFocus checks that a dock tab which was clicked shows its
// keyboard focus style once the keyboard is used. The click selects the tab and re-renders
// the dock; a key press then moves focus to the other tab of the group, which must be drawn
// with TabFocusVisible, and a key that keeps focus (End, on the last tab) must show it on the
// clicked tab.
func TestClickedDockTabShowsKeyboardFocus(t *testing.T) {
	mark := Hex("#123456")
	r, _ := dockRig(dockSplitLayout(), DockStyle{TabFocusVisible: Style{Background: mark}})
	x, y := center(t, r.rt, "dock/tab:log")
	r.clickAt(x, y)
	if got := treeNode(t, r.rt, "dock/tab:log"); got.id != r.rt.focusID {
		t.Fatalf("the click did not focus the tab")
	}
	if got := treeNode(t, r.rt, "dock/tab:log").style.Background; got == mark {
		t.Fatalf("the clicked tab shows its keyboard focus style before any key")
	}

	r.rt.KeyDown(input.KeyEnd, 0, false) // End selects the last tab, which the clicked tab is
	r.frame()
	if n := treeNode(t, r.rt, "dock/tab:log"); n.id != r.rt.focusID || n.style.Background != mark {
		t.Errorf("after a key, the clicked tab: focused %v, background %v; want focused and %v", n.id == r.rt.focusID, n.style.Background, mark)
	}

	r.rt.KeyDown(input.KeyLeft, 0, false)
	r.frame()
	n := treeNode(t, r.rt, "dock/tab:notes")
	if n.id != r.rt.focusID {
		t.Fatalf("Left did not move focus to the notes tab")
	}
	if r.rt.byID[n.id] != n {
		t.Errorf("byID does not hold the live notes tab")
	}
	if n.style.Background != mark {
		t.Errorf("the notes tab, focused from the keyboard, has background %v; want %v", n.style.Background, mark)
	}
}

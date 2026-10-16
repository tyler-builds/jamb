package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// reorderTabs is a reorderable tab list of Alpha, Beta (disabled) and Gamma, plus
// the tabs in extra. orders records what OnReorder saw.
func reorderTabs(sel *Signal[string], extra *Signal[[]Tab[string]], orders *[][]string, reorder bool) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		tabs := []Tab[string]{
			{Value: "a", Label: "Alpha", Panel: func() Element { return Text("Alpha panel") }},
			{Value: "b", Label: "Beta", Disabled: true},
			{Value: "c", Label: "Gamma", Panel: func() Element { return Text("Gamma panel") }},
		}
		tabs = append(tabs, extra.Get()...)
		tl := Tabs(sel, tabs...).Key("t").OnReorder(func(o []string) { *orders = append(*orders, o) })
		if reorder {
			tl.Reorderable()
		}
		return Div().Column().Padding(20).Child(tl)
	})
}

// tabNames returns the names of the tabs in the accessibility tree, in order.
func tabNames(r *accessRig) string {
	var out []string
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleTab {
			out = append(out, n.Name)
		}
		return true
	})
	return strings.Join(out, ",")
}

// dragTab drags the tab from onto the tab to, at its right or left quarter, and
// reports the number of drop lines while the pointer is over it.
func dragTab(t *testing.T, r *accessRig, from, to string, after bool) int {
	t.Helper()
	x0, y0 := center(t, r.rt, from)
	n, ok := r.rt.Find(to)
	if !ok {
		t.Fatalf("no element with key %q", to)
	}
	x1, y1 := n.X+n.W/4, n.Y+n.H/2
	if after {
		x1 = n.X + n.W*3/4
	}
	r.rt.PointerMove(x0, y0, 0)
	r.frame()
	r.rt.PointerDown(x0, y0, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(x0+dragThreshold, y0, 0)
	r.frame()
	r.rt.PointerMove(x1, y1, 0)
	r.frame()
	lines := len(partNodes(r.rt, "Tabs.DropLine"))
	r.rt.PointerUp(x1, y1, input.ButtonLeft, 0)
	r.frame()
	return lines
}

func TestTabsReorderWithKeyboard(t *testing.T) {
	sel := NewSignal("a")
	extra := NewSignal([]Tab[string](nil))
	var orders [][]string
	r := reorderTabs(sel, extra, &orders, true)
	takeAnnouncements()
	var said []string
	r.bridge.Listen(func(u access.Update) {
		for _, a := range u.Announcements {
			said = append(said, a.Text)
		}
	})
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyLeft, input.ModCtrl|input.ModShift) // already first
	if len(orders) != 0 {
		t.Fatalf("moving the first tab left reordered: %v", orders)
	}
	r.press(input.KeyRight, input.ModCtrl|input.ModShift)
	if got := tabNames(r); got != "Beta,Alpha,Gamma" {
		t.Fatalf("tabs are %s after Ctrl+Shift+Right, want Beta,Alpha,Gamma", got)
	}
	if len(orders) != 1 || !slices.Equal(orders[0], []string{"b", "a", "c"}) {
		t.Fatalf("OnReorder saw %v, want [[b a c]]", orders)
	}
	if got := r.focusedKey(); got != "t/tab:a" || sel.Peek() != "a" {
		t.Fatalf("focus %q and selection %q after the move, want the moved Alpha tab", got, sel.Peek())
	}
	for _, a := range takeAnnouncements() {
		said = append(said, a.Text)
	}
	if !slices.Contains(said, "Alpha moved to position 2 of 3") {
		t.Fatalf("announced %q, want the new position", said)
	}
	r.press(input.KeyRight, input.ModCtrl|input.ModShift)
	r.press(input.KeyRight, input.ModCtrl|input.ModShift) // already last
	if got := tabNames(r); got != "Beta,Gamma,Alpha" || len(orders) != 2 {
		t.Fatalf("tabs are %s after %d moves, want Beta,Gamma,Alpha after 2", got, len(orders))
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsReorderWithDrag(t *testing.T) {
	sel := NewSignal("a")
	extra := NewSignal([]Tab[string](nil))
	var orders [][]string
	r := reorderTabs(sel, extra, &orders, true)
	if lines := dragTab(t, r, "t/tab:a", "t/tab:c", true); lines != 1 {
		t.Fatalf("%d drop lines while dragging over Gamma, want 1", lines)
	}
	if got := tabNames(r); got != "Beta,Gamma,Alpha" {
		t.Fatalf("tabs are %s after dropping Alpha after Gamma, want Beta,Gamma,Alpha", got)
	}
	if n := len(partNodes(r.rt, "Tabs.DropLine")); n != 0 {
		t.Fatalf("%d drop lines after the drop, want none", n)
	}
	dragTab(t, r, "t/tab:a", "t/tab:b", false) // beside the disabled Beta
	if got := tabNames(r); got != "Alpha,Beta,Gamma" {
		t.Fatalf("tabs are %s after dropping Alpha before Beta, want Alpha,Beta,Gamma", got)
	}
	dragTab(t, r, "t/tab:b", "t/tab:c", true) // a disabled tab does not drag
	if got := tabNames(r); got != "Alpha,Beta,Gamma" || len(orders) != 2 {
		t.Fatalf("tabs are %s with %d reorders after dragging the disabled Beta", got, len(orders))
	}
	if sel.Peek() != "a" {
		t.Fatalf("dragging selected %q", sel.Peek())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// The user's order survives a render, and a tab added later goes at the end.
func TestTabsReorderKeepsOrderAndAppendsNewTabs(t *testing.T) {
	sel := NewSignal("a")
	extra := NewSignal([]Tab[string](nil))
	var orders [][]string
	r := reorderTabs(sel, extra, &orders, true)
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyRight, input.ModCtrl|input.ModShift)
	extra.Set([]Tab[string]{{Value: "d", Label: "Delta"}})
	r.frame()
	if got := tabNames(r); got != "Beta,Alpha,Gamma,Delta" {
		t.Fatalf("tabs are %s, want Beta,Alpha,Gamma,Delta", got)
	}
}

func TestTabsNotReorderableByDefault(t *testing.T) {
	sel := NewSignal("a")
	extra := NewSignal([]Tab[string](nil))
	var orders [][]string
	r := reorderTabs(sel, extra, &orders, false)
	r.focusKey(t, "t/tab:a")
	r.press(input.KeyRight, input.ModCtrl|input.ModShift)
	dragTab(t, r, "t/tab:a", "t/tab:c", true)
	if got := tabNames(r); got != "Alpha,Beta,Gamma" || len(orders) != 0 {
		t.Fatalf("tabs are %s with reorders %v without Reorderable", got, orders)
	}
}

func TestArrangeTabs(t *testing.T) {
	tabs := []Tab[int]{{Value: 1}, {Value: 2}, {Value: 3}}
	got := arrangeTabs(tabs, []int{3, 9, 1})
	var vals []int
	for _, tb := range got {
		vals = append(vals, tb.Value)
	}
	if !slices.Equal(vals, []int{3, 1, 2}) {
		t.Fatalf("arranged %v, want [3 1 2]: known values first, unknown ones skipped, new ones last", vals)
	}
}

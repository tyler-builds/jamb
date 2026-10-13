package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/layout"
)

// tabsDemo is a tab list with three tabs, the middle one disabled. Its panels
// record in built which of them are built, and the first holds a button.
func tabsDemo(sel *Signal[string], built *[]string) Element {
	return Div().Column().Child(Tabs(sel,
		Tab[string]{Value: "a", Label: "Alpha", Icon: icons.Check, Panel: func() Element {
			*built = append(*built, "a")
			return Div().Column().Child(Button("Inside alpha").Key("inside-a"))
		}},
		Tab[string]{Value: "b", Label: "Beta", Disabled: true, Panel: func() Element {
			*built = append(*built, "b")
			return Text("Beta panel")
		}},
		Tab[string]{Value: "c", Label: "Gamma", Panel: func() Element {
			*built = append(*built, "c")
			return Text("Gamma panel")
		}},
	).Key("settings").Label("Settings sections"))
}

func TestTabsBuildOnlyTheSelectedPanel(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	r := newAccessRig(func(ctx *Context) Element { return tabsDemo(sel, &built) })
	if got := strings.Join(built, ","); got != "a" {
		t.Fatalf("built panels %q at first, want only the selected one (a)", got)
	}
	r.clickKey(t, "settings/tab:c")
	if sel.Peek() != "c" {
		t.Fatalf("a click did not select the tab: %q", sel.Peek())
	}
	if got := strings.Join(built, ","); got != "a,c" {
		t.Fatalf("built panels %q after the click, want a,c: only the selected panel is built", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsArrowKeysSelectAndSkipDisabled(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	r := newAccessRig(func(ctx *Context) Element { return tabsDemo(sel, &built) })
	r.focusKey(t, "settings/tab:a")
	r.press(input.KeyRight, 0) // Beta is disabled, so Gamma is next
	if sel.Peek() != "c" {
		t.Fatalf("Right selected %q, want c", sel.Peek())
	}
	if got := r.focusedKey(); got != "settings/tab:c" {
		t.Fatalf("focus is on %q after Right, want the selected tab", got)
	}
	r.press(input.KeyRight, 0) // the last tab does not wrap
	if sel.Peek() != "c" {
		t.Fatalf("Right past the last tab selected %q", sel.Peek())
	}
	r.press(input.KeyHome, 0)
	if sel.Peek() != "a" {
		t.Fatalf("Home selected %q, want the first enabled tab a", sel.Peek())
	}
	r.press(input.KeyEnd, 0)
	if sel.Peek() != "c" {
		t.Fatalf("End selected %q, want the last tab c", sel.Peek())
	}
	r.press(input.KeyLeft, 0)
	if sel.Peek() != "a" {
		t.Fatalf("Left selected %q, want a", sel.Peek())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsOneTabStopAtTheSelectedTab(t *testing.T) {
	sel := NewSignal("c")
	var built []string
	r := newAccessRig(func(ctx *Context) Element { return tabsDemo(sel, &built) })
	r.frame()
	inOrder := func(key string) bool {
		n := r.nodeByKey(t, key)
		for _, id := range r.rt.tabOrder() {
			if id == n.id {
				return true
			}
		}
		return false
	}
	if !inOrder("settings/tab:c") {
		t.Fatalf("the selected tab is not in the Tab order")
	}
	if inOrder("settings/tab:a") {
		t.Fatalf("an unselected tab is in the Tab order")
	}
}

func TestTabsCtrlTabLoopsFromTheTabListAndThePanel(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	r := newAccessRig(func(ctx *Context) Element { return tabsDemo(sel, &built) })
	r.focusKey(t, "settings/tab:a")
	r.press(input.KeyTab, input.ModCtrl) // Beta is disabled, so Gamma
	if sel.Peek() != "c" {
		t.Fatalf("Ctrl+Tab selected %q, want c", sel.Peek())
	}
	r.press(input.KeyTab, input.ModCtrl) // wraps to the first tab
	if sel.Peek() != "a" {
		t.Fatalf("Ctrl+Tab did not wrap: selected %q", sel.Peek())
	}
	r.press(input.KeyTab, input.ModCtrl|input.ModShift) // back, and wraps to the last
	if sel.Peek() != "c" {
		t.Fatalf("Ctrl+Shift+Tab selected %q, want c", sel.Peek())
	}
	// From the panel of the selected tab, Ctrl+Tab switches the tab and moves focus
	// to the tab, because the panel it was in is replaced.
	sel.Set("a")
	r.frame()
	r.focusKey(t, "inside-a")
	r.press(input.KeyTab, input.ModCtrl)
	if sel.Peek() != "c" {
		t.Fatalf("Ctrl+Tab from the panel selected %q, want c", sel.Peek())
	}
	if got := r.focusedKey(); got != "settings/tab:c" {
		t.Fatalf("focus is on %q after Ctrl+Tab from the panel, want the selected tab", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsAccessibilityTree(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	r := newAccessRig(func(ctx *Context) Element { return tabsDemo(sel, &built) })
	list := r.stateOf(t, access.RoleTabList, "Settings sections")
	if !list.States.Has(access.StateSelectionContainer) {
		t.Fatalf("the tab list is not a selection container")
	}
	alpha := r.stateOf(t, access.RoleTab, "Alpha")
	accesstest.Check(t, alpha, access.RoleTab, "Alpha", access.StateSelectable|access.StateSelected|access.StateFocusable)
	if !alpha.Has(access.ActionSelect) {
		t.Fatalf("a tab has no Select action: %v", alpha.Actions)
	}
	if alpha.Has(access.ActionClick) {
		t.Fatalf("a tab has an Invoke (Click) action; a tab is selected, not invoked: %v", alpha.Actions)
	}
	beta := r.stateOf(t, access.RoleTab, "Beta")
	if !beta.States.Has(access.StateDisabled) || beta.States.Has(access.StateSelected) {
		t.Fatalf("Beta is %v, want disabled and not selected", beta.States)
	}
	// The panel is a pane labelled by the tab that shows it.
	var panel *access.Node
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleTabPanel {
			panel = n
		}
		return true
	})
	if panel == nil {
		t.Fatalf("the selected tab shows no tab panel")
	}
	if len(panel.LabelledBy) != 1 || panel.LabelledBy[0] != alpha.ID {
		t.Fatalf("the panel is labelled by %v, want the Alpha tab %d", panel.LabelledBy, alpha.ID)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsSelectionMovesWithSelectItem(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	r := newAccessRig(func(ctx *Context) Element { return tabsDemo(sel, &built) })
	r.rt.runAccess(r.stateOf(t, access.RoleTab, "Gamma").ID, access.Request{Action: access.ActionSelect})
	r.frame()
	if sel.Peek() != "c" {
		t.Fatalf("the Select action selected %q, want c", sel.Peek())
	}
	if strings.Join(built, ",") != "a,c" {
		t.Fatalf("built %q, want a,c", strings.Join(built, ","))
	}
}

func TestTabsHighContrastTextScaleAndReducedMotion(t *testing.T) {
	for _, mode := range []struct {
		name string
		sys  SystemSettings
	}{
		{"high contrast", SystemSettings{HighContrast: true, Colors: HighContrastBlack()}},
		{"225 percent", SystemSettings{TextScale: 225}},
	} {
		sel := NewSignal("a")
		var built []string
		r := newAccessRig(func(ctx *Context) Element { return tabsDemo(sel, &built) })
		r.rt.SetSystemSettings(mode.sys)
		r.frame()
		if len(r.warnings) != 0 {
			t.Fatalf("%s: warnings %v", mode.name, r.warnings)
		}
		if mode.sys.HighContrast {
			// The disabled tab's label is GrayText, not dimmed, under high contrast.
			found := false
			for _, n := range r.rt.byID {
				if n.kind == kindText && n.textEl != nil && n.textEl.text == "Beta" {
					found = true
					// The label inherits its color from its tab, so the color it is
					// painted with is the one its parent passed down.
					if c := effectiveTextColor(n); c != r.rt.theme.Disabled {
						t.Fatalf("high contrast: the disabled tab's label is %v, want GrayText %v", c, r.rt.theme.Disabled)
					}
				}
			}
			if !found {
				t.Fatalf("no label for the disabled tab Beta")
			}
		}
		tab := accessNode(t, r, access.RoleTab, "Alpha")
		if tab.style.Height != layout.Auto {
			t.Fatalf("%s: a tab has a fixed height %v; its height must grow with its text", mode.name, tab.style.Height)
		}
	}
	for _, reduced := range []bool{true, false} {
		sel := NewSignal("a")
		var built []string
		r := newAccessRig(func(ctx *Context) Element { return tabsDemo(sel, &built) })
		r.rt.SetSystemSettings(SystemSettings{ReducedMotion: reduced})
		r.frame()
		r.focusKey(t, "settings/tab:a")
		r.press(input.KeyRight, 0) // the indicator and the label ease over 120 ms
		eased := r.rt.NeedsFrame()
		if reduced && eased {
			t.Fatalf("under reduced motion, a tab change asks for frames")
		}
		if !reduced && !eased {
			t.Fatalf("without reduced motion, a tab change asks for no frame")
		}
	}
}

// collapsibleDemo is a collapsible section with a button in its content.
func collapsibleDemo(open *Signal[bool]) Element {
	return Div().Column().Child(Collapsible("Advanced options", open,
		Button("Reset defaults").Key("reset")).Key("advanced"))
}

func TestCollapsibleOpensWithClickSpaceAndEnter(t *testing.T) {
	open := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element { return collapsibleDemo(open) })
	header := r.stateOf(t, access.RoleButton, "Advanced options")
	if !header.States.Has(access.StateExpandable) || header.States.Has(access.StateExpanded) {
		t.Fatalf("a closed header is %v, want expandable and not expanded", header.States)
	}
	if n := countRole(r.tree(), access.RoleGroup); n != 0 {
		t.Fatalf("a closed section shows %d groups; its content is not built", n)
	}
	r.clickKey(t, "advanced/header")
	if !open.Peek() {
		t.Fatalf("a click did not open the section")
	}
	r.focusKey(t, "advanced/header")
	r.press(input.KeySpace, 0)
	if open.Peek() {
		t.Fatalf("Space did not close the section")
	}
	r.press(input.KeyEnter, 0)
	if !open.Peek() {
		t.Fatalf("Enter did not open the section")
	}
	header = r.stateOf(t, access.RoleButton, "Advanced options")
	if !header.States.Has(access.StateExpanded) {
		t.Fatalf("an open header is %v, want expanded", header.States)
	}
	// The content is a group labelled by the header.
	var content *access.Node
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleGroup {
			content = n
		}
		return true
	})
	if content == nil || len(content.LabelledBy) != 1 || content.LabelledBy[0] != header.ID {
		t.Fatalf("the content is not a group labelled by the header: %+v", content)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestCollapsibleExpandAndCollapseActions(t *testing.T) {
	open := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element { return collapsibleDemo(open) })
	header := r.stateOf(t, access.RoleButton, "Advanced options")
	r.rt.runAccess(header.ID, access.Request{Action: access.ActionExpand})
	r.frame()
	if !open.Peek() {
		t.Fatalf("the Expand action did not open the section")
	}
	r.rt.runAccess(header.ID, access.Request{Action: access.ActionCollapse})
	r.frame()
	if open.Peek() {
		t.Fatalf("the Collapse action did not close the section")
	}
}

func TestCollapsibleDisabledDoesNotOpen(t *testing.T) {
	open := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Collapsible("Locked", open, Text("hidden")).Disabled(true).Key("locked")
	})
	// A disabled header is not focusable, so it is clicked where it is drawn.
	rc := r.rt.nodeRect(r.nodeByKey(t, "locked/header"))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if open.Peek() {
		t.Fatalf("a disabled section opened")
	}
	if n := r.stateOf(t, access.RoleButton, "Locked"); !n.States.Has(access.StateDisabled) {
		t.Fatalf("a disabled header is %v", n.States)
	}
}

func TestAccordionSingleOpenClosesTheOthers(t *testing.T) {
	a, b := NewSignal(true), NewSignal(false)
	var changes []string
	r := newAccessRig(func(ctx *Context) Element {
		return Accordion(
			Collapsible("First", a, Text("one")).Key("first").OnChange(func(open bool) {
				changes = append(changes, fmt.Sprint("first ", open))
			}),
			Collapsible("Second", b, Text("two")).Key("second"),
		).Key("group").SingleOpen()
	})
	r.clickKey(t, "second/header")
	if !b.Peek() || a.Peek() {
		t.Fatalf("opening Second left First=%v Second=%v, want only Second open", a.Peek(), b.Peek())
	}
	if len(changes) != 1 || changes[0] != "first false" {
		t.Fatalf("First's OnChange ran %v, want once with false", changes)
	}
	r.clickKey(t, "second/header")
	if b.Peek() {
		t.Fatalf("a click on the open section did not close it")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestAccordionIndependentByDefault(t *testing.T) {
	a, b := NewSignal(true), NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Accordion(
			Collapsible("First", a, Text("one")).Key("first"),
			Collapsible("Second", b, Text("two")).Key("second"),
		)
	})
	r.clickKey(t, "second/header")
	if !a.Peek() || !b.Peek() {
		t.Fatalf("independent sections: First=%v Second=%v, want both open", a.Peek(), b.Peek())
	}
}

func countRole(t *access.Tree, role access.Role) int {
	n := 0
	t.Walk(func(nd *access.Node) bool {
		if nd.Role == role {
			n++
		}
		return true
	})
	return n
}

// treeDemo is a tree with three top-level nodes. Docs has two children, and its
// children function is counted, so a test can see when the children are asked for.
func treeDemo(sel *Signal[string], asked *int) []TreeNode {
	return []TreeNode{
		{Key: "docs", Label: "Docs", Icon: icons.Folder, Children: func() []TreeNode {
			*asked++
			return []TreeNode{
				{Key: "docs/a", Label: "alpha.txt"},
				{Key: "docs/b", Label: "beta.txt"},
			}
		}},
		{Key: "src", Label: "Src", Children: func() []TreeNode { return nil }},
		{Key: "zeta", Label: "Zeta"},
	}
}

// treeRig builds treeDemo in a tree of height 300, and records the selections.
func treeRig(sel *Signal[string], asked *int, picked *[]string) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Tree(treeDemo(sel, asked), sel).Key("files").
			Label("Project files").Style(Style{Height: Px(300)}).
			OnChange(func(key string) { *picked = append(*picked, key) }))
	})
}

// focusTree gives keyboard focus to the tree, as Tab does.
func (r *accessRig) focusTree(t *testing.T) {
	t.Helper()
	r.focusAccess(t, access.RoleTree, "Project files")
}

func TestTreeKeysMoveSelectOpenAndClose(t *testing.T) {
	sel := NewSignal("docs")
	asked := 0
	var picked []string
	r := treeRig(sel, &asked, &picked)
	if asked != 0 {
		t.Fatalf("a closed node asked for its children %d times", asked)
	}
	r.focusTree(t)
	r.press(input.KeyRight, 0) // opens Docs; the selection stays
	if !r.stateOf(t, access.RoleTreeItem, "Docs").States.Has(access.StateExpanded) || sel.Peek() != "docs" {
		t.Fatalf("Right did not open Docs, or moved the selection to %q", sel.Peek())
	}
	if asked == 0 {
		t.Fatalf("opening Docs did not ask for its children")
	}
	r.press(input.KeyRight, 0) // an open node moves to its first child
	if sel.Peek() != "docs/a" {
		t.Fatalf("Right on an open node selected %q, want its first child", sel.Peek())
	}
	r.press(input.KeyDown, 0)
	if sel.Peek() != "docs/b" {
		t.Fatalf("Down selected %q, want beta.txt", sel.Peek())
	}
	r.press(input.KeyLeft, 0) // a leaf moves to its parent
	if sel.Peek() != "docs" {
		t.Fatalf("Left on a leaf selected %q, want its parent Docs", sel.Peek())
	}
	r.press(input.KeyLeft, 0) // an open node closes
	if r.stateOf(t, access.RoleTreeItem, "Docs").States.Has(access.StateExpanded) {
		t.Fatalf("Left on an open node did not close it")
	}
	r.press(input.KeyEnd, 0)
	if sel.Peek() != "zeta" {
		t.Fatalf("End selected %q, want the last visible node", sel.Peek())
	}
	r.press(input.KeyHome, 0)
	if sel.Peek() != "docs" {
		t.Fatalf("Home selected %q, want the first visible node", sel.Peek())
	}
	r.press(input.KeyEnter, 0) // Enter opens the selected node
	if !r.stateOf(t, access.RoleTreeItem, "Docs").States.Has(access.StateExpanded) {
		t.Fatalf("Enter did not open Docs")
	}
	r.press(input.KeySpace, 0)
	if r.stateOf(t, access.RoleTreeItem, "Docs").States.Has(access.StateExpanded) {
		t.Fatalf("Space did not close Docs")
	}
	if len(picked) == 0 || picked[len(picked)-1] != "docs" {
		t.Fatalf("OnChange ran %v, want the last selection docs", picked)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTreeAsteriskOpensSiblingsAndTypeAheadSelects(t *testing.T) {
	sel := NewSignal("src")
	asked := 0
	var picked []string
	r := treeRig(sel, &asked, &picked)
	r.focusTree(t)
	r.typeText("*") // opens the closed siblings of Src that have children: Docs
	if !r.stateOf(t, access.RoleTreeItem, "Docs").States.Has(access.StateExpanded) {
		t.Fatalf("* did not open the sibling Docs")
	}
	r.typeText("z")
	if sel.Peek() != "zeta" {
		t.Fatalf("typing z selected %q, want Zeta", sel.Peek())
	}
}

func TestTreeRowsCarryLevelsAndSetPositions(t *testing.T) {
	sel := NewSignal("docs/a")
	asked := 0
	var picked []string
	r := treeRig(sel, &asked, &picked)
	docs := r.stateOf(t, access.RoleTreeItem, "Docs")
	r.rt.runAccess(docs.ID, access.Request{Action: access.ActionExpand})
	r.frame()
	tree := r.tree()
	alpha := accesstest.Find(t, tree, access.RoleTreeItem, "alpha.txt")
	if alpha.Level != 2 || alpha.PosInSet != 1 || alpha.SetSize != 2 {
		t.Fatalf("alpha.txt has level %d and position %d of %d, want 2, 1 of 2", alpha.Level, alpha.PosInSet, alpha.SetSize)
	}
	zeta := accesstest.Find(t, tree, access.RoleTreeItem, "Zeta")
	if zeta.Level != 1 || zeta.PosInSet != 3 || zeta.SetSize != 3 {
		t.Fatalf("Zeta has level %d and position %d of %d, want 1, 3 of 3", zeta.Level, zeta.PosInSet, zeta.SetSize)
	}
	if zeta.States.Has(access.StateExpandable) {
		t.Fatalf("a leaf is expandable")
	}
	if !alpha.States.Has(access.StateSelected) || !alpha.States.Has(access.StateSelectable) {
		t.Fatalf("the selected row is %v, want selectable and selected", alpha.States)
	}
	if !docs.States.Has(access.StateExpandable) {
		t.Fatalf("Docs is %v, want expandable", docs.States)
	}
	if n := countRole(tree, access.RoleButton); n != 0 {
		t.Fatalf("%d buttons in the tree; a chevron is part of its row", n)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTreeExpandAndCollapseActionsOnItems(t *testing.T) {
	sel := NewSignal("docs")
	asked := 0
	var picked []string
	r := treeRig(sel, &asked, &picked)
	docs := r.stateOf(t, access.RoleTreeItem, "Docs")
	r.rt.runAccess(docs.ID, access.Request{Action: access.ActionExpand})
	r.frame()
	if !r.stateOf(t, access.RoleTreeItem, "Docs").States.Has(access.StateExpanded) {
		t.Fatalf("the Expand action did not open Docs")
	}
	if sel.Peek() != "docs" || len(picked) != 0 {
		t.Fatalf("the Expand action changed the selection to %q (%v)", sel.Peek(), picked)
	}
	r.rt.runAccess(docs.ID, access.Request{Action: access.ActionCollapse})
	r.frame()
	if r.stateOf(t, access.RoleTreeItem, "Docs").States.Has(access.StateExpanded) {
		t.Fatalf("the Collapse action did not close Docs")
	}
	leaf := r.stateOf(t, access.RoleTreeItem, "Zeta")
	if leaf.Has(access.ActionExpand) {
		t.Fatalf("a leaf offers Expand: %v", leaf.Actions)
	}
}

// bigTreeRig builds a tree of 10,000 top-level nodes, each with one child, and
// selects the first. Only the rows in view are built.
func bigTreeRig(sel *Signal[string], picked *[]string) *accessRig {
	nodes := make([]TreeNode, 10000)
	for i := range nodes {
		nodes[i] = TreeNode{Key: fmt.Sprintf("n%05d", i), Label: fmt.Sprintf("Node %05d", i),
			Children: func() []TreeNode { return []TreeNode{{Key: fmt.Sprintf("n%05d/c", i), Label: "child"}} }}
	}
	return newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Tree(nodes, sel).Key("big").Label("Big").
			Style(Style{Height: Px(300)}).OnChange(func(k string) { *picked = append(*picked, k) }))
	})
}

func TestTreeVirtualizesAndKeepsFocusAndSelection(t *testing.T) {
	sel := NewSignal("n00000")
	var picked []string
	r := bigTreeRig(sel, &picked)
	if built := countRole(r.tree(), access.RoleTreeItem); built == 0 || built > 40 {
		t.Fatalf("%d of 10,000 rows are built; only the rows in view should be", built)
	}
	r.focusAccess(t, access.RoleTree, "Big")
	r.press(input.KeyEnd, 0) // the last row: the tree scrolls to it
	for range 4 {
		r.frame()
	}
	if sel.Peek() != "n09999" {
		t.Fatalf("End selected %q, want the last node", sel.Peek())
	}
	tree := r.tree()
	if got := countRole(tree, access.RoleTreeItem); got == 0 || got > 40 {
		t.Fatalf("%d rows are built after scrolling; only the rows in view should be", got)
	}
	last := accesstest.Find(t, tree, access.RoleTreeItem, "Node 09999")
	if !last.States.Has(access.StateSelected) {
		t.Fatalf("the selected row is %v after scrolling, want selected", last.States)
	}
	if tree.Focus == 0 {
		t.Fatalf("no node has keyboard focus after scrolling")
	}
	// Focus goes to the highlighted row, so a screen reader reads the selection.
	if tree.Focus != last.ID {
		t.Fatalf("focus is on node %d, want the selected row %d", tree.Focus, last.ID)
	}
	if len(picked) == 0 || picked[len(picked)-1] != "n09999" {
		t.Fatalf("OnChange ran %v, want n09999 last", picked)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTreeHighContrastTextScaleAndReducedMotion(t *testing.T) {
	for _, mode := range []struct {
		name string
		sys  SystemSettings
	}{
		{"high contrast", SystemSettings{HighContrast: true, Colors: HighContrastBlack()}},
		{"225 percent", SystemSettings{TextScale: 225}},
	} {
		sel := NewSignal("docs")
		asked := 0
		var picked []string
		r := treeRig(sel, &asked, &picked)
		r.rt.SetSystemSettings(mode.sys)
		r.frame()
		if len(r.warnings) != 0 {
			t.Fatalf("%s: warnings %v", mode.name, r.warnings)
		}
		row := accessNode(t, r, access.RoleTreeItem, "Docs")
		if mode.sys.HighContrast {
			if row.div.style.Background != r.rt.theme.Accent {
				t.Fatalf("high contrast: the selected row is not Highlight")
			}
		}
		if row.style.Height != layout.Auto {
			t.Fatalf("%s: a row has a fixed height; its height must grow with its text", mode.name)
		}
	}
	for _, reduced := range []bool{true, false} {
		sel := NewSignal("docs")
		asked := 0
		var picked []string
		r := treeRig(sel, &asked, &picked)
		r.rt.SetSystemSettings(SystemSettings{ReducedMotion: reduced})
		r.frame()
		r.focusTree(t)
		r.press(input.KeyRight, 0) // the chevron turns, and the row's state changes at once
		// A tree has no animation of its own, so opening a row asks for no frame in
		// either mode: the state changes at once, as it does under reduced motion.
		if r.rt.NeedsFrame() {
			t.Fatalf("reduced motion %v: opening a row asks for frames", reduced)
		}
	}
}

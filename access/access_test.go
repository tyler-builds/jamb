package access

import (
	"strings"
	"testing"
)

// sample is a window with a button and a text field inside a group.
func sample() []*Node {
	return []*Node{
		{ID: RootID, Role: RoleWindow, Children: []ID{2, 3}},
		{ID: 2, Role: RoleGroup, Parent: RootID, Children: []ID{4}},
		{ID: 3, Role: RoleButton, Name: "Save", Parent: RootID, States: StateFocusable},
		{ID: 4, Role: RoleTextField, Name: "Search", Parent: 2, States: StateFocusable | StateFocused},
		// Not reachable from the root, so NewTree drops it.
		{ID: 9, Role: RoleButton, Name: "Orphan", Parent: 0},
	}
}

func TestNewTreeKeepsOnlyReachableNodesInReadingOrder(t *testing.T) {
	tr := NewTree(4, sample())
	if tr.Len() != 4 {
		t.Fatalf("Len = %d, want 4 (the orphan is dropped)", tr.Len())
	}
	var names []string
	tr.Walk(func(n *Node) bool {
		names = append(names, n.Role.String())
		return true
	})
	want := "window group text field button"
	if got := strings.Join(names, " "); got != want {
		t.Fatalf("reading order %q, want %q", got, want)
	}
	if tr.Node(9) != nil {
		t.Fatal("the orphan is in the tree")
	}
}

func TestNilTreeIsEmpty(t *testing.T) {
	var tr *Tree
	if tr.Node(RootID) != nil || tr.Len() != 0 {
		t.Fatal("a nil tree has nodes")
	}
	tr.Walk(func(*Node) bool { t.Fatal("walked a nil tree"); return true })
	if n := Diff(nil, NewTree(0, sample())); !n.Structure || len(n.Added) != 4 {
		t.Fatalf("diff from nil = %+v, want every node added", n)
	}
}

func TestFindByRoleAndName(t *testing.T) {
	tr := NewTree(0, sample())
	if n := tr.Find(RoleButton, "Save"); n == nil || n.ID != 3 {
		t.Fatalf("Find(button, Save) = %+v", n)
	}
	if n := tr.Find(RoleButton, "Missing"); n != nil {
		t.Fatal("found a node that does not exist")
	}
}

func TestDumpShowsRolesNamesStatesAndBounds(t *testing.T) {
	nodes := sample()
	nodes[2].Bounds = Rect{X: 12, Y: 40, W: 120, H: 34}
	nodes[3].PosInSet, nodes[3].SetSize = 2, 5
	got := NewTree(4, nodes).Dump()
	for _, want := range []string{
		"window (1)\n",
		`  group (2)` + "\n",
		`    text field "Search" [focusable focused] 2 of 5 (4)`,
		`  button "Save" [focusable] @12,40 120x34 (3)`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("dump lacks %q:\n%s", want, got)
		}
	}
}

func TestDiffReportsAddedRemovedAndChanged(t *testing.T) {
	prev := NewTree(0, sample())
	next := sample()
	next[2].Name = "Save all" // changed in place
	next = append(next[:4], &Node{ID: 5, Role: RoleButton, Name: "New", Parent: RootID})
	next[0].Children = []ID{2, 3, 5}
	cur := NewTree(4, next)

	u := Diff(prev, cur)
	if !u.Structure {
		t.Fatal("an added node is not a structure change")
	}
	if !sameIDs(u.Added, []ID{5}) {
		t.Fatalf("Added = %v, want [5]", u.Added)
	}
	// The orphan was never in prev, so nothing was removed.
	if len(u.Removed) != 0 {
		t.Fatalf("Removed = %v, want none", u.Removed)
	}
	if !contains(u.Changed, 3) || !contains(u.Changed, RootID) {
		t.Fatalf("Changed = %v, want the button and the window (whose children changed)", u.Changed)
	}
	if contains(u.Changed, 2) || contains(u.Changed, 4) {
		t.Fatalf("Changed = %v includes unchanged nodes", u.Changed)
	}
	if !u.FocusChanged || u.Focus != 4 {
		t.Fatalf("focus %d changed %v; the focus was zero before", u.Focus, u.FocusChanged)
	}
}

func TestDiffOfEqualTreesIsEmpty(t *testing.T) {
	u := Diff(NewTree(4, sample()), NewTree(4, sample()))
	if u.Structure || u.FocusChanged || len(u.Added)+len(u.Removed)+len(u.Changed) != 0 {
		t.Fatalf("equal trees produced %+v", u)
	}
}

func TestNodeEqualComparesContent(t *testing.T) {
	a := &Node{ID: 1, Name: "x", Actions: []Action{ActionClick}}
	b := &Node{ID: 1, Name: "x", Actions: []Action{ActionClick}}
	if !a.Equal(b) {
		t.Fatal("equal nodes differ")
	}
	b.Actions = nil
	if a.Equal(b) {
		t.Fatal("different actions compare equal")
	}
}

func TestStateAndRoleStrings(t *testing.T) {
	if got := (StateFocused | StateDisabled).String(); got != "focused disabled" {
		t.Fatalf("state string %q", got)
	}
	if got := RoleCheckBox.String(); got != "check box" {
		t.Fatalf("role %q", got)
	}
	if got := ActionScrollIntoView.String(); got != "scroll into view" {
		t.Fatalf("action %q", got)
	}
	if got := Role(200).String(); !strings.HasPrefix(got, "role(") {
		t.Fatalf("out-of-range role %q", got)
	}
}

func TestInteractiveRoles(t *testing.T) {
	for _, r := range []Role{RoleButton, RoleTextField, RoleLink, RoleMenuItem} {
		if !r.Interactive() {
			t.Errorf("%s is not interactive", r)
		}
	}
	for _, r := range []Role{RoleGroup, RoleStaticText, RoleList, RoleDialog} {
		if r.Interactive() {
			t.Errorf("%s is interactive", r)
		}
	}
}

func TestBridgeActivatesOnceAndPublishes(t *testing.T) {
	b := NewBridge()
	if b.Active() || b.Tree() != nil {
		t.Fatal("a new bridge is active or has a tree")
	}
	activations := 0
	b.SetHooks(func() { activations++ }, nil)
	b.Activate()
	b.Activate()
	if activations != 1 || !b.Active() {
		t.Fatalf("activations = %d, active = %v", activations, b.Active())
	}

	var seen []Update
	b.Listen(func(u Update) { seen = append(seen, u) })
	tr := NewTree(0, sample())
	b.Publish(tr, Diff(nil, tr))
	if b.Tree() != tr {
		t.Fatal("Tree does not return the published snapshot")
	}
	if len(seen) != 1 || !seen[0].Structure {
		t.Fatalf("listener saw %+v", seen)
	}
}

func TestBridgePerformsActionsThroughTheRuntime(t *testing.T) {
	b := NewBridge()
	var gotID ID
	var gotAction Action
	b.SetHooks(nil, func(id ID, a Action) { gotID, gotAction = id, a })
	b.Perform(7, ActionFocus)
	if gotID != 7 || gotAction != ActionFocus {
		t.Fatalf("perform got %d %v", gotID, gotAction)
	}
}

func sameIDs(a, b []ID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(ids []ID, id ID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

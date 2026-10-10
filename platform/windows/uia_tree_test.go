package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// TestTreeControlTypes checks that a tree and its items map to the Tree and
// TreeItem control types, with the identifiers of UIAutomationClient.h.
func TestTreeControlTypes(t *testing.T) {
	if ctTree != 50023 || ctTreeItem != 50024 {
		t.Fatalf("Tree and TreeItem = %d and %d, want 50023 and 50024", ctTree, ctTreeItem)
	}
	if got := controlType(access.RoleTree); got != ctTree {
		t.Errorf("controlType(tree) = %d, want %d", got, ctTree)
	}
	if got := controlType(access.RoleTreeItem); got != ctTreeItem {
		t.Errorf("controlType(tree item) = %d, want %d", got, ctTreeItem)
	}
}

// TestTreeItemExpandCollapseStates checks that every tree item supports the
// ExpandCollapse pattern: a node with children reports Collapsed or Expanded, and
// a node with none reports LeafNode, as UI Automation requires.
func TestTreeItemExpandCollapseStates(t *testing.T) {
	leaf := &access.Node{ID: 2, Role: access.RoleTreeItem, Name: "readme.txt"}
	closed := &access.Node{ID: 3, Role: access.RoleTreeItem, Name: "src", States: access.StateExpandable}
	open := &access.Node{ID: 4, Role: access.RoleTreeItem, Name: "docs",
		States: access.StateExpandable | access.StateExpanded}

	if !supports(kindExpand, leaf) {
		t.Errorf("a leaf tree item does not support ExpandCollapse")
	}
	cases := []struct {
		n    *access.Node
		want int32
	}{
		{leaf, expandCollapseLeafNode},
		{closed, expandCollapseCollapsed},
		{open, expandCollapseExpanded},
	}
	for _, c := range cases {
		if v := nodeProperty(propExpandCollapseState, c.n, false); v.vt != vtI4 || variantI4Of(v) != c.want {
			t.Errorf("ExpandCollapseState of %q = %d, want %d", c.n.Name, variantI4Of(v), c.want)
		}
	}
}

// TestExpandCollapseLeafIsNotAStateOfOtherControls checks that a control that is
// not a tree item and cannot expand has no ExpandCollapse pattern, so the leaf
// state does not leak into other roles.
func TestExpandCollapseLeafIsNotAStateOfOtherControls(t *testing.T) {
	button := &access.Node{ID: 5, Role: access.RoleButton, Name: "Save"}
	if supports(kindExpand, button) {
		t.Errorf("a button without an expanded state reports ExpandCollapse")
	}
	if v := nodeProperty(propExpandCollapseState, button, false); v.vt != vtEmpty {
		t.Errorf("a button reports an ExpandCollapseState of tag %d", v.vt)
	}
}

package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// annotatedTree is a tree of Docs (with two files, described by a count), Src (described,
// a leaf) and Zeta (plain). menuOf is the menu the tree builds for a key; nil leaves the
// tree without a menu. picked records the keys the menu was built for.
func annotatedTree(sel *Signal[string], picked *[]string, menu bool, empty bool) *accessRig {
	nodes := []TreeNode{
		{Key: "docs", Label: "Docs", Description: "2 files", Children: func() []TreeNode {
			return []TreeNode{
				{Key: "docs/a", Label: "alpha.txt"},
				{Key: "docs/b", Label: "beta.txt"},
			}
		}},
		{Key: "src", Label: "Src", Description: "source", Children: func() []TreeNode { return nil }},
		{Key: "zeta", Label: "Zeta"},
	}
	return newAccessRig(func(ctx *Context) Element {
		tv := Tree(nodes, sel).Key("files").Label("Project files").
			RenderTrailing(func(n TreeNode, s PartState) Element {
				if n.Key == "docs" {
					return Text("2")
				}
				return Div()
			})
		if menu {
			tv.ContextMenu(func(key string) []MenuItem {
				*picked = append(*picked, key)
				if empty {
					return nil
				}
				return []MenuItem{{Label: "Copy name " + key, OnClick: func() {}}, {Label: "Show in history", OnClick: func() {}}}
			})
		}
		return Div().Column().Child(tv)
	})
}

func TestTreeNodeDescriptionIsTheRowsDescription(t *testing.T) {
	sel := NewSignal("")
	var picked []string
	r := annotatedTree(sel, &picked, false, false)
	if got := r.stateOf(t, access.RoleTreeItem, "Docs").Description; got != "2 files" {
		t.Fatalf("Docs has description %q, want \"2 files\"", got)
	}
	if got := r.stateOf(t, access.RoleTreeItem, "Zeta").Description; got != "" {
		t.Fatalf("Zeta has description %q, want none", got)
	}
	if len(partNodes(r.rt, "Tree.Description")) != 2 {
		t.Fatalf("%d description parts, want 2 (Docs and Src)", len(partNodes(r.rt, "Tree.Description")))
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTreeRenderTrailingSitsOnEveryRowAndIsHidden(t *testing.T) {
	sel := NewSignal("")
	var picked []string
	r := annotatedTree(sel, &picked, false, false)
	if n := len(partNodes(r.rt, "Tree.Trailing")); n != 3 {
		t.Fatalf("%d trailing parts, want one per row (3)", n)
	}
	// The trailing content is for sighted users: the count is not among the rows' names.
	r.tree().Walk(func(n *access.Node) bool {
		if n.Name == "2" {
			t.Fatalf("the trailing count is exposed to assistive technology")
		}
		return true
	})
}

func TestTreeRightClickSelectsTheRowAndOpensItsMenu(t *testing.T) {
	sel := NewSignal("docs")
	var picked []string
	r := annotatedTree(sel, &picked, true, false)
	n := keyNode(t, r.rt, "src")
	rc := r.rt.nodeRect(n)
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonRight, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonRight, 0)
	r.frame()
	if sel.Peek() != "src" {
		t.Fatalf("right click selected %q, want src", sel.Peek())
	}
	if r.rt.ctxMenu == nil || !slices.Equal(picked, []string{"src"}) {
		t.Fatalf("menu open %v, built for %v; want an open menu built for src", r.rt.ctxMenu != nil, picked)
	}
	if len(r.rt.ctxMenu.items) != 2 || r.rt.ctxMenu.items[0].Label != "Copy name src" {
		t.Fatalf("menu items %v", r.rt.ctxMenu.items)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTreeShiftF10OpensTheMenuOfTheSelectedRow(t *testing.T) {
	sel := NewSignal("docs")
	var picked []string
	r := annotatedTree(sel, &picked, true, false)
	r.focusTree(t)
	r.press(input.KeyDown, 0) // docs is closed, so Down selects Src
	if sel.Peek() != "src" {
		t.Fatalf("Down selected %q, want src", sel.Peek())
	}
	r.press(input.KeyF10, input.ModShift)
	if r.rt.ctxMenu == nil || !slices.Equal(picked, []string{"src"}) {
		t.Fatalf("Shift+F10 built the menu for %v, want src", picked)
	}
}

func TestTreeMenuKeyOpensTheMenuOfTheSelectedRow(t *testing.T) {
	sel := NewSignal("zeta")
	var picked []string
	r := annotatedTree(sel, &picked, true, false)
	r.focusTree(t)
	r.press(input.KeyMenu, 0)
	if r.rt.ctxMenu == nil || !slices.Equal(picked, []string{"zeta"}) {
		t.Fatalf("the Menu key built the menu for %v, want zeta", picked)
	}
}

func TestTreeWithoutContextMenuOpensNothing(t *testing.T) {
	sel := NewSignal("src")
	var picked []string
	r := annotatedTree(sel, &picked, false, false)
	r.focusTree(t)
	r.press(input.KeyF10, input.ModShift)
	if r.rt.ctxMenu != nil {
		t.Fatalf("a menu opened on a tree without ContextMenu")
	}
}

func TestTreeEmptyMenuOpensNothing(t *testing.T) {
	sel := NewSignal("src")
	var picked []string
	r := annotatedTree(sel, &picked, true, true)
	r.focusTree(t)
	r.press(input.KeyF10, input.ModShift)
	if r.rt.ctxMenu != nil {
		t.Fatalf("an empty menu opened")
	}
}

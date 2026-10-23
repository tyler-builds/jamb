package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// partTree is a tree with a folder that has a child, and a file. The folder is
// "docs" and its child "guide"; sel is the selected key, st the instance style,
// and row and label are the render slots, when they are not nil.
func partTree(sel *Signal[string], st TreeStyle, row, label func(TreeNode, PartState) Element) Element {
	nodes := []TreeNode{
		{Key: "docs", Label: "Docs", Icon: icons.Check, Children: func() []TreeNode {
			return []TreeNode{{Key: "guide", Label: "Guide"}}
		}},
		{Key: "src", Label: "Source", Description: "Code"},
	}
	tv := Tree(nodes, sel).Key("files").Label("Project files").Styles(st)
	if row != nil {
		tv.RenderRow(row)
	}
	if label != nil {
		tv.RenderLabel(label)
	}
	return Div().Column().Child(tv)
}

// anyMarked reports whether a node drawn for part carries the marker background
// (or, with text, the marker text color). A tree draws one node per row, so a
// part is checked on every node that draws it.
func anyMarked(rt *Runtime, part string, text bool) bool {
	for _, n := range partNodes(rt, part) {
		if text && n.style.TextColor == markFG || !text && n.style.Background == markBG {
			return true
		}
	}
	return false
}

func testTreeParts(t *testing.T, applied map[string]bool) {
	type treeCase struct {
		part  string
		set   func(*TreeStyle, Style)
		sel   string // the selected key
		drawn string // the element that draws the part
		text  bool
		act   func(*testing.T, *accessRig)
	}
	cases := []treeCase{
		{part: "Tree.Row", set: func(s *TreeStyle, m Style) { s.Row = m }, sel: "docs", drawn: "Tree.Row"},
		{part: "Tree.RowHover", set: func(s *TreeStyle, m Style) { s.RowHover = m }, sel: "docs", drawn: "Tree.Row",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "src")) }},
		{part: "Tree.RowSelected", set: func(s *TreeStyle, m Style) { s.RowSelected = m }, sel: "src", drawn: "Tree.Row"},
		{part: "Tree.Chevron", set: func(s *TreeStyle, m Style) { s.Chevron = m }, sel: "src", drawn: "Tree.Chevron"},
		{part: "Tree.Icon", set: func(s *TreeStyle, m Style) { s.Icon = m }, sel: "src", drawn: "Tree.Icon"},
		{part: "Tree.Label", set: func(s *TreeStyle, m Style) { s.Label = m }, sel: "src", drawn: "Tree.Label"},
		{part: "Tree.Description", set: func(s *TreeStyle, m Style) { s.Description = m }, sel: "src", drawn: "Tree.Description", text: true},
		{part: "Tree.TreeFocusVisible", set: func(s *TreeStyle, m Style) { s.TreeFocusVisible = m }, sel: "src", drawn: "Tree.Tree",
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "Tree.Tree", "")) }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st TreeStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			sel := NewSignal(c.sel)
			r := newAccessRig(func(ctx *Context) Element { return partTree(sel, st, nil, nil) })
			if c.act != nil {
				c.act(t, r)
			}
			if len(partNodes(r.rt, c.drawn)) == 0 {
				t.Fatalf("%s: no node draws %s", c.part, c.drawn)
			}
			if !anyMarked(r.rt, c.drawn, c.text) {
				t.Errorf("%s: the style does not reach a node that draws %s", c.part, c.drawn)
			}
		})
	}
}

// TestTreeIndentAndSelectedRowIsNotBold checks that the Indent size sets the indent of
// each level, and that the selected row's label keeps the default weight.
func TestTreeIndentAndSelectedRowIsNotBold(t *testing.T) {
	sel := NewSignal("docs")
	r := newAccessRig(func(ctx *Context) Element { return partTree(sel, TreeStyle{Indent: 30}, nil, nil) })
	r.focusKey(t, "files")
	r.press(input.KeyRight, 0) // opens "docs" and shows its child
	child := keyNode(t, r.rt, "guide")
	if got, want := child.style.Padding.Left, float32(4+30); got != want {
		t.Errorf("level 2 indent: left padding %v, want %v", got, want)
	}
	for _, n := range partNodes(r.rt, "Tree.Label") {
		if n.parent != nil && n.parent.key == "docs" && n.style.FontWeight != 0 {
			t.Errorf("the selected row's label has weight %v, want the default", n.style.FontWeight)
		}
	}
}

// TestTreeRenderSlotsKeepBehaviorAndNames checks the label slot: the row keeps
// its key, its accessible name and its selection, the slot's content is not in
// the access tree, and the slot is called again with the hovered state.
func TestTreeRenderSlotsKeepBehaviorAndNames(t *testing.T) {
	sel := NewSignal("docs")
	var calls int
	var last PartState
	label := func(n TreeNode, s PartState) Element {
		calls++
		last = s
		return Text("label " + n.Label)
	}
	r := newAccessRig(func(ctx *Context) Element { return partTree(sel, TreeStyle{}, nil, label) })
	r.focusKey(t, "files")
	r.press(input.KeyRight, 0) // opens "docs", so its child is built
	accessNode(t, r, access.RoleTreeItem, "Docs")
	r.tree().Walk(func(n *access.Node) bool {
		if n.Name == "label Docs" {
			t.Errorf("the slot's content is in the access tree as %q", n.Name)
		}
		return true
	})
	if n := keyNode(t, r.rt, "guide"); n.key != "guide" {
		t.Errorf("the child row lost its key")
	}
	before := calls
	hoverNode(r, keyNode(t, r.rt, "src"))
	if calls == before {
		t.Fatalf("the label slot was not called again on hover")
	}
	if !last.Hovered {
		t.Errorf("the slot's last state has Hovered false, want true")
	}
	r.focusKey(t, "files")
	r.press(input.KeyDown, 0) // the next visible row is the open folder's child
	if sel.Peek() != "guide" {
		t.Fatalf("Down selected %q, want guide", sel.Peek())
	}
}

// TestTreeUnderHighContrast checks the guardrails: under high contrast the
// selected row is Highlight with HighlightText on it, an app color does not
// reach the row, and the focus ring of the tree stays.
func TestTreeUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Tree = TreeStyle{RowSelected: Style{Background: markBG, TextColor: markFG}, Indent: 12}
	sel := NewSignal("src")
	r := newThemedRig(&theme, func(ctx *Context) Element { return partTree(sel, TreeStyle{}, nil, nil) })
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	n := keyNode(t, r.rt, "src")
	if n.style.Background != th.Accent {
		t.Errorf("high contrast: the selected row is %v, want Highlight %v", n.style.Background, th.Accent)
	}
	if got := effectiveTextColor(n); got != th.AccentText {
		t.Errorf("high contrast: the selected row's text is %v, want HighlightText %v", got, th.AccentText)
	}
	focusNode(r, partNode(t, r.rt, "Tree.Tree", ""))
	if len(partNode(t, r.rt, "Tree.Tree", "").style.Shadows) == 0 {
		t.Errorf("high contrast: the focused tree has no focus ring")
	}
}

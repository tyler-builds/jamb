//go:build windows

package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// TestHotPathsAllocateNothing guards the calls that UI Automation makes on every
// node of a walk. A client walking the tree makes thousands of them a second,
// so each allocation there becomes garbage the process keeps until its garbage
// collector runs. The budgets are zero.
func TestHotPathsAllocateNothing(t *testing.T) {
	b := testTree(0,
		&access.Node{ID: access.RootID, Role: access.RoleWindow, Children: []access.ID{2, 3}},
		node(2, access.RootID, 4),
		&access.Node{ID: 3, Role: access.RoleButton, Parent: access.RootID, Actions: []access.Action{access.ActionClick}},
		node(4, 2))
	root := cachedObject(kindRoot, access.RootID, b, 0x7f20, iidRawElementProviderSimple)
	defer releaseCall(root)
	rc := coreOf(root)
	nodeView := cachedObject(kindNode, 2, b, 0x7f20, iidRawElementProviderSimple)
	defer releaseCall(nodeView)
	nc := coreOf(nodeView)
	buttonView := cachedObject(kindNode, 3, b, 0x7f20, iidRawElementProviderSimple)
	defer releaseCall(buttonView)
	bc := coreOf(buttonView)
	if rc == nil || nc == nil {
		t.Fatal("no cores")
	}
	cases := map[string]func(){
		"root current":             func() { rc.current() },
		"node current":             func() { nc.current() },
		"root navigate first":      func() { rc.navigate(navFirstChild) },
		"node navigate next":       func() { nc.navigate(navNextSibling) },
		"node navigate parent":     func() { nc.navigate(navParent) },
		"node property ctype":      func() { nc.property(propControlType) },
		"node focused":             func() { nc.focused() },
		"node runtime id copy":     func() { runtimeIDOf(2) },
		"node without the pattern": func() { nc.pattern(patternInvoke) },
		"node pattern, cached":     func() { bc.pattern(patternInvoke) },
		"node toggle state":        func() { nc.toggleState() },
		"node expand state":        func() { nc.expandState() },
		"node is selected":         func() { nc.isSelected() },
		"node scroll state":        func() { nc.scrollState() },
		"node state property":      func() { nc.property(propToggleState) },
	}
	for name, fn := range cases {
		if n := testing.AllocsPerRun(100, fn); n != 0 {
			t.Errorf("%s allocates %.1f times per call, want 0", name, n)
		}
	}
}

// TestTextHotPathsAllocateNothing holds the text pattern's calls that run per
// query to no Go allocation: the pattern lookup, the state of the field and of a
// range, and the movements and comparisons of a range. Making a range object is
// not on this list, since the object's core is a Go allocation.
func TestTextHotPathsAllocateNothing(t *testing.T) {
	b := textBridge("hello world")
	const hwnd = 0x7f21
	patternView := cachedObject(kindNode, 2, b, hwnd, iidRawElementProviderSimple)
	defer releaseCall(patternView)
	fieldNode := coreOf(patternView)
	textView := cachedObject(kindText, 2, b, hwnd, iidTextProvider)
	defer releaseCall(textView)
	textCore := coreOf(textView)
	info := b.Tree().Node(2).Text
	docView := newTextRange(hwnd, b, 2, info.Whole(), iidTextRangeProvider)
	defer releaseCall(docView)
	docCore := coreOf(docView)
	doc, _ := docCore.textRange()
	cases := map[string]func(){
		"node text pattern, cached": func() { fieldNode.pattern(patternText) },
		"text provider state":       func() { textCore.textOf() },
		"range state":               func() { docCore.textRange() },
		"range move by character":   func() { doc.Move(access.UnitCharacter, 2) },
		"range move by word":        func() { doc.Move(access.UnitWord, 1) },
		"range expand to line":      func() { doc.Expand(access.UnitLine) },
		"range move endpoint":       func() { doc.MoveEndpoint(true, access.UnitWord, -1) },
		"range compare endpoints":   func() { doc.CompareEndpoints(false, doc, true) },
		"range find":                func() { doc.Find("world", false, true) },
	}
	for name, fn := range cases {
		if n := testing.AllocsPerRun(100, fn); n != 0 {
			t.Errorf("%s allocates %.1f times per call, want 0", name, n)
		}
	}
}

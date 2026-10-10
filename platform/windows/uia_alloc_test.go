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
		node(2, access.RootID, 4), node(3, access.RootID), node(4, 2))
	root := cachedObject(kindRoot, access.RootID, b, 0x7f20, iidRawElementProviderSimple)
	defer releaseCall(root)
	rc := coreOf(root)
	nodeView := cachedObject(kindNode, 2, b, 0x7f20, iidRawElementProviderSimple)
	defer releaseCall(nodeView)
	nc := coreOf(nodeView)
	if rc == nil || nc == nil {
		t.Fatal("no cores")
	}
	cases := map[string]func(){
		"root current":         func() { rc.current() },
		"node current":         func() { nc.current() },
		"root navigate first":  func() { rc.navigate(navFirstChild) },
		"node navigate next":   func() { nc.navigate(navNextSibling) },
		"node navigate parent": func() { nc.navigate(navParent) },
		"node property ctype":  func() { nc.property(propControlType) },
		"node focused":         func() { nc.focused() },
		"node runtime id copy": func() { runtimeIDOf(2) },
	}
	for name, fn := range cases {
		if n := testing.AllocsPerRun(100, fn); n != 0 {
			t.Errorf("%s allocates %.1f times per call, want 0", name, n)
		}
	}
}

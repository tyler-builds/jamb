// Package accesstest has helpers for tests that check an access tree: find a
// node by its role and name, and assert its role, name and states.
package accesstest

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// Find returns the first node, in reading order, with the role and name, and
// fails the test if there is none. The failure message lists the tree.
func Find(tb testing.TB, t *access.Tree, role access.Role, name string) *access.Node {
	tb.Helper()
	n := t.Find(role, name)
	if n == nil {
		tb.Fatalf("no %s named %q in the access tree:\n%s", role, name, t.Dump())
	}
	return n
}

// Absent fails the test if a node with the role and name exists.
func Absent(tb testing.TB, t *access.Tree, role access.Role, name string) {
	tb.Helper()
	if n := t.Find(role, name); n != nil {
		tb.Fatalf("unexpected %s named %q (ID %d) in the access tree:\n%s", role, name, n.ID, t.Dump())
	}
}

// Check asserts a node's role, and its name when name is not empty, and that
// it has all the states in want.
func Check(tb testing.TB, n *access.Node, role access.Role, name string, want access.State) {
	tb.Helper()
	if n == nil {
		tb.Fatal("node is nil")
	}
	if n.Role != role {
		tb.Errorf("node %d: role is %s, want %s", n.ID, n.Role, role)
	}
	if name != "" && n.Name != name {
		tb.Errorf("node %d: name is %q, want %q", n.ID, n.Name, name)
	}
	if !n.States.Has(want) {
		tb.Errorf("node %d: states are [%s], want [%s] set", n.ID, n.States, want)
	}
}

// Focused asserts that the tree's focus is on the node with the role and name.
func Focused(tb testing.TB, t *access.Tree, role access.Role, name string) {
	tb.Helper()
	n := Find(tb, t, role, name)
	if t.Focus != n.ID {
		tb.Fatalf("focus is on node %d, want %s %q (node %d)", t.Focus, role, name, n.ID)
	}
}

// Children returns the names of a node's children, in order, for asserting
// reading order. Children without a name are returned as their role.
func Children(t *access.Tree, n *access.Node) []string {
	var out []string
	for _, id := range n.Children {
		c := t.Node(id)
		if c == nil {
			continue
		}
		if c.Name != "" {
			out = append(out, c.Name)
		} else {
			out = append(out, c.Role.String())
		}
	}
	return out
}

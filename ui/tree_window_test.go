package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// TestTreeWindowsRowsInAColumn checks that a Tree with no size, in a column that
// is as tall as the window, builds the rows in view and not the 10,000 open rows
// under it: the tree's column may shrink below its rows (see Tree).
func TestTreeWindowsRowsInAColumn(t *testing.T) {
	rt, _ := treeBenchApp(100, 100)
	if n := rt.Stats().Nodes; n > 1000 {
		t.Errorf("a tree of 10,100 open rows built %d nodes, want the rows in view", n)
	}
}

// TestDataGridWindowsRowsInAColumn checks the same for a DataGrid with no size, in a
// column as tall as the window.
func TestDataGridWindowsRowsInAColumn(t *testing.T) {
	rows := dgItems(5000)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Size(Full, Full).Column().Child(Text("head")).
			Child(DataGrid(rows, func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs"))
	})
	dgSettle(r)
	if n := len(partNodes(r.rt, "DataGrid.Row")); n > 100 {
		t.Errorf("a grid of 5,000 rows built %d rows, want the rows in view", n)
	}
	if g := r.stateOf(t, access.RoleDataGrid, "Runs"); g.GridRows != 5000 {
		t.Errorf("the grid reports %d rows, want all 5,000", g.GridRows)
	}
}

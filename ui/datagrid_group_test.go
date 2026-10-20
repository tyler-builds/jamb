package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// dgGroupColumns are four columns: ID, then Street and City in the group Address,
// then Status, which grows.
func dgGroupColumns() []DataColumn[dgItem] {
	text := func(x dgItem) string { return x.ID }
	return []DataColumn[dgItem]{
		{Header: "ID", Text: text, Width: 60},
		{Header: "Street", Text: text, Width: 100, Group: "Address", Resizable: true},
		{Header: "City", Text: text, Width: 80, Group: "Address"},
		{Header: "Status", Text: text, Width: 80, Grow: 1},
	}
}

// dgGroupRig returns a rig with a grid of the columns cols.
func dgGroupRig(cols []DataColumn[dgItem]) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(5), func(x dgItem) string { return x.ID }, cols...).Label("Runs").
			Selection(SelectionSingle, nil)
	})
}

// dgGroupCells returns the cells of the groups of columns.
func dgGroupCells(r *accessRig) []*node { return partNodes(r.rt, "DataGrid.HeaderGroup") }

// TestDataGridHeaderGroupSpansItsColumns checks that a group's cell spans its
// columns, before and after one of them is resized, and that the rows start below
// the two rows of the header.
func TestDataGridHeaderGroupSpansItsColumns(t *testing.T) {
	r := dgGroupRig(dgGroupColumns())
	dgSettle(r)
	check := func(when string) {
		t.Helper()
		gs := dgGroupCells(r)
		if len(gs) != 1 {
			t.Fatalf("%s: %d group cells, want 1", when, len(gs))
		}
		g := r.rt.nodeRect(gs[0])
		s, c := r.rt.nodeRect(dgHeader(r, "Street")), r.rt.nodeRect(dgHeader(r, "City"))
		if g.X != s.X || g.X+g.W != c.X+c.W {
			t.Errorf("%s: the Address cell spans %v..%v, its columns %v..%v", when, g.X, g.X+g.W, s.X, c.X+c.W)
		}
		if g.Y >= s.Y {
			t.Errorf("%s: the Address cell is at %v, not above the header cells at %v", when, g.Y, s.Y)
		}
	}
	check("at first")
	focusNode(r, dgBody(t, r))
	r.press(input.KeyUp, 0)
	r.press(input.KeyRight, 0) // Street
	r.press(input.KeyRight, input.ModCtrl)
	r.press(input.KeyRight, input.ModCtrl)
	dgSettle(r)
	check("after Street was widened")
	body := dgBody(t, r)
	header := r.rt.nodeRect(dgHeader(r, "ID"))
	row := r.rt.nodeRect(dgCell(r, 0, 0))
	if row.Y < header.Y+header.H {
		t.Errorf("the first row is at %v, under the header that ends at %v", row.Y, header.Y+header.H)
	}
	if got, want := body.style.Padding.Top, 2*dataGridRowHeight(DataGridComfortable); got != want {
		t.Errorf("the body's top padding is %v, want the two rows of the header, %v", got, want)
	}
}

// TestDataGridHeaderGroupIsNotAStop checks that the keyboard does not reach the row
// of the groups: Up from the header cells stays on them.
func TestDataGridHeaderGroupIsNotAStop(t *testing.T) {
	r := dgGroupRig(dgGroupColumns())
	focusNode(r, dgBody(t, r))
	r.press(input.KeyUp, 0)
	r.press(input.KeyRight, 0)
	r.press(input.KeyUp, 0)
	if h := dgHeader(r, "Street"); h == nil || !h.div.acc.highlighted {
		t.Error("Up on a header cell left it")
	}
	r.press(input.KeyDown, 0)
	if f := dgFocusCell(r); f == nil || f.div.acc.gridRow != 1 || f.div.acc.gridCol != 2 {
		t.Errorf("Down from the header focused %+v, want row 1 of Street", f)
	}
}

// TestDataGridHeaderGroupAccess checks that assistive technology sees one header
// for each column, described by its group, and no cell of the groups.
func TestDataGridHeaderGroupAccess(t *testing.T) {
	r := dgGroupRig(dgGroupColumns())
	tree := r.tree()
	headers := 0
	var walk func(id access.ID)
	walk = func(id access.ID) {
		n := tree.Node(id)
		if n.Role == access.RoleHeaderItem {
			headers++
		}
		if n.Name == "Address" {
			t.Errorf("the group is in the access tree as %s %q", n.Role, n.Name)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(tree.Root)
	if headers != 4 {
		t.Errorf("the tree has %d header items, want 4", headers)
	}
	if h := r.stateOf(t, access.RoleHeaderItem, "City"); h.Description != "Address" {
		t.Errorf("the City header is described %q, want Address", h.Description)
	}
	if h := r.stateOf(t, access.RoleHeaderItem, "ID"); h.Description != "" {
		t.Errorf("the ID header is described %q, want nothing", h.Description)
	}
}

// TestDataGridHeaderGroupSplitByAPin checks that a group whose columns are pinned
// apart is drawn in pieces, one in each region.
func TestDataGridHeaderGroupSplitByAPin(t *testing.T) {
	cols := dgGroupColumns()
	cols[1].Pin = PinStart
	cols[3].Width = 800 // wider than the grid, so that it scrolls
	r := dgGroupRig(cols)
	dgScrollX(r, dgBody(t, r), 200)
	if dgBody(t, r).st.scrollX == 0 {
		t.Fatal("the grid did not scroll")
	}
	gs := dgGroupCells(r)
	if len(gs) != 2 {
		t.Fatalf("a group split by a pin has %d cells, want 2", len(gs))
	}
	s, c := r.rt.nodeRect(dgHeader(r, "Street")), r.rt.nodeRect(dgHeader(r, "City"))
	for _, g := range gs {
		rc := r.rt.nodeRect(g)
		if !(rc.X == s.X && rc.W == s.W) && !(rc.X == c.X && rc.W == c.W) {
			t.Errorf("a piece of the group is at %v..%v, over neither Street nor City", rc.X, rc.X+rc.W)
		}
	}
}

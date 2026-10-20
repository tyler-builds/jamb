package ui

import (
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// dgManyColumns returns n columns, each 100 wide, named C0, C1 and so on, whose
// cells show the column and the row. The first is editable.
func dgManyColumns(n int) []DataColumn[dgItem] {
	cols := make([]DataColumn[dgItem], n)
	for i := range cols {
		name := "C" + strconv.Itoa(i)
		cols[i] = DataColumn[dgItem]{Header: name, Width: 100,
			Text: func(x dgItem) string { return name + "/" + x.ID }}
	}
	cols[0].Edit = func(dgItem, string) error { return nil }
	return cols
}

// dgCellsOfRow returns the number of the built cells of row q (0-based).
func dgCellsOfRow(r *accessRig, q int) int {
	n := 0
	for _, c := range partNodes(r.rt, "DataGrid.Cell") {
		if c.div.acc != nil && c.div.acc.gridRow == q+1 {
			n++
		}
	}
	return n
}

// dgManyRig returns a rig with a grid of 20 rows and the columns cols.
func dgManyRig(cols []DataColumn[dgItem]) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(20), func(x dgItem) string { return x.ID }, cols...).Label("Runs").
			Selection(SelectionSingle, nil)
	})
}

// TestDataGridBuildsOnlyColumnsInView checks that a grid of 200 columns builds the
// cells of the columns in view, and reports all 200 columns and headers.
func TestDataGridBuildsOnlyColumnsInView(t *testing.T) {
	r := dgManyRig(dgManyColumns(200))
	dgSettle(r)
	if n := dgCellsOfRow(r, 0); n == 0 || n > 12 {
		t.Errorf("row 0 built %d cells of 200 columns, want the ones in view and a few more", n)
	}
	if n := len(partNodes(r.rt, "DataGrid.HeaderCell")); n != 200 {
		t.Errorf("the header built %d cells, want all 200", n)
	}
	g := r.stateOf(t, access.RoleDataGrid, "Runs")
	if g.GridColumns != 200 {
		t.Errorf("the grid reports %d columns, want 200", g.GridColumns)
	}
	// The row's name is still the text of every column.
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.div.key == "i0" && n.div.acc.name == "" {
			t.Error("row i0 has no name")
		}
	}
	// The row keeps its width: the cells not built are spacers.
	row := partNodes(r.rt, "DataGrid.Row")[0]
	if row.lay.W != 20000 {
		t.Errorf("the row is %v wide, want the 20000 of its columns", row.lay.W)
	}
}

// TestDataGridSidewaysScrollBuildsTheColumns checks that a sideways scroll builds the
// columns that come into view, at their places.
func TestDataGridSidewaysScrollBuildsTheColumns(t *testing.T) {
	r := dgManyRig(dgManyColumns(200))
	body := dgBody(t, r)
	dgScrollX(r, body, 10000)
	c := dgCell(r, 0, 102)
	if c == nil {
		t.Fatal("after a scroll to 10000 the cell of column 103 is not built")
	}
	left, _ := dgRows(r, body)
	if got, want := r.rt.nodeRect(c).X, left+10200-body.st.scrollX; got != want {
		t.Errorf("column 103 is at %v, want %v", got, want)
	}
	if dgCell(r, 0, 5) != nil {
		t.Error("column 6, far out of view, is still built")
	}
	// A press in view lands in the column under the pointer.
	rc := r.rt.nodeRect(c)
	dgClick(r, c, 0)
	if f := dgFocusCell(r); f == nil || f.div.acc.gridCol != 103 {
		t.Errorf("a press on column 103 at %v focused %+v", rc.X, f)
	}
}

// TestDataGridKeepsTheCursorAndEditColumns checks that the cell of the cursor and an
// editing cell stay built when their column scrolls out of view.
func TestDataGridKeepsTheCursorAndEditColumns(t *testing.T) {
	r := dgManyRig(dgManyColumns(200))
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyF2, 0) // edit column 1 of row 1
	body := dgBody(t, r)
	dgScrollX(r, body, 12000)
	if dgCell(r, 1, 0) == nil {
		t.Fatal("the editing cell was dropped when its column scrolled out of view")
	}
	if len(partNodes(r.rt, "DataGrid.Editor.Text")) != 1 {
		t.Error("the editor did not survive the scroll")
	}
	if dgCell(r, 1, 120) == nil {
		t.Error("the columns in view were not built")
	}
	r.press(input.KeyEscape, 0)
	dgScrollX(r, body, 12000)
	if f := dgFocusCell(r); f == nil || f.div.acc.gridCol != 1 || f.div.acc.gridRow != 2 {
		t.Errorf("the cursor's cell is not built out of view: %+v", f)
	}
	r.press(input.KeyEnd, 0)
	dgSettle(r)
	if f := dgFocusCell(r); f == nil || f.div.acc.gridCol != 200 {
		t.Errorf("End did not reach the last column: %+v", f)
	}
}

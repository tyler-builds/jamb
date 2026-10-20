package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// dgColumnsRig returns a rig with the grid of items whose columns are in the order
// kept in order, with the widths in ws, and each change of their layout counted in
// changes.
func dgColumnsRig(items []dgItem, order *Signal[[]string], ws *Signal[map[string]float32],
	changes *[]ColumnState, cols ...DataColumn[dgItem]) *accessRig {
	if len(cols) == 0 {
		cols = dgColumns()
	}
	return newAccessRig(func(ctx *Context) Element {
		g := DataGrid(items, func(x dgItem) string { return x.ID }, cols...).Label("Runs").
			Selection(SelectionSingle, nil).ColumnOrder(order).ColumnWidths(ws)
		if changes != nil {
			g.OnColumnsChange(func(s ColumnState) { *changes = append(*changes, s) })
		}
		return g
	})
}

// dgCellText returns the text of the cell at row and column (0-based), or "" when
// it is not built.
func dgCellText(r *accessRig, row, col int) string {
	if c := dgCell(r, row, col); c != nil {
		return c.div.acc.name
	}
	return ""
}

// TestDataGridColumnOrderIsApplied checks that ColumnOrder shows the columns in its
// order, and that the cells are numbered for assistive technology in that order.
func TestDataGridColumnOrderIsApplied(t *testing.T) {
	r := dgColumnsRig(dgItems(5), NewSignal([]string{"N", "Name"}), nil, nil)
	if got := dgCellText(r, 1, 0); got != "1" {
		t.Errorf("column 1 of row 1 is %q, want the N column's 1", got)
	}
	if got := dgCellText(r, 1, 1); got != "Item 000001" {
		t.Errorf("column 2 of row 1 is %q, want the Name column's text", got)
	}
	n, name := r.rt.nodeRect(dgHeader(r, "N")), r.rt.nodeRect(dgHeader(r, "Name"))
	if n.X >= name.X {
		t.Errorf("the N header is at %v and the Name header at %v; want N first", n.X, name.X)
	}
}

// TestDataGridColumnOrderDropsUnknownKeys checks that a key that names no column is
// dropped, and that the columns the order does not name follow in their given order.
func TestDataGridColumnOrderDropsUnknownKeys(t *testing.T) {
	cols := append(dgColumns(), DataColumn[dgItem]{Header: "ID", Text: func(x dgItem) string { return x.ID }})
	r := dgColumnsRig(dgItems(3), NewSignal([]string{"gone", "ID"}), nil, nil, cols...)
	for c, want := range []string{"i0", "Item 000000", "0"} {
		if got := dgCellText(r, 0, c); got != want {
			t.Errorf("column %d of row 0 is %q, want %q", c+1, got, want)
		}
	}
}

// TestDataGridCursorFollowsAMovedColumn checks that the focused cell, and an open
// edit, stay on their column when the order of the columns changes.
func TestDataGridCursorFollowsAMovedColumn(t *testing.T) {
	order := NewSignal([]string(nil))
	cols := dgColumns()
	cols[1].Edit = func(dgItem, string) error { return nil }
	r := dgColumnsRig(dgItems(5), order, nil, nil, cols...)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyRight, 0) // the N column
	order.Set([]string{"N", "Name"})
	r.frame()
	if f := dgFocusCell(r); f == nil || f.div.acc.gridCol != 1 || f.div.acc.name != "1" {
		t.Fatalf("after N moved first the focused cell is %v; want row 1 of N, now column 1", f)
	}
	r.press(input.KeyF2, 0)
	order.Set([]string{"Name", "N"})
	r.frame()
	if n := partNodes(r.rt, "DataGrid.Editor.Text"); len(n) != 1 {
		t.Fatalf("after N moved back the grid has %d editors, want 1", len(n))
	}
	if c := dgCell(r, 1, 1); c == nil || len(partNodesUnder(c, "DataGrid.Editor.Text")) != 1 {
		t.Error("the editor did not follow the N column to column 2")
	}
}

// partNodesUnder returns the nodes of the part under n.
func partNodesUnder(n *node, part string) []*node {
	var out []*node
	var walk func(n *node)
	walk = func(n *node) {
		for _, c := range n.children {
			if c.div != nil && c.div.part == part {
				out = append(out, c)
			}
			walk(c)
		}
	}
	walk(n)
	return out
}

// TestDataGridResizeDragReportsOnce checks that a drag of a column's edge reports
// the layout of the columns once, when it ends, and that each keyboard step and an
// auto-fit report it too.
func TestDataGridResizeDragReportsOnce(t *testing.T) {
	var changes []ColumnState
	ws := NewSignal(map[string]float32(nil))
	r := dgColumnsRig(dgItems(5), nil, ws, &changes)
	rc := r.rt.nodeRect(dgResizeHandle(t, r, "N"))
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	for d := float32(5); d <= 30; d += 5 {
		r.rt.PointerMove(x+d, y, 0)
		r.frame()
	}
	if len(changes) != 0 {
		t.Errorf("the drag reported %d changes before it ended, want none", len(changes))
	}
	r.rt.PointerUp(x+30, y, input.ButtonLeft, 0)
	r.frame()
	if len(changes) != 1 {
		t.Fatalf("the drag reported %d changes, want 1", len(changes))
	}
	if got := changes[0].Widths["N"]; got != 90 {
		t.Errorf("the change has N %v wide, want 90", got)
	}
	if got := changes[0].Order; len(got) != 2 || got[0] != "Name" || got[1] != "N" {
		t.Errorf("the change has the order %v, want [Name N]", got)
	}
	if got := ws.Peek()["N"]; got != 90 {
		t.Errorf("the width signal has N %v wide, want 90", got)
	}
	// A press on the edge that does not move it reports nothing.
	rc = r.rt.nodeRect(dgResizeHandle(t, r, "N"))
	x, y = rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
	if len(changes) != 1 {
		t.Errorf("a press that did not move the edge reported a change (%d in all)", len(changes))
	}
	focusNode(r, dgBody(t, r))
	r.press(input.KeyUp, 0) // the header
	r.press(input.KeyRight, 0)
	r.press(input.KeyRight, input.ModCtrl)
	r.press(input.KeyRight, input.ModCtrl)
	if len(changes) != 3 {
		t.Errorf("two keyboard steps gave %d changes in all, want 3", len(changes))
	}
}

// TestDataGridColumnWidthsSignalIsApplied checks that a width set in the signal of
// ColumnWidths sizes its column, then and later.
func TestDataGridColumnWidthsSignalIsApplied(t *testing.T) {
	ws := NewSignal(map[string]float32{"N": 200})
	r := dgColumnsRig(dgItems(3), nil, ws, nil)
	if got := dgHeader(r, "N").div.style.Basis.Value; got != 200 {
		t.Errorf("the N header is %v wide, want the 200 of the signal", got)
	}
	ws.Set(map[string]float32{"N": 70})
	r.frame()
	if got := dgHeader(r, "N").div.style.Basis.Value; got != 70 {
		t.Errorf("after the signal changed the N header is %v wide, want 70", got)
	}
}

// TestDataGridResizeReleaseDoesNotSort checks that the release of a drag of a
// sortable column's edge, which lands on its header cell, does not sort, and that
// a click on the header cell after it still does.
func TestDataGridResizeReleaseDoesNotSort(t *testing.T) {
	sort := NewSignal(SortState{})
	r := dgRig(dgItems(5), SelectionSingle, nil, sort, nil)
	rc := r.rt.nodeRect(dgResizeHandle(t, r, "N"))
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(x-20, y, 0)
	r.frame()
	r.rt.PointerUp(x-20, y, input.ButtonLeft, 0)
	r.frame()
	if got := sort.Peek(); got.Key != "" {
		t.Fatalf("releasing the edge sorted by %q", got.Key)
	}
	rc = r.rt.nodeRect(dgResizeHandle(t, r, "N"))
	x, y = rc.X-15, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
	if got := sort.Peek(); got.Key != "N" {
		t.Errorf("a click on the header after the resize sorted by %q, want N", got.Key)
	}
}

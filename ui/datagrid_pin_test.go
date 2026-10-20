package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"

	"github.com/tyler-builds/jamb/input"
)

// dgPinColumns are five columns wider than the grid: Z pinned to the end, given
// first, P pinned to the start, and A, B and C that scroll between them.
func dgPinColumns() []DataColumn[dgItem] {
	text := func(x dgItem) string { return x.ID }
	return []DataColumn[dgItem]{
		{Header: "Z", Text: func(x dgItem) string { return "z" + x.ID }, Width: 80, Pin: PinEnd},
		{Header: "A", Text: text, Width: 200, Movable: true},
		{Header: "P", Text: func(x dgItem) string { return "p" + x.ID }, Width: 100, Pin: PinStart, Movable: true},
		{Header: "B", Text: text, Width: 200, Movable: true},
		{Header: "C", Text: text, Width: 200, Movable: true},
	}
}

// dgPinRig returns a rig with a striped grid of the pinned columns, selecting in sel.
func dgPinRig(sel *Signal[[]string]) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(10), func(x dgItem) string { return x.ID }, dgPinColumns()...).Label("Runs").
			Selection(SelectionMultiple, sel).Striped()
	})
}

// dgScrollX scrolls the grid's body sideways to x and lets the scroll come to rest.
func dgScrollX(r *accessRig, body *node, x float32) {
	body.st.targetX = x
	r.rt.hostInvalidate()
	dgSettle(r)
}

// dgRows returns the left and right edges of the rows that the body shows.
func dgRows(r *accessRig, body *node) (left, right float32) {
	b := r.rt.nodeRect(body)
	be := body.style.borderEdges()
	return b.X + be.Left + body.style.Padding.Left, b.X + b.W - be.Right - body.style.Padding.Right
}

// TestDataGridPinnedColumnsOrderAndAccess checks that the start-pinned columns come
// first and the end-pinned last, and that the cells are numbered in that order.
func TestDataGridPinnedColumnsOrderAndAccess(t *testing.T) {
	r := dgPinRig(nil)
	for c, want := range []string{"pi0", "i0", "i0", "i0", "zi0"} {
		if got := dgCellText(r, 0, c); got != want {
			t.Errorf("column %d of row 0 is %q, want %q", c+1, got, want)
		}
	}
	if g := r.stateOf(t, access.RoleDataGrid, "Runs"); g.GridColumns != 5 {
		t.Errorf("the grid has %d columns, want 5", g.GridColumns)
	}
}

// TestDataGridPinnedCellsStayInView checks that the pinned cells and header cells
// keep their place on the screen as the other columns scroll sideways.
func TestDataGridPinnedCellsStayInView(t *testing.T) {
	r := dgPinRig(nil)
	body := dgBody(t, r)
	left, right := dgRows(r, body)
	for _, x := range []float32{0, 120, 1e6} {
		dgScrollX(r, body, x)
		if body.st.scrollX == 0 && x > 0 {
			t.Fatalf("the grid did not scroll to %v", x)
		}
		for _, n := range []*node{dgCell(r, 2, 0), dgHeader(r, "P")} {
			if got := r.rt.nodeRect(n).X; got != left {
				t.Errorf("at scroll %v a start-pinned cell is at %v, want %v", body.st.scrollX, got, left)
			}
		}
		for _, n := range []*node{dgCell(r, 2, 4), dgHeader(r, "Z")} {
			rc := r.rt.nodeRect(n)
			if got := rc.X + rc.W; got != right {
				t.Errorf("at scroll %v an end-pinned cell ends at %v, want %v", body.st.scrollX, got, right)
			}
		}
	}
}

// TestDataGridPressOnAPinnedCell checks that a press on a pinned cell, drawn over
// the cells that scroll under it, lands in the pinned column.
func TestDataGridPressOnAPinnedCell(t *testing.T) {
	r := dgPinRig(NewSignal([]string(nil)))
	body := dgBody(t, r)
	dgScrollX(r, body, 150)
	for _, c := range []int{0, 4} {
		dgClick(r, dgCell(r, 3, c), 0)
		f := dgFocusCell(r)
		if f == nil || f.div.acc.gridCol != c+1 || f.div.acc.gridRow != 4 {
			t.Errorf("a press on the pinned cell of row 3, column %d, focused %+v", c+1, f)
		}
	}
	// The cell under the middle of the view is a scrolling one.
	left, right := dgRows(r, body)
	y := r.rt.nodeRect(dgCell(r, 3, 0)).Y + 5
	r.clickAt((left+right)/2, y)
	if f := dgFocusCell(r); f == nil || f.div.acc.gridCol < 2 || f.div.acc.gridCol > 4 {
		t.Errorf("a press in the middle of the view focused %+v, want a column that scrolls", f)
	}
}

// TestDataGridRevealStopsAtThePinnedColumns checks that a column that the cursor
// moves to is scrolled into the part of the view between the pinned columns.
func TestDataGridRevealStopsAtThePinnedColumns(t *testing.T) {
	r := dgPinRig(nil)
	body := dgBody(t, r)
	focusNode(r, dgBody(t, r))
	left, right := dgRows(r, body)
	startR := left + 100
	endL := right - 80
	for range 4 {
		r.press(input.KeyRight, 0)
		dgSettle(r)
		f := dgFocusCell(r)
		if f == nil {
			t.Fatal("no focused cell")
		}
		c := f.div.acc.gridCol - 1
		if c == 0 || c == 4 {
			continue
		}
		rc := r.rt.nodeRect(f)
		if rc.X < startR-0.5 || rc.X+rc.W > endL+0.5 {
			t.Errorf("column %d is at %v..%v, under the pinned columns (%v..%v free)", c+1, rc.X, rc.X+rc.W, startR, endL)
		}
	}
	for range 4 {
		r.press(input.KeyLeft, 0)
		dgSettle(r)
	}
	if body.st.targetX != 0 {
		t.Errorf("back at the first column the grid is scrolled to %v, want 0", body.st.targetX)
	}
}

// TestDataGridPinnedCellsShowTheRowState checks that a pinned cell is filled with
// the grid's background and then its row's, so that a selected or striped row shows
// through it, and that the cells over which it is drawn do not.
func TestDataGridPinnedCellsShowTheRowState(t *testing.T) {
	r := dgPinRig(NewSignal([]string{"i2"}))
	dgScrollX(r, dgBody(t, r), 150)
	st := resolveDataGrid(&Context{rt: r.rt}, DataGridComfortable, 1, DataGridStyle{})
	for _, q := range []int{1, 2} { // striped, selected
		cell := dgCell(r, q, 0)
		row := cell.parent
		b := r.rt.px(cell.abs)
		var fills []Color
		for _, qd := range r.rt.scene.Quads {
			if qd.Bounds == b {
				fills = append(fills, Color{qd.Background.R, qd.Background.G, qd.Background.B, qd.Background.A})
			}
		}
		if len(fills) < 2 || fills[0].scene() != st.Grid.Background.scene() || fills[1].scene() != row.shownBG.scene() {
			t.Errorf("row %d: the pinned cell is filled with %v, want the grid's %v and then the row's %v",
				q, fills, st.Grid.Background, row.shownBG)
		}
		if row.shownBG.A == 0 {
			t.Errorf("row %d has no fill to show", q)
		}
	}
}

// TestDataGridPinnedColumnsMoveInTheirRegion checks that a pinned column moves only
// among the columns of its pin.
func TestDataGridPinnedColumnsMoveInTheirRegion(t *testing.T) {
	var changes []ColumnState
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(3), func(x dgItem) string { return x.ID }, dgPinColumns()...).Label("Runs").
			OnColumnsChange(func(s ColumnState) { changes = append(changes, s) })
	})
	focusNode(r, dgBody(t, r))
	r.press(input.KeyUp, 0) // the header of P
	r.press(input.KeyRight, input.ModCtrl|input.ModShift)
	if len(changes) != 0 {
		t.Errorf("the start-pinned P moved into the columns that scroll: %v", changes)
	}
	r.press(input.KeyRight, 0)
	r.press(input.KeyLeft, input.ModCtrl|input.ModShift)
	if len(changes) != 0 {
		t.Errorf("A moved into the start-pinned columns: %v", changes)
	}
	r.press(input.KeyRight, input.ModCtrl|input.ModShift)
	if len(changes) != 1 {
		t.Errorf("A did not move among the columns that scroll: %v", changes)
	}
}

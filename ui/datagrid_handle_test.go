package ui

import "testing"

// dgHandleRig returns a rig with the grid of items connected to h, sorted by sort.
func dgHandleRig(items []dgItem, h *DataGridHandle, sort *Signal[SortState], cols ...DataColumn[dgItem]) *accessRig {
	if len(cols) == 0 {
		cols = dgColumns()
	}
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(items, func(x dgItem) string { return x.ID }, cols...).Label("Runs").
			Selection(SelectionSingle, nil).Sort(sort).Handle(h)
	})
}

// dgSettle runs frames until the grid's scroll comes to rest.
func dgSettle(r *accessRig) {
	for range 40 {
		r.frame()
	}
}

// TestDataGridHandleScrollsToARowAfterASort checks that ScrollToRow finds the row by
// its key in the sorted order, and shows it.
func TestDataGridHandleScrollsToARowAfterASort(t *testing.T) {
	h := NewDataGridHandle()
	sort := NewSignal(SortState{Key: "Name", Desc: true})
	r := dgHandleRig(dgItems(1000), h, sort)
	h.ScrollToRow("i10") // near the end of the descending order
	dgSettle(r)
	keys := dgRowKeys(r)
	found := false
	for _, k := range keys {
		found = found || k == "i10"
	}
	if !found {
		t.Fatalf("after ScrollToRow(i10) the built rows are %v; want i10 among them", keys)
	}
	body := r.rt.nodeRect(dgBody(t, r))
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.div.key == "i10" {
			if rc := r.rt.nodeRect(n); rc.Y < body.Y || rc.Y+rc.H > body.Y+body.H {
				t.Errorf("row i10 is at %v..%v, outside the body %v..%v", rc.Y, rc.Y+rc.H, body.Y, body.Y+body.H)
			}
		}
	}
}

// TestDataGridHandleScrollsToAColumn checks that ScrollToColumn scrolls the grid
// sideways to a column out of view, and that a key that names nothing does nothing.
func TestDataGridHandleScrollsToAColumn(t *testing.T) {
	name := func(x dgItem) string { return x.Name }
	cols := []DataColumn[dgItem]{
		{Header: "A", Text: name, Width: 300},
		{Header: "B", Text: name, Width: 300},
		{Header: "C", Text: name, Width: 300},
	}
	h := NewDataGridHandle()
	r := dgHandleRig(dgItems(10), h, nil, cols...)
	h.ScrollToColumn("nothing")
	h.ScrollToRow("nothing")
	dgSettle(r)
	body := dgBody(t, r)
	if body.st.targetX != 0 || body.st.targetY != 0 {
		t.Fatalf("unknown keys scrolled the grid to %v, %v", body.st.targetX, body.st.targetY)
	}
	h.ScrollToColumn("C")
	dgSettle(r)
	if want := float32(900) - body.lay.W; body.st.targetX != want {
		t.Errorf("ScrollToColumn(C) scrolled to %v, want %v (the end of C at the right edge)", body.st.targetX, want)
	}
}

// TestDataGridHandleFocusesACell checks that FocusCell moves the cursor to the cell,
// gives the grid focus, and leaves the selection as it is.
func TestDataGridHandleFocusesACell(t *testing.T) {
	h := NewDataGridHandle()
	r := dgHandleRig(dgItems(100), h, nil)
	h.FocusCell("i60", "N")
	dgSettle(r)
	if r.rt.focusID != dgBody(t, r).id {
		t.Error("FocusCell did not focus the grid")
	}
	f := dgFocusCell(r)
	if f == nil || f.div.acc.gridRow != 61 || f.div.acc.gridCol != 2 {
		t.Fatalf("after FocusCell(i60, N) the focused cell is %+v, want row 61, column 2", f)
	}
	h.FocusCell("gone", "Name")
	dgSettle(r)
	if f := dgFocusCell(r); f == nil || f.div.acc.gridRow != 61 {
		t.Error("FocusCell with an unknown row moved the cursor")
	}
}

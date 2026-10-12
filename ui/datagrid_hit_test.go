package ui

import (
	"fmt"
	"testing"
)

// dgHitColumns are three columns whose middle one grows, so the columns after it
// are laid out further right than their base widths say.
func dgHitColumns() []DataColumn[dgItem] {
	name := func(x dgItem) string { return x.Name }
	return []DataColumn[dgItem]{
		{Header: "Run", Text: func(x dgItem) string { return x.ID }, Width: 100},
		{Header: "Service", Text: name, Width: 100, Grow: 1},
		{Header: "Status", Text: name, Width: 100},
	}
}

// dgShowCell scrolls the body of the grid sideways until the centre of the cell of
// row 0 in column c is inside the body, and reports whether it is.
func dgShowCell(t *testing.T, r *accessRig, c int) bool {
	body := r.rt.nodeRect(dgBody(t, r))
	for range 40 {
		cr := r.rt.nodeRect(dgCell(r, 0, c))
		x := cr.X + cr.W/2
		if x >= body.X && x <= body.X+body.W {
			return true
		}
		dir := float32(60)
		if x < body.X {
			dir = -60
		}
		r.rt.Scroll(body.X+body.W/2, body.Y+body.H/2, dir, 0, true, 0)
		r.frame()
	}
	return false
}

// dgPressEachColumn presses the cell of row 0 in each column and checks that the
// cursor lands in that column.
func dgPressEachColumn(t *testing.T, r *accessRig, cols []DataColumn[dgItem], label string) {
	t.Helper()
	for c := range cols {
		if !dgShowCell(t, r, c) {
			t.Fatalf("%s: column %d cannot be brought into view", label, c)
		}
		dgClick(r, dgCell(r, 0, c), 0)
		f := dgFocusCell(r)
		if f == nil {
			t.Fatalf("%s: no cell focused after a press on column %d", label, c)
		}
		if got := f.div.acc.gridCol - 1; got != c {
			t.Errorf("%s: a press on column %d focused column %d", label, c, got)
		}
	}
}

// TestDataGridPressLandsInTheCellUnderThePointer checks that a press selects the
// column the pointer is over, as the cells are laid out: a Grow column takes the
// spare width, so the cells after it start further right than their base widths.
// It is checked at the Text size setting and at 225 percent.
func TestDataGridPressLandsInTheCellUnderThePointer(t *testing.T) {
	for _, scale := range []int{100, 225} {
		r := dgRig(dgItems(3), SelectionSingle, NewSignal([]string(nil)), NewSignal(SortState{}), nil, dgHitColumns()...)
		r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
		r.rt.SetViewport(900, 400, 1) // the 225 percent columns fit without a scroll
		r.frame()
		dgPressEachColumn(t, r, dgHitColumns(), fmt.Sprintf("scale %d", scale))
	}
}

// TestDataGridPressAfterScrollingAndResizing checks the press on the cells of a grid
// whose columns are wider than the viewport, so that it scrolls sideways, and then
// on the cells of the same grid with its columns narrowed.
func TestDataGridPressAfterScrollingAndResizing(t *testing.T) {
	cols := dgHitColumns()
	cols[0].Width, cols[1].Width, cols[2].Width = 260, 260, 260 // wider than the viewport
	r := dgRig(dgItems(3), SelectionSingle, NewSignal([]string(nil)), NewSignal(SortState{}), nil, cols...)
	dgPressEachColumn(t, r, cols, "wide columns")
	// Narrower columns move the cells after them, and the Grow column takes the rest.
	cols[0].Width, cols[1].Width, cols[2].Width = 60, 40, 300
	r = dgRig(dgItems(3), SelectionSingle, NewSignal([]string(nil)), NewSignal(SortState{}), nil, cols...)
	dgPressEachColumn(t, r, cols, "narrow columns")
}

// TestDataGridHeaderCellFillsTheHeader checks that a header cell is as tall as the
// header row less its bottom border, so that its hover and sorted fills cover the
// whole cell, as a row's cells do, and not a box around the text.
func TestDataGridHeaderCellFillsTheHeader(t *testing.T) {
	cols := dgHitColumns()
	cols[2].Compare = SortBy(func(x dgItem) string { return x.Name })
	r := dgRig(dgItems(3), SelectionNone, nil, NewSignal(SortState{}), nil, cols...)
	header := partNodes(r.rt, "DataGrid.Header")
	if len(header) != 1 {
		t.Fatalf("want one header row, have %d", len(header))
	}
	hdr := r.rt.nodeRect(header[0])
	cell := dgHeader(r, "Status")
	if cell == nil {
		t.Fatal("no Status header cell")
	}
	rc := r.rt.nodeRect(cell)
	if want := hdr.H - 1; rc.H != want {
		t.Errorf("Status header cell is %v tall, want %v (the header row less its border)", rc.H, want)
	}
	if rc.Y != hdr.Y {
		t.Errorf("Status header cell starts at %v, the header row at %v", rc.Y, hdr.Y)
	}
}

package ui

import "testing"

// TestDataGridFillsTheViewportWhenNarrow checks that the header and the rows fill
// the grid when its columns total less, up to the gutter kept for the scrollbar,
// so that the grid has no gap at its right edge; and that they are as wide as the
// columns when those are wider.
func TestDataGridFillsTheViewportWhenNarrow(t *testing.T) {
	r := dgRig(dgItems(20), SelectionNone, nil, NewSignal(SortState{}), nil)
	for range 3 {
		r.frame()
	}
	body := dgBody(t, r)
	want := body.lay.W - 2 - dataGridGutter // the border is 1 px on each side
	if w := partNodes(r.rt, "DataGrid.Header")[0].lay.W; w != want {
		t.Errorf("header is %v wide in a %v-wide grid, want %v (less the scrollbar gutter)", w, body.lay.W, want)
	}
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.lay.W != want {
			t.Errorf("row %q is %v wide in a %v-wide grid, want %v", n.div.key, n.lay.W, body.lay.W, want)
			break
		}
	}

	wide := []DataColumn[dgItem]{
		{Header: "A", Text: func(x dgItem) string { return x.Name }, Width: 400},
		{Header: "B", Text: func(x dgItem) string { return x.Name }, Width: 400},
	}
	r = dgRig(dgItems(20), SelectionNone, nil, NewSignal(SortState{}), nil, wide...)
	for range 3 {
		r.frame()
	}
	if w := partNodes(r.rt, "DataGrid.Header")[0].lay.W; w != 800 {
		t.Errorf("header is %v wide, want the 800 of its columns", w)
	}
}

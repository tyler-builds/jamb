package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// dgCellRig returns a rig with a grid of n items that selects cells in the mode m,
// keeping them in cells, and sorts by sort.
func dgCellRig(n int, m SelectionMode, cells *Signal[[]CellKey], sort *Signal[SortState]) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(n), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").
			CellSelection(m, cells).Sort(sort)
	})
}

// dgCellNames returns the cells as "row/column", sorted, for comparing.
func dgCellNames(cells []CellKey) string {
	var out []string
	for _, c := range cells {
		out = append(out, c.Row+"/"+c.Column)
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

// TestDataGridCellSelectionByKeyboard checks the keys of a cell selection: a plain
// move selects the cell, Ctrl moves without selecting, Shift extends a rectangle
// from the anchor, Shift+Space selects the row's cells, Ctrl+Space the column's,
// and Ctrl+A every cell.
func TestDataGridCellSelectionByKeyboard(t *testing.T) {
	cells := NewSignal([]CellKey(nil))
	r := dgCellRig(4, SelectionMultiple, cells, NewSignal(SortState{}))
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	if got := dgCellNames(cells.Peek()); got != "i1/Name" {
		t.Errorf("Down selected %q, want i1/Name", got)
	}
	r.press(input.KeyRight, input.ModShift)
	r.press(input.KeyDown, input.ModShift)
	if got := dgCellNames(cells.Peek()); got != "i1/N i1/Name i2/N i2/Name" {
		t.Errorf("Shift+Right and Shift+Down selected %q, want the rectangle of rows 1 and 2", got)
	}
	r.press(input.KeyUp, input.ModCtrl)
	if got := len(cells.Peek()); got != 4 {
		t.Errorf("Ctrl+Up changed the selection to %d cells", got)
	}
	r.press(input.KeySpace, input.ModCtrl)
	if got := dgCellNames(cells.Peek()); got != "i0/N i1/N i2/N i3/N" {
		t.Errorf("Ctrl+Space selected %q, want the cells of the N column", got)
	}
	r.press(input.KeySpace, input.ModShift)
	if got := dgCellNames(cells.Peek()); got != "i1/N i1/Name" {
		t.Errorf("Shift+Space selected %q, want the cells of row 1", got)
	}
	r.press(input.KeyA, input.ModCtrl)
	if got := len(cells.Peek()); got != 8 {
		t.Errorf("Ctrl+A selected %d cells, want all 8", got)
	}
	r.press(input.KeyHome, 0)
	if got := dgCellNames(cells.Peek()); got != "i1/Name" {
		t.Errorf("Home selected %q, want i1/Name", got)
	}
	r.press(input.KeyEnd, input.ModShift)
	if got := dgCellNames(cells.Peek()); got != "i1/N i1/Name" {
		t.Errorf("Shift+End selected %q, want the row from the anchor", got)
	}
}

// TestDataGridCellSelectionByMouse checks that a click selects a cell, Ctrl+click
// toggles one, Shift+click selects the rectangle from the anchor, and a click on a
// header still sorts.
func TestDataGridCellSelectionByMouse(t *testing.T) {
	cells := NewSignal([]CellKey(nil))
	sort := NewSignal(SortState{})
	r := dgCellRig(5, SelectionMultiple, cells, sort)
	dgClick(r, dgCell(r, 1, 0), 0)
	dgClick(r, dgCell(r, 3, 1), input.ModShift)
	if got := dgCellNames(cells.Peek()); got != "i1/N i1/Name i2/N i2/Name i3/N i3/Name" {
		t.Errorf("Shift+click selected %q, want the rectangle from i1/Name to i3/N", got)
	}
	dgClick(r, dgCell(r, 2, 0), input.ModCtrl)
	if got := len(cells.Peek()); got != 5 {
		t.Errorf("Ctrl+click on a selected cell left %d cells, want 5", got)
	}
	dgClick(r, dgCell(r, 4, 1), 0)
	if got := dgCellNames(cells.Peek()); got != "i4/N" {
		t.Errorf("a click selected %q, want i4/N", got)
	}
	dgClick(r, dgHeader(r, "N"), 0)
	if got := sort.Peek(); got != (SortState{Key: "N"}) {
		t.Errorf("a click on the header sorted %+v, want by N", got)
	}
	if got := dgCellNames(cells.Peek()); got != "i4/N" {
		t.Errorf("a click on the header changed the cells to %q", got)
	}
}

// TestDataGridCellSelectionFollowsTheKeys checks that the selected cells stay on their
// rows through a sort, and that assistive technology sees the cells, not the rows,
// as the items of the selection.
func TestDataGridCellSelectionFollowsTheKeys(t *testing.T) {
	cells := NewSignal([]CellKey{{Row: "i2", Column: "N"}})
	sort := NewSignal(SortState{})
	r := dgCellRig(6, SelectionMultiple, cells, sort)
	sort.Set(SortState{Key: "Name", Desc: true})
	r.frame()
	tree := r.tree()
	var selected []string
	for _, nd := range partNodes(r.rt, "DataGrid.Cell") {
		if nd.div.acc.selected {
			selected = append(selected, nd.div.acc.name)
		}
	}
	if len(selected) != 1 {
		t.Fatalf("after the sort %d cells are selected, want 1", len(selected))
	}
	n := accessNodeFor(r, dgCell(r, 3, 1)) // i2 is the fourth of six in descending order
	if n == nil || !n.States.Has(access.StateSelectable) || !n.States.Has(access.StateSelected) {
		t.Errorf("the cell of i2 in N is not a selected item: %+v", n)
	}
	for _, nd := range partNodes(r.rt, "DataGrid.Row") {
		if a := tree.Node(accID(nd.id)); a != nil && a.States.Has(access.StateSelectable) {
			t.Errorf("row %q is an item of the selection", nd.div.key)
			break
		}
	}
	g := r.stateOf(t, access.RoleDataGrid, "Runs")
	if !g.States.Has(access.StateSelectionContainer) || !g.States.Has(access.StateMultiSelectable) {
		t.Errorf("the grid is not a multiple selection: %v", g.States)
	}
}

// TestDataGridCellSelectionFromAssistiveTechnology checks that the Select and
// AddToSelection actions of a cell select it, and that a cell selected that way
// shows CellSelected.
func TestDataGridCellSelectionFromAssistiveTechnology(t *testing.T) {
	cells := NewSignal([]CellKey(nil))
	r := dgCellRig(4, SelectionMultiple, cells, NewSignal(SortState{}))
	r.rt.runAccess(accID(dgCell(r, 1, 0).id), access.Request{Action: access.ActionSelect})
	r.frame()
	r.rt.runAccess(accID(dgCell(r, 2, 1).id), access.Request{Action: access.ActionAddToSelection})
	r.frame()
	if got := dgCellNames(cells.Peek()); got != "i1/Name i2/N" {
		t.Errorf("Select and AddToSelection selected %q, want i1/Name and i2/N", got)
	}
	st := resolveDataGrid(&Context{rt: r.rt}, DataGridComfortable, 1, DataGridStyle{})
	if got := dgCell(r, 2, 1).style.Background; got != st.CellSelected.Background {
		t.Errorf("a selected cell has the background %v, want CellSelected's %v", got, st.CellSelected.Background)
	}
}

// accessNodeFor returns the published access node of n, or nil.
func accessNodeFor(r *accessRig, n *node) *access.Node {
	if n == nil {
		return nil
	}
	return r.tree().Node(accID(n.id))
}

// TestDataGridCellSelectionReplacesSelection checks that the later of Selection and
// CellSelection applies, with a warning.
func TestDataGridCellSelectionReplacesSelection(t *testing.T) {
	rows := NewSignal([]string(nil))
	cells := NewSignal([]CellKey(nil))
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(3), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").
			Selection(SelectionMultiple, rows).CellSelection(SelectionSingle, cells)
	})
	dgClick(r, dgCell(r, 1, 1), 0)
	if len(rows.Peek()) != 0 || dgCellNames(cells.Peek()) != "i1/N" {
		t.Errorf("rows %v and cells %v after a click, want the cell only", rows.Peek(), cells.Peek())
	}
	warned := false
	for _, w := range r.warnings {
		warned = warned || strings.Contains(w, "CellSelection")
	}
	if !warned {
		t.Errorf("no warning about both selections: %v", r.warnings)
	}
}

package ui_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// headersRig is a results grid of 12 rows with row headers, whose column headers select.
func headersRig(t *testing.T) *gridRig {
	return newGridRig(t, orders(12), func(g *ui.DataGridElement[order]) *ui.DataGridElement[order] {
		return g.RowHeaders().HeaderSelects()
	})
}

// header returns the header item named name: a column's header, or a row's number.
func (r *gridRig) header(name string) *uitest.Element { return r.ByRole(access.RoleHeaderItem, name) }

// drag presses the left button on from, moves to to and releases it there.
func (r *gridRig) drag(from, to *uitest.Element, mods input.Modifiers) {
	a, b := from.Bounds(), to.Bounds()
	rt := r.Runtime()
	rt.PointerMove(a.X+a.W/2, a.Y+a.H/2, mods)
	rt.PointerDown(a.X+a.W/2, a.Y+a.H/2, input.ButtonLeft, 1, mods)
	r.Frame()
	rt.PointerMove(b.X+b.W/2, b.Y+b.H/2, mods)
	r.Frame()
	rt.PointerUp(b.X+b.W/2, b.Y+b.H/2, input.ButtonLeft, mods)
	r.Frame()
}

// rowsSelected returns the 1-based rows that are selected whole, as "1 3", from the
// cells of the access tree: each of the four columns selected.
func rowsSelected(u *uitest.UI) string {
	n := map[int]int{}
	for _, c := range strings.Fields(selectedCells(u)) {
		var row, col int
		if _, err := fmt.Sscanf(c, "%d/%d", &row, &col); err == nil {
			n[row]++
		}
	}
	var out []string
	for row := 1; row <= 20; row++ {
		if n[row] == 4 {
			out = append(out, strconv.Itoa(row))
		}
	}
	return strings.Join(out, " ")
}

// A click on a row's header selects the row, Ctrl+click adds a row and takes it out
// again, Shift+click selects the rows from the anchor, and a drag down the headers
// selects the rows it crosses.
func TestDataGridRowHeadersSelectRows(t *testing.T) {
	r := headersRig(t)
	r.clickWith(r.header("3"), 0)
	if got := rowsSelected(r.UI); got != "3" {
		t.Fatalf("a click on row 3's header selected rows %q, want 3", got)
	}
	if !r.header("3").Selected() || r.header("4").Selected() {
		t.Error("row 3's header does not report its row selected")
	}
	r.clickWith(r.header("6"), input.ModCtrl)
	if got := rowsSelected(r.UI); got != "3 6" {
		t.Errorf("Ctrl+click on row 6's header selected rows %q, want 3 6", got)
	}
	r.clickWith(r.header("8"), input.ModCtrl|input.ModShift)
	if got := rowsSelected(r.UI); got != "3 6 7 8" {
		t.Errorf("Ctrl+Shift+click on row 8's header selected rows %q, want 3 6 7 8", got)
	}
	r.clickWith(r.header("3"), input.ModCtrl)
	if got := rowsSelected(r.UI); got != "6 7 8" {
		t.Errorf("Ctrl+click on a selected row's header left rows %q, want 6 7 8", got)
	}
	r.clickWith(r.header("2"), 0)
	r.clickWith(r.header("4"), input.ModShift)
	if got := rowsSelected(r.UI); got != "2 3 4" {
		t.Errorf("Shift+click on row 4's header selected rows %q, want 2 3 4", got)
	}
	r.drag(r.header("9"), r.header("5"), 0)
	if got := rowsSelected(r.UI); got != "5 6 7 8 9" {
		t.Errorf("a drag from row 9's header to row 5's selected rows %q, want 5 to 9", got)
	}
	if got := r.ranges.Peek(); len(got) != 1 || !got[0].Columns.All || len(got[0].Rows.Keys) != 5 {
		t.Errorf("the drag made the ranges %+v, want one range of five rows", got)
	}
}

// With HeaderSelects a click on a column's header selects the column and does not sort;
// the arrow sorts. Ctrl adds a column, Shift extends, and a drag across the headers
// selects the columns it crosses. The corner selects every cell.
func TestDataGridHeaderSelectsColumns(t *testing.T) {
	r := headersRig(t)
	r.clickWith(r.header("Customer"), 0)
	if s := r.sort.Peek(); s != (ui.SortState{}) {
		t.Errorf("a click on a header sorted %+v; with HeaderSelects it selects", s)
	}
	if got := r.ranges.Peek(); len(got) != 1 || !got[0].Rows.All || got[0].Columns.Keys[0] != "Customer" {
		t.Fatalf("a click on the Customer header selected %+v, want the column", got)
	}
	if !r.header("Customer").Selected() {
		t.Error("the Customer header does not report its column selected")
	}
	r.clickWith(r.header("Total"), input.ModCtrl)
	if got := cellsOf(r.ranges.Peek(), orders(12)); strings.Count(got, "/Customer") != 12 || strings.Count(got, "/Total") != 12 ||
		strings.Count(got, "/Status") != 0 {
		t.Errorf("Ctrl+click on Total selected %q, want Customer and Total", got)
	}
	r.clickWith(r.header("OrderID"), input.ModShift)
	if got := cellsOf(r.ranges.Peek(), orders(12)); strings.Count(got, "/") != 12*4 {
		t.Errorf("Shift+click on OrderID from Total selected %d cells, want every column", strings.Count(got, "/"))
	}
	r.drag(r.header("Customer"), r.header("Status"), 0)
	if got := cellsOf(r.ranges.Peek(), orders(12)); strings.Count(got, "/") != 24 || strings.Count(got, "/Status") != 12 {
		t.Errorf("a drag from Customer to Status selected %d cells, want those two columns", strings.Count(got, "/"))
	}

	// The arrow of a sortable column sorts.
	arrow := r.header("Status").Bounds()
	rt := r.Runtime()
	x, y := arrow.X+arrow.W-14, arrow.Y+arrow.H/2
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.Frame()
	if s := r.sort.Peek(); s != (ui.SortState{Key: "Status"}) {
		t.Errorf("a click on the Status arrow sorted %+v, want by Status", s)
	}

	r.clickWith(r.ByRole(access.RoleButton, "Select all"), 0)
	if got := r.ranges.Peek(); len(got) != 1 || !got[0].Rows.All || !got[0].Columns.All {
		t.Errorf("the corner selected %+v, want every cell", got)
	}
}

// A row, a column and single cells make one selection, and each part shows.
func TestDataGridHeadersMixWithCells(t *testing.T) {
	r := headersRig(t)
	r.clickWith(r.header("2"), 0)
	r.clickWith(r.cell("1005"), input.ModCtrl)
	r.clickWith(r.cell("25.00"), input.ModCtrl)
	r.clickWith(r.header("Status"), input.ModCtrl)
	got := cellsOf(r.ranges.Peek(), orders(12))
	for _, want := range []string{"1001/OrderID", "1001/Total", "1005/OrderID", "1005/Total", "1000/Status", "1011/Status"} {
		if !strings.Contains(got, want) {
			t.Errorf("the selection %q does not have %s", got, want)
		}
	}
	if strings.Contains(got, "1005/Customer") || strings.Contains(got, "1000/Total") {
		t.Errorf("the selection %q has cells that were not clicked", got)
	}
	if !r.header("2").Selected() || r.header("6").Selected() || !r.header("Status").Selected() {
		t.Error("the headers do not report the row and the column selected whole")
	}
}

// The keys reach the headers: Left from the first column moves to the row's header,
// Space selects the row and Shift+Down extends it, Up from the first row's header
// moves to the corner, where Space selects every cell; on a column's header Space
// selects the column, Shift+Right extends it, and Enter sorts.
func TestDataGridHeadersByKeyboard(t *testing.T) {
	r := headersRig(t)
	r.cell("1002").Click()
	r.Press("left")
	r.Press("space")
	if got := rowsSelected(r.UI); got != "3" {
		t.Fatalf("Space on row 3's header selected rows %q, want 3", got)
	}
	r.Press("shift+down")
	r.Press("shift+down")
	if got := rowsSelected(r.UI); got != "3 4 5" {
		t.Errorf("Shift+Down twice selected rows %q, want 3 4 5", got)
	}
	r.Press("ctrl+up")
	r.Press("ctrl+up")
	r.Press("ctrl+up")
	r.Press("ctrl+up")
	r.Press("ctrl+space")
	if got := rowsSelected(r.UI); got != "1 3 4 5" {
		t.Errorf("Ctrl+Space on row 1's header selected rows %q, want 1 3 4 5", got)
	}
	r.Press("up")
	r.Press("space")
	if got := r.ranges.Peek(); len(got) != 1 || !got[0].Rows.All || !got[0].Columns.All {
		t.Errorf("Space on the corner selected %+v, want every cell", got)
	}
	r.Press("right") // the OrderID header
	r.Press("right") // the Customer header
	r.Press("space")
	r.Press("shift+right")
	if got := cellsOf(r.ranges.Peek(), orders(12)); strings.Count(got, "/") != 24 || strings.Count(got, "/Status") != 12 {
		t.Errorf("Space and Shift+Right on the Customer header selected %d cells, want Customer and Status", strings.Count(got, "/"))
	}
	r.Press("enter")
	if s := r.sort.Peek(); s != (ui.SortState{Key: "Status"}) {
		t.Errorf("Enter on the Status header sorted %+v, want by Status", s)
	}
	r.Press("ctrl+a")
	if got := r.ranges.Peek(); len(got) != 1 || !got[0].Rows.All || !got[0].Columns.All {
		t.Errorf("Ctrl+A selected %+v, want every cell", got)
	}
}

// Row headers are header items of their rows, column headers name their columns, and
// the corner is a button; a grid that selects rows selects a row from its header.
func TestDataGridRowHeadersAccessibility(t *testing.T) {
	r := headersRig(t)
	n := r.header("4").Node()
	if n.GridRow != 4 || n.GridColumn != 0 {
		t.Errorf("row 4's header is at row %d column %d, want row 4 and no column", n.GridRow, n.GridColumn)
	}
	if c := r.header("Status").Node(); c.GridRow != 0 || c.GridColumn != 3 {
		t.Errorf("the Status header is at row %d column %d, want column 3", c.GridRow, c.GridColumn)
	}
	if !r.HasRole(access.RoleButton, "Select all") {
		t.Error("the corner is not a button named Select all")
	}
	if g := r.ByRole(access.RoleDataGrid, "Results").Node(); g.GridColumns != 4 {
		t.Errorf("the grid reports %d columns, want 4: the row headers are not a column", g.GridColumns)
	}
	r.header("5").Perform(access.ActionSelect)
	if got := rowsSelected(r.UI); got != "5" {
		t.Errorf("the Select action of row 5's header selected rows %q, want 5", got)
	}
	if len(r.Warnings()) > 0 {
		t.Errorf("warnings: %v", r.Warnings())
	}

	picked := ui.NewSignal([]string(nil))
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.DataGrid(orders(6), orderKey, orderColumns()...).Label("Orders").
			Selection(ui.SelectionMultiple, picked).RowHeaders().Style(ui.Style{Height: ui.Px(300)})
	}, uitest.Size(520, 320))
	u.ByRole(access.RoleHeaderItem, "2").Click()
	u.ByRole(access.RoleButton, "Select all").Click()
	if got := len(picked.Peek()); got != 6 {
		t.Errorf("the corner of a grid that selects rows selected %d rows, want 6", got)
	}
	u.ByRole(access.RoleHeaderItem, "3").Click()
	if got := picked.Peek(); len(got) != 1 || got[0] != "1002" {
		t.Errorf("a click on row 3's header selected %v, want 1002", got)
	}
}

// The row headers stay at the start of the grid as the columns scroll sideways.
func TestDataGridRowHeadersStayInView(t *testing.T) {
	r := newGridRig(t, orders(12), func(g *ui.DataGridElement[order]) *ui.DataGridElement[order] {
		return g.RowHeaders().Width(ui.Px(240))
	})
	before, cell := r.header("1").Bounds().X, r.cell("1000").Bounds().X
	r.cell("1000").Click()
	for range 3 {
		r.Press("right")
	}
	r.Settle()
	if got := r.cell("1000").Bounds().X; got >= cell {
		t.Fatalf("the columns did not scroll: the first cell is at x %v, was %v", got, cell)
	}
	if got := r.header("1").Bounds().X; got != before {
		t.Errorf("row 1's header moved from x %v to %v as the columns scrolled", before, got)
	}
}

// A drag over the cells selects the rectangle from the cell it started on, and with
// Ctrl it keeps the rest of the selection.
func TestDataGridDragSelectsCells(t *testing.T) {
	r := newGridRig(t, orders(12), nil)
	r.drag(r.cell("Fabrikam"), r.cell("Cancelled"), 0) // 1001/Customer to 1002/Status
	if got := cellsOf(r.ranges.Peek(), orders(12)); got != "1001/Customer 1001/Status 1002/Customer 1002/Status" {
		t.Errorf("the drag selected %q, want rows 1001 and 1002 in Customer and Status", got)
	}
	r.drag(r.cell("1008"), r.cell("1009"), input.ModCtrl)
	if got := cellsOf(r.ranges.Peek(), orders(12)); !strings.Contains(got, "1001/Customer") ||
		!strings.Contains(got, "1008/OrderID 1009/OrderID") {
		t.Errorf("a Ctrl+drag selected %q, want the first rectangle and 1008-1009 in OrderID", got)
	}
	// A move with no button held, after the release, selects nothing more.
	b := r.cell("1011").Bounds()
	r.Runtime().PointerMove(b.X+b.W/2, b.Y+b.H/2, 0)
	r.Frame()
	if got := cellsOf(r.ranges.Peek(), orders(12)); strings.Contains(got, "1011") {
		t.Errorf("a move after the drag selected %q", got)
	}
}

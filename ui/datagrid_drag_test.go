package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// A range the app sets with both CellSelection and CellRanges given is listed in the
// cells too, and the grid's own changes keep both.
func TestDataGridRangesSetByTheAppReachTheCells(t *testing.T) {
	cells := ui.NewSignal([]ui.CellKey(nil))
	ranges := ui.NewSignal([]ui.CellRange(nil))
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.DataGrid(orders(4), orderKey, orderColumns()...).Label("Results").
			CellSelection(ui.SelectionMultiple, cells).CellRanges(ui.SelectionMultiple, ranges).
			Style(ui.Style{Height: ui.Px(300)})
	}, uitest.Size(520, 320))
	ranges.Set([]ui.CellRange{{Rows: ui.KeySet{Keys: []string{"1001"}}, Columns: ui.KeySet{All: true}}})
	u.Frame()
	u.Frame()
	if got := cellNames(cells.Peek()); got != "1001/OrderID 1001/Customer 1001/Status 1001/Total" {
		t.Errorf("after the app set row 1001 in the ranges the cells are %q", got)
	}
	ranges.Set(nil)
	u.Frame()
	if got := cells.Peek(); len(got) != 0 {
		t.Errorf("after the app cleared the ranges the cells are %v", got)
	}
	u.ByRole(access.RoleDataItem, "Tailspin").Click()
	if got := cellNames(cells.Peek()); got != "1003/Customer" || len(ranges.Peek()) != 1 {
		t.Errorf("a click set the cells %q and %d ranges, want 1003/Customer in one range", got, len(ranges.Peek()))
	}
}

func cellNames(cs []ui.CellKey) string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Row+"/"+c.Column)
	}
	return strings.Join(out, " ")
}

// A mouse press on a header, a row's header or the corner that no click follows
// leaves nothing behind: assistive technology's next Invoke of a header sorts at once.
func TestDataGridHeaderPressesDoNotSwallowTheNextInvoke(t *testing.T) {
	r := headersRig(t)
	invoke := func(what string) {
		t.Helper()
		r.sort.Set(ui.SortState{})
		r.Frame()
		r.header("Status").Perform(access.ActionClick)
		r.Frame()
		if s := r.sort.Peek(); s != (ui.SortState{Key: "Status"}) {
			t.Errorf("after %s, the first Invoke of the Status header sorted %+v, want by Status", what, s)
		}
		r.sort.Set(ui.SortState{})
		r.Frame()
	}
	r.clickWith(r.header("Total"), 0) // a header with no sort, so no click handler
	invoke("a click on the Total header")
	r.drag(r.header("Customer"), r.header("Total"), 0)
	invoke("a drag across the headers")
	r.drag(r.ByRole(access.RoleButton, "Select all"), r.header("Status"), 0)
	invoke("a press on the corner released over a header")
	r.drag(r.header("3"), r.header("Total"), 0)
	invoke("a drag from a row's header released over a column's")
	// A row pressed and released elsewhere does not swallow a click from assistive
	// technology either.
	r.drag(r.cell("Fabrikam"), r.header("Total"), 0)
	r.header("7").Perform(access.ActionClick)
	r.Frame()
	if got := rowsSelected(r.UI); got != "7" {
		t.Errorf("after a drag released outside the rows, Invoke of row 7's header selected rows %q, want 7", got)
	}
}

// While a cell is edited, or a control in a cell has focus, the grid's copy keys do
// not run: the editor's own Ctrl+C copies its text, and Ctrl+Shift+C and Ctrl+Insert,
// which it does not bind, leave the clipboard alone.
func TestDataGridCopyKeysLeaveCellEditorsAlone(t *testing.T) {
	cols := orderColumns()
	cols[1].Edit = func(order, string) error { return nil }
	cols[2].Edit = func(order, string) error { return nil }
	draft := ui.NewSignal("")
	cols[2].Editor = func(o order, value string, change func(string), commit, cancel func()) ui.Element {
		return ui.TextInput(draft).Label("Status")
	}
	cols[3].Interactive = true
	cols[3].Cell = func(o order) ui.Element {
		return ui.Button("Open " + orderKey(o)).Small()
	}
	ranges := ui.NewSignal([]ui.CellRange(nil))
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.DataGrid(orders(4), orderKey, cols...).Label("Results").
			CellRanges(ui.SelectionMultiple, ranges).Style(ui.Style{Height: ui.Px(300)})
	}, uitest.Size(520, 320))
	u.ByRole(access.RoleDataItem, "Fabrikam").Click()
	u.Press("f2")
	u.Press("ctrl+a") // the editor's: select its text
	for _, keys := range []string{"ctrl+shift+c", "ctrl+insert"} {
		u.SetClipboard("before")
		u.Press(keys)
		if got := u.Clipboard(); got != "before" {
			t.Errorf("%s in a cell's editor put %q on the clipboard", keys, got)
		}
	}
	u.Press("ctrl+c")
	if got := u.Clipboard(); got != "Fabrikam" {
		t.Errorf("Ctrl+C in a cell's editor copied %q, want the editor's text", got)
	}
	u.Press("escape")
	u.SetClipboard("")
	u.Press("ctrl+c")
	if got := u.Clipboard(); got != "Fabrikam" {
		t.Errorf("Ctrl+C on the grid after the edit copied %q, want the selected cell", got)
	}
	// A custom editor's text field.
	u.ByRole(access.RoleDataItem, "Pending").Click()
	u.Press("f2")
	u.Type("Late")
	for _, keys := range []string{"ctrl+shift+c", "ctrl+insert"} {
		u.SetClipboard("before")
		u.Press(keys)
		if got := u.Clipboard(); got != "before" {
			t.Errorf("%s in a custom editor put %q on the clipboard", keys, got)
		}
	}
	u.Press("ctrl+a")
	u.Press("ctrl+c")
	if got := u.Clipboard(); got != "Late" {
		t.Errorf("Ctrl+C in a custom editor copied %q, want its text", got)
	}
	u.Press("escape")
	// A focused button in an interactive cell.
	u.ByRole(access.RoleDataItem, "21.00").Click()
	u.Press("enter")
	if f := u.Focused(); f.Name() != "Open 1001" {
		t.Fatalf("Enter on the interactive cell focused %q, want its button", f.Name())
	}
	for _, keys := range []string{"ctrl+c", "ctrl+shift+c", "ctrl+insert"} {
		u.SetClipboard("before")
		u.Press(keys)
		if got := u.Clipboard(); got != "before" {
			t.Errorf("%s on a button in a cell put %q on the clipboard", keys, got)
		}
	}
}

// hold presses the left button at from, moves the pointer to (x, y), and holds it there
// for d, with frames running.
func (r *gridRig) hold(from *uitest.Element, x, y float32, d time.Duration) {
	a := from.Bounds()
	rt := r.Runtime()
	rt.PointerMove(a.X+a.W/2, a.Y+a.H/2, 0)
	rt.PointerDown(a.X+a.W/2, a.Y+a.H/2, input.ButtonLeft, 1, 0)
	r.Frame()
	rt.PointerMove(x, y, 0)
	r.Advance(d)
}

// release lets the left button go at (x, y).
func (r *gridRig) release(x, y float32) {
	r.Runtime().PointerUp(x, y, input.ButtonLeft, 0)
	r.Frame()
}

// A drag of cells held below the rows in view scrolls them and selects the rows that
// come into view; a pointer left of the grid keeps the first column. Released, the
// grid stops.
func TestDataGridDragPastTheEdgeScrolls(t *testing.T) {
	rows := orders(200)
	r := newGridRig(t, rows, nil)
	grid := r.ByRole(access.RoleDataGrid, "Results").Bounds()
	start := r.cell("Fabrikam") // 1001, Customer
	r.hold(start, grid.X-6, grid.Y+grid.H+20, 600*time.Millisecond)
	got := r.ranges.Peek()
	if len(got) != 1 || got[0].Rows.All || len(got[0].Rows.Keys) < 16 {
		t.Fatalf("a drag held below the grid selected %+v, want many rows scrolled into view", got)
	}
	if cols := got[0].Columns.Keys; len(cols) != 2 || cols[0] != "OrderID" || cols[1] != "Customer" {
		t.Errorf("a drag held left of the grid selected the columns %v, want OrderID and Customer", cols)
	}
	if r.HasRole(access.RoleDataItem, "1001") {
		t.Error("the rows did not scroll")
	}
	n := len(got[0].Rows.Keys)
	r.release(grid.X-6, grid.Y+grid.H+20)
	r.Advance(300 * time.Millisecond)
	if m := len(r.ranges.Peek()[0].Rows.Keys); m != n {
		t.Errorf("after the release the selection grew from %d rows to %d", n, m)
	}
	// Back up, over the header and above the grid: the rows scroll back.
	r.hold(r.cell(lastShownTotal(r)), grid.X+grid.W/2, grid.Y-10, 400*time.Millisecond)
	got = r.ranges.Peek()
	r.release(grid.X+grid.W/2, grid.Y-10)
	if len(got) != 1 || len(got[0].Rows.Keys) < 10 {
		t.Errorf("a drag held above the grid selected %+v, want rows scrolled into view above", got)
	}
}

// lastShownTotal returns the text of a Total cell shown, the last one in the tree.
func lastShownTotal(r *gridRig) string {
	last := ""
	r.Tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleDataItem && n.GridColumn == 4 {
			last = n.Name
		}
		return true
	})
	return last
}

// A drag down the row headers held below the grid selects whole rows as they scroll
// into view, and a drag across the column headers held right of a narrow grid
// scrolls the columns and selects them.
func TestDataGridHeaderDragsPastTheEdgeScroll(t *testing.T) {
	r := newGridRig(t, orders(200), func(g *ui.DataGridElement[order]) *ui.DataGridElement[order] {
		return g.RowHeaders().HeaderSelects().Width(ui.Px(220))
	})
	grid := r.ByRole(access.RoleDataGrid, "Results").Bounds()
	r.hold(r.header("2"), grid.X+grid.W+40, grid.Y+grid.H+15, 500*time.Millisecond)
	r.release(grid.X+grid.W+40, grid.Y+grid.H+15)
	got := r.ranges.Peek()
	if len(got) != 1 || !got[0].Columns.All || len(got[0].Rows.Keys) < 16 {
		t.Errorf("a drag down the row headers held below the grid selected %+v, want whole rows scrolled into view", got)
	}
	// The rows have scrolled: the headers are still where they are shown.
	r.hold(r.header("OrderID"), grid.X+grid.W+30, grid.Y+10, 600*time.Millisecond)
	r.release(grid.X+grid.W+30, grid.Y+10)
	got = r.ranges.Peek()
	if len(got) != 1 || !got[0].Rows.All || strings.Join(got[0].Columns.Keys, " ") != "OrderID Customer Status Total" {
		t.Errorf("a drag across the headers held right of the grid selected %+v, want every column", got)
	}
}

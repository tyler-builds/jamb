package ui_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// order is a row of the results grids of these tests.
type order struct {
	ID       int
	Customer string
	Status   string
	Total    string
}

func orders(n int) []order {
	names := []string{"Contoso", "Fabrikam", "Northwind", "Tailspin"}
	st := []string{"Shipped", "Pending", "Cancelled"}
	out := make([]order, n)
	for i := range out {
		out[i] = order{ID: 1000 + i, Customer: names[i%len(names)], Status: st[i%len(st)], Total: fmt.Sprintf("%d.00", 20+i)}
	}
	return out
}

func orderKey(o order) string { return fmt.Sprint(o.ID) }

func orderColumns() []ui.DataColumn[order] {
	return []ui.DataColumn[order]{
		{Header: "OrderID", Text: func(o order) string { return fmt.Sprint(o.ID) }, Width: 90,
			Compare: ui.SortBy(func(o order) int { return o.ID })},
		{Header: "Customer", Text: func(o order) string { return o.Customer }, Width: 110,
			Compare: ui.SortByText(func(o order) string { return o.Customer })},
		{Header: "Status", Text: func(o order) string { return o.Status }, Width: 100,
			Compare: ui.SortByText(func(o order) string { return o.Status })},
		{Header: "Total", Text: func(o order) string { return o.Total }, Width: 80, Align: ui.End},
	}
}

// gridRig is a results grid of the rows, whose grid the test changes with opts.
type gridRig struct {
	*uitest.UI
	ranges *ui.Signal[[]ui.CellRange]
	sort   *ui.Signal[ui.SortState]
}

func newGridRig(t *testing.T, rows []order, opts func(g *ui.DataGridElement[order]) *ui.DataGridElement[order]) *gridRig {
	r := &gridRig{ranges: ui.NewSignal([]ui.CellRange(nil)), sort: ui.NewSignal(ui.SortState{})}
	r.UI = uitest.New(t, func(ctx *ui.Context) ui.Element {
		g := ui.DataGrid(rows, orderKey, orderColumns()...).Label("Results").
			CellRanges(ui.SelectionMultiple, r.ranges).Sort(r.sort).Density(ui.DataGridCompact)
		if opts != nil {
			g = opts(g)
		}
		return ui.Div().Padding(8).Child(g.Style(ui.Style{Height: ui.Px(300)}))
	}, uitest.Size(520, 320))
	return r
}

// cell returns the cell with the text name.
func (r *gridRig) cell(name string) *uitest.Element { return r.ByRole(access.RoleDataItem, name) }

// clickWith clicks e with the modifiers mods held.
func (r *gridRig) clickWith(e *uitest.Element, mods input.Modifiers) {
	b := e.Bounds()
	x, y := b.X+b.W/2, b.Y+b.H/2
	rt := r.Runtime()
	rt.PointerMove(x, y, mods)
	rt.PointerDown(x, y, input.ButtonLeft, 1, mods)
	rt.PointerUp(x, y, input.ButtonLeft, mods)
	r.Frame()
}

// selectedCells returns the cells the access tree reports as selected, as "row/column"
// sorted, for the rows shown.
func selectedCells(u *uitest.UI) string {
	var out []string
	t := u.Tree()
	t.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleDataItem && n.States.Has(access.StateSelected) {
			out = append(out, fmt.Sprintf("%d/%d", n.GridRow, n.GridColumn))
		}
		return true
	})
	slices.Sort(out)
	return strings.Join(out, " ")
}

// cellsOf returns "row/column" of each cell the ranges have among the rows given, for
// the columns of the grid, sorted.
func cellsOf(rs []ui.CellRange, rows []order) string {
	var out []string
	for _, o := range rows {
		for _, c := range []string{"OrderID", "Customer", "Status", "Total"} {
			for _, r := range rs {
				if r.Has(ui.CellKey{Row: orderKey(o), Column: c}) {
					out = append(out, orderKey(o)+"/"+c)
					break
				}
			}
		}
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

// Ctrl+A, Ctrl+Space and Shift+Space each make one range, whatever the number of rows,
// and a Ctrl+click in them takes out one cell.
func TestDataGridRangesAreBlocksNotCells(t *testing.T) {
	rows := orders(5000)
	r := newGridRig(t, rows, nil)
	r.cell("Fabrikam").Click() // row 1001
	r.Press("ctrl+a")
	if got := r.ranges.Peek(); len(got) != 1 || !got[0].Rows.All || !got[0].Columns.All {
		t.Fatalf("Ctrl+A made %+v, want one range of every row and column", got)
	}
	r.clickWith(r.cell("Northwind"), input.ModCtrl) // row 1002, Customer
	got := r.ranges.Peek()
	if len(got) > 2 {
		t.Errorf("a Ctrl+click after Ctrl+A made %d ranges, want at most 2", len(got))
	}
	for _, rg := range got {
		if !rg.Rows.All && len(rg.Rows.Keys) > 1 {
			t.Errorf("a range lists %d rows; a Ctrl+click should not list every row", len(rg.Rows.Keys))
		}
	}
	if rangesHave(got, "1002", "Customer") || !rangesHave(got, "1002", "Status") || !rangesHave(got, "4999", "Total") {
		t.Errorf("after the Ctrl+click the selection is %+v, want every cell but 1002/Customer", got)
	}
	if r.cell("Northwind").Selected() || !r.cell("Contoso").Selected() {
		t.Error("the access tree does not show the cell taken out of the selection")
	}

	r.Press("ctrl+space")
	if got := r.ranges.Peek(); len(got) != 1 || !got[0].Rows.All || got[0].Columns.All ||
		!slices.Equal(got[0].Columns.Keys, []string{"Customer"}) {
		t.Errorf("Ctrl+Space made %+v, want the Customer column", got)
	}
	r.Press("shift+space")
	if got := r.ranges.Peek(); len(got) != 1 || got[0].Rows.All || !got[0].Columns.All ||
		!slices.Equal(got[0].Rows.Keys, []string{"1002"}) {
		t.Errorf("Shift+Space made %+v, want row 1002", got)
	}
}

// rangesHave reports whether a range of rs has the cell.
func rangesHave(rs []ui.CellRange, row, col string) bool {
	for _, r := range rs {
		if r.Has(ui.CellKey{Row: row, Column: col}) {
			return true
		}
	}
	return false
}

// Shift extends from the anchor in whole columns after Ctrl+Space and in whole rows
// after Shift+Space, and Ctrl+Shift+click adds a rectangle to the rest of the selection.
func TestDataGridRangesExtend(t *testing.T) {
	rows := orders(6)
	r := newGridRig(t, rows, nil)
	r.cell("Fabrikam").Click() // row 1001, Customer
	r.Press("ctrl+space")
	r.Press("shift+right")
	if got := cellsOf(r.ranges.Peek(), rows); strings.Count(got, "/Customer") != 6 || strings.Count(got, "/Status") != 6 ||
		strings.Contains(got, "/Total") || strings.Contains(got, "/OrderID") {
		t.Errorf("Ctrl+Space then Shift+Right selected %q, want the Customer and Status columns", got)
	}
	r.cell("Northwind").Click() // row 1002
	r.Press("shift+space")
	r.Press("shift+down")
	if got := cellsOf(r.ranges.Peek(), rows); strings.Count(got, "1002/") != 4 || strings.Count(got, "1003/") != 4 ||
		strings.Count(got, "/") != 8 {
		t.Errorf("Shift+Space then Shift+Down selected %q, want rows 1002 and 1003", got)
	}
	r.cell("Contoso").Click()                                      // row 1000
	r.clickWith(r.cell("Pending"), input.ModCtrl)                  // row 1001, Status
	r.clickWith(r.cell("Cancelled"), input.ModCtrl|input.ModShift) // row 1002, Status
	if got := cellsOf(r.ranges.Peek(), rows); got != "1000/Customer 1001/Status 1002/Status" {
		t.Errorf("click, Ctrl+click and Ctrl+Shift+click selected %q, want 1000/Customer with 1001-1002/Status", got)
	}
	r.clickWith(r.cell("Shipped"), input.ModShift) // row 1000, Status
	if got := cellsOf(r.ranges.Peek(), rows); got != "1000/Status 1001/Status" {
		t.Errorf("Shift+click selected %q, want the rectangle from the anchor alone", got)
	}
}

// A selection follows its rows through a sort, a range of every row takes in rows
// that are added, and a value the app sets selects those cells.
func TestDataGridRangesFollowKeys(t *testing.T) {
	rows := ui.NewSignal(orders(6))
	ranges := ui.NewSignal([]ui.CellRange(nil))
	sort := ui.NewSignal(ui.SortState{})
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.DataGrid(rows.Get(), orderKey, orderColumns()...).Label("Results").
			CellRanges(ui.SelectionMultiple, ranges).Sort(sort).Style(ui.Style{Height: ui.Px(300)})
	}, uitest.Size(520, 320))
	ranges.Set([]ui.CellRange{{Rows: ui.KeySet{Keys: []string{"1001"}}, Columns: ui.KeySet{Keys: []string{"Status"}}}})
	u.Frame()
	if !u.ByRole(access.RoleDataItem, "Pending").Selected() {
		t.Fatal("a range set by the app does not select its cell")
	}
	sort.Set(ui.SortState{Key: "OrderID", Desc: true})
	u.Frame()
	if got := selectedCells(u); got != "5/3" {
		t.Errorf("after a descending sort the selected cells are %q, want row 5 column 3 (1001's Status)", got)
	}
	ranges.Set([]ui.CellRange{{Rows: ui.KeySet{All: true}, Columns: ui.KeySet{Keys: []string{"Total"}}}})
	rows.Set(append(orders(6), order{ID: 2000, Customer: "Adatum", Status: "New", Total: "99.00"}))
	u.Frame()
	if !u.ByRole(access.RoleDataItem, "99.00").Selected() || u.ByRole(access.RoleDataItem, "New").Selected() {
		t.Error("a row added to a selected column does not take its cell in the column alone")
	}
}

// CellSelection and CellRanges can be given together: the grid keeps both, and a
// value the app sets in the cells selects them.
func TestDataGridCellSelectionIsKeptWithRanges(t *testing.T) {
	cells := ui.NewSignal([]ui.CellKey(nil))
	ranges := ui.NewSignal([]ui.CellRange(nil))
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.DataGrid(orders(4), orderKey, orderColumns()...).Label("Results").
			CellSelection(ui.SelectionMultiple, cells).CellRanges(ui.SelectionMultiple, ranges).
			Style(ui.Style{Height: ui.Px(300)})
	}, uitest.Size(520, 320))
	u.ByRole(access.RoleDataItem, "Fabrikam").Click()
	u.Press("ctrl+a")
	if len(cells.Peek()) != 16 || len(ranges.Peek()) != 1 {
		t.Fatalf("Ctrl+A set %d cells and %d ranges, want 16 and 1", len(cells.Peek()), len(ranges.Peek()))
	}
	cells.Set([]ui.CellKey{{Row: "1000", Column: "Total"}, {Row: "1001", Column: "Total"}, {Row: "1003", Column: "OrderID"}})
	u.Frame()
	if got := ranges.Peek(); len(got) != 2 || !slices.Equal(got[0].Rows.Keys, []string{"1000", "1001"}) {
		t.Errorf("the cells set by the app made the ranges %+v, want rows 1000-1001 in Total and 1003 in OrderID", got)
	}
	if !u.ByRole(access.RoleDataItem, "21.00").Selected() || u.ByRole(access.RoleDataItem, "22.00").Selected() {
		t.Error("the cells set by the app are not the ones selected")
	}
	if len(u.Warnings()) > 0 {
		t.Errorf("warnings: %v", u.Warnings())
	}
}

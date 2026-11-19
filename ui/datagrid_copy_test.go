package ui_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/actions"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// copied presses keys and returns what they put on the clipboard, with the line
// breaks as \n for reading.
func (r *gridRig) copied(keys string) string {
	r.SetClipboard("")
	r.Press(keys)
	return strings.ReplaceAll(r.Clipboard(), "\r\n", "\n")
}

// Ctrl+C copies a rectangle as tab-separated values with CR LF line breaks, and
// Ctrl+Shift+C puts the headers of its columns first.
func TestDataGridCopiesARectangle(t *testing.T) {
	r := newGridRig(t, orders(6), nil)
	r.cell("Fabrikam").Click()
	r.clickWith(r.cell("Cancelled"), input.ModShift) // 1001/Customer to 1002/Status
	r.SetClipboard("")
	r.Press("ctrl+c")
	if got := r.Clipboard(); got != "Fabrikam\tPending\r\nNorthwind\tCancelled" {
		t.Errorf("Ctrl+C copied %q", got)
	}
	if got := r.copied("ctrl+shift+c"); got != "Customer\tStatus\nFabrikam\tPending\nNorthwind\tCancelled" {
		t.Errorf("Ctrl+Shift+C copied %q", got)
	}
	if got := r.copied("ctrl+insert"); got != "Fabrikam\tPending\nNorthwind\tCancelled" {
		t.Errorf("Ctrl+Insert copied %q", got)
	}
}

// A selection that is not a rectangle copies the rows and the columns that have a
// selected cell, with an empty field for each cell among them that is not selected.
func TestDataGridCopiesASelectionThatIsNotARectangle(t *testing.T) {
	r := newGridRig(t, orders(6), func(g *ui.DataGridElement[order]) *ui.DataGridElement[order] { return g.RowHeaders() })
	r.cell("Fabrikam").Click()                 // 1001/Customer
	r.clickWith(r.cell("1004"), input.ModCtrl) // 1004/OrderID
	if got := r.copied("ctrl+shift+c"); got != "OrderID\tCustomer\n\tFabrikam\n1004\t" {
		t.Errorf("two cells copied %q", got)
	}
	r.clickWith(r.header("3"), input.ModCtrl) // row 1002, whole
	if got := r.copied("ctrl+c"); got != "\tFabrikam\t\t\n1002\tNorthwind\tCancelled\t22.00\n1004\t\t\t" {
		t.Errorf("a row and two cells copied %q", got)
	}
}

// The copy follows the columns and rows as they are shown: moved, pinned, sorted and
// filtered.
func TestDataGridCopiesWhatIsShown(t *testing.T) {
	colOrder := ui.NewSignal([]string{"Total", "OrderID"})
	filter := ui.NewSignal(false)
	ranges := ui.NewSignal([]ui.CellRange(nil))
	sort := ui.NewSignal(ui.SortState{Key: "OrderID", Desc: true})
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		cols := orderColumns()
		cols[2].Pin = ui.PinStart // Status
		rows := orders(5)
		if filter.Get() {
			rows = slicesFilter(rows, func(o order) bool { return o.Status != "Pending" })
		}
		return ui.DataGrid(rows, orderKey, cols...).Label("Results").
			CellRanges(ui.SelectionMultiple, ranges).Sort(sort).ColumnOrder(colOrder).Style(ui.Style{Height: ui.Px(300)})
	}, uitest.Size(520, 320))
	u.ByRole(access.RoleDataItem, "1003").Click()
	u.Press("ctrl+a")
	u.SetClipboard("")
	u.Press("ctrl+shift+c")
	want := "Status\tTotal\tOrderID\tCustomer\r\n" +
		"Pending\t24.00\t1004\tContoso\r\nShipped\t23.00\t1003\tTailspin\r\nCancelled\t22.00\t1002\tNorthwind\r\n" +
		"Pending\t21.00\t1001\tFabrikam\r\nShipped\t20.00\t1000\tContoso"
	if got := u.Clipboard(); got != want {
		t.Errorf("Ctrl+Shift+C copied\n%q\nwant\n%q", got, want)
	}
	filter.Set(true)
	u.Frame()
	u.Press("ctrl+c")
	if got := u.Clipboard(); strings.Contains(got, "Pending") || strings.Count(got, "\r\n") != 2 {
		t.Errorf("after a filter Ctrl+C copied %q, want the three rows shown", got)
	}
}

func slicesFilter[T any](s []T, keep func(T) bool) []T {
	var out []T
	for _, x := range s {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

// A cell with a tab, a line break or a double quote is quoted as a spreadsheet reads
// it, and DataColumn.CopyText and DataGrid.CopyText change the text.
func TestDataGridCopyQuotesAndCopyText(t *testing.T) {
	rows := []order{{ID: 1, Customer: "Tab\there", Status: "two\nlines", Total: `5" pipe`}, {ID: 2, Customer: "plain", Status: "ok", Total: "1"}}
	ranges := ui.NewSignal([]ui.CellRange{{Rows: ui.KeySet{All: true}, Columns: ui.KeySet{All: true}}})
	whole := ui.NewSignal(false)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		cols := orderColumns()
		cols[0].CopyText = func(o order) string { return "#" + orderKey(o) }
		g := ui.DataGrid(rows, orderKey, cols...).Label("Results").CellRanges(ui.SelectionMultiple, ranges).
			Style(ui.Style{Height: ui.Px(300)})
		if whole.Get() {
			g.CopyText(func(c ui.DataGridCopy[order]) string {
				if !c.Headers {
					return "" // the app copies by itself
				}
				return strings.Join(c.Columns, ",") + ";" + c.Rows[1].Customer + ";" + strconv.Itoa(strings.Count(c.Text(), "\r\n"))
			})
		}
		return g
	}, uitest.Size(520, 320))
	u.ByRole(access.RoleDataItem, "plain").Click()
	ranges.Set([]ui.CellRange{{Rows: ui.KeySet{All: true}, Columns: ui.KeySet{All: true}}})
	u.Frame()
	u.SetClipboard("")
	u.Press("ctrl+c")
	want := "#1\t\"Tab\there\"\t\"two\nlines\"\t\"5\"\" pipe\"\r\n#2\tplain\tok\t1"
	if got := u.Clipboard(); got != want {
		t.Errorf("Ctrl+C copied %q, want %q", got, want)
	}
	whole.Set(true)
	u.Frame()
	u.SetClipboard("before")
	u.Press("ctrl+c")
	if got := u.Clipboard(); got != "before" {
		t.Errorf("a CopyText that returns nothing put %q on the clipboard", got)
	}
	u.Press("ctrl+shift+c")
	if got := u.Clipboard(); got != "OrderID,Customer,Status,Total;plain;2" {
		t.Errorf("CopyText made %q", got)
	}
}

// With nothing selected the copy is the focused cell; a grid that selects rows copies
// its rows; the handle copies from a button; the keys can be rebound and unbound; and
// the context menu runs the copy actions.
func TestDataGridCopyFromElsewhere(t *testing.T) {
	picked := ui.NewSignal([]string(nil))
	h := ui.NewDataGridHandle()
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Child(ui.Button("Copy rows").Key("copy").OnClick(func() { h.Copy(true) })).
			Child(ui.DataGrid(orders(4), orderKey, orderColumns()...).Label("Orders").Handle(h).
				Selection(ui.SelectionMultiple, picked).Style(ui.Style{Height: ui.Px(260)}).
				ContextMenu(func(o order, col string) []ui.MenuItem {
					return []ui.MenuItem{{Action: actions.GridCopy}, {Action: actions.GridCopyWithHeaders}}
				}))
	}, uitest.Size(520, 320),
		uitest.Bindings(ui.BindIn("DataGrid", "ctrl+k", actions.GridCopy), ui.BindIn("DataGrid", "ctrl+insert", ui.Unbind(actions.GridCopy))))
	u.ByRole(access.RoleDataItem, "Northwind").Click()
	u.SetClipboard("")
	u.Press("ctrl+c")
	if got := u.Clipboard(); got != "1002\tNorthwind\tCancelled\t22.00" {
		t.Errorf("Ctrl+C on a selected row copied %q, want the row", got)
	}
	picked.Set(nil)
	u.Frame()
	u.SetClipboard("")
	u.Press("ctrl+c")
	if got := u.Clipboard(); got != "Northwind" {
		t.Errorf("Ctrl+C with nothing selected copied %q, want the focused cell", got)
	}
	picked.Set([]string{"1003", "1001"})
	u.Frame()
	u.Press("ctrl+k")
	if got := u.Clipboard(); got != "1001\tFabrikam\tPending\t21.00\r\n1003\tTailspin\tShipped\t23.00" {
		t.Errorf("Ctrl+K, bound to GridCopy, copied %q, want rows 1001 and 1003 in the order shown", got)
	}
	u.SetClipboard("")
	u.Press("ctrl+insert")
	if got := u.Clipboard(); got != "" {
		t.Errorf("Ctrl+Insert, unbound, copied %q", got)
	}
	u.Click("copy")
	if got := u.Clipboard(); !strings.HasPrefix(got, "OrderID\tCustomer\tStatus\tTotal\r\n1001") {
		t.Errorf("DataGridHandle.Copy(true) copied %q", got)
	}
	u.SetClipboard("")
	u.ByRole(access.RoleDataItem, "Tailspin").RightClick()
	u.ByRole(access.RoleMenuItem, "Copy with headers").Click()
	if got := u.Clipboard(); !strings.HasPrefix(got, "OrderID\tCustomer") || !strings.Contains(got, "1003") {
		t.Errorf("the context menu's Copy with headers copied %q", got)
	}
}

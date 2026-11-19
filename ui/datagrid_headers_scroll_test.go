package ui_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// After the rows scroll down, the headers still act where they are shown: a click on
// a column's header selects the column (HeaderSelects), Ctrl+click adds one, a drag
// across them selects a run, the arrow sorts, and the corner selects every cell. The
// row headers stay in line with the rows, and the corner with the row headers.
func TestDataGridHeaderSelectsAfterScroll(t *testing.T) {
	r := newGridRig(t, orders(200), func(g *ui.DataGridElement[order]) *ui.DataGridElement[order] {
		return g.RowHeaders().HeaderSelects()
	})
	r.ByRole(access.RoleDataGrid, "Results").Scroll(0, 600)
	r.Settle()
	if r.HasRole(access.RoleDataItem, "1000") {
		t.Fatal("the rows did not scroll")
	}
	r.clickWith(r.header("Customer"), 0)
	if got := r.ranges.Peek(); len(got) != 1 || !got[0].Rows.All || strings.Join(got[0].Columns.Keys, " ") != "Customer" {
		t.Fatalf("a click on the Customer header after a scroll selected %+v, want the column", got)
	}
	if s := r.sort.Peek(); s != (ui.SortState{}) {
		t.Errorf("a click on the Customer header after a scroll sorted %+v", s)
	}
	r.clickWith(r.header("Total"), input.ModCtrl)
	if got := cellsOf(r.ranges.Peek(), orders(200)); strings.Count(got, "/Total") != 200 || strings.Count(got, "/Customer") != 200 {
		t.Errorf("Ctrl+click on the Total header after a scroll selected %d cells, want Customer and Total", strings.Count(got, "/"))
	}
	r.drag(r.header("OrderID"), r.header("Status"), 0)
	if got := r.ranges.Peek(); len(got) != 1 || strings.Join(got[0].Columns.Keys, " ") != "OrderID Customer Status" {
		t.Errorf("a drag across the headers after a scroll selected %+v, want OrderID to Status", got)
	}
	// The arrow at the end of the Status header sorts.
	b := r.header("Status").Bounds()
	rt := r.Runtime()
	x, y := b.X+b.W-14, b.Y+b.H/2
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.Frame()
	if s := r.sort.Peek(); s != (ui.SortState{Key: "Status"}) {
		t.Errorf("a click on the Status arrow after a scroll sorted %+v, want by Status", s)
	}
	// Each row's header is level with its row's cells, and the corner and the column
	// headers are level with each other, over the row headers and the cells.
	corner := r.ByRole(access.RoleButton, "Select all").Bounds()
	head := r.header("OrderID").Bounds()
	if corner.Y != head.Y || corner.H != head.H {
		t.Errorf("the corner is at y %v, %v high; the headers at y %v, %v high", corner.Y, corner.H, head.Y, head.H)
	}
	first := firstShownRow(r)
	rh := r.header(first.name).Bounds()
	if rh.Y != first.b.Y || rh.X+rh.W != first.b.X || corner.X != rh.X || corner.W != rh.W {
		t.Errorf("row %s's header is at %+v and its first cell at %+v; the corner at %+v", first.name, rh, first.b, corner)
	}
	r.clickWith(r.ByRole(access.RoleButton, "Select all"), 0)
	if got := r.ranges.Peek(); len(got) != 1 || !got[0].Rows.All || !got[0].Columns.All {
		t.Errorf("the corner after a scroll selected %+v, want every cell", got)
	}
}

// shownRow is the first row in full view: its number, as its header names it, and
// its first cell's bounds.
type shownRow struct {
	name string
	b    ui.Rect
}

func firstShownRow(r *gridRig) shownRow {
	head := r.header("OrderID").Bounds()
	var out shownRow
	best := float32(1e9)
	r.Tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleDataItem && n.GridColumn == 1 && !n.States.Has(access.StateOffscreen) &&
			n.Bounds.Y >= head.Y+head.H && n.Bounds.Y < best {
			best = n.Bounds.Y
			out = shownRow{name: strconv.Itoa(n.GridRow), b: ui.Rect{X: n.Bounds.X, Y: n.Bounds.Y, W: n.Bounds.W, H: n.Bounds.H}}
		}
		return true
	})
	return out
}

package ui_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// hsRow is a row of the grids of these tests.
type hsRow struct{ ID int }

// hsGrid is a grid of 200 rows, 300 high, whose header stays at the top of its
// rows as they scroll, and the signals it reports to.
type hsGrid struct {
	u        *uitest.UI
	sort     *ui.Signal[ui.SortState]
	sel      *ui.Signal[[]string]
	order    *ui.Signal[[]string]
	widths   *ui.Signal[map[string]float32]
	menus    []string
	activate []int
}

// newHSGrid returns a grid of the columns cols, ID and Name when there are none.
// The columns of the grid are as wide as their headers' Width.
func newHSGrid(t *testing.T, w float32, cols ...ui.DataColumn[hsRow]) *hsGrid {
	t.Helper()
	text := func(r hsRow) string { return fmt.Sprint(r.ID) }
	if len(cols) == 0 {
		cols = []ui.DataColumn[hsRow]{
			{Header: "ID", Text: text, Width: 120, Resizable: true, Compare: ui.SortBy(func(r hsRow) int { return r.ID })},
			{Header: "Name", Text: text, Width: 120, Resizable: true, Movable: true, Compare: ui.SortByText(text)},
		}
	}
	rows := make([]hsRow, 200)
	for i := range rows {
		rows[i] = hsRow{i}
	}
	g := &hsGrid{
		sort:   ui.NewSignal(ui.SortState{}),
		sel:    ui.NewSignal([]string(nil)),
		order:  ui.NewSignal([]string(nil)),
		widths: ui.NewSignal(map[string]float32(nil)),
	}
	g.u = uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(8).Child(ui.DataGrid(rows, text, cols...).Label("G").
			Sort(g.sort).Selection(ui.SelectionSingle, g.sel).ColumnOrder(g.order).ColumnWidths(g.widths).
			OnActivate(func(r hsRow) { g.activate = append(g.activate, r.ID) }).
			HeaderContextMenu(func(column string) []ui.MenuItem {
				g.menus = append(g.menus, column)
				return []ui.MenuItem{{Label: "Hide column"}}
			}).
			Style(ui.Style{Height: ui.Px(300)}))
	}, uitest.Size(w, 320))
	return g
}

// header returns the header item of the column named name.
func (g *hsGrid) header(name string) *uitest.Element {
	return g.u.ByRole(access.RoleHeaderItem, name)
}

// scroll turns the wheel over the grid by dx and dy and lets the scroll settle.
func (g *hsGrid) scroll(dx, dy float32) {
	g.u.ByRole(access.RoleDataGrid, "G").Scroll(dx, dy)
	g.u.Settle()
}

// pressAt presses and releases the button b at (x, y).
func (g *hsGrid) pressAt(x, y float32, b input.MouseButton, clicks int) {
	rt := g.u.Runtime()
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, b, clicks, 0)
	rt.PointerUp(x, y, b, 0)
	g.u.Frame()
}

// dragFrom presses at (x, y), moves the pointer by dx in steps, and releases it.
func (g *hsGrid) dragFrom(x, y, dx float32) {
	rt := g.u.Runtime()
	rt.PointerMove(x, y, 0)
	g.u.Frame()
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	for _, f := range []float32{0.25, 0.5, 0.75, 1} {
		rt.PointerMove(x+dx*f, y, 0)
		g.u.Frame()
	}
	rt.PointerUp(x+dx, y, input.ButtonLeft, 0)
	g.u.Frame()
}

// hsScrolls are the vertical scrolls the tests are run at: none, less than the
// header is high, more, much more, and to the end.
var hsScrolls = []float32{0, 10, 40, 200, 1e6}

// TestDataGridHeaderClickSortsAfterScroll checks that a click on a header cell
// sorts however far the rows are scrolled, and that the header item is reported
// where it is drawn, at the top of the grid and on screen.
func TestDataGridHeaderClickSortsAfterScroll(t *testing.T) {
	for _, dy := range hsScrolls {
		t.Run(fmt.Sprint(dy), func(t *testing.T) {
			g := newHSGrid(t, 520)
			before := g.header("ID").Bounds()
			g.scroll(0, dy)
			n := g.header("ID").Node()
			if n.States.Has(access.StateOffscreen) {
				t.Errorf("after a scroll by %v the header item is off screen", dy)
			}
			if got := g.header("ID").Bounds(); got != before {
				t.Errorf("after a scroll by %v the header item is at %+v, want %+v", dy, got, before)
			}
			g.header("ID").Click()
			if got := g.sort.Peek(); got.Key != "ID" || got.Desc {
				t.Errorf("a click on the header after a scroll by %v sorted %+v, want ID ascending", dy, got)
			}
			if got := g.sel.Peek(); len(got) != 0 {
				t.Errorf("the click on the header selected the rows %v", got)
			}
		})
	}
}

// TestDataGridHeaderPressDoesNotReachTheRowUnder checks that a press where the
// header is drawn, on a cell or beside the last one, goes to the header and not to
// the row scrolled under it: it neither selects nor activates the row.
func TestDataGridHeaderPressDoesNotReachTheRowUnder(t *testing.T) {
	for _, dy := range hsScrolls[1:] {
		t.Run(fmt.Sprint(dy), func(t *testing.T) {
			g := newHSGrid(t, 520)
			b := g.header("Name").Bounds()
			g.scroll(0, dy)
			// The header is wider than its two cells: the points past the last cell
			// are on the header itself. The last is near its bottom edge.
			for _, p := range [][2]float32{{b.X + b.W/2, b.Y + b.H/2}, {b.X + b.W + 40, b.Y + b.H/2}, {b.X + b.W + 40, b.Y + b.H - 2}} {
				g.pressAt(p[0], p[1], input.ButtonLeft, 1)
				g.pressAt(p[0], p[1], input.ButtonLeft, 2)
				if got := g.sel.Peek(); len(got) != 0 {
					t.Errorf("a press at %v on the header selected the rows %v", p, got)
				}
				if len(g.activate) != 0 {
					t.Errorf("a double-click at %v on the header activated the rows %v", p, g.activate)
				}
			}
		})
	}
}

// TestDataGridHeaderResizeAfterScroll checks that the edge of a header cell
// resizes its column by a drag, and fits it by a double-click, after the rows
// have scrolled.
func TestDataGridHeaderResizeAfterScroll(t *testing.T) {
	for _, dy := range hsScrolls[1:] {
		t.Run(fmt.Sprint(dy), func(t *testing.T) {
			g := newHSGrid(t, 520)
			g.scroll(0, dy)
			b := g.header("ID").Bounds()
			g.dragFrom(b.X+b.W-3, b.Y+b.H/2, 40)
			if got := g.widths.Peek()["ID"]; got != 160 {
				t.Errorf("a drag of the edge by 40 after a scroll by %v made ID %v wide, want 160", dy, got)
			}
			if got := g.header("ID").Bounds().W; got != b.W+40 {
				t.Errorf("after the drag the ID header is %v wide, want %v", got, b.W+40)
			}
			if got := g.sort.Peek(); got.Key != "" {
				t.Errorf("the drag of the edge sorted by %q", got.Key)
			}
			b = g.header("ID").Bounds()
			g.pressAt(b.X+b.W-3, b.Y+b.H/2, input.ButtonLeft, 1)
			g.pressAt(b.X+b.W-3, b.Y+b.H/2, input.ButtonLeft, 2)
			if got := g.widths.Peek()["ID"]; got >= 160 {
				t.Errorf("a double-click on the edge left ID %v wide, want it fitted to its text", got)
			}
		})
	}
}

// TestDataGridHeaderMoveAndMenuAfterScroll checks that a movable header is moved
// by a drag, and that a right-press on a header opens the header's menu, after
// the rows have scrolled.
func TestDataGridHeaderMoveAndMenuAfterScroll(t *testing.T) {
	g := newHSGrid(t, 520)
	g.scroll(0, 200)
	n, id := g.header("Name").Bounds(), g.header("ID").Bounds()
	g.dragFrom(n.X+n.W/3, n.Y+n.H/2, id.X+id.W/4-(n.X+n.W/3))
	if got := g.order.Peek(); !slices.Equal(got, []string{"Name", "ID"}) {
		t.Errorf("dropping Name before ID after a scroll left the order %v, want [Name ID]", got)
	}
	g.header("ID").RightClick()
	if !slices.Equal(g.menus, []string{"ID"}) {
		t.Errorf("a right-press on the ID header after a scroll built the menus %v, want [ID]", g.menus)
	}
	if !g.u.HasRole(access.RoleMenuItem, "Hide column") {
		t.Error("the right-press on the header opened no menu")
	}
}

// TestDataGridHeaderClickAfterSidewaysScroll checks that the header cells take
// clicks where they are drawn when the columns are scrolled sideways as well as
// down: a column that scrolls, and the pinned columns, which do not.
func TestDataGridHeaderClickAfterSidewaysScroll(t *testing.T) {
	text := func(r hsRow) string { return fmt.Sprint(r.ID) }
	cols := []ui.DataColumn[hsRow]{{Header: "Pin", Text: text, Width: 100, Pin: ui.PinStart, Compare: ui.SortByText(text)}}
	for i := range 8 {
		h := fmt.Sprintf("C%d", i)
		cols = append(cols, ui.DataColumn[hsRow]{Header: h, Text: text, Width: 100, Compare: ui.SortByText(text)})
	}
	cols = append(cols, ui.DataColumn[hsRow]{Header: "End", Text: text, Width: 100, Pin: ui.PinEnd, Compare: ui.SortByText(text)})
	for _, dy := range hsScrolls {
		t.Run(fmt.Sprint(dy), func(t *testing.T) {
			g := newHSGrid(t, 520, cols...)
			pin, end := g.header("Pin").Bounds(), g.header("End").Bounds()
			g.scroll(250, dy)
			if got := g.header("Pin").Bounds(); got != pin {
				t.Errorf("the start-pinned header moved from %+v to %+v", pin, got)
			}
			if got := g.header("End").Bounds(); got != end {
				t.Errorf("the end-pinned header moved from %+v to %+v", end, got)
			}
			for _, name := range []string{"Pin", "C3", "End"} {
				g.header(name).Click()
				if got := g.sort.Peek(); got.Key != name {
					t.Errorf("a click on %s after a scroll by (250, %v) sorted by %q", name, dy, got.Key)
				}
			}
			if got := g.sel.Peek(); len(got) != 0 {
				t.Errorf("the clicks on the header selected the rows %v", got)
			}
		})
	}
}

// TestDataGridGroupedHeaderAfterScroll checks that a header with a row of groups
// above its cells takes presses where it is drawn after the rows have scrolled: a
// click on a cell sorts, and a press on a group's cell does not reach the row
// under it.
func TestDataGridGroupedHeaderAfterScroll(t *testing.T) {
	text := func(r hsRow) string { return fmt.Sprint(r.ID) }
	cols := []ui.DataColumn[hsRow]{
		{Header: "ID", Text: text, Width: 120, Group: "Run", Compare: ui.SortByText(text)},
		{Header: "Name", Text: text, Width: 120, Group: "Run", Compare: ui.SortByText(text)},
	}
	for _, dy := range hsScrolls {
		t.Run(fmt.Sprint(dy), func(t *testing.T) {
			g := newHSGrid(t, 520, cols...)
			before := g.header("Name").Bounds()
			g.scroll(0, dy)
			if got := g.header("Name").Bounds(); got != before {
				t.Errorf("after a scroll by %v the header item is at %+v, want %+v", dy, got, before)
			}
			// The groups' row is above the header cells.
			g.pressAt(before.X, before.Y-before.H/2, input.ButtonLeft, 1)
			if got := g.sel.Peek(); len(got) != 0 {
				t.Errorf("a press on the group's cell selected the rows %v", got)
			}
			g.header("Name").Click()
			if got := g.sort.Peek(); got.Key != "Name" {
				t.Errorf("a click on the header after a scroll by %v sorted by %q, want Name", dy, got.Key)
			}
		})
	}
}

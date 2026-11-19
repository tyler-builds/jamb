package ui_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// gapRig is a grid of 40 orders in the theme th that selects cells (or rows, with
// rowMode), with row headers, striped when striped is set, width wide.
type gapRig struct {
	*uitest.UI
	ranges *ui.Signal[[]ui.CellRange]
	picked *ui.Signal[[]string]
}

func newGapRig(t *testing.T, th *ui.Theme, rowMode, striped bool, wide float32) *gapRig {
	r := &gapRig{ranges: ui.NewSignal([]ui.CellRange(nil)), picked: ui.NewSignal([]string(nil))}
	r.UI = uitest.New(t, func(ctx *ui.Context) ui.Element {
		g := ui.DataGrid(orders(40), orderKey, orderColumns()...).Label("Results").RowHeaders().
			Density(ui.DataGridCompact).Width(ui.Px(wide)).Style(ui.Style{Height: ui.Px(300)})
		if rowMode {
			g.Selection(ui.SelectionMultiple, r.picked)
		} else {
			g.CellRanges(ui.SelectionMultiple, r.ranges)
		}
		if striped {
			g.Striped()
		}
		return ui.Div().Padding(8).Background(ctx.Theme().Background).Child(g)
	}, uitest.Size(520, 330), uitest.Theme(th))
	return r
}

// at returns the color of the screenshot at (x, y).
func at(img *image.RGBA, x, y float32) color.RGBA { return img.RGBAAt(int(x), int(y)) }

// cellAt returns the bounds of the cell shown at the 1-based row and column.
func (r *gapRig) cellAt(t *testing.T, row, col int) ui.Rect {
	t.Helper()
	var b access.Rect
	r.Tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleDataItem && n.GridRow == row && n.GridColumn == col {
			b = n.Bounds
		}
		return true
	})
	if b.Empty() {
		t.Fatalf("no cell at row %d, column %d", row, col)
	}
	return ui.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}
}

// checkJoined checks the separators of a selection of the rows from..to (1-based, as
// shown) in the columns 1 and 2: between two selected cells the separator has the
// cell's fill, under the last one it keeps its line, and in column 3, not selected,
// every separator keeps its line.
func (r *gapRig) checkJoined(t *testing.T, what string, from, to int) {
	t.Helper()
	img := r.Screenshot()
	for row := from; row <= to; row++ {
		plain := r.cellAt(t, row, 3) // not selected: its bottom is the separator's top
		sep := plain.Y + plain.H
		cell := r.cellAt(t, row, 1)
		x := cell.X + cell.W - 3 // clear of the text
		fill, line, other := at(img, x, sep-4), at(img, x, sep), at(img, plain.X+plain.W-3, sep)
		switch {
		case row < to && line != fill:
			t.Errorf("%s: the separator under row %d is %v between two selected cells, want their fill %v", what, row, line, fill)
		case row == to && line == fill:
			t.Errorf("%s: the separator under row %d, the last selected, has the fill %v; the edge should keep its line", what, row, line)
		case row == to && line != other:
			t.Errorf("%s: the separator under row %d is %v, want the line %v of the unselected column", what, row, line, other)
		}
		if other == fill {
			t.Errorf("%s: the separator under row %d in an unselected column has the selection's fill", what, row)
		}
		if h := r.cellAt(t, row, 1).Y; h != cell.Y || cell.Y != plain.Y {
			t.Errorf("%s: a joined cell of row %d starts at y %v, the others at %v", what, row, cell.Y, plain.Y)
		}
	}
}

// The selection's fill covers the separators between its rows, in a block of cells, a
// column and rows from their headers, in the dark and the light theme, striped or not,
// after the rows scroll up and down and the columns sideways. Its edge keeps the line.
func TestDataGridSelectionCoversTheSeparators(t *testing.T) {
	for _, th := range []*ui.Theme{ui.DarkTheme(), ui.LightTheme()} {
		for _, striped := range []bool{false, true} {
			name := th.Name
			if striped {
				name += ", striped"
			}
			r := newGapRig(t, th, false, striped, 500)
			keys := func(from, to int) []string {
				var ks []string
				for i := from; i <= to; i++ {
					ks = append(ks, orderKey(order{ID: 1000 + i}))
				}
				return ks
			}
			r.ranges.Set([]ui.CellRange{{Rows: ui.KeySet{Keys: keys(1, 4)}, Columns: ui.KeySet{Keys: []string{"OrderID", "Customer"}}}})
			r.Frame()
			r.checkJoined(t, name+", a block", 2, 5)
			r.ranges.Set([]ui.CellRange{{Rows: ui.KeySet{Keys: keys(1, 4)}, Columns: ui.KeySet{Keys: []string{"OrderID"}}},
				{Rows: ui.KeySet{Keys: keys(1, 4)}, Columns: ui.KeySet{Keys: []string{"Customer"}}}})
			r.Frame()
			r.checkJoined(t, name+", two ranges side by side", 2, 5)
			// Down the rows, then the block again.
			r.ByRole(access.RoleDataGrid, "Results").Scroll(0, 200)
			r.Settle()
			r.ranges.Set([]ui.CellRange{{Rows: ui.KeySet{Keys: keys(12, 15)}, Columns: ui.KeySet{Keys: []string{"OrderID", "Customer"}}}})
			r.Frame()
			first := 0
			r.Tree().Walk(func(n *access.Node) bool {
				if n.Role == access.RoleDataItem && n.Name == "1012" && !n.States.Has(access.StateOffscreen) {
					first = n.GridRow
				}
				return true
			})
			if first == 0 {
				t.Fatalf("%s: row 1012 is not shown after the scroll", name)
			}
			r.checkJoined(t, name+", scrolled down", first, first+3)
		}
	}

	// Sideways: a narrow grid scrolled to its last column keeps the fill joined.
	r := newGapRig(t, ui.DarkTheme(), false, false, 260)
	r.ranges.Set([]ui.CellRange{{Rows: ui.KeySet{Keys: []string{"1001", "1002", "1003"}}, Columns: ui.KeySet{All: true}}})
	r.Frame()
	r.ByRole(access.RoleDataGrid, "Results").Scroll(200, 0)
	r.Settle()
	img := r.Screenshot()
	total := r.cellAt(t, 2, 4)
	x, sep := total.X+total.W-3, r.cellAt(t, 2, 4).Y+r.cellAt(t, 2, 4).H
	if total.X < 8 || at(img, x, sep-1) != at(img, x, sep-6) {
		t.Errorf("after a sideways scroll the separator under a selected Total cell is %v, want its fill %v (cell at %+v)",
			at(img, x, sep-1), at(img, x, sep-6), total)
	}
}

// In a grid that selects rows, the separator between two selected rows is not drawn,
// and the one under the last selected row is; their row headers join too.
func TestDataGridSelectedRowsCoverTheSeparators(t *testing.T) {
	for _, th := range []*ui.Theme{ui.DarkTheme(), ui.LightTheme()} {
		r := newGapRig(t, th, true, false, 500)
		r.picked.Set([]string{"1001", "1002", "1003", "1006"})
		r.Frame()
		img := r.Screenshot()
		for row, joined := range map[int]bool{2: true, 3: true, 4: false, 7: false} {
			c := r.cellAt(t, row, 3)
			x, sep := c.X+c.W-3, c.Y+c.H
			if got := at(img, x, sep) == at(img, x, sep-4); got != joined {
				t.Errorf("%s: the separator under row %d is %v against the row's %v; joined %v, want %v",
					th.Name, row, at(img, x, sep), at(img, x, sep-4), got, joined)
			}
			h := r.ByRole(access.RoleHeaderItem, itoaRow(row)).Bounds()
			hx := h.X + 3
			if got := at(img, hx, sep) == at(img, hx, sep-4); got != joined {
				t.Errorf("%s: the separator under row %d's header joined %v, want %v", th.Name, row, got, joined)
			}
		}
	}
}

func itoaRow(n int) string { return string(rune('0' + n)) }

// The focused cell's ring stays whole inside a selection whose cells cover the
// separators, on every side, though its neighbors' fills are opaque.
func TestDataGridFocusRingOverAJoinedSelection(t *testing.T) {
	for _, th := range []*ui.Theme{ui.DarkTheme(), ui.LightTheme()} {
		r := newGapRig(t, th, false, false, 500)
		b := r.cellAt(t, 4, 2)
		rt := r.Runtime()
		rt.PointerMove(b.X+b.W/2, b.Y+b.H/2, 0)
		rt.PointerDown(b.X+b.W/2, b.Y+b.H/2, input.ButtonLeft, 1, 0)
		rt.PointerUp(b.X+b.W/2, b.Y+b.H/2, input.ButtonLeft, 0)
		r.Frame()
		r.Press("ctrl+a")
		b = r.cellAt(t, 4, 2)
		img := r.Screenshot()
		ring := color.RGBA{uint8(th.FocusRing.R*255 + 0.5), uint8(th.FocusRing.G*255 + 0.5), uint8(th.FocusRing.B*255 + 0.5), 255}
		near := func(c color.RGBA) bool {
			d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
			return d(c.R, ring.R) < 24 && d(c.G, ring.G) < 24 && d(c.B, ring.B) < 24
		}
		for side, p := range map[string][2]float32{
			"top": {b.X + b.W/2, b.Y - 1}, "bottom": {b.X + b.W/2, b.Y + b.H},
			"left": {b.X - 1, b.Y + b.H/2}, "right": {b.X + b.W, b.Y + b.H/2},
		} {
			if c := at(img, p[0], p[1]); !near(c) {
				t.Errorf("%s: the focused cell's ring at its %s is %v, want near %v", th.Name, side, c, ring)
			}
		}
	}
}

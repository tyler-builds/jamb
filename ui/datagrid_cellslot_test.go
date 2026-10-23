package ui

import (
	"testing"
)

// dgCanvasRig is a grid of two rows: a Name column, and a Bar column whose cells hold a
// Full by Full box keyed "bar-<id>". The Bar column has the cell style bar.
func dgCanvasRig(bar Style) *accessRig {
	items := []dgItem{{ID: "a", Name: "Alpha"}, {ID: "b", Name: "Beta"}}
	return newAccessRig(func(ctx *Context) Element {
		cols := []DataColumn[dgItem]{
			{Header: "Name", Text: func(x dgItem) string { return x.Name }, Grow: 1},
			{Header: "Bar", Text: func(x dgItem) string { return "" }, Width: 60, CellStyle: bar,
				Cell: func(x dgItem) Element {
					return Div().Key("bar-"+x.ID).Size(Full, Full).Background(Hex("#ff0000"))
				}},
		}
		return DataGrid(items, func(x dgItem) string { return x.ID }, cols...).Label("Runs")
	})
}

// boxAndCell returns the rectangle of the box with the key, and the rectangle of the
// grid cell that holds it.
func boxAndCell(t *testing.T, r *accessRig, key string) (Rect, Rect) {
	t.Helper()
	box, ok := r.rt.Find(key)
	if !ok {
		t.Fatalf("the box %q is not drawn", key)
	}
	for _, n := range partNodes(r.rt, "DataGrid.Cell") {
		c := r.rt.nodeRect(n)
		if box.X >= c.X && box.X < c.X+c.W && box.Y >= c.Y && box.Y < c.Y+c.H {
			return box, c
		}
	}
	t.Fatalf("no grid cell holds the box %q at %v", key, box)
	return box, box
}

// A Cell slot's wrapper fills the height of its row, so content sized with Full is
// as tall as the row, and the cell style of a column clears the padding of its cells.
func TestDataGridCellSlotFillsRowHeightAndCellStyleClears(t *testing.T) {
	r := dgCanvasRig(Style{Clear: ClearPadding})
	r.frame()
	bar, barCell := boxAndCell(t, r, "bar-a")
	nameCell := r.rt.nodeRect(partNodes(r.rt, "DataGrid.Cell")[0])
	if bar.H != nameCell.H || bar.H < 20 {
		t.Errorf("the box is %v tall in a row %v tall, want the row's height", bar.H, nameCell.H)
	}
	if bar.X != barCell.X || bar.W != barCell.W {
		t.Errorf("the box is at x %v and %v wide in a cell at x %v and %v wide, want the cell's padding cleared",
			bar.X, bar.W, barCell.X, barCell.W)
	}
}

// Without a CellStyle the cells of a column keep the grid's Cell padding, so the box
// is inset from its cell.
func TestDataGridCellStyleIsPerColumn(t *testing.T) {
	r := dgCanvasRig(Style{})
	r.frame()
	bar, barCell := boxAndCell(t, r, "bar-a")
	if bar.X <= barCell.X {
		t.Errorf("the box is at x %v in a cell at x %v, want the default padding to inset it", bar.X, barCell.X)
	}
}

package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// testDataGridColumnParts checks that the parts of the columns' layout reach the
// nodes that draw them, and records each part in applied, for the drift test.
func testDataGridColumnParts(t *testing.T, applied map[string]bool) {
	t.Run("DropIndicator", func(t *testing.T) {
		applied["DataGrid.DropIndicator"] = true
		r := newAccessRig(func(ctx *Context) Element {
			return DataGrid(dgItems(3), func(x dgItem) string { return x.ID }, dgMoveColumns()...).Label("Runs").
				Styles(DataGridStyle{DropIndicator: Style{Background: markBG}})
		})
		f, d := r.rt.nodeRect(dgHeader(r, "A")), r.rt.nodeRect(dgHeader(r, "C"))
		x0, y := f.X+f.W/3, f.Y+f.H/2
		r.rt.PointerMove(x0, y, 0)
		r.rt.PointerDown(x0, y, input.ButtonLeft, 1, 0)
		r.rt.PointerMove(x0+dragThreshold, y, 0)
		r.frame()
		r.rt.PointerMove(d.X+d.W/4, y, 0)
		r.frame()
		if !anyMarked(r.rt, "DataGrid.DropIndicator", false) {
			t.Error("the style does not reach the drop indicator")
		}
		r.rt.PointerUp(d.X+d.W/4, y, input.ButtonLeft, 0)
	})
	pinned := func(st DataGridStyle) *accessRig {
		return newAccessRig(func(ctx *Context) Element {
			return DataGrid(dgItems(3), func(x dgItem) string { return x.ID }, dgPinColumns()...).Label("Runs").Styles(st)
		})
	}
	t.Run("CellPinned", func(t *testing.T) {
		applied["DataGrid.CellPinned"] = true
		r := pinned(DataGridStyle{CellPinned: Style{Background: markBG}})
		if !anyMarked(r.rt, "DataGrid.Cell", false) {
			t.Error("the style does not reach the cells of a pinned column")
		}
	})
	t.Run("PinnedEdge", func(t *testing.T) {
		applied["DataGrid.PinnedEdge"] = true
		r := pinned(DataGridStyle{PinnedEdge: Style{Background: markBG}})
		if !anyMarked(r.rt, "DataGrid.Cell", false) || !anyMarked(r.rt, "DataGrid.HeaderCell", false) {
			t.Error("the style does not reach the cells and the header cells at the edge of the pinned columns")
		}
	})
	t.Run("HeaderGroup", func(t *testing.T) {
		applied["DataGrid.HeaderGroup"] = true
		r := newAccessRig(func(ctx *Context) Element {
			return DataGrid(dgItems(3), func(x dgItem) string { return x.ID }, dgGroupColumns()...).Label("Runs").
				Styles(DataGridStyle{HeaderGroup: Style{Background: markBG}})
		})
		if !anyMarked(r.rt, "DataGrid.HeaderGroup", false) {
			t.Error("the style does not reach the cell of a group of columns")
		}
	})
	t.Run("CellSelected", func(t *testing.T) {
		applied["DataGrid.CellSelected"] = true
		r := newAccessRig(func(ctx *Context) Element {
			return DataGrid(dgItems(3), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").
				CellSelection(SelectionMultiple, NewSignal([]CellKey{{Row: "i1", Column: "N"}})).
				Styles(DataGridStyle{CellSelected: Style{Background: markBG}})
		})
		if !anyMarked(r.rt, "DataGrid.Cell", false) {
			t.Error("the style does not reach a selected cell")
		}
	})
}

package ui

import (
	"strconv"
	"sync"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// dgWideItem is a row of four columns, for the selection benchmarks.
type dgWideItem struct {
	ID         int
	A, B, C, D string
}

var (
	dgWideOnce sync.Once
	dgWideRows []dgWideItem
)

// dgSelectBenchRig is a grid of 100,000 rows of four columns that selects cells, kept
// as a key for each cell (CellSelection) when cells is set, or as ranges (CellRanges).
func dgSelectBenchRig(b *testing.B, cells bool) *accessRig {
	b.Helper()
	dgWideOnce.Do(func() {
		dgWideRows = make([]dgWideItem, 100_000)
		for i := range dgWideRows {
			s := strconv.Itoa(i)
			dgWideRows[i] = dgWideItem{ID: i, A: "a" + s, B: "b" + s, C: "c" + s, D: "d" + s}
		}
	})
	col := func(h string, f func(dgWideItem) string) DataColumn[dgWideItem] {
		return DataColumn[dgWideItem]{Header: h, Text: f, Width: 100}
	}
	cs := []DataColumn[dgWideItem]{
		col("A", func(x dgWideItem) string { return x.A }), col("B", func(x dgWideItem) string { return x.B }),
		col("C", func(x dgWideItem) string { return x.C }), col("D", func(x dgWideItem) string { return x.D }),
	}
	cellSig, rangeSig := NewSignal([]CellKey(nil)), NewSignal([]CellRange(nil))
	return newAccessRig(func(ctx *Context) Element {
		g := DataGrid(dgWideRows, func(x dgWideItem) string { return strconv.Itoa(x.ID) }, cs...).Label("Results")
		if cells {
			return g.CellSelection(SelectionMultiple, cellSig)
		}
		return g.CellRanges(SelectionMultiple, rangeSig)
	})
}

// benchSelectAll times Ctrl+A on 100,000 rows of four columns, and its frame. When
// then is set, it times a Ctrl+click that follows the Ctrl+A instead.
func benchSelectAll(b *testing.B, cells, then bool) {
	r := dgSelectBenchRig(b, cells)
	dgClick(r, dgCell(r, 2, 1), 0)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		dgClick(r, dgCell(r, 2, 1), 0)
		if then {
			r.press(input.KeyA, input.ModCtrl)
		}
		b.StartTimer()
		if then {
			dgClick(r, dgCell(r, 3, 2), input.ModCtrl)
		} else {
			r.press(input.KeyA, input.ModCtrl)
		}
	}
}

// BenchmarkDataGridSelectAll100k is Ctrl+A on a grid of 100,000 rows of four columns,
// with the selection kept as cells (CellSelection) and as ranges (CellRanges).
func BenchmarkDataGridSelectAll100k(b *testing.B) {
	b.Run("cells", func(b *testing.B) { benchSelectAll(b, true, false) })
	b.Run("ranges", func(b *testing.B) { benchSelectAll(b, false, false) })
}

// BenchmarkDataGridCtrlClickAfterSelectAll100k is a Ctrl+click on a cell after Ctrl+A
// on a grid of 100,000 rows of four columns, which takes the cell out of the selection.
func BenchmarkDataGridCtrlClickAfterSelectAll100k(b *testing.B) {
	b.Run("cells", func(b *testing.B) { benchSelectAll(b, true, true) })
	b.Run("ranges", func(b *testing.B) { benchSelectAll(b, false, true) })
}

package ui

import "testing"

// TestDataGridDensityChangeKeepsTheTop checks that switching the density of a grid
// that is at the top keeps it at the top: the first row starts under the header, and
// the scroll offset stays at zero. The rows are measured at their new height, and the
// offset is rescaled from the first row, not from the top of the padding.
func TestDataGridDensityChangeKeepsTheTop(t *testing.T) {
	density := NewSignal(DataGridComfortable)
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(40), func(x dgItem) string { return x.ID }, dgColumns()...).
			Density(density.Get()).Label("Runs")
	})
	r.frame()
	density.Set(DataGridCompact)
	for range 4 {
		r.frame()
	}
	body := dgBody(t, r)
	if body.st.scrollY != 0 || body.st.targetY != 0 {
		t.Errorf("the offset after a density change is %v (target %v), want 0", body.st.scrollY, body.st.targetY)
	}
	header := partNodes(r.rt, "DataGrid.Header")[0]
	first := dgRowNode(t, r, "i0")
	if got, want := r.rt.nodeRect(first).Y, r.rt.nodeRect(header).Y+r.rt.nodeRect(header).H; got != want {
		t.Errorf("the first row starts at %v, want %v (under the header)", got, want)
	}
}

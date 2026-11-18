package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// TestDataGridCopyRefusesTooManyCells checks that a selection of more cells than a
// copy takes is not copied, and that a message says so.
func TestDataGridCopyRefusesTooManyCells(t *testing.T) {
	defer func(m int) { dataGridCopyMax = m }(dataGridCopyMax)
	dataGridCopyMax = 10
	var got []access.Update
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(6), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").
			CellRanges(SelectionMultiple, nil)
	})
	r.bridge.Listen(func(u access.Update) { got = append(got, u) })
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.rt.setClipboard("before")
	r.press(input.KeyA, input.ModCtrl)
	r.press(input.KeyC, input.ModCtrl)
	if s, _ := r.rt.clipboard(); s != "before" {
		t.Errorf("a copy of 12 cells, over a limit of 10, put %q on the clipboard", s)
	}
	said := ""
	for _, u := range got {
		for _, a := range u.Announcements {
			said += a.Text
		}
	}
	if !strings.Contains(said, "12 cells") {
		t.Errorf("the announcements %q do not say the copy was refused", said)
	}
	r.press(input.KeyDown, 0) // one cell
	r.press(input.KeyC, input.ModCtrl)
	if s, _ := r.rt.clipboard(); s != "Item 000002" {
		t.Errorf("a copy of one cell put %q on the clipboard", s)
	}
}

// BenchmarkDataGridCopy100k is Ctrl+C of every cell of 100,000 rows of four columns.
func BenchmarkDataGridCopy100k(b *testing.B) {
	r := dgSelectBenchRig(b, false)
	dgClick(r, dgCell(r, 2, 1), 0)
	r.press(input.KeyA, input.ModCtrl)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		r.press(input.KeyC, input.ModCtrl)
	}
	b.StopTimer()
	if s, _ := r.rt.clipboard(); len(s) < 100_000*4*5 {
		b.Fatalf("the copy is %d bytes", len(s))
	}
}

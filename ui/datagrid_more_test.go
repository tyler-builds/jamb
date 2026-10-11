package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"

	"github.com/tyler-builds/jamb/input"
)

// TestDataGridAutoFitOnDoubleClick checks that a double-click on the edge of a
// resizable header fits the column to the text of the rows in view, and that the
// fit is no narrower than the header.
func TestDataGridAutoFitOnDoubleClick(t *testing.T) {
	long := []DataColumn[dgItem]{
		{Header: "N", Text: func(x dgItem) string { return "a much longer value than the header" }, Width: 60, Resizable: true},
	}
	r := dgRig(dgItems(12), SelectionNone, nil, NewSignal(SortState{}), nil, long...)
	r.frame()
	before := dgHeader(r, "N").div.style.Basis.Value
	dgDoubleClick(r, dgResizeHandle(t, r, "N"))
	after := dgHeader(r, "N").div.style.Basis.Value
	if after <= before || after < textWidth(r.rt, "a much longer value than the header") {
		t.Errorf("double-click on the edge gave width %v from %v; want the width of the longest text in view", after, before)
	}
}

// dgResizeHandle returns the resize handle of the header cell named name.
func dgResizeHandle(t *testing.T, r *accessRig, name string) *node {
	t.Helper()
	for _, n := range partNodes(r.rt, "DataGrid.ResizeHandle") {
		if n.parent != nil && n.parent.div.acc != nil && n.parent.div.acc.name == name {
			return n
		}
	}
	t.Fatalf("no resize handle on the header %q", name)
	return nil
}

// TestDataGridResizeHandleShowsTheResizeCursor checks that the edge of a resizable
// header has the east-west resize cursor, and that a column that is not resizable
// has no handle.
func TestDataGridResizeHandleShowsTheResizeCursor(t *testing.T) {
	r := dgRig(dgItems(3), SelectionNone, nil, NewSignal(SortState{}), nil)
	if h := dgResizeHandle(t, r, "Name"); h.style.Cursor != input.CursorResizeEW {
		t.Errorf("the resize handle has cursor %v, want the east-west resize cursor", h.style.Cursor)
	}
	fixed := []DataColumn[dgItem]{{Header: "Fixed", Text: func(x dgItem) string { return x.Name }}}
	r = dgRig(dgItems(3), SelectionNone, nil, NewSignal(SortState{}), nil, fixed...)
	if n := len(partNodes(r.rt, "DataGrid.ResizeHandle")); n != 0 {
		t.Errorf("a column that is not resizable has %d resize handles", n)
	}
}

// TestDataGridEmptySlotReplacesTheMessage checks that Empty shows the element it is
// given, in place of "No rows".
func TestDataGridEmptySlotReplacesTheMessage(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid([]dgItem(nil), func(x dgItem) string { return x.ID }, dgColumns()...).
			Label("Runs").Empty(Text("Nothing has run yet"))
	})
	if n := r.tree().Find(access.RoleStaticText, "Nothing has run yet"); n == nil {
		t.Error("the empty slot does not show its element")
	}
	if n := r.tree().Find(access.RoleStaticText, "No rows"); n != nil {
		t.Error("the default message is shown with an empty slot")
	}
}

// TestDataGridSpaceToggleAndShiftSpace checks that Space toggles the focused row of a
// multiple selection, and that Shift+Space selects the rows from the anchor.
func TestDataGridSpaceToggleAndShiftSpace(t *testing.T) {
	sel := NewSignal([]string(nil))
	r := dgRig(dgItems(10), SelectionMultiple, sel, NewSignal(SortState{}), nil)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0) // row 1, selected by the plain move
	r.press(input.KeySpace, input.ModCtrl)
	if got := len(sel.Peek()); got != 0 {
		t.Errorf("Space on the selected row left %d rows selected, want none", got)
	}
	r.press(input.KeySpace, 0)
	if got := len(sel.Peek()); got != 1 {
		t.Errorf("Space on an unselected row selected %d rows, want the one", got)
	}
	r.press(input.KeyDown, input.ModShift)
	r.press(input.KeyDown, input.ModShift)
	r.press(input.KeySpace, input.ModShift)
	if got := len(sel.Peek()); got != 3 {
		t.Errorf("Shift+Space after Shift+Down twice selected %d rows, want three", got)
	}
}

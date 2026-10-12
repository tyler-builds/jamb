package ui

import (
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// fastKeysRig is a grid of 100 rows at 225% text in a short window. Its last column
// is editable, and is wider than the window, so the arrows must scroll it.
func fastKeysRig() *accessRig {
	rows := NewSignal(dgItems(100))
	sel := NewSignal([]string{})
	r := newAccessRig(func(ctx *Context) Element {
		cols := []DataColumn[dgItem]{
			{Header: "Name", Text: func(x dgItem) string { return x.Name }, Width: 180},
			{Header: "N", Text: func(x dgItem) string { return strconv.Itoa(x.N) }, Width: 120},
			{Header: "A", Text: func(x dgItem) string { return "a" }, Width: 120},
			{Header: "B", Text: func(x dgItem) string { return "b" }, Width: 120},
			{Header: "C", Text: func(x dgItem) string { return "c" }, Width: 120},
			{Header: "Label", Text: func(x dgItem) string { return x.Name }, Width: 200,
				Edit: func(x dgItem, v string) error { return nil }},
		}
		return DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, cols...).Label("Runs").
			Selection(SelectionMultiple, sel)
	})
	r.rt.SetSystemSettings(SystemSettings{TextScale: 225})
	r.rt.SetViewport(640, 300, 1)
	r.frame()
	return r
}

// TestArrowKeysBeforeAFrameEachMoveTheCursor checks that keys delivered before the
// grid is built again all act: a real keyboard (or SendKeys) can send Ctrl+Home, five
// Right presses and F2 before the next frame, and F2 must then open the editor of the
// last column. A key handler that read the cursor of the last render moved the cursor
// once for all five, and F2 opened nothing.
func TestArrowKeysBeforeAFrameEachMoveTheCursor(t *testing.T) {
	r := fastKeysRig()
	key := func(k input.Key, m input.Modifiers) {
		r.rt.KeyDown(k, m, false)
		r.rt.KeyUp(k, m)
	}
	dgPress(r, input.KeyTab)
	r.frame()
	key(input.KeyHome, input.ModCtrl)
	for i := 0; i < 5; i++ {
		key(input.KeyRight, 0)
	}
	key(input.KeyF2, 0)
	r.frame()
	r.frame()
	if !r.focusedAccess(t, access.RoleTextField, "Label") {
		t.Fatal("F2 after five Right presses did not open the editor of the Label column")
	}
}

// TestArrowKeysAtLargeTextScrollTheCellIntoView checks that the arrows reveal the cell
// they move to at 225%, so the editor opened by F2 is on screen.
func TestArrowKeysAtLargeTextScrollTheCellIntoView(t *testing.T) {
	r := fastKeysRig()
	dgPress(r, input.KeyTab)
	r.frame()
	for i := 0; i < 5; i++ {
		dgPress(r, input.KeyRight)
	}
	dgPress(r, input.KeyF2)
	r.frame()
	r.frame()
	if !r.focusedAccess(t, access.RoleTextField, "Label") {
		t.Fatal("the editor of the Label column did not take focus")
	}
	rc := r.rt.nodeRect(r.rt.byID[r.rt.focusID])
	// The column is wider than the window at this size, so its left edge, where the caret
	// is, must be on screen rather than the whole editor.
	if rc.X < 0 || rc.X >= 640 || rc.Y < 0 || rc.Y+rc.H > 300 {
		t.Fatalf("the editor of the Label column starts at %+v, outside a 640 by 300 window", rc)
	}
}

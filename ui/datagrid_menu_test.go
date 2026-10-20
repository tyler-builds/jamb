package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// dgMenuCall is a call of a grid's context menu builder: the row's key and the
// column, or the column of the header.
type dgMenuCall struct{ row, col string }

// dgMenuRig returns a rig with a grid of items in the selection mode m whose menus
// note their calls in calls. empty makes the row menu empty.
func dgMenuRig(items []dgItem, m SelectionMode, sel *Signal[[]string], calls *[]dgMenuCall, empty bool) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(items, func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").
			Selection(m, sel).
			ContextMenu(func(row dgItem, column string) []MenuItem {
				*calls = append(*calls, dgMenuCall{row.ID, column})
				if empty {
					return nil
				}
				return []MenuItem{{Label: "Copy"}}
			}).
			HeaderContextMenu(func(column string) []MenuItem {
				*calls = append(*calls, dgMenuCall{"", column})
				return []MenuItem{{Label: "Hide column"}}
			})
	})
}

// dgRightPress presses and releases the right button on n.
func dgRightPress(r *accessRig, n *node) {
	rc := r.rt.nodeRect(n)
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonRight, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonRight, 0)
	r.frame()
}

// TestDataGridRightPressSelectsAnUnselectedRow checks the rule of File Explorer: a
// right-press on a row that is not selected selects it and focuses the cell, and one
// on a row of a multiple selection keeps the selection.
func TestDataGridRightPressSelectsAnUnselectedRow(t *testing.T) {
	var calls []dgMenuCall
	sel := NewSignal([]string{"i1", "i2"})
	r := dgMenuRig(dgItems(10), SelectionMultiple, sel, &calls, false)
	dgRightPress(r, dgCell(r, 2, 1))
	if len(calls) != 1 || calls[0] != (dgMenuCall{"i2", "N"}) {
		t.Fatalf("a right-press on row 2 of N built the menus %v, want one for i2, N", calls)
	}
	if got := sel.Peek(); len(got) != 2 {
		t.Errorf("a right-press on a selected row changed the selection to %v", got)
	}
	if r.rt.ctxMenu == nil {
		t.Fatal("no menu opened")
	}
	r.rt.closeContextMenu(false)
	r.frame()
	dgRightPress(r, dgCell(r, 5, 0))
	if got := sel.Peek(); len(got) != 1 || got[0] != "i5" {
		t.Errorf("a right-press on an unselected row left the selection %v, want [i5]", got)
	}
	if f := dgFocusCell(r); f == nil || f.div.acc.gridRow != 6 || f.div.acc.gridCol != 1 {
		t.Errorf("the right-press did not move the focus to its cell: %+v", f)
	}
	if last := calls[len(calls)-1]; last != (dgMenuCall{"i5", "Name"}) {
		t.Errorf("the menu was built for %v, want i5, Name", last)
	}
}

// TestDataGridKeyboardMenuIsTheFocusedCells checks that Shift+F10 builds the menu of
// the focused cell, and the header's menu while the focus is on the header.
func TestDataGridKeyboardMenuIsTheFocusedCells(t *testing.T) {
	var calls []dgMenuCall
	r := dgMenuRig(dgItems(10), SelectionSingle, nil, &calls, false)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0)
	r.press(input.KeyRight, 0)
	r.press(input.KeyF10, input.ModShift)
	if len(calls) != 1 || calls[0] != (dgMenuCall{"i2", "N"}) {
		t.Fatalf("Shift+F10 on row 2 of N built %v, want one menu for i2, N", calls)
	}
	if r.rt.ctxMenu == nil || !r.rt.ctxMenu.keyboard {
		t.Fatal("Shift+F10 opened no keyboard menu")
	}
	r.press(input.KeyEscape, 0)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyHome, input.ModCtrl)
	r.press(input.KeyUp, 0) // the header
	r.press(input.KeyMenu, 0)
	if last := calls[len(calls)-1]; last != (dgMenuCall{"", "Name"}) {
		t.Errorf("the Menu key on the header built %v, want the header menu of Name", last)
	}
}

// TestDataGridHeaderMenuOnTheHeader checks that a right-press on a header cell opens
// the header's menu, and not the menu of the cells.
func TestDataGridHeaderMenuOnTheHeader(t *testing.T) {
	var calls []dgMenuCall
	sort := NewSignal(SortState{})
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(5), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").Sort(sort).
			HeaderContextMenu(func(column string) []MenuItem {
				calls = append(calls, dgMenuCall{"", column})
				return []MenuItem{{Label: "Hide column"}}
			})
	})
	dgRightPress(r, dgHeader(r, "N"))
	if len(calls) != 1 || calls[0] != (dgMenuCall{"", "N"}) {
		t.Fatalf("a right-press on the N header built %v, want its header menu", calls)
	}
	if r.rt.ctxMenu == nil {
		t.Error("the header menu did not open")
	}
	if got := sort.Peek(); got != (SortState{}) {
		t.Errorf("a right-press on the header sorted the grid: %+v", got)
	}
}

// TestDataGridEmptyMenuOpensNothing checks that a menu builder that returns no items
// opens nothing.
func TestDataGridEmptyMenuOpensNothing(t *testing.T) {
	var calls []dgMenuCall
	r := dgMenuRig(dgItems(5), SelectionSingle, nil, &calls, true)
	dgRightPress(r, dgCell(r, 1, 0))
	if len(calls) != 1 {
		t.Fatalf("the builder ran %d times, want once", len(calls))
	}
	if r.rt.ctxMenu != nil {
		t.Error("an empty menu opened")
	}
}

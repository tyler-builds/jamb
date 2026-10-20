package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// dgMoveColumns are three columns that the user can move and resize, A, B and C,
// each narrow enough for the three to fit the grid.
func dgMoveColumns() []DataColumn[dgItem] {
	text := func(x dgItem) string { return x.ID }
	return []DataColumn[dgItem]{
		{Header: "A", Text: text, Width: 100, Movable: true, Resizable: true, Compare: SortByText(text)},
		{Header: "B", Text: text, Width: 100, Movable: true, Resizable: true},
		{Header: "C", Text: text, Width: 100, Movable: true, Resizable: true},
	}
}

// dgMoveRig returns a rig with a grid of the columns cols, the order in order, the
// sort in sort, and the changes of the layout of the columns in changes.
func dgMoveRig(order *Signal[[]string], sort *Signal[SortState], changes *[]ColumnState, cols ...DataColumn[dgItem]) *accessRig {
	if len(cols) == 0 {
		cols = dgMoveColumns()
	}
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(5), func(x dgItem) string { return x.ID }, cols...).Label("Runs").
			ColumnOrder(order).Sort(sort).
			OnColumnsChange(func(s ColumnState) { *changes = append(*changes, s) })
	})
}

// dgHeaderOrder returns the names of the header cells, left to right.
func dgHeaderOrder(r *accessRig) []string {
	hs := partNodes(r.rt, "DataGrid.HeaderCell")
	slices.SortFunc(hs, func(a, b *node) int {
		return int(r.rt.nodeRect(a).X - r.rt.nodeRect(b).X)
	})
	var out []string
	for _, h := range hs {
		out = append(out, h.div.acc.name)
	}
	return out
}

// dgDragHeader drags the header cell from onto the header cell to, at its left or
// right quarter, and reports the number of drop indicators while it is over it.
func dgDragHeader(r *accessRig, from, to string, after bool) int {
	f, d := r.rt.nodeRect(dgHeader(r, from)), r.rt.nodeRect(dgHeader(r, to))
	x0, y0 := f.X+f.W/3, f.Y+f.H/2
	x1, y1 := d.X+d.W/4, d.Y+d.H/2
	if after {
		x1 = d.X + d.W*3/4
	}
	r.rt.PointerMove(x0, y0, 0)
	r.frame()
	r.rt.PointerDown(x0, y0, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(x0+dragThreshold, y0, 0)
	r.frame()
	r.rt.PointerMove(x1, y1, 0)
	r.frame()
	lines := len(partNodes(r.rt, "DataGrid.DropIndicator"))
	r.rt.PointerUp(x1, y1, input.ButtonLeft, 0)
	r.frame()
	return lines
}

// TestDataGridDropReordersTheColumns checks that dropping a header beside another
// moves its column there, reports the new order, and draws the indicator meanwhile.
func TestDataGridDropReordersTheColumns(t *testing.T) {
	var changes []ColumnState
	order := NewSignal([]string(nil))
	r := dgMoveRig(order, NewSignal(SortState{}), &changes)
	if lines := dgDragHeader(r, "A", "C", true); lines != 1 {
		t.Errorf("the drag drew %d drop indicators over C, want 1", lines)
	}
	if got := dgHeaderOrder(r); !slices.Equal(got, []string{"B", "C", "A"}) {
		t.Fatalf("after A was dropped after C the headers are %v, want B, C, A", got)
	}
	if len(changes) != 1 || !slices.Equal(changes[0].Order, []string{"B", "C", "A"}) {
		t.Errorf("the drop reported %v, want one change to B, C, A", changes)
	}
	if !slices.Equal(order.Peek(), []string{"B", "C", "A"}) {
		t.Errorf("the order signal is %v", order.Peek())
	}
	if n := len(partNodes(r.rt, "DataGrid.DropIndicator")); n != 0 {
		t.Errorf("%d drop indicators left after the drop", n)
	}
	dgDragHeader(r, "A", "B", false)
	if got := dgHeaderOrder(r); !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Errorf("after A was dropped before B the headers are %v, want A, B, C", got)
	}
	if got := dgCellText(r, 0, 0); got != "i0" {
		t.Errorf("column 1 of row 0 is %q after the moves", got)
	}
}

// TestDataGridMoveColumnWithKeyboard checks that Ctrl+Shift+Right and Left on a
// movable header move its column and announce its new place, and that the focus
// stays on the moved header.
func TestDataGridMoveColumnWithKeyboard(t *testing.T) {
	var changes []ColumnState
	r := dgMoveRig(nil, NewSignal(SortState{}), &changes)
	var said []string
	r.bridge.Listen(func(u access.Update) {
		for _, a := range u.Announcements {
			said = append(said, a.Text)
		}
	})
	takeAnnouncements()
	focusNode(r, dgBody(t, r))
	r.press(input.KeyUp, 0) // the header of A
	r.press(input.KeyRight, input.ModCtrl|input.ModShift)
	r.press(input.KeyRight, input.ModCtrl|input.ModShift)
	if got := dgHeaderOrder(r); !slices.Equal(got, []string{"B", "C", "A"}) {
		t.Fatalf("after two moves right the headers are %v, want B, C, A", got)
	}
	r.press(input.KeyRight, input.ModCtrl|input.ModShift) // already last
	if len(changes) != 2 {
		t.Errorf("the moves reported %d changes, want 2", len(changes))
	}
	for _, a := range takeAnnouncements() {
		said = append(said, a.Text)
	}
	if !slices.Contains(said, "A moved to position 3 of 3") {
		t.Errorf("announced %q, want A's new position", said)
	}
	h := dgHeader(r, "A")
	if h == nil || !h.div.acc.highlighted {
		t.Error("the focus did not stay on the moved A header")
	}
	r.press(input.KeyLeft, input.ModCtrl|input.ModShift)
	if got := dgHeaderOrder(r); !slices.Equal(got, []string{"B", "A", "C"}) {
		t.Errorf("after a move left the headers are %v, want B, A, C", got)
	}
}

// TestDataGridHeaderClickStillSorts checks that a click on a movable header sorts,
// since a press that does not move far is not a drag, and that Ctrl+Left still
// resizes, while Ctrl+Shift+Left on a column that cannot move resizes it as before.
func TestDataGridHeaderClickStillSorts(t *testing.T) {
	var changes []ColumnState
	sort := NewSignal(SortState{})
	r := dgMoveRig(nil, sort, &changes)
	dgClick(r, dgHeader(r, "A"), 0)
	if got := sort.Peek(); got != (SortState{Key: "A"}) {
		t.Errorf("a click on the movable A header sorted %+v, want by A", got)
	}
	focusNode(r, dgBody(t, r))
	r.press(input.KeyUp, 0)
	w := dgHeader(r, "A").div.style.Basis.Value
	r.press(input.KeyLeft, input.ModCtrl)
	if got := dgHeader(r, "A").div.style.Basis.Value; got != w-8 {
		t.Errorf("Ctrl+Left on a movable header gave width %v, want %v", got, w-8)
	}
	if got := dgHeaderOrder(r); !slices.Equal(got, []string{"A, sorted ascending", "B", "C"}) {
		t.Errorf("Ctrl+Left moved a column: %v", got)
	}

	fixed := dgMoveColumns()
	for i := range fixed {
		fixed[i].Movable = false
	}
	changes = nil
	r = dgMoveRig(nil, NewSignal(SortState{}), &changes, fixed...)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyUp, 0)
	r.press(input.KeyRight, input.ModCtrl|input.ModShift)
	if got := dgHeader(r, "A").div.style.Basis.Value; got != 108 {
		t.Errorf("Ctrl+Shift+Right on a column that cannot move gave width %v, want 108", got)
	}
	if n := dgDragHeader(r, "A", "C", true); n != 0 || !slices.Equal(dgHeaderOrder(r), []string{"A", "B", "C"}) {
		t.Errorf("a column that cannot move was dragged: %v", dgHeaderOrder(r))
	}
}

// TestDataGridMovableMakesEveryColumnMovable checks that the grid's Movable lets each
// column move.
func TestDataGridMovableMakesEveryColumnMovable(t *testing.T) {
	cols := dgMoveColumns()
	for i := range cols {
		cols[i].Movable = false
	}
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(3), func(x dgItem) string { return x.ID }, cols...).Label("Runs").Movable()
	})
	dgDragHeader(r, "C", "A", false)
	if got := dgHeaderOrder(r); !slices.Equal(got, []string{"C", "A", "B"}) {
		t.Errorf("with Movable the headers are %v after C was dropped before A, want C, A, B", got)
	}
}

// TestDataGridResizeDoesNotDragTheHeader checks that a drag of the edge of a movable
// column resizes it and does not move it.
func TestDataGridResizeDoesNotDragTheHeader(t *testing.T) {
	var changes []ColumnState
	cols := dgMoveColumns()
	cols[0].Compare = nil
	r := dgMoveRig(nil, NewSignal(SortState{}), &changes, cols...)
	rc := r.rt.nodeRect(dgResizeHandle(t, r, "A"))
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(x+20, y, 0)
	r.frame()
	r.rt.PointerMove(x+150, y, 0)
	r.frame()
	r.rt.PointerUp(x+150, y, input.ButtonLeft, 0)
	r.frame()
	if got := dgHeaderOrder(r); !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Errorf("a drag of A's edge moved the columns: %v", got)
	}
	if got := dgHeader(r, "A").div.style.Basis.Value; got != 250 {
		t.Errorf("a drag of A's edge by 150 px gave width %v, want 250", got)
	}
	if len(changes) != 1 || len(changes[0].Widths) != 1 {
		t.Errorf("the resize reported %v, want one change of A's width", changes)
	}
}

package ui

import (
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// dgProc is a row of the tree tests: a process, with the processes it groups.
type dgProc struct {
	ID, Name string
	Mem      int
	Kids     []dgProc
}

// dgProcTree is the tree of the tests. The group g1 has two children and g2 one, and
// l1 is a row with no children. The memory of a group is the sum of its children.
func dgProcTree() []dgProc {
	return []dgProc{
		{ID: "g1", Name: "chrome", Mem: 300, Kids: []dgProc{{ID: "c1", Name: "tab a", Mem: 100}, {ID: "c2", Name: "tab b", Mem: 200}}},
		{ID: "g2", Name: "zsh", Mem: 50, Kids: []dgProc{{ID: "s1", Name: "child", Mem: 50}}},
		{ID: "l1", Name: "leaf", Mem: 10},
	}
}

// dgProcColumns are the columns of the tree tests. Name is the first column, so it is
// the tree column.
func dgProcColumns() []DataColumn[dgProc] {
	return []DataColumn[dgProc]{
		{Header: "Name", Text: func(x dgProc) string { return x.Name }, Compare: SortByText(func(x dgProc) string { return x.Name }), Grow: 1},
		{Header: "Mem", Text: func(x dgProc) string { return strconv.Itoa(x.Mem) }, Compare: SortBy(func(x dgProc) int { return x.Mem }), Width: 80},
	}
}

// dgTreeRig returns a rig with the tree of dgProcTree, open by exp, selected by sel and
// sorted by sort. The events of OnExpand and OnCollapse are appended to ev as "+key"
// and "-key".
func dgTreeRig(rows []dgProc, exp, sel *Signal[[]string], sort *Signal[SortState], h *DataGridHandle, ev *[]string) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		g := DataGrid(rows, func(x dgProc) string { return x.ID }, dgProcColumns()...).Label("Processes").
			Selection(SelectionMultiple, sel).Sort(sort).
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(exp).
			OnExpand(func(k string) { *ev = append(*ev, "+"+k) }).
			OnCollapse(func(k string) { *ev = append(*ev, "-"+k) })
		if h != nil {
			g.Handle(h)
		}
		return g
	})
}

// dgFocused returns the key of the row and the 0-based column of the cell that has
// keyboard focus.
func dgFocused(t *testing.T, r *accessRig) (string, int) {
	t.Helper()
	c := dgFocusCell(r)
	if c == nil {
		t.Fatal("no cell has focus")
	}
	a := c.div.acc
	return dgRowKeys(r)[a.gridRow-1], a.gridCol - 1
}

// TestDataGridTreeShowsTheOpenRows checks that the rows shown are the depth-first
// flattening of the open rows, and that the open rows are the keys in the signal.
func TestDataGridTreeShowsTheOpenRows(t *testing.T) {
	var ev []string
	exp := NewSignal([]string{"g1"})
	r := dgTreeRig(dgProcTree(), exp, NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	if got, want := dgRowKeys(r), []string{"g1", "c1", "c2", "g2", "l1"}; !slices.Equal(got, want) {
		t.Fatalf("shown rows %v, want %v", got, want)
	}
	if len(ev) != 0 {
		t.Errorf("the app set the open rows, and OnExpand ran: %v", ev)
	}
}

// TestDataGridTreeChevronTogglesARow checks that a click on the chevron of a row opens
// and closes it, runs OnExpand and OnCollapse, and does not select the row. The
// chevron is not a Tab stop, and it is hidden from assistive technology.
func TestDataGridTreeChevronTogglesARow(t *testing.T) {
	var ev []string
	exp := NewSignal([]string(nil))
	sel := NewSignal([]string(nil))
	r := dgTreeRig(dgProcTree(), exp, sel, NewSignal(SortState{}), nil, &ev)
	chev := partNodesUnder(dgRowNode(t, r, "g2"), "DataGrid.TreeChevron")
	if len(chev) != 1 {
		t.Fatalf("the row of g2 has %d chevrons, want 1", len(chev))
	}
	if chev[0].div.focusable || !chev[0].div.acc.hidden {
		t.Error("the chevron is a Tab stop, or is not hidden from assistive technology")
	}
	dgClick(r, chev[0], 0)
	if !slices.Contains(exp.Peek(), "g2") {
		t.Fatalf("the open rows after the click are %v, want g2", exp.Peek())
	}
	if got, want := dgRowKeys(r), []string{"g1", "g2", "s1", "l1"}; !slices.Equal(got, want) {
		t.Fatalf("shown rows %v, want %v", got, want)
	}
	if len(sel.Peek()) != 0 {
		t.Errorf("the click on the chevron selected %v", sel.Peek())
	}
	dgClick(r, partNodesUnder(dgRowNode(t, r, "g2"), "DataGrid.TreeChevron")[0], 0)
	if slices.Contains(exp.Peek(), "g2") {
		t.Errorf("the second click left g2 open: %v", exp.Peek())
	}
	if want := []string{"+g2", "-g2"}; !slices.Equal(ev, want) {
		t.Errorf("events %v, want %v", ev, want)
	}
}

// TestDataGridTreeSortsWithinEachLevel checks that the sort orders the children of a
// row among their siblings, and the top rows among the top rows, by the column's Compare.
func TestDataGridTreeSortsWithinEachLevel(t *testing.T) {
	var ev []string
	sort := NewSignal(SortState{Key: "Mem", Desc: true})
	r := dgTreeRig(dgProcTree(), NewSignal([]string{"g1"}), NewSignal([]string(nil)), sort, nil, &ev)
	if got, want := dgRowKeys(r), []string{"g1", "c2", "c1", "g2", "l1"}; !slices.Equal(got, want) {
		t.Fatalf("shown rows by memory, descending: %v, want %v", got, want)
	}
	sort.Set(SortState{Key: "Mem"})
	r.frame()
	if got, want := dgRowKeys(r), []string{"l1", "g2", "g1", "c1", "c2"}; !slices.Equal(got, want) {
		t.Errorf("shown rows by memory, ascending: %v, want %v", got, want)
	}
}

// TestDataGridTreeKeysOpenMoveAndClose checks the keys of the tree column: Right opens a
// closed row, and on an open row moves to its first child; Left closes an open row, and
// on a row that is closed or a leaf moves to its parent. Elsewhere Left and Right move
// between the columns.
func TestDataGridTreeKeysOpenMoveAndClose(t *testing.T) {
	var ev []string
	exp := NewSignal([]string(nil))
	r := dgTreeRig(dgProcTree(), exp, NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0) // from g1 to g2
	if k, c := dgFocused(t, r); k != "g2" || c != 0 {
		t.Fatalf("focus on %s column %d, want g2 in the tree column", k, c)
	}
	r.press(input.KeyRight, 0)
	if !slices.Equal(exp.Peek(), []string{"g2"}) {
		t.Fatalf("Right on a closed row opened %v, want g2", exp.Peek())
	}
	if k, _ := dgFocused(t, r); k != "g2" {
		t.Fatalf("Right on a closed row moved the focus to %s", k)
	}
	r.press(input.KeyRight, 0)
	if k, _ := dgFocused(t, r); k != "s1" {
		t.Fatalf("Right on an open row moved the focus to %s, want its first child s1", k)
	}
	r.press(input.KeyLeft, 0)
	if k, _ := dgFocused(t, r); k != "g2" {
		t.Fatalf("Left on a leaf moved the focus to %s, want its parent g2", k)
	}
	r.press(input.KeyLeft, 0)
	if len(exp.Peek()) != 0 {
		t.Fatalf("Left on an open row left %v open", exp.Peek())
	}
	if k, c := dgFocused(t, r); k != "g2" || c != 0 {
		t.Fatalf("Left on the closed row moved the focus to %s column %d", k, c)
	}
	r.press(input.KeyLeft, 0) // a top row with no parent: nothing to do
	if k, c := dgFocused(t, r); k != "g2" || c != 0 {
		t.Fatalf("Left on a top row moved the focus to %s column %d", k, c)
	}
	r.press(input.KeyRight, 0)
	r.press(input.KeyRight, 0)
	if k, _ := dgFocused(t, r); k != "s1" {
		t.Fatalf("focus on %s, want s1", k)
	}
	r.press(input.KeyRight, 0) // a leaf: Right moves to the next column
	if k, c := dgFocused(t, r); k != "s1" || c != 1 {
		t.Fatalf("Right on a leaf moved the focus to %s column %d, want s1 column 1", k, c)
	}
	r.press(input.KeyLeft, 0) // not the tree column: Left moves back a column
	if k, c := dgFocused(t, r); k != "s1" || c != 0 {
		t.Fatalf("Left in another column moved the focus to %s column %d, want s1 column 0", k, c)
	}
}

// TestDataGridTreeCollapseMovesFocusToTheParent checks that a collapse that hides the
// focused row, from the signal or from the keys, moves the focus to the row above it
// that is shown, in the same column.
func TestDataGridTreeCollapseMovesFocusToTheParent(t *testing.T) {
	var ev []string
	exp := NewSignal([]string{"g2"})
	r := dgTreeRig(dgProcTree(), exp, NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0) // to s1, in g2
	r.press(input.KeyRight, 0)
	if k, c := dgFocused(t, r); k != "s1" || c != 1 {
		t.Fatalf("focus on %s column %d, want s1 column 1", k, c)
	}
	exp.Set(nil) // the app closes g2
	r.frame()
	if k, c := dgFocused(t, r); k != "g2" || c != 1 {
		t.Errorf("after the app closed g2 the focus is on %s column %d, want g2 column 1", k, c)
	}
}

// TestDataGridTreeScrollToRowOpensItsAncestors checks that ScrollToRow and FocusCell on
// a row that a collapse hides open the rows above it, through the signal, and then show
// it.
func TestDataGridTreeScrollToRowOpensItsAncestors(t *testing.T) {
	var ev []string
	h := NewDataGridHandle()
	exp := NewSignal([]string(nil))
	r := dgTreeRig(dgProcTree(), exp, NewSignal([]string(nil)), NewSignal(SortState{}), h, &ev)
	h.ScrollToRow("c2")
	dgSettle(r)
	if !slices.Equal(exp.Peek(), []string{"g1"}) {
		t.Fatalf("ScrollToRow(c2) left the open rows %v, want g1", exp.Peek())
	}
	if !slices.Contains(dgRowKeys(r), "c2") {
		t.Errorf("c2 is not shown after ScrollToRow: %v", dgRowKeys(r))
	}
	h.FocusCell("s1", "")
	dgSettle(r)
	if k, _ := dgFocused(t, r); k != "s1" {
		t.Errorf("FocusCell(s1) focused %s, want s1", k)
	}
	if want := []string{"+g1", "+g2"}; !slices.Equal(ev, want) {
		t.Errorf("events %v, want %v", ev, want)
	}
}

// TestDataGridTreeAccessibility checks the level, the position among the siblings, the
// size of the set and the expanded state that each row reports, and the number of rows
// that the grid reports, which is the number shown.
func TestDataGridTreeAccessibility(t *testing.T) {
	var ev []string
	r := dgTreeRig(dgProcTree(), NewSignal([]string{"g1"}), NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	check := func(key string, level, pos, size int, expandable, open bool) {
		t.Helper()
		a := dgRowNode(t, r, key).div.acc
		if a.level != level || a.setPos != pos || a.setSize != size {
			t.Errorf("%s: level %d position %d of %d, want level %d position %d of %d",
				key, a.level, a.setPos, a.setSize, level, pos, size)
		}
		if a.expandable != expandable || a.expanded != open {
			t.Errorf("%s: expandable %v expanded %v, want %v and %v", key, a.expandable, a.expanded, expandable, open)
		}
	}
	check("g1", 1, 1, 3, true, true)
	check("c1", 2, 1, 2, false, false)
	check("c2", 2, 2, 2, false, false)
	check("g2", 1, 2, 3, true, false)
	check("l1", 1, 3, 3, false, false)

	grid := r.stateOf(t, access.RoleDataGrid, "Processes")
	if grid.GridRows != 5 {
		t.Errorf("the grid reports %d rows, want the 5 shown", grid.GridRows)
	}
	row := accesstest.Find(t, r.tree(), access.RoleListItem, "chrome, 300")
	if row.Level != 1 || row.PosInSet != 1 || row.SetSize != 3 {
		t.Errorf("published row: level %d position %d of %d, want 1, 1 of 3", row.Level, row.PosInSet, row.SetSize)
	}
	if !row.States.Has(access.StateExpandable) || !row.States.Has(access.StateExpanded) {
		t.Errorf("published row states %q, want expandable and expanded", row.States)
	}
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

// TestDataGridTreeSelectAllSelectsTheShownRows checks that Ctrl+A selects the rows that
// are shown, and not the rows of the closed groups.
func TestDataGridTreeSelectAllSelectsTheShownRows(t *testing.T) {
	var ev []string
	sel := NewSignal([]string(nil))
	r := dgTreeRig(dgProcTree(), NewSignal([]string{"g1"}), sel, NewSignal(SortState{}), nil, &ev)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyA, input.ModCtrl)
	got := slices.Clone(sel.Peek())
	slices.Sort(got)
	if want := []string{"c1", "c2", "g1", "g2", "l1"}; !slices.Equal(got, want) {
		t.Errorf("Ctrl+A selected %v, want the shown rows %v", got, want)
	}
}

// TestDataGridTreeWarnsOnRepeatedKeys checks that a key used twice anywhere in the tree
// is reported, as a key of a flat grid is.
func TestDataGridTreeWarnsOnRepeatedKeys(t *testing.T) {
	var ev []string
	rows := []dgProc{{ID: "a", Name: "a", Kids: []dgProc{{ID: "a", Name: "child"}}}}
	r := dgTreeRig(rows, NewSignal([]string{"a"}), NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	r.frame()
	if len(r.warnings) == 0 {
		t.Error("a key used by a row and its child is not reported")
	}
}

// TestDataGridFlatHasNoTreeParts checks that a grid without Children builds no tree
// parts and reports its rows with no level, as before.
func TestDataGridFlatHasNoTreeParts(t *testing.T) {
	r := dgRig(dgItems(3), SelectionMultiple, NewSignal([]string(nil)), NewSignal(SortState{}), nil)
	if n := len(partNodes(r.rt, "DataGrid.TreeChevron")) + len(partNodes(r.rt, "DataGrid.TreeIndent")); n != 0 {
		t.Errorf("a flat grid builds %d tree parts", n)
	}
	a := dgRowNode(t, r, "i0").div.acc
	if a.level != 0 || a.expandable || a.setPos != 1 || a.setSize != 3 {
		t.Errorf("a flat row reports level %d expandable %v position %d of %d", a.level, a.expandable, a.setPos, a.setSize)
	}
}

// testDataGridTreeParts checks that the parts of the tree column reach the nodes that
// draw them, and records each part in applied, for the drift test.
func testDataGridTreeParts(t *testing.T, applied map[string]bool) {
	mark := Style{Background: markBG}
	applied["DataGrid.TreeIndent"], applied["DataGrid.TreeChevron"] = true, true
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgProcTree(), func(x dgProc) string { return x.ID }, dgProcColumns()...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(NewSignal([]string{"g1"})).
			Styles(DataGridStyle{TreeIndent: mark, TreeChevron: mark})
	})
	if !anyMarked(r.rt, "DataGrid.TreeIndent", false) {
		t.Error("the style does not reach the indent of a row below the top")
	}
	if !anyMarked(r.rt, "DataGrid.TreeChevron", false) {
		t.Error("the style does not reach the chevron of a row")
	}
}

// TestDataGridTreeEditsAChildRow checks that an edit of a row under an open group saves
// to that row, and not to the row at the same position in the flat order.
func TestDataGridTreeEditsAChildRow(t *testing.T) {
	var saved []string
	cols := dgProcColumns()
	cols[0].Edit = func(x dgProc, v string) error {
		saved = append(saved, x.ID+"="+v)
		return nil
	}
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgProcTree(), func(x dgProc) string { return x.ID }, cols...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(NewSignal([]string{"g1"}))
	})
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0) // to c2
	dgPress(r, input.KeyEnter)
	dgReplace(r, "Renamed")
	dgPress(r, input.KeyEnter)
	if want := []string{"c2=Renamed"}; !slices.Equal(saved, want) {
		t.Errorf("saved %v, want %v", saved, want)
	}
}

// TestDataGridTreeCellSelectionFollowsTheKeys checks that a cell of a child row is
// selected by its keys, as the cells of a flat grid are.
func TestDataGridTreeCellSelectionFollowsTheKeys(t *testing.T) {
	cells := NewSignal([]CellKey(nil))
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgProcTree(), func(x dgProc) string { return x.ID }, dgProcColumns()...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(NewSignal([]string{"g1"})).
			CellSelection(SelectionMultiple, cells)
	})
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0) // to c1, which a plain move selects
	if want := []CellKey{{Row: "c1", Column: "Name"}}; !slices.Equal(cells.Peek(), want) {
		t.Errorf("selected cells %v, want %v", cells.Peek(), want)
	}
}

// TestDataGridTreeShowsAChangeInPlace checks that a row whose values, or whose children,
// change in place in the slice of the tree show the change on the next frame, as the
// rows of a flat grid do.
func TestDataGridTreeShowsAChangeInPlace(t *testing.T) {
	rows := dgProcTree()
	again := NewSignal(0) // the app renders again after it changes the rows
	exp := NewSignal([]string{"g1"})
	r := newAccessRig(func(ctx *Context) Element {
		_ = again.Get()
		return DataGrid(rows, func(x dgProc) string { return x.ID }, dgProcColumns()...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(exp)
	})
	rows[0].Mem = 999
	rows[0].Kids[0].Name = "tab renamed"
	again.Set(1)
	r.frame()
	if got := dgCellText(r, 0, 1); got != "999" {
		t.Errorf("the memory of g1 is %q after it changed in place, want 999", got)
	}
	if got := dgCellText(r, 1, 0); got != "tab renamed" {
		t.Errorf("the name of c1 is %q after it changed in place, want tab renamed", got)
	}
	rows[0].Kids = append(rows[0].Kids, dgProc{ID: "c3", Name: "tab c", Mem: 5})
	again.Set(2)
	r.frame()
	if got, want := dgRowKeys(r), []string{"g1", "c1", "c2", "c3", "g2", "l1"}; !slices.Equal(got, want) {
		t.Errorf("shown rows %v after a child was added in place, want %v", got, want)
	}
}

// TestDataGridTreeRightTwiceBeforeAFrame checks that two Right presses with no frame
// between them open a closed row, and then move to its first child, and do not skip
// it to the row below.
func TestDataGridTreeRightTwiceBeforeAFrame(t *testing.T) {
	var ev []string
	exp := NewSignal([]string(nil))
	r := dgTreeRig(dgProcTree(), exp, NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0) // to g2
	r.rt.KeyDown(input.KeyRight, 0, false)
	r.rt.KeyUp(input.KeyRight, 0)
	r.rt.KeyDown(input.KeyRight, 0, false)
	r.rt.KeyUp(input.KeyRight, 0)
	r.frame()
	if !slices.Equal(exp.Peek(), []string{"g2"}) {
		t.Fatalf("the open rows are %v, want g2", exp.Peek())
	}
	if k, c := dgFocused(t, r); k != "s1" || c != 0 {
		t.Errorf("focus on %s column %d after two Right presses, want s1 column 0", k, c)
	}
	if want := []string{"+g2"}; !slices.Equal(ev, want) {
		t.Errorf("events %v, want %v", ev, want)
	}
}

// TestDataGridTreeStarOpensTheSiblings checks that the * key opens the closed rows among
// the siblings of the focused row that have children, and not the rows of another level.
func TestDataGridTreeStarOpensTheSiblings(t *testing.T) {
	var ev []string
	exp := NewSignal([]string(nil))
	r := dgTreeRig(dgProcTree(), exp, NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	focusNode(r, dgBody(t, r))
	r.typeText("*") // on g1: the top rows with children are g1 and g2
	if want := []string{"g1", "g2"}; !slices.Equal(exp.Peek(), want) {
		t.Fatalf("* on a top row opened %v, want %v", exp.Peek(), want)
	}
	if want := []string{"+g1", "+g2"}; !slices.Equal(ev, want) {
		t.Errorf("events %v, want %v", ev, want)
	}
	r.press(input.KeyDown, 0) // to c1, whose siblings have no children
	r.typeText("*")
	if want := []string{"g1", "g2"}; !slices.Equal(exp.Peek(), want) {
		t.Errorf("* on a leaf changed the open rows to %v, want %v", exp.Peek(), want)
	}
}

// TestDataGridTreeAccessibilityExpandAndCollapse checks that the expand and collapse
// actions of a row with children open and close it, through the same signal and events
// as the chevron, and that a row without children has neither.
func TestDataGridTreeAccessibilityExpandAndCollapse(t *testing.T) {
	var ev []string
	exp := NewSignal([]string(nil))
	r := dgTreeRig(dgProcTree(), exp, NewSignal([]string(nil)), NewSignal(SortState{}), nil, &ev)
	row := accesstest.Find(t, r.tree(), access.RoleListItem, "zsh, 50")
	r.bridge.Perform(row.ID, access.ActionExpand)
	r.frame()
	if !slices.Equal(exp.Peek(), []string{"g2"}) {
		t.Fatalf("Expand left the open rows %v, want g2", exp.Peek())
	}
	if !dgRowNode(t, r, "g2").div.acc.expanded {
		t.Error("g2 does not report expanded after Expand")
	}
	r.bridge.Perform(row.ID, access.ActionCollapse)
	r.frame()
	if len(exp.Peek()) != 0 {
		t.Errorf("Collapse left the open rows %v", exp.Peek())
	}
	if want := []string{"+g2", "-g2"}; !slices.Equal(ev, want) {
		t.Errorf("events %v, want %v", ev, want)
	}
	leaf := accesstest.Find(t, r.tree(), access.RoleListItem, "leaf, 10")
	if leaf.States.Has(access.StateExpandable) {
		t.Error("a row without children reports expandable")
	}
}

// TestDataGridTreeChevronStaysOnTheFirstShownColumn checks that the indent and the
// chevron of a tree are in the first column shown, when a column is moved to the front
// and when a column is pinned to the start.
func TestDataGridTreeChevronStaysOnTheFirstShownColumn(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		cols := dgProcColumns()
		order := NewSignal([]string{"Mem", "Name"})
		if pinned {
			cols[1].Pin = PinStart
			order = NewSignal([]string(nil))
		}
		r := newAccessRig(func(ctx *Context) Element {
			return DataGrid(dgProcTree(), func(x dgProc) string { return x.ID }, cols...).Label("Processes").
				Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(NewSignal([]string{"g1"})).
				ColumnOrder(order)
		})
		// The rows are g1, c1, c2, g2 and l1; c1 is the row of a level-2 process.
		first, second := dgCell(r, 1, 0), dgCell(r, 1, 1)
		if first == nil || second == nil {
			t.Fatalf("pinned %v: the cells of c1 are not built", pinned)
		}
		if n := len(partNodesUnder(first, "DataGrid.TreeIndent")); n != 1 {
			t.Errorf("pinned %v: the first shown column has %d indents for c1, want 1", pinned, n)
		}
		if n := len(partNodesUnder(first, "DataGrid.TreeChevron")); n != 1 {
			t.Errorf("pinned %v: the first shown column has %d chevrons for c1, want 1", pinned, n)
		}
		if n := len(partNodesUnder(second, "DataGrid.TreeChevron")) + len(partNodesUnder(second, "DataGrid.TreeIndent")); n != 0 {
			t.Errorf("pinned %v: the second shown column has %d tree parts", pinned, n)
		}
		if n := len(partNodesUnder(dgCell(r, 0, 0), "DataGrid.TreeChevron")); n != 1 {
			t.Errorf("pinned %v: g1 has %d chevrons in the first shown column, want 1", pinned, n)
		}
	}
}

// TestDataGridTreeLoadMoreCountsEveryRow checks that LoadMore asks once for the rows of a
// tree, and that opening a row, which shows rows the app has already given, does not ask
// again; and that rows added to the data ask again.
func TestDataGridTreeLoadMoreCountsEveryRow(t *testing.T) {
	groups := func(n int) []dgProc {
		out := make([]dgProc, n)
		for i := range out {
			id := "g" + strconv.Itoa(i)
			out[i] = dgProc{ID: id, Name: id, Kids: []dgProc{{ID: id + ".a", Name: "a"}, {ID: id + ".b", Name: "b"}}}
		}
		return out
	}
	calls := 0
	rows := NewSignal(groups(40))
	exp := NewSignal([]string(nil))
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(rows.Get(), func(x dgProc) string { return x.ID }, dgProcColumns()...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(exp).
			LoadMore(true, func() { calls++ })
	})
	dgSettle(r)
	body := dgBody(t, r)
	dgScrollTo(r, body, 1e6)
	if calls != 1 {
		t.Fatalf("LoadMore asked %d times at the end of the top rows, want once", calls)
	}
	exp.Set([]string{"g0"}) // two rows more are shown, but no row is given
	dgScrollTo(r, body, 1e6)
	if calls != 1 {
		t.Errorf("LoadMore asked %d times after a row was opened, want once", calls)
	}
	rows.Set(groups(60))
	dgSettle(r)
	dgScrollTo(r, body, 1e6)
	if calls != 2 {
		t.Errorf("LoadMore asked %d times after 20 rows were added, want twice", calls)
	}
}

// TestDataGridTreeAutoFitFitsTheIndent checks that a double-click on the edge of the tree
// column fits it to the deepest row shown, with the indent and the chevron of that row.
func TestDataGridTreeAutoFitFitsTheIndent(t *testing.T) {
	deep := []dgProc{{ID: "a", Name: "a", Kids: []dgProc{{ID: "b", Name: "b", Kids: []dgProc{
		{ID: "c", Name: "c", Kids: []dgProc{{ID: "d", Name: "deep leaf"}}}}}}}}
	cols := dgProcColumns()
	cols[0].Resizable = true
	exp := NewSignal([]string{"a", "b", "c"})
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(deep, func(x dgProc) string { return x.ID }, cols...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(exp)
	})
	r.frame()
	dgDoubleClick(r, dgResizeHandle(t, r, "Name"))
	// The deepest row is at level 4: three indents, and the chevron, then the text and its padding.
	want := textWidth(r.rt, "deep leaf") + 16 + 3*16 + 16
	if got := dgHeader(r, "Name").div.style.Basis.Value; got < want {
		t.Errorf("double-click on the edge gave width %v, want at least %v for the deepest row", got, want)
	}
}

// TestDataGridTreeEditSavesWhenACollapseHidesIt checks that an edit of a row that the app
// hides, by setting the Expanded keys, saves its pending value as a blur does, and that
// the collapse happens.
func TestDataGridTreeEditSavesWhenACollapseHidesIt(t *testing.T) {
	var saved []string
	cols := dgProcColumns()
	cols[0].Edit = func(x dgProc, v string) error {
		saved = append(saved, x.ID+"="+v)
		return nil
	}
	exp := NewSignal([]string{"g1"})
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgProcTree(), func(x dgProc) string { return x.ID }, cols...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(exp)
	})
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0) // to c2
	dgPress(r, input.KeyEnter)
	dgReplace(r, "Renamed")
	exp.Set(nil) // the app closes g1, which hides c2
	dgSettle(r)
	if want := []string{"c2=Renamed"}; !slices.Equal(saved, want) {
		t.Errorf("saved %v, want %v", saved, want)
	}
	if want := []string{"g1", "g2", "l1"}; !slices.Equal(dgRowKeys(r), want) {
		t.Errorf("shown rows %v after the collapse, want %v", dgRowKeys(r), want)
	}
	if n := len(partNodes(r.rt, "DataGrid.Editor.Text")); n != 0 {
		t.Errorf("%d editors are open after the save", n)
	}
}

// TestDataGridTreeEditErrorDoesNotBlockTheCollapse checks that a save that fails, for a
// row that a collapse hides, is announced, and that the collapse happens anyway.
func TestDataGridTreeEditErrorDoesNotBlockTheCollapse(t *testing.T) {
	cols := dgProcColumns()
	cols[0].Edit = func(x dgProc, v string) error { return errors.New("name taken") }
	exp := NewSignal([]string{"g1"})
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgProcTree(), func(x dgProc) string { return x.ID }, cols...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(exp)
	})
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0) // to c2
	dgPress(r, input.KeyEnter)
	dgReplace(r, "Taken")
	takeAnnouncements()
	var said []access.Announcement
	r.bridge.Listen(func(u access.Update) { said = append(said, u.Announcements...) })
	exp.Set(nil)
	dgSettle(r)
	if want := []string{"g1", "g2", "l1"}; !slices.Equal(dgRowKeys(r), want) {
		t.Errorf("shown rows %v after the collapse, want %v", dgRowKeys(r), want)
	}
	announced := false
	for _, a := range append(said, takeAnnouncements()...) {
		announced = announced || a.Text == "name taken"
	}
	if !announced {
		t.Error("the failed save was not announced")
	}
}

// TestDataGridTreeChevronClickSavesTheEdit checks that a click on the chevron of a row
// saves an edit that is open on a row below it, and that a click on the chevron that hides
// the edited row saves it too.
func TestDataGridTreeChevronClickSavesTheEdit(t *testing.T) {
	var saved []string
	cols := dgProcColumns()
	cols[0].Edit = func(x dgProc, v string) error {
		saved = append(saved, x.ID+"="+v)
		return nil
	}
	exp := NewSignal([]string{"g1"})
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgProcTree(), func(x dgProc) string { return x.ID }, cols...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(exp)
	})
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0) // to c1
	dgPress(r, input.KeyEnter)
	dgReplace(r, "One")
	dgClick(r, partNodesUnder(dgRowNode(t, r, "g2"), "DataGrid.TreeChevron")[0], 0)
	if want := []string{"c1=One"}; !slices.Equal(saved, want) {
		t.Fatalf("a click on the chevron of g2 saved %v, want %v", saved, want)
	}
	if !slices.Equal(exp.Peek(), []string{"g1", "g2"}) {
		t.Fatalf("the open rows are %v, want g1 and g2", exp.Peek())
	}

	// A click on the chevron of g1 hides c1, the edited row, and saves the edit.
	saved = nil
	exp.Set([]string{"g1"})
	dgSettle(r)
	focusNode(r, dgBody(t, r))
	dgDoubleClick(r, dgCell(r, 1, 0))
	dgReplace(r, "Two")
	dgClick(r, partNodesUnder(dgRowNode(t, r, "g1"), "DataGrid.TreeChevron")[0], 0)
	if want := []string{"c1=Two"}; !slices.Equal(saved, want) {
		t.Errorf("saved %v, want %v", saved, want)
	}
	if len(exp.Peek()) != 0 {
		t.Errorf("the open rows are %v after g1 closed, want none", exp.Peek())
	}
}

// dgTreeBenchOnce makes dgTreeBenchRows once: 1,000 groups of 10 processes each, which
// is 11,000 rows in all. The groups are closed, so 1,000 rows are shown.
var (
	dgTreeBenchOnce sync.Once
	dgTreeBenchRows []dgProc
)

// dgTreeBenchRig returns a rig with the tree of dgTreeBenchRows, with every group closed.
func dgTreeBenchRig(b *testing.B) *accessRig {
	b.Helper()
	dgTreeBenchOnce.Do(func() {
		for g := range 1000 {
			p := dgProc{ID: "g" + strconv.Itoa(g), Name: "group " + strconv.Itoa(g)}
			for c := range 10 {
				p.Kids = append(p.Kids, dgProc{ID: p.ID + "." + strconv.Itoa(c), Name: "process " + strconv.Itoa(c), Mem: c})
			}
			dgTreeBenchRows = append(dgTreeBenchRows, p)
		}
	})
	exp := NewSignal([]string(nil))
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgTreeBenchRows, func(x dgProc) string { return x.ID }, dgProcColumns()...).Label("Processes").
			Children(func(x dgProc) []dgProc { return x.Kids }).Expanded(exp)
	})
}

// BenchmarkDataGridTreeBuildFirst10k is the first frame of a tree of 11,000 rows, of
// which 1,000 are shown.
func BenchmarkDataGridTreeBuildFirst10k(b *testing.B) {
	dgTreeBenchRig(b) // the rows are made before the timer starts
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dgTreeBenchRig(b)
	}
}

// BenchmarkDataGridTreeKeyFrame10k is a frame that moves the focus one row down or up,
// as a key press does, in the tree of BenchmarkDataGridTreeBuildFirst10k. It alternates
// the two, so that the focus does not reach the end of the rows.
func BenchmarkDataGridTreeKeyFrame10k(b *testing.B) {
	r := dgTreeBenchRig(b)
	focusNode(r, partNodes(r.rt, "DataGrid.Grid")[0])
	r.frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			r.press(input.KeyDown, 0)
		} else {
			r.press(input.KeyUp, 0)
		}
	}
}

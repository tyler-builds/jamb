package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// dgItem is a row of the data grid tests. N has ties, so that a sort's stability
// shows: N is the index modulo 3.
type dgItem struct {
	ID, Name string
	N        int
}

func dgItems(n int) []dgItem {
	out := make([]dgItem, n)
	for i := range out {
		out[i] = dgItem{ID: "i" + strconv.Itoa(i), Name: fmt.Sprintf("Item %06d", i), N: i % 3}
	}
	return out
}

// dgColumns are the two columns of the tests: Name, which grows, and N, which has
// a fixed width. Both sort and both resize.
func dgColumns() []DataColumn[dgItem] {
	return []DataColumn[dgItem]{
		{Header: "Name", Text: func(x dgItem) string { return x.Name },
			Compare: func(a, b dgItem) int { return strings.Compare(a.Name, b.Name) }, Resizable: true, Grow: 1},
		{Header: "N", Text: func(x dgItem) string { return strconv.Itoa(x.N) },
			Compare: func(a, b dgItem) int { return a.N - b.N }, Width: 60, Resizable: true},
	}
}

// dgRig returns a rig with the grid of items, in the selection mode m, with the
// selection in sel, the sort in sort, and onAct as its activation.
func dgRig(items []dgItem, m SelectionMode, sel *Signal[[]string], sort *Signal[SortState],
	onAct func(dgItem), cols ...DataColumn[dgItem]) *accessRig {
	if len(cols) == 0 {
		cols = dgColumns()
	}
	return newAccessRig(func(ctx *Context) Element {
		g := DataGrid(items, func(x dgItem) string { return x.ID }, cols...).Label("Runs").Selection(m, sel).Sort(sort)
		if onAct != nil {
			g.OnActivate(onAct)
		}
		return g
	})
}

// dgBody is the scrolling body of the grid: the element that has focus.
func dgBody(t *testing.T, r *accessRig) *node {
	t.Helper()
	ns := partNodes(r.rt, "DataGrid.Grid")
	if len(ns) != 1 {
		t.Fatalf("want one grid body, have %d", len(ns))
	}
	return ns[0]
}

// dgCell returns the built cell at row and column (0-based), or nil.
func dgCell(r *accessRig, row, col int) *node {
	for _, n := range partNodes(r.rt, "DataGrid.Cell") {
		if a := n.div.acc; a != nil && a.gridRow == row+1 && a.gridCol == col+1 {
			return n
		}
	}
	return nil
}

// dgHeader returns the header cell whose name starts with header, or nil.
func dgHeader(r *accessRig, header string) *node {
	for _, n := range partNodes(r.rt, "DataGrid.HeaderCell") {
		if a := n.div.acc; a != nil && (a.name == header || strings.HasPrefix(a.name, header+", ")) {
			return n
		}
	}
	return nil
}

// dgRowKeys returns the keys of the built rows, top to bottom.
func dgRowKeys(r *accessRig) []string {
	var keys []string
	ns := partNodes(r.rt, "DataGrid.Row")
	// best returns the built row lowest after the row at y prev, on the screen.
	best := func(prev float32) *node {
		var b *node
		for _, n := range ns {
			y := r.rt.nodeRect(n).Y
			if y > prev && (b == nil || y < r.rt.nodeRect(b).Y) {
				b = n
			}
		}
		return b
	}
	y := float32(-1)
	for {
		b := best(y)
		if b == nil {
			return keys
		}
		keys = append(keys, b.div.key)
		y = r.rt.nodeRect(b).Y
	}
}

// dgFocusCell returns the cell the access tree reports as focused: the one that
// is highlighted.
func dgFocusCell(r *accessRig) *node {
	for _, n := range partNodes(r.rt, "DataGrid.Cell") {
		if n.div.acc != nil && n.div.acc.highlighted {
			return n
		}
	}
	return nil
}

// dgDoubleClick presses the left button twice on n, as a double-click does.
func dgDoubleClick(r *accessRig, n *node) {
	rc := r.rt.nodeRect(n)
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 2, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
}

// dgClick presses the left button on n with the modifiers mods.
func dgClick(r *accessRig, n *node, mods input.Modifiers) {
	rc := r.rt.nodeRect(n)
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, mods)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, mods)
	r.rt.PointerUp(x, y, input.ButtonLeft, mods)
	r.frame()
}

func TestDataGridBuildsOnlyRowsInView(t *testing.T) {
	r := dgRig(dgItems(100000), SelectionMultiple, NewSignal([]string(nil)), NewSignal(SortState{}), nil)
	rows := partNodes(r.rt, "DataGrid.Row")
	if len(rows) == 0 || len(rows) > 40 {
		t.Fatalf("built %d rows of 100,000; want a viewport's worth", len(rows))
	}
	if n := r.tree().Find(access.RoleDataGrid, "Runs"); n == nil {
		t.Fatal("no data grid in the access tree")
	}
}

func TestDataGridSortIsStableAndCycles(t *testing.T) {
	items := dgItems(12)
	sort := NewSignal(SortState{})
	r := dgRig(items, SelectionNone, nil, sort, nil)
	if got := dgRowKeys(r)[0]; got != "i0" {
		t.Fatalf("unsorted first row %q, want i0", got)
	}
	sort.Set(SortState{Key: "N"})
	r.frame()
	keys := dgRowKeys(r)
	if want := []string{"i0", "i3", "i6", "i9"}; strings.Join(keys[:4], ",") != strings.Join(want, ",") {
		t.Errorf("ascending by N starts %v, want %v (ties keep their order)", keys[:4], want)
	}
	sort.Set(SortState{Key: "N", Desc: true})
	r.frame()
	if got := dgRowKeys(r); got[0] != "i2" || got[1] != "i5" {
		t.Errorf("descending by N starts %v, want i2, i5 (ties keep their order)", got[:2])
	}
	sort.Set(SortState{})
	r.frame()
	if got := dgRowKeys(r)[0]; got != "i0" {
		t.Errorf("unsorted again starts at %q, want i0", got)
	}
}

func TestDataGridHeaderClickCyclesTheSort(t *testing.T) {
	sort := NewSignal(SortState{})
	r := dgRig(dgItems(12), SelectionNone, nil, sort, nil)
	for _, want := range []SortState{{Key: "N"}, {Key: "N", Desc: true}, {}} {
		dgClick(r, dgHeader(r, "N"), 0)
		if got := sort.Peek(); got != want {
			t.Errorf("after a header click the sort is %+v, want %+v", got, want)
		}
	}
	// Name is sortable too; a column with no Compare is not.
	cols := []DataColumn[dgItem]{{Header: "Name", Text: func(x dgItem) string { return x.Name }}}
	sort = NewSignal(SortState{})
	r = dgRig(dgItems(3), SelectionNone, nil, sort, nil, cols...)
	dgClick(r, dgHeader(r, "Name"), 0)
	if got := sort.Peek(); got != (SortState{}) {
		t.Errorf("a column without Compare sorted: %+v", got)
	}
}

func TestDataGridSortKeepsFocusAndSelection(t *testing.T) {
	items := dgItems(30)
	sel := NewSignal([]string(nil))
	sort := NewSignal(SortState{})
	r := dgRig(items, SelectionMultiple, sel, sort, nil)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0)
	focused := dgFocusCell(r)
	if focused == nil {
		t.Fatal("no focused cell after Down")
	}
	key := focused.parent.div.key
	if got := sel.Peek(); len(got) != 1 || got[0] != key {
		t.Fatalf("selection %v after moving to %q, want that row only", got, key)
	}
	sort.Set(SortState{Key: "N", Desc: true})
	r.frame()
	again := dgFocusCell(r)
	if again == nil || again.parent.div.key != key {
		t.Errorf("focus moved off row %q after a sort", key)
	}
	if got := sel.Peek(); len(got) != 1 || got[0] != key {
		t.Errorf("selection %v after a sort, want %q", got, key)
	}
}

func TestDataGridKeyboardMovesTheFocusedCell(t *testing.T) {
	items := dgItems(60)
	sel := NewSignal([]string(nil))
	r := dgRig(items, SelectionMultiple, sel, NewSignal(SortState{}), nil)
	focusNode(r, dgBody(t, r))
	cell := func() (row, col int) {
		c := dgFocusCell(r)
		if c == nil {
			t.Fatal("no focused cell")
		}
		return c.div.acc.gridRow - 1, c.div.acc.gridCol - 1
	}
	if row, col := cell(); row != 0 || col != 0 {
		t.Fatalf("first focus at row %d col %d, want 0, 0", row, col)
	}
	r.press(input.KeyRight, 0)
	if _, col := cell(); col != 1 {
		t.Errorf("Right: column %d, want 1", col)
	}
	r.press(input.KeyRight, 0)
	if _, col := cell(); col != 1 {
		t.Errorf("Right at the last column: column %d, want 1", col)
	}
	r.press(input.KeyHome, 0)
	if _, col := cell(); col != 0 {
		t.Errorf("Home: column %d, want 0", col)
	}
	r.press(input.KeyEnd, 0)
	if _, col := cell(); col != 1 {
		t.Errorf("End: column %d, want 1", col)
	}
	r.press(input.KeyEnd, input.ModCtrl)
	if row, col := cell(); row != 59 || col != 1 {
		t.Errorf("Ctrl+End at row %d col %d, want 59, 1", row, col)
	}
	r.press(input.KeyHome, input.ModCtrl)
	if row, col := cell(); row != 0 || col != 0 {
		t.Errorf("Ctrl+Home at row %d col %d, want 0, 0", row, col)
	}
	r.press(input.KeyPageDown, 0)
	if row, _ := cell(); row < 5 {
		t.Errorf("PageDown reached row %d, want a viewport down", row)
	}
	before, _ := cell()
	r.press(input.KeyUp, 0)
	if row, _ := cell(); row != before-1 {
		t.Errorf("Up: row %d, want %d", row, before-1)
	}
	r.press(input.KeyHome, input.ModCtrl)
	r.press(input.KeyUp, 0) // from the first row to the header
	if focused := dgFocusCell(r); focused != nil {
		t.Errorf("Up from the first row still focuses a cell: %+v", focused.div.acc)
	}
	if h := dgHeader(r, "Name"); h == nil || !h.div.acc.highlighted {
		t.Error("Up from the first row does not focus the header")
	}
	r.press(input.KeyDown, 0)
	if row, _ := cell(); row != 0 {
		t.Errorf("Down from the header: row %d, want 0", row)
	}
}

func TestDataGridSpaceAndEnterOnHeaderSort(t *testing.T) {
	sort := NewSignal(SortState{})
	r := dgRig(dgItems(12), SelectionMultiple, NewSignal([]string(nil)), sort, nil)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyUp, 0) // onto the header, at column 0 (Name)
	r.press(input.KeyRight, 0)
	r.press(input.KeyEnter, 0)
	if got := sort.Peek(); got != (SortState{Key: "N"}) {
		t.Errorf("Enter on the N header: sort %+v, want N ascending", got)
	}
	r.press(input.KeySpace, 0)
	if got := sort.Peek(); got != (SortState{Key: "N", Desc: true}) {
		t.Errorf("Space on the N header: sort %+v, want N descending", got)
	}
}

func TestDataGridSelectionModes(t *testing.T) {
	items := dgItems(20)
	t.Run("none", func(t *testing.T) {
		sel := NewSignal([]string(nil))
		r := dgRig(items, SelectionNone, sel, NewSignal(SortState{}), nil)
		dgClick(r, dgCell(r, 2, 0), 0)
		if got := sel.Peek(); len(got) != 0 {
			t.Errorf("a click selected %v with SelectionNone", got)
		}
	})
	t.Run("single", func(t *testing.T) {
		sel := NewSignal([]string(nil))
		r := dgRig(items, SelectionSingle, sel, NewSignal(SortState{}), nil)
		dgClick(r, dgCell(r, 1, 0), 0)
		dgClick(r, dgCell(r, 2, 0), input.ModCtrl)
		if got := sel.Peek(); len(got) != 1 || got[0] != "i2" {
			t.Errorf("single selection %v, want [i2] (Ctrl does not add in single mode)", got)
		}
	})
	t.Run("multiple", func(t *testing.T) {
		sel := NewSignal([]string(nil))
		r := dgRig(items, SelectionMultiple, sel, NewSignal(SortState{}), nil)
		dgClick(r, dgCell(r, 1, 0), 0)
		dgClick(r, dgCell(r, 3, 0), input.ModCtrl)
		if got := sel.Peek(); strings.Join(got, ",") != "i1,i3" {
			t.Errorf("Ctrl+click: %v, want [i1 i3]", got)
		}
		dgClick(r, dgCell(r, 5, 0), input.ModShift)
		if got := sel.Peek(); strings.Join(got, ",") != "i3,i4,i5" {
			t.Errorf("Shift+click from the anchor: %v, want [i3 i4 i5]", got)
		}
		dgClick(r, dgCell(r, 3, 0), input.ModCtrl)
		if got := sel.Peek(); strings.Join(got, ",") != "i4,i5" {
			t.Errorf("Ctrl+click on a selected row: %v, want it removed", got)
		}
		r.rt.KeyDown(input.KeyA, input.ModCtrl, false)
		r.rt.KeyUp(input.KeyA, input.ModCtrl)
		r.frame()
		if got := sel.Peek(); len(got) != len(items) {
			t.Errorf("Ctrl+A selected %d rows, want %d", len(got), len(items))
		}
	})
}

func TestDataGridSelectionFollowsKeysAcrossSort(t *testing.T) {
	sel := NewSignal([]string{"i7"})
	sort := NewSignal(SortState{})
	r := dgRig(dgItems(20), SelectionMultiple, sel, sort, nil)
	if got := dgCell(r, 0, 0); got == nil {
		t.Fatal("no first cell")
	}
	sort.Set(SortState{Key: "N", Desc: true})
	r.frame()
	var found bool
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.div.key == "i7" && n.div.acc.selected {
			found = true
		}
	}
	if !found {
		t.Error("the selected row i7 is not drawn as selected after a sort")
	}
}

func TestDataGridActivate(t *testing.T) {
	var got []string
	r := dgRig(dgItems(10), SelectionMultiple, NewSignal([]string(nil)), NewSignal(SortState{}),
		func(x dgItem) { got = append(got, x.ID) })
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnter, 0)
	if len(got) != 1 || got[0] != "i1" {
		t.Errorf("Enter activated %v, want i1", got)
	}
	dgDoubleClick(r, dgCell(r, 4, 0))
	if len(got) != 2 || got[1] != "i4" {
		t.Errorf("double-click activated %v, want i4 after i1", got)
	}
}

func TestDataGridResizeKeyboardAndMouse(t *testing.T) {
	r := dgRig(dgItems(10), SelectionNone, nil, NewSignal(SortState{}), nil)
	width := func() float32 { return dgHeader(r, "N").div.style.Basis.Value }
	focusNode(r, dgBody(t, r))
	r.press(input.KeyUp, 0)
	r.press(input.KeyRight, 0) // onto the N header
	w0 := width()
	r.press(input.KeyRight, input.ModCtrl)
	if got := width(); got != w0+8 {
		t.Errorf("Ctrl+Right on the header: width %v, want %v", got, w0+8)
	}
	r.press(input.KeyLeft, input.ModCtrl)
	r.press(input.KeyLeft, input.ModCtrl)
	if got := width(); got != w0-8 {
		t.Errorf("Ctrl+Left twice: width %v, want %v", got, w0-8)
	}
	// Mouse: drag the edge of the Name column, which is not the N column.
	var handle *node
	for _, n := range partNodes(r.rt, "DataGrid.ResizeHandle") {
		if n.parent != nil && n.parent.div.acc != nil && n.parent.div.acc.name == "Name" {
			handle = n
		}
	}
	if handle == nil {
		t.Fatal("no resize handle on the Name header")
	}
	rc := r.rt.nodeRect(handle)
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(x+30, y, 0)
	r.rt.PointerUp(x+30, y, input.ButtonLeft, 0)
	r.frame()
	if got := dgHeader(r, "Name").div.style.Basis.Value; got != 150 {
		t.Errorf("dragging the Name edge by 30 px gave width %v, want 150", got)
	}
}

func TestDataGridHeaderShowsTheSortState(t *testing.T) {
	sort := NewSignal(SortState{Key: "N", Desc: true})
	r := dgRig(dgItems(10), SelectionNone, nil, sort, nil)
	h := r.stateOf(t, access.RoleHeaderItem, "N, sorted descending")
	if h.ItemStatus != "sorted descending" {
		t.Errorf("header ItemStatus %q, want sorted descending", h.ItemStatus)
	}
	if !h.Has(access.ActionClick) {
		t.Error("a sortable header has no Invoke action")
	}
	accesstest.Find(t, r.tree(), access.RoleHeaderItem, "Name")
}

func TestDataGridAccessibilityShape(t *testing.T) {
	r := dgRig(dgItems(100000), SelectionMultiple, NewSignal([]string(nil)), NewSignal(SortState{}), nil)
	g := r.stateOf(t, access.RoleDataGrid, "Runs")
	if g.GridRows != 100000 || g.GridColumns != 2 {
		t.Errorf("grid shape %dx%d, want 100000 rows and 2 columns", g.GridRows, g.GridColumns)
	}
	if !g.States.Has(access.StateMultiSelectable) || !g.States.Has(access.StateSelectionContainer) {
		t.Errorf("grid states %q, want multi-selectable selection container", g.States)
	}
	if !g.States.Has(access.StateFocusable) {
		t.Error("the grid is not focusable")
	}
	item := r.stateOf(t, access.RoleDataItem, "Item 000000")
	if item.GridRow != 1 || item.GridColumn != 1 {
		t.Errorf("first cell at %d,%d, want 1,1", item.GridRow, item.GridColumn)
	}
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

func TestDataGridSelectionItemAction(t *testing.T) {
	sel := NewSignal([]string(nil))
	r := dgRig(dgItems(10), SelectionMultiple, sel, NewSignal(SortState{}), nil)
	row := accesstest.Find(t, r.tree(), access.RoleListItem, "Item 000002, 2")
	r.rt.runAccess(row.ID, access.Request{Action: access.ActionSelect})
	if got := sel.Peek(); len(got) != 1 || got[0] != "i2" {
		t.Errorf("ActionSelect on row i2 gave selection %v", got)
	}
}

func TestDataGridHorizontalScrollMovesTheHeader(t *testing.T) {
	cols := []DataColumn[dgItem]{
		{Header: "A", Text: func(x dgItem) string { return x.Name }, Width: 300},
		{Header: "B", Text: func(x dgItem) string { return x.Name }, Width: 300},
		{Header: "C", Text: func(x dgItem) string { return x.Name }, Width: 300},
	}
	r := dgRig(dgItems(20), SelectionNone, nil, NewSignal(SortState{}), nil, cols...)
	focusNode(r, dgBody(t, r))
	x0 := r.rt.nodeRect(dgHeader(r, "C")).X
	r.press(input.KeyRight, 0)
	r.press(input.KeyRight, 0)
	for range 8 {
		r.frame()
	}
	body := dgBody(t, r)
	if body.st.targetX <= 0 {
		t.Fatalf("moving to column C did not scroll the grid: targetX %v", body.st.targetX)
	}
	if got := r.rt.nodeRect(dgHeader(r, "C")).X; got >= x0 {
		t.Errorf("the header did not move with the columns: C at %v, was %v", got, x0)
	}
}

func TestDataGridHeaderStaysAtTheTop(t *testing.T) {
	r := dgRig(dgItems(500), SelectionNone, nil, NewSignal(SortState{}), nil)
	top := r.rt.nodeRect(dgHeader(r, "Name")).Y
	focusNode(r, dgBody(t, r))
	for range 3 {
		r.press(input.KeyPageDown, 0)
	}
	for range 8 {
		r.frame()
	}
	if got := r.rt.nodeRect(dgHeader(r, "Name")).Y; got != top {
		t.Errorf("header moved from %v to %v while the rows scrolled", top, got)
	}
}

func TestDataGridEmptyAndLoading(t *testing.T) {
	var none []dgItem
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(none, func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs")
	})
	if n := r.tree().Find(access.RoleStaticText, "No rows"); n == nil {
		t.Error("an empty grid does not say that it has no rows")
	}
	loading := NewSignal(true)
	r = newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(5), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").Loading(loading.Get())
	})
	if n := r.tree().Find(access.RoleProgressBar, "Loading"); n == nil {
		t.Error("a loading grid does not show a progress ring labelled Loading")
	}
}

func TestDataGridCompactRowsAreShorter(t *testing.T) {
	heights := func(d DataGridDensity) float32 {
		r := newAccessRig(func(ctx *Context) Element {
			return DataGrid(dgItems(10), func(x dgItem) string { return x.ID }, dgColumns()...).Density(d).Label("Runs")
		})
		rows := partNodes(r.rt, "DataGrid.Row")
		if len(rows) == 0 {
			t.Fatal("no rows")
		}
		return r.rt.nodeRect(rows[0]).H
	}
	if c, m := heights(DataGridCompact), heights(DataGridComfortable); c >= m {
		t.Errorf("compact rows are %v high, comfortable %v", c, m)
	}
}

func TestDataGridStripedRows(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(6), func(x dgItem) string { return x.ID }, dgColumns()...).Striped().Label("Runs")
	})
	var alt int
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.style.Background == DarkTheme().Sidebar {
			alt++
		}
	}
	if alt == 0 {
		t.Error("no alternate row is drawn with RowAlternate when the grid is striped")
	}
}

// dgBenchItems are the 100,000 rows of the benchmarks, made once.
var (
	dgBenchOnce sync.Once
	dgBenchRows []dgItem
)

// dgBenchRig is a grid of 100,000 rows, sorted by a signal the benchmarks set.
func dgBenchRig(b *testing.B) (*accessRig, *Signal[SortState]) {
	b.Helper()
	dgBenchOnce.Do(func() { dgBenchRows = dgItems(100000) })
	sort := NewSignal(SortState{})
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgBenchRows, func(x dgItem) string { return x.ID }, dgColumns()...).Sort(sort).Label("Runs")
	})
	return r, sort
}

// BenchmarkDataGridBuildFirst100k is the first frame of a grid of 100,000 rows: the
// rows in view are built, and the keys of all the rows are read.
func BenchmarkDataGridBuildFirst100k(b *testing.B) {
	dgBenchRig(b) // the rows are made before the timer starts
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dgBenchRig(b)
	}
}

// BenchmarkDataGridKeyFrame100k is a frame that moves the focus one row down, as a
// key press does: the grid is rebuilt for the rows in view.
func BenchmarkDataGridKeyFrame100k(b *testing.B) {
	r, _ := dgBenchRig(b)
	focusNode(r, partNodes(r.rt, "DataGrid.Grid")[0])
	r.frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.press(input.KeyDown, 0)
	}
}

// BenchmarkDataGridSort100k is a sort of 100,000 rows and the frame that shows it.
func BenchmarkDataGridSort100k(b *testing.B) {
	r, sort := dgBenchRig(b)
	r.frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			sort.Set(SortState{Key: "N", Desc: i%4 == 0})
		} else {
			sort.Set(SortState{})
		}
		r.frame()
	}
}

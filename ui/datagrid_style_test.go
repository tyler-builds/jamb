package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// partDataGrid returns a striped grid of n items with the instance style st, the
// selection in sel and the sort in sort.
func partDataGrid(n int, st DataGridStyle, sel *Signal[[]string], sort *Signal[SortState]) func(*Context) Element {
	return func(ctx *Context) Element {
		return DataGrid(dgItems(n), func(x dgItem) string { return x.ID }, dgColumns()...).
			Label("Runs").Styles(st).Selection(SelectionMultiple, sel).Sort(sort).Striped()
	}
}

// testDataGridParts checks that each part of the grid reaches a node that draws
// it, and it records each part in applied, for the drift test.
func testDataGridParts(t *testing.T, applied map[string]bool) {
	testDataGridEditParts(t, applied)
	type dgCase struct {
		part  string
		set   func(*DataGridStyle, Style)
		n     int
		sel   []string
		sort  SortState
		act   func(*testing.T, *accessRig)
		drawn string
	}
	hover := func(name string) func(*testing.T, *accessRig) {
		return func(t *testing.T, r *accessRig) { hoverNode(r, dgHeader(r, name)) }
	}
	cases := []dgCase{
		{part: "DataGrid.Grid", set: func(s *DataGridStyle, m Style) { s.Grid = m }, n: 6, drawn: "DataGrid.Grid"},
		{part: "DataGrid.GridFocusVisible", set: func(s *DataGridStyle, m Style) { s.GridFocusVisible = m }, n: 6,
			act:   func(t *testing.T, r *accessRig) { focusNode(r, dgBody(t, r)) },
			drawn: "DataGrid.Grid"},
		{part: "DataGrid.Header", set: func(s *DataGridStyle, m Style) { s.Header = m }, n: 6, drawn: "DataGrid.Header"},
		{part: "DataGrid.HeaderCell", set: func(s *DataGridStyle, m Style) { s.HeaderCell = m }, n: 6, drawn: "DataGrid.HeaderCell"},
		{part: "DataGrid.HeaderCellHover", set: func(s *DataGridStyle, m Style) { s.HeaderCellHover = m }, n: 6,
			act: hover("Name"), drawn: "DataGrid.HeaderCell"},
		{part: "DataGrid.HeaderCellSorted", set: func(s *DataGridStyle, m Style) { s.HeaderCellSorted = m }, n: 6,
			sort: SortState{Key: "N"}, drawn: "DataGrid.HeaderCell"},
		{part: "DataGrid.SortIndicator", set: func(s *DataGridStyle, m Style) { s.SortIndicator = m }, n: 6,
			sort: SortState{Key: "N"}, drawn: "DataGrid.SortIndicator"},
		{part: "DataGrid.ResizeHandle", set: func(s *DataGridStyle, m Style) { s.ResizeHandle = m }, n: 6, drawn: "DataGrid.ResizeHandle"},
		{part: "DataGrid.Row", set: func(s *DataGridStyle, m Style) { s.Row = m }, n: 6, drawn: "DataGrid.Row"},
		{part: "DataGrid.RowHover", set: func(s *DataGridStyle, m Style) { s.RowHover = m }, n: 6,
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNodes(r.rt, "DataGrid.Row")[0]) }, drawn: "DataGrid.Row"},
		{part: "DataGrid.RowSelected", set: func(s *DataGridStyle, m Style) { s.RowSelected = m }, n: 6,
			sel: []string{"i1"}, drawn: "DataGrid.Row"},
		{part: "DataGrid.RowAlternate", set: func(s *DataGridStyle, m Style) { s.RowAlternate = m }, n: 6, drawn: "DataGrid.Row"},
		{part: "DataGrid.Cell", set: func(s *DataGridStyle, m Style) { s.Cell = m }, n: 6, drawn: "DataGrid.Cell"},
		{part: "DataGrid.CellFocused", set: func(s *DataGridStyle, m Style) { s.CellFocused = m }, n: 6,
			act:   func(t *testing.T, r *accessRig) { focusNode(r, dgBody(t, r)) },
			drawn: "DataGrid.Cell"},
		{part: "DataGrid.Empty", set: func(s *DataGridStyle, m Style) { s.Empty = m }, n: 0, drawn: "DataGrid.Empty"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st DataGridStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			sel := NewSignal(c.sel)
			sort := NewSignal(c.sort)
			r := newAccessRig(partDataGrid(c.n, st, sel, sort))
			if c.act != nil {
				c.act(t, r)
			}
			if len(partNodes(r.rt, c.drawn)) == 0 {
				t.Fatalf("%s: no node draws %s", c.part, c.drawn)
			}
			if !anyMarked(r.rt, c.drawn, false) {
				t.Errorf("%s: the style does not reach a node that draws %s", c.part, c.drawn)
			}
		})
	}
}

// dgRowNode returns the built row with the data key key.
func dgRowNode(t *testing.T, r *accessRig, key string) *node {
	t.Helper()
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.div.key == key {
			return n
		}
	}
	t.Fatalf("no built row %q", key)
	return nil
}

// TestDataGridPrecedence checks that an instance style wins over the theme's
// DataGrid, which wins over the defaults, and that Clear removes a default.
func TestDataGridPrecedence(t *testing.T) {
	theme := *DarkTheme()
	theme.DataGrid = DataGridStyle{Row: Style{Background: Hex("#101010")}}
	sel := NewSignal([]string(nil))
	sort := NewSignal(SortState{})
	r := newThemedRig(&theme, partDataGrid(6, DataGridStyle{}, sel, sort))
	if got := dgRowNode(t, r, "i0").style.Background; got != Hex("#101010") {
		t.Errorf("theme row background %v, want the theme's", got)
	}
	r = newThemedRig(&theme, partDataGrid(6, DataGridStyle{Row: Style{Background: markBG}}, sel, sort))
	if got := dgRowNode(t, r, "i0").style.Background; got != markBG {
		t.Errorf("instance row background %v, want the instance's to win over the theme", got)
	}
	// The striped rows of the default look have a background; Clear removes it.
	r = newAccessRig(partDataGrid(6, DataGridStyle{RowAlternate: Style{Clear: ClearBackground}}, sel, sort))
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.style.Background == DarkTheme().Sidebar {
			t.Fatalf("ClearBackground did not remove the striped background")
		}
	}
}

// TestDataGridHighContrast checks that the selected row is the system Highlight
// with HighlightText on it, that an app's colors do not reach the grid, and that
// the focus ring stays.
func TestDataGridHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.DataGrid = DataGridStyle{Row: Style{Background: markBG, TextColor: markFG}}
	sel := NewSignal([]string{"i1"})
	sort := NewSignal(SortState{Key: "N"})
	r := newThemedRig(&theme, partDataGrid(6, DataGridStyle{Row: Style{Background: markBG}, RowSelected: Style{TextColor: markFG}}, sel, sort))
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	var selected *node
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.div.key == "i1" {
			selected = n
		}
	}
	if selected == nil {
		t.Fatal("the selected row is not built")
	}
	if selected.style.Background != th.Accent || selected.style.TextColor != th.AccentText {
		t.Errorf("high contrast: the selected row is %v on %v, want the Highlight %v with HighlightText %v",
			selected.style.Background, selected.style.TextColor, th.Accent, th.AccentText)
	}
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.style.Background == markBG || n.style.TextColor == markFG {
			t.Errorf("high contrast: an app color reached a row")
		}
	}
	// The sorted column's arrow is drawn in the text color, so it is visible.
	for _, n := range partNodes(r.rt, "DataGrid.SortIndicator") {
		if n.style.TextColor == markFG {
			t.Errorf("high contrast: the sort indicator takes an app color")
		}
	}
	focusNode(r, dgBody(t, r))
	if len(dgBody(t, r).style.Shadows) == 0 {
		t.Errorf("high contrast: the focused grid has no focus ring")
	}
}

// TestDataGridSelectedRowIsNotBold checks that the text of a selected row keeps its
// weight: the selection is the row's fill, and the text is not made heavier. The
// sorted header keeps its weight too, and shows the sort by its arrow.
func TestDataGridSelectedRowIsNotBold(t *testing.T) {
	sel := NewSignal([]string{"i1"})
	r := newAccessRig(partDataGrid(6, DataGridStyle{RowSelected: Style{Clear: ClearBackground}}, sel, NewSignal(SortState{})))
	found := false
	for _, n := range partNodes(r.rt, "DataGrid.Row") {
		if n.div.key != "i1" {
			continue
		}
		found = true
		for _, c := range n.children {
			if len(c.children) == 0 {
				continue
			}
			if txt := c.children[0]; txt.textEl != nil && txt.style.FontWeight != 0 {
				t.Errorf("selected row text has weight %v, want the default", txt.style.FontWeight)
			}
		}
	}
	if !found {
		t.Fatal("the selected row is not built")
	}
	if w := dataGridDefaults(DarkTheme(), DataGridComfortable, 1).HeaderCellSorted.FontWeight; w != 0 {
		t.Errorf("the sorted header has weight %v, want the default", w)
	}
}

// TestDataGridTextScaleGrowsRows checks that the rows grow with the Text size, so
// that their text is not clipped, and that the rows are taller than their text.
func TestDataGridTextScaleGrowsRows(t *testing.T) {
	height := func(scale int) float32 {
		r := newAccessRig(partDataGrid(6, DataGridStyle{}, NewSignal([]string(nil)), NewSignal(SortState{})))
		r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
		r.frame()
		rows := partNodes(r.rt, "DataGrid.Row")
		if len(rows) == 0 {
			t.Fatal("no rows")
		}
		row := rows[0]
		for _, c := range row.children {
			if c.lay.H > row.lay.H {
				t.Fatalf("at %d%% the text of a cell (%v) is taller than its row (%v)", scale, c.lay.H, row.lay.H)
			}
		}
		return row.lay.H
	}
	if small, big := height(100), height(225); big <= small {
		t.Errorf("rows at 225%% are %v high, at 100%% %v; they must grow", big, small)
	}
}

// TestDataGridRequestsNoFramesUnderReducedMotion checks that a row's hover is
// instant under reduced motion, as the parts' transitions are.
func TestDataGridRequestsNoFramesUnderReducedMotion(t *testing.T) {
	for _, reduced := range []bool{true, false} {
		st := DataGridStyle{Row: Style{Transition: 200 * time.Millisecond}, RowHover: Style{Background: markBG}}
		r := newAccessRig(partDataGrid(6, st, NewSignal([]string(nil)), NewSignal(SortState{})))
		r.rt.SetSystemSettings(SystemSettings{ReducedMotion: reduced})
		r.frame()
		hoverNode(r, dgRowNode(t, r, "i0"))
		eased := r.rt.NeedsFrame()
		if reduced && eased {
			t.Fatalf("under reduced motion the row hover asks for frames")
		}
		if !reduced && !eased {
			t.Fatalf("without reduced motion the row hover asks for no frames; it must ease")
		}
	}
}

// TestDataGridKeysMoveSelectionInEachMode checks that Shift+Down extends a
// multiple selection, and that a single selection follows the arrows.
func TestDataGridKeysMoveSelectionInEachMode(t *testing.T) {
	sel := NewSignal([]string(nil))
	r := dgRig(dgItems(10), SelectionSingle, sel, NewSignal(SortState{}), nil)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0)
	if got := sel.Peek(); len(got) != 1 || got[0] != "i2" {
		t.Errorf("single selection after two Downs: %v, want [i2]", got)
	}
	sel2 := NewSignal([]string(nil))
	r = dgRig(dgItems(10), SelectionMultiple, sel2, NewSignal(SortState{}), nil)
	focusNode(r, dgBody(t, r))
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, input.ModShift)
	r.press(input.KeyDown, input.ModShift)
	if got := sel2.Peek(); len(got) != 3 {
		t.Errorf("Shift+Down twice from row 1 selected %v, want three rows", got)
	}
}

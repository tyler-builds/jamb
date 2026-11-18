package ui

import (
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// A table made of plain elements reports its shape, the column of each header and the
// row and column of each cell.
func TestAccessGridAndCellSetTheShapeAndPositions(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Role(access.RoleDataGrid).Label("Releases").AccessGrid(2, 2).Grid(Auto, Auto).Children(
			Div().Role(access.RoleHeaderItem).AccessCell(0, 1).Child(Text("Name")),
			Div().Role(access.RoleHeaderItem).AccessCell(0, 2).Child(Text("Year")),
			Div().Role(access.RoleDataItem).AccessCell(1, 1).Child(Text("jamb")),
			Div().Role(access.RoleDataItem).AccessCell(1, 2).Child(Text("2026")),
			Div().Role(access.RoleDataItem).AccessCell(2, 1).Child(Text("notes")),
			Div().Role(access.RoleDataItem).AccessCell(2, 2).Child(Text("2025")),
		)
	})
	tr := r.tree()
	grid := accesstest.Find(t, tr, access.RoleDataGrid, "Releases")
	if grid.GridRows != 2 || grid.GridColumns != 2 {
		t.Fatalf("grid shape %d by %d, want 2 by 2", grid.GridRows, grid.GridColumns)
	}
	year := accesstest.Find(t, tr, access.RoleHeaderItem, "Year")
	if year.GridRow != 0 || year.GridColumn != 2 {
		t.Fatalf("header at row %d, column %d, want row 0, column 2", year.GridRow, year.GridColumn)
	}
	notes := accesstest.Find(t, tr, access.RoleDataItem, "notes")
	if notes.GridRow != 2 || notes.GridColumn != 1 {
		t.Fatalf("cell at row %d, column %d, want row 2, column 1", notes.GridRow, notes.GridColumn)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// AccessGrid and AccessCell on roles they do not apply to are not reported, and warn;
// so does a data item in row 0.
func TestAccessGridAndCellOnOtherRolesWarn(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Div().Role(access.RoleList).Label("Shapes").AccessGrid(3, 2),
			Div().Role(access.RoleListItem).AccessCell(1, 1).Child(Text("Square")),
			Div().Role(access.RoleDataGrid).Label("Grid").AccessGrid(1, 1).
				Child(Div().Role(access.RoleDataItem).AccessCell(0, 1).Child(Text("Header?"))),
		)
	})
	tr := r.tree()
	if l := accesstest.Find(t, tr, access.RoleList, "Shapes"); l.GridRows != 0 || l.GridColumns != 0 {
		t.Errorf("a list reports a grid shape %d by %d", l.GridRows, l.GridColumns)
	}
	if li := accesstest.Find(t, tr, access.RoleListItem, "Square"); li.GridRow != 0 || li.GridColumn != 0 {
		t.Errorf("a list item reports a cell position %d, %d", li.GridRow, li.GridColumn)
	}
	if got := warningsContaining(r.warnings, "AccessGrid applies to a data grid or a calendar, not a list"); len(got) != 1 {
		t.Errorf("warnings = %v, want one for AccessGrid on a list", r.warnings)
	}
	if got := warningsContaining(r.warnings, "AccessCell applies to a data item or a header item, not a list item"); len(got) != 1 {
		t.Errorf("warnings = %v, want one for AccessCell on a list item", r.warnings)
	}
	if got := warningsContaining(r.warnings, "data item has row 0"); len(got) != 1 {
		t.Errorf("warnings = %v, want one for the data item in row 0", r.warnings)
	}
}

// AccessPosition and AccessLevel set the position in the set and the depth of an item.
func TestAccessPositionAndLevelOfAnItem(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Role(access.RoleList).Label("Steps").Children(
			Div().Role(access.RoleListItem).AccessPosition(1, 2).AccessLevel(1).Child(Text("Install")),
			Div().Role(access.RoleListItem).AccessPosition(2, 2).AccessLevel(1).Child(Text("Run")).
				Child(Div().Role(access.RoleList).
					Child(Div().Role(access.RoleListItem).AccessPosition(1, 1).AccessLevel(2).Child(Text("Watch")))),
		)
	})
	tr := r.tree()
	install := accesstest.Find(t, tr, access.RoleListItem, "Install")
	if install.PosInSet != 1 || install.SetSize != 2 || install.Level != 1 {
		t.Errorf("Install is %d of %d at level %d, want 1 of 2 at level 1", install.PosInSet, install.SetSize, install.Level)
	}
	watch := accesstest.Find(t, tr, access.RoleListItem, "Watch")
	if watch.PosInSet != 1 || watch.SetSize != 1 || watch.Level != 2 {
		t.Errorf("Watch is %d of %d at level %d, want 1 of 1 at level 2", watch.PosInSet, watch.SetSize, watch.Level)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// The new methods panic on numbers that describe no place.
func TestAccessGridCellPositionAndLevelPanicOnBadNumbers(t *testing.T) {
	cases := map[string]func(){
		"AccessGrid(-1, 2)":     func() { Div().AccessGrid(-1, 2) },
		"AccessCell(1, 0)":      func() { Div().AccessCell(1, 0) },
		"AccessCell(-1, 1)":     func() { Div().AccessCell(-1, 1) },
		"AccessPosition(0, 3)":  func() { Div().AccessPosition(0, 3) },
		"AccessPosition(4, 3)":  func() { Div().AccessPosition(4, 3) },
		"AccessLevel(0)":        func() { Div().AccessLevel(0) },
		"AccessPosition(1, 0)":  func() { Div().AccessPosition(1, 0) },
		"AccessGrid(0, -1)":     func() { Div().AccessGrid(0, -1) },
		"AccessCell(0, -2)":     func() { Div().AccessCell(0, -2) },
		"AccessPosition(-1, 1)": func() { Div().AccessPosition(-1, 1) },
	}
	for name, fn := range cases {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s did not panic", name)
				}
			}()
			fn()
		}()
	}
}

// The examples of "Custom and virtualized grids" in docs/accessibility.md: a grid
// that builds only the rows on screen, and one drawn on a canvas, report the
// shape of the whole data and the place of each built cell, with no warnings.
func TestAccessGridOfACustomGrid(t *testing.T) {
	cols := []string{"id", "name"}
	rows := make([][]string, 1000)
	for i := range rows {
		rows[i] = []string{strconv.Itoa(i + 1), "row " + strconv.Itoa(i+1)}
	}
	first, last := 500, 519 // the rows on screen

	// The grid of Divs, as the guide has it.
	r := newAccessRig(func(ctx *Context) Element {
		grid := Div().Role(access.RoleDataGrid).Label("Results").AccessGrid(len(rows), len(cols)).ScrollY()
		head := Div().Row()
		for c, name := range cols {
			head.Child(Div().Role(access.RoleHeaderItem).AccessCell(0, c+1).Width(Px(120)).Child(Text(name)))
		}
		grid.Child(head)
		for r := first; r <= last; r++ {
			row := Div().Row().Key(strconv.Itoa(r)) // no role: its cells sit in the grid
			for c, v := range rows[r] {
				row.Child(Div().Role(access.RoleDataItem).AccessCell(r+1, c+1).Width(Px(120)).Child(Text(v)))
			}
			grid.Child(row)
		}
		return grid.Height(Px(300))
	})
	checkCustomGrid(t, r, "row 501")
	grid := accesstest.Find(t, r.tree(), access.RoleDataGrid, "Results")
	if !grid.Scroll.Container {
		t.Errorf("the grid's scroll container is not reported")
	}

	// The grid drawn on a canvas, with a Div over each cell on screen.
	var down []float32
	const cw, ch = 120, 20
	r = newAccessRig(func(ctx *Context) Element {
		var cells []Element
		for c, name := range cols {
			cells = append(cells, Div().Absolute().Left(float32(c*cw)).Top(0).Size(Px(cw), Px(ch)).
				Role(access.RoleHeaderItem).AccessCell(0, c+1).Label(name))
		}
		for r := first; r <= last; r++ {
			for c := range cols {
				cells = append(cells, Div().Absolute().Left(float32(c*cw)).Top(float32((r-first+1)*ch)).
					Size(Px(cw), Px(ch)).Role(access.RoleDataItem).AccessCell(r+1, c+1).Label(rows[r][c]))
			}
		}
		return Div().Child(Canvas(func(p *Painter, w, h float32) {}).Size(Px(240), Px(440)).
			Role(access.RoleDataGrid).Label("Results").AccessGrid(len(rows), len(cols)).
			OnMouseDown(func(e *MouseEvent) { down = append(down, e.LocalX, e.LocalY) }).
			Children(cells...))
	})
	checkCustomGrid(t, r, "row 501")
	// A press on a cell's Div reaches the canvas, in the canvas's coordinates.
	r.clickAt(cw+10, ch+5)
	if len(down) != 2 || down[0] != cw+10 || down[1] != ch+5 {
		t.Errorf("the canvas got presses at %v, want one at (%v, %v)", down, cw+10, ch+5)
	}
}

// checkCustomGrid checks the tree of the guide's grid of 1000 rows by 2 columns
// with rows 501 to 520 built, whose first cell of the second column is name.
func checkCustomGrid(t *testing.T, r *accessRig, name string) {
	t.Helper()
	tr := r.tree()
	grid := accesstest.Find(t, tr, access.RoleDataGrid, "Results")
	if grid.GridRows != 1000 || grid.GridColumns != 2 {
		t.Errorf("grid shape %d by %d, want 1000 by 2", grid.GridRows, grid.GridColumns)
	}
	if h := accesstest.Find(t, tr, access.RoleHeaderItem, "name"); h.GridColumn != 2 {
		t.Errorf("the name header heads column %d, want 2", h.GridColumn)
	}
	if c := accesstest.Find(t, tr, access.RoleDataItem, name); c.GridRow != 501 || c.GridColumn != 2 {
		t.Errorf("%q at row %d, column %d, want row 501, column 2", name, c.GridRow, c.GridColumn)
	}
	accesstest.Absent(t, tr, access.RoleDataItem, "row 1")
	if len(r.warnings) != 0 {
		t.Errorf("warnings: %v", r.warnings)
	}
}

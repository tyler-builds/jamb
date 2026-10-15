package ui

import (
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

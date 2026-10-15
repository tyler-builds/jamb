//go:build windows

package windows

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/internal/win32"
	"github.com/tyler-builds/jamb/ui"
)

// tableBridge returns the access bridge of a runtime that shows a table and a list made
// of plain ui elements with AccessGrid, AccessCell, AccessPosition and AccessLevel, as
// the markdown view makes them. The header cells are in a row of their own and each
// row of data in a row element, neither of them in the tree, so the cells are found
// under the grid at any depth. This test does not import markdown, which would add
// goldmark to the module graph of every app that imports this package.
func tableBridge(t *testing.T) *access.Bridge {
	t.Helper()
	cell := func(role access.Role, row, col int, text string) ui.Element {
		return ui.Div().Role(role).AccessCell(row, col).Child(ui.Text(text))
	}
	rt := ui.NewRuntime(func(*ui.Context) ui.Element {
		return ui.Div().Children(
			ui.Div().Role(access.RoleDataGrid).Description("Table, 2 columns, 3 rows").AccessGrid(2, 2).Children(
				ui.Div().Row().Children(cell(access.RoleHeaderItem, 0, 1, "Name"), cell(access.RoleHeaderItem, 0, 2, "Year")),
				ui.Div().Row().Children(cell(access.RoleDataItem, 1, 1, "jamb"), cell(access.RoleDataItem, 1, 2, "2026")),
				ui.Div().Row().Children(cell(access.RoleDataItem, 2, 1, "notes"), cell(access.RoleDataItem, 2, 2, "2025")),
			),
			ui.Div().Role(access.RoleList).Children(
				ui.Div().Role(access.RoleListItem).AccessPosition(1, 2).AccessLevel(1).Child(ui.Text("Install")),
				ui.Div().Role(access.RoleListItem).AccessPosition(2, 2).AccessLevel(1).Child(ui.Text("Run")),
			),
		)
	}, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	b := rt.AccessBridge()
	b.Activate()
	rt.Frame(time.Unix(1_000_000, 0))
	return b
}

// TestGridAndTablePatternsOfPlainElements reads a table made with the ui API through
// the Grid, Table, GridItem and TableItem patterns, as a screen reader's table
// navigation does.
func TestGridAndTablePatternsOfPlainElements(t *testing.T) {
	b := tableBridge(t)
	tr := b.Tree()
	gridNode := accesstest.Find(t, tr, access.RoleDataGrid, "")
	name := accesstest.Find(t, tr, access.RoleHeaderItem, "Name")
	year := accesstest.Find(t, tr, access.RoleHeaderItem, "Year")
	notes2025 := accesstest.Find(t, tr, access.RoleDataItem, "2025")

	grid := patternOn(t, b, gridNode.ID, patternGrid)
	if rows, hr := getOut(comMethod(grid, 4), grid); hr != hrOK || int32(rows) != 2 {
		t.Errorf("RowCount = %d (HRESULT 0x%08X), want 2", int32(rows), uint32(hr))
	}
	if cols, hr := getOut(comMethod(grid, 5), grid); hr != hrOK || int32(cols) != 2 {
		t.Errorf("ColumnCount = %d (HRESULT 0x%08X), want 2", int32(cols), uint32(hr))
	}
	item, hr := getOut(comMethod(grid, 3), grid, 1, 1) // GetItem, 0-based: the second row of data, the second column
	if hr != hrOK || item == 0 || coreOf(item).id != notes2025.ID {
		t.Fatalf("GetItem(1, 1) = %#x (HRESULT 0x%08X), want the cell 2025", item, uint32(hr))
	}
	releaseCall(item)
	if none, hr := getOut(comMethod(grid, 3), grid, 2, 0); hr != hrOK || none != 0 {
		t.Errorf("GetItem(2, 0) past the last row = %#x (HRESULT 0x%08X), want null", none, uint32(hr))
	}

	table := patternOn(t, b, gridNode.ID, patternTable)
	psa, hr := getOut(comMethod(table, 4), table) // GetColumnHeaders
	if hr != hrOK || psa == 0 {
		t.Fatalf("GetColumnHeaders HRESULT 0x%08X, array %#x", uint32(hr), psa)
	}
	if ub := win32.SafeArrayUBound(psa); ub != 1 {
		t.Fatalf("GetColumnHeaders has %d items, want 2", ub+1)
	}
	for i, want := range []access.ID{name.ID, year.ID} {
		var h uintptr
		win32.SafeArrayGetUnknown(psa, int32(i), &h)
		if h == 0 || coreOf(h).id != want {
			t.Errorf("column header %d is not the header item %d", i, want)
		}
		if h != 0 {
			releaseCall(h)
		}
	}
	win32.SafeArrayDestroy(psa)

	gridItem := patternOn(t, b, notes2025.ID, patternGridItem)
	if row, _ := getOut(comMethod(gridItem, 3), gridItem); int32(row) != 1 {
		t.Errorf("GridItem Row = %d, want 1 (0-based)", int32(row))
	}
	if col, _ := getOut(comMethod(gridItem, 4), gridItem); int32(col) != 1 {
		t.Errorf("GridItem Column = %d, want 1 (0-based)", int32(col))
	}
	if g, hr := getOut(comMethod(gridItem, 7), gridItem); hr != hrOK || g == 0 || coreOf(g).id != gridNode.ID {
		t.Errorf("ContainingGrid = %#x (HRESULT 0x%08X), want the grid", g, uint32(hr))
	} else {
		releaseCall(g)
	}

	tableItem := patternOn(t, b, notes2025.ID, patternTableItem)
	psa, hr = getOut(comMethod(tableItem, 4), tableItem) // GetColumnHeaderItems
	if hr != hrOK || psa == 0 {
		t.Fatalf("GetColumnHeaderItems HRESULT 0x%08X, array %#x", uint32(hr), psa)
	}
	if ub := win32.SafeArrayUBound(psa); ub != 0 {
		t.Fatalf("GetColumnHeaderItems has %d items, want 1", ub+1)
	}
	var h uintptr
	win32.SafeArrayGetUnknown(psa, 0, &h)
	if h == 0 || coreOf(h).id != year.ID {
		t.Error("the column header of the cell 2025 is not Year")
	}
	if h != 0 {
		releaseCall(h)
	}
	win32.SafeArrayDestroy(psa)
}

// TestListItemPositionAndLevelProperties reads the PositionInSet, SizeOfSet and Level
// properties of list items made with AccessPosition and AccessLevel.
func TestListItemPositionAndLevelProperties(t *testing.T) {
	b := tableBridge(t)
	run := accesstest.Find(t, b.Tree(), access.RoleListItem, "Run")
	testHwnd++
	c := &core{kind: kindNode, id: run.ID, hwnd: testHwnd, bridge: b}
	for _, p := range []struct {
		name string
		id   int32
		want int32
	}{{"PositionInSet", propPositionInSet, 2}, {"SizeOfSet", propSizeOfSet, 2}, {"Level", propLevel, 1}} {
		v, hr := c.property(p.id)
		if hr != hrOK || v.vt != vtI4 || variantI4Of(v) != p.want {
			t.Errorf("%s = %d (tag %d, HRESULT 0x%08X), want %d", p.name, variantI4Of(v), v.vt, uint32(hr), p.want)
		}
	}
}

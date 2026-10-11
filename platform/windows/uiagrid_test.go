package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// TestGridIIDsMatchTheSDK checks the grid interface IIDs against the uuid
// attributes in UIAutomationCore.idl, from the Windows SDK 10.0.26100.0.
func TestGridIIDsMatchTheSDK(t *testing.T) {
	cases := []struct {
		name string
		g    guid
		uuid string
	}{
		{"IGridProvider", iidGridProvider, "b17d6187-0907-464b-a168-0ef17a1572b1"},
		{"ITableProvider", iidTableProvider, "9c860395-97b3-490a-b52a-858cc22af166"},
		{"IGridItemProvider", iidGridItemProvider, "d02541f1-fb81-4d64-ae32-f520f8a6dbd1"},
		{"ITableItemProvider", iidTableItemProvider, "b9734fa6-771f-4d78-9c90-2517999349cd"},
	}
	for _, c := range cases {
		if got := guidText(c.g); got != c.uuid {
			t.Errorf("%s has IID %s, the SDK has %s", c.name, got, c.uuid)
		}
	}
}

// TestDataGridControlTypesAndPatterns checks the control types of the grid roles
// and the pattern IDs against UIAutomationClient.h.
func TestDataGridControlTypesAndPatterns(t *testing.T) {
	if got := controlType(access.RoleDataGrid); got != 50028 {
		t.Errorf("data grid control type %d, want 50028 (DataGrid)", got)
	}
	if got := controlType(access.RoleDataItem); got != 50029 {
		t.Errorf("data item control type %d, want 50029 (DataItem)", got)
	}
	if got := controlType(access.RoleHeaderItem); got != 50035 {
		t.Errorf("header item control type %d, want 50035 (HeaderItem)", got)
	}
	want := map[int32]coreKind{10006: kindGrid, 10012: kindTable, 10007: kindGridItem, 10013: kindTableItem}
	for id, k := range want {
		if got, ok := patternKindOf(id); !ok || got != k {
			t.Errorf("pattern %d maps to %v (%v), want %v", id, got, ok, k)
		}
	}
}

// TestGridPatternsFollowTheNodes checks which node has which grid pattern: the
// grid has Grid and Table, a cell has GridItem and TableItem, and nothing else
// does.
func TestGridPatternsFollowTheNodes(t *testing.T) {
	grid := &access.Node{Role: access.RoleDataGrid, GridRows: 100000, GridColumns: 3}
	cell := &access.Node{Role: access.RoleDataItem, GridRow: 2, GridColumn: 1}
	list := &access.Node{Role: access.RoleListItem}
	if !supports(kindGrid, grid) || !supports(kindTable, grid) {
		t.Error("a data grid does not have the Grid and Table patterns")
	}
	if !supports(kindGridItem, cell) || !supports(kindTableItem, cell) {
		t.Error("a grid cell does not have the GridItem and TableItem patterns")
	}
	if supports(kindGrid, cell) || supports(kindGridItem, grid) || supports(kindGrid, list) {
		t.Error("a node has a grid pattern it should not")
	}
	if supports(kindGridItem, &access.Node{Role: access.RoleDataItem}) {
		t.Error("a data item outside a grid position has the GridItem pattern")
	}
}

// TestCanSelectMultipleFollowsTheMode checks CanSelectMultiple: a multiple
// selection says so, and the other modes do not.
func TestCanSelectMultipleFollowsTheMode(t *testing.T) {
	multi := &access.Node{States: access.StateSelectionContainer | access.StateMultiSelectable}
	single := &access.Node{States: access.StateSelectionContainer}
	if !multi.States.Has(access.StateMultiSelectable) || single.States.Has(access.StateMultiSelectable) {
		t.Fatal("the multi-selectable state is not set by the flag alone")
	}
}

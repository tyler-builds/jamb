package ui

import (
	"fmt"
	"slices"
	"testing"
)

// TestDataGridFindsRowsByKeyWhenKeysAreSorted checks the rows of a grid whose keys
// are in strictly increasing order, which the grid finds without its table of
// positions (see posOf): a key still finds its row after a sort, and the row is
// shown.
func TestDataGridFindsRowsByKeyWhenKeysAreSorted(t *testing.T) {
	items := make([]dgItem, 1000)
	for i := range items {
		items[i] = dgItem{ID: fmt.Sprintf("k%04d", i), Name: fmt.Sprintf("Item %06d", i), N: i % 3}
	}
	h := NewDataGridHandle()
	sort := NewSignal(SortState{Key: "Name", Desc: true})
	r := dgHandleRig(items, h, sort)
	h.ScrollToRow("k0010") // near the end of the descending order
	dgSettle(r)
	found := false
	for _, k := range dgRowKeys(r) {
		found = found || k == "k0010"
	}
	if !found {
		t.Fatalf("after ScrollToRow(k0010) the built rows are %v; want k0010 among them", dgRowKeys(r))
	}
}

// TestDataGridSortedKeysAreUnique checks that a grid whose keys are sorted, but has
// no repeated key, gives no duplicate-key warning, and that one with a repeated key
// still does (see the warning in build).
func TestDataGridSortedKeysAreUnique(t *testing.T) {
	var warnings []string
	items := make([]dgItem, 50)
	for i := range items {
		items[i] = dgItem{ID: fmt.Sprintf("k%04d", i), Name: fmt.Sprintf("Item %06d", i)}
	}
	rows := NewSignal(items)
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").
			Sort(NewSignal(SortState{Key: "Name", Desc: true}))
	})
	r.rt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
	dgSettle(r)
	if len(warnings) != 0 {
		t.Fatalf("unique sorted keys warned: %q", warnings)
	}
	dup := slices.Clone(items)
	dup[7].ID = dup[6].ID // no longer increasing, and a repeated key
	rows.Set(dup)
	dgSettle(r)
	if len(warnings) == 0 {
		t.Fatal("a repeated key did not warn")
	}
}

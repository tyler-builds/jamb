package ui

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// dgBruteDup reports whether keys has a key used twice.
func dgBruteDup(keys []string) bool {
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			return true
		}
		seen[k] = true
	}
	return false
}

// TestDataGridCountsKeysAcrossEdits checks the keys that makeKeys makes again only
// for the rows an edit changed, whether they are in order, and the check for a
// repeated key, against the same made from every row, over random inserts,
// removals, moves, replacements and sorts, each given as a new slice.
func TestDataGridCountsKeysAcrossEdits(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	key := func(x dgItem) string { return x.ID }
	sh := &dgShared[dgItem]{}
	// Keys out of order, so that they are counted and not taken as unique.
	rows := make([]dgItem, 40)
	for i := range rows {
		rows[i] = dgItem{ID: fmt.Sprintf("k%02d", 39-i)}
	}
	for step := range 2000 {
		rows = slices.Clone(rows)
		switch r.IntN(5) {
		case 0: // insert a key, sometimes one that is already there
			k := fmt.Sprintf("n%d", step)
			if r.IntN(4) == 0 && len(rows) > 0 {
				k = rows[r.IntN(len(rows))].ID
			}
			rows = slices.Insert(rows, r.IntN(len(rows)+1), dgItem{ID: k})
		case 1: // remove a row
			if len(rows) > 1 {
				i := r.IntN(len(rows))
				rows = slices.Delete(rows, i, i+1)
			}
		case 2: // move a row
			i, j := r.IntN(len(rows)), r.IntN(len(rows))
			x := rows[i]
			rows = slices.Insert(slices.Delete(rows, i, i+1), j, x)
		case 3: // replace a key
			rows[r.IntN(len(rows))].ID = fmt.Sprintf("r%d", step)
		case 4: // put the keys in order, or out of it
			if r.IntN(2) == 0 {
				slices.SortFunc(rows, func(a, b dgItem) int { return strings.Compare(a.ID, b.ID) })
			} else {
				slices.Reverse(rows)
			}
		}
		sh.update(rows, key, nil, SortState{}, true)
		keys := make([]string, len(rows))
		for i, x := range rows {
			keys[i] = x.ID
		}
		if !slices.Equal(sh.keys, keys) {
			t.Fatalf("step %d: keys %v, want %v", step, sh.keys, keys)
		}
		increasing := true
		for i := 1; i < len(keys); i++ {
			increasing = increasing && keys[i-1] < keys[i]
		}
		if sh.sortedKeys != increasing {
			t.Fatalf("step %d: sortedKeys %v, want %v; keys %v", step, sh.sortedKeys, increasing, keys)
		}
		if want := dgBruteDup(keys); (sh.dup != "") != want {
			t.Fatalf("step %d: dup %q, want a repeated key %v; keys %v", step, sh.dup, want, keys)
		}
		if sh.dup != "" && !slices.Contains(keys, sh.dup) {
			t.Fatalf("step %d: dup %q is not a key of the rows", step, sh.dup)
		}
	}
}

// TestDataGridWarnsOfAKeyRepeatedByAnInsert checks that a grid whose keys are not in
// order warns when an insert in the middle repeats a key, which only the keys the
// insert changed are counted for.
func TestDataGridWarnsOfAKeyRepeatedByAnInsert(t *testing.T) {
	var warnings []string
	items := make([]dgItem, 200)
	for i := range items {
		items[i] = dgItem{ID: fmt.Sprintf("k%04d", 199-i), Name: fmt.Sprintf("Item %d", i)}
	}
	rows := NewSignal(items)
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs")
	})
	r.rt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
	dgSettle(r)
	moved := slices.Clone(items)
	moved = slices.Insert(slices.Delete(moved, 3, 4), 50, items[3])
	rows.Set(moved)
	dgSettle(r)
	if len(warnings) != 0 {
		t.Fatalf("unique keys warned after a move: %q", warnings)
	}
	dup := slices.Insert(slices.Clone(moved), 100, dgItem{ID: moved[10].ID})
	rows.Set(dup)
	dgSettle(r)
	if len(warnings) != 1 {
		t.Fatalf("a key repeated by an insert gave %d warnings, want one: %q", len(warnings), warnings)
	}
}

// TestDataGridCursorFollowsItsRowAfterAnInsert checks that the focused cell stays on
// its row when rows are inserted above it, which posNear finds near its last
// position.
func TestDataGridCursorFollowsItsRowAfterAnInsert(t *testing.T) {
	items := dgItems(20)
	rows := NewSignal(items)
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").
			Selection(SelectionSingle, nil)
	})
	dgSettle(r)
	dgClick(r, dgCell(r, 5, 0), 0)
	dgSettle(r)
	before := dgFocusCell(r)
	if before == nil || before.div.acc.gridRow != 6 {
		t.Fatalf("a click on row 6 focused %+v", before)
	}
	rows.Set(slices.Insert(slices.Clone(items), 1, dgItem{ID: "new1"}, dgItem{ID: "new2"}))
	dgSettle(r)
	after := dgFocusCell(r)
	if after == nil || after.div.acc.gridRow != 8 {
		t.Fatalf("after two rows were inserted above it, the focus is on %+v, want row 8", after)
	}
}

// TestDataGridSameRowsByteForByte checks dgSameHead and dgSameTail on rows with
// pointers and padding, across the blocks they compare memory in.
func TestDataGridSameRowsByteForByte(t *testing.T) {
	type row struct {
		ID   string
		On   bool // followed by padding
		N    int64
		Next *row
	}
	a := make([]row, 1000)
	for i := range a {
		a[i] = row{ID: fmt.Sprint(i), On: i%2 == 0, N: int64(i), Next: &a[(i+1)%len(a)]}
	}
	for _, at := range []int{0, 1, 99, 100, 500, 998, 999} {
		b := slices.Clone(a)
		b[at].N = -1
		if h := dgSameHead(a, b); h != at {
			t.Errorf("a change at %d: head %d, want %d", at, h, at)
		}
		if tl := dgSameTail(a, b); tl != len(a)-1-at {
			t.Errorf("a change at %d: tail %d, want %d", at, tl, len(a)-1-at)
		}
	}
	ins := slices.Insert(slices.Clone(a), 300, row{ID: "new"})
	if h, tl := dgSameHead(a, ins), dgSameTail(a, ins); h != 300 || tl != 700 {
		t.Errorf("an insert at 300: head %d and tail %d, want 300 and 700", h, tl)
	}
	if h, tl := dgSameHead(a, a), dgSameTail(a[:0], a); h != len(a) || tl != 0 {
		t.Errorf("the same rows: head %d, want %d; no rows: tail %d, want 0", h, len(a), tl)
	}
}

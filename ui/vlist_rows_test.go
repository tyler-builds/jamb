package ui

import (
	"fmt"
	"testing"
)

// rowRig is a ListOf of n rows whose row function counts the calls for each row
// and shows the value of the row's signal. The row reads its signal in its own
// render, so the signal re-renders the row, not the list (see ListOf).
func rowRig(n int, h *ListHandle, calls map[int]int, vals map[int]*Signal[int]) *vRig {
	v := &vRig{items: NewSignal(ids(n)), tick: NewSignal(0), tall: map[int]float32{}}
	v.overlayRig = newOverlayRig(func(ctx *Context) Element {
		_ = v.tick.Get()
		return Div().Child(ListOf(v.items.Get(), vKey, func(_ int, id int) Element {
			calls[id]++
			return Div().Key(vKey(id)).Height(Px(vHeight(id))).Child(Text(fmt.Sprint(vals[id].Get())))
		}).Key("list").Style(Style{Height: Px(400)}).Handle(h))
	}, 800, 600)
	return v
}

// TestListRowsAreBuiltOnlyWhenTheyComeIntoTheWindow checks that moving the window
// by a row does not render the rows that stay in it: only the row that comes in is
// rendered (see ListOf).
func TestListRowsAreBuiltOnlyWhenTheyComeIntoTheWindow(t *testing.T) {
	calls := map[int]int{}
	vals := map[int]*Signal[int]{}
	for i := range 200 {
		vals[i] = NewSignal(i)
	}
	v := rowRig(200, nil, calls, vals)
	v.frame()
	v.settle()
	vs := v.state()
	before := map[string]bool{}
	for i := vs.builtFrom; i < vs.built; i++ {
		before[vs.keys[i]] = true
	}
	snapshot := map[int]int{}
	for id, c := range calls {
		snapshot[id] = c
	}
	v.rt.Scroll(100, 100, 0, 400, true, 0)
	for range 4 {
		v.frame()
	}
	v.settle()
	fresh, kept := 0, 0
	for i := vs.builtFrom; i < vs.built; i++ {
		if before[vs.keys[i]] {
			kept++
		} else {
			fresh++
		}
	}
	if fresh == 0 || kept == 0 {
		t.Fatalf("the scroll moved the window to %d new rows and kept %d, want both", fresh, kept)
	}
	for id, c := range calls {
		if before[vKey(id)] && c != snapshot[id] {
			t.Errorf("row %d, which stayed in the window, was rendered %d more times", id, c-snapshot[id])
		}
	}
}

// TestRowRerendersWhenItsSignalChanges checks that a row whose signal changes is
// rendered again, and that the other rows, and the list, are not.
func TestRowRerendersWhenItsSignalChanges(t *testing.T) {
	calls := map[int]int{}
	vals := map[int]*Signal[int]{}
	for i := range 20 {
		vals[i] = NewSignal(i * 10)
	}
	v := rowRig(20, nil, calls, vals)
	v.frame()
	rowBefore, otherBefore := calls[3], calls[4]
	vals[3].Set(777)
	v.frame()
	if calls[3] != rowBefore+1 {
		t.Errorf("row 3 rendered %d times after its signal changed, want once", calls[3]-rowBefore)
	}
	if calls[4] != otherBefore {
		t.Errorf("row 4 rendered again though its signal did not change")
	}
	if n := v.overlayRig.rt.tree; findText(n, "777") == nil {
		t.Error("row 3 does not show its new value")
	}
}

// TestListRerendersWhenItsOwnStateChanges checks that a change to the list's own
// state, a scroll to a row out of view, builds the rows of the new window with the
// values they have.
func TestListRerendersWhenItsOwnStateChanges(t *testing.T) {
	calls := map[int]int{}
	vals := map[int]*Signal[int]{}
	for i := range 5000 {
		vals[i] = NewSignal(i)
	}
	h := NewListHandle()
	v := rowRig(5000, h, calls, vals)
	v.frame()
	h.ScrollToItem(3000)
	v.settle()
	vs := v.state()
	if vs.builtFrom > 3000 || vs.built <= 3000 {
		t.Fatalf("the window is [%d, %d), want row 3000 in it", vs.builtFrom, vs.built)
	}
	if calls[3000] == 0 {
		t.Fatal("row 3000 was not rendered")
	}
	if findText(v.overlayRig.rt.tree, "3000") == nil {
		t.Error("the row in the new window does not show its value")
	}
}

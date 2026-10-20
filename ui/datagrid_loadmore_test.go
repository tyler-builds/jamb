package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// dgLoadRig returns a rig with a grid of the items in rows that asks for more with
// LoadMore while more is set, counting the calls in calls, and shows LoadingMore
// while loading is set.
func dgLoadRig(rows *Signal[[]dgItem], more, loading *Signal[bool], calls *int) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return DataGrid(rows.Get(), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").
			Selection(SelectionSingle, nil).
			LoadMore(more.Get(), func() { *calls++ }).LoadingMore(loading.Get())
	})
}

// dgScrollTo scrolls the grid's body to y and lets the scroll come to rest.
func dgScrollTo(r *accessRig, body *node, y float32) {
	body.st.targetY = y
	r.rt.hostInvalidate()
	dgSettle(r)
}

// TestDataGridLoadMoreAsksOnceNearTheEnd checks that LoadMore asks for rows once when
// the rows near the end are built, and again only after the rows have grown.
func TestDataGridLoadMoreAsksOnceNearTheEnd(t *testing.T) {
	calls := 0
	rows := NewSignal(dgItems(100))
	r := dgLoadRig(rows, NewSignal(true), NewSignal(false), &calls)
	dgSettle(r)
	if calls != 0 {
		t.Fatalf("LoadMore asked %d times at the top of 100 rows, want none", calls)
	}
	body := dgBody(t, r)
	dgScrollTo(r, body, 1e6) // to the end
	if calls != 1 {
		t.Fatalf("LoadMore asked %d times at the end, want once", calls)
	}
	dgScrollTo(r, body, body.st.targetY-40)
	dgScrollTo(r, body, 1e6)
	if calls != 1 {
		t.Errorf("LoadMore asked %d times with the same rows, want once", calls)
	}
	rows.Set(dgItems(150))
	dgSettle(r)
	if calls != 1 {
		t.Errorf("LoadMore asked %d times after 50 rows were added below the view, want once", calls)
	}
	dgScrollTo(r, body, 1e6)
	if calls != 2 {
		t.Errorf("LoadMore asked %d times at the end of the new rows, want twice", calls)
	}
}

// TestDataGridLoadMoreByKeyboard checks that Ctrl+End, which moves to the last row,
// asks for more rows, and that LoadMore does not ask without hasMore.
func TestDataGridLoadMoreByKeyboard(t *testing.T) {
	for _, more := range []bool{false, true} {
		calls := 0
		r := dgLoadRig(NewSignal(dgItems(100)), NewSignal(more), NewSignal(false), &calls)
		focusNode(r, dgBody(t, r))
		r.press(input.KeyEnd, input.ModCtrl)
		r.press(input.KeyDown, 0)
		r.press(input.KeyPageDown, 0)
		dgSettle(r)
		if want := map[bool]int{false: 0, true: 1}[more]; calls != want {
			t.Errorf("with hasMore %v, Ctrl+End, Down and PageDown asked %d times, want %d", more, calls, want)
		}
	}
}

// TestDataGridLoadingMoreRow checks that LoadingMore shows a row after the last one,
// named for assistive technology and announced, that is not a row of the grid, and
// that Loading still shows its ring over the rows.
func TestDataGridLoadingMoreRow(t *testing.T) {
	calls := 0
	loading := NewSignal(false)
	r := dgLoadRig(NewSignal(dgItems(5)), NewSignal(false), loading, &calls)
	takeAnnouncements()
	var said []access.Announcement
	r.bridge.Listen(func(u access.Update) { said = append(said, u.Announcements...) })
	loading.Set(true)
	tree := r.tree()
	ring := tree.Find(access.RoleProgressBar, "Loading more rows")
	if ring == nil {
		t.Fatal("no progress ring named Loading more rows")
	}
	announced := false
	for _, a := range append(said, takeAnnouncements()...) {
		announced = announced || (a.Text == "Loading more rows" && a.Politeness == access.Polite)
	}
	if !announced {
		t.Error("LoadingMore was not announced politely")
	}
	g := r.stateOf(t, access.RoleDataGrid, "Runs")
	if g.GridRows != 5 {
		t.Errorf("the grid has %d rows with the loading row, want 5", g.GridRows)
	}
	var tail *node
	for _, nd := range r.rt.byID {
		if nd.key == "jamb.list.tail" {
			tail = nd
		}
	}
	if tail == nil {
		t.Fatal("no tail row built")
	}
	for n := tail; n != nil; n = n.parent {
		if a := n.div; a != nil && a.acc != nil && (a.acc.setPos != 0 || a.acc.gridRow != 0) {
			t.Errorf("the loading row has a list position %d or grid row %d", a.acc.setPos, a.acc.gridRow)
		}
	}
	if rows := partNodes(r.rt, "DataGrid.Row"); len(rows) != 5 {
		t.Errorf("the grid has %d built rows with the loading row, want 5", len(rows))
	}
	last := partNodes(r.rt, "DataGrid.Row")
	var bottom float32
	for _, n := range last {
		bottom = max(bottom, r.rt.nodeRect(n).Y+r.rt.nodeRect(n).H)
	}
	if y := r.rt.nodeRect(tail).Y; y < bottom-1 {
		t.Errorf("the loading row is at %v, above the end of the rows at %v", y, bottom)
	}

	r = newAccessRig(func(ctx *Context) Element {
		return DataGrid(dgItems(5), func(x dgItem) string { return x.ID }, dgColumns()...).Label("Runs").Loading(true)
	})
	if r.tree().Find(access.RoleProgressBar, "Loading") == nil {
		t.Error("Loading no longer shows its ring")
	}
}

package ui

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/scene"
)

// vKey is the key of the item with the given ID.
func vKey(id int) string { return fmt.Sprintf("v-%d", id) }

// vHeight is the height of a row. Rows differ in height so that the list has
// to measure them.
func vHeight(id int) float32 { return 20 + float32(id%5)*10 }

// ids returns the IDs 0..n-1.
func ids(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

// vRig is a ListOf over item IDs. tick re-renders the list without changing
// its items; tall adds height to the rows of the IDs it names.
type vRig struct {
	*overlayRig
	items *Signal[[]int]
	tick  *Signal[int]
	tall  map[int]float32
}

func newVRig(items []int, h *ListHandle) *vRig {
	v := &vRig{items: NewSignal(items), tick: NewSignal(0), tall: map[int]float32{}}
	v.overlayRig = newOverlayRig(func(ctx *Context) Element {
		_ = v.tick.Get()
		return Div().Child(ListOf(v.items.Get(), vKey, func(_ int, id int) Element {
			return Div().Key(vKey(id)).Height(Px(vHeight(id) + v.tall[id])).Child(Text(vKey(id)))
		}).Key("list").Style(Style{Height: Px(400)}).Handle(h))
	}, 800, 600)
	return v
}

// container is the scroll container of the list.
func (v *vRig) container() *node { return listContainer(v.overlayRig) }

// state is the list's persistent state.
func (v *vRig) state() *varState { return v.container().st.varList }

// settle runs frames until the scroll offset has reached its target. It
// always runs one frame: a precise wheel step moves the offset at once, and
// the paint that follows it is what lays out the rows.
func (v *vRig) settle() {
	for i := range 300 {
		v.now = v.now.Add(16 * time.Millisecond)
		v.frame()
		if c := v.container(); i > 0 && c.st.scrollY == c.st.targetY {
			return
		}
	}
}

// rowY returns the screen Y of every built row by ID in [0, n).
func (v *vRig) rowY(t *testing.T, n int) map[int]float32 {
	out := map[int]float32{}
	for id := range n {
		if rc, ok := v.rt.Find(vKey(id)); ok {
			out[id] = rc.Y
		}
	}
	return out
}

func TestListOfBuildsVisibleRows(t *testing.T) {
	v := newVRig(ids(100_000), nil)
	list := v.find(t, "list")
	if n := len(v.rt.byID); n > 400 {
		t.Errorf("%d nodes built for a 100,000-item list, want fewer than 400", n)
	}
	if _, ok := v.rt.Find(vKey(50_000)); ok {
		t.Error("item 50,000 is built although it is far below the viewport")
	}
	// Rows at the top are measured in order, so each sits at the sum of the
	// heights above it.
	top := float32(0)
	for i := range 6 {
		rc := v.find(t, vKey(i))
		if math.Abs(float64(rc.Y-(list.Y+top))) > 0.5 {
			t.Errorf("item %d at Y %v, want %v", i, rc.Y, list.Y+top)
		}
		top += vHeight(i)
	}
	// The content is the measured rows plus the estimate for the rest.
	s := v.state()
	est := s.hidx.totSum / float64(s.hidx.totCnt)
	want := s.hidx.totSum + est*float64(100_000-s.hidx.totCnt)
	if got := float64(v.container().lay.ContentH); math.Abs(got-want) > 1 {
		t.Errorf("content height %v, want measured sum + estimate x unmeasured = %v", got, want)
	}
	if s.hidx.totCnt == 0 {
		t.Fatal("no row was measured")
	}
	if len(v.rt.scrollbars(v.container(), nil)) == 0 {
		t.Error("no scrollbar on a list taller than its viewport")
	}
}

func TestListOfWheelScrollHasNoJumps(t *testing.T) {
	v := newVRig(ids(500), nil)
	prev := map[int]float32{}
	for step := range 25 {
		v.rt.Scroll(100, 100, 0, 120, true, 0)
		v.settle()
		cur := v.rowY(t, 500)
		if step > 0 {
			for id, y := range cur {
				p, ok := prev[id]
				if ok && math.Abs(float64(p-y-120)) > 0.01 {
					t.Fatalf("step %d: item %d moved %v, want exactly 120 (a measurement correction moved the view)", step, id, p-y)
				}
			}
		}
		prev = cur
	}

	// Scroll through the whole list: every row is then measured, so the
	// content height is the exact sum of the heights.
	for range 400 {
		if c := v.container(); c.st.targetY >= c.lay.ContentH-c.lay.H {
			break
		}
		v.rt.Scroll(100, 100, 0, 120, true, 0)
		v.settle()
	}
	sum := float32(0)
	for id := range 500 {
		sum += vHeight(id)
	}
	if got := v.container().lay.ContentH; got != sum {
		t.Errorf("content height %v after scrolling through all 500 items, want the exact sum %v", got, sum)
	}
}

func TestListOfScrollToItemFarAway(t *testing.T) {
	h := NewListHandle()
	v := newVRig(ids(100_000), h)
	h.ScrollToItem(50_000)
	for range 120 {
		v.now = v.now.Add(16 * time.Millisecond)
		v.frame()
	}
	list := v.find(t, "list")
	row := v.find(t, vKey(50_000))
	if row.Y < list.Y || row.Y+row.H > list.Y+list.H {
		t.Fatalf("item 50,000 %+v is not fully inside the list %+v", row, list)
	}
}

// vAnchorRig is a 1000-item list scrolled so that item 300 is at the top.
func vAnchorRig(t *testing.T) (*vRig, *ListHandle) {
	h := NewListHandle()
	v := newVRig(ids(1000), h)
	h.ScrollToItem(300)
	v.settle()
	if got := v.container().st.targetY; got == 0 {
		t.Fatal("list did not scroll")
	}
	return v, h
}

func TestListOfAnchorSurvivesInsertAndRemove(t *testing.T) {
	v, _ := vAnchorRig(t)
	before := v.find(t, vKey(300)).Y

	// Insert 20 items before index 100. They are above the window, so they
	// are not built and keep the estimate.
	src := ids(1000)
	inserted := make([]int, 0, 1020)
	inserted = append(inserted, src[:100]...)
	for i := range 20 {
		inserted = append(inserted, 1000+i)
	}
	inserted = append(inserted, src[100:]...)
	v.items.Set(inserted)
	v.frame()
	if got := v.find(t, vKey(300)).Y; math.Abs(float64(got-before)) > 0.5 {
		t.Fatalf("item 300 moved from %v to %v after 20 inserts above it", before, got)
	}

	// Remove 50 items above it.
	v.items.Set(inserted[50:])
	v.frame()
	if got := v.find(t, vKey(300)).Y; math.Abs(float64(got-before)) > 0.5 {
		t.Fatalf("item 300 moved from %v to %v after 50 removals above it", before, got)
	}
}

// topRow is the index and screen Y of the first row showing at the top of the
// list (the anchor).
func topRow(t *testing.T, v *vRig) (int, float32) {
	t.Helper()
	list := v.find(t, "list")
	for i := range 1000 {
		if rc, ok := v.rt.Find(vKey(i)); ok && rc.Y <= list.Y+0.5 && rc.Y+rc.H > list.Y+0.5 {
			return i, rc.Y
		}
	}
	t.Fatal("no row at the top of the list")
	return 0, 0
}

func TestListOfAnchorSurvivesMeasuredHeightChange(t *testing.T) {
	v, _ := vAnchorRig(t)
	at, before := topRow(t, v)

	// Replace an item above the anchor (built, in the window) with a taller
	// one. The new key is measured once it is built, and the anchor moves
	// only by the difference that lies above it.
	src := ids(1000)
	src[at-3] = 5000
	v.tall[5000] = 80
	v.items.Set(src)
	v.frame()
	if got := v.find(t, vKey(src[at])).Y; math.Abs(float64(got-before)) > 0.5 {
		t.Fatalf("anchor item %d moved from %v to %v after a taller item above it was built", at, before, got)
	}
	if got, want := v.find(t, vKey(5000)).H, vHeight(5000)+80; math.Abs(float64(got-want)) > 0.5 {
		t.Fatalf("replacement row height %v, want %v", got, want)
	}
}

func TestListOfUnbuiltHeightChangeIsEstimated(t *testing.T) {
	v, _ := vAnchorRig(t)
	before := v.find(t, vKey(300)).Y

	// Item 2 is far above the window, so it is not built: its new key keeps
	// the estimate until it is built, and nothing panics.
	src := ids(1000)
	src[2] = 5000
	v.tall[5000] = 80
	v.items.Set(src)
	v.frame()
	if _, ok := v.rt.Find(vKey(5000)); ok {
		t.Fatal("item 2 is built although it is far above the window")
	}
	// The anchor keeps its place through the estimate change.
	if got := v.find(t, vKey(300)).Y; math.Abs(float64(got-before)) > 0.5 {
		t.Fatalf("item 300 moved from %v to %v after an unbuilt item above it changed", before, got)
	}
}

func TestListOfAnchorItemRemoved(t *testing.T) {
	v, _ := vAnchorRig(t)
	before := v.find(t, vKey(300)).Y
	src := ids(1000)
	v.items.Set(append(append([]int{}, src[:300]...), src[301:]...))
	v.frame()
	// The anchor resolves to the item now at its old index (item 301).
	if got := v.find(t, vKey(301)).Y; math.Abs(float64(got-before)) > 0.5 {
		t.Fatalf("item 301 at %v after item 300 was removed, want %v", got, before)
	}
	// Removing every item must not panic either.
	v.items.Set(nil)
	v.frame()
	if n := v.container().st.varList.count; n != 0 {
		t.Fatalf("count %d after removing every item", n)
	}
}

// bottomRig is a list of items with heights set by vHeight, at the bottom of
// a 600px list.
func bottomRig(items []int, h *ListHandle) (*overlayRig, *Signal[[]int]) {
	s := NewSignal(items)
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Child(ListOf(s.Get(), vKey, func(_ int, id int) Element {
			return Div().Key(vKey(id)).Height(Px(vHeight(id))).Child(Text(vKey(id)))
		}).Key("list").Style(Style{Height: Px(600)}).Bottom().Handle(h))
	}, 800, 700)
	return r, s
}

func TestListOfBottomAndFollowTail(t *testing.T) {
	h := NewListHandle()
	r, items := bottomRig(ids(3), h)
	list := r.find(t, "list")
	last := r.find(t, vKey(2))
	if math.Abs(float64(last.Y+last.H-(list.Y+list.H))) > 0.5 {
		t.Fatalf("last short item ends at %v, want the bottom of the list %v", last.Y+last.H, list.Y+list.H)
	}
	if !h.AtEnd() {
		t.Fatal("a short list is not at its end")
	}

	// Appending while at the end keeps the new last item in view.
	appended := append(ids(3), ids(40)...)
	for i := range appended[3:] {
		appended[3+i] = 100 + i
	}
	items.Set(appended)
	r.frame()
	for range 3 {
		r.frame()
	}
	if !h.AtEnd() {
		t.Fatal("not at the end after items were appended at the end")
	}
	list = r.find(t, "list")
	lastRow := r.find(t, vKey(139))
	if lastRow.Y < list.Y || lastRow.Y+lastRow.H > list.Y+list.H {
		t.Fatalf("new last item %+v is not inside the list %+v", lastRow, list)
	}

	// Scroll up a little, then append: the view stays where it is.
	r.rt.Scroll(100, 100, 0, -100, true, 0)
	for range 60 {
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
	}
	if h.AtEnd() {
		t.Fatal("still at the end after scrolling up")
	}
	before := r.find(t, vKey(120)).Y
	more := append(append([]int{}, appended...), 300, 301, 302)
	items.Set(more)
	r.frame()
	if got := r.find(t, vKey(120)).Y; math.Abs(float64(got-before)) > 0.5 {
		t.Fatalf("view moved from %v to %v when items were appended while scrolled up", before, got)
	}
}

func TestListOfKeyedStateSurvivesScrollAndReorder(t *testing.T) {
	locals := map[int]*Signal[int]{}
	items := NewSignal(ids(200))
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Child(ListOf(items.Get(), vKey, func(_ int, id int) Element {
			return Component(func(ctx *Context) Element {
				n := Local(ctx, "n", id+100)
				locals[id] = n
				return Div().Key(vKey(id)).Height(Px(vHeight(id))).Child(Text(fmt.Sprint(n.Get())))
			}).Key(vKey(id))
		}).Key("list").Style(Style{Height: Px(400)}))
	}, 800, 600)
	locals[3].Set(777)
	r.frame()
	r.rt.Scroll(100, 100, 0, 40, true, 0)
	r.frame()
	if v := locals[3].Peek(); v != 777 {
		t.Fatalf("item 3 local is %d after scrolling, want 777", v)
	}
	swapped := ids(200)
	swapped[2], swapped[3] = swapped[3], swapped[2]
	items.Set(swapped)
	r.frame()
	if v := locals[3].Peek(); v != 777 {
		t.Fatalf("item 3 local is %d after reordering, want 777", v)
	}
}

func TestListOfSameItemsDoNotRebuild(t *testing.T) {
	v := newVRig(ids(100_000), nil)
	v.frame()
	before := v.state().rebuilds
	for range 5 {
		v.tick.Set(v.tick.Peek() + 1)
		v.frame()
	}
	if got := v.state().rebuilds; got != before {
		t.Fatalf("%d index rebuilds for re-renders with the same items, want none", got-before)
	}
	v.items.Set(ids(100_000))
	v.frame()
	if got := v.state().rebuilds; got != before+1 {
		t.Fatalf("new slice caused %d rebuilds, want 1", got-before)
	}
}

func TestListOfDuplicateKeysWarnOnce(t *testing.T) {
	var warnings []string
	items := NewSignal([]int{1, 2, 2, 3})
	r := &overlayRig{now: time.Unix(1_000_000, 0)}
	r.rt = NewRuntime(func(ctx *Context) Element {
		return Div().Child(ListOf(items.Get(), vKey, func(_ int, id int) Element {
			return Div().Key(vKey(id)).Height(Px(30)).Child(Text(vKey(id)))
		}).Key("list").Style(Style{Height: Px(400)}))
	}, DarkTheme(), nil)
	r.rt.clock = func() time.Time { return r.now }
	r.rt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
	r.rt.SetViewport(800, 600, 1)
	r.frame()
	for range 3 {
		r.rt.Scroll(100, 100, 0, 10, true, 0)
		r.frame()
	}
	items.Set([]int{1, 2, 2, 3, 4, 2})
	r.frame()
	if len(warnings) != 1 {
		t.Fatalf("%d warnings %q for duplicate keys, want exactly one", len(warnings), warnings)
	}
	r.find(t, vKey(3))
}

func BenchmarkListOf(b *testing.B) {
	newApp := func() (*Runtime, *Signal[[]int]) {
		items := NewSignal(ids(100_000))
		rt := NewRuntime(func(ctx *Context) Element {
			return Div().Height(Full).Child(ListOf(items.Get(), vKey, func(_ int, id int) Element {
				return Div().Key(vKey(id)).Height(Px(vHeight(id))).Child(Text(vKey(id)))
			}).Style(Style{Height: Px(800)}))
		}, DarkTheme(), benchText)
		rt.SetViewport(1200, 800, 1)
		rt.Frame(time.Now())
		return rt, items
	}
	b.Run("build/100k", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			newApp()
		}
	})
	b.Run("scroll/100k", func(b *testing.B) {
		rt, _ := newApp()
		container := rt.tree.children[0]
		b.ReportAllocs()
		for b.Loop() {
			if maxY := container.lay.ContentH - container.lay.H; container.st.targetY >= maxY-240 {
				rt.Scroll(100, 100, 0, -1e9, true, 0)
			}
			rt.Scroll(100, 100, 0, 120, true, 0)
			rt.Frame(time.Now())
		}
		b.ReportMetric(float64(len(rt.byID)), "nodes")
	})
	b.Run("rebuild/100k", func(b *testing.B) {
		rt, items := newApp()
		// Two slices with the same contents: each operation changes identity.
		a, c := ids(100_000), ids(100_000)
		b.ReportAllocs()
		for i := 0; b.Loop(); i++ {
			if i%2 == 0 {
				items.Set(a)
			} else {
				items.Set(c)
			}
			rt.Frame(time.Now())
		}
		b.ReportMetric(float64(len(rt.byID)), "nodes")
	})
}

// A list that has settled asks for no more frames, even when a row has no
// height (so it is never measured) and a ScrollToItem was pending for it.
func TestListOfGoesIdle(t *testing.T) {
	h := NewListHandle()
	items := NewSignal(ids(200))
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Child(ListOf(items.Get(), vKey, func(_ int, id int) Element {
			if id%2 == 0 {
				return Div().Key(vKey(id)) // no height and no content
			}
			return Div().Key(vKey(id)).Height(Px(vHeight(id))).Child(Text(vKey(id)))
		}).Key("list").Style(Style{Height: Px(400)}).Handle(h))
	}, 800, 600)
	h.ScrollToItem(150)
	for range 200 {
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
	}
	if r.rt.NeedsFrame() {
		t.Fatal("the list still asks for frames after it settled")
	}
}

// checkCovered fails if some part of the list's viewport has no built row
// over it. Rows are the nodes whose keys start with prefix.
func checkCovered(t *testing.T, r *overlayRig, prefix string) {
	t.Helper()
	list := r.find(t, "list")
	var rows []scene.Rect
	for _, n := range r.rt.byID {
		if strings.HasPrefix(n.key, prefix) {
			rows = append(rows, r.rt.nodeRect(n))
		}
	}
	for y := list.Y; y < list.Y+list.H; y++ {
		covered := false
		for _, rc := range rows {
			if rc.Y <= y && y < rc.Y+rc.H {
				covered = true
				break
			}
		}
		if !covered {
			t.Fatalf("no built row covers y=%v of the list %+v (%d rows built)", y, list, len(rows))
		}
	}
}

// scrollCoverage runs frames while ScrollToItem(i) animates, and checks after
// each frame that the viewport is fully covered by built rows.
func scrollCoverage(t *testing.T, r *overlayRig, h *ListHandle, prefix string, i, frames int) {
	t.Helper()
	h.ScrollToItem(i)
	for f := range frames {
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
		if f == 0 || f%10 == 0 {
			checkCovered(t, r, prefix)
		}
	}
}

func TestListOfNearJumpAnimates(t *testing.T) {
	h := NewListHandle()
	v := newVRig(ids(1000), h)
	h.ScrollToItem(12) // about 500 px down: nearer than a viewport
	eased := false
	for range 10 {
		v.now = v.now.Add(16 * time.Millisecond)
		v.frame()
		c := v.container()
		if c.st.targetY > 0 && c.st.scrollY > 0 && c.st.scrollY < c.st.targetY {
			eased = true
			break
		}
	}
	if !eased {
		c := v.container()
		t.Fatalf("no frame showed the offset between 0 and the target: offset %v, target %v", c.st.scrollY, c.st.targetY)
	}
}

func TestListOfScrollCoverage(t *testing.T) {
	h := NewListHandle()
	v := newVRig(ids(100_000), h)
	t.Run("near", func(t *testing.T) {
		scrollCoverage(t, v.overlayRig, h, "v-", 12, 120)
	})
	t.Run("far", func(t *testing.T) {
		scrollCoverage(t, v.overlayRig, h, "v-", 50_000, 120)
	})
}

func TestUniformListScrollCoverage(t *testing.T) {
	h := NewListHandle()
	r := newOverlayRig(listRoot(constCount(100_000), listRow, h), 800, 600)
	t.Run("near", func(t *testing.T) {
		scrollCoverage(t, r, h, "row-", 12, 120)
	})
	t.Run("far", func(t *testing.T) {
		scrollCoverage(t, r, h, "row-", 5000, 120)
	})
}

func TestListOfFarScrollBuildsFewRows(t *testing.T) {
	h := NewListHandle()
	v := newVRig(ids(100_000), h)
	if most := maxNodesDuringScroll(v.overlayRig, h, 50_000, 120); most >= 300 {
		t.Fatalf("%d nodes in one frame of a far ScrollToItem, want fewer than 300", most)
	}
}

// Measured heights of items that left the list are dropped, so a list whose
// items keep changing does not grow its height table without bound.
func TestListOfHeightsForgetRemovedItems(t *testing.T) {
	v := newVRig(ids(300), nil)
	next := 300
	for range 50 {
		items := make([]int, 300)
		for i := range items {
			items[i] = next // fresh keys every time
			next++
		}
		v.items.Set(items)
		v.frame()
	}
	if n := len(v.state().heights); n > 300 {
		t.Fatalf("%d heights kept for a list of 300 items, want at most 300", n)
	}
}

// OnAtEnd reports the list's end state on the first paint and then each change,
// from the paint that makes it, and stays quiet while the state holds.
func TestListOfOnAtEndReportsChanges(t *testing.T) {
	h := NewListHandle()
	var seen []bool
	items := NewSignal(ids(100))
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Child(ListOf(items.Get(), vKey, func(_ int, id int) Element {
			return Div().Key(vKey(id)).Height(Px(vHeight(id))).Child(Text(vKey(id)))
		}).Key("list").Style(Style{Height: Px(300)}).Handle(h).
			OnAtEnd(func(atEnd bool) { seen = append(seen, atEnd) }))
	}, 800, 700)
	r.frame()
	if len(seen) != 1 || seen[0] {
		t.Fatalf("on the first paint, at the top, reported %v, want [false]", seen)
	}
	h.ScrollToEnd()
	for range 60 {
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
	}
	if len(seen) != 2 || !seen[1] {
		t.Fatalf("after ScrollToEnd reported %v, want [false true]", seen)
	}
	r.rt.Scroll(100, 100, 0, -100, true, 0)
	for range 60 {
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
	}
	if len(seen) != 3 || seen[2] {
		t.Fatalf("after scrolling up reported %v, want [false true false]", seen)
	}
}

// A ListOf row gets its index in the items along with the item.
func TestListOfRowGetsItsIndex(t *testing.T) {
	items := []string{"a", "b", "c"}
	got := map[string]int{}
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(ListOf(items, func(s string) string { return s }, func(i int, s string) Element {
			got[s] = i
			return Text(s)
		}).Height(Px(200)))
	}, 300, 300, 1)
	rt.Frame(time.Now())
	if got["a"] != 0 || got["b"] != 1 || got["c"] != 2 {
		t.Fatalf("rows saw indexes %v, want a:0 b:1 c:2", got)
	}
}

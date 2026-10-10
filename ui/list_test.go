package ui

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// listRow is a row with a fixed height and a key derived from its index.
func listRow(i int) Element {
	return Div().Key(fmt.Sprintf("row-%d", i)).Height(Px(40)).Child(Text(fmt.Sprintf("Row %d", i)))
}

// listRoot renders a 400px-tall list keyed "list".
func listRoot(count func() int, row func(i int) Element, h *ListHandle) func(*Context) Element {
	return func(ctx *Context) Element {
		return Div().Child(UniformList(count(), row).Key("list").Style(Style{Height: Px(400)}).Handle(h))
	}
}

func constCount(n int) func() int { return func() int { return n } }

// listContainer is the scroll container of the list in a listRoot app.
func listContainer(r *overlayRig) *node { return r.rt.tree.children[0] }

func TestUniformListBuildsVisibleRows(t *testing.T) {
	r := newOverlayRig(listRoot(constCount(100_000), listRow, nil), 800, 600)
	r.find(t, "row-0")
	r.find(t, "row-9")
	if _, ok := r.rt.Find("row-500"); ok {
		t.Error("row-500 is built although it is far below the viewport")
	}
	if n := len(r.rt.byID); n > 300 {
		t.Errorf("%d nodes built for a 100,000-row list, want fewer than 300", n)
	}
}

func TestUniformListScrollExtent(t *testing.T) {
	r := newOverlayRig(listRoot(constCount(100_000), listRow, nil), 800, 600)
	c := listContainer(r)
	if want := float32(100_000 * 40); c.lay.ContentH != want {
		t.Errorf("content height %v, want %v", c.lay.ContentH, want)
	}
	if len(r.rt.scrollbars(c, nil)) == 0 {
		t.Error("no scrollbar on a list taller than its viewport")
	}
}

func TestUniformListScrollBuildsRowsInSameFrame(t *testing.T) {
	r := newOverlayRig(listRoot(constCount(100_000), listRow, nil), 800, 600)
	r.rt.Scroll(100, 100, 0, 20_000, true, 0)
	r.frame()
	list := r.find(t, "list")
	row := r.find(t, "row-500")
	if row.Y < list.Y || row.Y+row.H > list.Y+list.H {
		t.Errorf("row-500 %+v is not inside the list %+v", row, list)
	}
	if _, ok := r.rt.Find("row-0"); ok {
		t.Error("row-0 is still built after scrolling 20,000px down")
	}
}

func TestUniformListMeasuresRows(t *testing.T) {
	row := func(i int) Element {
		return Div().Key(fmt.Sprintf("m-%d", i)).Padding(9).Child(Text("Measured"))
	}
	r := newOverlayRig(listRoot(constCount(1000), row, nil), 800, 600)
	list := r.find(t, "list")
	rowH := r.find(t, "m-0").H
	if rowH <= 18 {
		t.Fatalf("measured row height %v, want the padded text height", rowH)
	}
	for _, i := range []int{1, 2, 5} {
		got := r.find(t, fmt.Sprintf("m-%d", i)).Y
		if want := list.Y + float32(i)*rowH; math.Abs(float64(got-want)) > 0.5 {
			t.Errorf("row %d at Y %v, want %v", i, got, want)
		}
	}
}

func TestUniformListHeightChangeKeepsTopRow(t *testing.T) {
	pad := NewSignal(float32(9))
	row := func(i int) Element {
		return Div().Key(fmt.Sprintf("h-%d", i)).Padding(pad.Get()).Child(Text("Changing"))
	}
	r := newOverlayRig(listRoot(constCount(1000), row, nil), 800, 600)
	oldH := r.find(t, "h-0").H
	r.rt.Scroll(100, 100, 0, 300*oldH, true, 0)
	r.frame()
	pad.Set(20)
	r.frame()
	list := r.find(t, "list")
	rowH := r.find(t, "h-301").Y - r.find(t, "h-300").Y
	if want := oldH + 22; math.Abs(float64(rowH-want)) > 0.5 {
		t.Fatalf("row pitch %v after the padding change, want %v", rowH, want)
	}
	top := r.find(t, "h-300")
	if d := top.Y - list.Y; d < -rowH || d > rowH {
		t.Errorf("row 300 at %v below the top of the list %v; want the first visible row", d, list.Y)
	}
}

func TestUniformListIdentityAcrossScroll(t *testing.T) {
	renders := map[int]int{}
	locals := map[int]*Signal[int]{}
	row := func(i int) Element {
		return Component(func(ctx *Context) Element {
			renders[i]++
			n := Local(ctx, "n", i+100)
			locals[i] = n
			return Div().Key(fmt.Sprintf("c-%d", i)).Height(Px(40)).Child(Text(fmt.Sprint(n.Get())))
		}).Key(fmt.Sprintf("c-%d", i))
	}
	r := newOverlayRig(listRoot(constCount(1000), row, nil), 800, 600)
	locals[3].Set(777)
	r.frame()
	before := map[int]int{3: renders[3], 5: renders[5]}
	r.rt.Scroll(100, 100, 0, 80, true, 0)
	r.frame()
	for _, i := range []int{3, 5} {
		if renders[i] != before[i] {
			t.Errorf("row %d rendered %d times, want %d after scrolling", i, renders[i], before[i])
		}
	}
	if v := locals[3].Peek(); v != 777 {
		t.Errorf("row 3 local is %d after scrolling, want 777", v)
	}

	// Scroll far enough that the built window moves (row 0 drops out) while
	// row 3 stays built: its instance, and so its local, is kept.
	r.rt.Scroll(100, 100, 0, 160, true, 0)
	r.frame()
	if _, ok := r.rt.Find("c-0"); ok {
		t.Fatal("row 0 is still built; the window did not move")
	}
	if v := locals[3].Peek(); v != 777 {
		t.Errorf("row 3 local is %d after the window moved, want 777", v)
	}
}

func TestUniformListScrollToItem(t *testing.T) {
	h := NewListHandle()
	r := newOverlayRig(listRoot(constCount(100_000), listRow, h), 800, 600)
	h.ScrollToItem(5000)
	for i := 0; i < 120; i++ {
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
	}
	list := r.find(t, "list")
	row := r.find(t, "row-5000")
	if row.Y < list.Y || row.Y+row.H > list.Y+list.H {
		t.Fatalf("row-5000 %+v is not fully inside the list %+v", row, list)
	}
	target := listContainer(r).st.targetY
	h.ScrollToItem(4998)
	r.frame()
	if got := listContainer(r).st.targetY; got != target {
		t.Errorf("scroll target moved from %v to %v for a row already in view", target, got)
	}
}

func TestUniformListCountShrink(t *testing.T) {
	count := NewSignal(100_000)
	r := newOverlayRig(listRoot(count.Get, listRow, nil), 800, 600)
	r.rt.Scroll(100, 100, 0, 1e7, true, 0)
	r.frame()
	count.Set(10)
	r.frame()
	for i := range 10 {
		r.find(t, fmt.Sprintf("row-%d", i))
	}
	if _, ok := r.rt.Find("row-10"); ok {
		t.Error("row-10 is built after the list shrank to 10 rows")
	}
	if y := listContainer(r).st.scrollY; y != 0 {
		t.Errorf("scroll offset %v after shrinking to a single screen, want 0", y)
	}
	count.Set(0)
	r.frame()
	if _, ok := r.rt.Find("row-0"); ok {
		t.Error("row-0 is built in an empty list")
	}
}

func TestUniformListClick(t *testing.T) {
	clicks := 0
	row := func(i int) Element {
		return Div().Key(fmt.Sprintf("row-%d", i)).Height(Px(40)).OnClick(func() { clicks++ }).Child(Text("Click"))
	}
	r := newOverlayRig(listRoot(constCount(1000), row, nil), 800, 600)
	if err := r.rt.ClickKey("row-3"); err != nil || clicks != 1 {
		t.Errorf("row-3 clicked %d times, err %v; want 1", clicks, err)
	}
}

// nodesBuiltPerFrame settles frames after ScrollToItem(i) and returns the
// largest number of nodes the tree held after any one frame.
func maxNodesDuringScroll(r *overlayRig, h *ListHandle, i, frames int) int {
	h.ScrollToItem(i)
	most := 0
	for range frames {
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
		most = max(most, len(r.rt.byID))
	}
	return most
}

func TestUniformListFarScrollBuildsFewRows(t *testing.T) {
	h := NewListHandle()
	r := newOverlayRig(listRoot(constCount(100_000), listRow, h), 800, 600)
	if most := maxNodesDuringScroll(r, h, 5000, 120); most >= 300 {
		t.Fatalf("%d nodes in one frame of a far ScrollToItem, want fewer than 300", most)
	}
	list := r.find(t, "list")
	row := r.find(t, "row-5000")
	if row.Y < list.Y || row.Y+row.H > list.Y+list.H {
		t.Fatalf("row-5000 %+v is not fully inside the list %+v", row, list)
	}
}

func TestUniformListNearJumpAnimates(t *testing.T) {
	h := NewListHandle()
	r := newOverlayRig(listRoot(constCount(100_000), listRow, h), 800, 600)
	h.ScrollToItem(12) // 120 px below the top: nearer than a viewport
	for range 2 {      // the target is set by one paint and eased by the next
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
	}
	st := listContainer(r).st
	if st.targetY <= 0 || st.scrollY <= 0 || st.scrollY >= st.targetY {
		t.Fatalf("after one frame the offset is %v toward a target of %v; want it between 0 and the target", st.scrollY, st.targetY)
	}
}

// listShape places a list in a tree. list builds the list from the current
// count; the shape returns the root of the app.
type listShape struct {
	name string
	root func(list func(*Context) Element) func(*Context) Element
}

var listShapes = []listShape{
	{"uniform list under a parent", func(list func(*Context) Element) func(*Context) Element {
		return func(*Context) Element { return Div().Child(Component(list)) }
	}},
	{"uniform list as the root", func(list func(*Context) Element) func(*Context) Element {
		return list
	}},
	{"uniform list under If", func(list func(*Context) Element) func(*Context) Element {
		return func(*Context) Element { return Div().Child(If(true, Component(list))) }
	}},
	{"uniform list under Map", func(list func(*Context) Element) func(*Context) Element {
		return func(*Context) Element {
			return Div().Children(Map([]int{0}, func(int, int) Element { return Component(list) })...)
		}
	}},
}

// TestListRowCountChangesUnderParent rebuilds a list whose row count changes
// while the list sits in several places in the tree. The count goes up, to
// zero, and back, and the rows follow each change.
func TestListRowCountChangesUnderParent(t *testing.T) {
	for _, s := range listShapes {
		t.Run(s.name, func(t *testing.T) {
			count := NewSignal(20)
			list := func(ctx *Context) Element {
				return UniformList(count.Get(), listRow).Key("list").Style(Style{Height: Px(400)})
			}
			r := newOverlayRig(s.root(list), 800, 600)
			for _, n := range []int{100, 0, 3, 0, 40} {
				count.Set(n)
				r.frame()
				checkListRows(t, r, n)
			}
		})
	}
}

// TestListOfRowCountChangesUnderParent is the ListOf equivalent of the test
// above: a ListOf returned directly from a component under a parent.
func TestListOfRowCountChangesUnderParent(t *testing.T) {
	count := NewSignal(20)
	items := func() []int {
		out := make([]int, count.Get())
		for i := range out {
			out[i] = i
		}
		return out
	}
	key := func(i int) string { return fmt.Sprint(i) }
	root := func(*Context) Element {
		return Div().Child(Component(func(*Context) Element {
			return ListOf(items(), key, listRowOf).Key("list").Style(Style{Height: Px(400)})
		}))
	}
	r := newOverlayRig(root, 800, 600)
	// A short list that grows must keep its first rows: 3 to 100 is growth
	// from a list that fits its viewport.
	for _, n := range []int{100, 0, 40, 3, 100} {
		count.Set(n)
		r.frame()
		checkListRows(t, r, n)
	}
}

// checkListRows checks that a list holds n rows: row 0 is built when n is
// positive, the rows past n are not, and the scroll extent is n rows tall.
func checkListRows(t *testing.T, r *overlayRig, n int) {
	t.Helper()
	if n == 0 {
		if _, ok := r.rt.Find("row-0"); ok {
			t.Error("row-0 is built with no rows")
		}
	} else {
		r.find(t, "row-0")
		if n <= 3 {
			r.find(t, fmt.Sprintf("row-%d", n-1))
		}
	}
	if _, ok := r.rt.Find(fmt.Sprintf("row-%d", n)); ok {
		t.Errorf("row-%d is built in a list of %d rows", n, n)
	}
	// The rows sit in one spacer, sized for every row of the list.
	if got, want := r.rt.findKey(r.rt.tree, "list").children[0].lay.H, float32(n*40); got != want {
		t.Errorf("spacer height %v, want %v", got, want)
	}
}

// listRowOf is listRow for an item of a ListOf, whose rows are keyed by the
// ListOf's key function instead.
func listRowOf(i int) Element { return listRow(i) }

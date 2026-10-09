package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// label is a payload type that only matches an interface.
type label string

func (l label) String() string { return string(l) }

// center returns the middle of the element with key, in device-logical pixels.
func center(t *testing.T, rt *Runtime, key string) (float32, float32) {
	t.Helper()
	r, ok := rt.Find(key)
	if !ok {
		t.Fatalf("no element with key %q", key)
	}
	return r.X + r.W/2, r.Y + r.H/2
}

// dragBetween presses on the element from, drags past the threshold, moves to
// the element to and releases there. Each step runs a frame, as the app does.
func dragBetween(t *testing.T, rt *Runtime, from, to string) {
	t.Helper()
	x0, y0 := center(t, rt, from)
	x1, y1 := center(t, rt, to)
	rt.PointerMove(x0, y0, 0)
	frame(rt)
	rt.PointerDown(x0, y0, input.ButtonLeft, 1, 0)
	rt.PointerMove(x0, y0+dragThreshold, 0)
	frame(rt)
	rt.PointerMove(x1, y1, 0)
	frame(rt)
	rt.PointerUp(x1, y1, input.ButtonLeft, 0)
	frame(rt)
}

func TestDragThresholdDecidesClickOrDrag(t *testing.T) {
	clicks, ends := 0, 0
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(20).Child(Div().Key("src").Size(Px(100), Px(40)).
			OnClick(func() { clicks++ }).
			OnDrag(Drag(42).End(func(bool) { ends++ })))
	}, 400, 300, 1)
	x, y := center(t, rt, "src")

	// Moving 2 px and releasing is a click.
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerMove(x+2, y, 0)
	rt.PointerUp(x+2, y, input.ButtonLeft, 0)
	frame(rt)
	if clicks != 1 || ends != 0 {
		t.Fatalf("a 2 px move: clicks=%d ends=%d; want a click and no drag", clicks, ends)
	}

	// Moving 4 px starts a drag, and its release is not a click.
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerMove(x+4, y, 0)
	frame(rt)
	rt.PointerUp(x+4, y, input.ButtonLeft, 0)
	frame(rt)
	if clicks != 1 || ends != 1 {
		t.Errorf("a 4 px move: clicks=%d ends=%d; want the drag to end without a click", clicks, ends)
	}
}

func TestDropMatchesPayloadByType(t *testing.T) {
	var got []string
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("int-src").Size(Px(100), Px(30)).OnDrag(Drag(42))).
			Child(Div().Key("label-src").Size(Px(100), Px(30)).OnDrag(Drag(label("hi")))).
			Child(Div().Key("int").Size(Px(100), Px(30)).
				OnDrop(Drop(func(v int, e *DropEvent) { got = append(got, "int") }))).
			Child(Div().Key("string").Size(Px(100), Px(30)).
				OnDrop(Drop(func(v string, e *DropEvent) { got = append(got, "string") }))).
			Child(Div().Key("stringer").Size(Px(100), Px(30)).
				OnDrop(Drop(func(v fmt.Stringer, e *DropEvent) { got = append(got, "stringer:"+v.String()) }))).
			Child(Div().Key("refuses").Size(Px(100), Px(30)).
				OnDrop(Drop(func(v int, e *DropEvent) { got = append(got, "refused") }).
					Accept(func(int) bool { return false })))
	}, 300, 400, 1)

	dragBetween(t, rt, "int-src", "int")
	dragBetween(t, rt, "int-src", "string")
	dragBetween(t, rt, "int-src", "refuses")
	dragBetween(t, rt, "label-src", "stringer")
	dragBetween(t, rt, "label-src", "string")
	dragBetween(t, rt, "label-src", "int")
	if want := []string{"int", "stringer:hi"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("drops = %v; want %v", got, want)
	}
}

func TestDragNilPayloadPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Drag of a nil interface did not panic")
		}
	}()
	Drag[any](nil)
}

func TestOverAndDraggingStylesApplyOnlyDuringTheDrag(t *testing.T) {
	red := Hex("#f00")
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("src").Size(Px(100), Px(30)).Background(Hex("#111")).
				Dragging(Style{Opacity: 0.5}).OnDrag(Drag(1))).
			Child(Div().Key("target").Size(Px(100), Px(30)).Background(Hex("#222")).
				OnDrop(Drop(func(int, *DropEvent) {}).Over(Style{Background: red})))
	}, 300, 200, 1)
	src, target := rt.findKey(rt.tree, "src"), rt.findKey(rt.tree, "target")
	sx, sy := center(t, rt, "src")
	tx, ty := center(t, rt, "target")

	rt.PointerMove(sx, sy, 0)
	rt.PointerDown(sx, sy, input.ButtonLeft, 1, 0)
	rt.PointerMove(sx, sy+dragThreshold, 0)
	frame(rt)
	if src.style.Opacity != 0.5 {
		t.Errorf("source opacity during the drag = %v; want Dragging's 0.5", src.style.Opacity)
	}
	rt.PointerMove(tx, ty, 0)
	frame(rt)
	if target.style.Background != red {
		t.Errorf("target background over it = %v; want its Over style", target.style.Background)
	}
	rt.PointerMove(250, 190, 0) // off every target
	frame(rt)
	if target.style.Background == red {
		t.Error("target keeps its Over style after the drag left it")
	}
	rt.PointerMove(tx, ty, 0)
	frame(rt)
	rt.PointerUp(tx, ty, input.ButtonLeft, 0)
	frame(rt)
	if target.style.Background == red {
		t.Error("target keeps its Over style after the drop")
	}
	if src.style.Opacity != 0 {
		t.Errorf("source opacity after the drop = %v; want its normal 0", src.style.Opacity)
	}
}

func TestMoveLeaveDropAndEnd(t *testing.T) {
	var log []string
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("src").Size(Px(100), Px(30)).OnDrag(Drag(7).
				End(func(dropped bool) { log = append(log, fmt.Sprintf("end:%v", dropped)) }))).
			Child(Div().Key("target").Size(Px(100), Px(60)).
				OnDrop(Drop(func(v int, e *DropEvent) {
					log = append(log, fmt.Sprintf("drop:%d:%.0f,%.0f", v, e.LocalX, e.LocalY))
				}).
					Move(func(v int, e *DropEvent) { log = append(log, fmt.Sprintf("move:%.0f,%.0f", e.LocalX, e.LocalY)) }).
					Leave(func() { log = append(log, "leave") })))
	}, 300, 300, 1)
	tr, _ := rt.Find("target")
	inX, inY := tr.X+40, tr.Y+20 // local (40, 20)

	sx, sy := center(t, rt, "src")
	rt.PointerMove(sx, sy, 0)
	rt.PointerDown(sx, sy, input.ButtonLeft, 1, 0)
	rt.PointerMove(sx, sy+dragThreshold, 0)
	frame(rt)
	rt.PointerMove(inX, inY, 0)
	frame(rt)
	rt.PointerMove(250, 280, 0) // leaves the target
	frame(rt)
	rt.PointerMove(inX, inY, 0)
	frame(rt)
	rt.PointerUp(inX, inY, input.ButtonLeft, 0)
	frame(rt)

	want := []string{"move:40,20", "leave", "move:40,20", "drop:7:40,20", "leave", "end:true"}
	if fmt.Sprint(log) != fmt.Sprint(want) {
		t.Errorf("callbacks = %v\nwant %v", log, want)
	}
}

// startDragFrom presses on src and drags past the threshold, leaving the drag
// active with the pointer over src.
func startDragFrom(t *testing.T, rt *Runtime, key string) (float32, float32) {
	t.Helper()
	x, y := center(t, rt, key)
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerMove(x, y+dragThreshold, 0)
	frame(rt)
	return x, y + dragThreshold
}

func TestDragEndsWithoutADropOnCancelAndMisses(t *testing.T) {
	cases := []struct {
		name   string
		cancel func(rt *Runtime, x, y float32)
	}{
		{"escape", func(rt *Runtime, x, y float32) { rt.KeyDown(input.KeyEscape, 0, false); rt.KeyUp(input.KeyEscape, 0) }},
		{"window blur", func(rt *Runtime, x, y float32) { rt.WindowFocus(false) }},
		{"release over nothing", func(rt *Runtime, x, y float32) { rt.PointerUp(250, 280, input.ButtonLeft, 0) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var log []string
			rt := newTestRuntime(func(ctx *Context) Element {
				return Div().Padding(10).Gap(10).
					Child(Div().Key("src").Size(Px(100), Px(30)).OnDrag(Drag(7).
						End(func(dropped bool) { log = append(log, fmt.Sprintf("end:%v", dropped)) }))).
					Child(Div().Key("target").Size(Px(100), Px(60)).
						OnDrop(Drop(func(int, *DropEvent) { log = append(log, "drop") })))
			}, 300, 300, 1)
			x, y := startDragFrom(t, rt, "src")
			tr, _ := rt.Find("target")
			rt.PointerMove(tr.X+10, tr.Y+10, 0)
			frame(rt)
			c.cancel(rt, x, y)
			frame(rt)
			if fmt.Sprint(log) != "[end:false]" {
				t.Errorf("callbacks = %v; want [end:false] with no drop", log)
			}
		})
	}
}

func TestNestedTargetsDeepestAcceptingWins(t *testing.T) {
	var got string
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("src").Size(Px(100), Px(30)).OnDrag(Drag(1))).
			Child(Div().Key("outer").Padding(10).OnDrop(Drop(func(int, *DropEvent) { got = "outer" })).
				Child(Div().Key("inner").Size(Px(200), Px(100)).
					OnDrop(Drop(func(string, *DropEvent) { got = "inner" })).
					Child(Div().Key("deep").Size(Px(60), Px(40)).
						OnDrop(Drop(func(int, *DropEvent) { got = "deep" })))))
	}, 300, 300, 1)

	dragBetween(t, rt, "src", "deep")
	if got != "deep" {
		t.Errorf("drop on the deepest accepting target: got %q", got)
	}
	got = ""
	dragBetween(t, rt, "src", "inner")
	if got != "outer" {
		t.Errorf("drop on a deeper target that refuses the payload: got %q; want the outer target", got)
	}
}

func TestPreviewIsPaintedAtThePointerAndNotHit(t *testing.T) {
	text := NewSignal("first")
	dropped := false
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("src").Size(Px(100), Px(30)).OnDrag(Drag(1).Preview(func() Element {
				return Div().Key("ghost").Size(Px(60), Px(20)).Background(Hex("#0f0")).Child(Text(text.Get()))
			}))).
			Child(Div().Key("target").Size(Px(200), Px(120)).
				OnDrop(Drop(func(int, *DropEvent) { dropped = true })))
	}, 400, 300, 1)
	sx, sy := center(t, rt, "src")
	rt.PointerMove(sx, sy, 0)
	rt.PointerDown(sx, sy, input.ButtonLeft, 1, 0)
	rt.PointerMove(sx, sy+dragThreshold, 0)
	frame(rt)

	// The preview (60x20) is centered on the pointer, which is at the source's
	// center (60, 25) moved down by the threshold: (60, 29).
	ghostAt := func(x, y float32) bool {
		for _, q := range rt.scene.Quads {
			if q.Bounds.X == x && q.Bounds.Y == y && q.Bounds.W == 60 && q.Bounds.H == 20 && q.Background == Hex("#0f0").scene() {
				return true
			}
		}
		return false
	}
	if !ghostAt(30, 19) {
		t.Fatal("the preview is not centered on the pointer when the drag starts: want its quad at (30, 19)")
	}

	// The preview covers the pointer's target, which must still be found.
	rt.PointerMove(100, 100, 0)
	frame(rt)
	if !ghostAt(70, 90) {
		t.Error("the preview is not centered on the pointer: want its quad at (70, 90)")
	}
	if rt.drag.target == 0 {
		t.Fatal("the target under the preview was not found")
	}

	// A signal the preview reads re-renders it.
	text.Set("second")
	frame(rt)
	if findText(rt.drag.preview.root, "second") == nil {
		t.Error("the preview did not re-render when its signal changed")
	}

	// After the drop, nothing of the preview is kept or painted.
	var ids []uint64
	var walk func(n *node)
	walk = func(n *node) {
		ids = append(ids, n.id)
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(rt.drag.preview.root)
	rt.PointerUp(100, 100, input.ButtonLeft, 0)
	frame(rt)
	if !dropped {
		t.Error("the drop did not reach the target under the preview")
	}
	for _, id := range ids {
		if rt.byID[id] != nil {
			t.Errorf("preview node %x is still known after the drag", id)
		}
	}
	if _, ok := rt.instances[dragPreviewID]; ok || rt.drag.preview != nil {
		t.Error("the preview's component outlived the drag")
	}
	if ghostAt(50, 81) {
		t.Error("the preview is still painted after the drag")
	}
}

func TestHoverStylesPauseDuringDrag(t *testing.T) {
	red := Hex("#f00")
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("src").Size(Px(100), Px(30)).OnDrag(Drag(1))).
			Child(Div().Key("hover").Size(Px(100), Px(30)).Hover(Style{Background: red}))
	}, 300, 300, 1)
	hover := rt.findKey(rt.tree, "hover")
	hx, hy := center(t, rt, "hover")
	rt.PointerMove(hx, hy, 0)
	frame(rt)
	if hover.style.Background != red {
		t.Fatal("hover style not applied before the drag")
	}
	sx, sy := center(t, rt, "src")
	rt.PointerMove(sx, sy, 0)
	rt.PointerDown(sx, sy, input.ButtonLeft, 1, 0)
	rt.PointerMove(sx, sy+dragThreshold, 0)
	frame(rt)
	rt.PointerMove(hx, hy, 0)
	frame(rt)
	if hover.style.Background == red {
		t.Error("a hover style applied during a drag")
	}
	rt.PointerUp(hx, hy, input.ButtonLeft, 0)
	frame(rt)
	if hover.style.Background != red {
		t.Error("hover did not resume after the drag ended over the element")
	}
}

func TestDropDeliversPayloadAfterSourceIsRemoved(t *testing.T) {
	show := NewSignal(true)
	got := 0
	rt := newTestRuntime(func(ctx *Context) Element {
		d := Div().Padding(10).Gap(10)
		if show.Get() {
			d.Child(Div().Key("src").Size(Px(100), Px(30)).OnDrag(Drag(42)))
		}
		return d.Child(Div().Key("target").Size(Px(100), Px(30)).
			OnDrop(Drop(func(v int, e *DropEvent) { got = v })))
	}, 300, 300, 1)
	sx, sy := center(t, rt, "src")
	tx, ty := center(t, rt, "target")
	rt.PointerMove(sx, sy, 0)
	rt.PointerDown(sx, sy, input.ButtonLeft, 1, 0)
	rt.PointerMove(sx, sy+dragThreshold, 0)
	frame(rt)
	show.Set(false)
	frame(rt)
	if rt.findKey(rt.tree, "src") != nil {
		t.Fatal("the source is still in the tree")
	}
	tx, ty = center(t, rt, "target") // the target moved up into the source's place
	rt.PointerMove(tx, ty, 0)
	frame(rt)
	rt.PointerUp(tx, ty, input.ButtonLeft, 0)
	frame(rt)
	if got != 42 {
		t.Errorf("drop after the source was removed delivered %d; want 42", got)
	}
}

// scrollRig is a window with a drag source above a 200 px list of 40 rows,
// each 50 px high. The list's bottom edge is at y = 240. Row target, if not
// negative, accepts drops and counts them in dropped.
func scrollRig(t *testing.T, target int, dropped *int) *Runtime {
	t.Helper()
	return newTestRuntime(func(ctx *Context) Element {
		list := Div().Key("list").Height(Px(200)).ScrollY()
		for i := 0; i < 40; i++ {
			row := Div().Keyf("row%d", i).Height(Px(50)).Shrink(0)
			if i == target {
				row.OnDrop(Drop(func(int, *DropEvent) { *dropped++ }))
			}
			list.Child(row)
		}
		return Div().Padding(10).Gap(10).
			Child(Div().Key("src").Size(Px(100), Px(30)).OnDrag(Drag(1))).
			Child(list)
	}, 300, 300, 1)
}

// dragNear starts a drag from src and moves the pointer to (x, y).
func dragNear(t *testing.T, rt *Runtime, x, y float32) {
	t.Helper()
	sx, sy := center(t, rt, "src")
	rt.PointerMove(sx, sy, 0)
	rt.PointerDown(sx, sy, input.ButtonLeft, 1, 0)
	rt.PointerMove(sx, sy+dragThreshold, 0)
	frame(rt)
	rt.PointerMove(x, y, 0)
	frame(rt)
}

func TestAutoscrollNearEdgeAndIdleAfter(t *testing.T) {
	rt := scrollRig(t, -1, new(int))
	list := rt.findKey(rt.tree, "list")
	now := time.Now()
	step := func() {
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
	dragNear(t, rt, 50, 230) // 10 px above the bottom edge

	for i := 0; i < 30; i++ {
		step()
	}
	if list.st.targetY <= 0 || list.st.scrollY != list.st.targetY {
		t.Fatalf("the list did not scroll toward the bottom edge: targetY=%v scrollY=%v", list.st.targetY, list.st.scrollY)
	}
	if !rt.NeedsFrame() {
		t.Error("a scrolling drag does not ask for frames")
	}

	// Out of the zone, the scroll stops and the runtime goes idle.
	rt.PointerMove(50, 120, 0)
	step()
	stopped := list.st.scrollY
	for i := 0; i < 10; i++ {
		step()
	}
	if list.st.scrollY != stopped {
		t.Errorf("the list kept scrolling after the pointer left the zone: %v -> %v", stopped, list.st.scrollY)
	}
	if rt.NeedsFrame() {
		t.Error("an idle drag keeps asking for frames")
	}

	rt.PointerUp(50, 120, input.ButtonLeft, 0)
	step()
	step()
	if rt.NeedsFrame() {
		t.Error("the runtime is not idle after the drag ended")
	}
}

func TestAutoscrollBringsTargetIntoViewAndHitsIt(t *testing.T) {
	dropped := 0
	rt := scrollRig(t, 12, &dropped) // row 12 starts far below the list
	if paintedKey(t, rt, "row12") {
		t.Fatal("row 12 is painted before any scrolling")
	}
	now := time.Now()
	dragNear(t, rt, 50, 230)
	for i := 0; i < 300 && !paintedKey(t, rt, "row12"); i++ {
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
	if !paintedKey(t, rt, "row12") {
		t.Fatal("autoscroll never brought row 12 into view")
	}
	// Wait until the row is well inside the list, away from the edge zone, and
	// aim at it.
	var x, y float32
	for i := 0; i < 300; i++ {
		x, y = center(t, rt, "row12")
		if y > 80 && y < 200 {
			break
		}
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
	rt.PointerMove(x, y, 0)
	frame(rt)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	frame(rt)
	if dropped != 1 {
		t.Errorf("drop on the scrolled-in target delivered %d drops; want 1", dropped)
	}
}

func TestDropFilesGoToFilesTargetsOnly(t *testing.T) {
	var gotOuter, gotInner Files
	ints := 0
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("ints").Size(Px(100), Px(30)).OnDrop(Drop(func(int, *DropEvent) { ints++ }))).
			Child(Div().Key("outer").Padding(10).Size(Px(200), Px(120)).
				OnDrop(Drop(func(f Files, e *DropEvent) { gotOuter = f })).
				Child(Div().Key("inner").Size(Px(100), Px(60)).
					OnDrop(Drop(func(f Files, e *DropEvent) { gotInner = f }))))
	}, 300, 300, 1)
	ix, iy := center(t, rt, "ints")
	rt.DropFiles(ix, iy, []string{"a.md"})
	if ints != 0 || gotOuter != nil {
		t.Errorf("files dropped on an int target: ints=%d outer=%v", ints, gotOuter)
	}

	or, _ := rt.Find("outer")
	rt.DropFiles(or.X+5, or.Y+5, []string{"a.md", "b.txt"}) // in outer's padding, outside inner
	if fmt.Sprint(gotOuter) != "[a.md b.txt]" {
		t.Errorf("outer target got %v; want the two paths", gotOuter)
	}

	nx, ny := center(t, rt, "inner")
	rt.DropFiles(nx, ny, []string{"c.md"})
	if fmt.Sprint(gotInner) != "[c.md]" || fmt.Sprint(gotOuter) != "[a.md b.txt]" {
		t.Errorf("nested: inner=%v outer=%v; want the deepest Files target to win", gotInner, gotOuter)
	}
}

// A DropEvent carries the target's size, so a row can tell which half of it
// the pointer is over (insert before or after) without knowing its height.
func TestDropEventHasTargetSize(t *testing.T) {
	var e DropEvent
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).
			Child(Div().Key("src").Size(Px(100), Px(30)).OnDrag(Drag(1))).
			Child(Div().Key("row").Size(Px(180), Px(64)).
				OnDrop(Drop(func(v int, ev *DropEvent) { e = *ev })))
	}, 300, 400, 1)
	dragBetween(t, rt, "src", "row")
	if e.W != 180 || e.H != 64 {
		t.Errorf("DropEvent size %vx%v, want the target's 180x64", e.W, e.H)
	}
	if e.LocalY < e.H/2-1 || e.LocalY > e.H/2+1 {
		t.Errorf("LocalY %v, want about half the height (the drag ends at the target's center)", e.LocalY)
	}
}

// A small preview of a wide source stays under the pointer: dragging a card
// from its middle must not put the chip far up and to the left.
func TestSmallPreviewOfWideSourceStaysUnderPointer(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).
			Child(Div().Key("card").Size(Px(900), Px(100)).OnDrag(Drag(1).Preview(func() Element {
				return Div().Size(Px(120), Px(32)).Background(Hex("#f0f"))
			})))
	}, 1000, 400, 1)
	x, y := center(t, rt, "card") // (460, 60)
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerMove(x+dragThreshold, y, 0)
	frame(rt)
	px, py := x+dragThreshold, y
	for _, q := range rt.scene.Quads {
		if q.Background == Hex("#f0f").scene() {
			if !(q.Bounds.X <= px && px <= q.Bounds.X+q.Bounds.W && q.Bounds.Y <= py && py <= q.Bounds.Y+q.Bounds.H) {
				t.Errorf("preview at %+v does not contain the pointer (%v, %v)", q.Bounds, px, py)
			}
			if cx, cy := q.Bounds.X+q.Bounds.W/2, q.Bounds.Y+q.Bounds.H/2; cx != px || cy != py {
				t.Errorf("preview centered at (%v, %v), want the pointer (%v, %v)", cx, cy, px, py)
			}
			return
		}
	}
	t.Fatal("no preview painted")
}

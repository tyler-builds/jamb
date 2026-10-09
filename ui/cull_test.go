package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// paintedKey reports whether the element with key was painted by the latest
// paint.
func paintedKey(t *testing.T, rt *Runtime, key string) bool {
	t.Helper()
	n := rt.findKey(rt.tree, key)
	if n == nil {
		t.Fatalf("no element with key %q", key)
	}
	return n.paintGen == rt.paintGen
}

// rowsColumn is a scrolling column of n rows, each 50 px high, keyed row<i>.
// Rows call clicked with their index when clicked.
func rowsColumn(n int, clicked func(int)) func(*Context) Element {
	return func(ctx *Context) Element {
		list := Div().Key("list").Height(Full).ScrollY()
		for i := 0; i < n; i++ {
			list.Child(Div().Keyf("row%d", i).Height(Px(50)).Shrink(0).OnClick(func() { clicked(i) }))
		}
		return list
	}
}

func TestCullingPaintsOnlyVisibleRows(t *testing.T) {
	rt := newTestRuntime(rowsColumn(2000, func(int) {}), 500, 400, 1)
	if got := rt.Stats().NodesPainted; got >= 200 {
		t.Fatalf("first paint drew %d nodes of 2000 rows; culling should keep it small", got)
	}
	if paintedKey(t, rt, "row1500") {
		t.Error("row 1500 is far below the viewport but was painted")
	}
	// Scroll 10,000 px: rows 200 to 207 are now in view.
	rt.Scroll(100, 100, 0, 10000, true, 0)
	frame(rt)
	if got := rt.Stats().NodesPainted; got >= 200 {
		t.Errorf("after scrolling, painted %d nodes; want fewer than 200", got)
	}
	for _, i := range []int{200, 205, 207} {
		if !paintedKey(t, rt, fmt.Sprintf("row%d", i)) {
			t.Errorf("row %d is in view after scrolling but was not painted", i)
		}
	}
	if paintedKey(t, rt, "row0") {
		t.Error("row 0 scrolled out of view but is still painted")
	}
}

func TestCullingKeepsAbsoluteChildThatReachesIntoView(t *testing.T) {
	// The parent sits below the 400 px viewport, but its badge is lifted by
	// Top(-300) into view. The parent's ink must include the badge.
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().
			Child(Div().Height(Px(500)).Key("filler")).
			Child(Div().Key("parent").Height(Px(100)).
				Child(Div().Key("badge").Absolute().Top(-300).Height(Px(50)).Background(Hex("#f00"))))
	}, 500, 400, 1)
	if !paintedKey(t, rt, "parent") {
		t.Fatal("the parent, whose absolute badge reaches into view, was culled")
	}
	if !paintedKey(t, rt, "badge") {
		t.Error("the badge is in view but was not painted")
	}
}

func TestCullingKeepsShadowThatReachesIntoView(t *testing.T) {
	card := func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().
			Child(Div().Height(Px(420))).
			Child(Div().Key("card").Height(Px(100)).Background(Hex("#222")).Shadow(ShadowLg))
	}
	rt := newTestRuntime(card, 500, 400, 1)
	// The card starts 20 px below the viewport. Its shadow (Y 12, blur 32)
	// reaches back up into it.
	if !paintedKey(t, rt, "card") {
		t.Fatal("a card whose shadow reaches into view was culled")
	}
	// The shadow's own rectangle starts at 420 + 12 = 432, below the view; its
	// blur reaches back up.
	found := false
	for _, q := range rt.scene.Quads {
		if q.Blur > 0 && q.Bounds.Y == 432 {
			found = true
		}
	}
	if !found {
		t.Error("the scene has no shadow quad for the off-screen card")
	}
}

func TestCullingPaintsElementMovedIntoViewByOffset(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().
			Child(Div().Height(Px(500))).
			Child(Div().Key("moved").Height(Px(100)).Offset(0, -300).Background(Hex("#222")))
	}, 500, 400, 1)
	if !paintedKey(t, rt, "moved") {
		t.Fatal("an element that its offset moves into view was culled")
	}
	if r := rt.nodeRect(rt.findKey(rt.tree, "moved")); r.Y != 200 {
		t.Errorf("offset element at y=%v, want 200", r.Y)
	}
}

func TestCullingPaintsEntranceSlideIntoView(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().
			Child(Div().Height(Px(500))).
			Child(Div().Key("entering").Height(Px(100)).Background(Hex("#222")).
				Enter(Fade().Slide(0, -300), Animation{Duration: time.Second}))
	}, 500, 400, 1)
	if !paintedKey(t, rt, "entering") {
		t.Fatal("an element entering from a slide into view was culled")
	}
}

func TestCullingHitTestsOnlyPaintedRows(t *testing.T) {
	var clicked []int
	rt := newTestRuntime(rowsColumn(2000, func(i int) { clicked = append(clicked, i) }), 500, 400, 1)
	rt.Scroll(100, 100, 0, 10000, true, 0)
	frame(rt)
	r, ok := rt.Find("row205")
	if !ok {
		t.Fatal("row 205 not found")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	frame(rt)
	if len(clicked) != 1 || clicked[0] != 205 {
		t.Fatalf("clicking visible row 205 clicked %v, want [205]", clicked)
	}

	// The pointer over the top of the viewport hits row 200 and nothing that
	// was culled.
	rt.PointerMove(100, 5, 0)
	frame(rt)
	path := rt.hitTest(100, 5)
	if len(path) == 0 || path[0].key != "row200" {
		t.Fatalf("hit path at the top of the viewport starts at %v, want row200", path)
	}
	for _, n := range path {
		if n.paintGen != rt.paintGen {
			t.Errorf("hit path contains %q, which was not painted", n.key)
		}
	}
	// A hover-only frame repaints just the visible rows.
	rt.Invalidate()
	frame(rt)
	if got := rt.Stats().NodesPainted; got >= 200 {
		t.Errorf("repaint painted %d nodes; want fewer than 200", got)
	}
}

func TestFindOffScreenRowUsesLayout(t *testing.T) {
	rt := newTestRuntime(rowsColumn(2000, func(int) {}), 500, 400, 1)
	rt.Scroll(100, 100, 0, 10000, true, 0)
	frame(rt)
	// Row i sits at i*50 in the content; the list is scrolled 10,000 px.
	for _, i := range []int{0, 150, 1500} {
		r, ok := rt.Find(fmt.Sprintf("row%d", i))
		want := float32(i*50) - 10000
		if !ok || r.Y != want || r.H != 50 {
			t.Errorf("row %d: Find = %+v (%v), want Y %v H 50", i, r, ok, want)
		}
	}
}

func TestDumpShowsCulledRowsAtLayoutPosition(t *testing.T) {
	rt := newTestRuntime(rowsColumn(400, func(int) {}), 500, 400, 1)
	rt.Scroll(100, 100, 0, 1000, true, 0)
	frame(rt)
	var sb strings.Builder
	rt.Dump(&sb, 2)
	// Row 300 is at 300*50 - 1000 = 14000 in the 500 px wide list; it is culled.
	want := "key=row300  [0,14000 500x50]"
	if !strings.Contains(sb.String(), want) {
		t.Errorf("dump does not show row 300 at its layout position; want %q in:\n%s", want, sb.String())
	}
}

func TestCullingWorksWithExitingRows(t *testing.T) {
	// Exit animations on list rows (animated deletes) must not switch culling
	// off. A row removed while on screen fades out as a ghost; one removed
	// while scrolled out of view was never drawn and just goes.
	show := NewSignal(true)
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div().Key("list").Height(Full).ScrollY()
		for i := 0; i < 400; i++ {
			if (i == 0 || i == 300) && !show.Get() {
				continue
			}
			list.Child(Div().Keyf("row%d", i).Height(Px(50)).Shrink(0).Exit(Fade(), Animation{Duration: time.Second}))
		}
		return list
	}, 500, 400, 1)
	if got := rt.Stats().NodesPainted; got >= 50 {
		t.Fatalf("painted %d nodes of 400 rows with Exit; culling should keep it small", got)
	}
	show.Set(false) // remove row0 (on screen) and row300 (off screen)
	frame(rt)
	if len(rt.ghosts) != 1 || rt.ghosts[0].key != "row0" {
		var keys []string
		for _, g := range rt.ghosts {
			keys = append(keys, g.key)
		}
		t.Errorf("ghosts %v, want only the on-screen row0", keys)
	}
}

func TestCullingKeepsClickOutsideListeners(t *testing.T) {
	// A listener scrolled out of view is culled from paint but still hears
	// clicks outside it.
	outside := 0
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().
			Child(Div().Height(Px(500))).
			Child(Div().Key("menu").Height(Px(60)).OnClickOutside(func() { outside++ }))
	}, 500, 400, 1)
	if paintedKey(t, rt, "menu") {
		t.Fatal("the off-screen listener was painted; this test needs it culled")
	}
	rt.PointerDown(100, 100, input.ButtonLeft, 1, 0)
	rt.PointerUp(100, 100, input.ButtonLeft, 0)
	frame(rt)
	if outside != 1 {
		t.Errorf("click outside fired %d times, want 1", outside)
	}
}
func TestCullingFocusedEditorRevealsWhileScrolledOut(t *testing.T) {
	v := NewSignal("hello")
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().
			Child(TextArea(v).Borderless()).
			Child(Div().Height(Px(2000)))
	})
	r.click(20, 10)
	scroller := r.rt.tree
	r.rt.Scroll(250, 200, 0, 800, true, 0)
	frame(r.rt)
	if scroller.st.scrollY < 700 {
		t.Fatalf("scroll did not move the editor out of view: scrollY %v", scroller.st.scrollY)
	}
	r.typeText("x")
	if scroller.st.scrollY >= 700 {
		t.Errorf("typing left the caret scrolled out of view (scrollY %v)", scroller.st.scrollY)
	}
	caret, _ := r.rt.TextInputRect()
	if caret.Y < 0 || caret.Bottom() > 400 {
		t.Errorf("caret at y=%v..%v is outside the 400px viewport", caret.Y, caret.Bottom())
	}
}

func TestHoverShadowRecomputesInk(t *testing.T) {
	const cards = 100
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div().Height(Full).ScrollY()
		for i := 0; i < cards; i++ {
			c := Div().Keyf("card%d", i).Height(Px(60)).Shrink(0)
			if i == 1 {
				c.Key("hovered").Height(Px(100)).Hover(Style{Shadows: []Shadow{ShadowLg}})
			}
			list.Child(c)
		}
		return list
	}, 500, 400, 1)
	n := rt.findKey(rt.tree, "hovered")
	if n.lay.Overhang != 0 {
		t.Fatalf("overhang before hover = %v, want 0", n.lay.Overhang)
	}
	r, _ := rt.Find("hovered")
	rt.PointerMove(r.X+r.W/2, r.Y+r.H/2, 0)
	frame(rt)
	// ShadowLg: Y 12 plus twice its blur of 32.
	if n.lay.Overhang != 76 {
		t.Errorf("overhang on hover = %v, want 76", n.lay.Overhang)
	}
	if n.lay.InkY1 < 100+76-0.01 {
		t.Errorf("ink bottom %v does not cover the hover shadow", n.lay.InkY1)
	}
	// Only the card and its ancestors were re-laid out, not the whole list.
	if got := rt.Stats().NodesLaidOut; got > 5 {
		t.Errorf("hover re-laid out %d nodes, want the card and its ancestors only", got)
	}
}

func TestCullingNeverCullsScaledChildren(t *testing.T) {
	// "grown" spans x -900..100 and is scaled 1.1 about its center at x -400.
	// Its child sits at x -10..-5, off screen, but the scale carries it to
	// about x 29..34, which is on screen. Nothing under a scaled node is culled.
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Height(Full).
			Child(Div().Key("grown").Absolute().Left(-900).Top(0).Width(Px(1000)).Height(Px(200)).Scale(1.1).
				Child(Div().Key("inner").Absolute().Left(890).Top(0).Width(Px(5)).Height(Px(20))))
	}, 500, 400, 1)
	if !paintedKey(t, rt, "grown") {
		t.Fatal("the scaled node reaches into view but was culled")
	}
	if !paintedKey(t, rt, "inner") {
		t.Error("a child of a scaled node was culled")
	}
}

func TestCullingPaintsOffsetTextIntoView(t *testing.T) {
	// Text and icons take their offsets from their style too.
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Height(Full).ScrollY().
			Child(Div().Height(Px(500))).
			Child(Text("moved").Key("moved").Style(Style{OffsetY: -300}))
	}, 500, 400, 1)
	if !paintedKey(t, rt, "moved") {
		t.Fatal("text that its offset moves into view was culled")
	}
}

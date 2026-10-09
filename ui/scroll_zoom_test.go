package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

func scrollList(rows int) func(*Context) Element {
	return func(ctx *Context) Element {
		list := Div().Key("list").Height(Full).ScrollY()
		for i := 0; i < rows; i++ {
			list.Child(Div().Keyf("row%d", i).Height(Px(50)).Shrink(0).OnClick(func() {}))
		}
		return list
	}
}

func TestScrollbarThumbDrag(t *testing.T) {
	rt := newTestRuntime(scrollList(20), 300, 400, 1) // content 1000, viewport 400
	rt.PointerMove(150, 100, 0)
	frame(rt)
	sb, ok := rt.scrollbarAt(294, 30)
	if !ok || sb.axis != axisY {
		t.Fatal("no vertical scrollbar at the right edge")
	}
	// Grab the thumb and drag it to the bottom of the track.
	tx, ty := sb.thumb.X+sb.thumb.W/2, sb.thumb.Y+sb.thumb.H/2
	rt.PointerDown(tx, ty, input.ButtonLeft, 1, 0)
	rt.PointerMove(tx, ty+1000, 0) // far below: clamps
	frame(rt)
	if st := rt.tree.st; st.scrollY != 600 {
		t.Fatalf("after dragging to the end, scrollY = %v, want 600", st.scrollY)
	}
	rt.PointerMove(tx, ty+(sb.track.H-sb.thumb.H)/2, 0) // back to the middle
	frame(rt)
	if st := rt.tree.st; st.scrollY < 290 || st.scrollY > 310 {
		t.Errorf("thumb in the middle: scrollY = %v, want ~300", st.scrollY)
	}
	rt.PointerUp(tx, 0, input.ButtonLeft, 0)
	frame(rt)
	if rt.sbDrag.active {
		t.Error("drag did not end on release")
	}
}

func TestScrollbarTrackClickPagesAndDoesNotClickContent(t *testing.T) {
	clicked := false
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div().Height(Full).ScrollY()
		for i := 0; i < 20; i++ {
			list.Child(Div().Height(Px(50)).Shrink(0).OnClick(func() { clicked = true }))
		}
		return list
	}, 300, 400, 1)
	rt.PointerMove(150, 100, 0)
	frame(rt)
	rt.PointerDown(294, 380, input.ButtonLeft, 1, 0) // track, below the thumb
	rt.PointerUp(294, 380, input.ButtonLeft, 0)
	for i := 0; i < 60; i++ {
		frame(rt) // let the page animation finish
	}
	if st := rt.tree.st; st.targetY != 360 {
		t.Errorf("track click paged to %v, want 360 (90%% of the viewport)", st.targetY)
	}
	if clicked {
		t.Error("clicking the scrollbar also clicked the row under it")
	}
}

func TestScrollbarHoverThickensAndShowsArrow(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div().Height(Full).ScrollY().Cursor(CursorText)
		for i := 0; i < 20; i++ {
			list.Child(Div().Height(Px(50)).Shrink(0))
		}
		return list
	}, 300, 400, 1)
	rt.PointerMove(150, 100, 0)
	frame(rt)
	if rt.Cursor() != CursorText {
		t.Fatal("setup: expected the text cursor over the list")
	}
	rt.PointerMove(294, 30, 0)
	frame(rt)
	if rt.sbHot.id == 0 || rt.Cursor() != CursorDefault {
		t.Errorf("over the scrollbar: hot=%v cursor=%v; want hot and an arrow", rt.sbHot, rt.Cursor())
	}
}

func TestHorizontalScrollbar(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		// Not the root: the root is always sized to the window.
		return Div().Child(Div().Height(Px(100)).ScrollX().Row().Child(Div().Size(Px(900), Px(80)).Shrink(0)))
	}, 300, 300, 1)
	rt.PointerMove(150, 50, 0)
	frame(rt)
	sb, ok := rt.scrollbarAt(150, 94)
	if !ok || sb.axis != axisX {
		t.Fatal("no horizontal scrollbar along the bottom")
	}
	x, y := sb.thumb.X+sb.thumb.W/2, sb.thumb.Y+sb.thumb.H/2
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerMove(x+1000, y, 0)
	rt.PointerUp(x+1000, y, input.ButtonLeft, 0)
	frame(rt)
	if st := rt.tree.children[0].st; st.scrollX != 600 {
		t.Errorf("scrollX = %v, want 600", st.scrollX)
	}
}

func TestZoomShortcutsAndCoordinates(t *testing.T) {
	clicks := 0
	var reported float32
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(20).
			Child(Div().Key("btn").Size(Px(100), Px(40)).OnClick(func() { clicks++ })).
			Child(Text("Zoomed text").Key("label"))
	}, 600, 400, 1)
	rt.OnZoomChange(func(z float32) { reported = z })

	rt.Press(input.KeyEqual, input.ModCtrl) // 110%
	rt.Press(input.KeyEqual, input.ModCtrl) // 125%
	if rt.Zoom() != 1.25 || reported != 1.25 {
		t.Fatalf("two Ctrl+= steps: zoom %v (reported %v), want 1.25", rt.Zoom(), reported)
	}
	rt.SetZoom(1.5)
	frame(rt)
	// The UI's own coordinates shrink by the zoom; the window does not.
	if rt.viewW != 400 || rt.scale != 1.5 {
		t.Errorf("at 150%%: view width %v scale %v, want 400 and 1.5", rt.viewW, rt.scale)
	}
	// A click at device position (20+50)*1.5 hits the button at its zoomed place.
	rt.PointerDown(105, 60, input.ButtonLeft, 1, 0)
	rt.PointerUp(105, 60, input.ButtonLeft, 0)
	if clicks != 1 {
		t.Errorf("click at the zoomed button position: clicks = %d", clicks)
	}
	if err := rt.ClickKey("btn"); err != nil || clicks != 2 {
		t.Errorf("ClickKey under zoom: err=%v clicks=%d", err, clicks)
	}
	// Text is rasterized at the zoomed pixel size, not scaled up.
	var maxH float32
	for _, g := range rt.scene.Glyphs {
		maxH = max(maxH, g.Bounds.H)
	}
	if maxH < 14 { // 14px text cap height ~10px at 100%; ~15px at 150%
		t.Errorf("tallest glyph %vpx; text was not re-rasterized at 150%%", maxH)
	}

	rt.Press(input.Key0, input.ModCtrl)
	if rt.Zoom() != 1 {
		t.Errorf("Ctrl+0: zoom %v, want 1", rt.Zoom())
	}
	rt.Scroll(100, 100, 0, -60, false, input.ModCtrl) // Ctrl+wheel up = zoom in
	if rt.Zoom() != 1.1 {
		t.Errorf("Ctrl+wheel: zoom %v, want 1.1", rt.Zoom())
	}
	rt.SetZoomShortcuts(false)
	rt.Press(input.KeyEqual, input.ModCtrl)
	if rt.Zoom() != 1.1 {
		t.Error("zoom shortcuts still active after SetZoomShortcuts(false)")
	}
}

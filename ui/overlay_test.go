package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/scene"
)

type overlayRig struct {
	rt  *Runtime
	now time.Time
}

func newOverlayRig(root func(*Context) Element, w, h float32) *overlayRig {
	r := &overlayRig{now: time.Unix(1_000_000, 0)}
	r.rt = NewRuntime(root, DarkTheme, nil)
	r.rt.clock = func() time.Time { return r.now }
	r.rt.SetViewport(w, h, 1)
	r.rt.Frame(r.now)
	return r
}

func (r *overlayRig) frame() { r.rt.Frame(r.now) }

func (r *overlayRig) click(x, y float32) {
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
}

func (r *overlayRig) find(t *testing.T, key string) scene.Rect {
	t.Helper()
	rc, ok := r.rt.Find(key)
	if !ok {
		t.Fatalf("no element %q", key)
	}
	return rc
}

func TestPlaceOverlay(t *testing.T) {
	view := scene.Rect{W: 800, H: 600}
	a := scene.Rect{X: 100, Y: 100, W: 80, H: 30}
	tests := []struct {
		name         string
		p            Placement
		a            scene.Rect
		w, h         float32
		wantX, wantY float32
	}{
		{"BelowStart", BelowStart, a, 60, 40, 100, 134},
		{"BelowCenter", BelowCenter, a, 60, 40, 110, 134},
		{"BelowEnd", BelowEnd, a, 60, 40, 120, 134},
		{"AboveStart", AboveStart, a, 60, 40, 100, 56},
		{"AboveCenter", AboveCenter, a, 60, 40, 110, 56},
		{"AboveEnd", AboveEnd, a, 60, 40, 120, 56},
		{"RightStart", RightStart, a, 60, 40, 184, 100},
		{"LeftStart", LeftStart, a, 60, 40, 36, 100},
		{"flip below to above", BelowStart, scene.Rect{X: 100, Y: 550, W: 80, H: 30}, 60, 100, 100, 446},
		{"flip above to below", AboveStart, scene.Rect{X: 100, Y: 20, W: 80, H: 30}, 60, 100, 100, 54},
		{"flip right to left", RightStart, scene.Rect{X: 700, Y: 100, W: 80, H: 30}, 150, 40, 546, 100},
		{"shift in from the left", BelowStart, scene.Rect{X: 0, Y: 100, W: 20, H: 30}, 60, 40, 8, 134},
		{"shift in from the right", BelowEnd, scene.Rect{X: 790, Y: 100, W: 10, H: 30}, 300, 40, 492, 134},
	}
	for _, tc := range tests {
		x, y := placeOverlay(tc.p, 4, tc.a, tc.w, tc.h, view)
		if x != tc.wantX || y != tc.wantY {
			t.Errorf("%s: got (%v,%v), want (%v,%v)", tc.name, x, y, tc.wantX, tc.wantY)
		}
	}
}

func TestOverlayEscapesClipping(t *testing.T) {
	hits := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Row().
			Child(Div().Key("sidebar").Size(Px(200), Px(300)).ScrollY().
				Child(Div().Key("btn").Size(Px(160), Px(40)).
					Child(Div().Key("menu").Overlay(BelowStart).Size(Px(240), Px(200)).Row().
						Child(Div().Width(Px(200))).
						Child(Div().Key("item").Size(Px(40), Px(40)).OnClick(func() { hits++ })))))
	}, 800, 600)
	menu, side := r.find(t, "menu"), r.find(t, "sidebar")
	if menu.W != 240 {
		t.Errorf("menu width %v, want 240", menu.W)
	}
	if menu.X+menu.W <= side.X+side.W {
		t.Errorf("menu %+v does not extend past the sidebar %+v", menu, side)
	}
	item := r.find(t, "item")
	if item.X < side.X+side.W {
		t.Fatalf("item %+v is inside the sidebar %+v", item, side)
	}
	if err := r.rt.ClickKey("item"); err != nil || hits != 1 {
		t.Errorf("item clicked %d times, err %v; want 1", hits, err)
	}
}

func TestOverlayFlipsNearBottom(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Child(Spacer()).
			Child(Div().Key("anchor").Height(Px(40)).
				Child(Div().Key("menu").Overlay(BelowStart).Size(Px(100), Px(100))))
	}, 800, 600)
	anchor, menu := r.find(t, "anchor"), r.find(t, "menu")
	if anchor.Y < 500 {
		t.Fatalf("anchor at %v, expected near the bottom", anchor.Y)
	}
	if menu.Y+menu.H > anchor.Y {
		t.Errorf("menu %+v not above anchor %+v", menu, anchor)
	}
}

func TestOverlayCoversOtherBranches(t *testing.T) {
	over, under := 0, 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Row().Padding(10).
			Child(Div().Key("a").Size(Px(100), Px(40)).
				Child(Div().Key("menu").Overlay(BelowStart).Size(Px(200), Px(100)).OnClick(func() { over++ }))).
			Child(Div().Key("b").Size(Px(300), Px(300)).OnClick(func() { under++ }))
	}, 800, 600)
	b := r.find(t, "b")
	menu := r.find(t, "menu")
	r.click(b.X+20, menu.Y+20)
	if over != 1 || under != 0 {
		t.Errorf("overlap click: overlay %d, underlying %d; want 1, 0", over, under)
	}
	r.click(b.X+250, b.Y+250)
	if under != 1 {
		t.Errorf("clicking the branch away from the overlay: %d, want 1", under)
	}
}

func TestOnClickOutside(t *testing.T) {
	open := NewSignal(false)
	closes, toggles := 0, 0
	r := newOverlayRig(func(ctx *Context) Element {
		toggle := Div().Key("toggle").Size(Px(100), Px(30)).OnClick(func() { toggles++; open.Set(!open.Peek()) })
		if open.Get() {
			toggle.Child(Div().Key("menu").Overlay(BelowStart).Size(Px(150), Px(100)).
				OnClickOutside(func() { closes++; open.Set(false) }).
				Child(Div().Key("inner").Height(Px(30)).OnClick(func() {})))
		}
		return Div().Padding(20).Child(toggle)
	}, 800, 600)
	if err := r.rt.ClickKey("toggle"); err != nil || !open.Peek() {
		t.Fatalf("toggle did not open the menu (err %v)", err)
	}
	r.click(700, 500)
	if open.Peek() || closes != 1 {
		t.Errorf("click elsewhere: open=%v closes=%d; want closed once", open.Peek(), closes)
	}
	r.rt.ClickKey("toggle")
	if !open.Peek() {
		t.Fatal("menu did not reopen")
	}
	if err := r.rt.ClickKey("inner"); err != nil || !open.Peek() || closes != 1 {
		t.Errorf("click inside: open=%v closes=%d (err %v); want still open", open.Peek(), closes, err)
	}
	toggles = 0
	r.rt.ClickKey("toggle")
	if open.Peek() || toggles != 1 || closes != 1 {
		t.Errorf("click on anchor: open=%v toggles=%d closes=%d; want closed by the toggle alone", open.Peek(), toggles, closes)
	}
}

func TestNestedOverlayIsInside(t *testing.T) {
	outer, sub := 0, 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).Child(Div().Key("anchor").Size(Px(100), Px(30)).
			Child(Div().Key("menu").Overlay(BelowStart).Size(Px(150), Px(100)).
				OnClickOutside(func() { outer++ }).
				Child(Div().Key("row").Height(Px(30)).
					Child(Div().Key("submenu").Overlay(RightStart).Size(Px(100), Px(60)).
						Child(Div().Key("subitem").Height(Px(30)).OnClick(func() { sub++ }))))))
	}, 800, 600)
	if err := r.rt.ClickKey("subitem"); err != nil || sub != 1 {
		t.Fatalf("subitem clicks %d, err %v", sub, err)
	}
	if outer != 0 {
		t.Errorf("click in the submenu fired the outer OnClickOutside %d times", outer)
	}
	r.click(700, 500)
	if outer != 1 {
		t.Errorf("click elsewhere fired OnClickOutside %d times, want 1", outer)
	}
}

func TestTooltip(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Padding(20).Gap(20).
			Child(Div().Key("t").Size(Px(100), Px(30)).Tooltip("Hello")).
			Child(Button("Go").Key("b").Tooltip("Run it"))
	}, 800, 600)
	hover := func(key string) {
		rc := r.find(t, key)
		r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
		r.frame()
	}
	if _, ok := r.rt.Find("jamb.tooltip"); ok {
		t.Fatal("tooltip present before hover")
	}
	hover("t")
	if _, ok := r.rt.Find("jamb.tooltip"); ok {
		t.Fatal("tooltip appeared immediately")
	}
	r.now = r.now.Add(600 * time.Millisecond)
	r.frame()
	tip, ok := r.rt.Find("jamb.tooltip")
	if !ok {
		t.Fatal("tooltip missing after the delay")
	}
	if tr := r.find(t, "t"); tip.Y < tr.Y+tr.H {
		t.Errorf("tooltip %+v not below target %+v", tip, tr)
	}
	r.rt.PointerMove(700, 500, 0)
	r.frame()
	if _, ok := r.rt.Find("jamb.tooltip"); ok {
		t.Error("tooltip still shown after the pointer left")
	}

	hover("b")
	r.now = r.now.Add(600 * time.Millisecond)
	r.frame()
	tip, ok = r.rt.Find("jamb.tooltip")
	if !ok {
		t.Fatal("button tooltip missing")
	}
	if br := r.find(t, "b"); tip.Y < br.Y+br.H {
		t.Errorf("button tooltip %+v not below button %+v", tip, br)
	}
}

func TestOverlayOfScrolledAwayAnchorIsInert(t *testing.T) {
	hits := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().
			Child(Div().Key("scroller").Size(Px(200), Px(100)).ScrollY().
				Child(Div().Key("anchor").Size(Px(100), Px(30)).
					Child(Div().Key("ov").Overlay(BelowStart).Size(Px(100), Px(200)).OnClick(func() { hits++ }))).
				Child(Div().Height(Px(500))))
	}, 800, 600)
	r.click(10, 150)
	if hits != 1 {
		t.Fatalf("overlay not hit while its anchor is visible (%d)", hits)
	}
	r.rt.Scroll(50, 50, 0, 40, true, 0)
	r.frame()
	r.click(10, 150)
	if hits != 1 {
		t.Errorf("overlay of a scrolled-away anchor was hit")
	}
}

func TestClickInOverlayDoesNotClickAnchor(t *testing.T) {
	toggles := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().
			Child(Div().Key("anchor").Size(Px(100), Px(30)).OnClick(func() { toggles++ }).
				Child(Div().Key("menu").Overlay(BelowStart).Size(Px(150), Px(80)).Padding(10)))
	}, 800, 600)
	m := r.find(t, "menu")
	r.click(m.X+2, m.Y+2) // the menu's padding: no handler of its own
	if toggles != 0 {
		t.Errorf("clicking inside the overlay clicked its anchor %d times", toggles)
	}
}

func TestOverlayResizesWithWindow(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().
			Child(Div().Size(Px(100), Px(30)).
				Child(Div().Key("menu").Overlay(BelowStart).Width(Percent(50)).Height(Px(40))))
	}, 800, 600)
	if w := r.find(t, "menu").W; w != 400 {
		t.Fatalf("menu width = %v, want 400 (half the window)", w)
	}
	r.rt.SetViewport(600, 600, 1)
	r.frame()
	if w := r.find(t, "menu").W; w != 300 {
		t.Errorf("after resizing, menu width = %v, want 300", w)
	}
}

func TestOverlayOnZeroSizeAnchor(t *testing.T) {
	hits := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Size(Px(800), Px(600)).
			Child(Div().Absolute().Top(48).Left(0).Right(0).Height(Px(0)).
				Child(Div().Key("menu").Overlay(BelowCenter).Size(Px(200), Px(100)).OnClick(func() { hits++ })))
	}, 800, 600)
	m := r.find(t, "menu")
	if m.W != 200 || m.H != 100 || m.Y < 48 {
		t.Fatalf("menu under a zero-height anchor at %+v, want 200x100 below y=48", m)
	}
	if err := r.rt.ClickKey("menu"); err != nil || hits != 1 {
		t.Errorf("menu clicked %d times (err %v), want 1", hits, err)
	}
}

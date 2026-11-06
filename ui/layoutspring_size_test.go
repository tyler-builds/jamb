package ui_test

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// growBox is a column holding a box keyed "box", h tall, with the layout spring
// sp and the overflow of clip, over a box keyed "below". The box holds a 160 ×
// 30 child keyed "child", 10 px from its top, and a text keyed "label".
func growBox(h *ui.Signal[float32], sp ui.Spring, clip bool) func(*ui.Context) ui.Element {
	return func(*ui.Context) ui.Element {
		box := ui.Div().Key("box").Column().Width(ui.Px(200)).Height(ui.Px(h.Get())).PaddingTop(10).
			Background(ui.Hex("#333")).Radius(8).LayoutSpring(sp).
			Child(ui.Div().Key("child").Size(ui.Px(160), ui.Px(30)).Background(ui.Hex("#555"))).
			Child(ui.Div().Key("label").Child(ui.Text("A label that keeps its width")))
		if clip {
			box.ClipContent()
		}
		return ui.Div().Column().Child(box).Child(ui.Div().Key("below").Size(ui.Px(200), ui.Px(20)))
	}
}

// heights records the height of the element keyed key at every frame of d.
func heights(u *uitest.UI, key string, d time.Duration) []float32 {
	var out []float32
	for end := u.Now().Add(d); u.Now().Before(end); {
		u.Advance(16 * time.Millisecond)
		out = append(out, u.Get(key).Bounds().H)
	}
	return out
}

// glides fails the test unless path goes steadily from near from to exactly to,
// never passing it, in steps of at most step, and takes 10 frames or more.
func glides(t *testing.T, what string, path []float32, from, to, step float32) {
	t.Helper()
	dir := float32(1)
	if to < from {
		dir = -1
	}
	prev := from
	for i, v := range path {
		if (v-prev)*dir < 0 || (v-prev)*dir > step || (v-to)*dir > 0 {
			t.Fatalf("%s goes from %v to %v at frame %d (path %v), want a steady glide from %v to %v", what, prev, v, i, path, from, to)
		}
		prev = v
	}
	if prev != to {
		t.Fatalf("%s ends at %v (path %v), want %v", what, prev, path, to)
	}
	if n := len(path) - len(trimEnd(path, to)); len(path)-n < 10 {
		t.Errorf("%s reached %v after %d frames (path %v), want 10 or more", what, to, len(path)-n, path)
	}
}

// trimEnd returns path without the values equal to v at its end.
func trimEnd(path []float32, v float32) []float32 {
	for len(path) > 0 && path[len(path)-1] == v {
		path = path[:len(path)-1]
	}
	return path
}

// A box the layout makes taller keeps its height on the frame of the change,
// then grows to its new height over many frames, never passing it, and comes
// to rest there; made shorter again, it shrinks the same way. What is below it
// takes its new place at once, as it has no spring of its own.
func TestLayoutSizeSpringGrowsAndShrinks(t *testing.T) {
	h := ui.NewSignal[float32](40)
	u := uitest.New(t, growBox(h, reflow, false))
	rec := u.Record()
	h.Set(140)
	u.Frame()
	if got := u.Get("box").Bounds().H; got != 40 {
		t.Fatalf("on the frame of the change the box is %v tall, want its old height 40", got)
	}
	if y := u.Get("below").Bounds().Y; y != 140 {
		t.Errorf("the box below is at Y %v, want its new place 140 at once", y)
	}
	glides(t, "growing, the box's height", heights(u, "box", time.Second), 40, 140, 15)
	if !u.Runtime().Settled() {
		t.Errorf("the UI has not settled a second after the change")
	}
	h.Set(40)
	u.Frame()
	glides(t, "shrinking, the box's height", heights(u, "box", time.Second), 140, 40, 15)
	rec.Check(t)
}

// A box whose height changes again mid-flight keeps its size and the speed it
// grows at: on the next frame it goes on growing, then turns back, with no jump.
func TestLayoutSizeSpringRetargetKeepsVelocity(t *testing.T) {
	h := ui.NewSignal[float32](40)
	u := uitest.New(t, growBox(h, reflow, false))
	h.Set(140)
	u.Frame()
	u.Advance(80 * time.Millisecond)
	h1 := u.Get("box").Bounds().H
	u.Advance(16 * time.Millisecond)
	h2 := u.Get("box").Bounds().H
	if !(h2 > h1 && h1 > 40 && h2 < 140) {
		t.Fatalf("growing: height %v then %v, want rising between 40 and 140", h1, h2)
	}
	h.Set(40)
	u.Frame()
	if got := u.Get("box").Bounds().H; got != h2 {
		t.Errorf("on the frame of the retarget the box is %v tall, want what it was, %v", got, h2)
	}
	u.Advance(16 * time.Millisecond)
	h3 := u.Get("box").Bounds().H
	if h3 <= h2 {
		t.Errorf("one frame after the retarget the height is %v, want still growing past %v (the velocity is kept)", h3, h2)
	}
	if h3-h2 > 2*(h2-h1) {
		t.Errorf("one frame after the retarget the height jumped from %v to %v", h2, h3)
	}
	u.Settle()
	if got := u.Get("box").Bounds().H; got != 40 {
		t.Errorf("settled at height %v, want 40", got)
	}
}

// sample returns the element keyed key in the last frame rec recorded, and
// whether that frame drew it.
func sample(rec *uitest.Recording, key string) (uitest.Sample, bool) {
	f := rec.Frames()
	for _, s := range f[len(f)-1].Elements {
		if s.Key == key {
			return s, true
		}
	}
	return uitest.Sample{}, false
}

// While a box springs to a new size, what it holds keeps its own layout and
// moves with the box: a child keeps its size and its place from the box's top,
// and a text keeps its width. Only the box's own background is drawn at the
// size the spring has reached.
func TestLayoutSizeSpringChildrenKeepTheirLayout(t *testing.T) {
	h := ui.NewSignal[float32](140)
	u := uitest.New(t, growBox(h, reflow, false))
	label := u.Get("label").Bounds()
	h.Set(60)
	u.Frame()
	u.Advance(100 * time.Millisecond)
	box, child := u.Get("box").Bounds(), u.Get("child").Bounds()
	if box.H <= 60 || box.H >= 140 {
		t.Fatalf("the box is %v tall 100 ms into its change, want between 60 and 140", box.H)
	}
	if child.W != 160 || child.H != 30 || child.Y != box.Y+10 || child.X != box.X {
		t.Errorf("the child is %v in a box at %v, want 160 × 30, 10 px from its top", child, box)
	}
	if got := u.Get("label").Bounds(); got.W != label.W || got.H != label.H {
		t.Errorf("the label is %v while the box springs, want the size it had, %v", got, label)
	}
	if bg := u.Get("box").Style().Bounds; bg.H != box.H {
		t.Errorf("the box's background is drawn %v tall, want the spring's height %v", bg.H, box.H)
	}
}

// What a box holds is clipped to the size the box is drawn at only when the
// box clips. A box growing from 20 to 100 px holds a child 70 px down: a box
// that does not clip shows it from the start, and one that clips (ClipContent)
// only once it has grown past it.
func TestLayoutSizeSpringClipsOnlyWhenTheElementClips(t *testing.T) {
	for _, clip := range []bool{false, true} {
		h := ui.NewSignal[float32](20)
		u := uitest.New(t, func(ctx *ui.Context) ui.Element {
			box := ui.Div().Key("box").Column().Width(ui.Px(200)).Height(ui.Px(h.Get())).
				Background(ui.Hex("#333")).LayoutSpring(reflow).
				Child(ui.Div().Height(ui.Px(70)).Shrink(0)).
				Child(ui.Div().Key("tail").Size(ui.Px(200), ui.Px(10)).Shrink(0).Background(ui.Hex("#555")))
			if clip {
				box.ClipContent()
			}
			return ui.Div().Column().Child(box)
		})
		rec := u.Record()
		h.Set(100)
		u.Frame()
		u.Advance(16 * time.Millisecond)
		if b := u.Get("box").Bounds(); b.H >= 60 {
			t.Fatalf("clip %v: the box is %v tall a frame into its change, want under 60", clip, b.H)
		}
		tail, ok := sample(rec, "tail")
		if shown := ok && tail.Visible; shown == clip {
			t.Errorf("clip %v: a child past the drawn bottom of the box is shown: %v", clip, shown)
		}
		if b := u.Get("tail").Bounds(); b.W != 200 || b.H != 10 {
			t.Errorf("clip %v: the child is %v × %v, want 200 × 10", clip, b.W, b.H)
		}
		u.Settle()
		if tail, ok := sample(rec, "tail"); !ok || !tail.Visible {
			t.Errorf("clip %v: the child is not shown once the box has grown", clip)
		}
	}
}

// A box springs from its size only once it has been drawn: on its first
// appearance it has its size at once, and with reduced motion, or when the
// window's size changes, it takes a new size at once.
func TestLayoutSizeSpringSnaps(t *testing.T) {
	show := ui.NewSignal(false)
	h := ui.NewSignal[float32](40)
	app := func(*ui.Context) ui.Element {
		col := ui.Div().Column()
		if show.Get() {
			col.Child(ui.Div().Key("new").Size(ui.Px(100), ui.Px(80)).LayoutSpring(reflow))
		}
		// The box's width is a share of the window's.
		return col.Child(ui.Div().Key("box").Width(ui.Percent(50)).Height(ui.Px(h.Get())).LayoutSpring(reflow))
	}
	u := uitest.New(t, app)
	show.Set(true)
	u.Frame()
	if b := u.Get("new").Bounds(); b.W != 100 || b.H != 80 {
		t.Errorf("a new box is drawn %v × %v, want its size 100 × 80 at once", b.W, b.H)
	}
	u.Settle()
	u.Runtime().SetViewport(600, 600, 1)
	u.Frame()
	if w := u.Get("box").Bounds().W; w != 300 {
		t.Errorf("after the window was resized the box is %v wide, want 300 at once", w)
	}
	if !u.Runtime().Settled() {
		t.Errorf("after a resize the UI animates")
	}

	u = uitest.New(t, app, uitest.Settings(ui.SystemSettings{ReducedMotion: true}))
	h.Set(140)
	u.Frame()
	if got := u.Get("box").Bounds().H; got != 140 {
		t.Errorf("with reduced motion the box is %v tall, want its new height 140 at once", got)
	}
	if !u.Runtime().Settled() {
		t.Errorf("with reduced motion the UI animates")
	}
}

// Hit testing follows the size a box is drawn at: while a box shrinks, a click
// below its new bottom but inside its drawn box reaches it.
func TestLayoutSizeSpringHitTestingFollowsTheBox(t *testing.T) {
	h := ui.NewSignal[float32](200)
	clicks := 0
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Column().Child(ui.Div().Key("box").Size(ui.Px(200), ui.Px(h.Get())).
			Background(ui.Hex("#333")).LayoutSpring(reflow).OnClick(func() { clicks++ }))
	})
	h.Set(40)
	u.Frame()
	u.Advance(64 * time.Millisecond)
	b := u.Get("box").Bounds()
	if b.H <= 100 || b.H >= 200 {
		t.Fatalf("the box is %v tall 64 ms into its change, want between 100 and 200", b.H)
	}
	rt := u.Runtime()
	click := func(y float32) {
		rt.PointerMove(100, y, 0)
		rt.PointerDown(100, y, input.ButtonLeft, 1, 0)
		rt.PointerUp(100, y, input.ButtonLeft, 0)
		u.Frame()
	}
	click(80) // below its new bottom, inside its drawn box
	if clicks != 1 {
		t.Errorf("a click inside the box as drawn made %d clicks, want 1", clicks)
	}
	if a, d := u.Get("box").Node().Bounds, u.Get("box").Bounds(); abs(a.H-d.H) > 1 {
		t.Errorf("the box's accessibility bounds are %v, want its drawn box %v (to the pixel)", a, d)
	}
	u.Settle()
	click(80)
	if clicks != 1 {
		t.Errorf("a click below the box at rest reached it")
	}
}

// ---- widgets ----

// A Collapsible springs open and closed: the section grows and shrinks over
// many frames, its content is revealed and covered by its bottom edge (never
// drawn past it), and nothing pops. With KeepMounted, which hides the content
// rather than removing it, the same holds. A rebuild while it closes does not
// cut the closing short.
func TestCollapsibleSpringsOpenAndClosed(t *testing.T) {
	for _, keep := range []bool{false, true} {
		open, rebuilds := ui.NewSignal(false), ui.NewSignal(0)
		u := uitest.New(t, func(*ui.Context) ui.Element {
			_ = rebuilds.Get()
			c := ui.Collapsible("Details", open, ui.Div().Key("body").Height(ui.Px(100)).Background(ui.Hex("#444"))).Key("sec")
			if keep {
				c.KeepMounted()
			}
			return ui.Div().Width(ui.Px(300)).Child(c)
		})
		closed := u.Get("sec").Bounds().H
		rec := u.Record()
		u.ByRole(access.RoleButton, "Details").Click()
		path := heights(u, "sec", time.Second)
		full := path[len(path)-1]
		glides(t, "opening, the section's height", path, closed, full, 20)
		u.ByRole(access.RoleButton, "Details").Click()
		u.Frame()
		u.Advance(50 * time.Millisecond)
		rebuilds.Set(1)
		u.Advance(50 * time.Millisecond)
		sec := u.Get("sec").Bounds()
		if sec.H <= closed || sec.H >= full {
			t.Fatalf("keep %v: closing, the section is %v tall after 100 ms, want between %v and %v", keep, sec.H, closed, full)
		}
		// The content is drawn while the section closes, inside it.
		f := rec.Frames()
		var body *uitest.Sample
		for i, s := range f[len(f)-1].Elements {
			if s.Key == "body" {
				body = &f[len(f)-1].Elements[i]
			}
		}
		if body == nil || !body.Exiting || !body.Visible {
			t.Errorf("keep %v: closing, the content is not drawn while the section shrinks: %+v", keep, body)
		} else if body.Opacity >= 1 || body.Opacity <= 0 {
			t.Errorf("keep %v: closing, the content is drawn at opacity %v, want fading", keep, body.Opacity)
		}
		u.Settle()
		if got := u.Get("sec").Bounds().H; got != closed {
			t.Errorf("keep %v: closed, the section is %v tall, want %v", keep, got, closed)
		}
		if _, ok := sample(rec, "body"); ok {
			t.Errorf("keep %v: the content is still drawn once the section has closed", keep)
		}
		rec.Check(t)
	}
}

// In an Accordion with one section open at a time, opening a section closes
// the open one, and the sections between glide: nothing pops or jumps.
func TestAccordionSectionsGlide(t *testing.T) {
	a, b, c := ui.NewSignal(true), ui.NewSignal(false), ui.NewSignal(false)
	body := func() ui.Element { return ui.Div().Height(ui.Px(80)).Background(ui.Hex("#444")) }
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Width(ui.Px(300)).Child(ui.Accordion(
			ui.Collapsible("A", a, body()).Key("a"),
			ui.Collapsible("B", b, body()).Key("b"),
			ui.Collapsible("C", c, body()).Key("c"),
		).SingleOpen())
	})
	before := u.Get("c").Bounds().Y
	rec := u.Record()
	u.ByRole(access.RoleButton, "B").Click()
	u.Advance(100 * time.Millisecond)
	// A shrinks and B grows by the same amount, so C stays about where it was.
	if y := u.Get("b").Bounds().Y; y >= before || y <= u.Get("a").Bounds().Y {
		t.Errorf("B is at Y %v while A closes, want gliding up below A", y)
	}
	u.Settle()
	if a.Get() || !b.Get() {
		t.Errorf("after opening B: A open %v, B open %v", a.Get(), b.Get())
	}
	if w := u.Warnings(); len(w) > 0 {
		t.Errorf("warnings: %q, want none (the sections need no key)", w)
	}
	rec.Check(t)
}

// A Sidebar springs to its new width as it collapses and expands; its labels
// are covered as it narrows and revealed as it widens, and nothing pops.
func TestSidebarSpringsItsWidth(t *testing.T) {
	sel := ui.NewSignal("inbox")
	collapsed := ui.NewSignal(false)
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Row().Height(ui.Px(300)).Child(ui.Sidebar(sel,
			ui.SidebarHeading("Mail"),
			ui.SidebarItem{Key: "inbox", Label: "Inbox", Badge: 4},
			ui.SidebarItem{Key: "sent", Label: "Sent"},
		).Key("nav").Collapsed(collapsed).Toggle())
	})
	wide := u.Get("nav").Bounds().W
	rec := u.Record()
	u.ByRole(access.RoleButton, "Collapse navigation").Click()
	path := []float32{}
	for range 60 {
		u.Advance(16 * time.Millisecond)
		path = append(path, u.Get("nav").Bounds().W)
	}
	glides(t, "collapsing, the sidebar's width", path, wide, 48, 30)
	u.ByRole(access.RoleButton, "Expand navigation").Click()
	u.Settle()
	if w := u.Get("nav").Bounds().W; w != wide {
		t.Errorf("expanded again, the sidebar is %v wide, want %v", w, wide)
	}
	rec.Check(t)
}

// With reduced motion a Collapsible and a Sidebar open, close, collapse and
// expand at once, and keep nothing drawn after it has gone.
func TestDisclosureWidgetsSnapWithReducedMotion(t *testing.T) {
	open, collapsed, sel := ui.NewSignal(false), ui.NewSignal(false), ui.NewSignal("a")
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Row().Height(ui.Px(300)).
			Child(ui.Sidebar(sel, ui.SidebarItem{Key: "a", Label: "Alpha"}).Key("nav").Collapsed(collapsed).Toggle()).
			Child(ui.Div().Width(ui.Px(300)).Child(ui.Collapsible("Details", open,
				ui.Div().Key("body").Height(ui.Px(100))).Key("sec")))
	}, uitest.Settings(ui.SystemSettings{ReducedMotion: true}))
	closed := u.Get("sec").Bounds().H
	u.ByRole(access.RoleButton, "Details").Click()
	if h := u.Get("sec").Bounds().H; h < closed+100 || !u.Runtime().Settled() {
		t.Errorf("with reduced motion the open section is %v tall, want at least %v at once", h, closed+100)
	}
	rec := u.Record()
	u.ByRole(access.RoleButton, "Details").Click()
	if _, ok := sample(rec, "body"); ok {
		t.Errorf("with reduced motion the content of a closed section is still drawn")
	}
	u.ByRole(access.RoleButton, "Collapse navigation").Click()
	if w := u.Get("nav").Bounds().W; w != 48 {
		t.Errorf("with reduced motion the collapsed sidebar is %v wide, want 48 at once", w)
	}
	if !u.Runtime().Settled() {
		t.Errorf("with reduced motion the UI animates")
	}
}

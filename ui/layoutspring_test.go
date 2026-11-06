package ui_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// reflow is the layout spring of these tests: the theme's default.
var reflow = ui.MotionTokens{}.ReflowSpring()

// rowList is a column of 20 px rows keyed by their names, in the order of
// order, whose rows have the layout spring sp.
func rowList(order *ui.Signal[[]string], sp ui.Spring) func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		list := ui.Div().Column().Width(ui.Px(200))
		for _, name := range order.Get() {
			list.Child(ui.Div().Key(name).Height(ui.Px(20)).Background(ui.Hex("#333")).LayoutSpring(sp))
		}
		return list
	}
}

// ys records the Y of the element keyed key at every frame of d.
func ys(u *uitest.UI, key string, d time.Duration) []float32 {
	var out []float32
	for end := u.Now().Add(d); u.Now().Before(end); {
		u.Advance(16 * time.Millisecond)
		out = append(out, u.Get(key).Bounds().Y)
	}
	return out
}

// A row the layout moves keeps its place on the frame of the move, then glides
// to its new place over many frames, never passing it, and comes to rest there:
// the UI settles.
func TestLayoutSpringMovesAndSettles(t *testing.T) {
	order := ui.NewSignal([]string{"a", "b", "c"})
	u := uitest.New(t, rowList(order, reflow))
	rec := u.Record()
	order.Set([]string{"c", "a", "b"})
	u.Frame()
	if y := u.Get("c").Bounds().Y; y != 40 {
		t.Fatalf("on the frame of the move c is drawn at Y %v, want its old place 40", y)
	}
	path := ys(u, "c", time.Second)
	for i := 1; i < len(path); i++ {
		if path[i] > path[i-1] || path[i-1]-path[i] > 6 {
			t.Fatalf("c moves from %v to %v in one frame (path %v), want a steady glide up", path[i-1], path[i], path)
		}
	}
	if moving := slices.IndexFunc(path, func(y float32) bool { return y == 0 }); moving < 10 {
		t.Errorf("c reached its place after %d frames (path %v), want 10 or more", moving, path)
	}
	if !u.Runtime().Settled() {
		t.Errorf("the UI has not settled a second after the move")
	}
	for key, want := range map[string]float32{"c": 0, "a": 20, "b": 40} {
		if y := u.Get(key).Bounds().Y; y != want {
			t.Errorf("%s rests at Y %v, want %v", key, y, want)
		}
	}
	rec.Check(t)
}

// A row whose target changes mid-flight keeps its place and velocity: on the
// next frame it goes on the way it was going, then turns back, with no jump.
func TestLayoutSpringRetargetKeepsVelocity(t *testing.T) {
	order := ui.NewSignal([]string{"a", "b", "c"})
	u := uitest.New(t, rowList(order, reflow))
	order.Set([]string{"c", "a", "b"})
	u.Frame()
	u.Advance(80 * time.Millisecond)
	y1 := u.Get("c").Bounds().Y
	u.Advance(16 * time.Millisecond)
	y2 := u.Get("c").Bounds().Y
	if !(y2 < y1 && y1 < 40 && y2 > 0) {
		t.Fatalf("moving up: Y %v then %v, want falling between 40 and 0", y1, y2)
	}
	order.Set([]string{"a", "b", "c"}) // back where it came from
	u.Frame()
	if y := u.Get("c").Bounds().Y; y != y2 {
		t.Errorf("on the frame of the retarget c is at Y %v, want where it was, %v", y, y2)
	}
	u.Advance(16 * time.Millisecond)
	y3 := u.Get("c").Bounds().Y
	if y3 >= y2 {
		t.Errorf("one frame after the retarget Y = %v, want still rising past %v (the velocity is kept)", y3, y2)
	}
	if y2-y3 > 2*(y1-y2) {
		t.Errorf("one frame after the retarget Y jumped from %v to %v", y2, y3)
	}
	u.Settle()
	if y := u.Get("c").Bounds().Y; y != 40 {
		t.Errorf("settled at Y %v, want 40", y)
	}
}

// A row that first appears is drawn at its place at once, and the rows it
// pushes down glide.
func TestLayoutSpringFirstAppearanceSnaps(t *testing.T) {
	order := ui.NewSignal([]string{"a", "b"})
	u := uitest.New(t, rowList(order, reflow))
	order.Set([]string{"new", "a", "b"})
	u.Frame()
	if y := u.Get("new").Bounds().Y; y != 0 {
		t.Errorf("a new row is drawn at Y %v, want its place 0", y)
	}
	if y := u.Get("a").Bounds().Y; y != 0 {
		t.Errorf("on the frame of the insert a is at Y %v, want its old place 0", y)
	}
	u.Advance(64 * time.Millisecond)
	if y := u.Get("a").Bounds().Y; y <= 0 || y >= 20 {
		t.Errorf("a is at Y %v while it glides, want between 0 and 20", y)
	}
	u.Settle()
}

// With reduced motion the rows take their new places at once and nothing
// asks for frames.
func TestLayoutSpringReducedMotionSnaps(t *testing.T) {
	order := ui.NewSignal([]string{"a", "b", "c"})
	u := uitest.New(t, rowList(order, reflow), uitest.Settings(ui.SystemSettings{ReducedMotion: true}))
	order.Set([]string{"c", "a", "b"})
	u.Frame()
	if y := u.Get("c").Bounds().Y; y != 0 {
		t.Errorf("with reduced motion c is at Y %v, want its new place 0", y)
	}
	if !u.Runtime().Settled() {
		t.Errorf("with reduced motion the UI is not settled after the move")
	}
}

// A change of the window's size, zoom or text scale moves the elements at
// once: an element centered in the window, and one under text that grows,
// are at their new places on the frame of the change.
func TestLayoutSpringSnapsOnResizeZoomAndTextScale(t *testing.T) {
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Column().Align(ui.Center).Width(ui.Percent(100)).
			Child(ui.Text("Heading").FontSize(14)).
			Child(ui.Div().Key("box").Size(ui.Px(40), ui.Px(20)).LayoutSpring(reflow)).
			Child(ui.Div().Key("twin").Size(ui.Px(40), ui.Px(20)))
	})
	same := func(when string) {
		t.Helper()
		b, w := u.Get("box").Bounds(), u.Get("twin").Bounds()
		if b.X != w.X || b.Y+20 != w.Y {
			t.Errorf("%s: the springing box is at (%v,%v) and its twin at (%v,%v), want it at its place, above its twin",
				when, b.X, b.Y, w.X, w.Y)
		}
		if !u.Runtime().Settled() {
			t.Errorf("%s: the UI animates", when)
		}
	}
	u.Runtime().SetViewport(600, 600, 1)
	u.Frame()
	same("after a resize")
	u.Runtime().SetZoom(1.5)
	u.Frame()
	same("after a zoom")
	u.Runtime().SetSystemSettings(ui.SystemSettings{TextScale: 200})
	u.Frame()
	same("after the text scale changed")
}

// A scroll never springs: as the content scrolls, a springing row stays at the
// same distance from one without a spring, on every frame.
func TestLayoutSpringScrollDoesNotSpring(t *testing.T) {
	u := uitest.New(t, func(*ui.Context) ui.Element {
		list := ui.Div().Key("scroller").Column().ScrollY().Height(ui.Px(100)).Width(ui.Px(200))
		for i := range 30 {
			row := ui.Div().Key(fmt.Sprintf("row%d", i)).Height(ui.Px(20))
			if i%2 == 0 {
				row.LayoutSpring(reflow)
			}
			list.Child(row)
		}
		return list
	})
	rec := u.Record()
	u.Get("scroller").Scroll(0, -200)
	for range 30 {
		u.Advance(16 * time.Millisecond)
		a, b := u.Get("row4").Bounds().Y, u.Get("row5").Bounds().Y
		if b-a != 20 {
			t.Fatalf("while scrolling row4 is at Y %v and row5 at %v, want 20 apart", a, b)
		}
	}
	u.Settle()
	rec.Check(t)
}

// A parent that springs carries its children: a child with a spring of its
// own that keeps its place in the parent moves with it, never twice as far.
func TestLayoutSpringParentDoesNotDoubleMoveChildren(t *testing.T) {
	above := ui.NewSignal(false)
	u := uitest.New(t, func(*ui.Context) ui.Element {
		col := ui.Div().Column()
		if above.Get() {
			col.Child(ui.Div().Key("above").Height(ui.Px(60)))
		}
		return col.Child(ui.Div().Key("parent").Column().PaddingTop(10).LayoutSpring(reflow).
			Child(ui.Div().Key("child").Size(ui.Px(40), ui.Px(20)).LayoutSpring(reflow)))
	})
	above.Set(true)
	u.Frame()
	moved := false
	for range 40 {
		u.Advance(16 * time.Millisecond)
		p, c := u.Get("parent").Bounds().Y, u.Get("child").Bounds().Y
		if d := c - p - 10; d > 0.01 || d < -0.01 {
			t.Fatalf("the child is at Y %v in a parent at %v, want 10 below it", c, p)
		}
		moved = moved || p > 0 && p < 60
	}
	if !moved {
		t.Errorf("the parent did not glide")
	}
}

// Hit testing follows where a moving row is drawn: a click there reaches it,
// and a click in its laid-out box, where it is not drawn yet, does not.
func TestLayoutSpringHitTestingFollowsTheRow(t *testing.T) {
	pad := ui.NewSignal(false)
	clicks := 0
	u := uitest.New(t, func(*ui.Context) ui.Element {
		col := ui.Div().Column().Width(ui.Px(200))
		if pad.Get() {
			col.Child(ui.Div().Key("pad").Height(ui.Px(60)))
		}
		return col.Child(ui.Div().Key("row").Size(ui.Px(200), ui.Px(20)).LayoutSpring(reflow).
			OnClick(func() { clicks++ }))
	})
	pad.Set(true)
	u.Frame()
	u.Advance(100 * time.Millisecond)
	r := u.Get("row").Bounds()
	if r.Y <= 10 || r.Y >= 50 {
		t.Fatalf("the row is at Y %v 100 ms into its move, want well between 0 and 60", r.Y)
	}
	rt := u.Runtime()
	click := func(x, y float32) {
		rt.PointerMove(x, y, 0)
		rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
		rt.PointerUp(x, y, input.ButtonLeft, 0)
		u.Frame()
	}
	click(100, r.Y+r.H/2)
	if clicks != 1 {
		t.Errorf("a click on the row where it is drawn, at Y %v, made %d clicks, want 1", r.Y+r.H/2, clicks)
	}
	click(100, 70) // the middle of its laid-out box
	if clicks != 1 {
		t.Errorf("a click in the row's laid-out box, where it is not drawn yet, reached it")
	}
}

// ChildrenLayoutSpring springs the direct children only: a child moved within
// a row snaps, and an unkeyed grandchild needs no key.
func TestChildrenLayoutSpringIsForDirectChildren(t *testing.T) {
	order := ui.NewSignal([]string{"a", "b"})
	indent := ui.NewSignal(false)
	u := uitest.New(t, func(*ui.Context) ui.Element {
		list := ui.Div().Column().Width(ui.Px(200)).ChildrenLayoutSpring(reflow)
		for _, name := range order.Get() {
			row := ui.Div().Key(name).Row().Height(ui.Px(20))
			if indent.Get() {
				row.Child(ui.Div().Width(ui.Px(30)))
			}
			list.Child(row.Child(ui.Div().Key(name+"/label").Size(ui.Px(40), ui.Px(10))))
		}
		return list
	})
	indent.Set(true)
	u.Frame()
	if x := u.Get("a/label").Bounds().X; x != 30 {
		t.Errorf("a grandchild moved in its row is at X %v, want its new place 30 at once", x)
	}
	order.Set([]string{"b", "a"})
	u.Frame()
	u.Advance(64 * time.Millisecond)
	if y := u.Get("b").Bounds().Y; y <= 0 || y >= 20 {
		t.Errorf("a direct child is at Y %v while it glides, want between 0 and 20", y)
	}
	u.Settle()
	if w := u.Warnings(); len(w) > 0 {
		t.Errorf("warnings: %q, want none", w)
	}
}

// An element that springs with no key draws a warning, once, in a UI with
// warnings on (debug builds and tests).
func TestLayoutSpringWithoutKeyWarns(t *testing.T) {
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Column().ChildrenLayoutSpring(reflow).
			Child(ui.Div().Key("keyed").Height(ui.Px(20))).
			Child(ui.Text("unkeyed row"))
	})
	u.Frame()
	var got []string
	for _, w := range u.Warnings() {
		if strings.Contains(w, "layout spring") {
			got = append(got, w)
		}
	}
	if len(got) != 1 || !strings.Contains(got[0], "no key") || !strings.Contains(got[0], "unkeyed row") {
		t.Errorf("warnings %q, want one for the unkeyed text", got)
	}
}

// In a UniformList, rows that a scroll brings in appear at their places, and
// the rows already there do not move against the content; a reorder makes a
// row with a layout spring glide.
func TestLayoutSpringUniformList(t *testing.T) {
	items := ui.NewSignal(make([]string, 0, 200))
	var init []string
	for i := range 200 {
		init = append(init, fmt.Sprintf("item%d", i))
	}
	items.Set(init)
	u := uitest.New(t, func(*ui.Context) ui.Element {
		list := items.Get()
		return ui.UniformList(len(list), func(i int) ui.Element {
			return ui.Div().Key(list[i]).Height(ui.Px(20)).LayoutSpring(reflow).Child(ui.Text(list[i]))
		}).Key("list").Style(ui.Style{Height: ui.Px(200)})
	})
	rec := u.Record()
	u.Get("list").Scroll(0, -600)
	for range 40 {
		u.Advance(16 * time.Millisecond)
		var prev ui.Rect
		for i := range 200 {
			if !u.Has(init[i]) {
				continue
			}
			r := u.Get(init[i]).Bounds()
			if prev.H > 0 && r.Y-prev.Y != 20 {
				t.Fatalf("while scrolling %s is at Y %v, 20 below the row before it at %v? want 20 apart", init[i], r.Y, prev.Y)
			}
			prev = r
		}
	}
	u.Settle()
	rec.Check(t)

	first := ""
	for i := range 200 {
		if u.Has(init[i]) && u.Get(init[i]).Bounds().Y >= 0 {
			first = init[i]
			break
		}
	}
	at := slices.Index(init, first)
	next := slices.Clone(init)
	next[at], next[at+1] = next[at+1], next[at]
	y0 := u.Get(first).Bounds().Y
	items.Set(next)
	u.Frame()
	u.Advance(64 * time.Millisecond)
	if y := u.Get(first).Bounds().Y; y <= y0 || y >= y0+20 {
		t.Errorf("a reordered row is at Y %v, want gliding between %v and %v", y, y0, y0+20)
	}
	u.Settle()
}

// When the rows of a UniformList take a new height, every row moves by the
// measurement, not by a reorder: the rows snap to their places.
func TestLayoutSpringUniformListNewRowHeightSnaps(t *testing.T) {
	h := ui.NewSignal[float32](20)
	u := uitest.New(t, func(*ui.Context) ui.Element {
		rowH := h.Get()
		return ui.UniformList(50, func(i int) ui.Element {
			return ui.Div().Key(fmt.Sprintf("item%d", i)).Height(ui.Px(rowH)).LayoutSpring(reflow)
		}).Style(ui.Style{Height: ui.Px(300)})
	})
	h.Set(30)
	u.Frame()
	a, b := u.Get("item2").Bounds().Y, u.Get("item5").Bounds().Y
	if b-a != 90 {
		t.Errorf("after the rows grew item2 is at Y %v and item5 at %v, want 90 apart at once", a, b)
	}
	if !u.Runtime().Settled() {
		t.Errorf("the rows animate after a new row height")
	}
}

// When a toast times out, the toasts that stay glide to their new places (the
// theme's Reflow spring): the stack does not jump.
func TestToastStackGlides(t *testing.T) {
	var ctx *ui.Context
	u := uitest.New(t, func(c *ui.Context) ui.Element {
		ctx = c
		return ui.Div().Width(ui.Percent(100)).Height(ui.Percent(100)).Child(ui.Toaster())
	})
	// The stack grows from the bottom of the window. A new toast appears in
	// its place, below the others, and holds still while they glide up.
	ctx.Toast(ui.ToastInfo, "First").Duration(10 * time.Second)
	u.Settle()
	const key = "jamb.toast:1"
	first := u.Get(key).Bounds().Y
	ctx.Toast(ui.ToastInfo, "Second").Duration(2 * time.Second)
	u.Frame()
	var path []float32
	for range 30 {
		path = append(path, u.Get("jamb.toast:2").Bounds().Y)
		u.Advance(16 * time.Millisecond)
	}
	if y := u.Get(key).Bounds().Y; y >= first {
		t.Errorf("the first toast is at Y %v after the second came, want above %v", y, first)
	}
	u.Settle()
	second := u.Get("jamb.toast:2").Bounds().Y
	// Its entrance slides it up 8 px; nothing else moves it.
	for i, y := range path {
		if y < second-0.01 || y > second+8.01 {
			t.Fatalf("frame %d after it was added, the new toast is at Y %v, want within its 8 px entrance of %v (path %v)",
				i, y, second, path)
		}
	}
	// The second toast, below the first, leaves first, and the first drops
	// into its place.
	rec := u.Record()
	y0 := u.Get(key).Bounds().Y
	u.Advance(3 * time.Second)
	u.Settle()
	y1 := u.Get(key).Bounds().Y
	if y0 == y1 {
		t.Fatalf("the first toast stayed at Y %v when the second left", y0)
	}
	rec.Check(t)
	places := 0
	last := float32(-1)
	for _, f := range rec.Frames() {
		for _, s := range f.Elements {
			if s.Key == key && !s.Exiting && s.Bounds.Y != last {
				places, last = places+1, s.Bounds.Y
			}
		}
	}
	if places < 10 {
		t.Errorf("the first toast was drawn at %d places on its way from %v to %v, want 10 or more", places, y0, y1)
	}
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// The composite elements take a layout spring through their style: a button
// and a text glide when the layout moves them.
func TestLayoutSpringOnCompositeElements(t *testing.T) {
	first := ui.NewSignal(true)
	u := uitest.New(t, func(*ui.Context) ui.Element {
		col := ui.Div().Column().Width(ui.Px(200))
		if first.Get() {
			col.Child(ui.Div().Key("spacer").Height(ui.Px(60)))
		}
		return col.
			Child(ui.Button("Save").Key("save").LayoutSpring(reflow)).
			Child(ui.Text("Saved").Key("note").LayoutSpring(reflow))
	})
	save, note := u.Get("save").Bounds().Y, u.Get("note").Bounds().Y
	first.Set(false)
	u.Frame()
	u.Advance(64 * time.Millisecond)
	if y := u.Get("save").Bounds().Y; y <= 0 || y >= save {
		t.Errorf("the button is at Y %v while it glides, want between 0 and %v", y, save)
	}
	if y := u.Get("note").Bounds().Y; y <= note-60 || y >= note {
		t.Errorf("the text is at Y %v while it glides, want between %v and %v", y, note-60, note)
	}
	u.Settle()
	if y := u.Get("save").Bounds().Y; y != 0 {
		t.Errorf("the button rests at Y %v, want 0", y)
	}
}

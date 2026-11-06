package ui_test

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

var (
	red   = ui.Hex("#ff0000")
	blue  = ui.Hex("#0000ff")
	green = ui.Hex("#00ff00")
)

// photo is a tile with the Matched id "photo": a green band over its top
// quarter, and below it a red left half and a blue right half, so a
// screenshot shows how it is scaled.
func photo() *ui.DivElement {
	return ui.Div().Key("photo").Matched("photo").Column().Radius(6).ClipContent().Background(ui.Hex("#222")).
		Child(ui.Div().Key("photo-band").Grow(1).Background(green)).
		Child(ui.Div().Row().Grow(3).
			Child(ui.Div().Key("photo-left").Grow(1).Background(red)).
			Child(ui.Div().Key("photo-right").Grow(1).Background(blue)))
}

// zoomApp shows photo as an 80 × 60 tile at (10, 10) of a 400 × 300 window,
// or, when zoom is set, as a 320 × 240 picture at (40, 30). grid and stage
// change the containers it is in; tile and big change the tile itself.
type zoomApp struct {
	zoom        *ui.Signal[bool]
	grid, stage func(*ui.DivElement) *ui.DivElement
	tile, big   func(*ui.DivElement) *ui.DivElement
}

func (a zoomApp) root(ctx *ui.Context) ui.Element {
	id := func(d *ui.DivElement) *ui.DivElement { return d }
	grid, stage, tile, big := id, id, id, id
	if a.grid != nil {
		grid = a.grid
	}
	if a.stage != nil {
		stage = a.stage
	}
	if a.tile != nil {
		tile = a.tile
	}
	if a.big != nil {
		big = a.big
	}
	root := ui.Div().Size(ui.Px(400), ui.Px(300))
	if a.zoom.Get() {
		root.Child(stage(ui.Div().Key("stage").Absolute().Left(40).Top(30).Size(ui.Px(320), ui.Px(240)).
			Child(big(photo().Size(ui.Px(320), ui.Px(240))))))
	} else {
		root.Child(grid(ui.Div().Key("grid").Absolute().Left(10).Top(10).
			Child(tile(photo().Size(ui.Px(80), ui.Px(60))))))
	}
	return root
}

func newZoom(t *testing.T, a zoomApp, opts ...uitest.Option) *uitest.UI {
	t.Helper()
	if a.zoom == nil {
		a.zoom = ui.NewSignal(false)
	}
	u := uitest.New(t, a.root, append([]uitest.Option{uitest.Size(400, 300)}, opts...)...)
	u.Settle()
	return u
}

// boundsPath records the bounds of the element keyed key at every frame of d.
func boundsPath(u *uitest.UI, key string, d time.Duration) []ui.Rect {
	var out []ui.Rect
	for end := u.Now().Add(d); u.Now().Before(end); {
		u.Advance(16 * time.Millisecond)
		out = append(out, u.Get(key).Bounds())
	}
	return out
}

func nearRect(a, b ui.Rect, eps float32) bool {
	d := func(x, y float32) bool { return x-y <= eps && y-x <= eps }
	return d(a.X, b.X) && d(a.Y, b.Y) && d(a.W, b.W) && d(a.H, b.H)
}

// When the tile leaves the grid and the picture comes on the stage, the
// picture is drawn where the tile was on the frame of the change, flies to
// its own place and size over many frames, never passing them, and settles
// there exactly. Nothing pops or jumps.
func TestMatchedFliesFromOldPlace(t *testing.T) {
	zoom := ui.NewSignal(false)
	u := newZoom(t, zoomApp{zoom: zoom})
	tile := ui.Rect{X: 10, Y: 10, W: 80, H: 60}
	if b := u.Get("photo").Bounds(); b != tile {
		t.Fatalf("the tile is at %v, want %v", b, tile)
	}
	rec := u.Record()
	zoom.Set(true)
	u.Frame()
	if b := u.Get("photo").Bounds(); !nearRect(b, tile, 0.01) {
		t.Fatalf("on the frame of the change the picture is drawn at %v, want where the tile was, %v", b, tile)
	}
	u.Advance(16 * time.Millisecond)
	if st := u.Runtime().Stats(); st.LaidOut || !st.Painted {
		t.Errorf("a frame of the flight laid out %v and painted %v, want paint only", st.LaidOut, st.Painted)
	}
	path := boundsPath(u, "photo", time.Second)
	for i := 1; i < len(path); i++ {
		p, q := path[i-1], path[i]
		if q.W < p.W || q.H < p.H || q.X < p.X || q.Y < p.Y {
			t.Fatalf("the picture goes from %v to %v in one frame (path %v), want it to grow and move steadily", p, q, path)
		}
		if q.W-p.W > 40 {
			t.Fatalf("the picture grows from %v to %v in one frame, want a glide", p.W, q.W)
		}
	}
	big := ui.Rect{X: 40, Y: 30, W: 320, H: 240}
	frames := 0
	for frames < len(path) && path[frames] != big {
		frames++
	}
	if frames < 10 {
		t.Errorf("the picture reached its place after %d frames (path %v), want 10 or more", frames, path)
	}
	if !u.Runtime().Settled() {
		t.Errorf("the UI has not settled a second after the change")
	}
	if b := u.Get("photo").Bounds(); b != big {
		t.Errorf("the picture rests at %v, want %v", b, big)
	}
	// Its children keep their final layout once it has landed.
	if b := u.Get("photo-right").Bounds(); b != (ui.Rect{X: 200, Y: 90, W: 160, H: 180}) {
		t.Errorf("the right half rests at %v, want (200, 90, 160, 180)", b)
	}
	rec.Check(t)
	if w := u.Warnings(); len(w) > 0 {
		t.Errorf("warnings: %v", w)
	}
}

// pixel returns the color of the screenshot at logical (x, y).
func pixel(img *image.RGBA, x, y int) color.RGBA { return img.RGBAAt(x, y) }

func isColor(c color.RGBA, want ui.Color) bool {
	r, g, b := int(want.R*255+0.5), int(want.G*255+0.5), int(want.B*255+0.5)
	d := func(a uint8, b int) bool { return int(a)-b <= 8 && b-int(a) <= 8 }
	return d(c.R, r) && d(c.G, g) && d(c.B, b)
}

// A flight scales the element's final layout uniformly to cover its frame,
// centered, and clips it to the frame: a 4:3 picture flying from a square tile
// shows its middle, undistorted, and nothing of it outside the tile.
func TestMatchedScalesToCoverItsFrame(t *testing.T) {
	zoom := ui.NewSignal(false)
	// The tile is square, 80 × 80: the picture (320 × 240) covers it at a
	// scale of 1/3, as 106.7 × 80 centered on (50, 50), so its band is the
	// top 20 px, its halves meet at x = 50, and 13.3 px of each side are
	// clipped.
	u := newZoom(t, zoomApp{zoom: zoom, tile: func(d *ui.DivElement) *ui.DivElement { return d.Size(ui.Px(80), ui.Px(80)) }})
	bg := pixel(u.Screenshot(), 395, 295)
	zoom.Set(true)
	u.Frame()
	img := u.Screenshot()
	for _, c := range []struct {
		x, y int
		want string
	}{
		{12, 50, "red"}, {47, 50, "red"}, {53, 50, "blue"}, {88, 50, "blue"},
		{30, 12, "green"}, {70, 27, "green"}, {30, 33, "red"}, {70, 88, "blue"},
		{7, 50, "background"}, {93, 50, "background"}, {50, 7, "background"}, {50, 93, "background"},
		// The frame's corners are rounded, though the picture clips its
		// content to its own box, which the flight scales past the frame.
		{10, 10, "background"}, {89, 89, "background"},
	} {
		got := pixel(img, c.x, c.y)
		ok := false
		switch c.want {
		case "red":
			ok = isColor(got, red)
		case "blue":
			ok = isColor(got, blue)
		case "green":
			ok = isColor(got, green)
		default:
			ok = got == bg
		}
		if !ok {
			t.Errorf("at (%d, %d) the first frame of the flight draws %v, want %s", c.x, c.y, got, c.want)
		}
	}
	// Mid-flight the frame is between the two places and still covered with
	// both halves, meeting at its center.
	u.Advance(120 * time.Millisecond)
	f := u.Get("photo").Bounds()
	if f.W <= 80 || f.W >= 320 {
		t.Fatalf("mid-flight the frame is %v, want between the tile and the picture", f)
	}
	img = u.Screenshot()
	mx, my := int(f.X+f.W/2), int(f.Y+f.H/2)
	if !isColor(pixel(img, mx-3, my), red) || !isColor(pixel(img, mx+3, my), blue) {
		t.Errorf("mid-flight the halves do not meet at the frame's center (%d, %d): %v | %v", mx, my, pixel(img, mx-3, my), pixel(img, mx+3, my))
	}
	if !isColor(pixel(img, int(f.X)+3, my), red) || !isColor(pixel(img, int(f.X+f.W)-3, my), blue) {
		t.Errorf("mid-flight the frame %v is not covered to its edges", f)
	}
	u.Settle()
}

// A flight is drawn above the rest of the window and outside its ancestors'
// opacity: the stage it lands in fades in from nothing, and a panel drawn
// after it in tree order overlaps its path, yet the picture is drawn at full
// opacity, above the panel. The tile's Exit does not play: the picture
// replaces it.
func TestMatchedPaintsAboveFadingAncestors(t *testing.T) {
	zoom := ui.NewSignal(false)
	a := zoomApp{zoom: zoom,
		stage: func(d *ui.DivElement) *ui.DivElement {
			return d.Enter(ui.Fade(), ui.Animation{Duration: 300 * time.Millisecond})
		},
		grid: func(d *ui.DivElement) *ui.DivElement {
			return d.Exit(ui.Fade(), ui.Animation{Duration: 300 * time.Millisecond})
		},
		tile: func(d *ui.DivElement) *ui.DivElement {
			return d.Exit(ui.Fade(), ui.Animation{Duration: 300 * time.Millisecond})
		},
	}
	root := func(ctx *ui.Context) ui.Element {
		return ui.Div().Child(a.root(ctx)).
			Child(ui.Div().Key("panel").Absolute().Left(0).Top(0).Size(ui.Px(120), ui.Px(100)).Background(ui.Hex("#808080")))
	}
	u := uitest.New(t, root, uitest.Size(400, 300))
	u.Settle()
	rec := u.Record()
	zoom.Set(true)
	u.Frame()
	if o := u.Get("photo").Style().Opacity; o < 0.99 {
		t.Errorf("the picture is drawn at opacity %v on the first frame, inside a stage that fades in from 0; want 1", o)
	}
	img := u.Screenshot()
	if c := pixel(img, 30, 40); !isColor(c, red) {
		t.Errorf("under the panel, the picture's left half is drawn as %v, want red: the flight is drawn above the panel", c)
	}
	u.Advance(64 * time.Millisecond)
	if o := u.Get("photo").Style().Opacity; o < 0.6 {
		t.Errorf("a few frames in, the picture is at opacity %v while the stage fades in; want near 1", o)
	}
	for _, f := range rec.Frames() {
		for _, s := range f.Elements {
			if s.Exiting && s.Matched == "photo" {
				t.Fatalf("the tile was drawn as exiting: its Exit played, though the picture replaced it")
			}
		}
	}
	u.Settle()
	rec.Check(t)
}

// A flight keeps the rectangular clips of where it is going: a picture that
// lands in a scroll viewport is drawn only inside the viewport, even where it
// starts outside it.
func TestMatchedKeepsDestinationClips(t *testing.T) {
	zoom := ui.NewSignal(false)
	root := func(ctx *ui.Context) ui.Element {
		root := ui.Div().Size(ui.Px(400), ui.Px(300))
		if zoom.Get() {
			root.Child(ui.Div().Key("viewport").Absolute().Left(150).Top(150).Size(ui.Px(200), ui.Px(100)).
				ScrollY().
				Child(photo().Size(ui.Px(200), ui.Px(150))))
		} else {
			root.Child(ui.Div().Absolute().Left(10).Top(10).Child(photo().Size(ui.Px(80), ui.Px(60))))
		}
		return root
	}
	u := uitest.New(t, root, uitest.Size(400, 300))
	u.Settle()
	bg := pixel(u.Screenshot(), 395, 5)
	zoom.Set(true)
	u.Frame()
	img := u.Screenshot()
	if c := pixel(img, 40, 40); c != bg {
		t.Errorf("the flight starts outside its viewport, and is drawn there: %v at (40, 40)", c)
	}
	for range 40 {
		u.Advance(16 * time.Millisecond)
		f := u.Get("photo").Bounds()
		img := u.Screenshot()
		// Above the viewport, inside the frame, nothing of it is drawn.
		if f.Y < 145 && f.X+f.W > 160 && f.X < 340 {
			x := int(max(f.X, 151)) + 4
			y := int(max(f.Y, 0)) + 1
			if y < 148 && x < 348 {
				if c := pixel(img, x, y); c != bg {
					t.Fatalf("the flight %v is drawn at (%d, %d), above its viewport, as %v", f, x, y, c)
				}
			}
		}
	}
	u.Settle()
	if c := pixel(u.Screenshot(), 160, 200); !isColor(c, red) {
		t.Errorf("once landed, the picture's left half is drawn as %v, want red", c)
	}
	if c := pixel(u.Screenshot(), 160, 255); c != bg {
		t.Errorf("once landed, the picture's part below the viewport is drawn: %v", c)
	}
}

// Matching back mid-flight starts from where the element is drawn, with its
// velocity: on the frame of the change it stays where it was, and on the next
// it goes on growing a little before it turns back to the tile.
func TestMatchedInterruptionKeepsVelocity(t *testing.T) {
	zoom := ui.NewSignal(false)
	u := newZoom(t, zoomApp{zoom: zoom})
	rec := u.Record()
	zoom.Set(true)
	u.Frame()
	u.Advance(80 * time.Millisecond)
	w1 := u.Get("photo").Bounds().W
	u.Advance(16 * time.Millisecond)
	b2 := u.Get("photo").Bounds()
	if !(b2.W > w1 && w1 > 80 && b2.W < 320) {
		t.Fatalf("growing: width %v then %v, want rising between 80 and 320", w1, b2.W)
	}
	zoom.Set(false)
	u.Frame()
	if b := u.Get("photo").Bounds(); !nearRect(b, b2, 0.01) {
		t.Errorf("on the frame it is matched back the tile is at %v, want where the picture was, %v", b, b2)
	}
	u.Advance(16 * time.Millisecond)
	w3 := u.Get("photo").Bounds().W
	if w3 <= b2.W {
		t.Errorf("one frame after it is matched back the width is %v, want still growing past %v (the velocity is kept)", w3, b2.W)
	}
	if w3-b2.W > 2*(b2.W-w1) {
		t.Errorf("one frame after it is matched back the width jumped from %v to %v", b2.W, w3)
	}
	u.Settle()
	if b := u.Get("photo").Bounds(); b != (ui.Rect{X: 10, Y: 10, W: 80, H: 60}) {
		t.Errorf("the tile rests at %v, want (10, 10, 80, 60)", b)
	}
	rec.Check(t)
}

// With reduced motion nothing flies: the picture is at its place at once, the
// tile is gone, and the UI is at rest.
func TestMatchedReducedMotion(t *testing.T) {
	zoom := ui.NewSignal(false)
	u := newZoom(t, zoomApp{zoom: zoom}, uitest.Settings(ui.SystemSettings{ReducedMotion: true}))
	zoom.Set(true)
	u.Frame()
	if b := u.Get("photo").Bounds(); b != (ui.Rect{X: 40, Y: 30, W: 320, H: 240}) {
		t.Errorf("with reduced motion the picture is at %v, want its place at once", b)
	}
	if !u.Runtime().Settled() {
		t.Errorf("with reduced motion the UI is not settled after the change")
	}
}

// With reduced motion the old element exits and the new one enters by their
// own Exit and Enter, in place, when the theme lets them play.
func TestMatchedReducedMotionExitsAndEnters(t *testing.T) {
	zoom := ui.NewSignal(false)
	fade := ui.Animation{Duration: 200 * time.Millisecond}
	a := zoomApp{zoom: zoom,
		tile: func(d *ui.DivElement) *ui.DivElement { return d.Exit(ui.Fade(), fade) },
		big:  func(d *ui.DivElement) *ui.DivElement { return d.Enter(ui.Fade(), fade) },
	}
	u := newZoom(t, a, uitest.Settings(ui.SystemSettings{ReducedMotion: true}))
	rec := u.Record()
	zoom.Set(true)
	u.Frame()
	if b := u.Get("photo").Bounds(); b != (ui.Rect{X: 40, Y: 30, W: 320, H: 240}) {
		t.Errorf("with reduced motion the picture is at %v, want its place", b)
	}
	f := rec.Frames()[len(rec.Frames())-1]
	for _, s := range f.Elements {
		if s.Matched == "photo" && s.Bounds.W == 80 && !s.Exiting {
			t.Errorf("with reduced motion the tile is still drawn in place, not exiting")
		}
	}
}

// Two elements drawn at once with one id: the runtime warns, once, and the
// last in tree order is the one matched.
func TestMatchedDuplicateIDWarnsAndLastWins(t *testing.T) {
	zoom := ui.NewSignal(false)
	root := func(ctx *ui.Context) ui.Element {
		root := ui.Div().Size(ui.Px(400), ui.Px(300))
		if zoom.Get() {
			root.Child(ui.Div().Absolute().Left(40).Top(30).Child(photo().Size(ui.Px(320), ui.Px(240))))
		} else {
			root.Child(ui.Div().Absolute().Left(10).Top(10).Row().Gap(10).
				Child(ui.Div().Key("first").Matched("photo").Size(ui.Px(80), ui.Px(60)).Background(red)).
				Child(ui.Div().Key("second").Matched("photo").Size(ui.Px(80), ui.Px(60)).Background(blue)))
		}
		return root
	}
	u := uitest.New(t, root, uitest.Size(400, 300))
	u.Settle()
	u.Advance(100 * time.Millisecond) // more paints: still one warning
	if w := u.Warnings(); len(w) != 1 || !strings.Contains(w[0], `"photo"`) {
		t.Fatalf("warnings %q, want one about the shared id \"photo\"", w)
	}
	zoom.Set(true)
	u.Frame()
	if b := u.Get("photo").Bounds(); !nearRect(b, ui.Rect{X: 100, Y: 10, W: 80, H: 60}, 0.01) {
		t.Errorf("the picture starts at %v, want where the second (last) tile was, (100, 10, 80, 60)", b)
	}
	u.Settle()
}

// Hit testing, Find and the accessibility tree follow the element where its
// flight draws it.
func TestMatchedHitTestingFollowsTheFlight(t *testing.T) {
	zoom := ui.NewSignal(false)
	clicks := 0
	a := zoomApp{zoom: zoom, big: func(d *ui.DivElement) *ui.DivElement {
		return d.Label("Picture").OnClick(func() { clicks++ })
	}}
	u := newZoom(t, a)
	zoom.Set(true)
	u.Frame()
	u.Advance(64 * time.Millisecond)
	f := u.Get("photo").Bounds()
	if f.X+f.W >= 360 {
		t.Fatalf("the flight %v has landed already", f)
	}
	acc := u.ByName("Picture").Bounds()
	if !nearRect(acc, f, 1) {
		t.Errorf("the accessibility bounds are %v, want the flight's frame %v", acc, f)
	}
	u.Get("photo").Click() // at the frame's center, which its laid-out box does not hold
	if clicks != 1 {
		t.Errorf("a click at the center of the flight ran OnClick %d times, want 1", clicks)
	}
	u.Settle()
}

// A rebuild that leaves the picture's layout alone, mid-flight, does not
// start its flight again.
func TestMatchedRebuildKeepsFlight(t *testing.T) {
	zoom := ui.NewSignal(false)
	tick := ui.NewSignal(0)
	a := zoomApp{zoom: zoom}
	root := func(ctx *ui.Context) ui.Element {
		_ = tick.Get()
		return a.root(ctx)
	}
	u := uitest.New(t, root, uitest.Size(400, 300))
	u.Settle()
	zoom.Set(true)
	u.Frame()
	u.Advance(64 * time.Millisecond)
	b1 := u.Get("photo").Bounds()
	tick.Set(1)
	u.Advance(16 * time.Millisecond)
	if b := u.Get("photo").Bounds(); b.W <= b1.W || b.W-b1.W > 40 {
		t.Errorf("after a rebuild mid-flight the width went from %v to %v, want the flight to go on", b1.W, b.W)
	}
	u.Settle()
}

// A button with a Matched id flies as a div does, and so do a text, an icon
// and an image inside a div with one: from where the old one was, to their
// own place.
func TestMatchedElementKinds(t *testing.T) {
	pic := ui.ImageFromImage(image.NewRGBA(image.Rect(0, 0, 4, 3)))
	for _, tc := range []struct {
		name string
		mk   func(big bool) ui.Element
	}{
		{"text", func(big bool) ui.Element { return ui.Div().Key("el").Matched("el").Child(ui.Text("Lake")) }},
		{"icon", func(big bool) ui.Element { return ui.Div().Key("el").Matched("el").Child(ui.Icon(icons.Check)) }},
		{"image", func(big bool) ui.Element {
			if big {
				return ui.Div().Key("el").Matched("el").Child(ui.Image(pic).Size(ui.Px(160), ui.Px(120)))
			}
			return ui.Div().Key("el").Matched("el").Child(ui.Image(pic).Size(ui.Px(40), ui.Px(30)))
		}},
		{"button", func(big bool) ui.Element { return ui.Button("Open").Key("el").Matched("el") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			moved := ui.NewSignal(false)
			u := uitest.New(t, func(ctx *ui.Context) ui.Element {
				root := ui.Div().Size(ui.Px(400), ui.Px(300))
				if moved.Get() {
					return root.Child(ui.Div().Key("there").Absolute().Left(200).Top(150).Child(tc.mk(true)))
				}
				return root.Child(ui.Div().Key("here").Absolute().Left(10).Top(10).Child(tc.mk(false)))
			}, uitest.Size(400, 300))
			u.Settle()
			from := u.Get("el").Bounds()
			moved.Set(true)
			u.Frame()
			if b := u.Get("el").Bounds(); !nearRect(b, from, 0.01) {
				t.Errorf("on the frame of the change it is at %v, want where it was, %v", b, from)
			}
			u.Advance(100 * time.Millisecond)
			if b := u.Get("el").Bounds(); b.X <= from.X || b.X >= 200 {
				t.Errorf("mid-flight it is at %v, want between %v and 200", b, from.X)
			}
			u.Settle()
			if b := u.Get("el").Bounds(); b.X != 200 || b.Y != 150 {
				t.Errorf("it rests at %v, want at (200, 150)", b)
			}
		})
	}
}

package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/internal/text"
)

// titleBarRig is a window whose top 60 px are a title bar: a drag area with a
// focusable field, a button and a minimize button inside it, and a plain body
// below.
func titleBarRig(t *testing.T) *Runtime {
	t.Helper()
	root := func(ctx *Context) Element {
		return Div().Children(
			Div().Key("bar").Size(Full, Px(60)).WindowArea(WindowAreaDrag).Children(
				Text("Title"),
				Div().Key("field").Absolute().Left(200).Top(10).Size(Px(40), Px(40)).Focusable(),
				Div().Key("button").Absolute().Left(300).Top(10).Size(Px(40), Px(40)).OnClick(func() {}),
				Div().Key("min").Absolute().Left(400).Top(0).Size(Px(46), Px(32)).WindowArea(WindowAreaMinimize),
			),
			Div().Key("body").Size(Full, Px(100)),
		)
	}
	return newTestRuntime(root, 800, 300, 1)
}

func TestWindowAreaDragsUnlessAnInteractiveChildIsHit(t *testing.T) {
	rt := titleBarRig(t)
	cases := []struct {
		name string
		x, y float32
		want WindowArea
	}{
		{"title bar background", 600, 20, WindowAreaDrag},
		{"title text", 10, 20, WindowAreaDrag},
		{"focusable child inside the drag area", 220, 20, WindowAreaNone},
		{"clickable child inside the drag area", 320, 20, WindowAreaNone},
		{"button area inside the drag area", 420, 10, WindowAreaMinimize},
		{"below the title bar", 100, 150, WindowAreaNone},
	}
	for _, c := range cases {
		if got := rt.hostWindowAreaAt(c.x, c.y); got != c.want {
			t.Errorf("%s at (%v, %v): WindowAreaAt = %v, want %v", c.name, c.x, c.y, got, c.want)
		}
	}
}

func TestWindowAreaIgnoresZoomLikePointerInput(t *testing.T) {
	rt := titleBarRig(t)
	rt.SetZoom(2)
	rt.Frame(time.Now())
	// The title bar is 60 logical pixels tall, so at zoom 2 it spans device
	// pixels 0..120; points are device pixels, as for PointerMove.
	if got := rt.hostWindowAreaAt(700, 80); got != WindowAreaDrag {
		t.Errorf("zoomed WindowAreaAt(700, 80) = %v, want WindowAreaDrag", got)
	}
	if got := rt.hostWindowAreaAt(700, 130); got != WindowAreaNone {
		t.Errorf("zoomed WindowAreaAt(700, 130) = %v, want WindowAreaNone", got)
	}
}

func TestWindowAreaNoneWithoutMarkers(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Size(Full, Px(60)).Child(Text("plain"))
	}, 300, 200, 1)
	if got := rt.hostWindowAreaAt(10, 10); got != WindowAreaNone {
		t.Errorf("an unmarked window: WindowAreaAt = %v, want WindowAreaNone", got)
	}
}

func TestWindowAreaOnlyPaintedElements(t *testing.T) {
	// A drag area that is scrolled out of view was not painted and must not
	// count, since its rectangle is stale.
	root := func(ctx *Context) Element {
		return Div().ScrollY().Size(Full, Px(100)).Child(
			Div().Size(Full, Px(400)).Children(
				Div().Size(Full, Px(60)).WindowArea(WindowAreaDrag),
				Div().Size(Full, Px(340)),
			))
	}
	rt := newTestRuntime(root, 300, 100, 1)
	if got := rt.hostWindowAreaAt(10, 10); got != WindowAreaDrag {
		t.Fatalf("before scrolling: %v, want WindowAreaDrag", got)
	}
	rt.Scroll(10, 10, 0, 300, true, 0)
	rt.Frame(time.Now())
	if got := rt.hostWindowAreaAt(10, 10); got != WindowAreaNone {
		t.Errorf("after scrolling the drag area out of view: %v, want WindowAreaNone", got)
	}
}

// fakeWindow records the actions a title bar performs on its window.
type fakeWindow struct {
	maximized, focused, fullscreen bool
	calls                          []string
	// The dialogs are recorded in calls; their callbacks are kept, so a test
	// answers a dialog by calling fileDone or msgDone.
	fileOpts []dialog.FileOptions
	fileDone func(dialog.FileResult)
	msgOpts  []dialog.MessageOptions
	msgDone  func(dialog.Button)
}

func (f *fakeWindow) Minimize()            { f.calls = append(f.calls, "minimize") }
func (f *fakeWindow) ToggleMaximize()      { f.calls = append(f.calls, "toggle") }
func (f *fakeWindow) Maximized() bool      { return f.maximized }
func (f *fakeWindow) Close()               { f.calls = append(f.calls, "close") }
func (f *fakeWindow) SetTitle(s string)    { f.calls = append(f.calls, "title:"+s) }
func (f *fakeWindow) Focused() bool        { return f.focused }
func (f *fakeWindow) Focus()               { f.calls = append(f.calls, "focus") }
func (f *fakeWindow) Closed() bool         { return false }
func (f *fakeWindow) Position() (int, int) { return 10, 20 }
func (f *fakeWindow) Size() (int, int)     { return 640, 480 }
func (f *fakeWindow) SetPosition(x, y int) {
	f.calls = append(f.calls, fmt.Sprintf("move:%d,%d", x, y))
}
func (f *fakeWindow) SetSize(w, h int) { f.calls = append(f.calls, fmt.Sprintf("size:%dx%d", w, h)) }
func (f *fakeWindow) SetFullscreen(on bool) {
	f.fullscreen = on
	f.calls = append(f.calls, fmt.Sprintf("fullscreen:%v", on))
}
func (f *fakeWindow) Fullscreen() bool { return f.fullscreen }

func (f *fakeWindow) OpenFiles(o dialog.FileOptions, done func(dialog.FileResult)) {
	f.calls = append(f.calls, "open")
	f.fileOpts, f.fileDone = append(f.fileOpts, o), done
}

func (f *fakeWindow) SaveFile(o dialog.FileOptions, done func(dialog.FileResult)) {
	f.calls = append(f.calls, "save")
	f.fileOpts, f.fileDone = append(f.fileOpts, o), done
}

func (f *fakeWindow) PickFolder(o dialog.FileOptions, done func(dialog.FileResult)) {
	f.calls = append(f.calls, "folder")
	f.fileOpts, f.fileDone = append(f.fileOpts, o), done
}

func (f *fakeWindow) MessageBox(o dialog.MessageOptions, done func(dialog.Button)) {
	f.calls = append(f.calls, "message")
	f.msgOpts, f.msgDone = append(f.msgOpts, o), done
}

func TestWindowStateIsReactive(t *testing.T) {
	win := &fakeWindow{}
	runs := 0
	var seen []bool
	rt := NewRuntime(func(ctx *Context) Element {
		runs++
		seen = append(seen, ctx.Window().Maximized())
		return Textf("maximized=%v", ctx.Window().Maximized())
	}, DarkTheme(), nil)
	rt.SetWindow(win)
	rt.SetViewport(300, 200, 1)
	rt.Frame(time.Now())

	// The app flips the state; the component that read it re-renders.
	win.maximized = true
	rt.hostSetMaximized(true)
	if !rt.NeedsFrame() {
		t.Fatal("changing the window state did not request a frame")
	}
	rt.Frame(time.Now())
	if runs != 2 || !seen[len(seen)-1] {
		t.Fatalf("after maximizing: renders=%d seen=%v; want a second render that sees maximized", runs, seen)
	}
	// Restoring it re-renders again, with the new value.
	win.maximized = false
	rt.hostSetMaximized(false)
	rt.Frame(time.Now())
	if runs != 3 || seen[len(seen)-1] {
		t.Errorf("after restoring: renders=%d seen=%v; want a third render that sees restored", runs, seen)
	}
}

func TestWindowFocusIsReactive(t *testing.T) {
	runs := 0
	rt := NewRuntime(func(ctx *Context) Element {
		runs++
		return Textf("focused=%v", ctx.Window().Focused())
	}, DarkTheme(), nil)
	rt.SetViewport(300, 200, 1)
	rt.Frame(time.Now())
	rt.hostWindowFocus(true)
	rt.Frame(time.Now())
	if runs != 2 {
		t.Fatalf("losing or gaining focus: renders=%d, want 2", runs)
	}
	rt.hostWindowFocus(false)
	rt.Frame(time.Now())
	if runs != 3 {
		t.Errorf("after blur: renders=%d, want 3", runs)
	}
}

func TestWindowActionsReachTheControls(t *testing.T) {
	win := &fakeWindow{}
	rt := NewRuntime(func(ctx *Context) Element {
		return Div().Child(Button("Min").OnClick(func() { ctx.Window().Minimize() })).
			Child(Button("Max").OnClick(func() { ctx.Window().ToggleMaximize() }))
	}, DarkTheme(), nil)
	rt.SetWindow(win)
	rt.SetViewport(300, 200, 1)
	rt.Frame(time.Now())
	ctx := &Context{rt: rt}
	ctx.Window().SetTitle("x")
	ctx.Window().Close()
	if got := len(win.calls); got != 2 || win.calls[0] != "title:x" || win.calls[1] != "close" {
		t.Errorf("calls = %v, want [title:x close]", win.calls)
	}
}

func TestHeadlessWindowDoesNothing(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme(), nil)
	ctx := &Context{rt: rt}
	ctx.Window().Minimize()
	ctx.Window().ToggleMaximize()
	ctx.Window().Close()
	ctx.Window().SetTitle("nothing")
	if ctx.Window().Maximized() || ctx.Window().Focused() {
		t.Error("a headless window reports maximized or focused by default")
	}
}

// Two runtimes (two windows) read the same signal; setting it re-renders the
// component in each, and wakes each runtime's own window.
func TestSharedSignalRerendersInEveryWindow(t *testing.T) {
	shared := NewSignal(0)
	ts := text.NewSystem() // all windows share one text system
	type window struct {
		rt   *Runtime
		runs int
		woke int
	}
	open := func() *window {
		w := &window{}
		w.rt = NewRuntime(func(ctx *Context) Element {
			w.runs++
			return Textf("v=%d", shared.Get())
		}, DarkTheme(), ts)
		w.rt.hostSetWakeFunc(func() { w.woke++ })
		w.rt.SetViewport(300, 200, 1)
		w.rt.Frame(time.Now())
		return w
	}
	a, b := open(), open()
	a.woke, b.woke = 0, 0

	shared.Set(7)
	if a.woke == 0 || b.woke == 0 {
		t.Fatalf("setting the signal woke a=%d b=%d windows; want both", a.woke, b.woke)
	}
	a.rt.Frame(time.Now())
	b.rt.Frame(time.Now())
	if a.runs != 2 || b.runs != 2 {
		t.Errorf("after Set: a rendered %d times, b %d; want 2 each", a.runs, b.runs)
	}
}

// The window's geometry goes through to the app's controls, and a headless
// runtime reports its viewport.
func TestWindowGeometryControls(t *testing.T) {
	var ctl Window
	r := newOverlayRig(func(ctx *Context) Element { ctl = ctx.Window(); return Div() }, 300, 200)
	if w, h := ctl.Size(); w != 300 || h != 200 {
		t.Fatalf("headless size %dx%d, want the viewport", w, h)
	}
	fw := &fakeWindow{}
	r.rt.SetWindow(fw)
	ctl.SetSize(800, 600)
	ctl.SetPosition(40, 50)
	ctl.SetFullscreen(true)
	if !ctl.Fullscreen() {
		t.Fatal("Fullscreen is false after SetFullscreen(true)")
	}
	if x, y := ctl.Position(); x != 10 || y != 20 {
		t.Fatalf("position %d,%d, want the window's", x, y)
	}
	want := []string{"size:800x600", "move:40,50", "fullscreen:true"}
	if fmt.Sprint(fw.calls) != fmt.Sprint(want) {
		t.Fatalf("calls %v, want %v", fw.calls, want)
	}
}

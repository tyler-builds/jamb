// Package app runs a jamb application: it opens native windows, creates the
// GPU renderer and drives one ui runtime per window.
//
//	func main() {
//	    app.New().Run(app.Window{
//	        Title: "Hello", Width: 800, Height: 600,
//	        Root: func(ctx *ui.Context) ui.Element { return ui.Text("Hello") },
//	    })
//	}
//
// Further windows open with Open, from event handlers or from goroutines. All
// windows share the UI thread, one text system and one GPU device; each has
// its own ui runtime, theme, zoom, focus and key bindings.
package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/internal/watch"
	"github.com/tyler-builds/jamb/keymap"
	"github.com/tyler-builds/jamb/platform"
	"github.com/tyler-builds/jamb/render"
	"github.com/tyler-builds/jamb/text"
	"github.com/tyler-builds/jamb/ui"
)

// processStart approximates process start for startup measurements.
var processStart = time.Now()

// cascade is how far a window opened without a position is offset from the
// last window, in logical pixels.
const cascade = 32

// Window describes an application window and its content.
type Window struct {
	Title     string
	Width     int // logical pixels; default 1024
	Height    int // logical pixels; default 720
	MinWidth  int
	MinHeight int
	// Theme defaults to ui.DarkTheme.
	Theme *ui.Theme
	// Root renders the whole window. It is a component: it re-runs when a
	// signal it reads changes.
	Root func(ctx *ui.Context) ui.Element
	// Zoom is the initial UI zoom (0 = 100%). Users change it with Ctrl+=,
	// Ctrl+-, Ctrl+0 and Ctrl+wheel; OnZoom is called after each change so
	// the app can remember it.
	Zoom   float32
	OnZoom func(zoom float32)
	// Bindings maps keys to actions for the whole window; see ui.Bind.
	Bindings []ui.Binding
	// KeymapFile is a JSON keymap whose bindings come after Bindings, so they
	// override them; see keymap.Parse. A missing file is fine. The file is
	// reloaded when it changes, and the app keeps running with the bindings it
	// had if the change does not parse. Problems are printed to stderr, and
	// the valid bindings still apply. Actions lists the actions the file may
	// name.
	KeymapFile string
	Actions    []*ui.Action
	// OnKeymapLoad is called on the UI thread after the startup load and after
	// each reload that changed the bindings, with nil on success or the
	// problems. Use it to show errors in the app.
	OnKeymapLoad func(err error)

	// Chrome selects who draws the title bar: the operating system (the
	// default) or the app; see ChromeCustom.
	Chrome Chrome
	// X and Y place the window's top-left corner in logical screen pixels.
	// Both zero centers the first window, and cascades a later one 32 pixels
	// from the last window opened.
	X, Y int
	// OnClose is called when the window is asked to close, by the user, by
	// WindowHandle.Close or by the app. Return false to keep the window open,
	// for example to ask about unsaved changes first.
	OnClose func() bool
	// OnClosed is called on the UI thread after the window has closed, or
	// after Open failed to open it.
	OnClosed func()
	// QuitWhenClosed quits the app when this window closes, even while other
	// windows are open. Before it closes, the OnClose of every other window is
	// asked; if one returns false, nothing closes. Then the other windows close
	// too, each without being asked again. By default the app quits when the last
	// window closes.
	QuitWhenClosed bool
}

// Chrome selects who draws a window's title bar.
type Chrome uint8

const (
	// ChromeNative keeps the operating system's title bar and frame.
	ChromeNative Chrome = iota
	// ChromeCustom removes the operating system's title bar, and the app
	// draws its own. The window keeps its native resize borders, Aero Snap,
	// shadow and maximize behavior. The app marks its drag and button areas
	// with ui.DivElement.WindowArea; ui.TitleBarButtons draws the usual
	// buttons.
	ChromeCustom
)

// App is an application instance. A process has one App: its windows share
// the UI thread, the text system and the GPU device.
type App struct{}

// New creates an application.
func New() *App { return &App{} }

// Run opens the window and blocks until the app quits: when the last window
// closes, or when a QuitWhenClosed window closes. On failure it prints the
// error and exits with status 1. Use RunE to handle errors yourself.
func (a *App) Run(w Window) {
	if err := a.RunE(w); err != nil {
		fmt.Fprintln(os.Stderr, "jamb:", err)
		os.Exit(1)
	}
}

// RunE is Run returning an error instead of exiting. Call it once per process.
func (a *App) RunE(w Window) error {
	if w.Root == nil {
		return fmt.Errorf("app.Window.Root is nil; set it to a func(*ui.Context) ui.Element that returns your UI")
	}
	w = withDefaults(w)
	application.dev = loadDevOptions()
	application.gpu.adapter = gpuAdapter
	// Parse fonts and warm the shaper while the first window and GPU device
	// are being created; the first window's runtime picks the result up.
	ready := make(chan *text.System, 1)
	application.textReady = ready
	go func() {
		ts := text.NewSystem()
		ts.Warm(w.Theme.FontSize)
		ready <- ts
	}()
	return runPlatform(func() error {
		_, err := openWindow(w, nil)
		return err
	})
}

// Open opens another window. It is safe from event handlers and from
// goroutines: the window is opened on the UI thread, and it is ready once the
// returned handle's methods have an effect. The app keeps running while the
// window is open (see Window.QuitWhenClosed).
func Open(w Window) *WindowHandle {
	h := &WindowHandle{}
	postUI(func() {
		if w.Root == nil {
			openFailed(w, h, errors.New("Window.Root is nil"))
			return
		}
		if _, err := openWindow(withDefaults(w), h); err != nil {
			openFailed(w, h, err)
		}
	})
	return h
}

// openFailed reports a window Open could not open. The window never existed,
// so OnDestroy does not run; OnClosed is called here instead, so state the app
// keeps per open window (a badge, a handle in a map) is released.
func openFailed(w Window, h *WindowHandle, err error) {
	fmt.Fprintln(os.Stderr, "jamb: app.Open:", err)
	h.closed.Store(true)
	if w.OnClosed != nil {
		w.OnClosed()
	}
}

// WindowHandle controls a window opened with Open. Its methods may be called
// from any goroutine; each runs on the UI thread, in call order.
type WindowHandle struct {
	win    *host // set on the UI thread once the window exists
	closed atomic.Bool
}

// Close asks the window to close. Its OnClose may keep it open.
func (h *WindowHandle) Close() {
	h.onUI(func(w *host) { w.win.Close() })
}

// Focus brings the window to the front and gives it keyboard focus.
func (h *WindowHandle) Focus() {
	h.onUI(func(w *host) { w.win.Focus() })
}

// SetTitle changes the window's title.
func (h *WindowHandle) SetTitle(title string) {
	h.onUI(func(w *host) { w.win.SetTitle(title) })
}

// Closed reports whether the window has closed, or failed to open.
func (h *WindowHandle) Closed() bool { return h.closed.Load() }

func (h *WindowHandle) onUI(fn func(*host)) {
	postUI(func() {
		if h.win != nil {
			fn(h.win)
		}
	})
}

// application is the state of the process's App. It is only used on the UI
// thread, except textReady, which the goroutine started by RunE fills once.
var application struct {
	dev       devOptions
	textReady chan *text.System
	text      *text.System // the shared text system, once received
	windows   []*host      // open windows, in the order they opened
	opened    int          // windows opened so far
	script    *script      // JAMB_SCRIPT, which drives the windows
	quitting  bool         // a QuitWhenClosed window is closing all windows
	gpu       gpuRecovery  // the shared GPU device, while it is lost; see recovery.go
}

func withDefaults(w Window) Window {
	if w.Width <= 0 {
		w.Width = 1024
	}
	if w.Height <= 0 {
		w.Height = 720
	}
	if w.Theme == nil {
		w.Theme = ui.DarkTheme
	}
	return w
}

// openWindow creates a window for cfg and shows it. handle, when set, is the
// Open handle that controls the window.
func openWindow(cfg Window, handle *WindowHandle) (*host, error) {
	h := &host{cfg: cfg, handle: handle, primary: application.opened == 0, dev: application.dev, openedAt: time.Now()}
	application.opened++
	opts := platform.WindowOptions{
		Title: cfg.Title, Width: cfg.Width, Height: cfg.Height,
		MinWidth: cfg.MinWidth, MinHeight: cfg.MinHeight, Dark: cfg.Theme.Dark,
		CustomTitleBar: cfg.Chrome == ChromeCustom,
	}
	if h.primary {
		opts.Frame = h.dev.loadGeometry()
	}
	if opts.Frame.W == 0 {
		opts.Origin = origin(cfg)
	}
	if _, err := openPlatform(opts, h); err != nil {
		return nil, err
	}
	return h, nil
}

// origin is where a window opened without a saved position goes: at X, Y if
// they are set, else cascaded from the last window. nil centers the window.
func origin(cfg Window) *platform.Point {
	if cfg.X != 0 || cfg.Y != 0 {
		return &platform.Point{X: cfg.X, Y: cfg.Y}
	}
	if n := len(application.windows); n > 0 {
		last := application.windows[n-1].win
		f, s := last.Frame(), last.Scale()
		return &platform.Point{X: int(float32(f.X)/s) + cascade, Y: int(float32(f.Y)/s) + cascade}
	}
	return nil
}

// sharedText returns the text system all windows use. The first call waits for
// the goroutine started by RunE; it reports whether it had to wait.
func sharedText() (*text.System, bool) {
	if application.text != nil {
		return application.text, false
	}
	application.text = <-application.textReady
	return application.text, true
}

// host is one window: it implements platform.Handler and drives the window's
// ui runtime and renderer.
type host struct {
	cfg      Window
	handle   *WindowHandle
	primary  bool // the first window of the process: it saves its geometry
	dev      devOptions
	openedAt time.Time

	win     platform.Window
	rt      *ui.Runtime
	r       render.Renderer
	focused bool

	keymapFile *keymap.File
	stopWatch  func() // stops the keymap file watcher, if one runs
	stress     *stress

	frames int

	traceFrames int
	traceSince  time.Time
	err         error
}

// OnCreate implements platform.Handler.
func (h *host) OnCreate(w platform.Window) error {
	t0 := time.Now()
	h.win = w
	if application.gpu.lost {
		// The device is rebuilt on a frame, not here; a window opened before
		// that cannot be given a swap chain yet.
		return errors.New("the GPU device was lost and has not been rebuilt yet; open the window again in a moment")
	}
	width, height := w.Size()
	r, err := newRenderer(w, width, height)
	if err != nil {
		return err
	}
	h.r = r
	desc := ""
	if d, ok := r.(interface{ Describe() string }); ok {
		desc = " (" + d.Describe() + ")"
	}
	h.dev.tracef("renderer ready in %v%s", time.Since(t0), desc)
	t1 := time.Now()
	ts, waited := sharedText()
	if waited {
		h.dev.tracef("fonts ready (waited %v after the renderer)", time.Since(t1))
	}
	h.rt = ui.NewRuntime(h.cfg.Root, h.cfg.Theme, ts)
	h.rt.SetWakeFunc(w.Wake)
	h.rt.SetServices(ui.Services{Clipboard: w.Clipboard, SetClipboard: w.SetClipboard, WakeAt: w.WakeAt})
	h.rt.SetWindowControls(windowControls{h})
	h.rt.SetAccessWarnings(h.dev.a11yWarn)
	h.applySettings()
	h.rt.SetMaximized(w.Maximized())
	h.rt.AddBindings(h.cfg.Bindings...)
	if h.cfg.KeymapFile != "" {
		h.keymapFile = &keymap.File{Path: h.cfg.KeymapFile, Actions: h.cfg.Actions}
		h.loadKeymap(true, 0)
		h.watchKeymap()
	}
	if h.cfg.Zoom > 0 {
		h.rt.SetZoom(h.cfg.Zoom)
	}
	h.rt.OnZoomChange(h.cfg.OnZoom)
	h.updateViewport(w)
	if h.primary {
		if h.dev.stress != "" {
			h.stress = newStress(h.dev.stress)
		}
		if h.dev.script != "" {
			application.script, err = parseScript(h.dev.script)
			if err != nil {
				return err
			}
		}
	}
	if h.handle != nil {
		h.handle.win = h
	}
	application.windows = append(application.windows, h)
	return nil
}

// keymapRetries is how many times a keymap file that could not be read is
// read again, keymapRetryDelay apart, before the failure is reported. An editor
// that is still writing the file can briefly refuse other readers, and no
// further change notification may follow.
const (
	keymapRetries    = 3
	keymapRetryDelay = 250 * time.Millisecond
)

// loadKeymap applies the keymap file on the UI thread. It reports the result
// at startup, and after a reload only if the bindings changed. attempt counts
// the retries of a file that could not be read.
func (h *host) loadKeymap(startup bool, attempt int) {
	changed, err := h.keymapFile.Apply(h.rt)
	var readErr *fs.PathError
	if errors.As(err, &readErr) && attempt < keymapRetries {
		time.AfterFunc(keymapRetryDelay, func() {
			h.rt.Post(func() { h.loadKeymap(startup, attempt+1) })
		})
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb:", err)
	}
	if (changed || startup) && h.cfg.OnKeymapLoad != nil {
		h.cfg.OnKeymapLoad(err)
	}
}

// watchKeymap reloads the keymap file when its directory changes. Without a
// watch the app still runs, with the bindings from startup.
func (h *host) watchKeymap() {
	dir := filepath.Dir(h.cfg.KeymapFile)
	stop, err := watch.Watch(dir, 100*time.Millisecond, func() {
		h.rt.Post(func() { h.loadKeymap(false, 0) })
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb: live reload of the keymap is off:", err)
		return
	}
	h.stopWatch = stop
}

func (h *host) updateViewport(w platform.Window) {
	width, height := w.Size()
	s := w.Scale()
	h.rt.SetViewport(float32(width)/s, float32(height)/s, s)
}

// OnEvent implements platform.Handler.
func (h *host) OnEvent(w platform.Window, e platform.Event) {
	s := w.Scale()
	x, y := e.X/s, e.Y/s
	switch e.Kind {
	case platform.EventResize, platform.EventScaleChanged:
		h.updateViewport(w)
		h.saveGeometry(w)
	case platform.EventMoved:
		h.saveGeometry(w)
	case platform.EventSettingsChanged:
		h.applySettings()
	case platform.EventWindowState:
		h.rt.SetMaximized(w.Maximized())
	case platform.EventFileDrop:
		h.rt.DropFiles(x, y, e.Paths)
	case platform.EventMouseMove:
		h.rt.PointerMove(x, y, e.Mods)
	case platform.EventMouseLeave:
		h.rt.PointerLeave()
	case platform.EventMouseDown:
		h.rt.PointerDown(x, y, e.Button, e.Clicks, e.Mods)
	case platform.EventMouseUp:
		h.rt.PointerUp(x, y, e.Button, e.Mods)
	case platform.EventScroll:
		h.rt.Scroll(x, y, e.DeltaX, e.DeltaY, e.Precise, e.Mods)
	case platform.EventKeyDown:
		h.rt.KeyDown(e.Key, e.Mods, e.Repeat)
	case platform.EventKeyUp:
		h.rt.KeyUp(e.Key, e.Mods)
	case platform.EventTextInput:
		h.rt.TextInput(e.Text)
	case platform.EventFocus:
		h.focused = true
		h.rt.WindowFocus(true)
	case platform.EventBlur:
		h.focused = false
		h.rt.WindowFocus(false)
	}
	w.SetCursor(h.rt.Cursor())
	if h.rt.NeedsFrame() {
		w.Wake()
	}
}

// saveGeometry remembers the first window's position and size for the next run.
func (h *host) saveGeometry(w platform.Window) {
	if h.primary {
		h.dev.saveGeometry(w.Frame())
	}
}

// OnClose implements platform.Handler.
// OnClose implements platform.Handler. A QuitWhenClosed window asks the OnClose
// of every other open window first. If one of them keeps its window open, the
// whole close is cancelled, this window included. Otherwise the other windows
// close too, without being asked again.
func (h *host) OnClose(w platform.Window) bool {
	if application.quitting {
		return true
	}
	if h.cfg.QuitWhenClosed {
		for _, o := range application.windows {
			if o != h && o.cfg.OnClose != nil && !o.cfg.OnClose() {
				return false
			}
		}
	}
	if h.cfg.OnClose != nil && !h.cfg.OnClose() {
		return false
	}
	if h.cfg.QuitWhenClosed {
		application.quitting = true
		for _, o := range application.windows {
			if o != h {
				o.win.Close()
			}
		}
	}
	return true
}

// HasMenuBar implements platform.MenuBarHandler.
func (h *host) HasMenuBar() bool { return h.rt != nil && h.rt.HasMenuBar() }

// AccessBridge implements platform.AccessHandler. It is nil until the runtime
// exists, which the adapter treats as no accessibility support yet.
func (h *host) AccessBridge() *access.Bridge {
	if h.rt == nil {
		return nil
	}
	return h.rt.AccessBridge()
}

// TitleArea implements platform.TitleBarHandler: it maps a point in physical
// client pixels to the title-bar area the ui marked there.
func (h *host) TitleArea(w platform.Window, x, y float32) platform.TitleArea {
	if h.rt == nil {
		return platform.TitleNone
	}
	s := w.Scale()
	switch h.rt.WindowAreaAt(x/s, y/s) {
	case ui.AreaDrag:
		return platform.TitleDrag
	case ui.AreaMinimize:
		return platform.TitleMinimize
	case ui.AreaMaximize:
		return platform.TitleMaximize
	case ui.AreaClose:
		return platform.TitleClose
	}
	return platform.TitleNone
}

// OnFrame implements platform.Handler.
func (h *host) OnFrame(w platform.Window) bool {
	width, height := w.Size()
	if width <= 0 || height <= 0 {
		return false // minimized
	}
	h.updateViewport(w)
	frameStart := time.Now()
	if !h.gpuReady(frameStart) {
		return false // the device is lost; the window is woken when it may draw
	}
	if h.stress != nil {
		h.stress.begin(h, h.dev.stressSecs)
	}
	sc := h.rt.Frame(frameStart)
	// Tell the OS where the caret is, for input-method (IME) windows.
	var ime platform.Rect
	if r, ok := h.rt.TextInputRect(); ok {
		s := w.Scale()
		ime = platform.Rect{X: int(r.X * s), Y: int(r.Y * s), W: int(r.W*s + 1), H: int(r.H * s)}
	}
	w.SetTextInputRect(ime)
	script := application.script
	if script != nil {
		script.beforeDraw(h, w)
	}
	t0 := time.Now()
	if err := render.Draw(h.r, sc, width, height); err != nil {
		if errors.Is(err, render.ErrDeviceLost) {
			loseGPU(err)
			return false
		}
		if h.err == nil {
			fmt.Fprintln(os.Stderr, "jamb: rendering failed:", err)
		}
		h.err = err
		return false
	}
	h.frames++
	if h.frames == 1 {
		if h.primary {
			h.dev.tracef("first frame presented %v after process start", time.Since(processStart))
			h.dev.reportReady()
		} else {
			h.dev.tracef("window opened: first frame presented %v after Open", time.Since(h.openedAt))
		}
	}
	again := h.rt.NeedsFrame()
	if h.dev.trace {
		st := h.rt.Stats()
		if st.Built || st.LaidOut {
			h.dev.tracef("frame %d: nodes=%d rendered=%d build=%v layout=%v paint=%v gpu-submit=%v quads=%d glyphs=%d",
				h.frames, st.Nodes, st.Rendered, st.BuildTime, st.LayoutTime, st.PaintTime, time.Since(t0), st.Quads, st.Glyphs)
		}
		// Once per second of activity, say how many frames were rendered and
		// why the loop is still awake, to make idle-CPU problems obvious.
		h.traceFrames++
		if h.traceSince.IsZero() {
			h.traceSince = time.Now()
		}
		if time.Since(h.traceSince) >= time.Second {
			h.dev.tracef("%d frames in the last %.1fs (still animating: %v)", h.traceFrames, time.Since(h.traceSince).Seconds(), again)
			h.traceFrames, h.traceSince = 0, time.Now()
		}
	}
	if h.stress != nil {
		// Present() blocks for vsync; measure only the CPU side of the frame.
		return h.stress.end(h, w, cpuTime(frameStart, t0, h))
	}
	if script != nil {
		again = script.afterDraw(h, w) || again
	}
	return again
}

// cpuTime is the frame's CPU work: runtime stages plus command submission,
// excluding time spent blocked in Present.
func cpuTime(frameStart, submitStart time.Time, h *host) time.Duration {
	st := h.rt.Stats()
	return st.BuildTime + st.LayoutTime + st.PaintTime + time.Since(submitStart) - h.presentWait()
}

func (h *host) presentWait() time.Duration {
	if p, ok := h.r.(interface{ LastPresentWait() time.Duration }); ok {
		return p.LastPresentWait()
	}
	return 0
}

// OnDestroy implements platform.Handler. The app quits when its last window
// closes, or when a QuitWhenClosed window does.
func (h *host) OnDestroy(w platform.Window) {
	if h.r != nil {
		h.r.Close()
		h.r = nil
	}
	if h.stopWatch != nil {
		h.stopWatch()
		h.stopWatch = nil
	}
	application.windows = slices.DeleteFunc(application.windows, func(o *host) bool { return o == h })
	if h.handle != nil {
		h.handle.closed.Store(true)
		h.handle.win = nil
	}
	if h.cfg.OnClosed != nil {
		h.cfg.OnClosed()
	}
	// While a QuitWhenClosed window is closing the others, the app quits with the
	// last of them.
	if len(application.windows) == 0 || (h.cfg.QuitWhenClosed && !application.quitting) {
		quitUI()
	}
}

// windowControls adapts a host to ui.WindowControls.
type windowControls struct{ h *host }

func (c windowControls) Minimize()         { c.h.win.Minimize() }
func (c windowControls) ToggleMaximize()   { c.h.win.ToggleMaximize() }
func (c windowControls) Maximized() bool   { return c.h.win.Maximized() }
func (c windowControls) Close()            { c.h.win.Close() }
func (c windowControls) SetTitle(s string) { c.h.win.SetTitle(s) }
func (c windowControls) Focused() bool     { return c.h.focused }

func (c windowControls) OpenFiles(o dialog.FileOptions, done func(dialog.FileResult)) {
	c.h.win.ShowFileDialog(dialog.Open, o, done)
}

func (c windowControls) SaveFile(o dialog.FileOptions, done func(dialog.FileResult)) {
	c.h.win.ShowFileDialog(dialog.Save, o, done)
}

func (c windowControls) PickFolder(o dialog.FileOptions, done func(dialog.FileResult)) {
	c.h.win.ShowFileDialog(dialog.Folder, o, done)
}

func (c windowControls) MessageBox(o dialog.MessageOptions, done func(dialog.Button)) {
	c.h.win.ShowMessageBox(o, done)
}

// FrameWaitable implements platform.FrameWaiter: the window's swap chain
// signals when it can take another frame, so the adapter renders it then.
func (h *host) FrameWaitable() uintptr {
	if application.gpu.lost {
		return 0 // a lost chain may never signal; OnFrame waits for the rebuild instead
	}
	if wr, ok := h.r.(interface{ FrameWaitable() uintptr }); ok {
		return wr.FrameWaitable()
	}
	return 0
}

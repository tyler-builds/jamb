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
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/internal/watch"
	"github.com/tyler-builds/jamb/keymap"
	"github.com/tyler-builds/jamb/ui"
)

// processStart approximates process start for startup measurements.
var processStart = time.Now()

// cascade is how far a window opened without a position is offset from the
// last window, in logical pixels.
const cascade = 32

// Window describes an application window and its content.
type Window struct {
	// Title is the title of the window, which its title bar and taskbar button show.
	Title  string
	Width  int // logical pixels; default 1024
	Height int // logical pixels; default 720
	// MinWidth is the smallest width that the user can resize the window to, in logical pixels.
	MinWidth int
	// MinHeight is the smallest height that the user can resize the window to, in logical pixels.
	MinHeight int
	// Theme pins the window to one theme. Without it, the window follows the
	// system's app mode (light or dark), live: it uses DarkTheme while Windows
	// is in dark mode and LightTheme otherwise, by default ui.DarkTheme() and
	// ui.LightTheme(). ctx.SetTheme also pins one theme.
	Theme *ui.Theme
	// LightTheme is the theme used in light mode; by default it is ui.LightTheme().
	LightTheme *ui.Theme
	// DarkTheme is the theme used in dark mode; by default it is ui.DarkTheme().
	DarkTheme *ui.Theme
	// Strings is the text that the built-in controls show and announce. Without it,
	// the window uses the display language of Windows (see ui.SystemStrings), and
	// English where that language has no translation or cannot be read. Give
	// ui.StringsFor(tag) to choose a language, or start from ui.EnglishStrings() and
	// set the messages to change. A field left empty shows English (see ui.Strings).
	Strings *ui.Strings
	// Root renders the whole window. It is a component: it re-runs when a
	// signal it reads changes.
	Root func(ctx *ui.Context) ui.Element
	// Zoom is the initial UI zoom (0 = 100%). Users change it with Ctrl+=,
	// Ctrl+-, Ctrl+0 and Ctrl+wheel; OnZoom is called after each change so
	// the app can remember it.
	Zoom float32
	// OnZoom is called after each change of the zoom, with the new zoom.
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
	// Actions lists the actions that KeymapFile may name.
	Actions []*ui.Action
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
	// OnClose is called when the window is asked to close, by the user, by Close on
	// the ui.Window that Open returns, or by the app. Return false to keep the window
	// open, for example to ask about unsaved changes first.
	OnClose func() bool
	// OnClosed is called on the UI thread after the window has closed, or
	// after Open failed to open it.
	OnClosed func()
	// OnGeometry is called on the UI thread as the window is moved, resized or
	// given another scale, with its position and content size in logical pixels,
	// as X, Y, Width and Height are. It is called for each step of a move or a
	// resize, so an app that saves the geometry should wait until it is still.
	OnGeometry func(x, y, width, height int)
	// keepOnScreen moves the window in from the edges of its monitor when it opens
	// at X and Y (see OpenDockWindow).
	keepOnScreen bool
	// QuitWhenClosed quits the app when this window closes, even while other
	// windows are open. Before it closes, the OnClose of every other window is
	// asked; if one returns false, nothing closes. Then the other windows close
	// too, each without being asked again. By default the app quits when the last
	// window closes.
	QuitWhenClosed bool
	// CloseToTray, while it holds true, hides the window when the user closes it,
	// instead of closing it, and the app keeps running. The window's taskbar button
	// goes with it. Show (ui.Window's Show) brings the window back, which a tray
	// icon's OnClick does for a window the user hid. The app owns the signal, so it
	// can offer the choice as a setting: a Switch bound to the signal takes effect at
	// the next close. The window hides only while one of the app's tray icons is
	// shown, since without one the user could not show it again; otherwise it closes
	// as usual. OnClose is not asked while the window hides, and the window handle's
	// Close still closes the window, asking OnClose as usual. Nil is false.
	CloseToTray *ui.Signal[bool]
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
type App struct {
	fonts []appFont
	// devTools turns the visual inspector on for every window; see DevTools.
	devTools bool
	// The identity settings; see AppID, JumpList and SingleInstance.
	appID          string
	jumpList       *JumpList
	singleInstance bool
	onArgs         func(args []string, dir string)
	// notifyHandler is called for clicks on notifications; see OnNotification.
	notifyHandler func(NotificationActivation)
}

// appFont is a font given to App.Font.
type appFont struct {
	family string
	weight ui.FontWeight
	data   []byte
}

// New creates an application.
func New() *App { return &App{} }

// Font registers a font file (TrueType or OpenType data, such as a //go:embed
// file) as the upright face of family at weight, for every window of the app.
// Text uses it with Font(family) or Style.FontFamily; a weight the family has no
// face for uses the nearest one, and italic text uses the upright face. A family
// named like a built-in one ("" for the UI font, "mono") replaces that face.
//
//	app.New().
//		Font("Inter", ui.FontRegular, interRegular).
//		Font("Inter", ui.FontBold, interBold).
//		Run(app.Window{...})
//
// The fonts are parsed while the first window opens. Data that is not a font
// makes RunE return an error, and Run print it and exit.
func (a *App) Font(family string, weight ui.FontWeight, data []byte) *App {
	a.fonts = append(a.fonts, appFont{family, weight, data})
	return a
}

// DevTools turns the visual inspector on in a release build as well, for every
// window: Ctrl+Shift+I opens an inspector window for the window that has the
// focus, and closes it again. The inspector is on by default under `jamb dev`
// and with JAMB_INSPECT=1, and it shows the elements, styles, accessibility and
// frame timings of a window; see docs/tools.md, "Visual inspector".
//
// The inspector is linked only into a program that imports package devtools.
// Without that import, DevTools(true) does nothing, and the program prints a
// warning once, to stderr, that says how to link it.
func (a *App) DevTools(on bool) *App {
	a.devTools = on
	return a
}

// Run opens the window and blocks until the app quits: when the last window
// closes, or when a QuitWhenClosed window closes. On failure it prints the
// error and exits with status 1. Use RunE to handle errors yourself. A later
// launch of a SingleInstance app returns at once, without a window, after its
// command line has been handed to the running app.
func (a *App) Run(w Window) {
	if err := a.RunE(w); err != nil {
		fmt.Fprintln(os.Stderr, "jamb:", err)
		os.Exit(1)
	}
}

// RunE is Run returning an error instead of exiting. Call it once per process.
func (a *App) RunE(w Window) error {
	// jamb package asks for the AppID before anything starts (see printAppIDFlag).
	if printAppID(os.Args, a.appID, os.Stdout) {
		os.Exit(0)
	}
	if w.Root == nil {
		return fmt.Errorf("app.Window.Root is nil; set it to a func(*ui.Context) ui.Element that returns your UI")
	}
	if err := a.checkIdentity(); err != nil {
		return err
	}
	w = withDefaults(w)
	// A later launch of a single instance app returns here, with its command line
	// handed over, before any device or font is made.
	var owned *instanceNames
	if a.singleInstance {
		var handed bool
		if owned, handed = a.joinInstance(); handed {
			return nil
		}
	}
	application.dev = loadDevOptions()
	if d := application.dev; d.heapLog != "" {
		startHeapLog(d.heapLog, time.Duration(d.heapSecs*float64(time.Second)))
	}
	application.devTools = a.devTools
	application.gpu.adapter = gpuAdapter
	prepareDevice()
	// Parse fonts and warm the shaper while the first window and GPU device
	// are being created; the first window's runtime picks the result up.
	ready := make(chan textResult, 1)
	application.textReady = ready
	fonts, size := a.fonts, w.themeFor(false).FontSize
	go func() { ready <- loadText(fonts, size) }()
	defer endUI() // a read that waits for the UI thread stops waiting
	return runPlatform(func() error {
		a.setIdentity()
		if _, err := openWindow(w, nil); err != nil {
			return err
		}
		if owned != nil {
			// A notification click that started this process is delivered after the window.
			a.notificationArgs(os.Args[1:])
			a.serveHandoffs(owned)
		}
		return nil
	})
}

// Open opens another window. It is safe from event handlers and from
// goroutines: the window is opened on the UI thread. The returned ui.Window
// controls it; actions on it before the window exists run once it is open, and
// Closed reports whether it closed or failed to open. The app keeps running
// while the window is open (see Window.QuitWhenClosed).
func Open(w Window) ui.Window {
	h := &appWindow{}
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
func openFailed(w Window, h *appWindow, err error) {
	fmt.Fprintln(os.Stderr, "jamb: app.Open:", err)
	h.closed.Store(true)
	if w.OnClosed != nil {
		w.OnClosed()
	}
}

// application is the state of the process's App. It is only used on the UI
// thread, except textReady, which the goroutine started by RunE fills once.
var application struct {
	dev       devOptions
	textReady chan textResult
	text      *text.System // the shared text system, once received
	windows   []*host      // open windows, in the order they opened
	opened    int          // windows opened so far
	script    *script      // JAMB_SCRIPT, which drives the windows
	quitting  bool         // a QuitWhenClosed window is closing all windows
	appID     string       // the AppID, which names the tray icons' identity (see Tray)
	gpu       gpuRecovery  // the shared GPU device, while it is lost; see recovery.go
	devTools  bool         // the visual inspector may open (see App.DevTools and inspect.go)
}

func withDefaults(w Window) Window {
	if w.Width <= 0 {
		w.Width = 1024
	}
	if w.Height <= 0 {
		w.Height = 720
	}
	if w.Strings == nil {
		w.Strings = ui.SystemStrings()
	}
	if w.Theme == nil {
		if w.LightTheme == nil {
			w.LightTheme = ui.LightTheme()
		}
		if w.DarkTheme == nil {
			w.DarkTheme = ui.DarkTheme()
		}
	}
	return w
}

// themeFor returns the theme the window uses when the system's app mode is dark
// or light: its pinned Theme, or the one of its pair.
func (w Window) themeFor(dark bool) *ui.Theme {
	switch {
	case w.Theme != nil:
		return w.Theme
	case dark:
		return w.DarkTheme
	}
	return w.LightTheme
}

// openWindow creates a window for cfg and shows it. handle, when set, is the
// window Open returned; otherwise the window gets its own.
func openWindow(cfg Window, handle *appWindow) (*host, error) {
	if handle == nil {
		handle = &appWindow{}
	}
	h := &host{cfg: cfg, handle: handle, primary: application.opened == 0, dev: application.dev, openedAt: time.Now(),
		devTools: application.devTools || application.dev.inspectOn()}
	application.opened++
	opts := platform.WindowOptions{
		Title: cfg.Title, Width: cfg.Width, Height: cfg.Height,
		MinWidth: cfg.MinWidth, MinHeight: cfg.MinHeight, Dark: cfg.themeFor(systemSettings(h.dev).Dark).Dark,
		CustomTitleBar: cfg.Chrome == ChromeCustom,
		NoActivate:     h.dev.noActivate,
	}
	if h.primary && h.dev.position != nil {
		// JAMB_POSITION places the first window exactly; the saved geometry is not used.
		opts.Origin = h.dev.position
	} else {
		if h.primary {
			opts.Frame = h.dev.loadGeometry()
		}
		if opts.Frame.W == 0 {
			opts.Origin = origin(cfg)
			opts.KeepOnScreen = cfg.keepOnScreen
		}
	}
	h.darkFrame = opts.Dark
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

// loadText creates the text system with the app's fonts registered, and warms
// it at the theme's font size.
func loadText(fonts []appFont, size float32) textResult {
	ts := text.NewSystem()
	for _, f := range fonts {
		if err := ts.Registry().Register(f.family, f.weight, f.data); err != nil {
			return textResult{err: fmt.Errorf("app.Font(%q, %d): %w", f.family, f.weight, err)}
		}
	}
	ts.Warm(size)
	return textResult{ts: ts}
}

// textResult is what the goroutine started by RunE sends: the text system, or
// the error of a font the app registered.
type textResult struct {
	ts  *text.System
	err error
}

// sharedText returns the text system all windows use. The first call waits for
// the goroutine started by RunE; it reports whether it had to wait. It fails
// when a font given to App.Font could not be loaded.
func sharedText() (*text.System, bool, error) {
	if application.text != nil {
		return application.text, false, nil
	}
	res := <-application.textReady
	if res.err != nil {
		return nil, true, res.err
	}
	application.text = res.ts
	return application.text, true, nil
}

// host is one window: it implements platform.Handler and drives the window's
// ui runtime and renderer.
type host struct {
	cfg      Window
	handle   *appWindow // the window's ui.Window
	primary  bool       // the first window of the process: it saves its geometry
	dev      devOptions
	openedAt time.Time
	// devTools is whether Ctrl+Shift+I opens the inspector of this window; insp
	// is the inspector while one is open, and inspected is set on an inspector's
	// own window (see inspect.go).
	devTools  bool
	insp      *inspector
	inspected *host
	// closeAsked is set by the window handle's Close, so a CloseToTray window
	// closes rather than hides; OnClose clears it.
	closeAsked bool

	win platform.Window
	rt  *ui.Runtime
	rh  runtimeHost // the host side of rt
	// darkFrame is whether the window's decorations are dark, as last set.
	darkFrame bool
	r         render.Renderer
	focused   bool

	keymapFile *keymap.File
	stopWatch  func() // stops the keymap file watcher, if one runs
	stress     *stress
	fs         *frameStats // JAMB_FRAMESTATS, on the first window only

	frames    int
	presented bool // the last OnFrame reached the screen, see Presented
	// preScene is the first frame's scene, built in OnCreate for a client area of
	// preW×preH; nil once a frame has used it.
	preScene   *scene.Scene
	preW, preH int

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
	t1 := time.Now()
	ts, waited, err := sharedText()
	if err != nil {
		return err
	}
	if waited {
		h.dev.tracef("fonts ready (waited %v)", time.Since(t1))
	}
	h.rt = ui.NewRuntime(h.cfg.Root, h.cfg.themeFor(false), ts)
	h.rt.SetStrings(h.cfg.Strings)
	if h.cfg.Theme == nil {
		// applySettings, below, picks the one the system's app mode asks for.
		h.rt.SetThemes(h.cfg.LightTheme, h.cfg.DarkTheme)
	}
	h.rh = hostOf(h.rt)
	h.rh.SetWakeFunc(w.Wake)
	h.rh.SetServices(w.Clipboard, w.SetClipboard, w.WakeAt)
	h.rh.SetImageClipboard(w.ClipboardImage, w.SetClipboardImage, w.HasClipboardImage)
	h.rh.SetOpenURL(w.OpenURL)
	h.rh.SetNativeCaption(h.cfg.Chrome == ChromeCustom) // the platform presses the title-bar buttons
	h.rh.SetCancelComposition(w.CancelComposition)
	h.rh.SetClientOrigin(func() (int, int) { p := w.ClientOrigin(); return p.X, p.Y })
	h.rt.SetWindow(h.handle)
	h.rt.SetAccessWarnings(h.dev.a11yWarn)
	h.applySettings()
	h.rh.SetMaximized(w.Maximized())
	h.rt.AddBindings(h.cfg.Bindings...)
	if h.cfg.KeymapFile != "" {
		h.keymapFile = &keymap.File{Path: h.cfg.KeymapFile, Actions: h.cfg.Actions}
		h.loadKeymap(true, 0)
		h.watchKeymap()
	}
	if h.cfg.Zoom > 0 {
		h.rt.SetZoom(h.cfg.Zoom)
	}
	h.rh.OnZoom(h.cfg.OnZoom)
	h.updateViewport(w)
	if h.primary {
		if h.dev.stress != "" {
			h.stress = newStress(h.dev.stress)
		}
		if h.dev.frameStats != "" {
			h.fs = newFrameStats(h.dev.frameStats)
		}
		if h.dev.script != "" {
			application.script, err = parseScript(h.dev.script)
			if err != nil {
				return err
			}
		}
	}
	h.handle.host = h
	// The first frame is built while the GPU device is still being created (see
	// prepareDevice): the layout and paint do not need the device, so the window
	// waits only for what is left of it. OnFrame presents this scene.
	if width, height := w.Size(); width > 0 && height > 0 {
		h.preScene = h.rh.Frame(time.Now())
		h.preW, h.preH = width, height
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
	h.dev.tracef("renderer ready %v after process start (OnCreate took %v)%s", time.Since(processStart), time.Since(t0), desc)
	application.windows = append(application.windows, h)
	h.startInspector()
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
		h.reportGeometry(w)
	case platform.EventMoved:
		h.saveGeometry(w)
		h.reportGeometry(w)
		h.fitBuffers(w)
	case platform.EventSettingsChanged:
		h.applySettings()
	case platform.EventWindowState:
		h.rh.SetMaximized(w.Maximized())
	case platform.EventFileDrop:
		h.rt.DropFiles(x, y, e.Paths)
	case platform.EventMouseMove:
		h.rt.PointerMove(x, y, e.Mods)
	case platform.EventMouseLeave:
		h.rh.PointerLeave()
	case platform.EventMouseDown:
		h.rt.PointerDown(x, y, e.Button, e.Clicks, e.Mods)
	case platform.EventMouseUp:
		h.rt.PointerUp(x, y, e.Button, e.Mods)
	case platform.EventScroll:
		h.rt.Scroll(x, y, e.DeltaX, e.DeltaY, e.Precise, e.Mods)
	case platform.EventGesture:
		h.rt.Pinch(x, y, gesturePhase(e.Phase), e.Scale, e.PanX/s, e.PanY/s, e.Inertia, e.Mods)
	case platform.EventKeyDown:
		if !h.inspectKey(e.Key, e.Mods) {
			h.rt.KeyDown(e.Key, e.Mods, e.Repeat)
		}
	case platform.EventKeyUp:
		h.rt.KeyUp(e.Key, e.Mods)
	case platform.EventTextInput:
		h.rt.TextInput(e.Text)
	case platform.EventComposition:
		h.rh.Compose(e.Text, e.Cursor, e.ClauseStart, e.ClauseEnd)
	case platform.EventFocus:
		h.focused = true
		h.rh.WindowFocus(true)
	case platform.EventBlur:
		h.focused = false
		h.rh.WindowFocus(false)
	}
	w.SetCursor(h.rh.Cursor())
	needsFrame := h.rt.NeedsFrame()
	if k, ok := inputKindOf(e.Kind); ok && h.fs != nil {
		h.fs.input(k, e.Arrived, needsFrame)
	}
	if needsFrame {
		w.Wake()
	}
}

// gesturePhase maps the phase of a platform gesture to the one the UI runtime uses.
func gesturePhase(p platform.GesturePhase) ui.GesturePhase {
	switch p {
	case platform.GestureUpdate:
		return ui.GestureUpdate
	case platform.GestureEnd:
		return ui.GestureEnd
	}
	return ui.GestureBegin
}

// saveGeometry remembers the first window's position and size for the next run.
func (h *host) saveGeometry(w platform.Window) {
	if h.primary {
		h.dev.saveGeometry(w.Frame())
	}
}

// reportGeometry gives the window's position and content size, in logical pixels, to the
// app's OnGeometry, if it has one.
func (h *host) reportGeometry(w platform.Window) {
	if h.cfg.OnGeometry == nil {
		return
	}
	f, s := w.Frame(), w.Scale()
	cw, ch := w.Size()
	h.cfg.OnGeometry(int(float32(f.X)/s+0.5), int(float32(f.Y)/s+0.5),
		int(float32(cw)/s+0.5), int(float32(ch)/s+0.5))
}

// OnClose implements platform.Handler.
// OnClose implements platform.Handler. A QuitWhenClosed window asks the OnClose
// of every other open window first. If one of them keeps its window open, the
// whole close is cancelled, this window included. Otherwise the other windows
// close too, without being asked again.
func (h *host) OnClose(w platform.Window) bool {
	// What the window saves late is saved now, before the app asks or quits.
	if h.rh != nil {
		h.rh.FlushTimers()
	}
	asked := h.closeAsked
	h.closeAsked = false
	if application.quitting {
		return true
	}
	// A CloseToTray window hides for a close the user asks for; the app's own
	// Close, and the quit of the app, still close it.
	if closeHides(h.cfg.CloseToTray, shownTrays, asked) {
		w.Hide()
		return false
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
func (h *host) HasMenuBar() bool { return h.rh != nil && h.rh.HasMenuBar() }

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
	switch h.rh.WindowAreaAt(x/s, y/s) {
	case ui.WindowAreaDrag:
		return platform.TitleDrag
	case ui.WindowAreaMinimize:
		return platform.TitleMinimize
	case ui.WindowAreaMaximize:
		return platform.TitleMaximize
	case ui.WindowAreaClose:
		return platform.TitleClose
	}
	return platform.TitleNone
}

// Presented implements platform.Presenter: the last frame was shown, so the
// window needs no repaint on WM_PAINT at its size.
func (h *host) Presented() bool { return h.presented }

// liveResizer is a renderer whose buffers keep headroom while the user drags
// the window's edge, and fit the window otherwise (see d3d11.Renderer.resize).
type liveResizer interface {
	SetLiveResize(on bool)
	FitBuffers() (bool, error)
}

// liveWindow is a window that reports a live resize: the user dragging its edge
// or moving it.
type liveWindow interface{ LiveResize() bool }

// setLiveResize tells the renderer whether the window is in a live resize, before
// a frame that may resize its buffers.
func (h *host) setLiveResize(w platform.Window) {
	r, ok := h.r.(liveResizer)
	if !ok {
		return
	}
	lw, ok := w.(liveWindow)
	r.SetLiveResize(ok && lw.LiveResize())
}

// fitBuffers makes the window's swap chain buffers exactly the window's size once
// the user has finished moving or resizing it (see d3d11.Renderer.FitBuffers), and
// draws a frame into them.
func (h *host) fitBuffers(w platform.Window) {
	r, ok := h.r.(liveResizer)
	if !ok {
		return
	}
	r.SetLiveResize(false)
	fitted, err := r.FitBuffers()
	if err != nil {
		if errors.Is(err, render.ErrDeviceLost) {
			loseGPU(err)
		} else {
			fmt.Fprintln(os.Stderr, "jamb: fitting the window's buffers failed:", err)
		}
		return
	}
	if fitted {
		w.Wake()
	}
}

// OnFrame implements platform.Handler.
func (h *host) OnFrame(w platform.Window) bool {
	h.presented = false
	width, height := w.Size()
	if !drawWanted(w.Visible(), h.frames > 0, width, height) {
		// Hidden or minimized: nothing is drawn, and no animation runs. Showing the
		// window wakes it, and its first frame then reads the current state.
		return false
	}
	h.updateViewport(w)
	h.setLiveResize(w)
	// The native title bar follows the theme: the app may switch it, or the
	// system's app mode may, when the window follows it.
	if dark := h.rt.Theme().Dark; dark != h.darkFrame {
		h.darkFrame = dark
		w.SetDark(dark)
	}
	frameStart := time.Now()
	frameAt := clockNow()
	if !h.gpuReady(frameStart) {
		return false // the device is lost; the window is woken when it may draw
	}
	if h.stress != nil {
		h.stress.begin(h, h.dev.stressSecs)
	}
	sc := h.preScene
	h.preScene = nil
	if !preSceneFits(sc, h.preW, h.preH, width, height) {
		sc = h.rh.Frame(frameStart)
	}
	if h.insp != nil {
		h.insp.frame(h.rt, frameStart)
	}
	// Tell the OS where the caret is, for input-method (IME) windows.
	var ime platform.Rect
	if r, ok := h.rh.TextInputRect(); ok {
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
	if o, ok := h.r.(interface{ Occluded() bool }); ok && o.Occluded() {
		// Covered by another window: the frame was not shown, so the window
		// stops asking for frames until DXGI wakes it (see render/d3d11).
		return false
	}
	h.presented = true
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
	// The CPU side of the frame ends where Present starts; Present blocks for vsync.
	start, end := presentTimes(h.r, clockNow())
	if h.fs != nil {
		h.fs.presented(h.rt.Stats(), frameAt, start, end, frameWanted(h.rt.FrameRequested(), h.stress != nil), w)
	}
	if h.stress != nil {
		return h.stress.end(h, w, start-frameAt)
	}
	if script != nil {
		again = script.afterDraw(h, w) || again
	}
	return again
}

// OnDestroy implements platform.Handler. The app quits when its last window
// closes, or when a QuitWhenClosed window does.
func (h *host) OnDestroy(w platform.Window) {
	if h.fs != nil {
		h.fs.finish()
		h.fs = nil
	}
	if h.rh != nil {
		h.rh.Close() // the window's queries cancel their fetches
	}
	if h.r != nil {
		h.r.Close()
		h.r = nil
	}
	if h.stopWatch != nil {
		h.stopWatch()
		h.stopWatch = nil
	}
	if ins := h.insp; ins != nil {
		// The inspected window is going: its inspector closes with it, after this
		// window's destruction has finished.
		h.insp = nil
		postUI(ins.win.Close)
	}
	application.windows = slices.DeleteFunc(application.windows, func(o *host) bool { return o == h })
	h.handle.closed.Store(true)
	h.handle.host = nil
	if h.cfg.OnClosed != nil {
		h.cfg.OnClosed()
	}
	// While a QuitWhenClosed window is closing the others, the app quits with the
	// last of them.
	if len(application.windows) == 0 || (h.cfg.QuitWhenClosed && !application.quitting) {
		quitUI()
	}
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

// runtimeHost is the window host's side of a ui.Runtime: the methods that only
// app calls, which ui keeps out of its public API (see internal/uihost).
type runtimeHost interface {
	Frame(now time.Time) *scene.Scene
	SetWakeFunc(fn func())
	SetServices(clipboard func() (string, bool), setClipboard func(string), wakeAt func(time.Time))
	SetImageClipboard(get func() (image.Image, bool), set func(image.Image), has func() bool)
	SetOpenURL(fn func(string) error)
	SetCancelComposition(fn func())
	SetClientOrigin(fn func() (x, y int))
	Compose(text string, cursor, clauseStart, clauseEnd int)
	SetMaximized(on bool)
	SetNativeCaption(on bool)
	OnZoom(fn func(float32))
	Invalidate()
	InvalidateAll()
	PointerLeave()
	WindowFocus(focused bool)
	Cursor() input.Cursor
	HasMenuBar() bool
	TextInputRect() (scene.Rect, bool)
	WindowAreaAt(x, y float32) ui.WindowArea
	FlushTimers()
	Close()
}

func hostOf(rt *ui.Runtime) runtimeHost { return uihost.Of(rt).(runtimeHost) }

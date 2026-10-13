//go:build windows

// Package windows is the Win32 platform adapter. It is pure Go (no cgo) and
// talks to user32/dwmapi through syscall.
//
// One UI thread runs one message loop for every window of the process. Each
// window has its own Handler; the adapter finds a window from its HWND.
package windows

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

const (
	msgWake = win32.WM_APP + 1 // posted to a window: its Wake was called
	msgPost = win32.WM_APP + 2 // posted to the thread: run the Post queue
)

type window struct {
	hwnd    uintptr
	handler platform.Handler
	opts    platform.WindowOptions

	dpi    uint32
	width  int
	height int

	// fullscreen is set while the window covers its monitor; savedStyle and
	// savedPlacement are what SetFullscreen(false) restores.
	fullscreen     bool
	savedStyle     uintptr
	savedPlacement win32.WINDOWPLACEMENT
	cursor         input.Cursor
	cursors        map[input.Cursor]uintptr

	wakePending atomic.Bool
	frameWanted bool // set on the UI thread
	tracking    bool // mouse-leave tracking active
	ncHover     bool // the pointer is over a title-bar button (non-client area)
	buttonsDown int
	highSurr    uint16
	inSizeMove  bool
	ready       bool // OnCreate succeeded; frames may be rendered
	maximized   bool // last reported state, see platform.EventWindowState
	wakeAt      time.Time
	imeRect     platform.Rect
	caretH      int32 // height of the hidden system caret, or 0 when there is none (see placeCaret)
	// caption is the title-bar button the mouse went down on; its action runs
	// when the button is released over it.
	caption platform.TitleArea
	// uiaRoot is the window's UI Automation root object, as a
	// IRawElementProviderSimple view. The window holds one reference to it
	// until it is destroyed. uiaBridge is the bridge the window listens to; it
	// is nil until the window first answers UI Automation.
	uiaRoot   uintptr
	uiaBridge *access.Bridge
}

// The state of the UI thread, shared by all windows of the process.
var (
	// windows maps HWND to window. It is only touched on the UI thread.
	windows = map[uintptr]*window{}
	// creating is the window whose CreateWindowExW call is running; its
	// WM_NCCREATE, the first message a window gets, binds it to its HWND.
	creating *window
	// uiThread is the thread id of the loop, or 0 before Run.
	uiThread atomic.Uint32
	// quitRequested ends the loop; it is set on the UI thread (see Quit).
	quitRequested bool

	postMu sync.Mutex
	posted []func()
)

// Run runs the event loop on the calling goroutine, which becomes the UI
// thread, until Quit is called. It calls start on the UI thread before the
// loop runs; start typically opens the first window. An error from start is
// returned without running the loop.
func Run(start func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	win32.EnablePerMonitorDPI()
	win32.OleInitialize()
	inst := win32.GetModuleHandle()
	className, _ := syscall.UTF16PtrFromString("JambWindow")
	wc := win32.WNDCLASSEXW{
		Style:         win32.CS_DBLCLKS,
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     inst,
		HCursor:       win32.LoadCursor(win32.IDC_ARROW),
		LpszClassName: className,
	}
	wc.CbSize = uint32(sizeofWndClass)
	if _, err := win32.RegisterClassEx(&wc); err != nil {
		return fmt.Errorf("platform/windows: RegisterClassExW failed: %v", err)
	}
	// Creating the thread's message queue first lets Post reach the loop from
	// any goroutine once uiThread is set.
	var msg win32.MSG
	win32.PeekMessage(&msg)
	uiThread.Store(win32.GetCurrentThreadID())

	if err := start(); err != nil {
		return err
	}
	runLoop()
	uiThread.Store(0)
	return nil
}

// runLoop dispatches messages and renders the windows that want a frame. A
// window renders once its swap chain can take a frame (its frame latency
// waitable is signaled), so Present does not block and the windows pace
// themselves in parallel. A window without a waitable renders at once. When
// nothing wants a frame, the loop sleeps in GetMessage, so an idle application
// uses no CPU.
func runLoop() {
	runPosted()
	var msg win32.MSG
	for !quitRequested {
		for win32.PeekMessage(&msg) {
			dispatch(&msg)
		}
		if quitRequested {
			return
		}
		var waiting []*window
		var handles []uintptr
		rendered := false
		for _, w := range windows {
			if !w.ready || !(w.frameWanted || w.wakePending.Load()) {
				continue
			}
			h := w.waitable()
			if h == 0 || w.width <= 0 || w.height <= 0 {
				w.render()
				rendered = true
				continue
			}
			if win32.WaitForSingleObject(h, 0) == win32.WAIT_OBJECT_0 {
				w.render()
				rendered = true
				continue
			}
			waiting = append(waiting, w)
			handles = append(handles, h)
		}
		if rendered {
			continue
		}
		if len(handles) > 0 {
			// Sleep until a swap chain can take a frame or a message arrives.
			if i := win32.MsgWaitForAny(handles); i < uint32(len(handles)) {
				waiting[i].render()
			} else if i == win32.WAIT_FAILED {
				for _, w := range waiting {
					w.render()
				}
			}
			continue
		}
		if !win32.GetMessage(&msg) {
			return
		}
		dispatch(&msg)
	}
}

func dispatch(msg *win32.MSG) {
	if msg.Hwnd == 0 {
		if msg.Message == msgPost {
			runPosted()
		}
		return
	}
	win32.TranslateMessage(msg)
	win32.DispatchMessage(msg)
}

// Quit ends the loop when it next runs. It is safe from any goroutine; the
// windows must already be closed (or their OnDestroy has run).
func Quit() {
	Post(func() { quitRequested = true })
}

// OnUIThread reports whether the caller runs on the UI thread. The loop's
// goroutine is locked to its thread, so no other goroutine runs there.
func OnUIThread() bool {
	t := uiThread.Load()
	return t != 0 && t == win32.GetCurrentThreadID()
}

// Post runs fn on the UI thread, in order with the other posted functions. It
// is safe from any goroutine and wakes the loop if it is idle.
func Post(fn func()) {
	postMu.Lock()
	posted = append(posted, fn)
	postMu.Unlock()
	if t := uiThread.Load(); t != 0 {
		win32.PostThreadMessage(t, msgPost)
	}
}

func runPosted() {
	postMu.Lock()
	fns := posted
	posted = nil
	postMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// Open creates a window and shows it. It must be called on the UI thread. The
// handler's OnCreate runs before the window appears, and its first frame is
// rendered before that too.
func Open(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	w := &window{handler: h, opts: opts, cursors: map[input.Cursor]uintptr{}}
	className, _ := syscall.UTF16PtrFromString("JambWindow")

	// Size the window for the system DPI first; WM_DPICHANGED corrects it if
	// the window lands on a monitor with a different scale.
	dpi := win32.GetDpiForSystem()
	w.dpi = dpi
	ww, wh := w.outerSize(opts.Width, opts.Height, dpi)
	title, _ := syscall.UTF16PtrFromString(opts.Title)
	creating = w
	hwnd, err := win32.CreateWindowEx(win32.WS_EX_APPWINDOW, className, title, win32.WS_OVERLAPPEDWINDOW,
		0, 0, ww, wh, 0, 0, win32.GetModuleHandle())
	creating = nil
	if err != nil {
		return nil, fmt.Errorf("platform/windows: CreateWindowExW failed: %v", err)
	}
	win32.DragAcceptFiles(hwnd, true)
	w.dpi = win32.GetDpiForWindow(hwnd)
	if w.dpi != dpi {
		ww, wh = w.outerSize(opts.Width, opts.Height, w.dpi)
	}
	if opts.CustomTitleBar {
		w.applyCustomFrame()
	}
	switch f := opts.Frame; {
	case f.W > 0 && f.H > 0:
		win32.SetWindowPos(hwnd, int32(f.X), int32(f.Y), int32(f.W), int32(f.H), win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
		w.dpi = win32.GetDpiForWindow(hwnd)
	case opts.Origin != nil:
		o := opts.Origin
		win32.SetWindowPos(hwnd, int32(scaleInt(o.X, dpi)), int32(scaleInt(o.Y, dpi)), ww, wh, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
	default:
		work := win32.WorkArea(hwnd)
		x := work.Left + (work.Right-work.Left-ww)/2
		y := work.Top + (work.Bottom-work.Top-wh)/2
		win32.SetWindowPos(hwnd, x, y, ww, wh, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
	}
	win32.SetDarkTitleBar(hwnd, opts.Dark)
	w.updateSize()

	if err := h.OnCreate(w); err != nil {
		win32.DestroyWindow(hwnd)
		return nil, err
	}
	w.ready = true
	// Render the first frame before showing to avoid a blank/white flash.
	w.frameWanted = h.OnFrame(w)
	w.Show()
	return w, nil
}

// frame renders a window for a message (WM_PAINT or WM_SIZE, during a resize).
// It waits briefly for the swap chain, so the frame is not dropped and Present
// does not block for long.
func (w *window) frame() {
	if h := w.waitable(); h != 0 && w.width > 0 && w.height > 0 {
		win32.WaitForSingleObject(h, 50)
	}
	w.render()
}

// render calls the handler to draw one frame, once the swap chain can take it.
func (w *window) render() {
	w.wakePending.Store(false)
	w.frameWanted = w.handler.OnFrame(w)
}

// waitable returns the handler's frame latency waitable, or 0.
func (w *window) waitable() uintptr {
	if fw, ok := w.handler.(platform.FrameWaiter); ok {
		return fw.FrameWaitable()
	}
	return 0
}

func (w *window) outerSize(width, height int, dpi uint32) (int32, int32) {
	r := win32.RECT{Right: int32(scaleInt(width, dpi)), Bottom: int32(scaleInt(height, dpi))}
	if w.opts.CustomTitleBar {
		// The client area leaves the resize frame outside on the sides and bottom.
		fx, fy := w.frameInsets()
		return r.Right + 2*fx, r.Bottom + fy
	}
	win32.AdjustWindowRectExForDpi(&r, win32.WS_OVERLAPPEDWINDOW, win32.WS_EX_APPWINDOW, dpi)
	return r.Right - r.Left, r.Bottom - r.Top
}

func scaleInt(v int, dpi uint32) int { return (v*int(dpi) + 48) / 96 }

func (w *window) updateSize() {
	r := win32.GetClientRect(w.hwnd)
	w.width, w.height = int(r.Right-r.Left), int(r.Bottom-r.Top)
}

func (w *window) Handle() uintptr   { return w.hwnd }
func (w *window) Size() (int, int)  { return w.width, w.height }
func (w *window) Scale() float32    { return float32(w.dpi) / 96 }
func (w *window) SetTitle(t string) { win32.SetWindowText(w.hwnd, t) }
func (w *window) Frame() platform.Rect {
	r := win32.GetWindowRect(w.hwnd)
	return platform.Rect{X: int(r.Left), Y: int(r.Top), W: int(r.Right - r.Left), H: int(r.Bottom - r.Top)}
}
func (w *window) Show()  { win32.ShowWindow(w.hwnd, win32.SW_SHOW) }
func (w *window) Close() { win32.PostMessage(w.hwnd, win32.WM_CLOSE, 0, 0) }

// Minimize implements platform.Window.
func (w *window) Minimize() { win32.ShowWindow(w.hwnd, win32.SW_MINIMIZE) }

// ToggleMaximize implements platform.Window.
func (w *window) ToggleMaximize() {
	if win32.IsZoomed(w.hwnd) {
		win32.ShowWindow(w.hwnd, win32.SW_RESTORE)
	} else {
		win32.ShowWindow(w.hwnd, win32.SW_MAXIMIZE)
	}
}

// Maximized implements platform.Window.
func (w *window) Maximized() bool { return win32.IsZoomed(w.hwnd) }

// restoreForGeometry leaves fullscreen and maximized, so a new size or
// position applies to the restored window.
func (w *window) restoreForGeometry() {
	if w.fullscreen {
		w.SetFullscreen(false)
	}
	if win32.IsZoomed(w.hwnd) {
		win32.ShowWindow(w.hwnd, win32.SW_RESTORE)
	}
}

func (w *window) SetSize(width, height int) {
	w.restoreForGeometry()
	ww, wh := w.outerSize(width, height, w.dpi)
	win32.SetWindowPos(w.hwnd, 0, 0, ww, wh, win32.SWP_NOMOVE|win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
}

func (w *window) SetPosition(x, y int) {
	w.restoreForGeometry()
	win32.SetWindowPos(w.hwnd, int32(scaleInt(x, w.dpi)), int32(scaleInt(y, w.dpi)), 0, 0,
		win32.SWP_NOSIZE|win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
}

func (w *window) Fullscreen() bool { return w.fullscreen }

func (w *window) SetDark(on bool) { win32.SetDarkTitleBar(w.hwnd, on) }

func (w *window) OpenURL(url string) error {
	if !win32.ShellOpen(w.hwnd, url) {
		return fmt.Errorf("platform/windows: the system could not open %q", url)
	}
	return nil
}

// SetFullscreen removes the frame and covers the monitor the window is on, as
// Windows apps conventionally do: the style and placement are saved, and
// restored when the window leaves fullscreen.
func (w *window) SetFullscreen(on bool) {
	if on == w.fullscreen {
		return
	}
	if on {
		w.savedStyle = win32.WindowStyle(w.hwnd)
		w.savedPlacement = win32.GetWindowPlacement(w.hwnd)
		m := win32.MonitorRect(w.hwnd)
		w.fullscreen = true
		win32.SetWindowStyle(w.hwnd, w.savedStyle&^win32.WS_OVERLAPPEDWINDOW)
		win32.SetWindowPos(w.hwnd, m.Left, m.Top, m.Right-m.Left, m.Bottom-m.Top,
			win32.SWP_NOOWNERZORDER|win32.SWP_FRAMECHANGED)
		return
	}
	w.fullscreen = false
	win32.SetWindowStyle(w.hwnd, w.savedStyle)
	win32.SetWindowPlacement(w.hwnd, &w.savedPlacement)
	win32.SetWindowPos(w.hwnd, 0, 0, 0, 0,
		win32.SWP_NOMOVE|win32.SWP_NOSIZE|win32.SWP_NOZORDER|win32.SWP_NOOWNERZORDER|win32.SWP_FRAMECHANGED)
}

// Focus implements platform.Window. Windows only lets an app take the
// foreground when the user is working with it; otherwise the taskbar flashes.
func (w *window) Focus() {
	if win32.IsIconic(w.hwnd) {
		win32.ShowWindow(w.hwnd, win32.SW_RESTORE)
	}
	win32.SetForegroundWindow(w.hwnd)
}

func (w *window) SetCursor(c input.Cursor) {
	if c == w.cursor {
		return
	}
	w.cursor = c
	win32.SetCursor(w.cursorHandle(c))
}

func (w *window) Wake() {
	if w.wakePending.CompareAndSwap(false, true) {
		win32.PostMessage(w.hwnd, msgWake, 0, 0)
	}
}

func (w *window) cursorHandle(c input.Cursor) uintptr {
	if h, ok := w.cursors[c]; ok {
		return h
	}
	id := uintptr(win32.IDC_ARROW)
	switch c {
	case input.CursorPointer:
		id = win32.IDC_HAND
	case input.CursorText:
		id = win32.IDC_IBEAM
	case input.CursorResizeEW:
		id = win32.IDC_SIZEWE
	case input.CursorResizeNS:
		id = win32.IDC_SIZENS
	case input.CursorNotAllowed:
		id = win32.IDC_NO
	}
	h := win32.LoadCursor(id)
	w.cursors[c] = h
	return h
}

func (w *window) emit(e platform.Event) {
	e.Mods = currentMods()
	w.handler.OnEvent(w, e)
}

// ownsF10 reports whether the F10 key is the application's. F10 is a system
// key, and the system's default handling turns it into the menu bar's keyboard
// mode, which then takes the keys that follow (Down, Tab) from the window, so the
// application would never see them; a context menu opened with Shift+F10 is then
// stuck. The application handles F10 itself, so the default is not run for it.
// Alt+F10 keeps the system's handling.
func ownsF10(wp uintptr) bool {
	return wp == win32.VK_F10 && win32.GetKeyState(win32.VK_MENU) >= 0
}

func currentMods() input.Modifiers {
	var m input.Modifiers
	if win32.GetKeyState(win32.VK_SHIFT) < 0 {
		m |= input.ModShift
	}
	if win32.GetKeyState(win32.VK_CONTROL) < 0 {
		m |= input.ModCtrl
	}
	if win32.GetKeyState(win32.VK_MENU) < 0 {
		m |= input.ModAlt
	}
	if win32.GetKeyState(win32.VK_LWIN) < 0 || win32.GetKeyState(win32.VK_RWIN) < 0 {
		m |= input.ModSuper
	}
	return m
}

// trackMouse asks Windows for the mouse-leave message, which also covers the
// non-client area for a custom title bar.
func (w *window) trackMouse() {
	if !w.tracking {
		win32.TrackMouseLeave(w.hwnd, w.opts.CustomTitleBar)
		w.tracking = true
	}
}

// showsMenuBar reports whether the handler's window has an app menu bar, which
// takes a lone Alt press (see platform.MenuBarHandler).
func showsMenuBar(h platform.Handler) bool {
	mb, ok := h.(platform.MenuBarHandler)
	return ok && mb.HasMenuBar()
}

func wndProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	w := windows[hwnd]
	if w == nil && msg == win32.WM_NCCREATE && creating != nil {
		w = creating
		creating = nil
		w.hwnd = hwnd
		windows[hwnd] = w
	}
	if w == nil {
		return win32.DefWindowProc(hwnd, msg, wp, lp)
	}
	switch msg {
	case msgWake:
		return 0

	case win32.WM_OCCLUSION:
		// The window may be visible again: render, which shows what changed while it was covered.
		w.Wake()
		return 0

	case win32.WM_TIMER:
		if wp == wakeTimerID {
			w.onTimer()
			return 0
		}
		if wp == modalTimerID {
			pumpFrames()
			return 0
		}

	case win32.WM_IME_STARTCOMPOSITION:
		// Place the composition window at the caret, then let Windows draw it.
		if r := w.imeRect; r.H > 0 {
			win32.SetIMEPosition(hwnd, int32(r.X), int32(r.Y), int32(r.H))
		}

	case win32.WM_ERASEBKGND:
		return 1

	case win32.WM_PAINT:
		win32.ValidateRect(hwnd)
		if w.ready {
			w.frame()
		}
		return 0

	case win32.WM_SIZE:
		w.updateSize()
		if now := win32.IsZoomed(hwnd); w.ready && now != w.maximized {
			w.maximized = now
			w.emit(platform.Event{Kind: platform.EventWindowState})
		}
		if w.ready && w.width > 0 && w.height > 0 {
			w.emit(platform.Event{Kind: platform.EventResize})
			// Render synchronously so content follows the frame during the
			// modal resize loop, where our own loop is not running.
			w.frame()
		}
		return 0

	case win32.WM_ENTERSIZEMOVE:
		w.inSizeMove = true
		w.startFrames()
		return 0
	case win32.WM_EXITSIZEMOVE:
		w.inSizeMove = false
		w.stopFrames()
		w.emit(platform.Event{Kind: platform.EventMoved})
		return 0

	case win32.WM_ENTERMENULOOP:
		// The system menu, which F10 or a lone Alt opens in a window without a
		// menu bar, runs its own loop, as a dialog does: frames are pumped.
		w.startFrames()
		return 0
	case win32.WM_EXITMENULOOP:
		w.stopFrames()
		return 0

	case win32.WM_DPICHANGED:
		w.dpi = uint32(win32.HIWORD(wp))
		r := (*win32.RECT)(ptr(lp))
		w.emit(platform.Event{Kind: platform.EventScaleChanged})
		win32.SetWindowPos(hwnd, r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
		return 0

	case win32.WM_GETMINMAXINFO:
		if w.opts.MinWidth > 0 || w.opts.MinHeight > 0 {
			mmi := (*win32.MINMAXINFO)(ptr(lp))
			mw, mh := w.outerSize(w.opts.MinWidth, w.opts.MinHeight, w.dpi)
			mmi.PtMinTrackSize = win32.POINT{X: mw, Y: mh}
		}
		return 0

	case win32.WM_NCCALCSIZE:
		if w.opts.CustomTitleBar && wp != 0 {
			return w.calcClientSize(lp)
		}

	case win32.WM_NCHITTEST:
		if w.fullscreen {
			// A fullscreen window has no frame and no title bar to drag.
			return win32.HTCLIENT
		}
		if w.opts.CustomTitleBar {
			return w.hitTest(lp)
		}

	case win32.WM_SETCURSOR:
		if win32.LOWORD(lp) == win32.HTCLIENT {
			win32.SetCursor(w.cursorHandle(w.cursor))
			return 1
		}

	case win32.WM_MOUSEMOVE:
		w.trackMouse()
		x, y := win32.PointFromLParam(lp)
		w.emit(platform.Event{Kind: platform.EventMouseMove, X: float32(x), Y: float32(y)})
		return 0

	case win32.WM_NCMOUSEMOVE:
		if w.opts.CustomTitleBar {
			w.trackMouse()
			// Title-bar buttons hover like client controls. Other non-client
			// areas (the resize borders, the drag area) leave the runtime's
			// hover alone; the cursor is Windows' there.
			if captionArea(wp) != platform.TitleNone {
				x, y := w.clientPoint(lp)
				w.ncHover = true
				w.emit(platform.Event{Kind: platform.EventMouseMove, X: x, Y: y})
			} else if w.ncHover {
				w.ncHover = false
				w.emit(platform.Event{Kind: platform.EventMouseLeave})
			}
		}

	case win32.WM_MOUSELEAVE:
		w.tracking = false
		w.emit(platform.Event{Kind: platform.EventMouseLeave})
		return 0

	case win32.WM_NCMOUSELEAVE:
		w.tracking = false
		if w.ncHover {
			w.ncHover = false
			w.emit(platform.Event{Kind: platform.EventMouseLeave})
		}
		// Like the other non-client mouse messages, the default handling sees it.

	case win32.WM_NCLBUTTONDOWN, win32.WM_NCLBUTTONDBLCLK:
		// A title-bar button is pressed like a client control. The other
		// non-client areas are Windows' (moving, snapping, the system menu).
		if area := captionArea(wp); w.opts.CustomTitleBar && area != platform.TitleNone {
			x, y := w.clientPoint(lp)
			clicks := 1
			if msg == win32.WM_NCLBUTTONDBLCLK {
				clicks = 2
			}
			w.buttonDown(area, x, y, clicks)
			return 0
		}

	case win32.WM_NCLBUTTONUP:
		if w.caption != platform.TitleNone {
			x, y := w.clientPoint(lp)
			w.buttonUp(x, y)
			return 0
		}

	case win32.WM_LBUTTONDOWN, win32.WM_RBUTTONDOWN, win32.WM_MBUTTONDOWN,
		win32.WM_LBUTTONDBLCLK, win32.WM_RBUTTONDBLCLK:
		x, y := win32.PointFromLParam(lp)
		clicks := 1
		if msg == win32.WM_LBUTTONDBLCLK || msg == win32.WM_RBUTTONDBLCLK {
			clicks = 2
		}
		if w.buttonsDown == 0 {
			win32.SetCapture(hwnd)
		}
		w.buttonsDown++
		w.emit(platform.Event{Kind: platform.EventMouseDown, X: float32(x), Y: float32(y), Button: buttonOf(msg), Clicks: clicks})
		return 0

	case win32.WM_LBUTTONUP, win32.WM_RBUTTONUP, win32.WM_MBUTTONUP:
		x, y := win32.PointFromLParam(lp)
		if w.caption != platform.TitleNone {
			// The release of a title-bar button press; the mouse is captured,
			// so it arrives here even outside the button.
			w.buttonUp(float32(x), float32(y))
			return 0
		}
		if w.buttonsDown > 0 {
			w.buttonsDown--
			if w.buttonsDown == 0 {
				win32.ReleaseCapture()
			}
		}
		w.emit(platform.Event{Kind: platform.EventMouseUp, X: float32(x), Y: float32(y), Button: buttonOf(msg)})
		return 0

	case win32.WM_MOUSEWHEEL, win32.WM_MOUSEHWHEEL:
		sx, sy := win32.PointFromLParam(lp)
		p := win32.POINT{X: sx, Y: sy}
		win32.ScreenToClient(hwnd, &p)
		delta := float32(int16(win32.HIWORD(wp)))
		// Detents of 120 are classic wheels; anything else is a precise device.
		precise := int(delta)%win32.WHEEL_DELTA != 0
		// One notch scrolls three 20px lines, like most native apps.
		px := delta / win32.WHEEL_DELTA * 60
		e := platform.Event{Kind: platform.EventScroll, X: float32(p.X), Y: float32(p.Y), Precise: precise}
		if msg == win32.WM_MOUSEWHEEL {
			e.DeltaY = -px
			if currentMods().Has(input.ModShift) {
				e.DeltaX, e.DeltaY = e.DeltaY, 0
			}
		} else {
			e.DeltaX = px
		}
		w.emit(e)
		return 0

	case win32.WM_SYSCOMMAND:
		// Alt pressed and released alone asks for the system menu (SC_KEYMENU
		// with no character). An app menu bar takes that key itself, so the
		// system does not open its menu; a window without one keeps it.
		if wp&0xFFF0 == win32.SC_KEYMENU && lp == 0 && showsMenuBar(w.handler) {
			return 0
		}

	case win32.WM_KEYDOWN, win32.WM_SYSKEYDOWN:
		w.emit(platform.Event{Kind: platform.EventKeyDown, Key: mapKey(wp), Repeat: lp&(1<<30) != 0})
		if msg == win32.WM_SYSKEYDOWN && !ownsF10(wp) {
			break // keep Alt+F4 and the system menu working
		}
		return 0

	case win32.WM_KEYUP, win32.WM_SYSKEYUP:
		w.emit(platform.Event{Kind: platform.EventKeyUp, Key: mapKey(wp)})
		if msg == win32.WM_SYSKEYUP && !ownsF10(wp) {
			break
		}
		return 0

	case win32.WM_CHAR:
		c := uint16(wp)
		switch {
		case utf16.IsSurrogate(rune(c)) && c < 0xDC00:
			w.highSurr = c
			return 0
		case utf16.IsSurrogate(rune(c)):
			r := utf16.DecodeRune(rune(w.highSurr), rune(c))
			w.highSurr = 0
			w.emit(platform.Event{Kind: platform.EventTextInput, Text: string(r)})
		case c >= 0x20 && c != 0x7f:
			w.emit(platform.Event{Kind: platform.EventTextInput, Text: string(rune(c))})
		}
		return 0

	case win32.WM_SETFOCUS:
		w.emit(platform.Event{Kind: platform.EventFocus})
		return 0
	case win32.WM_KILLFOCUS:
		w.caption = platform.TitleNone
		w.emit(platform.Event{Kind: platform.EventBlur})
		return 0

	case win32.WM_DROPFILES:
		paths, p := win32.DroppedFiles(wp)
		w.emit(platform.Event{Kind: platform.EventFileDrop, X: float32(p.X), Y: float32(p.Y), Paths: paths})
		return 0

	case win32.WM_SETTINGCHANGE, win32.WM_SYSCOLORCHANGE:
		// Every WM_SETTINGCHANGE is reported, not only the settings that
		// ReadSettings reads, because the application's reader is cheap. The
		// default handling below still runs for both messages.
		w.emit(platform.Event{Kind: platform.EventSettingsChanged})

	case win32.WM_GETOBJECT:
		if r, ok := w.answerAccess(wp, lp); ok {
			return r
		}

	case win32.WM_CLOSE:
		if !w.handler.OnClose(w) {
			return 0
		}
		win32.DestroyWindow(hwnd)
		return 0

	case win32.WM_DESTROY:
		delete(windows, hwnd)
		w.endAccess()
		if w.ready {
			w.ready = false
			w.handler.OnDestroy(w)
		}
		return 0
	}
	return win32.DefWindowProc(hwnd, msg, wp, lp)
}

// clientPoint converts the screen position in the lParam of a non-client
// mouse message to physical client pixels.
func (w *window) clientPoint(lp uintptr) (float32, float32) {
	sx, sy := win32.PointFromLParam(lp)
	p := win32.POINT{X: sx, Y: sy}
	win32.ScreenToClient(w.hwnd, &p)
	return float32(p.X), float32(p.Y)
}

// buttonDown starts a press on a title-bar button.
func (w *window) buttonDown(area platform.TitleArea, x, y float32, clicks int) {
	if w.buttonsDown == 0 {
		win32.SetCapture(w.hwnd)
	}
	w.buttonsDown++
	w.caption = area
	w.emit(platform.Event{Kind: platform.EventMouseDown, X: x, Y: y, Button: input.ButtonLeft, Clicks: clicks})
}

// buttonUp ends a press on a title-bar button. The button's action runs only
// when the release is over the same button, as with a client control.
func (w *window) buttonUp(x, y float32) {
	area := w.caption
	w.caption = platform.TitleNone
	if w.buttonsDown > 0 {
		w.buttonsDown--
		if w.buttonsDown == 0 {
			win32.ReleaseCapture()
		}
	}
	w.emit(platform.Event{Kind: platform.EventMouseUp, X: x, Y: y, Button: input.ButtonLeft})
	if w.titleArea(x, y) != area {
		return
	}
	switch area {
	case platform.TitleMinimize:
		w.Minimize()
	case platform.TitleMaximize:
		w.ToggleMaximize()
	case platform.TitleClose:
		w.Close()
	}
}

func buttonOf(msg uint32) input.MouseButton {
	switch msg {
	case win32.WM_RBUTTONDOWN, win32.WM_RBUTTONUP, win32.WM_RBUTTONDBLCLK:
		return input.ButtonRight
	case win32.WM_MBUTTONDOWN, win32.WM_MBUTTONUP:
		return input.ButtonMiddle
	}
	return input.ButtonLeft
}

func mapKey(vk uintptr) input.Key {
	switch {
	case vk >= 'A' && vk <= 'Z':
		return input.KeyA + input.Key(vk-'A')
	case vk >= '0' && vk <= '9':
		return input.Key0 + input.Key(vk-'0')
	case vk >= 0x70 && vk <= 0x7B:
		return input.KeyF1 + input.Key(vk-0x70)
	}
	switch vk {
	case 0x0D:
		return input.KeyEnter
	case 0x1B:
		return input.KeyEscape
	case 0x08:
		return input.KeyBackspace
	case 0x2E:
		return input.KeyDelete
	case 0x09:
		return input.KeyTab
	case 0x20:
		return input.KeySpace
	case 0x25:
		return input.KeyLeft
	case 0x27:
		return input.KeyRight
	case 0x26:
		return input.KeyUp
	case 0x28:
		return input.KeyDown
	case 0x24:
		return input.KeyHome
	case 0x23:
		return input.KeyEnd
	case 0x21:
		return input.KeyPageUp
	case 0x22:
		return input.KeyPageDown
	case 0x2D:
		return input.KeyInsert
	case 0xBB, 0x6B: // VK_OEM_PLUS, VK_ADD
		return input.KeyEqual
	case 0xBD, 0x6D: // VK_OEM_MINUS, VK_SUBTRACT
		return input.KeyMinus
	case 0x60: // VK_NUMPAD0
		return input.Key0
	case win32.VK_APPS:
		return input.KeyMenu
	}
	return input.KeyUnknown
}

// Package headless is a platform adapter whose windows exist only in memory:
// nothing appears on the screen, no window takes the keyboard focus, and the
// CPU renderer (render/soft) draws each frame into a buffer the window owns.
// The app runs as it does in a real window, so JAMB_SCRIPT, the control channel
// and screenshots work the same. JAMB_HEADLESS=1 selects it (see app/headless.go).
//
// Frames are paced at 60 per second while something animates, and drawn only
// when asked otherwise, as a real window's are. Input comes only from the app's
// tools: scripts and the control channel.
package headless

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"os"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render/soft"
)

// Scale is the ratio of physical to logical pixels of the windows opened after
// it is set. Set it before Run.
var Scale float32 = 1

// frameInterval paces the frames of a window that asks for another at once.
const frameInterval = time.Second / 60

var (
	postMu sync.Mutex
	posted []func()
	wakeCh = make(chan struct{}, 1)

	uiGoroutine atomic.Uint64 // the goroutine of the loop, while it runs
	running     bool          // Run is running; UI thread only
	quit        bool          // Quit was asked for; UI thread only
	windows     []*window     // open windows, UI thread only
	timers      []*timer      // UI thread only
	focused     *window       // the window with the keyboard, UI thread only

	start = time.Now()

	clipText    string
	clipTextSet bool
	clipImage   image.Image
)

// Run runs the loop on the calling goroutine, locked to its thread, until
// Quit. start runs on the UI thread first; its error is returned at once.
func Run(startFn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	uiGoroutine.Store(goroutineID())
	running, quit = true, false
	defer func() {
		uiGoroutine.Store(0)
		running = false
	}()
	if err := startFn(); err != nil {
		return err
	}
	for !quit {
		runPosted()
		runTimers()
		if quit {
			break
		}
		now := time.Now()
		next := time.Time{}
		for _, w := range slices.Clone(windows) {
			due, at := w.frameDue(now)
			if due {
				w.render(now)
				_, at = w.frameDue(now) // when the frame after it is due, if it asked for one
			}
			if !at.IsZero() && (next.IsZero() || at.Before(next)) {
				next = at
			}
		}
		if t, ok := nextTimer(); ok && (next.IsZero() || t.Before(next)) {
			next = t
		}
		if anyPosted() || anyWake() {
			continue
		}
		if next.IsZero() {
			<-wakeCh
			continue
		}
		select {
		case <-wakeCh:
		case <-time.After(time.Until(next)):
		}
	}
	return nil
}

// Quit ends the loop when it next runs. It is safe from any goroutine.
func Quit() { Post(func() { quit = true }) }

// OnUIThread reports whether the caller runs on the loop's goroutine.
func OnUIThread() bool {
	id := uiGoroutine.Load()
	return id != 0 && id == goroutineID()
}

// Post runs fn on the UI thread, in order with the other posted functions. It
// is safe from any goroutine.
func Post(fn func()) {
	postMu.Lock()
	posted = append(posted, fn)
	postMu.Unlock()
	signal()
}

// Clock returns a monotonic clock in nanoseconds, the clock of Event.Arrived.
func Clock() int64 { return int64(time.Since(start)) }

func signal() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

func anyPosted() bool {
	postMu.Lock()
	defer postMu.Unlock()
	return len(posted) > 0
}

// anyWake reports whether a window was woken and is due a frame now.
func anyWake() bool {
	for _, w := range windows {
		if w.ready && !w.destroyed && w.Visible() && w.wakePending.Load() {
			return true
		}
	}
	return false
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

// goroutineID returns the current goroutine's ID, read from its stack header
// ("goroutine 18 [running]:"). It is used only to answer OnUIThread.
func goroutineID() uint64 {
	var buf [64]byte
	b := buf[:runtime.Stack(buf[:], false)]
	b, _ = bytes.CutPrefix(b, []byte("goroutine "))
	if i := bytes.IndexByte(b, ' '); i > 0 {
		b = b[:i]
	}
	id, _ := strconv.ParseUint(string(b), 10, 64)
	return id
}

type timer struct {
	at      time.Time
	fn      func()
	stopped bool
}

func after(d time.Duration, fn func()) *timer {
	t := &timer{at: time.Now().Add(d), fn: fn}
	timers = append(timers, t)
	return t
}

func (t *timer) stop() {
	if t != nil {
		t.stopped = true
	}
}

func nextTimer() (time.Time, bool) {
	var next time.Time
	for _, t := range timers {
		if !t.stopped && (next.IsZero() || t.at.Before(next)) {
			next = t.at
		}
	}
	return next, !next.IsZero()
}

func runTimers() {
	now := time.Now()
	var due []*timer
	keep := timers[:0]
	for _, t := range timers {
		switch {
		case t.stopped:
		case !t.at.After(now):
			due = append(due, t)
		default:
			keep = append(keep, t)
		}
	}
	clear(timers[len(keep):])
	timers = keep
	for _, t := range due {
		if !t.stopped {
			t.fn()
		}
	}
}

// window is a window in memory.
type window struct {
	handler platform.Handler
	opts    platform.WindowOptions
	title   string
	logW    int // the size in logical pixels
	logH    int
	scale   float32

	ready, hidden, minimized, maximized, fullscreen bool
	closing, destroyed                              bool

	frameWanted bool // OnFrame asked for another frame
	wakePending atomic.Bool
	lastFrame   time.Time
	wakeAt      time.Time
	wakeTmr     *timer

	buf    []byte
	stride int
}

// Open creates a window. It must be called on the UI thread, inside Run. The
// handler's OnCreate and the first frame run before it returns, and the window
// takes the keyboard, as a newly opened window does.
func Open(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	if !running {
		return nil, errors.New("platform/headless: Open called outside Run")
	}
	w := &window{handler: h, opts: opts, title: opts.Title, scale: Scale}
	w.logW, w.logH = max(opts.Width, opts.MinWidth, 1), max(opts.Height, opts.MinHeight, 1)
	windows = append(windows, w)
	if err := h.OnCreate(w); err != nil {
		w.destroy()
		return nil, err
	}
	w.ready = true
	w.render(time.Now())
	w.Focus()
	return w, nil
}

// frameDue reports whether the window draws a frame now, or else when its
// next paced frame is due (zero when it waits for a wake).
func (w *window) frameDue(now time.Time) (bool, time.Time) {
	if !w.ready || w.destroyed || !w.Visible() {
		return false, time.Time{}
	}
	if w.wakePending.Load() {
		return true, time.Time{}
	}
	if w.frameWanted {
		at := w.lastFrame.Add(frameInterval)
		if !now.Before(at) {
			return true, time.Time{}
		}
		return false, at
	}
	return false, time.Time{}
}

func (w *window) render(now time.Time) {
	w.wakePending.Store(false)
	w.lastFrame = now
	w.frameWanted = w.handler.OnFrame(w)
}

func (w *window) emit(e platform.Event) {
	if w.destroyed {
		return
	}
	if e.Arrived == 0 {
		e.Arrived = Clock()
	}
	w.handler.OnEvent(w, e)
}

func (w *window) requestClose() {
	if w.closing || w.destroyed {
		return
	}
	w.closing = true
	ok := w.handler.OnClose(w)
	w.closing = false
	if ok {
		w.destroy()
		w.handler.OnDestroy(w)
	}
}

func (w *window) destroy() {
	if w.destroyed {
		return
	}
	w.destroyed = true
	w.wakeTmr.stop()
	windows = slices.DeleteFunc(windows, func(o *window) bool { return o == w })
	if focused == w {
		focused = nil
	}
	w.buf = nil
}

// ---- soft.Target ----

// Target returns where the CPU renderer draws the window's frames: w itself.
func (w *window) Target() soft.Target { return w }

// Buffer implements soft.Target with a buffer the window keeps.
func (w *window) Buffer(width, height int) ([]byte, int, error) {
	if width <= 0 || height <= 0 {
		return nil, 0, fmt.Errorf("platform/headless: no buffer of %d×%d", width, height)
	}
	n := width * height * 4
	if cap(w.buf) < n {
		w.buf = make([]byte, n)
	}
	w.buf, w.stride = w.buf[:n], width*4
	return w.buf, w.stride, nil
}

// Present implements soft.Target. The frame stays in the buffer, where a
// capture (render.Renderer.CaptureNextFrame) reads it.
func (w *window) Present() error { return nil }

// ---- platform.Window ----

func (w *window) Handle() uintptr { return 0 }

func (w *window) Size() (int, int) {
	return int(float32(w.logW)*w.scale + 0.5), int(float32(w.logH)*w.scale + 0.5)
}

func (w *window) Scale() float32 { return w.scale }

func (w *window) SetTitle(t string) { w.title = t }

func (w *window) Frame() platform.Rect {
	pw, ph := w.Size()
	return platform.Rect{W: pw, H: ph}
}

func (w *window) ClientOrigin() platform.Point { return platform.Point{} }

func (w *window) SetCursor(c input.Cursor) {}

func (w *window) Show() {
	if w.destroyed {
		return
	}
	w.hidden, w.minimized = false, false
	w.Wake()
}

func (w *window) Close() { Post(w.requestClose) }

func (w *window) Minimize() { w.minimized = true }

func (w *window) ToggleMaximize() { w.maximized = !w.maximized }

func (w *window) Maximized() bool { return w.maximized }

// SetSize resizes the window, in logical pixels, as a person dragging its
// corner would.
func (w *window) SetSize(width, height int) {
	if width <= 0 || height <= 0 || w.destroyed {
		return
	}
	w.logW, w.logH = max(width, w.opts.MinWidth), max(height, w.opts.MinHeight)
	w.emit(platform.Event{Kind: platform.EventResize})
	w.Wake()
}

func (w *window) SetPosition(x, y int) {}

func (w *window) SetFullscreen(on bool) { w.fullscreen = on }

func (w *window) Fullscreen() bool { return w.fullscreen }

func (w *window) SetDark(on bool) {}

// OpenURL opens nothing: a headless run must not start a browser. It reports
// the URL on stderr.
func (w *window) OpenURL(url string) error {
	fmt.Fprintln(os.Stderr, "jamb: headless: not opening", url)
	return nil
}

// Focus gives the window the keyboard, and takes it from the window that had
// it, as the system does when a window is activated.
func (w *window) Focus() {
	if w.destroyed || focused == w {
		return
	}
	if old := focused; old != nil {
		focused = nil
		old.emit(platform.Event{Kind: platform.EventBlur})
	}
	focused = w
	w.emit(platform.Event{Kind: platform.EventFocus})
}

func (w *window) Wake() {
	if w.wakePending.CompareAndSwap(false, true) {
		signal()
	}
}

func (w *window) WakeAt(t time.Time) {
	if !w.Visible() || w.destroyed {
		return
	}
	if !w.wakeAt.IsZero() && !t.Before(w.wakeAt) {
		return
	}
	w.wakeAt = t
	w.wakeTmr.stop()
	w.wakeTmr = after(time.Until(t), func() {
		w.wakeAt = time.Time{}
		w.wakeTmr = nil
		w.wakePending.Store(true)
	})
}

// The clipboard is the process's own, so a headless run never reads or
// changes the system's.
func (w *window) Clipboard() (string, bool)           { return clipText, clipTextSet }
func (w *window) SetClipboard(text string)            { clipText, clipTextSet, clipImage = text, true, nil }
func (w *window) ClipboardImage() (image.Image, bool) { return clipImage, clipImage != nil }
func (w *window) HasClipboardImage() bool             { return clipImage != nil }
func (w *window) SetClipboardImage(img image.Image) {
	clipImage, clipText, clipTextSet = img, "", false
}

func (w *window) SetTextInputRect(r platform.Rect) {}
func (w *window) CancelComposition()               {}

// ShowFileDialog answers at once as a canceled dialog: a headless run shows
// no system dialogs.
func (w *window) ShowFileDialog(kind dialog.FileKind, o dialog.FileOptions, done func(dialog.FileResult)) {
	Post(func() { done(dialog.FileResult{Canceled: true}) })
}

// ShowMessageBox answers at once with no button, as a box that could not be
// shown does.
func (w *window) ShowMessageBox(o dialog.MessageOptions, done func(dialog.Button)) {
	Post(func() { done(dialog.NoButton) })
}

func (w *window) Hide() { w.hidden = true }

func (w *window) Restore() {
	w.Show()
	w.maximized = false
}

func (w *window) Visible() bool { return !w.hidden && !w.minimized && !w.destroyed }

func (w *window) Shown() bool { return !w.hidden && !w.destroyed }

func (w *window) SetTaskbarProgress(state platform.TaskbarState, value float32) {}

func (w *window) SetTaskbarOverlay(img image.Image, description string) error { return nil }

package app

import (
	"sync"
	"sync/atomic"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/ui"
)

// appWindow is the ui.Window of one window: the value Open returns, and the
// window that ctx.Window() reaches inside it. Every window has one.
//
// Its methods may be called from any goroutine. An action runs on the UI
// thread: at once when it is called there and the window exists, else posted
// in call order, so an action right after Open runs once the window is open. A
// read from another goroutine is answered by the UI thread, and the caller
// waits for it.
type appWindow struct {
	host   *host // the window while it exists; read and written on the UI thread only
	closed atomic.Bool
}

// uiDone is closed when the UI loop has ended, so a read that waits for the UI
// thread does not wait forever after the app quits.
var (
	uiDone     = make(chan struct{})
	uiDoneOnce sync.Once
)

func endUI() { uiDoneOnce.Do(func() { close(uiDone) }) }

// do runs fn with the window on the UI thread. It does nothing once the window
// has closed.
func (w *appWindow) do(fn func(h *host)) {
	if onUIThread() && w.host != nil {
		fn(w.host)
		return
	}
	postUI(func() {
		if w.host != nil {
			fn(w.host)
		}
	})
}

// read returns fn's result for the window, computed on the UI thread, or zero
// when the window does not exist (yet, or any more) or the app has quit.
func read[T any](w *appWindow, zero T, fn func(h *host) T) T {
	if onUIThread() {
		if w.host == nil {
			return zero
		}
		return fn(w.host)
	}
	if w.closed.Load() {
		return zero
	}
	ch := make(chan T, 1)
	postUI(func() {
		if w.host == nil {
			ch <- zero
			return
		}
		ch <- fn(w.host)
	})
	select {
	case v := <-ch:
		return v
	case <-uiDone:
		return zero
	}
}

func (w *appWindow) Minimize()             { w.do(func(h *host) { h.win.Minimize() }) }
func (w *appWindow) ToggleMaximize()       { w.do(func(h *host) { h.win.ToggleMaximize() }) }
func (w *appWindow) Focus()                { w.do(func(h *host) { h.win.Focus() }) }
func (w *appWindow) SetTitle(title string) { w.do(func(h *host) { h.win.SetTitle(title) }) }
func (w *appWindow) Closed() bool          { return w.closed.Load() }

// Close closes the window even when it is a CloseToTray window; see host.OnClose.
func (w *appWindow) Close() { w.do(func(h *host) { h.closeAsked = true; h.win.Close() }) }

// RequestClose closes the window as its close button does: OnClose is asked, and
// CloseToTray may hide it instead (see ui.Window).
func (w *appWindow) RequestClose() { w.do(func(h *host) { h.closeAsked = false; h.win.Close() }) }

// Show and Hide implement ui.Window.
func (w *appWindow) Show() { w.do(func(h *host) { h.win.Restore() }) }
func (w *appWindow) Hide() { w.do(func(h *host) { h.win.Hide() }) }

func (w *appWindow) Maximized() bool {
	return read(w, false, func(h *host) bool { return h.win.Maximized() })
}

func (w *appWindow) Focused() bool { return read(w, false, func(h *host) bool { return h.focused }) }

// Position and Size are in logical pixels: the window's pixels divided by its
// scale, as Window's X, Y, Width and Height are.
func (w *appWindow) Position() (x, y int) {
	p := read(w, [2]int{}, func(h *host) [2]int {
		f, s := h.win.Frame(), h.win.Scale()
		return [2]int{int(float32(f.X)/s + 0.5), int(float32(f.Y)/s + 0.5)}
	})
	return p[0], p[1]
}

func (w *appWindow) Size() (width, height int) {
	p := read(w, [2]int{}, func(h *host) [2]int {
		cw, ch := h.win.Size()
		s := h.win.Scale()
		return [2]int{int(float32(cw)/s + 0.5), int(float32(ch)/s + 0.5)}
	})
	return p[0], p[1]
}

func (w *appWindow) SetPosition(x, y int) { w.do(func(h *host) { h.win.SetPosition(x, y) }) }
func (w *appWindow) SetSize(width, height int) {
	w.do(func(h *host) { h.win.SetSize(width, height) })
}
func (w *appWindow) SetFullscreen(on bool) { w.do(func(h *host) { h.win.SetFullscreen(on) }) }
func (w *appWindow) Fullscreen() bool {
	return read(w, false, func(h *host) bool { return h.win.Fullscreen() })
}

func (w *appWindow) OpenFiles(o dialog.FileOptions, done func(dialog.FileResult)) {
	w.do(func(h *host) { h.win.ShowFileDialog(dialog.Open, o, done) })
}

func (w *appWindow) SaveFile(o dialog.FileOptions, done func(dialog.FileResult)) {
	w.do(func(h *host) { h.win.ShowFileDialog(dialog.Save, o, done) })
}

func (w *appWindow) PickFolder(o dialog.FileOptions, done func(dialog.FileResult)) {
	w.do(func(h *host) { h.win.ShowFileDialog(dialog.Folder, o, done) })
}

func (w *appWindow) MessageBox(o dialog.MessageOptions, done func(dialog.Button)) {
	w.do(func(h *host) { h.win.ShowMessageBox(o, done) })
}

var _ ui.Window = (*appWindow)(nil)

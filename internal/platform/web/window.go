//go:build js

package web

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync/atomic"
	"syscall/js"
	"time"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render/soft"
)

// window is a canvas of the page.
type window struct {
	handler platform.Handler
	opts    platform.WindowOptions

	canvas js.Value
	ctx    js.Value // the canvas's 2D context
	owned  bool     // the window made the canvas, and removes it when it closes

	// The size of the canvas in CSS pixels and its device pixel ratio, read
	// on the UI goroutine when the page reports a change.
	cssW, cssH float64
	scale      float32

	ready, hidden, destroyed, closing bool

	frameWanted bool        // OnFrame asked for another frame
	wakePending atomic.Bool // Wake asked for a frame
	rafAsked    atomic.Bool // a requestAnimationFrame callback is pending
	rafFired    atomic.Bool // it ran: the display is ready for a frame
	rafFunc     js.Func
	wakeAt      time.Time
	wakeTmr     *timer

	// The frame the CPU renderer draws, premultiplied BGRA, and the RGBA copy
	// that goes to the canvas.
	buf, rgba          []byte
	stride, bufW, bufH int
	imageData, view    js.Value // the canvas's ImageData, and a Uint8Array over its pixels

	funcs    []js.Func // listeners, released when the window closes
	removers []func()  // remove the listeners from their targets
	in       pointerState

	clipText string // the page's own clipboard (see Clipboard)
}

// Open makes the page's window. It must be called on the UI goroutine, inside
// Run. A page holds one window, so Open fails while one is open.
func Open(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	if !running {
		return nil, errors.New("platform/web: Open called outside Run")
	}
	if len(windows) > 0 {
		return nil, errors.New("platform/web: a web page holds one window")
	}
	doc := js.Global().Get("document")
	w := &window{handler: h, opts: opts}
	if c := doc.Call("getElementById", "jamb"); c.Truthy() && c.Get("tagName").String() == "CANVAS" {
		w.canvas = c
	} else {
		// The window is the whole page.
		w.canvas, w.owned = doc.Call("createElement", "canvas"), true
		st := w.canvas.Get("style")
		st.Set("position", "fixed")
		st.Set("inset", "0")
		st.Set("width", "100vw")
		st.Set("height", "100vh")
		st.Set("display", "block")
		doc.Get("body").Call("appendChild", w.canvas)
	}
	st := w.canvas.Get("style")
	st.Set("outline", "none")
	st.Set("touchAction", "none") // the app handles touches; the page does not scroll or zoom
	st.Set("userSelect", "none")
	w.canvas.Set("tabIndex", 0)
	w.ctx = w.canvas.Call("getContext", "2d", map[string]any{"alpha": false})
	if !w.ctx.Truthy() {
		return nil, errors.New("platform/web: the canvas has no 2D context")
	}
	w.SetTitle(opts.Title)
	w.measure()
	w.rafFunc = js.FuncOf(func(js.Value, []js.Value) any {
		w.rafAsked.Store(false)
		w.rafFired.Store(true)
		signal()
		return nil
	})
	w.listen()
	windows = append(windows, w)
	if err := h.OnCreate(w); err != nil {
		w.destroy()
		return nil, err
	}
	w.ready = true
	w.Wake()
	w.canvas.Call("focus")
	return w, nil
}

// measure reads the canvas's size and the device pixel ratio.
func (w *window) measure() {
	w.cssW = w.canvas.Get("clientWidth").Float()
	w.cssH = w.canvas.Get("clientHeight").Float()
	w.scale = float32(js.Global().Get("devicePixelRatio").Float())
	if w.scale <= 0 {
		w.scale = 1
	}
}

// resized handles a change of the canvas's size or of the device pixel ratio.
func (w *window) resized() {
	oldW, oldH, oldScale := w.cssW, w.cssH, w.scale
	w.measure()
	if w.scale != oldScale {
		w.emit(platform.Event{Kind: platform.EventScaleChanged})
	}
	if w.cssW != oldW || w.cssH != oldH || w.scale != oldScale {
		w.emit(platform.Event{Kind: platform.EventResize})
	}
	w.Wake()
}

// frame draws a frame when one is wanted and the display is ready for it, and
// otherwise asks the browser for the next display frame when one is wanted.
func (w *window) frame() {
	if !w.ready || w.destroyed || !w.Visible() {
		return
	}
	if !w.wakePending.Load() && !w.frameWanted {
		return
	}
	if w.rafFired.Swap(false) {
		w.wakePending.Store(false)
		w.frameWanted = w.handler.OnFrame(w)
		if !w.wakePending.Load() && !w.frameWanted {
			return
		}
	}
	if w.rafAsked.CompareAndSwap(false, true) {
		js.Global().Call("requestAnimationFrame", w.rafFunc)
	}
}

// anyFrameReady reports whether a window can draw a wanted frame now.
func anyFrameReady() bool {
	for _, w := range windows {
		if w.ready && !w.destroyed && w.Visible() && w.rafFired.Load() && (w.wakePending.Load() || w.frameWanted) {
			return true
		}
	}
	return false
}

func (w *window) emit(e platform.Event) {
	if w.destroyed || !w.ready {
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
	w.unlisten()
	w.rafFunc.Release()
	if w.owned {
		w.canvas.Call("remove")
	}
	for i, o := range windows {
		if o == w {
			windows = append(windows[:i], windows[i+1:]...)
			break
		}
	}
	w.buf, w.rgba = nil, nil
}

// ---- soft.Target ----

// Target returns where the CPU renderer draws the window's frames: w itself.
func (w *window) Target() soft.Target { return w }

// Buffer implements soft.Target. It also sizes the canvas's backing store to
// the frame.
func (w *window) Buffer(width, height int) ([]byte, int, error) {
	if width <= 0 || height <= 0 {
		return nil, 0, fmt.Errorf("platform/web: no buffer of %d×%d", width, height)
	}
	n := width * height * 4
	if cap(w.buf) < n {
		w.buf, w.rgba = make([]byte, n), make([]byte, n)
	}
	w.buf, w.rgba, w.stride = w.buf[:n], w.rgba[:n], width*4
	if width != w.bufW || height != w.bufH {
		w.bufW, w.bufH = width, height
		w.canvas.Set("width", width)
		w.canvas.Set("height", height)
		w.imageData = w.ctx.Call("createImageData", width, height)
		w.view = js.Global().Get("Uint8Array").New(w.imageData.Get("data").Get("buffer"))
	}
	return w.buf, w.stride, nil
}

// Present implements soft.Target: it copies the frame to the canvas. The
// renderer draws BGRA and the canvas takes RGBA. A window is opaque, so the
// premultiplied colors are the canvas's straight ones, and alpha is 255.
func (w *window) Present() error {
	if w.imageData.IsUndefined() {
		return nil
	}
	src, dst := w.buf, w.rgba
	for i := 0; i+4 <= len(src); i += 4 {
		dst[i], dst[i+1], dst[i+2], dst[i+3] = src[i+2], src[i+1], src[i], 255
	}
	js.CopyBytesToJS(w.view, dst)
	w.ctx.Call("putImageData", w.imageData, 0, 0)
	return nil
}

// ---- platform.Window ----

func (w *window) Handle() uintptr { return 0 }

// Size returns the canvas's size in device pixels.
func (w *window) Size() (int, int) {
	return int(math.Round(w.cssW * float64(w.scale))), int(math.Round(w.cssH * float64(w.scale)))
}

func (w *window) Scale() float32 { return w.scale }

func (w *window) SetTitle(t string) {
	if t != "" {
		js.Global().Get("document").Set("title", t)
	}
}

func (w *window) Frame() platform.Rect {
	pw, ph := w.Size()
	return platform.Rect{W: pw, H: ph}
}

// ClientOrigin returns the canvas's position in the browser's viewport.
func (w *window) ClientOrigin() platform.Point {
	r := w.canvas.Call("getBoundingClientRect")
	return platform.Point{X: int(r.Get("left").Float()), Y: int(r.Get("top").Float())}
}

var cursors = map[input.Cursor]string{
	input.CursorDefault:    "default",
	input.CursorPointer:    "pointer",
	input.CursorText:       "text",
	input.CursorResizeEW:   "ew-resize",
	input.CursorResizeNS:   "ns-resize",
	input.CursorNotAllowed: "not-allowed",
}

func (w *window) SetCursor(c input.Cursor) {
	name, ok := cursors[c]
	if !ok {
		name = "default"
	}
	w.canvas.Get("style").Set("cursor", name)
}

func (w *window) Show() {
	if w.destroyed {
		return
	}
	w.hidden = false
	w.canvas.Get("style").Set("visibility", "visible")
	w.Wake()
}

func (w *window) Close() { Post(w.requestClose) }

// A page's window is as large as the page lets it be: it does not minimize,
// maximize, resize or move itself.
func (w *window) Minimize()                 {}
func (w *window) ToggleMaximize()           {}
func (w *window) Maximized() bool           { return false }
func (w *window) SetSize(width, height int) {}
func (w *window) SetPosition(x, y int)      {}
func (w *window) SetDark(on bool)           {}

func (w *window) SetFullscreen(on bool) {
	doc := js.Global().Get("document")
	switch {
	case on && !w.Fullscreen():
		if f := w.canvas.Get("requestFullscreen"); f.Type() == js.TypeFunction {
			w.canvas.Call("requestFullscreen")
		}
	case !on && w.Fullscreen():
		doc.Call("exitFullscreen")
	}
}

func (w *window) Fullscreen() bool {
	return js.Global().Get("document").Get("fullscreenElement").Equal(w.canvas)
}

// OpenURL opens url in a new tab.
func (w *window) OpenURL(url string) error {
	js.Global().Call("open", url, "_blank", "noopener,noreferrer")
	return nil
}

func (w *window) Focus() { w.canvas.Call("focus") }

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

// The clipboard is the page's own for now: text copied in the app pastes in
// the app. Copied text is also offered to the browser's clipboard, which may
// refuse it outside a user gesture.
func (w *window) Clipboard() (string, bool) { return w.clipText, w.clipText != "" }

func (w *window) SetClipboard(text string) {
	w.clipText = text
	if c := js.Global().Get("navigator").Get("clipboard"); c.Truthy() {
		p := c.Call("writeText", text)
		var ignore js.Func
		ignore = js.FuncOf(func(js.Value, []js.Value) any { ignore.Release(); return nil })
		p.Call("catch", ignore)
	}
}

func (w *window) ClipboardImage() (image.Image, bool) { return nil, false }
func (w *window) HasClipboardImage() bool             { return false }
func (w *window) SetClipboardImage(img image.Image)   {}

func (w *window) SetTextInputRect(r platform.Rect) {}
func (w *window) CancelComposition()               {}

// ShowFileDialog answers as canceled: a page's file dialogs come later.
func (w *window) ShowFileDialog(kind dialog.FileKind, o dialog.FileOptions, done func(dialog.FileResult)) {
	Post(func() { done(dialog.FileResult{Canceled: true}) })
}

// ShowMessageBox answers with no button, as a box that could not be shown does.
func (w *window) ShowMessageBox(o dialog.MessageOptions, done func(dialog.Button)) {
	Post(func() { done(dialog.NoButton) })
}

func (w *window) Hide() {
	w.hidden = true
	w.canvas.Get("style").Set("visibility", "hidden")
}

func (w *window) Restore() { w.Show() }

// Visible reports whether the window shows: not hidden, in a page that is not
// in a background tab.
func (w *window) Visible() bool {
	return !w.hidden && !w.destroyed && js.Global().Get("document").Get("visibilityState").String() != "hidden"
}

func (w *window) Shown() bool { return !w.hidden && !w.destroyed }

func (w *window) SetTaskbarProgress(state platform.TaskbarState, value float32) {}

func (w *window) SetTaskbarOverlay(img image.Image, description string) error { return nil }

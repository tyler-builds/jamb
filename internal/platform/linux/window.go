//go:build linux

package linux

import (
	"errors"
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync/atomic"
	"time"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/platform/linux/wl"
	"github.com/tyler-builds/jamb/internal/render/soft"
)

// xdg_toplevel states.
const (
	stateMaximized  = 1
	stateFullscreen = 2
	stateActivated  = 4
	stateSuspended  = 9
)

type window struct {
	d       *display
	handler platform.Handler
	opts    platform.WindowOptions

	surface    *wl.Proxy
	xdgSurface *wl.Proxy
	// toplevelSession is the window's entry in the window session (see
	// session.go), or nil.
	toplevelSession *wl.Proxy
	toplevel        *wl.Proxy
	decoration      *wl.Proxy
	fractional      *wl.Proxy
	viewport        *wl.Proxy
	swap            *swapchain

	// The size in logical (surface) pixels, and the scale: physical pixels
	// are their product. scale120 is the fractional scale in 120ths, or 0.
	logW, logH int
	intScale   int
	scale120   uint32

	configured  bool // the first configure has been acked
	ready       bool // OnCreate has returned
	mapped      bool // a buffer is attached and the window is shown
	hidden      bool // Hide was called
	closing     bool
	destroyed   bool
	frameWanted bool        // OnFrame asked for another frame
	wakePending atomic.Bool // Wake was called
	callback    *wl.Proxy   // the pending frame callback, or nil
	callbackAt  time.Time
	needsDraw   bool // the size or scale changed: draw even without a request

	maximized, fullscreen, activated, suspended bool
	// serverFrame is set when the compositor draws the window's frame.
	serverFrame bool
	// ownTitleBar is set when the window asked for the system's title bar and
	// the compositor draws none: the app draws one (see NeedsTitleBar).
	ownTitleBar bool

	cursor  input.Cursor
	wakeAt  time.Time
	wakeTmr *timer
	title   string
}

// Open creates a window. It must be called on the UI thread. The handler's
// OnCreate runs before the window appears, and its first frame is drawn
// before that too.
func Open(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	d := disp
	if d == nil {
		return nil, errors.New("platform/linux: Open called outside Run")
	}
	w := &window{d: d, handler: h, opts: opts, intScale: 1, title: opts.Title}
	w.logW, w.logH = max(opts.Width, 1), max(opts.Height, 1)
	if f := opts.Frame; f.W > 0 && f.H > 0 {
		// A saved frame is in physical pixels at the scale it was saved at;
		// the scale is not known yet, so it is taken as logical.
		w.logW, w.logH = f.W, f.H
	}
	c := d.conn
	w.surface = c.NewProxy("wl_surface", d.compositor.Version, w.surfaceEvent)
	d.compositor.Send(0, wl.NewID{P: w.surface}) // create_surface
	if d.fractional != nil {
		w.fractional = c.NewProxy("wp_fractional_scale_v1", 1, w.fractionalEvent)
		d.fractional.Send(1, wl.NewID{P: w.fractional}, w.surface) // get_fractional_scale
		w.viewport = c.NewProxy("wp_viewport", 1, nil)
		d.viewporter.Send(1, wl.NewID{P: w.viewport}, w.surface) // get_viewport
	}
	w.xdgSurface = c.NewProxy("xdg_surface", d.wmBase.Version, w.xdgSurfaceEvent)
	d.wmBase.Send(2, wl.NewID{P: w.xdgSurface}, w.surface) // get_xdg_surface
	w.toplevel = c.NewProxy("xdg_toplevel", d.wmBase.Version, w.toplevelEvent)
	w.xdgSurface.Send(1, wl.NewID{P: w.toplevel}) // get_toplevel
	w.toplevel.Send(2, opts.Title)                // set_title
	w.toplevel.Send(3, appID())                   // set_app_id
	if opts.MinWidth > 0 || opts.MinHeight > 0 {
		w.toplevel.Send(8, int32(opts.MinWidth), int32(opts.MinHeight)) // set_min_size
	}
	if d.decoration != nil {
		w.decoration = c.NewProxy("zxdg_toplevel_decoration_v1", 1, func(op uint16, dec *wl.Decoder) {
			if op == 0 { // configure(mode)
				w.serverFrame = dec.Uint() == 2
			}
		})
		d.decoration.Send(1, wl.NewID{P: w.decoration}, w.toplevel) // get_toplevel_decoration
		mode := uint32(2)                                           // server_side
		if opts.CustomTitleBar {
			mode = 1 // client_side: the app draws its own title bar
		}
		w.decoration.Send(1, mode) // set_mode
	}
	w.swap = &swapchain{w: w}
	d.placeWindow(w) // before the first commit, as restoring requires
	d.windows = append(d.windows, w)

	// The initial commit, with no buffer, asks for the first configure.
	w.surface.Send(6) // commit
	for !w.configured {
		if err := c.Roundtrip(); err != nil {
			w.destroy()
			return nil, err
		}
	}
	if !w.opts.CustomTitleBar && !w.serverFrame {
		// The compositor draws no frame, as GNOME's does not: the window gets
		// the client's frame, as a custom title bar does, and the app draws a
		// title bar for it (see NeedsTitleBar).
		w.opts.CustomTitleBar = true
		w.ownTitleBar = true
	}
	if err := h.OnCreate(w); err != nil {
		w.destroy()
		return nil, err
	}
	w.ready = true
	// Draw the first frame before the window appears, so it never shows empty.
	w.render()
	return w, nil
}

// appIDOverride is the app ID that SetAppID set, or "". UI thread only.
var appIDOverride string

// SetAppID names the app for the compositor, in place of the executable's name.
// It applies to the windows opened after it, so call it before the first. The
// desktop matches it to the .desktop file of the same name, which gives the
// app's icon and name in the task manager.
func SetAppID(id string) { appIDOverride = id }

// appID names the app for the compositor (its icon and its .desktop file): the
// ID that SetAppID set, else the executable's name.
func appID() string {
	if appIDOverride != "" {
		return appIDOverride
	}
	exe, err := os.Executable()
	if err != nil {
		return "jamb"
	}
	return filepath.Base(exe)
}

func (w *window) surfaceEvent(op uint16, d *wl.Decoder) {
	switch op {
	case 2: // preferred_buffer_scale(factor), wl_compositor 6
		if s := int(d.Int()); s > 0 && s != w.intScale {
			w.intScale = s
			if w.scale120 == 0 {
				w.scaleChanged()
			}
		}
	}
}

func (w *window) fractionalEvent(op uint16, d *wl.Decoder) {
	if op == 0 { // preferred_scale(scale), in 120ths
		if s := d.Uint(); s > 0 && s != w.scale120 {
			w.scale120 = s
			w.scaleChanged()
		}
	}
}

func (w *window) scaleChanged() {
	w.needsDraw = true
	if w.ready {
		w.emit(platform.Event{Kind: platform.EventScaleChanged})
	}
}

func (w *window) xdgSurfaceEvent(op uint16, d *wl.Decoder) {
	if op == 0 { // configure(serial)
		w.xdgSurface.Send(4, d.Uint()) // ack_configure
		w.configured = true
		w.needsDraw = true
	}
}

func (w *window) toplevelEvent(op uint16, d *wl.Decoder) {
	switch op {
	case 0: // configure(width, height, states)
		width, height := int(d.Int()), int(d.Int())
		states := d.Array()
		maximized, fullscreen, activated, suspended := false, false, false, false
		for i := 0; i+4 <= len(states); i += 4 {
			switch uint32(states[i]) | uint32(states[i+1])<<8 | uint32(states[i+2])<<16 | uint32(states[i+3])<<24 {
			case stateMaximized:
				maximized = true
			case stateFullscreen:
				fullscreen = true
			case stateActivated:
				activated = true
			case stateSuspended:
				suspended = true
			}
		}
		stateChanged := maximized != w.maximized || fullscreen != w.fullscreen
		w.maximized, w.fullscreen, w.activated, w.suspended = maximized, fullscreen, activated, suspended
		resized := false
		if width > 0 && height > 0 && (width != w.logW || height != w.logH) {
			w.logW, w.logH = max(width, w.opts.MinWidth, 1), max(height, w.opts.MinHeight, 1)
			resized = true
		}
		if w.ready {
			if stateChanged {
				w.emit(platform.Event{Kind: platform.EventWindowState})
			}
			if resized {
				w.emit(platform.Event{Kind: platform.EventResize})
			}
		}
	case 1: // close
		w.requestClose()
	}
}

// scale returns the ratio of physical to logical pixels.
func (w *window) scale() float64 {
	if w.scale120 > 0 {
		return float64(w.scale120) / 120
	}
	return float64(w.intScale)
}

// physical converts a logical length to physical pixels, as the compositor
// rounds a fractional scale.
func (w *window) physical(v int) int {
	return int(math.Round(float64(v) * w.scale()))
}

// frameDue reports whether the loop should render the window now.
func (w *window) frameDue() bool {
	if !w.ready || w.destroyed || w.hidden || !w.configured {
		return false
	}
	if !(w.frameWanted || w.wakePending.Load() || w.needsDraw) {
		return false
	}
	// A frame callback that never arrives (the compositor stops sending them
	// to a window that is not on screen) must not stall the window for good:
	// after a second, draw anyway.
	return w.callback == nil || w.needsDraw || time.Since(w.callbackAt) > time.Second
}

// render calls the handler to draw one frame.
func (w *window) render() {
	w.wakePending.Store(false)
	w.needsDraw = false
	w.frameWanted = w.handler.OnFrame(w)
}

// present attaches a drawn buffer to the surface and commits it, asking for a
// frame callback to pace the next frame.
func (w *window) present(b *shmBuffer) {
	if w.destroyed {
		return
	}
	s := w.surface
	if w.viewport != nil && w.scale120 > 0 {
		s.Send(8, int32(1))                              // set_buffer_scale
		w.viewport.Send(2, int32(w.logW), int32(w.logH)) // set_destination
	} else {
		if w.viewport != nil {
			w.viewport.Send(2, int32(-1), int32(-1)) // unset the destination
		}
		s.Send(8, int32(w.intScale)) // set_buffer_scale
	}
	s.Send(1, b.buf, int32(0), int32(0))                           // attach
	s.Send(9, int32(0), int32(0), int32(b.width), int32(b.height)) // damage_buffer
	if w.callback == nil {
		cb := w.d.conn.NewProxy("wl_callback", 1, nil)
		cb.On = func(op uint16, d *wl.Decoder) {
			if w.callback == cb {
				w.callback = nil
			}
		}
		s.Send(3, wl.NewID{P: cb}) // frame
		w.callback, w.callbackAt = cb, time.Now()
	}
	s.Send(6) // commit
	w.mapped = true
}

// clientFrame reports whether the window draws its own frame, so it resizes
// itself from its edges: a custom title bar the compositor does not cover.
// NeedsTitleBar reports whether the window asked for the system's title bar on
// a compositor that draws none, such as GNOME's: the app draws a title bar for
// it, which the window treats as a custom one (moves, caption buttons, resize
// edges). The host asks it after the window is created.
func (w *window) NeedsTitleBar() bool { return w.ownTitleBar }

func (w *window) clientFrame() bool { return w.opts.CustomTitleBar && !w.serverFrame }

// renderer returns the target the CPU renderer draws the window's frames into.
func (w *window) target() soft.Target { return w.swap }

func (w *window) emit(e platform.Event) {
	if w.destroyed {
		return
	}
	if w.d.seat != nil && w.d.seat.kb != nil {
		e.Mods = w.d.seat.kb.mods
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

// destroy releases the window's objects.
func (w *window) destroy() {
	if w.destroyed {
		return
	}
	w.destroyed = true
	w.wakeTmr.stop()
	w.swap.close()
	for _, p := range []*wl.Proxy{w.decoration, w.toplevel, w.xdgSurface, w.viewport, w.fractional} {
		if p != nil {
			p.Send(0) // destroy
			p.Destroyed()
		}
	}
	w.surface.Send(0) // destroy
	w.surface.Destroyed()
	w.d.windows = slices.DeleteFunc(w.d.windows, func(o *window) bool { return o == w })
	if s := w.d.seat; s != nil {
		s.forget(w)
	}
	w.d.conn.Flush()
}

// ---- platform.Window ----

// Handle implements platform.Window. Wayland has no handle a GPU backend
// needs: the CPU renderer draws through the window's target.
func (w *window) Handle() uintptr { return uintptr(w.surface.ID) }

// Target returns where the CPU renderer draws the window's frames.
func (w *window) Target() soft.Target { return w.target() }

func (w *window) Size() (int, int) { return w.physical(w.logW), w.physical(w.logH) }
func (w *window) Scale() float32   { return float32(w.scale()) }

func (w *window) SetTitle(t string) {
	if t != w.title && !w.destroyed {
		w.title = t
		w.toplevel.Send(2, t) // set_title
	}
}

// Frame implements platform.Window. Wayland does not tell a client where its
// window is, so the frame is at the origin.
func (w *window) Frame() platform.Rect {
	pw, ph := w.Size()
	return platform.Rect{W: pw, H: ph}
}

// ClientOrigin implements platform.Window; see Frame.
func (w *window) ClientOrigin() platform.Point { return platform.Point{} }

func (w *window) SetCursor(c input.Cursor) {
	if c == w.cursor {
		return
	}
	w.cursor = c
	if s := w.d.seat; s != nil && s.pointerOn == w {
		s.applyCursor(c)
	}
}

// Show implements platform.Window. A window is shown when its first frame is
// drawn; Show after Hide maps it again.
func (w *window) Show() {
	if !w.hidden || w.destroyed {
		return
	}
	w.hidden = false
	w.configured = false
	w.surface.Send(6) // commit: the initial commit again, for a configure
	w.needsDraw = true
	signalLoop()
}

func (w *window) Close() { Post(w.requestClose) }

// Minimize implements platform.Window.
func (w *window) Minimize() {
	if !w.destroyed {
		w.toplevel.Send(13) // set_minimized
	}
}

func (w *window) ToggleMaximize() {
	if w.destroyed {
		return
	}
	if w.maximized {
		w.toplevel.Send(10) // unset_maximized
	} else {
		w.toplevel.Send(9) // set_maximized
	}
}

func (w *window) Maximized() bool { return w.maximized }

// SetSize implements platform.Window, in logical pixels. A tiled or maximized
// window keeps the size the compositor gives it.
func (w *window) SetSize(width, height int) {
	if width <= 0 || height <= 0 || w.maximized || w.fullscreen {
		return
	}
	w.logW, w.logH = max(width, w.opts.MinWidth), max(height, w.opts.MinHeight)
	w.needsDraw = true
	w.emit(platform.Event{Kind: platform.EventResize})
}

// SetPosition implements platform.Window. Wayland lets only the compositor
// place windows, so it does nothing.
func (w *window) SetPosition(x, y int) {}

func (w *window) SetFullscreen(on bool) {
	if w.destroyed || on == w.fullscreen {
		return
	}
	if on {
		w.toplevel.Send(11, (*wl.Proxy)(nil)) // set_fullscreen on the current output
	} else {
		w.toplevel.Send(12) // unset_fullscreen
	}
}

func (w *window) Fullscreen() bool { return w.fullscreen }

// SetDark implements platform.Window. The compositor draws its decorations in
// its own theme.
func (w *window) SetDark(on bool) {}

// OpenURL opens url with the desktop's handler for it.
func (w *window) OpenURL(url string) error {
	cmd := exec.Command("xdg-open", url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// Focus implements platform.Window. Taking focus needs an activation token
// from the compositor (xdg-activation), which is not implemented yet.
func (w *window) Focus() {}

func (w *window) Wake() {
	if w.wakePending.CompareAndSwap(false, true) {
		signalLoop()
	}
}

// WakeAt implements platform.Window with one timer at the earliest time asked.
func (w *window) WakeAt(t time.Time) {
	if !w.Visible() || w.destroyed {
		return
	}
	if !w.wakeAt.IsZero() && !t.Before(w.wakeAt) {
		return
	}
	w.wakeAt = t
	w.wakeTmr.stop()
	w.wakeTmr = w.d.after(time.Until(t), func() {
		w.wakeAt = time.Time{}
		w.wakeTmr = nil
		w.wakePending.Store(true)
	})
}

// The clipboard is the seat's selection (see clipboard). Without a data
// device, it is the process's own, so copy and paste work within the app.
func (w *window) Clipboard() (string, bool) {
	if c := w.d.clip; c != nil {
		return c.text()
	}
	return localClip, localClipSet
}

func (w *window) SetClipboard(text string) {
	if c := w.d.clip; c != nil && w.d.seat != nil {
		c.setText(text, w.d.seat.serial)
		return
	}
	localClip, localClipSet = text, true
}

func (w *window) ClipboardImage() (image.Image, bool) {
	if c := w.d.clip; c != nil {
		return c.image()
	}
	return nil, false
}

func (w *window) HasClipboardImage() bool { return w.d.clip != nil && w.d.clip.hasImage() }

func (w *window) SetClipboardImage(img image.Image) {
	if c := w.d.clip; c != nil && w.d.seat != nil {
		c.setImage(img, w.d.seat.serial)
	}
}

var (
	localClip    string
	localClipSet bool
)

// SetTextInputRect and CancelComposition wait for text-input-v3.
func (w *window) SetTextInputRect(r platform.Rect) {}
func (w *window) CancelComposition()               {}

// ShowFileDialog implements platform.Window with the desktop's file dialog,
// through the file chooser portal.
func (w *window) ShowFileDialog(kind dialog.FileKind, o dialog.FileOptions, done func(dialog.FileResult)) {
	showFileDialog(kind, o, done)
}

// ShowMessageBox implements platform.Window. Not implemented yet: it answers
// with no button, as a closed box does.
func (w *window) ShowMessageBox(o dialog.MessageOptions, done func(dialog.Button)) {
	Post(func() { done(dialog.NoButton) })
}

// Hide unmaps the window: a null buffer is committed.
func (w *window) Hide() {
	if w.hidden || w.destroyed {
		return
	}
	w.hidden = true
	w.mapped = false
	w.surface.Send(1, (*wl.Proxy)(nil), int32(0), int32(0)) // attach(null)
	w.surface.Send(6)                                       // commit
}

func (w *window) Restore() {
	w.Show()
	if w.maximized {
		w.toplevel.Send(10) // unset_maximized
	}
}

// Visible implements platform.Window: shown, and not suspended (a compositor
// suspends a window that is minimized or fully covered).
func (w *window) Visible() bool { return !w.hidden && !w.suspended && !w.destroyed }

func (w *window) Shown() bool { return !w.hidden && !w.destroyed }

// The taskbar progress and badge have no Wayland protocol.
func (w *window) SetTaskbarProgress(state platform.TaskbarState, value float32) {}
func (w *window) SetTaskbarOverlay(img image.Image, description string) error   { return nil }

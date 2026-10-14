package ui

import (
	"image"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/uihost"
)

// runtimeHost is the window host's side of a Runtime: the methods that only
// the app package calls (see internal/uihost). Its type is unexported, so the
// methods stay out of ui's documentation.
type runtimeHost Runtime

func init() { uihost.Of = func(rt any) any { return (*runtimeHost)(rt.(*Runtime)) } }

func (h *runtimeHost) rt() *Runtime { return (*Runtime)(h) }

// SetWakeFunc sets the function that asks the host for a frame.
func (h *runtimeHost) SetWakeFunc(fn func()) { h.rt().hostSetWakeFunc(fn) }

// SetServices connects the platform's clipboard and timed wake-ups.
func (h *runtimeHost) SetServices(clipboard func() (string, bool), setClipboard func(string), wakeAt func(time.Time)) {
	h.rt().hostSetServices(platformServices{Clipboard: clipboard, SetClipboard: setClipboard, WakeAt: wakeAt})
}

// SetImageClipboard connects the platform's image clipboard (see
// Context.ClipboardImage).
func (h *runtimeHost) SetImageClipboard(get func() (image.Image, bool), set func(image.Image), has func() bool) {
	s := &h.rt().services
	s.ClipboardImage, s.SetClipboardImage, s.HasClipboardImage = get, set, has
}

// SetCancelComposition connects the platform's way to make the input method
// drop what it is composing (see commitComposition).
func (h *runtimeHost) SetCancelComposition(fn func()) { h.rt().services.CancelComposition = fn }

// Compose shows what an input method is composing in the focused text field:
// text, with the input method's caret at byte cursor and the clause being
// converted from byte clauseStart to clauseEnd. Empty text ends it.
func (h *runtimeHost) Compose(text string, cursor, clauseStart, clauseEnd int) {
	h.rt().hostCompose(composition{text: text, cursor: cursor, clauseStart: clauseStart, clauseEnd: clauseEnd})
}

// SetClientOrigin connects the platform's client area origin on the screen, which a drag
// released outside the window is placed by (see screenOrigin).
func (h *runtimeHost) SetClientOrigin(fn func() (x, y int)) { h.rt().clientOrigin = fn }

// SetOpenURL connects the platform's way to open a URL (see Context.OpenURL).
func (h *runtimeHost) SetOpenURL(fn func(string) error) { h.rt().services.OpenURL = fn }

// SetMaximized tells the runtime whether the window is maximized.
func (h *runtimeHost) SetMaximized(on bool) { h.rt().hostSetMaximized(on) }

// OnZoom sets the function called when the zoom changes.
func (h *runtimeHost) OnZoom(fn func(float32)) { h.rt().hostOnZoom(fn) }

// Invalidate and InvalidateAll ask for a rebuild (the dev hooks use them).
func (h *runtimeHost) Invalidate()    { h.rt().hostInvalidate() }
func (h *runtimeHost) InvalidateAll() { h.rt().hostInvalidateAll() }

// PointerLeave reports that the pointer left the window.
func (h *runtimeHost) PointerLeave() { h.rt().hostPointerLeave() }

// FlushTimers runs the timers that are pending now, so that a window that is closing
// saves what it has not yet saved (see Runtime.at).
func (h *runtimeHost) FlushTimers() { h.rt().runTimers(time.Time{}, true) }

// WindowFocus reports that the window gained or lost focus.
func (h *runtimeHost) WindowFocus(focused bool) { h.rt().hostWindowFocus(focused) }

// Cursor returns the cursor the window should show.
func (h *runtimeHost) Cursor() input.Cursor { return h.rt().hostCursor() }

// HasMenuBar reports whether the UI has a menu bar, for Alt handling.
func (h *runtimeHost) HasMenuBar() bool { return h.rt().hostHasMenuBar() }

// TextInputRect returns the caret rectangle for the IME window.
func (h *runtimeHost) TextInputRect() (scene.Rect, bool) { return h.rt().hostTextInputRect() }

// WindowAreaAt returns the custom title bar area at a point, in logical pixels.
func (h *runtimeHost) WindowAreaAt(x, y float32) WindowArea { return h.rt().hostWindowAreaAt(x, y) }

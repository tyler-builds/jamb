package ui

import (
	"image"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/uihost"
)

// runtimeHost is the window host's side of a Runtime: the methods that only
// the app package, and ui/uitest for a headless runtime, call (see
// internal/uihost). Its type is unexported, so the methods stay out of ui's
// documentation.
type runtimeHost Runtime

func init() { uihost.Of = func(rt any) any { return (*runtimeHost)(rt.(*Runtime)) } }

func (h *runtimeHost) rt() *Runtime { return (*Runtime)(h) }

// Frame brings the scene up to date (see Runtime.Frame) and returns it, for the host to draw.
func (h *runtimeHost) Frame(now time.Time) *scene.Scene { return h.rt().frame(now) }

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

// SetNativeCaption tells the runtime that the platform runs the pointer clicks of
// the title-bar button areas, as it does for a window whose title bar the app
// draws, so the runtime does not run their OnClick for the pointer as well.
func (h *runtimeHost) SetNativeCaption(on bool) { h.rt().nativeCaption = on }

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

// Close tells the runtime that its window has closed: the fetches of its Query calls
// are canceled, and none starts again (see Query).
func (h *runtimeHost) Close() { h.rt().hostClose() }

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

// ---- for ui/uitest ----
//
// A headless runtime in a test is driven by ui/uitest, which uses these as a
// window host uses the methods above.

// SetClock replaces the clock that timers, tooltips, motion and the caret read
// (time.Now by default), so a test steps time itself.
func (h *runtimeHost) SetClock(now func() time.Time) { h.rt().clock = now }

// LocalClipboard and SetLocalClipboard read and replace the text of the
// in-process clipboard, which a runtime uses when no platform clipboard is
// connected. They never touch the system clipboard.
func (h *runtimeHost) LocalClipboard() string { return h.rt().memClipboard }

// SetLocalClipboard replaces the in-process clipboard with text.
func (h *runtimeHost) SetLocalClipboard(s string) { h.rt().memClipboard, h.rt().memImage = s, nil }

// LocalClipboardImage and SetLocalClipboardImage are the image of the in-process
// clipboard (see LocalClipboard).
func (h *runtimeHost) LocalClipboardImage() image.Image { return h.rt().memImage }

// SetLocalClipboardImage replaces the in-process clipboard with an image.
func (h *runtimeHost) SetLocalClipboardImage(img image.Image) {
	h.rt().memClipboard, h.rt().memImage = "", img
}

// PressKeys presses and releases the strokes of keys, written as for Bind
// ("ctrl+a", "shift+tab", "ctrl+k ctrl+s"), and runs a frame after each. The
// error says what is wrong with a malformed keys string.
func (h *runtimeHost) PressKeys(keys string) error {
	strokes, err := parseKeys(keys)
	if err != nil {
		return err
	}
	for _, s := range strokes {
		h.rt().Press(s.key, s.mods)
	}
	return nil
}

// TextOf returns the text inside the first element whose key is key (see
// Find): its text elements and the values of its text fields, in tree order,
// joined by spaces. Elements hidden by Display(false) are left out.
func (h *runtimeHost) TextOf(key string) (string, bool) {
	rt := h.rt()
	n := rt.findKey(rt.tree, key)
	if n == nil {
		return "", false
	}
	var parts []string
	var walk func(n *node)
	walk = func(n *node) {
		if n.flags&flagNoDisplay != 0 {
			return
		}
		switch n.kind {
		case kindText:
			parts = append(parts, n.textEl.text)
		case kindEditor:
			parts = append(parts, n.editor.value.Peek())
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	if !rt.undisplayed(n) {
		walk(n)
	}
	return strings.Join(parts, " "), true
}

// AccessNode returns the node of tree (see AccessTree) that stands for the
// first element whose key is key: the element's own node, or when it has none,
// as a composite element such as TextInput has not, the first node of an
// element inside it, in tree order. It returns nil when there is neither.
func (h *runtimeHost) AccessNode(tree *access.Tree, key string) *access.Node {
	rt := h.rt()
	var walk func(n *node) *access.Node
	walk = func(n *node) *access.Node {
		if an := tree.Node(accID(n.id)); an != nil {
			return an
		}
		for _, c := range n.children {
			if an := walk(c); an != nil {
				return an
			}
		}
		return nil
	}
	if n := rt.findKey(rt.tree, key); n != nil {
		return walk(n)
	}
	return nil
}

// Perform runs an access action on a node of the current access tree (see
// AccessTree) at once, as an assistive technology's request runs at the next
// frame. It does nothing when the node is gone.
func (h *runtimeHost) Perform(id access.ID, r access.Request) {
	rt := h.rt()
	a := rt.accessState()
	_, a.targets = rt.buildAccess()
	rt.runAccess(id, r)
}

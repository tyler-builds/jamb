// Package platform defines the framework-owned contract between the
// cross-platform core and an operating-system adapter.
//
// Adapters live in sub-packages (platform/windows, platform/darwin,
// platform/linux). Nothing outside those sub-packages may call OS APIs, and
// nothing in this package knows about the ui, layout or render packages.
//
// An adapter runs one UI thread and one event loop for all of the process's
// windows. It provides these package-level functions, which the app package
// calls:
//
//	Run(start func() error) error   // runs start on the UI thread, then the loop until Quit
//	Open(opts WindowOptions, h Handler) (Window, error)  // UI thread only
//	Quit()                          // ends the loop; safe from any goroutine
//	Post(fn func())                 // runs fn on the UI thread; safe from any goroutine
package platform

import (
	"time"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/input"
)

// WindowOptions describes the window an adapter should create.
// Sizes and Origin are logical pixels (device-independent); the adapter scales
// them by the system's DPI.
type WindowOptions struct {
	Title     string
	Width     int
	Height    int
	MinWidth  int
	MinHeight int
	// Dark asks the OS to draw dark window decorations where supported.
	Dark bool
	// Frame, when non-zero, restores a previous outer window rectangle in
	// physical screen pixels (see Window.Frame) instead of centering.
	Frame Rect
	// Origin, when non-nil, places the window's top-left corner at this
	// logical screen position instead of centering it. It is ignored when
	// Frame is set.
	Origin *Point
	// CustomTitleBar removes the operating system's title bar. The app draws
	// its own, marks the parts that drag or press with the handler's
	// TitleBarHandler, and the window keeps its native resize borders, snap
	// and shadow. See TitleBarHandler.
	CustomTitleBar bool
}

// Point is a position in logical pixels (see WindowOptions.Origin).
type Point struct{ X, Y int }

// Rect is a rectangle in physical screen pixels.
type Rect struct{ X, Y, W, H int }

// TitleArea is a part of a custom title bar, as reported by TitleBarHandler.
type TitleArea uint8

const (
	// TitleNone is the client area: ordinary pointer events.
	TitleNone TitleArea = iota
	// TitleDrag moves the window when dragged, and maximizes it on double-click.
	TitleDrag
	// TitleMinimize minimizes the window when the button is released.
	TitleMinimize
	// TitleMaximize toggles maximized when the button is released.
	TitleMaximize
	// TitleClose closes the window when the button is released.
	TitleClose
)

// TitleBarHandler is optionally implemented by a Handler of a window opened
// with CustomTitleBar. The adapter asks it which title-bar area lies under a
// point, so the native hit test can move, snap and close the window. Points
// are physical pixels relative to the client area.
type TitleBarHandler interface {
	TitleArea(w Window, x, y float32) TitleArea
}

// MenuBarHandler is optionally implemented by a Handler whose window shows an
// application menu bar. When Alt is pressed and released alone, the adapter
// asks it, and if the bar takes that key itself, the system menu does not
// open. Windows without a menu bar keep the system's behavior.
type MenuBarHandler interface {
	HasMenuBar() bool
}

// FrameWaiter is optionally implemented by a Handler whose frames the GPU paces.
// FrameWaitable returns a handle that is signaled when the window can take its
// next frame, or 0 when it has none. The adapter renders such a window once the
// handle is signaled, so no frame blocks the event loop and windows that are
// animating run in parallel.
type FrameWaiter interface {
	FrameWaitable() uintptr
}

// Window is a native window owned by an adapter.
type Window interface {
	// Handle returns the native handle a GPU backend needs to create a
	// surface (HWND on Windows, NSView* on macOS, X11/Wayland surface on Linux).
	Handle() uintptr
	// Size returns the client area in physical pixels.
	Size() (width, height int)
	// Scale returns the ratio of physical to logical pixels (1.0 at 96 DPI).
	Scale() float32
	SetTitle(title string)
	// Frame returns the outer window rectangle in physical screen pixels.
	Frame() Rect
	SetCursor(c input.Cursor)
	// Show makes the window visible. Adapters create windows hidden so the
	// first frame can be rendered before anything appears on screen.
	Show()
	// Close requests the window to close (Handler.OnClose may keep it open).
	Close()
	// Minimize minimizes the window to the taskbar.
	Minimize()
	// ToggleMaximize maximizes a restored window, or restores a maximized one.
	ToggleMaximize()
	// Maximized reports whether the window is maximized.
	Maximized() bool
	// Focus brings the window to the front and gives it keyboard focus.
	Focus()
	// Wake schedules a call to Handler.OnFrame. It is safe to call from any
	// goroutine and must coalesce: any number of calls before the next frame
	// produce one frame (platform/windows posts at most one wake message).
	Wake()
	// WakeAt schedules a frame at (or soon after) t, for timed effects such
	// as a blinking caret, without rendering in between. Only the earliest
	// pending time is kept. UI thread only.
	WakeAt(t time.Time)

	// Clipboard returns the clipboard's text, with LF line endings.
	Clipboard() (string, bool)
	// SetClipboard replaces the clipboard's contents.
	SetClipboard(text string)
	// SetTextInputRect tells the OS where text is being typed (the caret,
	// in physical client pixels) so input-method windows appear next to it.
	// A zero rect means no text field has focus.
	SetTextInputRect(r Rect)

	// ShowFileDialog opens a file or folder dialog owned by this window. It
	// returns at once, and done runs on the UI thread when the dialog closes.
	ShowFileDialog(kind dialog.FileKind, o dialog.FileOptions, done func(dialog.FileResult))
	// ShowMessageBox opens a message box owned by this window, and calls done
	// on the UI thread with the button chosen when it closes.
	ShowMessageBox(o dialog.MessageOptions, done func(dialog.Button))
}

// Handler receives everything the adapter produces. All methods are called on
// the adapter's UI thread.
type Handler interface {
	// OnCreate is called once the window exists but before it is shown.
	OnCreate(w Window) error
	// OnEvent delivers one input or window event.
	OnEvent(w Window, e Event)
	// OnFrame renders. It returns true when another frame is wanted
	// immediately (an animation is running). Otherwise the adapter sleeps until
	// the next event or Wake call, so idle applications use no CPU.
	OnFrame(w Window) (again bool)
	// OnClose is called when the user or the app closes the window. It
	// returns false to keep the window open.
	OnClose(w Window) (close bool)
	// OnDestroy is called once when the window is gone. The event loop ends
	// when the app calls Quit, not when the last window closes.
	OnDestroy(w Window)
}

// EventKind identifies the kind of an Event.
type EventKind uint8

const (
	EventMouseMove EventKind = iota + 1
	EventMouseDown
	EventMouseUp
	EventMouseLeave
	EventScroll
	EventKeyDown
	EventKeyUp
	EventTextInput
	EventFocus
	EventBlur
	EventResize
	// EventMoved is sent when the user finishes moving or resizing the window.
	EventMoved
	EventScaleChanged
	// EventWindowState is sent when the window is maximized or restored.
	// Window.Maximized reports the new state.
	EventWindowState
	// EventFileDrop is sent when files are dropped on the window from the
	// operating system. X and Y give the drop point, and Paths the files.
	EventFileDrop
)

// Event is a flat, allocation-free event record. Which fields are meaningful
// depends on Kind.
type Event struct {
	Kind EventKind

	// Pointer position in physical pixels relative to the client area.
	X, Y float32
	// Button for mouse down/up.
	Button input.MouseButton
	// Clicks is 1 for single and 2 for double clicks (mouse down only).
	Clicks int

	// Scroll deltas in logical pixels. Positive DeltaY scrolls content up
	// (reveals content further down), positive DeltaX reveals content to the right.
	DeltaX, DeltaY float32
	// Precise is true for touchpads/high-resolution wheels that should not be
	// smoothed.
	Precise bool

	Key    input.Key
	Mods   input.Modifiers
	Repeat bool
	Text   string
	// Paths are the files of an EventFileDrop, as local file system paths.
	Paths []string
}

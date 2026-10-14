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
	"image"
	"time"

	"github.com/tyler-builds/jamb/access"
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
	// KeepOnScreen moves a window that opens at Origin in from the edges of the monitor it
	// lands on, so that it fits in that monitor's work area. Only Windows does this; other
	// platforms place the window at Origin.
	KeepOnScreen bool
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

// AccessHandler is optionally implemented by a Handler whose UI is exposed to
// assistive technology. AccessBridge returns the bridge that the window's UI
// runtime publishes its access tree on, or nil before the runtime exists. The
// adapter activates the bridge when an assistive technology first asks for the
// window's tree, and receives the tree and its updates through the bridge (see
// access.Bridge).
type AccessHandler interface {
	AccessBridge() *access.Bridge
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
	// ClientOrigin returns the top-left corner of the client area on the screen, in
	// logical pixels. A client point (x, y) of the window is at this origin plus (x, y).
	ClientOrigin() Point
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
	// SetSize sets the size of the client area, and SetPosition moves the
	// window's outer top-left corner, both in logical pixels at the window's
	// scale. A maximized or fullscreen window is restored first.
	SetSize(width, height int)
	SetPosition(x, y int)
	// SetFullscreen makes the window cover its monitor without a frame, or
	// puts it back where it was. Fullscreen reports which.
	SetFullscreen(on bool)
	Fullscreen() bool
	// SetDark asks for the dark window decorations (the native title bar and
	// frame), to match a dark theme.
	SetDark(on bool)
	// OpenURL opens url with the system's handler for its scheme, such as the
	// default browser. The caller has checked the scheme.
	OpenURL(url string) error
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
	// ClipboardImage returns the clipboard's image, if it holds one.
	ClipboardImage() (image.Image, bool)
	// HasClipboardImage reports whether the clipboard holds an image, without
	// reading it.
	HasClipboardImage() bool
	// SetClipboardImage replaces the clipboard's contents with img.
	SetClipboardImage(img image.Image)
	// SetTextInputRect tells the OS where text is being typed (the caret,
	// in physical client pixels) so input-method windows appear next to it.
	// A zero rect means no text field has focus.
	SetTextInputRect(r Rect)
	// CancelComposition makes the input method drop the text it is composing,
	// without typing it and without sending an EventComposition for it. The
	// application types the composition itself first when it should be kept.
	CancelComposition()

	// ShowFileDialog opens a file or folder dialog owned by this window. It
	// returns at once, and done runs on the UI thread when the dialog closes.
	ShowFileDialog(kind dialog.FileKind, o dialog.FileOptions, done func(dialog.FileResult))
	// ShowMessageBox opens a message box owned by this window, and calls done
	// on the UI thread with the button chosen when it closes.
	ShowMessageBox(o dialog.MessageOptions, done func(dialog.Button))

	// Hide hides the window and its taskbar button, and Restore shows it again
	// and brings it to the front.
	Hide()
	Restore()
	// Visible reports whether the window is on screen: not hidden, and not
	// minimized. A window that is not visible draws nothing and schedules no
	// timed wake-ups (see Handler.OnFrame and WakeAt).
	Visible() bool
	// SetTaskbarProgress sets the progress bar of the window's taskbar button.
	// value is from 0 to 1; TaskbarIndeterminate ignores it.
	SetTaskbarProgress(state TaskbarState, value float32)
	// SetTaskbarOverlay sets the badge drawn over the taskbar button's icon, or
	// removes it when img is nil. description is what assistive technology reads.
	// It returns an error when the icon cannot be made.
	SetTaskbarOverlay(img image.Image, description string) error
}

// TaskbarState is the state of a taskbar button's progress bar.
type TaskbarState uint8

const (
	// TaskbarNone shows no progress bar.
	TaskbarNone TaskbarState = iota
	// TaskbarNormal shows progress up to the value.
	TaskbarNormal
	// TaskbarIndeterminate shows that work is going on, without a value.
	TaskbarIndeterminate
	// TaskbarPaused shows progress up to the value, in the paused color.
	TaskbarPaused
	// TaskbarError shows progress up to the value, in the error color.
	TaskbarError
)

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

// Settings are the operating system's accessibility and text settings, as the
// platform's ReadSettings reports them. The application re-reads them when
// EventSettingsChanged arrives.
type Settings struct {
	HighContrast  bool          // the high contrast theme is on (SPI_GETHIGHCONTRAST, HCF_HIGHCONTRASTON)
	Colors        SystemColors  // the system colors (GetSysColor)
	TextScale     int           // percent, 100..225 (Settings > Accessibility > Text size)
	ReducedMotion bool          // client area animations are off (SPI_GETCLIENTAREAANIMATION false)
	CaretBlink    time.Duration // GetCaretBlinkTime in ms; 0 means the caret does not blink (INFINITE)
	CaretWidth    int           // SPI_GETCARETWIDTH, in device pixels (the text cursor thickness; default 1)
	Dark          bool          // apps use the dark mode (Settings > Personalization > Colors)
}

// SystemColors are the system colors of the current theme, each as a 0x00RRGGBB
// value. The field names are the COLOR_* indices of GetSysColor, whose standard
// numeric values are noted on each field. A zero value means the color is not
// known (see DefaultSettings).
type SystemColors struct {
	Window        uint32 // COLOR_WINDOW (5)
	WindowText    uint32 // COLOR_WINDOWTEXT (8)
	Highlight     uint32 // COLOR_HIGHLIGHT (13)
	HighlightText uint32 // COLOR_HIGHLIGHTTEXT (14)
	HotLight      uint32 // COLOR_HOTLIGHT (26)
	GrayText      uint32 // COLOR_GRAYTEXT (17)
	ButtonFace    uint32 // COLOR_BTNFACE (15)
	ButtonText    uint32 // COLOR_BTNTEXT (18)
	ButtonShadow  uint32 // COLOR_BTNSHADOW (16)
	WindowFrame   uint32 // COLOR_WINDOWFRAME (6)
	CaptionText   uint32 // COLOR_CAPTIONTEXT (9)
	ActiveCaption uint32 // COLOR_ACTIVECAPTION (2)
	Menu          uint32 // COLOR_MENU (4)
	MenuText      uint32 // COLOR_MENUTEXT (7)
	InfoBack      uint32 // COLOR_INFOBK (24)
}

// DefaultSettings returns the settings that the platform reports when it cannot
// read the operating system's settings. Platforms that do not read settings
// use it as well. The colors are zero, so the application uses its own.
func DefaultSettings() Settings {
	return Settings{
		HighContrast:  false,
		TextScale:     100,
		ReducedMotion: false,
		CaretBlink:    530 * time.Millisecond,
		CaretWidth:    1,
	}
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
	// EventSettingsChanged is sent when an operating system setting that the
	// application reads through Settings changed (high contrast, text size,
	// animation effects, caret, or system colors). The application re-reads
	// them with the platform's ReadSettings.
	EventSettingsChanged
	// EventComposition is sent while an input method (IME) composes text, such
	// as Chinese, Japanese or Korean, which the focused field shows in place.
	// Text is the text being composed, and empty when the composition ends;
	// Cursor is the input method's caret in it, and ClauseStart and ClauseEnd
	// are the clause being converted (equal when there is none), all as byte
	// offsets in Text. The text the input method confirms arrives afterwards,
	// or in the same message, as an EventTextInput.
	EventComposition
	// EventGesture is sent for a pinch of a touchpad or touchscreen, as the
	// operating system's gesture recognizer reports it. Phase says where the
	// gesture is, Scale, PanX and PanY say what changed since the previous
	// event of the gesture, and X and Y give its center (see GesturePhase).
	EventGesture
)

// GesturePhase says where a gesture is, for an EventGesture.
type GesturePhase uint8

const (
	// GestureBegin starts a gesture. It carries no change.
	GestureBegin GesturePhase = iota
	// GestureUpdate carries the change since the previous event of the gesture.
	GestureUpdate
	// GestureEnd ends a gesture, once the fingers have lifted and any inertia
	// has run out, or when a new gesture interrupts the inertia. It carries no
	// change.
	GestureEnd
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
	// Cursor, ClauseStart and ClauseEnd are byte offsets in Text of an
	// EventComposition.
	Cursor, ClauseStart, ClauseEnd int

	// Phase, Scale, PanX, PanY and Inertia describe an EventGesture. Scale is
	// the multiplicative change of the zoom since the previous event, 1 when
	// there is none. PanX and PanY are the translation since the previous
	// event, in physical pixels. Inertia is set while the gesture continues
	// after the fingers have lifted.
	Phase      GesturePhase
	Scale      float32
	PanX, PanY float32
	Inertia    bool
}

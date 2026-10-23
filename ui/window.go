package ui

import (
	"image"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/icons"
)

// WindowArea marks the part of a custom title bar an element is in. The
// platform gives each area its native behavior: dragging WindowAreaDrag moves the
// window (double-clicking maximizes it, and Aero Snap works), and releasing a
// button area performs its action. Windows that keep the operating system's
// title bar ignore areas.
type WindowArea uint8

const (
	// WindowAreaNone is the client area: ordinary pointer events.
	WindowAreaNone WindowArea = iota
	// WindowAreaDrag moves the window when dragged.
	WindowAreaDrag
	// WindowAreaMinimize minimizes the window when released over it.
	WindowAreaMinimize
	// WindowAreaMaximize maximizes or restores the window when released over it.
	WindowAreaMaximize
	// WindowAreaClose closes the window when released over it, as the user's
	// close (see Window.RequestClose).
	WindowAreaClose
)

// isButton reports whether a is one of the title-bar buttons, whose pointer
// clicks the platform runs.
func (a WindowArea) isButton() bool {
	return a == WindowAreaMinimize || a == WindowAreaMaximize || a == WindowAreaClose
}

// WindowArea marks d as part of the window's title bar. Elements inside an
// WindowAreaDrag element are dragged with the window too, unless they take input
// of their own (see WindowAreaAt): a search box or a button in the title bar
// stays usable. In a window whose title bar the app draws, the platform runs the
// button areas' actions for the pointer, so an OnClick on such an element runs
// only for the keyboard and assistive technology, and must do the same thing.
func (d *DivElement) WindowArea(a WindowArea) *DivElement { d.area = a; return d }

// hostWindowAreaAt returns the title-bar area under (x, y), in device-logical
// pixels as for PointerMove, or WindowAreaNone. A point is in an WindowAreaDrag area
// unless an element above that area on the hit path takes input of its own:
// one with OnClick, OnMouseDown, focus (which includes text fields) or OnDrag.
// A title-bar button area is the platform's even when its element has an OnClick,
// which serves the keyboard and assistive technology: the platform then presses
// the button (and Windows shows Snap Layouts over maximize), and the runtime
// leaves the pointer click to it (see nativeCaption). It uses the hit testing of
// the last paint, so painted elements only.
func (rt *Runtime) hostWindowAreaAt(x, y float32) WindowArea {
	x, y = x/rt.zoom, y/rt.zoom
	for _, n := range rt.hitTest(x, y) {
		d := n.div
		if d == nil {
			continue
		}
		if d.area.isButton() {
			return d.area
		}
		if d.interactive() {
			return WindowAreaNone
		}
		if d.area != WindowAreaNone {
			return d.area
		}
	}
	return WindowAreaNone
}

// interactive reports whether the element takes pointer or keyboard input of
// its own.
func (d *DivElement) interactive() bool {
	return d.h.onClick != nil || d.h.onMouseDown != nil || d.focusable || d.dragSpec != nil
}

// Window is a window of the app: the one a component is shown in
// (Context.Window), or one opened with app.Open. Its methods may be called from
// any goroutine. An action runs on the UI thread: at once when it is called
// there, else in call order with the other posted work. A read from another
// goroutine waits for the UI thread to answer it. Methods may be added to Window
// in later versions, so an implementation should embed NopWindow, which has every
// method of Window.
type Window interface {
	// Minimize minimizes the window to the taskbar.
	Minimize()
	// ToggleMaximize maximizes the window, or restores it when it is maximized.
	ToggleMaximize()
	// Focus brings the window to the front and gives it keyboard focus.
	Focus()
	// Closed reports whether the window has closed, or failed to open.
	Closed() bool
	// Maximized reports whether the window is maximized. Read inside a
	// component, it is reactive: the component re-renders when it changes.
	Maximized() bool
	// Close closes the window, as the app's own decision: OnClose is asked, and
	// app.Window's CloseToTray does not apply.
	Close()
	// RequestClose asks the window to close as the user's close button does:
	// OnClose is asked, and with app.Window's CloseToTray the window may hide to
	// the notification area instead. A close button that the app draws itself
	// calls it.
	RequestClose()
	// SetTitle sets the title that the title bar and the taskbar show.
	SetTitle(title string)
	// Focused reports whether the window has keyboard focus. Read inside a
	// component, it is reactive.
	Focused() bool

	// Position returns the window's top-left corner on the screen, and Size
	// the size of its content, in logical pixels as app.Window takes them.
	Position() (x, y int)
	// Size returns the size of the window's content in logical pixels; see Position.
	Size() (width, height int)
	// SetPosition moves the window and SetSize resizes its content, in logical
	// pixels. A maximized or fullscreen window is restored first.
	SetPosition(x, y int)
	// SetSize resizes the window's content, in logical pixels; see SetPosition.
	SetSize(width, height int)
	// SetFullscreen makes the window cover its monitor without a frame, or puts
	// it back where it was. Fullscreen reports which.
	SetFullscreen(on bool)
	// Fullscreen reports whether the window is fullscreen; see SetFullscreen.
	Fullscreen() bool

	// OpenFiles shows the system's file-open dialog, owned by this window. It
	// returns at once; done runs on the UI thread when the dialog closes, with
	// the files chosen or Canceled set. See the dialog package.
	OpenFiles(o dialog.FileOptions, done func(dialog.FileResult))
	// SaveFile shows the system's file-save dialog, owned by this window. done
	// runs as for OpenFiles.
	SaveFile(o dialog.FileOptions, done func(dialog.FileResult))
	// PickFolder shows the system's folder dialog, owned by this window. done
	// runs as for OpenFiles.
	PickFolder(o dialog.FileOptions, done func(dialog.FileResult))
	// Show shows a window that is hidden or minimized, and brings it to the front.
	// Hide hides the window and its taskbar button, and the app keeps running; a
	// hidden window draws nothing until Show. These are Windows operations, and
	// see also app.Window.CloseToTray.
	Show()
	// Hide hides the window and its taskbar button; see Show.
	Hide()
	// Visible reports whether the window is shown: it has opened, is not hidden
	// (by Hide, or by a close with app.Window.CloseToTray), and has not closed. A
	// minimized window is visible.
	Visible() bool
	// SetTaskbarProgress sets the progress bar of the window's taskbar button.
	SetTaskbarProgress(p TaskbarProgress)
	// SetTaskbarBadge sets the badge drawn over the taskbar button's icon, or
	// removes it when img is nil. The badge is drawn at the small icon size. Its
	// description is what assistive technology reads, such as "3 unread", and it
	// is required with an image: without one, the badge is not set.
	SetTaskbarBadge(img image.Image, description string)
	// MessageBox shows a message box, owned by this window, and runs done with
	// the button chosen.
	MessageBox(o dialog.MessageOptions, done func(dialog.Button))
}

// NopWindow is a Window whose methods do nothing and return zero values; Closed,
// Focused, Maximized, Fullscreen and Visible report false. A test fake embeds it and
// overrides the methods that it records or answers, so the fake keeps compiling
// when Window gains methods.
type NopWindow struct{}

var _ Window = NopWindow{}

// Minimize does nothing.
func (NopWindow) Minimize() {}

// ToggleMaximize does nothing.
func (NopWindow) ToggleMaximize() {}

// Maximized returns false.
func (NopWindow) Maximized() bool { return false }

// Focus does nothing.
func (NopWindow) Focus() {}

// Closed returns false.
func (NopWindow) Closed() bool { return false }

// Close does nothing.
func (NopWindow) Close() {}

// RequestClose does nothing.
func (NopWindow) RequestClose() {}

// SetTitle does nothing.
func (NopWindow) SetTitle(string) {}

// Focused returns false.
func (NopWindow) Focused() bool { return false }

// Position returns the zero position.
func (NopWindow) Position() (x, y int) { return 0, 0 }

// Size returns zero.
func (NopWindow) Size() (width, height int) { return 0, 0 }

// SetPosition does nothing.
func (NopWindow) SetPosition(x, y int) {}

// SetSize does nothing.
func (NopWindow) SetSize(width, height int) {}

// SetFullscreen does nothing.
func (NopWindow) SetFullscreen(on bool) {}

// Fullscreen returns false.
func (NopWindow) Fullscreen() bool { return false }

// OpenFiles does nothing, and its done function is not called.
func (NopWindow) OpenFiles(dialog.FileOptions, func(dialog.FileResult)) {}

// SaveFile does nothing, and its done function is not called.
func (NopWindow) SaveFile(dialog.FileOptions, func(dialog.FileResult)) {}

// PickFolder does nothing, and its done function is not called.
func (NopWindow) PickFolder(dialog.FileOptions, func(dialog.FileResult)) {}

// Show does nothing.
func (NopWindow) Show() {}

// Hide does nothing.
func (NopWindow) Hide() {}

// Visible returns false.
func (NopWindow) Visible() bool { return false }

// SetTaskbarProgress does nothing.
func (NopWindow) SetTaskbarProgress(TaskbarProgress) {}

// SetTaskbarBadge does nothing.
func (NopWindow) SetTaskbarBadge(image.Image, string) {}

// MessageBox does nothing, and its done function is not called.
func (NopWindow) MessageBox(dialog.MessageOptions, func(dialog.Button)) {}

// windowRef is what Context.Window returns. Its actions go to the app's
// controls (a headless runtime has none, so they do nothing). Maximized and
// Focused read signals that the app keeps current, so a component that reads
// them re-renders when the window's state changes.
type windowRef struct{ rt *Runtime }

func (w windowRef) Minimize() {
	if c := w.rt.windowCtl; c != nil {
		c.Minimize()
	}
}

func (w windowRef) ToggleMaximize() {
	if c := w.rt.windowCtl; c != nil {
		c.ToggleMaximize()
	}
}

func (w windowRef) Close() {
	if c := w.rt.windowCtl; c != nil {
		c.Close()
	}
}

func (w windowRef) RequestClose() {
	if c := w.rt.windowCtl; c != nil {
		c.RequestClose()
	}
}

func (w windowRef) SetTitle(title string) {
	if c := w.rt.windowCtl; c != nil {
		c.SetTitle(title)
	}
}

func (w windowRef) Maximized() bool { return w.rt.maximized.Get() }

func (w windowRef) Focus() {
	if c := w.rt.windowCtl; c != nil {
		c.Focus()
	}
}

// The visibility and taskbar calls go to the app's controls. A headless runtime
// has no window, so they do nothing.

func (w windowRef) Show() {
	if c := w.rt.windowCtl; c != nil {
		c.Show()
	}
}

func (w windowRef) Hide() {
	if c := w.rt.windowCtl; c != nil {
		c.Hide()
	}
}

// Visible reports true without an app window: a headless runtime draws for as
// long as it is used, as Closed reports false.
func (w windowRef) Visible() bool {
	if c := w.rt.windowCtl; c != nil {
		return c.Visible()
	}
	return true
}

func (w windowRef) SetTaskbarProgress(p TaskbarProgress) {
	if c := w.rt.windowCtl; c != nil {
		c.SetTaskbarProgress(p)
	}
}

func (w windowRef) SetTaskbarBadge(img image.Image, description string) {
	if c := w.rt.windowCtl; c != nil {
		c.SetTaskbarBadge(img, description)
	}
}

// Closed reports false without an app window: a headless runtime is open for as
// long as it is used.
func (w windowRef) Closed() bool {
	if c := w.rt.windowCtl; c != nil {
		return c.Closed()
	}
	return false
}

// The geometry goes to the app's controls. A headless runtime has no window: its
// position is 0, 0 and its size is the viewport's.

func (w windowRef) Position() (x, y int) {
	if c := w.rt.windowCtl; c != nil {
		return c.Position()
	}
	return 0, 0
}

func (w windowRef) Size() (width, height int) {
	if c := w.rt.windowCtl; c != nil {
		return c.Size()
	}
	return int(w.rt.viewW), int(w.rt.viewH)
}

func (w windowRef) SetPosition(x, y int) {
	if c := w.rt.windowCtl; c != nil {
		c.SetPosition(x, y)
	}
}

func (w windowRef) SetSize(width, height int) {
	if c := w.rt.windowCtl; c != nil {
		c.SetSize(width, height)
	}
}

func (w windowRef) SetFullscreen(on bool) {
	if c := w.rt.windowCtl; c != nil {
		c.SetFullscreen(on)
	}
}

func (w windowRef) Fullscreen() bool {
	if c := w.rt.windowCtl; c != nil {
		return c.Fullscreen()
	}
	return false
}
func (w windowRef) Focused() bool { return w.rt.focused.Get() }

// The dialogs go to the app's controls. A headless runtime has no window, so
// they do nothing and done is not called.

func (w windowRef) OpenFiles(o dialog.FileOptions, done func(dialog.FileResult)) {
	if c := w.rt.windowCtl; c != nil {
		c.OpenFiles(o, done)
	}
}

func (w windowRef) SaveFile(o dialog.FileOptions, done func(dialog.FileResult)) {
	if c := w.rt.windowCtl; c != nil {
		c.SaveFile(o, done)
	}
}

func (w windowRef) PickFolder(o dialog.FileOptions, done func(dialog.FileResult)) {
	if c := w.rt.windowCtl; c != nil {
		c.PickFolder(o, done)
	}
}

func (w windowRef) MessageBox(o dialog.MessageOptions, done func(dialog.Button)) {
	if c := w.rt.windowCtl; c != nil {
		c.MessageBox(o, done)
	}
}

// SetWindow connects the runtime to its window. The app calls it once,
// before the first frame.
func (rt *Runtime) SetWindow(w Window) { rt.windowCtl = w }

// hostSetMaximized records whether the window is maximized, for components that
// read Context.Window().Maximized(). Call it on the UI thread.
func (rt *Runtime) hostSetMaximized(on bool) { rt.maximized.Set(on) }

// TitleBarButtons returns the minimize, maximize-or-restore and close buttons
// of a custom title bar, for its right end. They are 46 logical pixels wide, as
// Windows 11's are, and take the height of the bar they are in, so in a taller bar
// they still reach its top and bottom edges and the close button's red meets the
// window's corner. A bar whose height comes from its content gets 32 px buttons. Their areas are marked, so the native hit
// testing presses them, shows Snap Layouts over the maximize button, and
// performs their actions on release. The close button turns red on hover.
func TitleBarButtons() Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		win := ctx.Window()
		strs := ctx.Strings()
		maximize, maximizeKey, maximizeLabel := icons.Maximize, "Maximize", strs.Maximize
		if win.Maximized() {
			maximize, maximizeKey, maximizeLabel = icons.Restore, "Restore", strs.Restore
		}
		// key is the identity of the button, which stays English; label is its text.
		button := func(ic *icons.Icon, area WindowArea, key, label string) *DivElement {
			d := Div().Key(key).Width(Px(46)).MinHeight(Px(32)).AlignSelf(Stretch).Center().
				TextColor(t.TextMuted).Transition(120 * time.Millisecond).
				Hover(Style{Background: t.SurfaceHover, TextColor: t.Text}).
				Pressed(Style{Background: t.SurfaceActive, TextColor: t.Text}).
				WindowArea(area).
				Child(Icon(ic).Size(10)).
				// The native hit test does the press, and the runtime leaves a
				// pointer click on the button to it (see nativeCaption), so the
				// click handler serves the keyboard and assistive technology.
				Role(access.RoleButton).Label(label).OnClick(func() {
				switch area {
				case WindowAreaMinimize:
					win.Minimize()
				case WindowAreaMaximize:
					win.ToggleMaximize()
				case WindowAreaClose:
					win.RequestClose() // the user's close: CloseToTray applies
				}
			})
			// Windows shows its own Snap Layouts flyout over the maximize button, so
			// that button gets no tooltip of ours.
			if area != WindowAreaMaximize {
				d.Tooltip(label)
			}
			return d
		}
		closeButton := button(icons.X, WindowAreaClose, "Close", strs.Close).
			Hover(Style{Background: Hex("#e81123"), TextColor: Hex("#ffffff")}).
			Pressed(Style{Background: Hex("#f1707a"), TextColor: Hex("#ffffff")})
		// The row stretches to the bar's height, whatever the bar aligns its children to.
		return Div().Row().MinHeight(Px(32)).AlignSelf(Stretch).
			Child(button(icons.Minimize, WindowAreaMinimize, "Minimize", strs.Minimize)).
			Child(button(maximize, WindowAreaMaximize, maximizeKey, maximizeLabel)).
			Child(closeButton)
	})
}

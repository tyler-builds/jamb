package ui

import (
	"time"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/icons"
)

// WindowArea marks the part of a custom title bar an element is in. The
// platform gives each area its native behavior: dragging AreaDrag moves the
// window (double-clicking maximizes it, and Aero Snap works), and releasing a
// button area performs its action. Windows that keep the operating system's
// title bar ignore areas.
type WindowArea uint8

const (
	// AreaNone is the client area: ordinary pointer events.
	AreaNone WindowArea = iota
	// AreaDrag moves the window when dragged.
	AreaDrag
	// AreaMinimize minimizes the window when released over it.
	AreaMinimize
	// AreaMaximize maximizes or restores the window when released over it.
	AreaMaximize
	// AreaClose closes the window when released over it.
	AreaClose
)

// WindowArea marks d as part of the window's title bar. Elements inside an
// AreaDrag element are dragged with the window too, unless they take input
// of their own (see WindowAreaAt): a search box or a button in the title bar
// stays usable.
func (d *DivElement) WindowArea(a WindowArea) *DivElement { d.area = a; return d }

// WindowAreaAt returns the title-bar area under (x, y), in device-logical
// pixels as for PointerMove, or AreaNone. A point is in an AreaDrag area
// unless an element above that area on the hit path takes input of its own:
// one with OnClick, OnMouseDown, focus (which includes text fields) or OnDrag.
// It uses the hit testing of the last paint, so painted elements only.
func (rt *Runtime) WindowAreaAt(x, y float32) WindowArea {
	x, y = x/rt.zoom, y/rt.zoom
	for _, n := range rt.hitTest(x, y) {
		d := n.div
		if d == nil {
			continue
		}
		if d.interactive() {
			return AreaNone
		}
		if d.area != AreaNone {
			return d.area
		}
	}
	return AreaNone
}

// interactive reports whether the element takes pointer or keyboard input of
// its own.
func (d *DivElement) interactive() bool {
	return d.h.onClick != nil || d.h.onMouseDown != nil || d.focusable || d.dragSpec != nil
}

// WindowControls is the app's handle on the window a runtime is shown in. Get
// it inside a component with Context.Window.
type WindowControls interface {
	Minimize()
	ToggleMaximize()
	// Maximized reports whether the window is maximized. Read inside a
	// component, it is reactive: the component re-renders when it changes.
	Maximized() bool
	Close()
	SetTitle(title string)
	// Focused reports whether the window has keyboard focus. Read inside a
	// component, it is reactive.
	Focused() bool

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
	// MessageBox shows a message box, owned by this window, and runs done with
	// the button chosen.
	MessageBox(o dialog.MessageOptions, done func(dialog.Button))
}

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

func (w windowRef) SetTitle(title string) {
	if c := w.rt.windowCtl; c != nil {
		c.SetTitle(title)
	}
}

func (w windowRef) Maximized() bool { return w.rt.maximized.Get() }
func (w windowRef) Focused() bool   { return w.rt.focused.Get() }

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

// SetWindowControls connects the runtime to its window. The app calls it once,
// before the first frame.
func (rt *Runtime) SetWindowControls(c WindowControls) { rt.windowCtl = c }

// SetMaximized records whether the window is maximized, for components that
// read Context.Window().Maximized(). Call it on the UI thread.
func (rt *Runtime) SetMaximized(on bool) { rt.maximized.Set(on) }

// TitleBarButtons returns the minimize, maximize-or-restore and close buttons
// of a custom title bar, for its right end. They are sized like Windows 11's
// (46×32 logical pixels each) and their areas are marked, so the native hit
// testing presses them, shows Snap Layouts over the maximize button, and
// performs their actions on release. The close button turns red on hover.
func TitleBarButtons() Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		win := ctx.Window()
		maximize, maximizeLabel := icons.Maximize, "Maximize"
		if win.Maximized() {
			maximize, maximizeLabel = icons.Restore, "Restore"
		}
		button := func(ic *icons.Icon, area WindowArea, label string) *DivElement {
			d := Div().Key(label).Width(Px(46)).Height(Px(32)).Center().
				TextColor(t.TextMuted).Transition(120 * time.Millisecond).
				Hover(Style{Background: t.SurfaceHover, TextColor: t.Text}).
				Pressed(Style{Background: t.SurfaceActive, TextColor: t.Text}).
				WindowArea(area).
				Child(Icon(ic).Size(10))
			// Windows shows its own Snap Layouts flyout over the maximize button, so
			// that button gets no tooltip of ours.
			if area != AreaMaximize {
				d.Tooltip(label)
			}
			return d
		}
		closeButton := button(icons.X, AreaClose, "Close").
			Hover(Style{Background: Hex("#e81123"), TextColor: Hex("#ffffff")}).
			Pressed(Style{Background: Hex("#f1707a"), TextColor: Hex("#ffffff")})
		return Div().Row().Height(Px(32)).
			Child(button(icons.Minimize, AreaMinimize, "Minimize")).
			Child(button(maximize, AreaMaximize, maximizeLabel)).
			Child(closeButton)
	})
}

package app

import "github.com/tyler-builds/jamb/ui"

// OpenDockWindow opens the window of a floating dock group with Open. It is the opener
// that ui.Dock.Floating takes:
//
//	ui.Dock(layout, panels).Floating(app.OpenDockWindow)
//
// The window takes the group's title, position and size, and its close handlers are the
// dock's, so the layout records the window's place and the group's panels return to the
// dock when the window closes. The window is not quit with the app's last window: the main
// window should set QuitWhenClosed, so that closing it closes the floating windows too.
func OpenDockWindow(w ui.DockWindow) ui.Window {
	return Open(Window{
		Title:    w.Title,
		X:        w.X,
		Y:        w.Y,
		Width:    w.Width,
		Height:   w.Height,
		Root:     w.Root,
		OnClose:  w.OnClose,
		OnClosed: w.OnClosed,
	})
}

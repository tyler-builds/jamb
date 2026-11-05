package ui

// DefaultTitleBarHeight is the height of the title bar that SetDefaultTitleBar
// draws, in logical pixels: GNOME's header bar height, as the bar is drawn on
// GNOME.
const DefaultTitleBarHeight = 46

// placeWindow places an overlay at the window's top left corner, not kept
// inside a margin (see placeOverlay). The default title bar uses it.
const placeWindow Placement = 120

// SetDefaultTitleBar gives the window a title bar of the runtime's own, with
// title and the caption buttons (minimize, maximize and close), or takes it
// away. The host turns it on for a window that asked for the system's title bar
// on a desktop that draws none, such as GNOME, whose compositor leaves window
// frames to the apps. The app's root element is laid out under the bar, in the
// rest of the window, and keeps its IDs and its layout otherwise. Calling it
// again with on changes the title. Call it on the UI thread.
func (rt *Runtime) SetDefaultTitleBar(on bool, title string) {
	if rt.titleBarText == nil {
		rt.titleBarText = NewSignal(title)
	} else {
		rt.titleBarText.Set(title)
	}
	h := float32(0)
	if on {
		h = DefaultTitleBarHeight
	}
	if h == rt.titleBarHeight {
		return
	}
	rt.titleBarHeight = h
	if rt.tree != nil {
		rt.invalidateLayout(rt.tree)
	}
	if on {
		rt.setLayer(titleBarLayerID, rt.defaultTitleBar)
	} else {
		rt.setLayer(titleBarLayerID, nil)
	}
}

// defaultTitleBar renders the default title bar: a TitleBar, which drags the
// window and holds the caption buttons, with the title in bold in its middle, as
// GNOME's header bar has it, pinned to the top of the window above the app.
func (rt *Runtime) defaultTitleBar(ctx *Context) Element {
	h := Px(rt.titleBarHeight)
	return Div().Overlay(placeWindow).Width(Percent(100)).Height(h).Child(
		TitleBar().Width(Percent(100)).Height(h).Center(Text(rt.titleBarText.Get()).Bold().NoWrap()))
}

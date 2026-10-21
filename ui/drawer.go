package ui

// DrawerSide is the edge of the window a Drawer slides in from.
type DrawerSide uint8

const (
	DrawerRight  DrawerSide = iota // the right edge, the full height of the window (the default)
	DrawerLeft                     // the left edge, the full height of the window
	DrawerTop                      // the top edge, the full width of the window
	DrawerBottom                   // the bottom edge, the full width of the window
)

// drawerWidth and drawerHeight are a drawer panel's size across its side, at
// the left or right and at the top or bottom.
const (
	drawerWidth  = 360
	drawerHeight = 280
)

// DrawerStyle is the look of each part of a drawer: the same parts as a modal's
// (Scrim, Panel, Title, Body and Footer). A part's style merges over the
// built-in defaults, then over the theme's Drawer, then over the Styles of the
// instance; see docs/styling.md.
type DrawerStyle = ModalStyle

// DrawerElement is a panel that slides in from an edge of the window over a
// dimmed backdrop, with a title, a body and footer buttons: settings, details
// or filters that need more room than a popover. It is a modal: build it only
// while it is open, last in the window's root element, as a Modal is.
//
//	ui.If(open.Get(), ui.Drawer("Filters", filters).
//		Side(ui.DrawerRight).Footer(reset, apply).OnClose(close))
//
// Focus moves into it when it opens and returns when it closes, Tab stays
// inside it, and Escape or a press on the backdrop closes it (see OnClose).
type DrawerElement struct{ m ModalElement }

// Drawer creates a drawer with a title and a body, at the right edge.
func Drawer(title string, body Element) *DrawerElement {
	return &DrawerElement{m: ModalElement{title: title, body: body, backdrop: true, drawer: true}}
}

func (*DrawerElement) isElement()                    {}
func (d *DrawerElement) elementKey() string          { return d.m.key }
func (d *DrawerElement) expand(ctx *Context) Element { return d.m.expand(ctx) }

// Key identifies the drawer among its siblings (see DivElement.Key).
func (d *DrawerElement) Key(k string) *DrawerElement { d.m.key = k; return d }

// Side sets the edge the drawer slides in from, DrawerRight by default.
func (d *DrawerElement) Side(s DrawerSide) *DrawerElement { d.m.side = s; return d }

// Footer sets the buttons under the body, at the right, in the order given.
func (d *DrawerElement) Footer(buttons ...Element) *DrawerElement { d.m.Footer(buttons...); return d }

// Default makes b the default button: Enter runs its OnClick (see Modal.Default).
func (d *DrawerElement) Default(b *ButtonElement) *DrawerElement { d.m.Default(b); return d }

// InitialFocus sets the element that takes focus when the drawer opens (see
// Modal.InitialFocus).
func (d *DrawerElement) InitialFocus(h *FocusHandle) *DrawerElement { d.m.InitialFocus(h); return d }

// OnClose sets what runs when the user asks for the drawer to close: Escape, or
// a press on the backdrop. It should remove the drawer from the app's state.
func (d *DrawerElement) OnClose(fn func()) *DrawerElement { d.m.OnClose(fn); return d }

// CloseButton shows a close button at the top right of the drawer, when show is
// set (see Modal.CloseButton).
func (d *DrawerElement) CloseButton(show bool) *DrawerElement { d.m.CloseButton(show); return d }

// Styles sets the look of the parts of the drawer, for this instance only.
func (d *DrawerElement) Styles(st DrawerStyle) *DrawerElement { d.m.Styles(st); return d }

// The sizing methods size the panel across its side: the width of a left or
// right drawer (360 px by default) and the height of a top or bottom one (280 px).
// A percentage is of the window. See Sizing in docs/styling.md.
func (d *DrawerElement) Width(l Length) *DrawerElement { d.m.size.setWidth(l); return d }

// MinWidth sets the smallest width of the drawer.
func (d *DrawerElement) MinWidth(l Length) *DrawerElement { d.m.size.setMinWidth(l); return d }

// MaxWidth sets the largest width of the drawer.
func (d *DrawerElement) MaxWidth(l Length) *DrawerElement { d.m.size.setMaxWidth(l); return d }

// Height sets the height of the drawer.
func (d *DrawerElement) Height(l Length) *DrawerElement { d.m.size.setHeight(l); return d }

// MinHeight sets the smallest height of the drawer.
func (d *DrawerElement) MinHeight(l Length) *DrawerElement { d.m.size.setMinHeight(l); return d }

// MaxHeight sets the largest height of the drawer.
func (d *DrawerElement) MaxHeight(l Length) *DrawerElement { d.m.size.setMaxHeight(l); return d }

// drawerDefaults returns the built-in look of a drawer at side, from the colors
// of t: a modal's parts, with a square panel against the window's edge.
func drawerDefaults(t *Theme, side DrawerSide) DrawerStyle {
	d := modalDefaults(t)
	d.Panel.Radius = 0
	d.Panel.Width, d.Panel.Height = Px(drawerWidth), Auto
	if side == DrawerTop || side == DrawerBottom {
		d.Panel.Width, d.Panel.Height = Auto, Px(drawerHeight)
	}
	return d
}

// resolveDrawer returns the look of a drawer: the defaults, merged with the
// theme's Drawer and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveDrawer(ctx *Context, side DrawerSide, inst DrawerStyle) DrawerStyle {
	t := ctx.Theme()
	return layered(ctx, drawerDefaults(t, side), DrawerStyle.merge, DrawerStyle.plain,
		themeLayer(t.Drawer), instanceLayer(inst))
}

// Edge placements put a drawer's panel against a side of its anchor, the
// backdrop that covers the window. They are not kept inside a margin, as other
// placements are (see placeOverlay).
const (
	edgeRight Placement = 100 + iota
	edgeLeft
	edgeTop
	edgeBottom
)

// edgeOf returns the edge placement of a drawer side.
func edgeOf(s DrawerSide) Placement {
	switch s {
	case DrawerLeft:
		return edgeLeft
	case DrawerTop:
		return edgeTop
	case DrawerBottom:
		return edgeBottom
	}
	return edgeRight
}

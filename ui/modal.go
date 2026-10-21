package ui

import (
	"slices"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// modalWidth is the width of a Modal's panel, in logical pixels.
const modalWidth = 420

// modalMargin is the space, in logical pixels, between a modal's panel and the edges
// of the window when the panel is as tall as the window allows.
const modalMargin = 24

// ModalElement is a dialog drawn over the window: a dimmed backdrop, and a
// panel centered on it with a title, a body and footer buttons. Build it only
// while it is open, as a dropdown is built only while it is open. Place it last
// in the window's root element, which must fill the window:
//
//	ui.If(open.Get(), ui.Modal("New project", form).
//		Footer(cancel, create).Default(create).OnClose(close))
//
// Keyboard focus moves into the modal when it opens, and Tab and Shift+Tab
// cycle through its fields and buttons only. Focus returns to the element that
// had it when the modal closes. Escape closes the modal, as it closes an
// overlay, and a press on the backdrop closes it when CloseOnBackdrop is set.
// Enter runs the default button, except that a focused button is pressed
// instead, and a multi-line field takes the newline.
type ModalElement struct {
	title    string
	body     Element
	footer   []Element
	def      *ButtonElement
	initial  *FocusHandle
	onClose  func()
	backdrop bool
	// closeBtn shows a close button at the top right of the panel (see CloseButton),
	// and closeSet records that CloseButton was called, which a non-modal dialog
	// needs to tell its default from an explicit false.
	closeBtn, closeSet bool
	// nonModal leaves the window behind the dialog usable: there is no scrim, and
	// focus is not trapped (see NonModal). outside makes a press outside the panel
	// close it (see CloseOnClickOutside).
	nonModal, outside bool
	styles            ModalStyle
	size              sizing // the panel's sizes, from the sizing methods (see Sizing in docs/styling.md)
	key               string // the element key, see Key
	// drawer makes the modal a Drawer: its panel is against the window's side
	// edge, with the Drawer parts and styles.
	drawer bool
	side   DrawerSide
	// The fields an AlertDialog sets (see alertdialog.go): the role of the panel,
	// the header that replaces the title, the key of the message that describes
	// the panel, and the resolved look of the parts, whose names are AlertDialog's.
	role        access.Role
	header      Element
	describedBy string
	alert       *ModalStyle
}

// Key identifies the modal among its siblings, so it keeps its state when the
// siblings around it change (see DivElement.Key).
func (m *ModalElement) Key(k string) *ModalElement { m.key = k; return m }

func (m *ModalElement) elementKey() string { return m.key }

// ModalStyle is the look of each part of a modal. A part's style merges over the
// built-in defaults, then over the theme's Modal, then over the Styles of the
// instance; see docs/styling.md. Scrim is the dimmed backdrop, Panel the dialog
// on it, Title its title, Body the area of the body and Footer the row of
// buttons. The panel's width is its Width, 420 px by default. Header is the row of
// the title and the close button, and Close the box around that button; both are
// built only when the modal has a close button (see CloseButton).
type ModalStyle struct {
	Scrim  Style // the dimmed backdrop behind the dialog
	Panel  Style // the dialog
	Header Style // the row of the title and the close button, at the top of the dialog
	Title  Style // the title, at the top of the dialog
	Close  Style // the box around the close button, at the top right
	Body   Style // the body, under the title
	Footer Style // the row of buttons, at the bottom
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ModalStyle) merge(o ModalStyle) ModalStyle {
	return ModalStyle{
		Scrim:  s.Scrim.Merge(o.Scrim),
		Panel:  s.Panel.Merge(o.Panel),
		Header: s.Header.Merge(o.Header),
		Title:  s.Title.Merge(o.Title),
		Close:  s.Close.Merge(o.Close),
		Body:   s.Body.Merge(o.Body),
		Footer: s.Footer.Merge(o.Footer),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ModalStyle) plain() ModalStyle {
	return ModalStyle{
		Scrim:  plainStyle(s.Scrim),
		Panel:  plainStyle(s.Panel),
		Header: plainStyle(s.Header),
		Title:  plainStyle(s.Title),
		Close:  plainStyle(s.Close),
		Body:   plainStyle(s.Body),
		Footer: plainStyle(s.Footer),
	}
}

// modalDefaults returns the built-in look of a modal, from the colors of t.
func modalDefaults(t *Theme) ModalStyle {
	return ModalStyle{
		Scrim: Style{Background: t.Scrim},
		Panel: Style{Width: Px(modalWidth), Background: t.Surface, BorderWidth: 1, BorderColor: t.Border,
			Radius: 10, Shadows: []Shadow{ShadowLg}, Padding: All(20), Gap: 16},
		Header: Style{Direction: Row, Align: Start, Gap: 8},
		Title:  Style{FontSize: 16, FontWeight: FontSemibold},
		Close:  Style{NoShrink: true},
		Body:   Style{Direction: Column, Gap: 12},
		Footer: Style{Direction: Row, Justify: End, Gap: 8, Wrap: true},
	}
}

// resolveModal returns the look of a modal: the defaults, merged with the
// theme's Modal and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveModal(ctx *Context, inst ModalStyle) ModalStyle {
	t := ctx.Theme()
	return layered(ctx, modalDefaults(t), ModalStyle.merge, ModalStyle.plain,
		themeLayer(t.Modal), instanceLayer(inst))
}

// Styles sets the look of the parts of the modal, for this instance only. Set
// fields win over the theme's Modal and the defaults; see ModalStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (m *ModalElement) Styles(st ModalStyle) *ModalElement {
	m.styles = m.styles.merge(st)
	return m
}

// The sizing methods give the panel a width. The panel is 420 px wide by default. A
// percentage is of the window's width, so Width(Percent(90)) leaves a margin on each
// side. See Sizing in docs/styling.md.
func (m *ModalElement) Width(l Length) *ModalElement { m.size.setWidth(l); return m }

// MinWidth sets the smallest width of the modal.
func (m *ModalElement) MinWidth(l Length) *ModalElement { m.size.setMinWidth(l); return m }

// MaxWidth sets the largest width of the modal.
func (m *ModalElement) MaxWidth(l Length) *ModalElement { m.size.setMaxWidth(l); return m }

// modalSpec is what the runtime keeps from a built Modal, on its backdrop
// element (DivElement.modal).
type modalSpec struct {
	def      *ButtonElement // the default button, or nil
	initial  *FocusHandle   // the element to focus first, or nil
	nonModal bool           // a non-modal dialog: it does not trap focus (see NonModal)
	close    func()         // what the dialog's close does, run by Escape from inside a non-modal one
}

// modalFrame is an open modal, and the element that had focus before it opened.
// Focus returns to ret when the modal leaves the tree. last is the element inside
// it that had focus most recently, for a non-modal dialog that F6 enters.
type modalFrame struct {
	id   uint64
	ret  uint64
	last uint64
}

// Modal creates a dialog with a title and a body. Add its buttons with Footer.
func Modal(title string, body Element) *ModalElement {
	return &ModalElement{title: title, body: body}
}

// Footer sets the buttons under the body. They sit at the right, in the order
// given.
func (m *ModalElement) Footer(buttons ...Element) *ModalElement {
	m.footer = buttons
	return m
}

// Default makes b the default button: Enter runs its OnClick, and it takes
// focus when nothing in the body does (see InitialFocus).
func (m *ModalElement) Default(b *ButtonElement) *ModalElement {
	if b.handle == nil {
		b.handle = NewFocusHandle()
	}
	m.def = b
	return m
}

// InitialFocus sets the element that takes focus when the modal opens: the one
// h is attached to. By default that is the first field of the body that Tab
// reaches, or else the default button.
func (m *ModalElement) InitialFocus(h *FocusHandle) *ModalElement {
	m.initial = h
	return m
}

// OnClose sets what runs when the user asks for the modal to close: Escape, or
// a press on the backdrop when CloseOnBackdrop is set. It should remove the
// modal from the app's state. Without OnClose, Escape does nothing.
func (m *ModalElement) OnClose(fn func()) *ModalElement {
	m.onClose = fn
	return m
}

// CloseOnBackdrop makes a press on the backdrop, outside the panel, close the
// modal (see OnClose).
func (m *ModalElement) CloseOnBackdrop() *ModalElement {
	m.backdrop = true
	return m
}

// CloseButton shows a close button at the top right of the panel, beside the
// title, when show is set. Pressing it runs OnClose, and it is built only when
// there is an OnClose. It is a small icon button labelled with Strings.Close,
// first in the Tab order; initial focus skips it. A non-modal dialog shows one
// unless CloseButton(false) is called.
func (m *ModalElement) CloseButton(show bool) *ModalElement {
	m.closeBtn, m.closeSet = show, true
	return m
}

// closeShown reports whether the panel has a close button: one that is asked for,
// or, for a non-modal dialog, one that is not declined, and only with an OnClose.
func (m *ModalElement) closeShown() bool {
	show := m.closeBtn || (m.nonModal && !m.closeSet)
	return show && m.onClose != nil
}

// NonModal makes the dialog non-modal, for a window that stays usable while the
// dialog is open, such as a find bar or a properties sheet. There is no scrim, so
// the window behind is not dimmed, and a press on it reaches the window. Focus is
// not trapped: Tab moves through the whole window, and F6 cycles the dialog with
// the window's other regions. A non-modal dialog has a close button unless
// CloseButton(false) is called. Escape closes it while focus is inside it, and
// Enter runs its default button only then. A press outside it does not close it
// unless CloseOnClickOutside is called. Several non-modal dialogs can be open, and
// the one that was last pressed or focused is on top. A modal that opens later
// covers them, as it covers the window.
func (m *ModalElement) NonModal() *ModalElement {
	m.nonModal = true
	return m
}

// CloseOnClickOutside makes a press outside the panel close the dialog (see
// OnClose). For a modal it is CloseOnBackdrop. A non-modal dialog closes on a
// press in the window behind it only when this is called.
func (m *ModalElement) CloseOnClickOutside() *ModalElement {
	m.backdrop, m.outside = true, true
	return m
}

func (*ModalElement) isElement() {}

func (m *ModalElement) expand(ctx *Context) Element {
	part := "Modal"
	var st ModalStyle
	switch {
	case m.alert != nil:
		part, st = "AlertDialog", *m.alert
	case m.drawer:
		part = "Drawer"
		st = resolveDrawer(ctx, m.side, m.styles)
	default:
		st = resolveModal(ctx, m.styles)
	}
	var title Element = Div().asPart(part + ".Title").Style(st.Title).Shrink(0).Child(Text(m.title))
	if m.header != nil {
		title = m.header
	}
	role := access.RoleDialog
	if m.role != access.RoleUnknown {
		role = m.role
	}
	closeFn := func() {
		if m.onClose != nil {
			m.onClose()
		}
	}
	if m.closeShown() {
		// The title and the close box share a row at the top of the panel. The
		// box's key is scoped by the modal's key, as the message of an alert dialog is.
		key := "jamb.modal/close"
		if m.key != "" {
			key = m.key + "/close"
		}
		closeBox := Div().asPart(part + ".Close").Style(st.Close).Child(
			IconButton(icons.X).Label(ctx.Strings().Close).Small().Ghost().OnClick(closeFn).Key(key))
		closeBox.noInitialFocus = true
		title = Div().asPart(part+".Header").Style(st.Header).Shrink(0).Children(
			Div().asPart(part+".Title").Style(st.Title).Grow(1).Child(Text(m.title)),
			closeBox)
	}
	spec := &modalSpec{def: m.def, initial: m.initial, nonModal: m.nonModal, close: closeFn}

	footer := Div().asPart(part + ".Footer").Style(st.Footer).Shrink(0).Children(m.footer...)
	footer.noInitialFocus = true
	// The panel is at most the window's height less the margin, so its title and footer
	// stay on screen. A body that does not fit scrolls, and the focus reveals the field
	// in it (see FocusHandle.Focus).
	_, winH := ctx.WindowSize()
	placement, margin := OverCenter, float32(modalMargin)
	enter, exit := Fade().Slide(0, -6).Grow(0.98), Fade().Grow(0.98)
	if m.drawer {
		// A drawer's panel is against the window's edge, across the whole side,
		// and slides in from it.
		placement, margin = edgeOf(m.side), 0
		dx, dy := float32(24), float32(0)
		switch m.side {
		case DrawerLeft:
			dx = -24
		case DrawerTop:
			dx, dy = 0, -24
		case DrawerBottom:
			dx, dy = 0, 24
		}
		enter, exit = Fade().Slide(dx, dy), Fade().Slide(dx, dy)
	}
	panel := Div().asPart(part+".Panel").Style(st.Panel).
		// The panel and the backdrop take focus by click, so a click on either
		// does not leave the modal without focus. Neither is in the Tab order.
		Focusable().TabStop(false).
		Enter(enter, Animation{Duration: ctx.Theme().Motion.enter(140 * time.Millisecond)}).
		Exit(exit, Animation{Duration: ctx.Theme().Motion.exit(90 * time.Millisecond)}).
		// A press on the panel is not a press on the backdrop behind it.
		OnMouseDown(func(e *MouseEvent) { e.StopPropagation() }).
		Child(title).
		Child(Div().asPart(part + ".Body").Style(st.Body).Grow(1).ScrollY().Child(m.body)).
		Child(footer).
		Role(role).Label(m.title)
	if !m.nonModal {
		// A modal's panel is an overlay, placed on its backdrop. A non-modal panel is a
		// child of its layer, which holds it in place (see below).
		panel.Overlay(placement)
	}
	if !m.nonModal || m.outside {
		// The overlay's dismiss handler: Escape closes the topmost overlay with one
		// (see "Escape and overlays"). A press outside the panel is always on the
		// backdrop, which the panel counts as its parent, so it does not reach this.
		// A non-modal panel closes on a press outside it only when asked to.
		panel.OnClickOutside(closeFn)
	}
	if m.describedBy != "" {
		panel.DescribedBy(m.describedBy)
	}

	if !m.drawer {
		panel.MaxHeight(Px(winH - 2*margin))
	}
	if m.drawer {
		// The panel spans the backdrop, which covers the window, so it follows the
		// window's size without being built again.
		if m.side == DrawerLeft || m.side == DrawerRight {
			panel.Height(Full)
		} else {
			panel.Width(Full)
		}
	}
	m.size.apply(panel)
	if m.nonModal {
		// There is no scrim. The layer is transparent and covers the window, and
		// hitThrough lets a press on it reach the window behind. The panel sits at its
		// side, or in the middle, inside the margin.
		layer := Div().Absolute().Inset(0).Padding(margin).
			Enter(Fade(), Animation{Duration: ctx.Theme().Motion.enter(140 * time.Millisecond)}).
			Exit(Fade(), Animation{Duration: ctx.Theme().Motion.exit(90 * time.Millisecond)}).
			Child(panel)
		switch {
		case !m.drawer:
			layer.Column().Justify(Center).Align(Center)
		case m.side == DrawerLeft:
			layer.Row().Justify(Start).Align(Stretch)
		case m.side == DrawerRight:
			layer.Row().Justify(End).Align(Stretch)
		case m.side == DrawerTop:
			layer.Column().Justify(Start).Align(Stretch)
		default:
			layer.Column().Justify(End).Align(Stretch)
		}
		layer.hitThrough = true
		layer.modal = spec
		return layer
	}
	// The backdrop's padding is the margin around the panel, so a panel of 100% is the
	// window less the margin.
	backdrop := Div().asPart(part+".Scrim").Absolute().Inset(0).Style(st.Scrim).Padding(margin).
		Focusable().TabStop(false).
		Enter(Fade(), Animation{Duration: ctx.Theme().Motion.enter(140 * time.Millisecond)}).
		Exit(Fade(), Animation{Duration: ctx.Theme().Motion.exit(90 * time.Millisecond)}).
		OnMouseDown(func(e *MouseEvent) {
			if m.backdrop && e.Button == input.ButtonLeft {
				closeFn()
			}
		}).
		Child(panel)
	backdrop.modal = spec
	return backdrop
}

// topModal returns the backdrop of the last open Modal in the tree, or nil. A
// non-modal dialog is not a modal, so it does not trap focus or keys.
func (rt *Runtime) topModal() *node {
	var top *node
	var walk func(n *node)
	walk = func(n *node) {
		if n.div != nil && n.div.modal != nil && !n.div.modal.nonModal {
			top = n
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	if rt.tree != nil {
		walk(rt.tree)
	}
	return top
}

// modalMounted records a Modal that has just appeared. The element with focus
// now is where focus returns when it closes, and the initial focus is set once
// the tree is built (see buildDirty).
func (rt *Runtime) modalMounted(n *node) {
	rt.modals = append(rt.modals, modalFrame{id: n.id, ret: rt.focusID})
	rt.pendingModal = n.id
}

// modalFocus returns the element that takes focus when the modal at n opens:
// the one the modal names, else the first element of the body that Tab reaches,
// else the default button. It returns 0 when there is none.
func (rt *Runtime) modalFocus(n *node) uint64 {
	spec := n.div.modal
	if h := spec.initial; h != nil && h.id != 0 && rt.byID[h.id] != nil {
		return h.id
	}
	var found uint64
	var walk func(c *node)
	walk = func(c *node) {
		if found != 0 || c.flags&flagNoDisplay != 0 || (c.div != nil && (c.div.noInitialFocus || c.div.tabSkip)) {
			return
		}
		if d := c.div; d != nil && d.focusable && !d.noTabStop && d.tabIndex >= 0 {
			found = c.id
			return
		}
		for _, k := range c.children {
			walk(k)
		}
	}
	walk(n)
	if found == 0 && spec.def != nil && spec.def.handle != nil {
		found = spec.def.handle.id
	}
	return found
}

// modalClosed runs when a Modal leaves the tree. Its frame is dropped, and if
// focus was inside it, focus goes back to the element it returns to. Focus that
// has moved elsewhere is left alone.
func (rt *Runtime) modalClosed(n *node) {
	i := slices.IndexFunc(rt.modals, func(f modalFrame) bool { return f.id == n.id })
	if i < 0 {
		return
	}
	ret := rt.modals[i].ret
	rt.modals = slices.Delete(rt.modals, i, i+1)
	if ret != 0 && (rt.focusID == 0 || subtreeHas(n, rt.focusID)) {
		rt.pendingFocus = ret
	}
}

// subtreeHas reports whether n, or a node under it, has the element id.
func subtreeHas(n *node, id uint64) bool {
	if n.id == id {
		return true
	}
	for _, c := range n.children {
		if subtreeHas(c, id) {
			return true
		}
	}
	return false
}

// focusDialog returns the innermost dialog, modal or non-modal, that has focus
// inside it, or nil.
func (rt *Runtime) focusDialog() *node {
	for _, n := range rt.focusPath() {
		if n.div != nil && n.div.modal != nil {
			return n
		}
	}
	return nil
}

// modalDefault runs the default button of the dialog that has focus, as Enter
// does in a dialog, or else of the open modal. A non-modal dialog's default runs
// only from inside it. It reports whether there was one to run.
func (rt *Runtime) modalDefault() bool {
	m := rt.focusDialog()
	if m == nil {
		m = rt.topModal()
	}
	if m == nil {
		return false
	}
	b := m.div.modal.def
	if b == nil || b.disabled || b.loading || b.onClick == nil {
		return false
	}
	b.onClick()
	return true
}

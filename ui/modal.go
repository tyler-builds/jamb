package ui

import (
	"slices"
	"time"

	"github.com/tyler-builds/jamb/access"
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
	styles   ModalStyle
}

// ModalStyle is the look of each part of a modal. A part's style merges over the
// built-in defaults, then over the theme's Modal, then over the Styles of the
// instance; see docs/styling.md. Scrim is the dimmed backdrop, Panel the dialog
// on it, Title its title, Body the area of the body and Footer the row of
// buttons. The panel's width is its Width, 420 px by default.
type ModalStyle struct {
	Scrim  Style // the dimmed backdrop behind the dialog
	Panel  Style // the dialog
	Title  Style // the title, at the top of the dialog
	Body   Style // the body, under the title
	Footer Style // the row of buttons, at the bottom
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ModalStyle) merge(o ModalStyle) ModalStyle {
	return ModalStyle{
		Scrim:  s.Scrim.Merge(o.Scrim),
		Panel:  s.Panel.Merge(o.Panel),
		Title:  s.Title.Merge(o.Title),
		Body:   s.Body.Merge(o.Body),
		Footer: s.Footer.Merge(o.Footer),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ModalStyle) plain() ModalStyle {
	return ModalStyle{
		Scrim:  plainStyle(s.Scrim),
		Panel:  plainStyle(s.Panel),
		Title:  plainStyle(s.Title),
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
		Title:  Style{FontSize: 16, FontWeight: FontSemibold},
		Body:   Style{Direction: Column, Gap: 12},
		Footer: Style{Direction: Row, Justify: End, Gap: 8, Wrap: true},
	}
}

// resolveModal returns the look of a modal: the defaults, merged with the
// theme's Modal and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveModal(t *Theme, inst ModalStyle) ModalStyle {
	st := modalDefaults(t)
	for _, o := range []ModalStyle{t.Modal, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// Styles sets the look of the parts of the modal, for this instance only. Set
// fields win over the theme's Modal and the defaults; see ModalStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (m *ModalElement) Styles(st ModalStyle) *ModalElement {
	m.styles = m.styles.merge(st)
	return m
}

// modalSpec is what the runtime keeps from a built Modal, on its backdrop
// element (DivElement.modal).
type modalSpec struct {
	def     *ButtonElement // the default button, or nil
	initial *FocusHandle   // the element to focus first, or nil
}

// modalFrame is an open modal, and the element that had focus before it opened.
// Focus returns to ret when the modal leaves the tree.
type modalFrame struct {
	id  uint64
	ret uint64
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

func (*ModalElement) isElement() {}

func (m *ModalElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	st := resolveModal(t, m.styles)
	closeFn := func() {
		if m.onClose != nil {
			m.onClose()
		}
	}
	spec := &modalSpec{def: m.def, initial: m.initial}

	footer := Div().asPart("Modal.Footer").Style(st.Footer).Shrink(0).Children(m.footer...)
	footer.modalFooter = true
	// The panel is at most the window's height less the margin, so its title and footer
	// stay on screen. A body that does not fit scrolls, and the focus reveals the field
	// in it (see FocusHandle.Focus).
	_, winH := ctx.WindowSize()
	panel := Div().asPart("Modal.Panel").Overlay(OverCenter).Style(st.Panel).MaxHeight(Px(winH-2*modalMargin)).
		// The panel and the backdrop take focus by click, so a click on either
		// does not leave the modal without focus. Neither is in the Tab order.
		Focusable().TabStop(false).
		Enter(Fade().Slide(0, -6).Grow(0.98), Animation{Duration: 140 * time.Millisecond}).
		Exit(Fade().Grow(0.98), Animation{Duration: 90 * time.Millisecond}).
		// A press on the panel is not a press on the backdrop behind it.
		OnMouseDown(func(e *MouseEvent) { e.StopPropagation() }).
		// The overlay's dismiss handler: Escape closes the topmost overlay with
		// one (see "Escape and overlays"). A press outside the panel is always on
		// the backdrop, which the panel counts as its parent, so it does not
		// reach this.
		OnClickOutside(closeFn).
		Child(Div().asPart("Modal.Title").Style(st.Title).Shrink(0).Child(Text(m.title))).
		Child(Div().asPart("Modal.Body").Style(st.Body).Grow(1).ScrollY().Child(m.body)).
		Child(footer).
		Role(access.RoleDialog).Label(m.title)

	// The backdrop's padding is the margin around the panel, so a panel of 100% is the
	// window less the margin.
	backdrop := Div().asPart("Modal.Scrim").Absolute().Inset(0).Style(st.Scrim).Padding(modalMargin).
		Focusable().TabStop(false).
		Enter(Fade(), Animation{Duration: 140 * time.Millisecond}).
		Exit(Fade(), Animation{Duration: 90 * time.Millisecond}).
		OnMouseDown(func(e *MouseEvent) {
			if m.backdrop && e.Button == input.ButtonLeft {
				closeFn()
			}
		}).
		Child(panel)
	backdrop.modal = spec
	return backdrop
}

// topModal returns the backdrop of the last open Modal in the tree, or nil.
func (rt *Runtime) topModal() *node {
	var top *node
	var walk func(n *node)
	walk = func(n *node) {
		if n.div != nil && n.div.modal != nil {
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
		if found != 0 || c.flags&flagNoDisplay != 0 || (c.div != nil && (c.div.modalFooter || c.div.tabSkip)) {
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

// modalDefault runs the default button of the open modal, as Enter does in a
// dialog. It reports whether there was one to run.
func (rt *Runtime) modalDefault() bool {
	m := rt.topModal()
	if m == nil {
		return false
	}
	b := m.div.modal.def
	if b == nil || b.disabled || b.onClick == nil {
		return false
	}
	b.onClick()
	return true
}

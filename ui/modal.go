package ui

import (
	"slices"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// modalWidth is the width of a Modal's panel, in logical pixels.
const modalWidth = 420

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
	closeFn := func() {
		if m.onClose != nil {
			m.onClose()
		}
	}
	spec := &modalSpec{def: m.def, initial: m.initial}

	footer := Div().Row().Justify(End).Gap(8).Children(m.footer...)
	footer.modalFooter = true
	panel := Div().Overlay(OverCenter).Width(Px(modalWidth)).
		Background(t.Surface).Border(1, t.Border).Radius(10).Shadow(ShadowLg).Padding(20).Gap(16).
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
		Child(Text(m.title).Size(16).Semibold()).
		Child(Div().Column().Gap(12).Child(m.body)).
		Child(footer).
		Role(access.RoleDialog).Label(m.title)

	backdrop := Div().Absolute().Inset(0).Background(t.Scrim).
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
		if found != 0 || c.flags&flagNoDisplay != 0 || (c.div != nil && c.div.modalFooter) {
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

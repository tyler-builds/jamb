package uitest

import (
	"strings"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// Element is an element that Get, ByName, ByRole or Focused found. It is
// looked up again each time it is used, so it follows the UI as it changes;
// a method used after the element has gone fails the test.
//
// An element found by key is the element of that key. Its node in the
// accessibility tree is the element's own, or when it has none, the first node
// inside it: a TextInput's key is on the box around the field, and its node is
// the field's. An element found in the accessibility tree is that node, and the
// element of its key, when it has one.
type Element struct {
	u    *UI
	key  string                          // set when found by key
	find func(*access.Tree) *access.Node // set when found in the access tree
	desc string                          // what was looked for, for failure messages
}

// Exists reports whether the element is still there. It never fails.
func (e *Element) Exists() bool {
	if e.find != nil {
		return e.find(e.u.Tree()) != nil
	}
	return e.u.Has(e.key)
}

// Key returns the element's key, or "" when it has none.
func (e *Element) Key() string {
	if e.find == nil {
		return e.key
	}
	e.u.tb.Helper()
	return e.Node().Key
}

// ---- reading ----

// Node returns the element's node in the accessibility tree as it is now, and
// fails the test if it has none: an element that assistive technology does
// not see, such as a plain layout container, has no node.
func (e *Element) Node() *access.Node {
	e.u.tb.Helper()
	t := e.u.Tree()
	var n *access.Node
	if e.find != nil {
		n = e.find(t)
	} else {
		e.mustExist()
		n = e.u.host.AccessNode(t, e.key)
	}
	if n == nil {
		e.u.tb.Fatalf("uitest: the %s has no node in the access tree:\n%s", e.desc, t.Dump())
	}
	return n
}

// Name returns the element's accessible name.
func (e *Element) Name() string { e.u.tb.Helper(); return e.Node().Name }

// Value returns the element's accessible value: the text of a text field, the
// choice of a combo box.
func (e *Element) Value() string { e.u.tb.Helper(); return e.Node().Value }

// States returns the element's accessibility states.
func (e *Element) States() access.State { e.u.tb.Helper(); return e.Node().States }

// Focused reports whether the element has keyboard focus.
func (e *Element) Focused() bool { e.u.tb.Helper(); return e.States().Has(access.StateFocused) }

// Disabled reports whether the element is shown but cannot be operated.
func (e *Element) Disabled() bool { e.u.tb.Helper(); return e.States().Has(access.StateDisabled) }

// Selected reports whether the element is selected, as a list item or a tab is.
func (e *Element) Selected() bool { e.u.tb.Helper(); return e.States().Has(access.StateSelected) }

// Checked reports whether a check box, switch or radio button is on.
func (e *Element) Checked() bool { e.u.tb.Helper(); return e.States().Has(access.StateChecked) }

// Expanded reports whether the element is open, as a menu, a combo box or a
// tree item is.
func (e *Element) Expanded() bool { e.u.tb.Helper(); return e.States().Has(access.StateExpanded) }

// Text returns the text inside the element: its text elements and the values
// of its text fields, in tree order, joined by spaces. It reads the element of
// the key, so an element found in the access tree must have a key; for one
// without, read Name or Value.
func (e *Element) Text() string {
	e.u.tb.Helper()
	s, ok := e.u.host.TextOf(e.keyed("Text"))
	if !ok {
		e.gone()
	}
	return s
}

// Style returns how the latest frame drew the element (see ui.DrawnStyle):
// its fill, border, text color, opacity and ring, with its hover, press and
// focus styles and its transitions applied. It reads the element of the key,
// so an element found in the access tree must have a key.
func (e *Element) Style() ui.DrawnStyle {
	e.u.tb.Helper()
	d, ok := e.u.rt.StyleOf(e.keyed("Style"))
	if !ok {
		e.gone()
	}
	return d
}

// FocusRing reports whether a ring is drawn around the element, as the built-in
// controls draw their focus ring while focus is shown (see ui.DrawnStyle.Ring).
func (e *Element) FocusRing() bool { e.u.tb.Helper(); return e.Style().RingWidth > 0 }

// Bounds returns the element's rectangle in logical pixels, window
// coordinates, as ui.Runtime.Find reports it.
func (e *Element) Bounds() ui.Rect {
	e.u.tb.Helper()
	if e.find == nil {
		r, ok := e.u.rt.Find(e.key)
		if !ok {
			e.gone()
		}
		return r
	}
	// The access tree's bounds are physical pixels.
	b, s := e.Node().Bounds, e.u.scale*e.u.rt.Zoom()
	return ui.Rect{X: b.X / s, Y: b.Y / s, W: b.W / s, H: b.H / s}
}

// ---- acting ----

// Click moves the pointer to the center of the element and clicks the left
// button.
func (e *Element) Click() {
	e.u.tb.Helper()
	e.press(input.ButtonLeft, 1)
}

// DoubleClick clicks the left button twice at the center of the element.
func (e *Element) DoubleClick() {
	e.u.tb.Helper()
	e.press(input.ButtonLeft, 1)
	e.press(input.ButtonLeft, 2)
}

// RightClick clicks the right button at the center of the element, which opens
// its context menu.
func (e *Element) RightClick() {
	e.u.tb.Helper()
	e.press(input.ButtonRight, 1)
}

// Hover moves the pointer to the center of the element. Advance the clock to
// show its tooltip.
func (e *Element) Hover() {
	e.u.tb.Helper()
	x, y := e.center()
	e.u.rt.PointerMove(x, y, 0)
	e.u.Frame()
}

// Scroll turns the wheel over the element by dx and dy logical pixels:
// positive dy scrolls the content up, to show what is below. The scroll
// container under the pointer moves at once.
func (e *Element) Scroll(dx, dy float32) {
	e.u.tb.Helper()
	x, y := e.center()
	e.u.rt.PointerMove(x, y, 0)
	e.u.rt.Scroll(x, y, dx, dy, true, 0)
	e.u.Frame()
}

// Focus gives the element keyboard focus, as assistive technology does. It does
// not press a key, so focus is not shown as keyboard focus (FocusVisible
// styles) unless a key was pressed since the last click. It fails the test if
// the element cannot take focus.
func (e *Element) Focus() {
	e.u.tb.Helper()
	e.Perform(access.ActionFocus)
}

// Type gives the element focus, unless it has it, and types text into it.
func (e *Element) Type(text string) {
	e.u.tb.Helper()
	if !e.Focused() {
		e.Focus()
	}
	e.u.Type(text)
}

// Perform asks the element to perform an accessibility action, as assistive
// technology does (expand, collapse, toggle, select...), and runs a frame. It
// fails the test if the element does not support the action.
func (e *Element) Perform(a access.Action) {
	e.u.tb.Helper()
	n := e.Node()
	if !n.Has(a) {
		names := make([]string, len(n.Actions))
		for i, x := range n.Actions {
			names[i] = x.String()
		}
		e.u.tb.Fatalf("uitest: the %s does not support %s; its actions are [%s]", e.desc, a, strings.Join(names, ", "))
	}
	e.u.host.Perform(n.ID, access.Request{Action: a})
	e.u.Frame()
}

// ---- helpers ----

// press clicks button at the element's center, as the clicks-th click of a
// series.
func (e *Element) press(b input.MouseButton, clicks int) {
	e.u.tb.Helper()
	x, y := e.center()
	rt := e.u.rt
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, b, clicks, 0)
	rt.PointerUp(x, y, b, 0)
	e.u.Frame()
}

// center returns the element's center in the coordinates of the pointer
// methods: logical pixels before the zoom.
func (e *Element) center() (x, y float32) {
	e.u.tb.Helper()
	r := e.Bounds()
	if r.W <= 0 || r.H <= 0 {
		e.u.tb.Fatalf("uitest: the %s has no size on screen (%v); is it hidden or scrolled away?", e.desc, r)
	}
	z := e.u.rt.Zoom()
	return (r.X + r.W/2) * z, (r.Y + r.H/2) * z
}

// keyed returns the key that a by-key query of method reads, and fails the
// test when the element has none.
func (e *Element) keyed(method string) string {
	e.u.tb.Helper()
	if e.find == nil {
		return e.key
	}
	k := e.Node().Key
	if k == "" {
		e.u.tb.Fatalf("uitest: %s reads an element by its key, and the %s has none; give it one with Key", method, e.desc)
	}
	return k
}

// mustExist fails the test when an element found by key has gone.
func (e *Element) mustExist() {
	e.u.tb.Helper()
	if !e.u.Has(e.key) {
		e.gone()
	}
}

// gone fails the test because the element is no longer there.
func (e *Element) gone() {
	e.u.tb.Helper()
	e.u.tb.Fatalf("uitest: the %s is gone; the element tree is:\n%s", e.desc, e.u.dump())
}

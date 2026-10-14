package ui

import "github.com/tyler-builds/jamb/access"

// An element that draws its own text, such as a code editor on a Canvas, has no
// editor state for the runtime to read. Its package gives the Text pattern through
// TextPattern instead, and the runtime fills in where the text is drawn.

// TextPattern gives the element the Text pattern of a text field or text area, for an
// element that draws its own text. Its role should be access.RoleTextArea. fn is called
// on the UI thread each time the access tree is built, and returns the state of the text:
// its content (access.TextModel), its selection and caret as points, and its font, with
// FontSize in points at the default text size. The runtime sets where the content is drawn:
// OriginX and OriginY from the element's position, Scale from the display scale, Clip from
// the element's bounds, and it scales FontSize by the text size setting. A nil result leaves
// the element without the pattern, and so does a result with no content.
//
// The content must be immutable, since the published tree is read from any goroutine after
// fn returns: it must not refer to state that the UI thread changes. A change of the content's
// text is reported to assistive technology as a text change, and a change of the selection as
// a selection change, when the tree is built again. The tree is built again after a frame that
// changed the UI, so the element should render the state it reports (as an editor renders its
// view): a change that nothing renders waits for the next frame that changes something else.
func (d *DivElement) TextPattern(fn func() *access.TextInfo) *DivElement {
	d.access().text = fn
	return d
}

// TextSelect makes the Text pattern of the element writable: when assistive technology selects a
// range of its text (Select, or a selection set on the Text pattern), fn is called on the UI
// thread with the range's byte offsets, Anchor and Caret, as the content's model gives them. The
// element should apply the selection and render it. Without TextSelect the pattern is read-only,
// and the element's actions do not include setting the selection.
func (d *DivElement) TextSelect(fn func(anchor, caret int)) *DivElement {
	d.access().selectText = fn
	return d
}

// TextReveal makes ScrollIntoView of the element's text work for an element that scrolls its own
// text: fn is called on the UI thread, and it should scroll the caret into view. Without it,
// ScrollIntoView scrolls the element into view in its scroll container, as for any element.
func (d *DivElement) TextReveal(fn func()) *DivElement {
	d.access().revealText = fn
	return d
}

// textPattern returns the Text pattern of the element n, from fn (see TextPattern).
func (b *accessBuilder) textPattern(n *node, fn func() *access.TextInfo) *access.TextInfo {
	ti := fn()
	if ti == nil || ti.Content == nil {
		return nil
	}
	ti.OriginX = n.abs.X * b.rt.scale
	ti.OriginY = n.abs.Y * b.rt.scale
	ti.Scale = b.rt.scale
	ti.Clip = b.bounds(n)
	ti.FontSize *= float64(b.rt.textFactor)
	return ti
}

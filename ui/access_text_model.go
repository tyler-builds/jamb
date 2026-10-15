package ui

import "github.com/tyler-builds/jamb/access"

// An element that draws its own text, such as a code editor on a Canvas, has no
// editor state for the runtime to read. Its package gives the Text pattern through
// DivElement.TextAccess instead, and the runtime fills in where the text is drawn.

// TextAccess is the Text pattern of an element that draws its own text, given to
// DivElement.TextAccess. Info gives the pattern; Select and Reveal are its writable
// side, and each may be nil.
type TextAccess struct {
	// Info returns the state of the text, or nil for no pattern. It is called on the
	// UI thread each time the access tree is built, and returns the content
	// (access.TextModel), the selection and caret as points, and the font, with
	// FontSize in points at the default text size. The runtime sets where the content
	// is drawn: OriginX and OriginY from the element's position, Scale from the display
	// scale, Clip from the element's bounds, and it scales FontSize by the text size
	// setting. A result with no content leaves the element without the pattern.
	//
	// The content must be immutable, since the published tree is read from any
	// goroutine after Info returns: it must not refer to state that the UI thread
	// changes. A change of the content's text is reported to assistive technology as
	// a text change, and a change of the selection as a selection change, when the
	// tree is built again. The tree is built again after a frame that changed the UI,
	// so the element should render the state it reports (as an editor renders its
	// view): a change that nothing renders waits for the next frame that changes
	// something else.
	Info func() *access.TextInfo
	// Select makes the Text pattern writable: when assistive technology selects a range
	// of its text (Select, or a selection set on the Text pattern), Select is called on
	// the UI thread with the range's byte offsets, anchor and caret, as the content's
	// model gives them. The element should apply the selection and render it. Without
	// Select the pattern is read-only, and the element's actions do not include setting
	// the selection.
	Select func(anchor, caret int)
	// Reveal makes ScrollIntoView of the element's text work for an element that
	// scrolls its own text: Reveal is called on the UI thread, and it should scroll the
	// caret into view. Without it, ScrollIntoView scrolls the element into view in its
	// scroll container, as for any element.
	Reveal func()
}

// TextAccess gives the element the Text pattern of a text field or text area, for an
// element that draws its own text (see TextAccess). Its role should be
// access.RoleTextArea.
func (d *DivElement) TextAccess(t TextAccess) *DivElement {
	a := d.access()
	a.text, a.selectText, a.revealText = t.Info, t.Select, t.Reveal
	return d
}

// textPattern returns the Text pattern of the element n, from fn (see TextAccess).
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

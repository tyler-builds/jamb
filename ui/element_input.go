package ui

import "github.com/tyler-builds/jamb/internal/scene"

// Rect is a rectangle in logical pixels: X and Y are the top-left corner, and W and H the size.
// Element-relative where a function says so (IME.Caret), window coordinates otherwise.
type Rect = scene.Rect

// An element that draws its own text, such as a canvas, takes the composition
// of an input method from the host through the hooks below, as a text field
// does (see ime.go). The element shows the composition where its caret is,
// underlined, and it is not part of the element's text until it is committed.
// The callbacks run on the UI thread.

// Composition is the text an input method is composing: Text, with the input
// method's caret at byte Cursor in it, and the clause being converted from byte
// ClauseStart to ClauseEnd. The clause is empty when there is none.
type Composition struct {
	// Text is the text of the composition.
	Text string
	// Cursor is the byte offset of the caret in Text.
	Cursor int
	// ClauseStart and ClauseEnd are the byte range of the clause that is being converted.
	ClauseStart, ClauseEnd int
}

// IME are the input-method hooks of an element that draws its own text, given to
// DivElement.IME.
type IME struct {
	// Update shows the composition where the caret is. It runs with each change of
	// the composition, and with an empty Composition when the composition ends without
	// a result, so the element stops showing it. The element's text does not change:
	// the composition is typed when Commit commits it, and a confirmed result arrives
	// through OnTextInput, as typed text, which replaces the composition.
	Update func(Composition)
	// Commit types the composition as one edit and ends it. The host calls it before
	// something else changes the element or its focus: a click, a key the element
	// handles (see Context.CommitComposition), or focus moving to another element.
	// Without Commit, the composition is dropped.
	Commit func()
	// Caret reports the caret of the element, in logical pixels from its top left,
	// where the input method's candidate window opens. The host calls it when it places
	// that window, so it reads what the last paint drew.
	Caret func() Rect
}

// IME makes the focused element take the composition of an input method itself,
// through the hooks of m (see IME).
func (d *DivElement) IME(m IME) *DivElement {
	d.ime = &m
	return d
}

// composes reports whether the element takes the composition itself (see
// IME).
func (d *DivElement) composes() bool { return d.ime != nil && d.ime.Update != nil }

// CommitComposition types what an input method is composing in the focused
// element. An element that handles a click or a key itself calls it before its
// change, so the composition is typed first (see IME.Commit). It does
// nothing when nothing is composed.
func (c *Context) CommitComposition() { c.rt.commitComposition() }

// public returns c as the Composition that IME.Update reports.
func (c composition) public() Composition {
	return Composition{Text: c.text, Cursor: c.cursor, ClauseStart: c.clauseStart, ClauseEnd: c.clauseEnd}
}

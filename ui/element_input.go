package ui

import "github.com/tyler-builds/jamb/internal/scene"

// Rect is a rectangle in logical pixels: X and Y are the top-left corner, and W and H the size.
// Element-relative where a function says so (CompositionCaret), window coordinates otherwise.
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
	Text                   string
	Cursor                 int
	ClauseStart, ClauseEnd int
}

// compositionHooks are the callbacks of an element that takes the composition
// itself (see OnComposition).
type compositionHooks struct {
	update func(Composition)
	commit func()
	caret  func() Rect
}

// OnComposition makes the focused element show the composition of an input
// method. fn runs with each change of the composition, and with an empty
// Composition when the composition ends without a result, so the element stops
// showing it. The element's text does not change: the composition is typed when
// OnCommitComposition commits it, and a confirmed result arrives through
// OnTextInput, as typed text, which replaces the composition.
func (d *DivElement) OnComposition(fn func(Composition)) *DivElement {
	d.imeHooks().update = fn
	return d
}

// OnCommitComposition sets the function that types the composition as one edit
// and ends it. The host calls it before something else changes the element or
// its focus: a click, a key the element handles (see Context.CommitComposition),
// or focus moving to another element. Without it, the composition is dropped.
func (d *DivElement) OnCommitComposition(fn func()) *DivElement {
	d.imeHooks().commit = fn
	return d
}

// CompositionCaret sets the function that reports the caret of the element, in
// logical pixels from its top left, where the input method's candidate window
// opens. The host calls it when it places that window, so it reads what the
// last paint drew.
func (d *DivElement) CompositionCaret(fn func() Rect) *DivElement {
	d.imeHooks().caret = fn
	return d
}

// imeHooks returns the element's composition hooks, making them on first use.
func (d *DivElement) imeHooks() *compositionHooks {
	if d.ime == nil {
		d.ime = &compositionHooks{}
	}
	return d.ime
}

// composes reports whether the element takes the composition itself (see
// OnComposition).
func (d *DivElement) composes() bool { return d.ime != nil && d.ime.update != nil }

// CommitComposition types what an input method is composing in the focused
// element. An element that handles a click or a key itself calls it before its
// change, so the composition is typed first (see OnCommitComposition). It does
// nothing when nothing is composed.
func (c *Context) CommitComposition() { c.rt.commitComposition() }

// public returns c as the Composition that OnComposition reports.
func (c composition) public() Composition {
	return Composition{Text: c.text, Cursor: c.cursor, ClauseStart: c.clauseStart, ClauseEnd: c.clauseEnd}
}

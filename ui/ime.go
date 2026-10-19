package ui

import (
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
)

// An input method (IME) composes text, such as Chinese, Japanese or Korean,
// before it is typed: the user enters keys, the input method shows what they
// would become and offers candidates, and only the confirmed text is typed.
// The text being composed is drawn inside the focused field, at the caret and
// underlined, as a native field draws it; it is not part of the field's value
// until the input method confirms it, which arrives as typed text. An element
// that draws its own text takes the composition through DivElement.IME instead
// (see element_input.go).

// composition is the text an input method is composing in an editor. The
// offsets are bytes in text.
type composition struct {
	text string
	// cursor is the input method's caret in text.
	cursor int
	// clauseStart and clauseEnd are the clause being converted, which the
	// candidate list is for; they are equal when there is none.
	clauseStart, clauseEnd int
}

// clamped returns c with its offsets moved to the characters of its text.
func (c composition) clamped() composition {
	c.cursor = clampToRune(c.text, c.cursor)
	c.clauseStart = clampToRune(c.text, c.clauseStart)
	c.clauseEnd = max(clampToRune(c.text, c.clauseEnd), c.clauseStart)
	return c
}

// hostCompose shows what an input method is composing in the focused field,
// replacing what it showed before. Empty text ends the composition without
// typing anything: the input method was canceled, or typed its result already.
func (rt *Runtime) hostCompose(c composition) {
	if rt.composing != 0 && rt.composing != rt.focusID {
		rt.dropComposition() // focus moved without a focus change, as when the field went away
	}
	if n := rt.hookFocused(); n != nil {
		rt.hostComposeHook(n, c)
		return
	}
	n, es, v := rt.editorOf(rt.focusID)
	if n == nil || n.editor.locked() || n.editor.password {
		// A read-only field, or no field, shows nothing. A masked field does not show
		// the composition either, as a one-time code input does not: the text that the
		// input method confirms is typed as typed text is.
		return
	}
	if c.text == "" {
		if rt.composing != 0 {
			rt.dropComposition()
			rt.editorTouched(n.id)
		}
		return
	}
	if rt.composing == 0 && es.hasSelection() {
		// The composition replaces the selection, as typing does.
		a, b := es.selection()
		rt.editorSet(n.id, v[:a]+v[b:], a, editOther)
	}
	es.comp = c.clamped()
	rt.composing = n.id
	rt.editorTouched(n.id)
}

// hostComposeHook shows what an input method is composing in n, the focused
// element that takes the composition itself (see DivElement.IME). Empty text
// ends it, as it does in a field.
func (rt *Runtime) hostComposeHook(n *node, c composition) {
	if c.text == "" {
		if rt.composing != 0 {
			rt.dropComposition()
		}
		return
	}
	rt.composing = n.id
	n.div.ime.Update(c.clamped().public())
	rt.needPaint = true
}

// hookFocused returns the focused element when it takes the composition itself
// (see DivElement.IME), or nil.
func (rt *Runtime) hookFocused() *node {
	if n := rt.byID[rt.focusID]; n != nil && n.div != nil && n.div.composes() {
		return n
	}
	return nil
}

// dropComposition forgets the composition without typing it.
func (rt *Runtime) dropComposition() {
	if n := rt.byID[rt.composing]; n != nil {
		if n.st != nil && n.st.edit != nil {
			n.st.edit.comp = composition{}
		}
		if n.div != nil && n.div.composes() {
			n.div.ime.Update(Composition{})
		}
	}
	rt.composing = 0
	rt.needPaint = true
}

// commitComposition types the text being composed into its field, as it is
// shown, and tells the input method to drop it, before something else changes
// the field or focus: a click, a key the field handles, or focus moving away.
// The input method's own confirmation would arrive too late, after that
// change, or in another field.
func (rt *Runtime) commitComposition() {
	id := rt.composing
	if id == 0 {
		return
	}
	if n := rt.byID[id]; n != nil && n.div != nil && n.div.composes() && n.div.ime.Commit != nil {
		// The element types its composition, and ends it (see IME.Commit).
		rt.composing = 0
		rt.needPaint = true
		n.div.ime.Commit()
	} else {
		var t string
		if n := rt.byID[id]; n != nil && n.st != nil && n.st.edit != nil {
			t = n.st.edit.comp.text
		}
		rt.dropComposition()
		if t != "" {
			rt.editorInsert(id, t, editTyping)
		}
	}
	if rt.services.CancelComposition != nil {
		rt.services.CancelComposition()
	}
}

// editorTyped types s into the editor: text from the keyboard, or the result
// an input method confirmed, which takes the place of the composition.
func (rt *Runtime) editorTyped(id uint64, s string) {
	if rt.composing == id {
		rt.dropComposition()
	}
	rt.editorInsert(id, s, editTyping)
}

// composedText returns what an editor shows while it has a composition: the
// value with the composition at the caret. at is where the composition
// starts in the result.
func composedText(v string, es *editState) (shown string, at int) {
	at = clampToRune(v, es.caret)
	return v[:at] + es.comp.text + v[at:], at
}

// paintComposition underlines the composition, which starts at byte at of the
// layout l drawn at x, y: a thin line under all of it, and a thick one under
// the clause being converted. It returns the rectangle the input method's
// candidate list should open next to: the clause, or else the caret.
func (rt *Runtime) paintComposition(l *text.Layout, es *editState, at int, x, y float32, c Color, caret scene.Rect) scene.Rect {
	thin := max(1, float32(int(rt.scale+0.5))) / rt.scale
	underline := func(from, to int, h float32) {
		rt.selRects = l.SelectionRects(from, to, rt.selRects[:0])
		for _, r := range rt.selRects {
			rt.scene.AddQuad(scene.Quad{
				Bounds:     rt.px(scene.Rect{X: x + r.X, Y: y + r.Y + r.H - h, W: r.W, H: h}),
				Background: c.scene(),
			})
		}
	}
	comp := es.comp
	underline(at, at+len(comp.text), thin)
	if comp.clauseEnd > comp.clauseStart {
		underline(at+comp.clauseStart, at+comp.clauseEnd, 2*thin)
		cx, top := l.Caret(at + comp.clauseStart)
		return scene.Rect{X: x + cx, Y: y + top, W: caret.W, H: l.LineHeight}
	}
	return caret
}

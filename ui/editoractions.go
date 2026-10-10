package ui

import (
	"github.com/tyler-builds/jamb/input"
)

// The keys of a text field are actions, so key bindings can change them and
// an app can bind the same actions. DefaultBindings maps the usual keys to
// them; BindIn("Editor", "ctrl+d", DeleteForward) adds one more. Each action
// does what its key did when fields handled keys directly.

var (
	MoveLeft          = NewAction("editor.move-left", "Move left").Hide()
	MoveRight         = NewAction("editor.move-right", "Move right").Hide()
	MoveWordLeft      = NewAction("editor.move-word-left", "Move word left").Hide()
	MoveWordRight     = NewAction("editor.move-word-right", "Move word right").Hide()
	SelectLeft        = NewAction("editor.select-left", "Select left").Hide()
	SelectRight       = NewAction("editor.select-right", "Select right").Hide()
	SelectWordLeft    = NewAction("editor.select-word-left", "Select word left").Hide()
	SelectWordRight   = NewAction("editor.select-word-right", "Select word right").Hide()
	MoveUp            = NewAction("editor.move-up", "Move up").Hide()
	MoveDown          = NewAction("editor.move-down", "Move down").Hide()
	SelectUp          = NewAction("editor.select-up", "Select up").Hide()
	SelectDown        = NewAction("editor.select-down", "Select down").Hide()
	PageUp            = NewAction("editor.page-up", "Page up").Hide()
	PageDown          = NewAction("editor.page-down", "Page down").Hide()
	SelectPageUp      = NewAction("editor.select-page-up", "Select page up").Hide()
	SelectPageDown    = NewAction("editor.select-page-down", "Select page down").Hide()
	MoveLineStart     = NewAction("editor.move-line-start", "Move to line start").Hide()
	MoveLineEnd       = NewAction("editor.move-line-end", "Move to line end").Hide()
	SelectLineStart   = NewAction("editor.select-line-start", "Select to line start").Hide()
	SelectLineEnd     = NewAction("editor.select-line-end", "Select to line end").Hide()
	MoveToStart       = NewAction("editor.move-to-start", "Move to start").Hide()
	MoveToEnd         = NewAction("editor.move-to-end", "Move to end").Hide()
	SelectToStart     = NewAction("editor.select-to-start", "Select to start").Hide()
	SelectToEnd       = NewAction("editor.select-to-end", "Select to end").Hide()
	DeleteBack        = NewAction("editor.delete-back", "Delete back").Hide()
	DeleteForward     = NewAction("editor.delete-forward", "Delete forward").Hide()
	DeleteWordBack    = NewAction("editor.delete-word-back", "Delete word back").Hide()
	DeleteWordForward = NewAction("editor.delete-word-forward", "Delete word forward").Hide()
	Newline           = NewAction("editor.newline", "New line").Hide()
	Indent            = NewAction("editor.indent", "Indent").Hide()
	Confirm           = NewAction("editor.confirm", "Confirm").Hide()
	SelectAll         = NewAction("editor.select-all", "Select all").Hide()
	Copy              = NewAction("editor.copy", "Copy").Hide()
	Cut               = NewAction("editor.cut", "Cut").Hide()
	Paste             = NewAction("editor.paste", "Paste").Hide()
	Undo              = NewAction("editor.undo", "Undo").Hide()
	Redo              = NewAction("editor.redo", "Redo").Hide()
	Cancel            = NewAction("editor.cancel", "Cancel").Hide()
)

// editorActions says what each action does to the field with the given id.
// buildEditor gives every field an OnAction handler for each entry.
var editorActions = []struct {
	a  *Action
	fn func(rt *Runtime, id uint64)
}{
	{MoveLeft, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, -1, false, false) }},
	{MoveRight, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, 1, false, false) }},
	{MoveWordLeft, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, -1, true, false) }},
	{MoveWordRight, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, 1, true, false) }},
	{SelectLeft, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, -1, false, true) }},
	{SelectRight, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, 1, false, true) }},
	{SelectWordLeft, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, -1, true, true) }},
	{SelectWordRight, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, 1, true, true) }},
	{MoveUp, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyUp, false) }},
	{MoveDown, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyDown, false) }},
	{SelectUp, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyUp, true) }},
	{SelectDown, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyDown, true) }},
	{PageUp, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyPageUp, false) }},
	{PageDown, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyPageDown, false) }},
	{SelectPageUp, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyPageUp, true) }},
	{SelectPageDown, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyPageDown, true) }},
	{MoveLineStart, func(rt *Runtime, id uint64) { rt.editorLineEdge(id, true, false) }},
	{MoveLineEnd, func(rt *Runtime, id uint64) { rt.editorLineEdge(id, false, false) }},
	{SelectLineStart, func(rt *Runtime, id uint64) { rt.editorLineEdge(id, true, true) }},
	{SelectLineEnd, func(rt *Runtime, id uint64) { rt.editorLineEdge(id, false, true) }},
	{MoveToStart, func(rt *Runtime, id uint64) { rt.editorDocEdge(id, true, false) }},
	{MoveToEnd, func(rt *Runtime, id uint64) { rt.editorDocEdge(id, false, false) }},
	{SelectToStart, func(rt *Runtime, id uint64) { rt.editorDocEdge(id, true, true) }},
	{SelectToEnd, func(rt *Runtime, id uint64) { rt.editorDocEdge(id, false, true) }},
	{DeleteBack, func(rt *Runtime, id uint64) { rt.editorDeleteStep(id, -1, false) }},
	{DeleteForward, func(rt *Runtime, id uint64) { rt.editorDeleteStep(id, 1, false) }},
	{DeleteWordBack, func(rt *Runtime, id uint64) { rt.editorDeleteStep(id, -1, true) }},
	{DeleteWordForward, func(rt *Runtime, id uint64) { rt.editorDeleteStep(id, 1, true) }},
	{Newline, func(rt *Runtime, id uint64) { rt.editorInsert(id, "\n", editOther) }},
	{Indent, func(rt *Runtime, id uint64) { rt.editorInsert(id, "    ", editTyping) }},
	{Confirm, (*Runtime).editorConfirm},
	{SelectAll, (*Runtime).editorSelectAll},
	{Copy, (*Runtime).editorCopy},
	{Cut, (*Runtime).editorCut},
	{Paste, (*Runtime).editorPaste},
	{Undo, func(rt *Runtime, id uint64) { rt.editorUndo(id, false) }},
	{Redo, func(rt *Runtime, id uint64) { rt.editorUndo(id, true) }},
	{Cancel, func(rt *Runtime, id uint64) { rt.setFocus(0) }},
}

// EditorActions lists the actions of text fields, in the order of the editor
// table. Keymap files may name them.
func EditorActions() []*Action {
	out := make([]*Action, 0, len(editorActions))
	for _, e := range editorActions {
		out = append(out, e.a)
	}
	return out
}

// DefaultBindings are the key bindings every runtime starts with: the keys of
// text fields. NewRuntime adds them first, so an app's binding at the same
// depth wins. In a palette, for example, BindIn("Palette > Editor", "up", ...)
// beats the field's own Up.
func DefaultBindings() []Binding {
	return []Binding{
		BindIn("Editor", "left", MoveLeft),
		BindIn("Editor", "right", MoveRight),
		BindIn("Editor", "ctrl+left", MoveWordLeft),
		BindIn("Editor", "ctrl+right", MoveWordRight),
		BindIn("Editor", "shift+left", SelectLeft),
		BindIn("Editor", "shift+right", SelectRight),
		BindIn("Editor", "ctrl+shift+left", SelectWordLeft),
		BindIn("Editor", "ctrl+shift+right", SelectWordRight),
		BindIn("Editor", "up", MoveUp),
		BindIn("Editor", "down", MoveDown),
		BindIn("Editor", "pageup", PageUp),
		BindIn("Editor", "pagedown", PageDown),
		BindIn("Editor", "shift+up", SelectUp),
		BindIn("Editor", "shift+down", SelectDown),
		BindIn("Editor", "shift+pageup", SelectPageUp),
		BindIn("Editor", "shift+pagedown", SelectPageDown),
		BindIn("Editor", "home", MoveLineStart),
		BindIn("Editor", "end", MoveLineEnd),
		BindIn("Editor", "shift+home", SelectLineStart),
		BindIn("Editor", "shift+end", SelectLineEnd),
		BindIn("Editor", "ctrl+home", MoveToStart),
		BindIn("Editor", "ctrl+end", MoveToEnd),
		BindIn("Editor", "ctrl+shift+home", SelectToStart),
		BindIn("Editor", "ctrl+shift+end", SelectToEnd),
		BindIn("Editor", "backspace", DeleteBack),
		BindIn("Editor", "shift+backspace", DeleteBack),
		BindIn("Editor", "ctrl+backspace", DeleteWordBack),
		BindIn("Editor", "delete", DeleteForward),
		BindIn("Editor", "ctrl+delete", DeleteWordForward),
		BindIn("Editor", "ctrl+a", SelectAll),
		BindIn("Editor", "ctrl+c", Copy),
		BindIn("Editor", "ctrl+x", Cut),
		BindIn("Editor", "ctrl+v", Paste),
		BindIn("Editor", "ctrl+z", Undo),
		BindIn("Editor", "ctrl+shift+z", Redo),
		BindIn("Editor", "ctrl+y", Redo),
		BindIn("Editor", "escape", Cancel),
		BindIn("Editor && mode == full", "enter", Newline),
		BindIn("Editor && mode == full", "shift+enter", Newline),
		BindIn("Editor && mode == full", "tab", Indent),
		BindIn("Editor && mode == single_line", "enter", Confirm),
	}
}

// editorMoveTo puts the caret at p. Without extend the selection collapses
// to p; with it, the selection grows or shrinks from its anchor.
func (rt *Runtime) editorMoveTo(id uint64, p int, extend bool) {
	n, es, _ := rt.editorOf(id)
	if n == nil {
		return
	}
	es.caret = p
	if !extend {
		es.anchor = p
	}
	rt.editorTouched(id)
}

// editorHorizontal moves the caret one grapheme (or word) left (dir -1) or
// right (dir 1). Without extend, a selection collapses to the edge in that
// direction instead of moving.
func (rt *Runtime) editorHorizontal(id uint64, dir int, word, extend bool) {
	n, es, v := rt.editorOf(id)
	if n == nil {
		return
	}
	es.havePrefX = false
	a, b := es.selection()
	if !extend && es.hasSelection() {
		if dir < 0 {
			rt.editorMoveTo(id, a, false)
		} else {
			rt.editorMoveTo(id, b, false)
		}
		return
	}
	p := es.caret
	switch {
	case word && dir < 0:
		p = wordBackward(v, p)
	case word:
		p = wordForward(v, p)
	case dir < 0:
		p = rt.text.PrevGrapheme(v, p)
	default:
		p = rt.text.NextGrapheme(v, p)
	}
	rt.editorMoveTo(id, p, extend)
}

// editorVerticalMove moves the caret by lines (k is Up, Down, PageUp or
// PageDown), keeping its column. A single-line field moves to its start or
// end instead.
func (rt *Runtime) editorVerticalMove(id uint64, k input.Key, extend bool) {
	n, es, v := rt.editorOf(id)
	if n == nil {
		return
	}
	if !n.editor.multiline {
		p := len(v)
		if k == input.KeyUp || k == input.KeyPageUp {
			p = 0
		}
		rt.editorMoveTo(id, p, extend)
		return
	}
	rt.editorMoveTo(id, rt.editorVertical(es, v, k), extend)
}

// editorLineEdge moves to the start or end of the visual line in a
// multi-line field, or to the start or end of the field otherwise.
func (rt *Runtime) editorLineEdge(id uint64, home, extend bool) {
	n, es, v := rt.editorOf(id)
	if n == nil {
		return
	}
	es.havePrefX = false
	p := len(v)
	if home {
		p = 0
	}
	if n.editor.multiline && es.layout != nil {
		start, end := es.layout.LineBounds(es.layout.LineOf(es.caret))
		if home {
			p = start
		} else {
			p = end
			if end > start && end < len(v) && v[end-1] == ' ' && v[end] != '\n' {
				p-- // soft-wrapped line: stay before the space that wrapped
			}
		}
	}
	rt.editorMoveTo(id, p, extend)
}

// editorDocEdge moves to the start or end of the whole field.
func (rt *Runtime) editorDocEdge(id uint64, home, extend bool) {
	n, es, v := rt.editorOf(id)
	if n == nil {
		return
	}
	es.havePrefX = false
	p := len(v)
	if home {
		p = 0
	}
	rt.editorMoveTo(id, p, extend)
}

// editorDeleteStep deletes the selection if there is one, else the grapheme
// (or word) before the caret (dir -1) or after it (dir 1).
func (rt *Runtime) editorDeleteStep(id uint64, dir int, word bool) {
	n, es, v := rt.editorOf(id)
	if n == nil {
		return
	}
	a, b := es.selection()
	switch {
	case a != b:
		rt.editorDelete(id, a, b)
	case dir < 0 && word:
		rt.editorDelete(id, wordBackward(v, a), a)
	case dir < 0:
		rt.editorDelete(id, rt.text.PrevGrapheme(v, a), a)
	case word:
		rt.editorDelete(id, a, wordForward(v, a))
	default:
		rt.editorDelete(id, a, rt.text.NextGrapheme(v, a))
	}
}

func (rt *Runtime) editorConfirm(id uint64) {
	n, _, v := rt.editorOf(id)
	if n == nil {
		return
	}
	if n.editor.onSubmit != nil {
		n.editor.onSubmit(v)
		return
	}
	// A field without OnSubmit passes Enter to the open modal's default button.
	rt.modalDefault()
}

func (rt *Runtime) editorSelectAll(id uint64) {
	n, es, v := rt.editorOf(id)
	if n == nil {
		return
	}
	es.anchor, es.caret = 0, len(v)
	rt.editorTouched(id)
}

func (rt *Runtime) editorCopy(id uint64) {
	n, es, v := rt.editorOf(id)
	if n == nil || !es.hasSelection() {
		return
	}
	a, b := es.selection()
	rt.setClipboard(v[a:b])
}

func (rt *Runtime) editorCut(id uint64) {
	rt.editorCopy(id)
	n, es, v := rt.editorOf(id)
	if n == nil || !es.hasSelection() {
		return
	}
	a, b := es.selection()
	rt.editorSet(id, v[:a]+v[b:], a, editOther)
}

func (rt *Runtime) editorPaste(id uint64) {
	if s, ok := rt.clipboard(); ok && s != "" {
		rt.editorInsert(id, s, editOther)
	}
}

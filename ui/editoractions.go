package ui

import (
	"github.com/tyler-builds/jamb/input"
)

// The keys of a text field are actions, so key bindings can change them and
// an app can bind the same actions. DefaultBindings maps the usual keys to
// them. The actions package exports them for apps (actions.Undo...), and
// BindIn("Editor", "ctrl+d", actions.DeleteForward) adds one more key. Each
// action does what its key did when fields handled keys directly.

var (
	moveLeft          = NewAction("editor.move-left", "Move left").Hide()
	moveRight         = NewAction("editor.move-right", "Move right").Hide()
	moveWordLeft      = NewAction("editor.move-word-left", "Move word left").Hide()
	moveWordRight     = NewAction("editor.move-word-right", "Move word right").Hide()
	selectLeft        = NewAction("editor.select-left", "Select left").Hide()
	selectRight       = NewAction("editor.select-right", "Select right").Hide()
	selectWordLeft    = NewAction("editor.select-word-left", "Select word left").Hide()
	selectWordRight   = NewAction("editor.select-word-right", "Select word right").Hide()
	moveUp            = NewAction("editor.move-up", "Move up").Hide()
	moveDown          = NewAction("editor.move-down", "Move down").Hide()
	selectUp          = NewAction("editor.select-up", "Select up").Hide()
	selectDown        = NewAction("editor.select-down", "Select down").Hide()
	pageUp            = NewAction("editor.page-up", "Page up").Hide()
	pageDown          = NewAction("editor.page-down", "Page down").Hide()
	selectPageUp      = NewAction("editor.select-page-up", "Select page up").Hide()
	selectPageDown    = NewAction("editor.select-page-down", "Select page down").Hide()
	moveLineStart     = NewAction("editor.move-line-start", "Move to line start").Hide()
	moveLineEnd       = NewAction("editor.move-line-end", "Move to line end").Hide()
	selectLineStart   = NewAction("editor.select-line-start", "Select to line start").Hide()
	selectLineEnd     = NewAction("editor.select-line-end", "Select to line end").Hide()
	moveToStart       = NewAction("editor.move-to-start", "Move to start").Hide()
	moveToEnd         = NewAction("editor.move-to-end", "Move to end").Hide()
	selectToStart     = NewAction("editor.select-to-start", "Select to start").Hide()
	selectToEnd       = NewAction("editor.select-to-end", "Select to end").Hide()
	deleteBack        = NewAction("editor.delete-back", "Delete back").Hide()
	deleteForward     = NewAction("editor.delete-forward", "Delete forward").Hide()
	deleteWordBack    = NewAction("editor.delete-word-back", "Delete word back").Hide()
	deleteWordForward = NewAction("editor.delete-word-forward", "Delete word forward").Hide()
	newline           = NewAction("editor.newline", "New line").Hide()
	indent            = NewAction("editor.indent", "Indent").Hide()
	confirm           = NewAction("editor.confirm", "Confirm").Hide()
	selectAll         = NewAction("editor.select-all", "Select all").Hide()
	copyAction        = NewAction("editor.copy", "Copy").Hide()
	cut               = NewAction("editor.cut", "Cut").Hide()
	paste             = NewAction("editor.paste", "Paste").Hide()
	undo              = NewAction("editor.undo", "Undo").Hide()
	redo              = NewAction("editor.redo", "Redo").Hide()
	cancel            = NewAction("editor.cancel", "Cancel").Hide()
)

// editorActions says what each action does to the field with the given id.
// buildEditor gives every field an OnAction handler for each entry.
var editorActions = []struct {
	a  *Action
	fn func(rt *Runtime, id uint64)
}{
	{moveLeft, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, -1, false, false) }},
	{moveRight, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, 1, false, false) }},
	{moveWordLeft, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, -1, true, false) }},
	{moveWordRight, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, 1, true, false) }},
	{selectLeft, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, -1, false, true) }},
	{selectRight, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, 1, false, true) }},
	{selectWordLeft, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, -1, true, true) }},
	{selectWordRight, func(rt *Runtime, id uint64) { rt.editorHorizontal(id, 1, true, true) }},
	{moveUp, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyUp, false) }},
	{moveDown, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyDown, false) }},
	{selectUp, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyUp, true) }},
	{selectDown, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyDown, true) }},
	{pageUp, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyPageUp, false) }},
	{pageDown, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyPageDown, false) }},
	{selectPageUp, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyPageUp, true) }},
	{selectPageDown, func(rt *Runtime, id uint64) { rt.editorVerticalMove(id, input.KeyPageDown, true) }},
	{moveLineStart, func(rt *Runtime, id uint64) { rt.editorLineEdge(id, true, false) }},
	{moveLineEnd, func(rt *Runtime, id uint64) { rt.editorLineEdge(id, false, false) }},
	{selectLineStart, func(rt *Runtime, id uint64) { rt.editorLineEdge(id, true, true) }},
	{selectLineEnd, func(rt *Runtime, id uint64) { rt.editorLineEdge(id, false, true) }},
	{moveToStart, func(rt *Runtime, id uint64) { rt.editorDocEdge(id, true, false) }},
	{moveToEnd, func(rt *Runtime, id uint64) { rt.editorDocEdge(id, false, false) }},
	{selectToStart, func(rt *Runtime, id uint64) { rt.editorDocEdge(id, true, true) }},
	{selectToEnd, func(rt *Runtime, id uint64) { rt.editorDocEdge(id, false, true) }},
	{deleteBack, func(rt *Runtime, id uint64) { rt.editorDeleteStep(id, -1, false) }},
	{deleteForward, func(rt *Runtime, id uint64) { rt.editorDeleteStep(id, 1, false) }},
	{deleteWordBack, func(rt *Runtime, id uint64) { rt.editorDeleteStep(id, -1, true) }},
	{deleteWordForward, func(rt *Runtime, id uint64) { rt.editorDeleteStep(id, 1, true) }},
	{newline, func(rt *Runtime, id uint64) { rt.editorInsert(id, "\n", editOther) }},
	{indent, func(rt *Runtime, id uint64) { rt.editorInsert(id, "    ", editTyping) }},
	{confirm, (*Runtime).editorConfirm},
	{selectAll, (*Runtime).editorSelectAll},
	{copyAction, (*Runtime).editorCopy},
	{cut, (*Runtime).editorCut},
	{paste, (*Runtime).editorPaste},
	{undo, func(rt *Runtime, id uint64) { rt.editorUndo(id, false) }},
	{redo, func(rt *Runtime, id uint64) { rt.editorUndo(id, true) }},
	{cancel, func(rt *Runtime, id uint64) {
		// An editor that sets onCancel (a grid's cell editor) handles Escape itself.
		if n := rt.byID[id]; n != nil && n.editor != nil && n.editor.onCancel != nil {
			n.editor.onCancel()
			return
		}
		rt.setFocus(0)
	}},
}

// EditorActions lists the actions of text fields, in the order of the editor
// table. Keymap files may name them. The actions package has each of them as a
// variable.
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
		BindIn("Editor", "left", moveLeft),
		BindIn("Editor", "right", moveRight),
		BindIn("Editor", "ctrl+left", moveWordLeft),
		BindIn("Editor", "ctrl+right", moveWordRight),
		BindIn("Editor", "shift+left", selectLeft),
		BindIn("Editor", "shift+right", selectRight),
		BindIn("Editor", "ctrl+shift+left", selectWordLeft),
		BindIn("Editor", "ctrl+shift+right", selectWordRight),
		BindIn("Editor", "up", moveUp),
		BindIn("Editor", "down", moveDown),
		BindIn("Editor", "pageup", pageUp),
		BindIn("Editor", "pagedown", pageDown),
		BindIn("Editor", "shift+up", selectUp),
		BindIn("Editor", "shift+down", selectDown),
		BindIn("Editor", "shift+pageup", selectPageUp),
		BindIn("Editor", "shift+pagedown", selectPageDown),
		BindIn("Editor", "home", moveLineStart),
		BindIn("Editor", "end", moveLineEnd),
		BindIn("Editor", "shift+home", selectLineStart),
		BindIn("Editor", "shift+end", selectLineEnd),
		BindIn("Editor", "ctrl+home", moveToStart),
		BindIn("Editor", "ctrl+end", moveToEnd),
		BindIn("Editor", "ctrl+shift+home", selectToStart),
		BindIn("Editor", "ctrl+shift+end", selectToEnd),
		BindIn("Editor", "backspace", deleteBack),
		BindIn("Editor", "shift+backspace", deleteBack),
		BindIn("Editor", "ctrl+backspace", deleteWordBack),
		BindIn("Editor", "delete", deleteForward),
		BindIn("Editor", "ctrl+delete", deleteWordForward),
		BindIn("Editor", "ctrl+a", selectAll),
		BindIn("Editor", "ctrl+c", copyAction),
		BindIn("Editor", "ctrl+x", cut),
		BindIn("Editor", "ctrl+v", paste),
		BindIn("Editor", "ctrl+z", undo),
		BindIn("Editor", "ctrl+shift+z", redo),
		BindIn("Editor", "ctrl+y", redo),
		BindIn("Editor", "escape", cancel),
		BindIn("Editor && mode == full", "enter", newline),
		BindIn("Editor && mode == full", "shift+enter", newline),
		BindIn("Editor && mode == full", "tab", indent),
		BindIn("Editor && mode == single_line", "enter", confirm),
		// A combo box's keys come after the editor's: at the same depth the later
		// binding runs first, and an action runs only while its combo box has a handler
		// for it (see combobox.go).
		BindIn("Combobox > Editor", "down", comboboxDown),
		BindIn("Combobox > Editor", "up", comboboxUp),
		BindIn("Combobox > Editor", "enter", comboboxEnter),
		BindIn("Combobox > Editor", "escape", comboboxEscape),
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
		return
	}
	n := rt.byID[id]
	if n == nil || n.editor == nil || n.editor.onPasteImage == nil || n.editor.locked() {
		return
	}
	if img, ok := rt.clipboardImage(); ok {
		n.editor.onPasteImage(img)
	}
}

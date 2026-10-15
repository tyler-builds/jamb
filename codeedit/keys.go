package codeedit

import (
	"strings"

	"github.com/tyler-builds/jamb/ui"
)

// The commands of the editor are actions, so that key bindings and menus can run them, and
// an app can bind them to other keys or turn them off. Bindings maps the usual keys to them,
// in the "CodeEditor" key context that the editor sets, and the package registers them as
// default bindings (ui.RegisterDefaultBindings), so the keys work in every window without the
// app binding them. Movement and typing commands are hidden from a command palette, as the
// ui's text field actions are; the others are listed. An app binds other keys, or turns one
// off, with its own bindings, which win over the defaults:
//
//	rt.AddBindings(ui.BindIn("CodeEditor", "ctrl+k", codeedit.CmdFind))
//	rt.AddBindings(ui.BindIn("CodeEditor", "ctrl+z", ui.Unbind(codeedit.CmdUndo)))
var (
	CmdMoveLeft          = ui.NewAction("codeedit.move-left", "Move left").Hide()
	CmdMoveRight         = ui.NewAction("codeedit.move-right", "Move right").Hide()
	CmdMoveWordLeft      = ui.NewAction("codeedit.move-word-left", "Move word left").Hide()
	CmdMoveWordRight     = ui.NewAction("codeedit.move-word-right", "Move word right").Hide()
	CmdMoveUp            = ui.NewAction("codeedit.move-up", "Move up").Hide()
	CmdMoveDown          = ui.NewAction("codeedit.move-down", "Move down").Hide()
	CmdMoveLineStart     = ui.NewAction("codeedit.move-line-start", "Move to line start").Hide()
	CmdMoveLineEnd       = ui.NewAction("codeedit.move-line-end", "Move to line end").Hide()
	CmdMoveDocStart      = ui.NewAction("codeedit.move-doc-start", "Move to start").Hide()
	CmdMoveDocEnd        = ui.NewAction("codeedit.move-doc-end", "Move to end").Hide()
	CmdPageUp            = ui.NewAction("codeedit.page-up", "Page up").Hide()
	CmdPageDown          = ui.NewAction("codeedit.page-down", "Page down").Hide()
	CmdSelectLeft        = ui.NewAction("codeedit.select-left", "Select left").Hide()
	CmdSelectRight       = ui.NewAction("codeedit.select-right", "Select right").Hide()
	CmdSelectWordLeft    = ui.NewAction("codeedit.select-word-left", "Select word left").Hide()
	CmdSelectWordRight   = ui.NewAction("codeedit.select-word-right", "Select word right").Hide()
	CmdSelectUp          = ui.NewAction("codeedit.select-up", "Select up").Hide()
	CmdSelectDown        = ui.NewAction("codeedit.select-down", "Select down").Hide()
	CmdSelectLineStart   = ui.NewAction("codeedit.select-line-start", "Select to line start").Hide()
	CmdSelectLineEnd     = ui.NewAction("codeedit.select-line-end", "Select to line end").Hide()
	CmdSelectDocStart    = ui.NewAction("codeedit.select-doc-start", "Select to start").Hide()
	CmdSelectDocEnd      = ui.NewAction("codeedit.select-doc-end", "Select to end").Hide()
	CmdSelectPageUp      = ui.NewAction("codeedit.select-page-up", "Select page up").Hide()
	CmdSelectPageDown    = ui.NewAction("codeedit.select-page-down", "Select page down").Hide()
	CmdSelectAll         = ui.NewAction("codeedit.select-all", "Select all").Hide()
	CmdCollapse          = ui.NewAction("codeedit.collapse", "Single cursor").Hide()
	CmdDeleteBack        = ui.NewAction("codeedit.delete-back", "Delete back").Hide()
	CmdDeleteForward     = ui.NewAction("codeedit.delete-forward", "Delete forward").Hide()
	CmdDeleteWordBack    = ui.NewAction("codeedit.delete-word-back", "Delete word back").Hide()
	CmdDeleteWordForward = ui.NewAction("codeedit.delete-word-forward", "Delete word forward").Hide()
	CmdNewline           = ui.NewAction("codeedit.newline", "New line").Hide()
	CmdIndent            = ui.NewAction("codeedit.indent", "Indent").Hide()
	CmdOutdent           = ui.NewAction("codeedit.outdent", "Outdent").Hide()
	CmdUndo              = ui.NewAction("codeedit.undo", "Undo").Hide()
	CmdRedo              = ui.NewAction("codeedit.redo", "Redo").Hide()
	CmdCopy              = ui.NewAction("codeedit.copy", "Copy").Hide()
	CmdCut               = ui.NewAction("codeedit.cut", "Cut").Hide()
	CmdPaste             = ui.NewAction("codeedit.paste", "Paste")
	CmdComment           = ui.NewAction("codeedit.comment", "Toggle line comment")
	CmdAddNextOccurrence = ui.NewAction("codeedit.add-next-occurrence", "Add next occurrence")
	CmdAddCaretAbove     = ui.NewAction("codeedit.add-caret-above", "Add cursor above")
	CmdAddCaretBelow     = ui.NewAction("codeedit.add-caret-below", "Add cursor below")
	CmdFind              = ui.NewAction("codeedit.find", "Find")
	CmdReplace           = ui.NewAction("codeedit.replace", "Replace")
	CmdFindNext          = ui.NewAction("codeedit.find-next", "Find next")
	CmdFindPrevious      = ui.NewAction("codeedit.find-previous", "Find previous")
	CmdGoToLine          = ui.NewAction("codeedit.go-to-line", "Go to line")
	CmdFold              = ui.NewAction("codeedit.fold", "Fold")
	CmdUnfold            = ui.NewAction("codeedit.unfold", "Unfold")
)

// editorAction is one command and what it does to the editor it runs in.
type editorAction struct {
	action *ui.Action
	fn     func(e *EditorElement, vs *viewState)
}

// writable runs fn only when the editor is writable.
func writable(fn func(e *EditorElement, vs *viewState)) func(e *EditorElement, vs *viewState) {
	return func(e *EditorElement, vs *viewState) {
		if !e.readOnly {
			fn(e, vs)
		}
	}
}

// doEdit runs an edit of the document, for a writable editor.
func doEdit(fn func(d *Document) error) func(e *EditorElement, vs *viewState) {
	return writable(func(e *EditorElement, vs *viewState) {
		vs.edit(func(d *Document) error { return fn(d) })
	})
}

// moveCmd runs the movement m, with or without extending the selection.
func moveCmd(m Move, extend bool) func(e *EditorElement, vs *viewState) {
	return func(e *EditorElement, vs *viewState) { vs.move(m, extend) }
}

// editorActions says what each command does. buildEditor's canvas gets an OnAction handler
// for each entry.
var editorActions = buildActions()

func buildActions() []editorAction {
	out := []editorAction{
		{CmdMoveLeft, moveCmd(MoveLeft, false)},
		{CmdMoveRight, moveCmd(MoveRight, false)},
		{CmdMoveWordLeft, moveCmd(MoveWordLeft, false)},
		{CmdMoveWordRight, moveCmd(MoveWordRight, false)},
		{CmdMoveUp, moveCmd(MoveUp, false)},
		{CmdMoveDown, moveCmd(MoveDown, false)},
		{CmdMoveLineStart, moveCmd(MoveLineStart, false)},
		{CmdMoveLineEnd, moveCmd(MoveLineEnd, false)},
		{CmdMoveDocStart, moveCmd(MoveDocStart, false)},
		{CmdMoveDocEnd, moveCmd(MoveDocEnd, false)},
		{CmdPageUp, moveCmd(MovePageUp, false)},
		{CmdPageDown, moveCmd(MovePageDown, false)},
		{CmdSelectLeft, moveCmd(MoveLeft, true)},
		{CmdSelectRight, moveCmd(MoveRight, true)},
		{CmdSelectWordLeft, moveCmd(MoveWordLeft, true)},
		{CmdSelectWordRight, moveCmd(MoveWordRight, true)},
		{CmdSelectUp, moveCmd(MoveUp, true)},
		{CmdSelectDown, moveCmd(MoveDown, true)},
		{CmdSelectLineStart, moveCmd(MoveLineStart, true)},
		{CmdSelectLineEnd, moveCmd(MoveLineEnd, true)},
		{CmdSelectDocStart, moveCmd(MoveDocStart, true)},
		{CmdSelectDocEnd, moveCmd(MoveDocEnd, true)},
		{CmdSelectPageUp, moveCmd(MovePageUp, true)},
		{CmdSelectPageDown, moveCmd(MovePageDown, true)},
		{CmdSelectAll, func(e *EditorElement, vs *viewState) { vs.doc.SelectAll(); vs.restart = true; vs.touch() }},
		{CmdCollapse, func(e *EditorElement, vs *viewState) { vs.doc.Collapse(); vs.touch() }},
		{CmdDeleteBack, doEdit(func(d *Document) error { return d.DeleteBackward() })},
		{CmdDeleteForward, doEdit(func(d *Document) error { return d.DeleteForward() })},
		{CmdDeleteWordBack, doEdit(func(d *Document) error { return d.DeleteWordBackward() })},
		{CmdDeleteWordForward, doEdit(func(d *Document) error { return d.DeleteWordForward() })},
		{CmdNewline, doEdit(func(d *Document) error { return d.Newline() })},
		{CmdIndent, doEdit(func(d *Document) error { return d.Indent() })},
		{CmdOutdent, doEdit(func(d *Document) error { return d.Outdent() })},
		{CmdUndo, writable(func(e *EditorElement, vs *viewState) { vs.undoRedo(false) })},
		{CmdRedo, writable(func(e *EditorElement, vs *viewState) { vs.undoRedo(true) })},
		{CmdCopy, func(e *EditorElement, vs *viewState) { vs.copy() }},
		{CmdCut, writable(func(e *EditorElement, vs *viewState) {
			vs.copy()
			vs.edit(func(d *Document) error { return d.Paste("") })
		})},
		{CmdPaste, writable(func(e *EditorElement, vs *viewState) { vs.paste() })},
		{CmdComment, writable(func(e *EditorElement, vs *viewState) {
			vs.edit(func(d *Document) error { return d.ToggleComment(e.comment) })
		})},
		{CmdAddNextOccurrence, func(e *EditorElement, vs *viewState) { vs.doc.AddNextOccurrence(); vs.reveal = true; vs.touch() }},
		{CmdAddCaretAbove, func(e *EditorElement, vs *viewState) { vs.doc.AddCaretVertical(-1); vs.reveal = true; vs.touch() }},
		{CmdAddCaretBelow, func(e *EditorElement, vs *viewState) { vs.doc.AddCaretVertical(1); vs.reveal = true; vs.touch() }},
		{CmdFind, func(e *EditorElement, vs *viewState) { vs.openBar(barFind) }},
		{CmdReplace, writable(func(e *EditorElement, vs *viewState) { vs.openBar(barReplace) })},
		{CmdFindNext, func(e *EditorElement, vs *viewState) { vs.findStep(true) }},
		{CmdFindPrevious, func(e *EditorElement, vs *viewState) { vs.findStep(false) }},
		{CmdGoToLine, func(e *EditorElement, vs *viewState) { vs.openBar(barGoto) }},
		{CmdFold, func(e *EditorElement, vs *viewState) { vs.doc.Fold(vs.doc.RowOf(vs.doc.Caret())); vs.touch() }},
		{CmdUnfold, func(e *EditorElement, vs *viewState) { vs.doc.Unfold(vs.doc.RowOf(vs.doc.Caret())); vs.touch() }},
	}
	return out
}

// The editor's keys are default bindings of every runtime.
func init() { ui.RegisterDefaultBindings(Bindings()...) }

// Bindings returns the key bindings of the editor's commands, in the "CodeEditor" context
// that the editor's canvas sets. They are registered as default bindings, so an app need not
// add them; it can list them, as a keyboard help does. The punctuation keys name the key
// position as the US layout prints it (see ui.Bind), so ctrl+shift+[ is the key that is [ on
// the US layout.
func Bindings() []ui.Binding {
	b := func(keys string, a *ui.Action) ui.Binding { return ui.BindIn("CodeEditor", keys, a) }
	return []ui.Binding{
		b("left", CmdMoveLeft), b("right", CmdMoveRight),
		b("ctrl+left", CmdMoveWordLeft), b("ctrl+right", CmdMoveWordRight),
		b("up", CmdMoveUp), b("down", CmdMoveDown),
		b("home", CmdMoveLineStart), b("end", CmdMoveLineEnd),
		b("ctrl+home", CmdMoveDocStart), b("ctrl+end", CmdMoveDocEnd),
		b("pageup", CmdPageUp), b("pagedown", CmdPageDown),
		b("shift+left", CmdSelectLeft), b("shift+right", CmdSelectRight),
		b("ctrl+shift+left", CmdSelectWordLeft), b("ctrl+shift+right", CmdSelectWordRight),
		b("shift+up", CmdSelectUp), b("shift+down", CmdSelectDown),
		b("shift+home", CmdSelectLineStart), b("shift+end", CmdSelectLineEnd),
		b("ctrl+shift+home", CmdSelectDocStart), b("ctrl+shift+end", CmdSelectDocEnd),
		b("shift+pageup", CmdSelectPageUp), b("shift+pagedown", CmdSelectPageDown),
		b("ctrl+a", CmdSelectAll), b("escape", CmdCollapse),
		b("backspace", CmdDeleteBack), b("delete", CmdDeleteForward),
		b("ctrl+backspace", CmdDeleteWordBack), b("ctrl+delete", CmdDeleteWordForward),
		b("enter", CmdNewline), b("tab", CmdIndent), b("shift+tab", CmdOutdent),
		b("ctrl+z", CmdUndo), b("ctrl+y", CmdRedo), b("ctrl+shift+z", CmdRedo),
		b("ctrl+c", CmdCopy), b("ctrl+x", CmdCut), b("ctrl+v", CmdPaste),
		b("ctrl+d", CmdAddNextOccurrence),
		b("ctrl+alt+up", CmdAddCaretAbove), b("ctrl+alt+down", CmdAddCaretBelow),
		b("ctrl+f", CmdFind), b("ctrl+h", CmdReplace), b("ctrl+g", CmdGoToLine),
		b("f3", CmdFindNext), b("shift+f3", CmdFindPrevious),
		b("ctrl+/", CmdComment), b("ctrl+shift+[", CmdFold), b("ctrl+shift+]", CmdUnfold),
	}
}

// undoRedo undoes or redoes the latest step of the document, and scrolls to the caret.
func (vs *viewState) undoRedo(redo bool) {
	vs.restart = true
	if redo {
		vs.doc.Redo()
	} else {
		vs.doc.Undo()
	}
	vs.reveal = true
	vs.touch()
}

// copy puts the selected text on the clipboard. Nothing is copied when every selection is a
// caret.
func (vs *viewState) copy() {
	if s := vs.doc.Copy(); s != "" && vs.ctx != nil {
		vs.ctx.SetClipboard(s)
	}
}

// paste types the clipboard's text over the selections, as Paste does. Line endings are
// read as line feeds.
func (vs *viewState) paste() {
	if vs.ctx == nil {
		return
	}
	s, ok := vs.ctx.Clipboard()
	if !ok || s == "" {
		return
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	vs.edit(func(d *Document) error { return d.Paste(s) })
}

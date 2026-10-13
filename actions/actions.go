// Package actions holds the actions of jamb's text fields and text areas: the
// commands their keys run, such as Undo, Copy and MoveWordLeft. Bind them to
// other keys, put them in menus, or turn their keys off:
//
//	ui.BindIn("Editor", "ctrl+d", actions.DeleteForward)
//	ui.BindIn("Editor", "ctrl+z", ui.Unbind(actions.Undo))
//	ui.MenuItem{Action: actions.Copy}
//
// ui.DefaultBindings maps the usual keys to them. Every action here is hidden
// (see ui.Action.Hide), so a command palette leaves them out.
package actions

import "github.com/tyler-builds/jamb/ui"

// The actions are the ones ui's fields handle, looked up by name.
var (
	MoveLeft          = find("editor.move-left")           // Move left
	MoveRight         = find("editor.move-right")          // Move right
	MoveWordLeft      = find("editor.move-word-left")      // Move word left
	MoveWordRight     = find("editor.move-word-right")     // Move word right
	SelectLeft        = find("editor.select-left")         // Select left
	SelectRight       = find("editor.select-right")        // Select right
	SelectWordLeft    = find("editor.select-word-left")    // Select word left
	SelectWordRight   = find("editor.select-word-right")   // Select word right
	MoveUp            = find("editor.move-up")             // Move up
	MoveDown          = find("editor.move-down")           // Move down
	SelectUp          = find("editor.select-up")           // Select up
	SelectDown        = find("editor.select-down")         // Select down
	PageUp            = find("editor.page-up")             // Page up
	PageDown          = find("editor.page-down")           // Page down
	SelectPageUp      = find("editor.select-page-up")      // Select page up
	SelectPageDown    = find("editor.select-page-down")    // Select page down
	MoveLineStart     = find("editor.move-line-start")     // Move to line start
	MoveLineEnd       = find("editor.move-line-end")       // Move to line end
	SelectLineStart   = find("editor.select-line-start")   // Select to line start
	SelectLineEnd     = find("editor.select-line-end")     // Select to line end
	MoveToStart       = find("editor.move-to-start")       // Move to start
	MoveToEnd         = find("editor.move-to-end")         // Move to end
	SelectToStart     = find("editor.select-to-start")     // Select to start
	SelectToEnd       = find("editor.select-to-end")       // Select to end
	DeleteBack        = find("editor.delete-back")         // Delete back
	DeleteForward     = find("editor.delete-forward")      // Delete forward
	DeleteWordBack    = find("editor.delete-word-back")    // Delete word back
	DeleteWordForward = find("editor.delete-word-forward") // Delete word forward
	Newline           = find("editor.newline")             // New line
	Indent            = find("editor.indent")              // Indent
	Confirm           = find("editor.confirm")             // Confirm
	SelectAll         = find("editor.select-all")          // Select all
	Copy              = find("editor.copy")                // Copy
	Cut               = find("editor.cut")                 // Cut
	Paste             = find("editor.paste")               // Paste
	Undo              = find("editor.undo")                // Undo
	Redo              = find("editor.redo")                // Redo
	Cancel            = find("editor.cancel")              // Cancel
)

func find(name string) *ui.Action {
	for _, a := range ui.EditorActions() {
		if a.Name() == name {
			return a
		}
	}
	panic("actions: ui has no editor action " + name)
}

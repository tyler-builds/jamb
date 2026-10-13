package actions

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// Every editor action of ui has exactly one variable here.
func TestEveryEditorActionHasAVariable(t *testing.T) {
	vars := []*ui.Action{
		MoveLeft,
		MoveRight,
		MoveWordLeft,
		MoveWordRight,
		SelectLeft,
		SelectRight,
		SelectWordLeft,
		SelectWordRight,
		MoveUp,
		MoveDown,
		SelectUp,
		SelectDown,
		PageUp,
		PageDown,
		SelectPageUp,
		SelectPageDown,
		MoveLineStart,
		MoveLineEnd,
		SelectLineStart,
		SelectLineEnd,
		MoveToStart,
		MoveToEnd,
		SelectToStart,
		SelectToEnd,
		DeleteBack,
		DeleteForward,
		DeleteWordBack,
		DeleteWordForward,
		Newline,
		Indent,
		Confirm,
		SelectAll,
		Copy,
		Cut,
		Paste,
		Undo,
		Redo,
		Cancel,
	}
	seen := map[*ui.Action]bool{}
	for _, a := range vars {
		if a == nil || seen[a] {
			t.Fatalf("an action is missing or listed twice: %v", a)
		}
		seen[a] = true
	}
	for _, a := range ui.EditorActions() {
		if !seen[a] {
			t.Errorf("ui's editor action %q has no variable in package actions", a.Name())
		}
	}
}

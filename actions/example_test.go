package actions_test

import (
	"github.com/tyler-builds/jamb/actions"
	"github.com/tyler-builds/jamb/ui"
)

// Bindings, an unbinding and a menu item that use the actions of the package.
func Example() {
	ui.BindIn("Editor", "ctrl+d", actions.DeleteForward)
	ui.BindIn("Editor", "ctrl+z", ui.Unbind(actions.Undo))
	_ = ui.MenuItem{Action: actions.Copy}
}

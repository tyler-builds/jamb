package views

import "github.com/tyler-builds/jamb/ui"

// palette is the command palette, open while a.paletteOpen is true (the Palette
// action, Ctrl+Shift+P, opens it). It lists the actions available where focus is,
// and a command run from it acts there: on the note the user was in.
func (a *App) palette() ui.Element { return ui.CommandPalette(a.paletteOpen) }

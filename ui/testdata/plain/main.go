// Command plain draws a button in a runtime with no window, and attaches no
// inspector. The ui tests build it and check that the inspector's functions
// are not linked into its binary.
package main

import (
	"time"

	"github.com/tyler-builds/jamb/ui"
)

func main() {
	rt := ui.NewRuntime(func(*ui.Context) ui.Element { return ui.Button("OK") }, ui.DarkTheme(), nil)
	rt.SetViewport(320, 240, 1)
	rt.Frame(time.Now())
}

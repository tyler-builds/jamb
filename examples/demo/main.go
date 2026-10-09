package main

import (
	"github.com/tyler-builds/jamb/app"

	"github.com/tyler-builds/jamb/examples/demo/views"
)

func main() {
	store := views.NewStore()

	application := app.New()
	application.Run(app.Window{
		Title:     "Jamb Demo",
		Width:     1200,
		Height:    760,
		MinWidth:  760,
		MinHeight: 480,
		// The app draws its own title bar (see views.App).
		Chrome: app.ChromeCustom,
		// Closing the main window quits, even with project windows open.
		QuitWhenClosed: true,
		Root:           views.App(store),
		// Keyboard shortcuts for the main window (see views.Bindings).
		Bindings: views.Bindings(),
	})
}

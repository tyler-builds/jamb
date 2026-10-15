package app_test

import (
	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/ui"
)

// A one-window application. It is compiled, not run, by the tests.
func Example() {
	app.New().Run(app.Window{
		Title: "Hello", Width: 800, Height: 600,
		Root: func(ctx *ui.Context) ui.Element { return ui.Text("Hello") },
	})
}

// Fonts are registered before Run, and apply to every window of the app.
func ExampleApp_Font() {
	var interRegular, interBold []byte // the font files, for example from go:embed
	app.New().
		Font("Inter", ui.FontRegular, interRegular).
		Font("Inter", ui.FontBold, interBold).
		Run(app.Window{Title: "Fonts", Width: 640, Height: 480,
			Root: func(ctx *ui.Context) ui.Element { return ui.Text("Fonts") }})
}

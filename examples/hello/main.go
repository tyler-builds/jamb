// Command hello is a minimal jamb window: a centered greeting and a button
// that counts its clicks.
package main

import (
	"fmt"

	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/ui"
)

func main() {
	clicks := ui.NewSignal(0)

	app.New().Run(app.Window{
		Title:  "Hello",
		Width:  800,
		Height: 600,
		// Root is a component, so it runs again whenever clicks changes.
		Root: func(ctx *ui.Context) ui.Element {
			t := ctx.Theme()
			return ui.Div().Column().Center().Gap(24).Background(t.Background).
				Child(ui.Text("Hello, world!").FontSize(24).TextColor(t.Text)).
				Child(ui.Button(fmt.Sprintf("Clicked %d times", clicks.Get())).
					OnClick(func() { clicks.Update(func(n int) int { return n + 1 }) }))
		},
	})
}

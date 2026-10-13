package views

import "github.com/tyler-builds/jamb/ui"

// The demo's themes are the built-in themes with the part styles the
// Customizing section shows: the line under an underline tab is 3 px. The app
// passes these to app.Window and to ctx.SetTheme, and the theme switches use
// them too.
var (
	DemoDarkTheme  = demoTheme(ui.DarkTheme())
	DemoLightTheme = demoTheme(ui.LightTheme())
)

// demoTheme sets the demo's part styles on base and returns it: the line under
// an underline tab is 3 px, and every button has a 4 px radius.
func demoTheme(base *ui.Theme) *ui.Theme {
	t := base
	t.Tabs = ui.TabsStyle{Indicator: ui.Style{Height: ui.Px(3)}}
	t.Button = ui.ButtonStyle{Root: ui.Style{Radius: 4}}
	return t
}

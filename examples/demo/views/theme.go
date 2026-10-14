package views

import (
	"strings"

	"github.com/tyler-builds/jamb/ui"
)

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

// themeOptions lists the predefined themes for the theme picker, in the order of
// ui.Themes.
var themeOptions = func() []ui.Option[string] {
	var opts []ui.Option[string]
	for _, info := range ui.Themes() {
		opts = append(opts, ui.Option[string]{Value: info.Name, Label: info.Name})
	}
	return opts
}()

// demoThemeNamed returns a new copy of the predefined theme with the name, with
// the demo's part styles. It returns false for a name that is not predefined.
func demoThemeNamed(name string) (*ui.Theme, bool) {
	t, ok := ui.NewTheme(name)
	if !ok {
		return nil, false
	}
	return demoTheme(t), true
}

// themeOptionName returns the name that the theme picker shows for t, or "" for
// a theme that is not predefined. The built-in themes are named "dark" and
// "light" in Theme.Name, and "Dark" and "Light" in ui.Themes.
func themeOptionName(t *ui.Theme) string {
	for _, info := range ui.Themes() {
		if strings.EqualFold(info.Name, t.Name) {
			return info.Name
		}
	}
	return ""
}

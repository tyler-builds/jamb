package views

import (
	"fmt"
	"math"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// Settings shows switches bound to signals, grouped in tabs: the theme and the UI
// zoom, the notifications, and the keyboard shortcuts.
func Settings(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		// The theme and the zoom can also change from the menus and the keys, so the
		// local signals of the switch and the slider are brought up to date here.
		dark := ui.Local(ctx, "dark", t.Dark)
		if dark.Peek() != t.Dark {
			dark.Set(t.Dark)
		}
		zoom := ui.Local(ctx, "zoom", float64(ctx.Zoom()))
		if float32(zoom.Peek()) != ctx.Zoom() {
			zoom.Set(float64(ctx.Zoom()))
		}
		tab := ui.Local(ctx, "tab", "appearance")
		return ui.Div().Grow(1).ScrollY().
			Child(ui.Div().PaddingX(32).PaddingY(28).Gap(20).MaxWidth(ui.Px(760)).
				Child(ui.Div().Gap(4).
					Child(ui.Div().Heading(1).Child(ui.Text("Settings").Size(26).Semibold())).
					Child(ui.Text("Preferences are plain signals; switches and sliders re-render only this page.").Color(t.TextMuted))).
				Child(ui.Tabs(tab,
					ui.Tab[string]{Value: "appearance", Label: "Appearance", Panel: func() ui.Element {
						return Card(t).Gap(14).
							Child(SectionTitle(t, "Appearance")).
							Child(SettingRow(t, "Dark mode", "Use the dark zinc theme.",
								ui.Switch("Dark mode", dark).OnChange(func(on bool) {
									if on {
										ctx.SetTheme(ui.DarkTheme)
									} else {
										ctx.SetTheme(ui.LightTheme)
									}
								}))).
							Child(ui.Divider()).
							Child(SettingRow(t, "Compact activity", "Tighter rows on the Activity page.",
								ui.Switch("Compact activity", s.Compact))).
							Child(ui.Divider()).
							Child(SettingRow(t, "UI zoom", "Scale the whole interface, from 80% to 150%.",
								ui.Slider(zoom, 0.8, 1.5, 0.1).Label("UI zoom").Width(180).
									Format(func(v float64) string { return fmt.Sprintf("%d%%", int(math.Round(v*100))) }).
									OnChange(func(v float64) { ctx.SetZoom(float32(v)) })))
					}},
					ui.Tab[string]{Value: "notifications", Label: "Notifications", Panel: func() ui.Element {
						return Card(t).Gap(14).
							Child(SectionTitle(t, "Notifications")).
							Child(SettingRow(t, "Desktop notifications", "Show a notification when a build finishes.",
								ui.Switch("Desktop notifications", s.Notify))).
							Child(ui.Divider()).
							Child(SettingRow(t, "Danger zone", "Remove every project from this workspace.",
								ui.Button("Delete all").Danger().Small().Icon(icons.Trash).OnClick(func() {
									s.Projects.Set(nil)
								})))
					}},
					ui.Tab[string]{Value: "shortcuts", Label: "Keyboard shortcuts", Panel: func() ui.Element {
						return shortcuts(ctx)
					}},
				).Key("settings-tabs").Label("Settings sections")))
	})
}

// shortcuts lists the shortcut actions with their keys.
func shortcuts(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	card := Card(t).Gap(10).
		Child(SectionTitle(t, "Keyboard shortcuts")).
		Child(ui.Text("The shortcuts work on every page.").Size(13).Color(t.TextMuted))
	for _, a := range shortcutActions {
		keys := ctx.KeysFor(a)
		if keys == "" {
			continue
		}
		card.Child(ui.Divider()).Child(ui.Div().Row().Align(ui.Center).Justify(ui.SpaceBetween).Gap(16).PaddingY(4).
			Child(ui.Text(a.Label()).Size(13)).
			Child(ui.Div().PaddingX(8).Height(ui.Px(22)).Radius(5).Center().Background(t.SurfaceActive).
				Child(ui.Text(keys).Size(12).Mono().Color(t.TextMuted).NoWrap())))
	}
	return card
}

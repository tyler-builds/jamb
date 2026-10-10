package views

import (
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// Settings shows toggles bound to signals, including the theme, and the
// keyboard shortcuts.
func Settings(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		dark := t.Dark
		return ui.Div().Grow(1).ScrollY().
			Child(ui.Div().PaddingX(32).PaddingY(28).Gap(20).MaxWidth(ui.Px(760)).
				Child(ui.Div().Gap(4).
					Child(ui.Text("Settings").Size(26).Semibold()).
					Child(ui.Text("Preferences are plain signals; toggles re-render only this page.").Color(t.TextMuted))).
				Child(Card(t).Gap(14).
					Child(SectionTitle(t, "Appearance")).
					Child(SettingRow(t, "Dark mode", "Use the dark zinc theme.",
						Toggle(t, dark, func(on bool) {
							if on {
								ctx.SetTheme(ui.DarkTheme)
							} else {
								ctx.SetTheme(ui.LightTheme)
							}
						}).Label("Dark mode"))).
					Child(ui.Divider()).
					Child(SettingRow(t, "Compact activity", "Tighter rows on the Activity page.",
						Toggle(t, s.Compact.Get(), s.Compact.Set).Label("Compact activity")))).
				Child(Card(t).Gap(14).
					Child(SectionTitle(t, "Notifications")).
					Child(SettingRow(t, "Desktop notifications", "Show a notification when a build finishes.",
						Toggle(t, s.Notify.Get(), s.Notify.Set).Label("Desktop notifications"))).
					Child(ui.Divider()).
					Child(SettingRow(t, "Danger zone", "Remove every project from this workspace.",
						ui.Button("Delete all").Danger().Small().Icon(icons.Trash).OnClick(func() {
							s.Projects.Set(nil)
						})))).
				Child(shortcuts(ctx)))
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

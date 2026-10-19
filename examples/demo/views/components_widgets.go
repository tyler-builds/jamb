package views

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// widgetsSection shows the smaller widgets: a sidebar, breadcrumbs,
// pagination, avatars, tags, skeletons, a color picker, a date range picker and
// group boxes.
func widgetsSection(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		nav := ui.Local(ctx, "w.nav", "inbox")
		collapsed := ui.Local(ctx, "w.collapsed", false)
		page := ui.Local(ctx, "w.page", 4)
		color := ui.Local(ctx, "w.color", ui.Hex("#3b82f6"))
		from := ui.Local(ctx, "w.from", time.Time{})
		to := ui.Local(ctx, "w.to", time.Time{})
		tags := ui.Local(ctx, "w.tags", []string{"Design", "Backend", "Urgent"})
		loading := ui.Local(ctx, "w.loading", true)
		email := ui.Local(ctx, "w.email", true)
		desktop := ui.Local(ctx, "w.desktop", false)

		tagRow := ui.Div().Row().Wrap().Gap(6)
		for i, name := range tags.Get() {
			tagRow.Child(ui.Tag(name).Key(name).OnRemove(func() {
				cur := tags.Peek()
				tags.Set(append(append([]string(nil), cur[:i]...), cur[i+1:]...))
				ui.Announce(name+" removed", access.Polite)
			}))
		}

		var preview ui.Element
		if loading.Get() {
			preview = ui.Div().Row().Gap(10).Width(ui.Px(260)).
				Child(ui.Skeleton().Circle(36)).
				Child(ui.Div().Grow(1).Child(ui.Skeleton().Lines(3)))
		} else {
			preview = ui.Div().Row().Gap(10).Width(ui.Px(260)).Align(ui.Start).
				Child(ui.Avatar("Dana Lee").Size(36).AccessHidden()).
				Child(ui.Text("Dana Lee left a comment on the design review.").FontSize(13))
		}

		return section(t, "More widgets",
			sample(t, "Sidebar, with Collapsed and Toggle",
				ui.Div().Row().Height(ui.Px(240)).Border(1, t.Border).Radius(8).ClipContent().
					Child(ui.Sidebar(nav,
						ui.SidebarHeading("Mail"),
						ui.SidebarItem{Key: "inbox", Label: "Inbox", Icon: icons.Inbox, Badge: 4},
						ui.SidebarItem{Key: "starred", Label: "Starred", Icon: icons.Star},
						ui.SidebarHeading("Workspace"),
						ui.SidebarItem{Key: "files", Label: "Files", Icon: icons.Folder},
						ui.SidebarItem{Key: "people", Label: "People", Icon: icons.Users, Disabled: true},
					).Key("demo-sidebar").Label("Sample navigation").Collapsed(collapsed).Toggle()).
					Child(ui.Div().Padding(12).Child(ui.Textf("Showing %s", nav.Get()).TextColor(t.TextMuted)))),
			sample(t, "Breadcrumbs, with MaxItems(3)", ui.Div().Gap(10).
				Child(ui.Breadcrumbs(
					ui.Crumb{Label: "Home", OnClick: pressed("Home")},
					ui.Crumb{Label: "Projects", OnClick: pressed("Projects")},
					ui.Crumb{Label: "Website", OnClick: pressed("Website")},
					ui.Crumb{Label: "Settings"})).
				Child(ui.Breadcrumbs(
					ui.Crumb{Label: "Home", OnClick: pressed("Home")},
					ui.Crumb{Label: "Projects", OnClick: pressed("Projects")},
					ui.Crumb{Label: "Website", OnClick: pressed("Website")},
					ui.Crumb{Label: "Pages", OnClick: pressed("Pages")},
					ui.Crumb{Label: "Pricing"}).MaxItems(3))),
			sample(t, "Pagination, and Compact", ui.Div().Gap(10).
				Child(ui.Pagination(page, 20).Key("demo-pages")).
				Child(ui.Pagination(page, 20).Compact().Key("demo-pages-compact")).
				Child(ui.Textf("Page %d", page.Get()).FontSize(12).TextColor(t.TextMuted))),
			sample(t, "Avatars: initials, sizes, and AccessHidden", ui.Div().Row().Gap(8).Align(ui.Center).
				Child(ui.Avatar("Dana Lee")).
				Child(ui.Avatar("Sam Ortiz").Size(40)).
				Child(ui.Avatar("Ari").Size(48)).
				Child(ui.Avatar("Mei Chen").Size(24))),
			sample(t, "Tags with OnRemove", tagRow),
			sample(t, "Skeleton while loading", ui.Div().Gap(10).
				Child(preview).
				Child(ui.Button(map[bool]string{true: "Show content", false: "Show skeleton"}[loading.Get()]).Small().
					OnClick(func() { loading.Set(!loading.Peek()) }))),
			sample(t, "ColorPicker with Alpha and Swatches", ui.Div().Row().Gap(10).Align(ui.Center).
				Child(ui.ColorPicker(color).Alpha().Label("Accent color").
					Swatches(ui.Hex("#ef4444"), ui.Hex("#f59e0b"), ui.Hex("#22c55e"), ui.Hex("#3b82f6"), ui.Hex("#a855f7"))).
				Child(ui.Div().Size(ui.Px(28), ui.Px(28)).Radius(6).Background(color.Get()).Border(1, t.Border))),
			sample(t, "DateRangePicker", ui.Div().Gap(6).
				Child(ui.DateRangePicker(from, to).Label("Report period")).
				Child(ui.Text(rangeText(from.Get(), to.Get())).FontSize(12).TextColor(t.TextMuted))),
			sample(t, "GroupBox, outlined and Fill", ui.Div().Row().Wrap().Gap(12).Align(ui.Start).
				Child(ui.GroupBox("Notifications").Description("How you hear about changes.").Width(ui.Px(240)).
					Child(ui.Checkbox("Email", email)).
					Child(ui.Checkbox("Desktop", desktop))).
				Child(ui.GroupBox("Storage").Fill().Width(ui.Px(240)).
					Child(ui.ProgressBar(0.7).Label("Storage used")).
					Child(ui.Text("7 of 10 GB used").FontSize(12).TextColor(t.TextMuted)))))
	})
}

// rangeText describes a chosen range in days.
func rangeText(from, to time.Time) string {
	if from.IsZero() || to.IsZero() {
		return "No range chosen"
	}
	return fmt.Sprintf("%d days", int(to.Sub(from).Hours()/24)+1)
}

package views

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/demo/assets"
)

var avatar = ui.MustImage(assets.AvatarPNG)

// App returns the root render function for the window. Its actions are the
// keyboard shortcuts (see Bindings); the root handles them for every page.
func App(s *Store) func(ctx *ui.Context) ui.Element {
	goTo := func(p Page) func() { return func() { s.Page.Set(p) } }
	return func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		return ui.Div().Row().Size(ui.Full, ui.Full).
			Background(t.Background).TextColor(t.Text).
			OnAction(GoDashboard, goTo(PageDashboard)).
			OnAction(GoProjects, goTo(PageProjects)).
			OnAction(GoActivity, goTo(PageActivity)).
			OnAction(GoComponents, goTo(PageComponents)).
			OnAction(GoSettings, goTo(PageSettings)).
			OnAction(GoMarkdown, goTo(PageMarkdown)).
			OnAction(FocusSearch, s.Search.Focus).
			// On the root, so Ctrl+F works wherever the focus is, or with none; the
			// filter is there only on the Components page.
			OnAction(FilterComponents, func() {
				if s.Page.Peek() == PageComponents {
					s.SectionField.Focus()
				}
			}).
			OnAction(ShowPalette, func() { s.PaletteOpen.Set(true) }).
			OnAction(QuickOpen, func() { s.QuickOpen.Set(true) }).
			OnAction(ToggleTheme, func() {
				if ctx.Theme().Dark {
					ctx.SetTheme(DemoLightTheme)
				} else {
					ctx.SetTheme(DemoDarkTheme)
				}
			}).
			Child(Sidebar(s)).
			Child(ui.Div().Width(ui.Px(1)).Height(ui.Full).Background(t.Border)).
			Child(Main(s)).
			// The non-modal dialogs come before the modals, so that the modals cover
			// them: the find and properties samples of the Components page.
			Child(sampleFind(s)).
			Child(sampleSheet(s)).
			// Last, so the modals cover the window: the New Project form and the
			// form sample of the Components page.
			Child(newProjectModal(s)).
			Child(sampleModal(s)).
			Child(sampleDrawer(s)).
			Child(sampleAlertDialog(s)).
			// The command palette lists the actions available where focus is: the
			// shortcuts above, and those of the focused element.
			Child(ui.CommandPalette(s.PaletteOpen)).
			// The quick open lists the projects, before the commands it does not list.
			Child(ui.CommandPalette(s.QuickOpen).ItemsOnly().Placeholder("Go to a project").
				Items(func(string) []ui.PaletteItem { return projectItems(s) })).
			// Last of all, so the toasts sit above the modals. One Toaster shows every toast.
			Child(ui.Toaster().Key("toaster"))
	}
}

// projectItems are the projects as the quick open lists them: each by name, with its
// status as the detail, and choosing one shows it on the Projects page.
func projectItems(s *Store) []ui.PaletteItem {
	projects := s.Projects.Peek()
	items := make([]ui.PaletteItem, 0, len(projects))
	for _, p := range projects {
		id := p.ID
		items = append(items, ui.PaletteItem{Key: fmt.Sprint(id), Label: p.Name, Detail: p.Status.String(), Icon: p.Icon,
			Run: func() { s.Selected.Set(id); s.Page.Set(PageProjects) }})
	}
	return items
}

// Sidebar is the navigation column. It re-renders only when the page or the
// project count changes.
func Sidebar(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		page := s.Page.Get()
		count := len(s.Projects.Get())
		focused := ctx.Window().Focused()

		nav := func(p Page, ic *icons.Icon, label string, badge int, a *ui.Action) ui.Element {
			active := p == page
			// Tab reaches each item; Enter or Space opens its page.
			item := ui.Div().Key("nav-"+label).AccessSelected(active).Row().Align(ui.Center).Gap(10).MinHeight(ui.Px(36)).PaddingX(10).Radius(8).
				TextColor(t.TextMuted).Transition(fast).Tooltip(tip(ctx, label, a)).
				Hover(ui.Style{Background: t.SurfaceHover, TextColor: t.Text}).
				When(active, func(d *ui.DivElement) {
					// High contrast fills the selection with Highlight, whose text is AccentText.
					on := t.Text
					if t.HighContrast {
						on = t.AccentText
					}
					d.Background(t.SurfaceActive).TextColor(on)
				}).
				FocusVisible(ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: t.FocusRing}}}).
				Focusable().
				OnClick(func() { s.Page.Set(p) }).
				Child(ui.Icon(ic).Size(17)).
				Child(ui.Text(label).Medium().Grow(1).NoWrap())
			if badge > 0 {
				item.Badged(ui.Badge(badge).Variant(ui.BadgeNeutral).Noun("items"))
			}
			if active {
				item.Child(ui.Div().Absolute().Left(-12).Top(9).Size(ui.Px(3), ui.Px(18)).Radius(2).Background(t.Accent))
			}
			return item
		}

		return ui.Div().Width(ui.Px(244 * layoutScale(ctx))).Height(ui.Full).Shrink(0).
			Background(t.Panel).PaddingX(12).PaddingY(14).Gap(2).SelectionContainer().Label("Navigation").
			Child(brand(t, focused)).
			Child(sectionLabel(t, "Menu")).
			Child(nav(PageDashboard, icons.Dashboard, "Dashboard", 0, GoDashboard)).
			Child(nav(PageProjects, icons.Folder, "Projects", count, GoProjects)).
			Child(nav(PageActivity, icons.Activity, "Activity", 0, GoActivity)).
			Child(nav(PageComponents, icons.Layers, "Components", 0, GoComponents)).
			Child(nav(PageMarkdown, icons.File, "Markdown", 0, GoMarkdown)).
			Child(sectionLabel(t, "Workspace")).
			Child(nav(PageSettings, icons.Settings, "Settings", 0, GoSettings)).
			Child(ui.Spacer()).
			Child(themeSwitch(ctx)).
			Child(userCard(s))
	})
}

// brand is the logo at the top of the sidebar. It is part of the window's title
// bar, so it drags the window; it dims while the window is unfocused.
func brand(t *ui.Theme, focused bool) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(10).PaddingX(6).Height(ui.Px(40)).MarginBottom(8).
		WindowArea(ui.WindowAreaDrag).Opacity(focusOpacity(focused)).
		Child(ui.Div().Size(ui.Px(28), ui.Px(28)).Radius(8).Center().
			BackgroundGradient(ui.LinearGradient(135, t.Accent, ui.Hex("#c026d3")).InOklab()).
			Shadow(ui.Shadow{Y: 2, Blur: 8, Color: t.Accent.Alpha(0.45)}).
			Child(ui.Icon(icons.Zap).Size(16).Color(ui.Hex("#ffffff")))).
		Child(ui.Text("Jamb").FontSize(17).Bold()).
		Child(ui.Text("v0.1").FontSize(12).TextColor(t.TextSubtle))
}

func sectionLabel(t *ui.Theme, s string) ui.Element {
	return ui.Text(s).FontSize(12).Medium().TextColor(t.TextSubtle).
		Style(ui.Style{Padding: ui.Edges{Top: 14, Left: 10, Bottom: 6}})
}

func themeSwitch(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	option := func(ic *icons.Icon, label string, theme *ui.Theme) ui.Element {
		active := ctx.AppTheme() == theme
		return ui.Div().Row().Grow(1).Height(ui.Px(30)).Radius(6).Center().Gap(6).AccessSelected(active).
			TextColor(t.TextMuted).Transition(fast).Tooltip(tip(ctx, label+" theme", ToggleTheme)).
			Hover(ui.Style{TextColor: t.Text}).
			When(active, func(d *ui.DivElement) {
				d.Background(t.Surface).TextColor(t.Text).Shadow(ui.ShadowSm)
			}).
			OnClick(func() { ctx.SetTheme(theme) }).
			Child(ui.Icon(ic).Size(14)).
			Child(ui.Text(label).FontSize(13).Medium())
	}
	return ui.Div().Row().Padding(3).Gap(3).Radius(9).Background(t.SurfaceActive.Alpha(0.6)).MarginBottom(10).
		SelectionContainer().Label("Theme").
		Child(option(icons.Moon, "Dark", DemoDarkTheme)).
		Child(option(icons.Sun, "Light", DemoLightTheme))
}

// userCard is the account row at the bottom of the sidebar; clicking it
// opens a menu above it.
func userCard(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		open := ui.Local(ctx, "menu", false)
		toggle := func() { open.Set(!open.Peek()) }
		close := func() { open.Set(false) }

		row := func(ic *icons.Icon, label string, fn func()) *ui.DivElement {
			return ui.Div().Row().Align(ui.Center).Gap(8).Height(ui.Px(32)).PaddingX(8).Radius(6).
				Hover(ui.Style{Background: t.SurfaceHover}).
				OnClick(fn).
				Child(ui.Icon(ic).Size(15).Color(t.TextMuted)).
				Child(ui.Text(label).FontSize(13))
		}
		card := ui.Div().Key("user-card").Row().Align(ui.Center).Gap(10).Padding(8).Radius(10).
			Transition(fast).Hover(ui.Style{Background: t.SurfaceHover}).
			OnClick(toggle).
			Child(ui.Image(avatar).Alt("Account avatar").Size(ui.Px(34), ui.Px(34)).Radius(17)).
			Child(ui.Div().Grow(1).Gap(1).
				Child(ui.Text("Alex Rivera").FontSize(13).Semibold().Truncate()).
				Child(ui.Text("alex@example.com").FontSize(12).TextColor(t.TextSubtle).Truncate())).
			Child(ui.Icon(icons.MoreHorizontal).Size(16).Color(t.TextSubtle))
		if open.Get() {
			card.Child(ui.Div().Key("user-menu").Overlay(ui.AboveEnd).Width(ui.Px(200)).Padding(4).
				Radius(10).Background(t.Surface).Border(1, t.Border).Shadow(ui.ShadowLg).
				Enter(ui.Fade().Slide(0, 6).Grow(0.98), ui.Animation{Duration: 140 * time.Millisecond}).
				Exit(ui.Fade().Grow(0.98), ui.Animation{Duration: 90 * time.Millisecond}).
				OnClickOutside(close).
				Child(row(icons.Users, "Profile", close)).
				Child(row(icons.Settings, "Settings", func() {
					s.Page.Set(PageSettings)
					close()
				}).Key("menu-settings")).
				Child(row(icons.ArrowRight, "Sign out", close)))
		}
		return card
	})
}

// Main is the header plus the current page.
func Main(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		page := s.Page.Get()
		focused := ctx.Window().Focused()
		var content ui.Element
		switch page {
		case PageDashboard:
			content = Dashboard(s)
		case PageProjects:
			content = Projects(s)
		case PageActivity:
			content = Activity(s)
		case PageComponents:
			content = Components(s)
		case PageSettings:
			content = Settings(s)
		case PageMarkdown:
			content = Markdown()
		}
		// MinWidth 0: the main column fits the window instead of forcing it wider.
		return ui.Div().Grow(1).MinWidth(ui.Px(0)).Height(ui.Full).
			Child(titleBar(t, "")).
			Child(header(ctx, s, page, focused)).
			Child(ui.Div().Height(ui.Px(1)).Background(t.Border)).
			Child(content).
			Child(statusBar(s))
	})
}

// statusBar is the bar along the bottom of the main column: the page, and the
// number of projects and starred projects. It re-renders only when they change.
func statusBar(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		projects := s.Projects.Get()
		starred := 0
		for _, p := range projects {
			if p.Starred {
				starred++
			}
		}
		return ui.StatusBar().
			Start(ui.Text(pageTitles[s.Page.Get()]).Truncate()).
			End(ui.Text(fmt.Sprintf("%d projects", len(projects))), ui.Text(fmt.Sprintf("%d starred", starred)))
	})
}

var pageTitles = map[Page]string{
	PageDashboard: "Dashboard", PageProjects: "Projects", PageActivity: "Activity",
	PageComponents: "Components", PageSettings: "Settings", PageMarkdown: "Markdown",
}

// titleBar is the 32 px strip at the top of a window with a custom title bar: an
// optional title on the left and the caption buttons on the right. Apart from
// the title, the strip drags the window. It dims while the window is
// unfocused. Main and project windows both use it.
func titleBar(t *ui.Theme, title string) ui.Element {
	bar := ui.TitleBar().Height(ui.Px(32))
	if title != "" {
		bar.Child(ui.Div().Shrink(1).MinWidth(ui.Px(0)).PaddingX(12).
			Child(ui.Text(title).FontSize(12).Medium().TextColor(t.TextSubtle).Truncate()))
	}
	return bar
}

// focusOpacity dims the title bar slightly while the window is unfocused.
func focusOpacity(focused bool) float32 {
	if focused {
		return 1
	}
	return 0.6
}

// header is the breadcrumb and the search, notifications and New Project row.
// Its background drags the window; the controls in it do not.
func header(ctx *ui.Context, s *Store, page Page, focused bool) ui.Element {
	t := ctx.Theme()
	return ui.Div().Row().Wrap().MinHeight(ui.Px(60)).Shrink(0).Align(ui.Center).Justify(ui.SpaceBetween).PaddingX(24).Gap(16).
		WindowArea(ui.WindowAreaDrag).
		Child(ui.Div().Row().Align(ui.Center).Gap(8).MinWidth(ui.Px(0)).Opacity(focusOpacity(focused)).
			Child(ui.Text("Workspace").TextColor(t.TextSubtle).Truncate()). // gives way first when narrow
			Child(ui.Icon(icons.ChevronRight).Size(14).Color(t.TextSubtle)).
			Child(ui.Text(pageTitles[page]).Medium().Truncate())).
		Child(ui.Div().Row().Align(ui.Center).Gap(8).Wrap().
			// The wrapper carries the tooltip, since a text field has none.
			Child(ui.Div().Tooltip(tip(ctx, "Search", FocusSearch)).
				// The suggestions are the projects; picking one selects it. MinWidth lets the
				// combo box shrink when the header is narrow; the controls wrap past it.
				Child(ui.Combobox(s.Query, searchOptions(s)).Label("Search projects").Placeholder("Search projects…").
					Icon(icons.Search).FocusHandle(s.Search).EmptyText("No matching projects").
					Width(ui.Px(240 * layoutScale(ctx))).MinWidth(ui.Px(120)).
					RenderOption(searchOption(s, t)).
					OnChange(func(string) { s.Page.Set(PageProjects) }).
					OnPick(func(id int) { s.Selected.Set(id); s.Page.Set(PageProjects) }))).
			Child(inboxButton(ctx, s)).
			Child(ui.Button("New Project").Key("new-project").Icon(icons.Plus).OnClick(s.OpenNewProject)))
}

// layoutScale grows the widths of the wide controls with the Text size, up to
// 150%, so that their labels fit. Heights need no help: they are minimums that
// grow with their content.
func layoutScale(ctx *ui.Context) float32 {
	return min(float32(ctx.TextScale())/100, 1.5)
}

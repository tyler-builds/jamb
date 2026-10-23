package views

import (
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// titleBar is the app's own title bar: the mark and the repository's breadcrumb
// at the start, the search centered on the window, and Refresh, the theme and the
// window buttons at the end. ui.TitleBar drags the window from its empty parts
// and dims while the window is in the background.
func (a *App) titleBar(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	mark := ui.Div().Center().Size(ui.Px(22), ui.Px(22)).Radius(6).Shrink(0).
		BackgroundGradient(ui.LinearGradient(135, ui.Hex("#7c86f0"), ui.Hex("#4f5bd5")).InOklab()).
		Child(ui.Icon(icons.GitBranch).Size(13).Color(ui.Hex("#ffffff")))
	return ui.TitleBar().Key("titlebar").Height(ui.Px(44)).
		Style(ui.Style{Background: t.Panel, BorderEdges: ui.Edges{Bottom: 1}, BorderColor: t.Border}).
		Child(ui.Div().Row().Align(ui.Center).Gap(10).PaddingLeft(14).MinWidth(ui.Px(0)).
			Child(mark).Child(a.crumbs())).
		Center(a.searchField()).
		Trailing(
			ui.IconButton(icons.Refresh).Ghost().Small().Label("Refresh").Tooltip("Refresh (F5)").OnClick(a.refresh),
			ui.IconButton(themeIcon(t)).Ghost().Small().Label("Toggle theme").Tooltip("Light or dark theme").
				OnClick(func() { a.toggleTheme(ctx) }),
		)
}

func themeIcon(t *ui.Theme) *icons.Icon {
	if t.Dark {
		return icons.Sun
	}
	return icons.Moon
}

// crumbs names the repository and its branch, as "jamb / main".
func (a *App) crumbs() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		r := a.s.CurrentRepo()
		if r == nil {
			return ui.Text("Git").FontWeight(ui.FontSemibold)
		}
		info := r.Info.Get()
		head := info.Head
		if head == "" && len(info.HeadHash) >= 7 {
			head = info.HeadHash[:7]
		}
		row := ui.Div().Row().Align(ui.Center).Gap(8).MinWidth(ui.Px(0)).
			Child(ui.Text(info.Name).FontWeight(ui.FontSemibold).Truncate())
		if head != "" {
			row.Child(ui.Text("/").TextColor(t.TextSubtle)).
				Child(ui.Div().Row().Align(ui.Center).Gap(5).
					Child(ui.Icon(icons.GitBranch).Size(13).Color(t.TextMuted)).
					Child(ui.Text(head).TextColor(t.TextMuted).Truncate()))
		}
		return row
	})
}

// searchField filters the history of the repository shown. Enter and F3 go to
// the next match, Shift+Enter and Shift+F3 to the previous one.
func (a *App) searchField() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		r := a.s.CurrentRepo()
		if r == nil {
			return ui.Div()
		}
		v := a.viewOf(r)
		q := v.query.Get()
		trailing := ui.Element(ui.Kbd("Ctrl+F"))
		if q != "" {
			n := 0
			for _, e := range r.Entries.Get() {
				if matches(q, e.Hash, e.Subject, e.Author) {
					n++
				}
			}
			trailing = ui.Text(itoa(n) + " found").FontSize(11.5).TextColor(t.TextSubtle)
		}
		return ui.TextInput(v.query).Key("search").Small().Label("Find commits").
			Placeholder("Find commits, authors, hashes").Icon(icons.Search).Clearable().
			FocusHandle(a.search).Trailing(trailing).
			OnSubmit(func(string) { a.step(1) }).
			OnKeyDown(func(e *ui.KeyEvent) {
				if e.Key == input.KeyEnter && e.Mods == input.ModShift {
					a.step(-1)
					e.StopPropagation()
				}
			}).
			Style(ui.Style{Width: ui.Px(300)})
	})
}

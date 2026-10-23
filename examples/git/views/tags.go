package views

import (
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/repo"
	"github.com/tyler-builds/jamb/examples/git/store"
)

// tags is the table of the repository's tags, newest first. Enter or a double
// click shows the tag's commit in the history.
func (a *App) tags(r *store.Repo) ui.Element {
	v := a.viewOf(r)
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		now := a.layout.Now()
		var tags []repo.Ref
		for _, ref := range r.Refs.Get() {
			if ref.Kind == repo.Tag {
				tags = append(tags, ref)
			}
		}
		pad := func(el ui.Element) ui.Element {
			return ui.Div().Row().Align(ui.Center).Grow(1).MinWidth(ui.Px(0)).PaddingX(8).Gap(8).Child(el)
		}
		cols := []ui.DataColumn[repo.Ref]{
			{Key: "name", Header: "Tag", Width: 220, MinWidth: 100, Resizable: true,
				Text:    func(g repo.Ref) string { return g.Name },
				Compare: ui.SortByText(func(g repo.Ref) string { return g.Name }),
				Cell: func(g repo.Ref) ui.Element {
					return pad(ui.Div().Row().Align(ui.Center).Gap(8).MinWidth(ui.Px(0)).
						Child(ui.Icon(icons.Tag).Size(13).Color(t.TextMuted)).
						Child(ui.Text(g.Name).FontWeight(ui.FontMedium).Truncate()))
				}},
			{Key: "message", Header: "Message", Width: 360, Grow: 1, MinWidth: 120, Resizable: true,
				Text: func(g repo.Ref) string { return g.Subject },
				Cell: func(g repo.Ref) ui.Element {
					return pad(ui.Text(g.Subject).TextColor(t.TextMuted).Truncate())
				}},
			{Key: "kind", Header: "Kind", Width: 110, Resizable: true,
				Text:    kindText,
				Compare: ui.SortByText(kindText),
				Cell: func(g repo.Ref) ui.Element {
					return pad(ui.Tag(kindText(g)))
				}},
			{Key: "date", Header: "Date", Width: 130, Resizable: true,
				Text:    func(g repo.Ref) string { return ui.FormatRelative(g.When, now) },
				Compare: ui.SortBy(func(g repo.Ref) int64 { return g.When.Unix() }),
				Cell: func(g repo.Ref) ui.Element {
					return pad(ui.Div().TextColor(t.TextMuted).Child(ui.TimeAgo(g.When)))
				}},
			{Key: "commit", Header: "Commit", Width: 90, Resizable: true,
				Text: func(g repo.Ref) string { return g.Target[:min(7, len(g.Target))] },
				Cell: func(g repo.Ref) ui.Element {
					return pad(ui.Text(g.Target[:min(7, len(g.Target))]).Mono().FontSize(12).TextColor(t.TextSubtle))
				}},
		}
		return ui.DataGrid(tags, func(g repo.Ref) string { return g.Name }, cols...).
			Key("tags").Label("Tags").Sort(v.tagSort).
			Selection(ui.SelectionSingle, nil).
			OnActivate(func(g repo.Ref) { a.revealTag(ctx, r, g) }).
			ContextMenu(func(g repo.Ref, _ string) []ui.MenuItem {
				return []ui.MenuItem{
					{Label: "Show in history", Icon: icons.Activity, OnClick: func() { a.revealTag(ctx, r, g) }},
					{Label: "Copy tag name", Icon: icons.Copy, OnClick: func() { ctx.SetClipboard(g.Name) }},
				}
			}).
			Empty(ui.Empty("No tags").Icon(icons.Tag).Description("Tags mark releases and other points in the history.")).
			Styles(gridStyle(t)).
			Style(ui.Style{Grow: 1})
	})
}

func kindText(g repo.Ref) string {
	if g.Annotated {
		return "Annotated"
	}
	return "Lightweight"
}

func (a *App) revealTag(ctx *ui.Context, r *store.Repo, g repo.Ref) {
	a.revealCommit(ctx, r, g.Target, g.Name)
}

package views

import (
	"strings"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/store"
)

// history is the commit list: the graph, the message with its refs, the author,
// the date and the hash. Only the rows in view are built, and the next page of
// history loads as the list nears its end.
func (a *App) history(r *store.Repo) ui.Element {
	v := a.viewOf(r)
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		entries := r.Entries.Get()
		info := r.Info.Get()
		query := v.query.Get()
		loading := r.Loading.Get()
		now := a.layout.Now()
		// A row that does not match the search is faded, not hidden, so the graph
		// stays whole.
		faded := func(e store.Entry) bool { return query != "" && !matches(query, e.Hash, e.Subject, e.Author) }
		cell := func(e store.Entry, el ui.Element) ui.Element {
			d := ui.Div().Row().Align(ui.Center).Grow(1).MinWidth(ui.Px(0)).Gap(8).Child(el)
			if faded(e) {
				d.Opacity(0.35)
			}
			return d
		}
		cols := []ui.DataColumn[store.Entry]{
			// The graph's cells have no padding, so each row's slice meets the next.
			{Key: "graph", Header: "Graph", Width: graphWidth(r.Lanes.Get()), MinWidth: 40, Resizable: true,
				CellStyle: ui.Style{Clear: ui.ClearPadding},
				Text: func(e store.Entry) string {
					if e.Row.Merge {
						return "Merge"
					}
					return ""
				},
				Cell: func(e store.Entry) ui.Element {
					g := graphCell(t, e, e.Hash == info.HeadHash)
					if faded(e) {
						return ui.Div().Size(ui.Full, ui.Full).Opacity(0.35).Child(g)
					}
					return g
				}},
			{Key: "message", Header: "Message", Width: 420, MinWidth: 160, Grow: 1, Resizable: true,
				Text: func(e store.Entry) string {
					var parts []string
					for _, l := range labels(e.Refs, info.Head) {
						parts = append(parts, l.text())
					}
					return strings.Join(append(parts, e.Subject), ", ")
				},
				Cell: func(e store.Entry) ui.Element {
					row := ui.Div().Row().Align(ui.Center).Gap(8).Grow(1).MinWidth(ui.Px(0))
					if ls := labels(e.Refs, info.Head); len(ls) > 0 {
						row.Child(pills(t, ls, laneColor(t, e.Row.Lane)))
					}
					subject := ui.Text(e.Subject).Truncate()
					if e.Row.Merge {
						subject.TextColor(t.TextMuted)
					}
					return cell(e, row.Child(subject))
				}},
			{Key: "author", Header: "Author", Width: 170, MinWidth: 80, Resizable: true,
				Text: func(e store.Entry) string { return e.Author },
				Cell: func(e store.Entry) ui.Element {
					return cell(e, ui.Div().Row().Align(ui.Center).Gap(8).MinWidth(ui.Px(0)).
						Child(ui.Avatar(e.Author).Size(18).AccessHidden()).
						Child(ui.Text(e.Author).TextColor(t.TextMuted).Truncate()))
				}},
			{Key: "date", Header: "Date", Width: 120, MinWidth: 70, Resizable: true,
				Text: func(e store.Entry) string { return ui.FormatRelative(e.When, now) },
				Cell: func(e store.Entry) ui.Element {
					return cell(e, ui.Div().TextColor(t.TextMuted).Child(ui.TimeAgo(e.When)))
				}},
			{Key: "hash", Header: "Commit", Width: 86, MinWidth: 60, Resizable: true,
				Text: func(e store.Entry) string { return e.Short() },
				Cell: func(e store.Entry) ui.Element {
					return cell(e, ui.Text(e.Short()).Mono().FontSize(12).TextColor(t.TextSubtle))
				}},
		}
		return ui.DataGrid(entries, func(e store.Entry) string { return e.Hash }, cols...).
			Key("history").Label("Commits").
			Selection(ui.SelectionSingle, r.Selected).
			ColumnWidths(v.widths).
			Handle(v.grid).
			LoadMore(r.More.Get(), r.LoadMore).LoadingMore(loading && len(entries) > 0).
			Loading(loading && len(entries) == 0).
			ContextMenu(func(e store.Entry, _ string) []ui.MenuItem { return a.commitMenu(ctx, r, e) }).
			Empty(ui.Empty("No commits").Icon(icons.GitBranch).Description("This repository has no history yet.")).
			Styles(gridStyle(t)).
			Style(ui.Style{Grow: 1})
	})
}

// gridStyle is the look of the app's tables: no frame and no lines between
// rows, a quiet header and a soft fill for the hovered and selected rows. The
// graph needs rows that meet, so the rows have no border.
func gridStyle(t *ui.Theme) ui.DataGridStyle {
	return ui.DataGridStyle{
		Grid: ui.Style{Clear: ui.ClearBorder, Background: t.Background},
		Header: ui.Style{Background: t.Background, FontSize: 11.5, FontWeight: ui.FontMedium,
			TextColor: t.TextSubtle},
		HeaderCellHover: ui.Style{Background: t.SurfaceHover},
		Row:             ui.Style{Clear: ui.ClearBorder},
		RowHover:        ui.Style{Background: t.SurfaceHover},
		RowSelected:     ui.Style{Background: t.AccentSoft},
	}
}

// commitMenu is the context menu of a commit in the history.
func (a *App) commitMenu(ctx *ui.Context, r *store.Repo, e store.Entry) []ui.MenuItem {
	items := []ui.MenuItem{
		{Label: "Copy commit hash", Icon: icons.Copy, OnClick: func() {
			ctx.SetClipboard(e.Hash)
			ctx.Toast(ui.ToastSuccess, "Copied "+e.Short())
		}},
		{Label: "Copy message", OnClick: func() { ctx.SetClipboard(e.Subject) }},
	}
	if len(e.Parents) > 0 {
		items = append(items, ui.MenuSeparator(), ui.MenuItem{Label: "Go to parent", Icon: icons.ArrowLeft,
			OnClick: func() { a.revealCommit(ctx, r, e.Parents[0], "The parent") }})
	}
	return items
}

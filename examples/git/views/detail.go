package views

import (
	"context"
	"time"

	"github.com/tyler-builds/jamb/access"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/repo"
	"github.com/tyler-builds/jamb/examples/git/store"
)

// detail is the pane beside the history: the selected commit's message, who
// made it and when, its parents and refs, and the files it changed. The details
// are read with ui.Query, keyed by the repository and the commit, so moving back
// to a commit seen a moment ago shows it at once.
func (a *App) detail(r *store.Repo) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		e, ok := r.Selection()
		pane := ui.Div().Column().Size(ui.Full, ui.Full).Background(t.Panel)
		if !ok {
			return pane.Child(detailBar(ctx, nil)).
				Child(ui.Div().Column().Center().Grow(1).Child(ui.Empty("No commit selected").Icon(icons.Info).
					Description("Select a commit to see its changes.")))
		}
		info := r.Info.Get()
		res := ui.Query(ctx, "commit:"+info.Root+":"+e.Hash, func(c context.Context) (repo.Details, error) {
			return repo.Show(c, info.Root, e.Hash)
		})
		d := res.Data
		body := ui.Div().Column().Gap(18).Padding(20).
			Child(ui.Div().Column().Gap(8).
				Child(ui.Text(e.Subject).FontSize(15).FontWeight(ui.FontSemibold).Selectable()).
				Child(ui.If(d.Body != "", ui.Text(d.Body).TextColor(t.TextMuted).Selectable()))).
			Child(a.people(t, e, d)).
			Child(a.facts(ctx, r, e, info))
		switch {
		case res.Err != nil:
			body.Child(ui.Alert("Could not read the commit").Danger().Description(res.Err.Error()))
		case res.Loading:
			body.Child(ui.Div().Column().Gap(8).
				Child(ui.Skeleton().Height(ui.Px(14)).Width(ui.Percent(60))).
				Child(ui.Skeleton().Height(ui.Px(14)).Width(ui.Percent(80))).
				Child(ui.Skeleton().Height(ui.Px(14)).Width(ui.Percent(45))))
		default:
			body.Child(a.files(t, r, d))
		}
		return pane.Child(detailBar(ctx, &e)).
			Child(ui.Div().Column().Grow(1).MinHeight(ui.Px(0)).ScrollY().Child(body))
	})
}

// detailBar is the bar at the top of the details: the commit's hash and a
// button that copies it.
func detailBar(ctx *ui.Context, e *store.Entry) ui.Element {
	t := ctx.Theme()
	bar := ui.Div().Row().Align(ui.Center).Gap(8).Height(ui.Px(barHeight)).Shrink(0).
		PaddingEdges(ui.Edges{Left: 20, Right: 12}).BorderEdges(ui.Edges{Bottom: 1}, t.Border).
		Child(ui.Text("Commit").FontSize(12.5).TextColor(t.TextSubtle))
	if e == nil {
		return bar
	}
	return bar.
		Child(ui.Text(e.Hash[:min(12, len(e.Hash))]).Mono().FontSize(12.5).Selectable()).
		Child(ui.Div().Grow(1)).
		Child(ui.CopyButton(e.Hash).Key("copy-hash").Ghost().Small().Tooltip("Copy hash (Ctrl+Shift+C)"))
}

// people shows the author and, when it is someone else or much later, the committer.
func (a *App) people(t *ui.Theme, e store.Entry, d repo.Details) ui.Element {
	person := func(name, email, verb string, when time.Time) ui.Element {
		return ui.Div().Row().Align(ui.Center).Gap(10).
			Child(ui.Avatar(name).Size(30).AccessHidden()).
			Child(ui.Div().Column().Gap(2).MinWidth(ui.Px(0)).
				Child(ui.Div().Row().Gap(6).Align(ui.Center).
					Child(ui.Text(name).FontWeight(ui.FontMedium).Truncate()).
					Child(ui.Text(email).FontSize(12).TextColor(t.TextSubtle).Truncate())).
				Child(ui.Div().Row().Gap(4).FontSize(12).TextColor(t.TextMuted).
					Child(ui.Text(verb)).Child(ui.TimeAgo(when))))
	}
	col := ui.Div().Column().Gap(12).Child(person(e.Author, e.Email, "authored", e.When))
	if d.Hash == e.Hash && d.Committer != "" && (d.Committer != e.Author || d.Committed.Sub(e.When).Abs().Hours() > 1) {
		col.Child(person(d.Committer, d.CommitterEmail, "committed", d.Committed))
	}
	return col
}

// facts is the hash, the parents and the refs, as a list of terms.
func (a *App) facts(ctx *ui.Context, r *store.Repo, e store.Entry, info repo.Info) ui.Element {
	t := ctx.Theme()
	parents := ui.Div().Row().Wrap().Gap(6)
	if len(e.Parents) == 0 {
		parents.Child(ui.Text("None, the first commit").TextColor(t.TextMuted))
	}
	// Each parent is a chip, as the refs are, that selects its commit.
	for _, p := range e.Parents {
		short := p[:min(7, len(p))]
		parents.Child(ui.Div().Row().Align(ui.Center).Height(ui.Px(22)).PaddingX(7).Radius(5).
			Background(t.SurfaceActive).Cursor(ui.CursorPointer).TabIndex(0).
			Role(access.RoleButton).Label("Go to parent " + short).Tooltip("Go to " + short).
			Hover(ui.Style{Background: t.SurfaceHover.Mix(t.Text, 0.08)}).
			FocusVisible(ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: t.FocusRing}}}).
			Child(ui.Text(short).Mono().FontSize(12)).
			OnClick(func() {
				if !a.selectCommit(r, p) {
					ctx.Toast(ui.ToastInfo, "That commit is further back than the history loaded so far.")
				}
			}))
	}
	list := ui.DescriptionList().
		Item("Parents", parents)
	if ls := labels(e.Refs, info.Head); len(ls) > 0 {
		refs := ui.Div().Row().Wrap().Gap(4)
		for _, l := range ls {
			refs.Child(pill(t, l, laneColor(t, e.Row.Lane)))
		}
		list.Item("Refs", refs)
	}
	return list
}

// files lists the files the commit changed. Choosing one, with the pointer or
// the arrow keys, shows its diff in place of the history.
func (a *App) files(t *ui.Theme, r *store.Repo, d repo.Details) ui.Element {
	v := a.viewOf(r)
	added, deleted := 0, 0
	for _, f := range d.Files {
		added += max(f.Added, 0)
		deleted += max(f.Deleted, 0)
	}
	title := itoa(len(d.Files)) + " files changed"
	if len(d.Files) == 1 {
		title = "1 file changed"
	}
	if len(d.Parents) > 1 {
		title += " from the first parent"
	}
	head := ui.Div().Row().Align(ui.Center).Gap(8).
		Child(ui.Text(title).FontWeight(ui.FontMedium)).
		Child(ui.Div().Grow(1)).
		Child(ui.Text("+" + itoa(added)).Mono().FontSize(12).TextColor(t.Success)).
		Child(ui.Text("−" + itoa(deleted)).Mono().FontSize(12).TextColor(t.Danger))
	byPath := map[string]repo.FileChange{}
	for _, f := range d.Files {
		byPath[f.Path] = f
	}
	list := ui.ListBox(d.Files, func(f repo.FileChange) string { return f.Path }, v.fileSel,
		func(f repo.FileChange, _ ui.PartState) ui.Element {
			dir, base := splitPath(f.Path)
			counts := ui.Div().Row().Gap(6).Shrink(0)
			if f.Added < 0 {
				counts.Child(ui.Text("binary").FontSize(11.5).TextColor(t.TextSubtle))
			} else {
				counts.Child(ui.Text("+" + itoa(f.Added)).Mono().FontSize(11.5).TextColor(t.Success)).
					Child(ui.Text("−" + itoa(f.Deleted)).Mono().FontSize(11.5).TextColor(t.Danger))
			}
			return ui.Div().Row().Align(ui.Center).Gap(8).Grow(1).MinWidth(ui.Px(0)).
				Child(statusMark(t, f.Status)).
				Child(ui.Div().Row().Grow(1).MinWidth(ui.Px(0)).
					Child(ui.Text(dir).TextColor(t.TextSubtle).Truncate()).
					Child(ui.Div().Shrink(0).Child(ui.Text(base)))).
				Child(counts)
		}).
		Key("files").Label("Changed files").
		ItemName(func(f repo.FileChange) string {
			name := f.Path
			if f.OldPath != "" {
				name = f.OldPath + " → " + f.Path
			}
			return statusWord(f.Status) + " " + name
		}).
		OnChange(func(keys []string) {
			if len(keys) == 1 {
				v.diff.Set(&diffTarget{commit: d.Commit, file: byPath[keys[0]]})
			}
		})
	return ui.Div().Key("detail-files").Column().Gap(8).Child(head).Child(list)
}

// statusMark is a file's status as a small colored letter.
func statusMark(t *ui.Theme, s repo.FileStatus) ui.Element {
	c := t.Warning
	switch s {
	case repo.Added:
		c = t.Success
	case repo.Deleted:
		c = t.Danger
	case repo.Renamed, repo.Copied:
		c = t.Link
	}
	return ui.Div().Center().Size(ui.Px(18), ui.Px(18)).Radius(4).Shrink(0).Background(c.Alpha(0.14)).
		Child(ui.Text(string(rune(s))).FontSize(11).FontWeight(ui.FontSemibold).TextColor(c)).
		Tooltip(statusWord(s))
}

func statusWord(s repo.FileStatus) string {
	switch s {
	case repo.Added:
		return "Added"
	case repo.Deleted:
		return "Deleted"
	case repo.Renamed:
		return "Renamed"
	case repo.Copied:
		return "Copied"
	case repo.TypeChg:
		return "Type changed"
	}
	return "Modified"
}

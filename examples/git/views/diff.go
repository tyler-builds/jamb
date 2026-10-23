package views

import (
	"context"

	"github.com/tyler-builds/jamb/codeedit"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/repo"
	"github.com/tyler-builds/jamb/examples/git/store"
)

// diffTarget is a file of a commit whose diff is shown.
type diffTarget struct {
	commit repo.Commit
	file   repo.FileChange
}

// diffView shows one file's diff in place of the history, read-only, with the
// diff colors of the code editor. Escape goes back to the history.
func (a *App) diffView(r *store.Repo, target *diffTarget) ui.Element {
	v := a.viewOf(r)
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		key := target.commit.Hash + ":" + target.file.Path
		root := r.Info.Peek().Root
		res := ui.Query(ctx, "diff:"+root+":"+key, func(c context.Context) (*codeedit.Document, error) {
			text, err := repo.Diff(c, root, target.commit, target.file)
			if err != nil {
				return nil, err
			}
			if text == "" {
				text = "No textual changes."
			}
			return codeedit.NewDocument(text), nil
		})
		dir, base := splitPath(target.file.Path)
		head := ui.Div().Row().Align(ui.Center).Gap(10).Height(ui.Px(40)).Shrink(0).PaddingX(12).
			BorderEdges(ui.Edges{Bottom: 1}, t.Border).
			Child(ui.Button("History").Key("diff-back").Icon(icons.ArrowLeft).Ghost().Small().Tooltip("Back to history (Escape)").
				OnClick(func() { v.closeDiff() })).
			Child(ui.Div().Width(ui.Px(1)).Height(ui.Px(16)).Background(t.Border)).
			Child(statusMark(t, target.file.Status)).
			Child(ui.Div().Row().MinWidth(ui.Px(0)).Grow(1).
				Child(ui.Text(dir).TextColor(t.TextSubtle).Truncate()).
				Child(ui.Div().Shrink(0).Child(ui.Text(base).Medium()))).
			Child(ui.Text(target.commit.Short()).Mono().FontSize(12).TextColor(t.TextSubtle))
		var body ui.Element
		switch {
		case res.Err != nil:
			body = ui.Div().Column().Center().Grow(1).
				Child(ui.Empty("Could not read the diff").Icon(icons.Alert).Description(res.Err.Error()))
		case res.Data == nil:
			body = ui.Div().Key("diff-loading").Column().Center().Grow(1).Child(ui.ProgressRing().Label("Loading the diff"))
		default:
			body = codeedit.Editor(res.Data).Key(key).Label("Diff of "+target.file.Path).
				Language("diff").ReadOnly(true).LineNumbers(false).Size(ui.Full, ui.Full).Grow(1)
		}
		return ui.Div().Key("diff-view").Column().Grow(1).MinHeight(ui.Px(0)).KeyContext("DiffView").
			Child(head).
			Child(ui.Div().Column().Grow(1).MinHeight(ui.Px(0)).Child(body))
	})
}

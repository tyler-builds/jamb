package views

import (
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
		doc := ui.Local(ctx, "doc", (*codeedit.Document)(nil))
		failed := ui.Local(ctx, "err", "")
		key := target.commit.Hash + ":" + target.file.Path
		loaded := ui.Local(ctx, "loaded", "")
		if loaded.Peek() != key {
			loaded.Set(key)
			doc.Set(nil)
			root := r.Info.Peek().Root
			go func() {
				text, err := repo.Diff(root, target.commit, target.file)
				ctx.Post(func() {
					if loaded.Peek() != key {
						return
					}
					if err != nil {
						failed.Set(err.Error())
						return
					}
					if text == "" {
						text = "No textual changes."
					}
					doc.Set(codeedit.NewDocument(text))
				})
			}()
		}
		dir, base := splitPath(target.file.Path)
		head := ui.Div().Row().Align(ui.Center).Gap(10).Height(ui.Px(40)).Shrink(0).PaddingX(12).
			BorderEdges(ui.Edges{Bottom: 1}, t.Border).
			Child(ui.Button("History").Key("diff-back").Icon(icons.ArrowLeft).Ghost().Small().Tooltip("Back to history (Escape)").
				OnClick(func() { v.diff.Set(nil) })).
			Child(ui.Div().Width(ui.Px(1)).Height(ui.Px(16)).Background(t.Border)).
			Child(statusMark(t, target.file.Status)).
			Child(ui.Div().Row().MinWidth(ui.Px(0)).Grow(1).
				Child(ui.Text(dir).TextColor(t.TextSubtle).Truncate()).
				Child(ui.Div().Shrink(0).Child(ui.Text(base).Medium()))).
			Child(ui.Text(target.commit.Short()).Mono().FontSize(12).TextColor(t.TextSubtle))
		var body ui.Element
		switch {
		case failed.Get() != "":
			body = ui.Div().Column().Center().Grow(1).
				Child(ui.Empty("Could not read the diff").Icon(icons.Alert).Description(failed.Get()))
		case doc.Get() == nil:
			body = ui.Div().Key("diff-loading").Column().Center().Grow(1).Child(ui.ProgressRing().Label("Loading the diff"))
		default:
			body = codeedit.Editor(doc.Get()).Key(key).Label("Diff of "+target.file.Path).
				Language("diff").ReadOnly(true).LineNumbers(false).Size(ui.Full, ui.Full).Grow(1)
		}
		return ui.Div().Key("diff-view").Column().Grow(1).MinHeight(ui.Px(0)).KeyContext("DiffView").
			Child(head).
			Child(ui.Div().Column().Grow(1).MinHeight(ui.Px(0)).Child(body))
	})
}

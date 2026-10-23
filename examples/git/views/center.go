package views

import (
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/repo"
	"github.com/tyler-builds/jamb/examples/git/store"
)

// main is the area right of the sidebar. Every repository's view stays built
// and is hidden while another is shown, so its scroll position, selection and
// search survive switching between repositories.
func (a *App) main() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		repos := a.s.Repos.Get()
		cur := a.s.Current.Get()
		if len(repos) == 0 {
			return a.welcome(ctx)
		}
		box := ui.Div().Column().Size(ui.Full, ui.Full)
		for _, r := range repos {
			box.Child(ui.Div().Key(r.Path).Column().Grow(1).MinHeight(ui.Px(0)).Display(r.Path == cur).
				Child(a.repoView(r)))
		}
		return box
	})
}

// welcome is the empty state of a window with no repositories.
func (a *App) welcome(ctx *ui.Context) ui.Element {
	return ui.Div().Column().Center().Size(ui.Full, ui.Full).
		Child(ui.Empty("No repositories yet").Icon(icons.GitBranch).
			Description("Add a folder that holds a Git repository to see its history, branches and tags.").
			Action(ui.Button("Add repository").Icon(icons.Plus).OnClick(func() { a.addRepo(ctx) })))
}

// repoView is one repository: its page switcher, the history or tags beside the
// commit's details, and its error, if reading it failed.
func (a *App) repoView(r *store.Repo) ui.Element {
	v := a.viewOf(r)
	detailSize := ui.NewSignal(ui.Px(float32(a.layout.Detail.Peek())))
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		page := v.page.Get()
		diff := v.diff.Get()
		var body ui.Element
		switch {
		case r.Err.Get() != "" && len(r.Entries.Get()) == 0:
			body = ui.Div().Column().Center().Grow(1).
				Child(ui.Empty("Could not read this repository").Icon(icons.Alert).Description(r.Err.Get()).
					Action(ui.Button("Try again").Secondary().OnClick(r.Reload)))
		case page == PageTags:
			body = a.tags(r)
		default:
			// The history stays built under a diff, so closing the diff keeps its place.
			body = ui.Div().Column().Grow(1).MinHeight(ui.Px(0)).
				Child(ui.Div().Column().Grow(1).MinHeight(ui.Px(0)).Display(diff == nil).Child(a.history(r))).
				Child(ui.If(diff != nil, a.diffView(r, diff)))
		}
		center := ui.Div().Column().Size(ui.Full, ui.Full).
			Child(a.toolbar(r, page)).
			Child(body)
		return ui.Split(
			ui.Pane(center).Label("Commits").Min(ui.Px(360)),
			ui.Pane(a.detail(r)).Label("Commit details").SizeSignal(detailSize).
				Min(ui.Px(280)).Max(ui.Percent(55)).Collapsible(),
		).Key("detail-split").Label("Resize details").Styles(splitStyle(t)).OnChange(func() {
			a.layout.Detail.Set(float64(detailSize.Peek().Value))
			a.changed()
		})
	})
}

// toolbar switches between the history and the tags, and counts them.
func (a *App) toolbar(r *store.Repo, page Page) ui.Element {
	v := a.viewOf(r)
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		tags := 0
		for _, ref := range r.Refs.Get() {
			if ref.Kind == repo.Tag {
				tags++
			}
		}
		history := "History"
		if n := len(r.Entries.Get()); n > 0 {
			history += "  " + itoa(n)
			if r.More.Get() {
				history += "+"
			}
		}
		return ui.Div().Row().Align(ui.Center).Height(ui.Px(barHeight)).Shrink(0).PaddingX(12).Gap(8).
			BorderEdges(ui.Edges{Bottom: 1}, t.Border).
			Child(ui.Tabs(v.page,
				ui.Tab[Page]{Value: PageHistory, Label: history, Icon: icons.Activity},
				ui.Tab[Page]{Value: PageTags, Label: "Tags  " + itoa(tags), Icon: icons.Tag},
			).Key("page").Label("Page").Variant(ui.TabsSegmented).Styles(segmented(t)).
				OnChange(func(Page) { v.diff.Set(nil) })).
			Child(ui.Div().Grow(1)).
			Child(ui.Button("HEAD").Icon(icons.Home).Ghost().Small().Tooltip("Go to HEAD (Ctrl+H)").OnClick(a.goHead))
	})
}

// segmented is a quieter segmented control than the default: a raised segment
// on a sunken track, not an accent fill.
func segmented(t *ui.Theme) ui.TabsStyle {
	return ui.TabsStyle{
		List:        ui.Style{Background: t.Panel, Radius: 7, Padding: ui.All(2), BorderWidth: 1, BorderColor: t.Border},
		Tab:         ui.Style{Height: ui.Px(26), Padding: ui.XY(10, 0), Radius: 5, FontSize: 12.5, TextColor: t.TextMuted},
		TabSelected: ui.Style{Background: t.SurfaceActive, TextColor: t.Text},
		TabHover:    ui.Style{TextColor: t.Text},
	}
}

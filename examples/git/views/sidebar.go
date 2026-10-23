package views

import (
	"strings"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/repo"
	"github.com/tyler-builds/jamb/examples/git/store"
)

// sidebar lists the repositories, and the branches, remote branches and tags
// of the one shown.
func (a *App) sidebar() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		col := ui.Div().Column().Size(ui.Full, ui.Full).Background(t.Panel).
			Child(a.repoList()).
			Child(ui.Div().Height(ui.Px(1)).Shrink(0).Background(t.Border).MarginX(14))
		if r := a.s.CurrentRepo(); r != nil {
			col.Child(a.refTree(r))
		}
		return col
	})
}

// barHeight is the height of the bars at the top of the sidebar, the history and
// the details, so their bottom borders make one line across the window.
const barHeight = 44

// heading is the sidebar's top bar: a small section title, with an optional
// button at its end.
func heading(t *ui.Theme, text string, end ui.Element) ui.Element {
	row := ui.Div().Row().Align(ui.Center).Height(ui.Px(barHeight)).Shrink(0).
		PaddingEdges(ui.Edges{Left: 14, Right: 8}).BorderEdges(ui.Edges{Bottom: 1}, t.Border).
		Child(ui.Div().Grow(1).Role(access.RoleHeading).Label(text).
			Child(ui.Text(text).FontSize(11.5).FontWeight(ui.FontMedium).TextColor(t.TextSubtle)))
	if end != nil {
		row.Child(end)
	}
	return row
}

// repoList is the list of repositories. Choosing one shows it, and its context
// menu refreshes it, copies its path or removes it from the list.
func (a *App) repoList() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		repos := a.s.Repos.Get()
		list := ui.ListBox(repos, func(r *store.Repo) string { return r.Path }, a.repoSel,
			func(r *store.Repo, _ ui.PartState) ui.Element { return a.repoRow(t, r) }).
			Key("repos").Label("Repositories").
			ItemName(func(r *store.Repo) string { return r.Info.Peek().Name }).
			Styles(ui.ListBoxStyle{Item: ui.Style{MinHeight: ui.Px(40)}}).
			OnChange(func(keys []string) {
				if len(keys) == 1 {
					a.s.Current.Set(keys[0])
					a.changed()
				}
			}).
			ContextMenu(func(r *store.Repo) []ui.MenuItem {
				return []ui.MenuItem{
					{Label: "Refresh", Icon: icons.Refresh, OnClick: r.Reload},
					{Label: "Copy path", Icon: icons.Copy, OnClick: func() { ctx.SetClipboard(r.Info.Peek().Root) }},
					ui.MenuSeparator(),
					{Label: "Remove from list", Icon: icons.X, OnClick: func() {
						a.s.Remove(r.Path)
						delete(a.views, r.Path)
						a.syncRepos()
						a.changed()
					}},
				}
			}).
			Style(ui.Style{Padding: ui.All(6), MaxHeight: ui.Px(260)})
		return ui.Div().Column().Shrink(0).
			Child(heading(t, "Repositories", ui.IconButton(icons.Plus).Ghost().Small().
				Label("Add repository").Tooltip("Add repository (Ctrl+O)").
				OnClick(func() { a.addRepo(ctx) }))).
			Child(list)
	})
}

// repoRow is one repository in the list: a colored square with its initial, its
// name and branch, and whether it is loading or failed to load.
func (a *App) repoRow(t *ui.Theme, r *store.Repo) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		info := r.Info.Get()
		branch := info.Head
		if branch == "" && len(info.HeadHash) >= 7 {
			branch = info.HeadHash[:7]
		}
		square := ui.Div().Center().Size(ui.Px(24), ui.Px(24)).Radius(6).Shrink(0).
			Background(laneColor(t, int(hashName(info.Name)))).
			Child(ui.Text(strings.ToUpper(info.Name[:min(1, len(info.Name))])).FontSize(11).
				Weight(ui.FontSemibold).TextColor(ui.Hex("#ffffff")))
		status := ui.Element(nil)
		switch {
		case r.Err.Get() != "":
			status = ui.Icon(icons.Alert).Size(13).Color(t.Danger)
		case r.Loading.Get():
			status = ui.ProgressRing().Size(12).Label("Loading")
		}
		return ui.Div().Row().Align(ui.Center).Gap(10).Grow(1).MinWidth(ui.Px(0)).Tooltip(info.Root).
			Child(square).
			Child(ui.Div().Column().Grow(1).MinWidth(ui.Px(0)).
				Child(ui.Text(info.Name).FontWeight(ui.FontMedium).Truncate()).
				Child(ui.If(branch != "", ui.Text(branch).FontSize(11.5).TextColor(t.TextSubtle).Truncate()))).
			Child(ui.If(status != nil, status))
	})
}

// hashName is a small, stable number for a name, to pick its color.
func hashName(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	return h % 8
}

// Tree keys: a prefix for the kind, then the ref's name, so a branch and a tag
// with the same name stay apart.
const (
	keyBranches = "group:branches"
	keyRemotes  = "group:remotes"
	keyTags     = "group:tags"
)

func refKey(r repo.Ref) string {
	switch r.Kind {
	case repo.Remote:
		return "remote:" + r.Name
	case repo.Tag:
		return "tag:" + r.Name
	}
	return "branch:" + r.Name
}

// refTree lists the refs of r by kind, remote branches under their remote. Choosing
// one selects its commit in the history. A branch's description names its upstream,
// and its row ends with how far it is ahead and behind; a group's row ends with its
// count.
func (a *App) refTree(r *store.Repo) ui.Element {
	v := a.viewOf(r)
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		refs := r.Refs.Get()
		info := r.Info.Get()
		byKey := map[string]repo.Ref{}
		var branches, tags []ui.TreeNode
		remotes := map[string][]ui.TreeNode{}
		var remoteNames []string
		for _, ref := range refs {
			k := refKey(ref)
			byKey[k] = ref
			switch ref.Kind {
			case repo.Branch:
				desc := ref.Upstream
				if ref.Name == info.Head {
					desc = strings.TrimSpace("current " + desc)
				}
				branches = append(branches, ui.TreeNode{Key: k, Label: ref.Name, Icon: icons.GitBranch, Description: desc})
			case repo.Remote:
				remote, name, _ := strings.Cut(ref.Name, "/")
				if _, ok := remotes[remote]; !ok {
					remoteNames = append(remoteNames, remote)
				}
				remotes[remote] = append(remotes[remote], ui.TreeNode{Key: k, Label: name, Icon: icons.GitBranch})
			case repo.Tag:
				tags = append(tags, ui.TreeNode{Key: k, Label: ref.Name, Icon: icons.Tag})
			}
		}
		var remoteNodes []ui.TreeNode
		for _, name := range remoteNames {
			kids := remotes[name]
			remoteNodes = append(remoteNodes, ui.TreeNode{Key: "remote-group:" + name, Label: name, Icon: icons.Link,
				Children: func() []ui.TreeNode { return kids }})
		}
		counts := map[string]int{}
		group := func(key, label string, ic *icons.Icon, kids []ui.TreeNode) ui.TreeNode {
			counts[key] = len(kids)
			return ui.TreeNode{Key: key, Label: label, Icon: ic, Children: func() []ui.TreeNode { return kids }}
		}
		nodes := []ui.TreeNode{
			group(keyBranches, "Branches", icons.GitBranch, branches),
			group(keyRemotes, "Remotes", icons.Link, remoteNodes),
			group(keyTags, "Tags", icons.Tag, tags),
		}
		return ui.Div().Column().Grow(1).MinHeight(ui.Px(0)).PaddingEdges(ui.Edges{Top: 6, Bottom: 6}).
			Child(ui.Tree(nodes, v.refSel).Key("refs").Label("Branches and tags").Expanded(v.refOpen).
				Styles(ui.TreeStyle{Row: ui.Style{Height: ui.Px(28), Radius: 6}, RowSelected: ui.Style{Background: t.SurfaceActive}}).
				RenderTrailing(func(n ui.TreeNode, _ ui.PartState) ui.Element {
					return refTrailing(t, n, byKey, counts, info)
				}).
				ContextMenu(func(key string) []ui.MenuItem {
					ref, ok := byKey[key]
					if !ok {
						return nil
					}
					return []ui.MenuItem{
						{Label: "Show in history", Icon: icons.Activity, OnClick: func() { a.revealRef(ctx, r, ref) }},
						{Label: "Copy name", Icon: icons.Copy, OnClick: func() { ctx.SetClipboard(ref.Name) }},
						{Label: "Copy commit hash", OnClick: func() { ctx.SetClipboard(ref.Target) }},
					}
				}).
				OnChange(func(key string) {
					if ref, ok := byKey[key]; ok {
						a.revealRef(ctx, r, ref)
					}
				}).
				Style(ui.Style{Grow: 1, Padding: ui.XY(6, 0)}))
	})
}

// revealRef selects the commit a ref points at, or says it is not loaded yet.
func (a *App) revealRef(ctx *ui.Context, r *store.Repo, ref repo.Ref) {
	if !a.selectCommit(r, ref.Target) {
		ctx.Toast(ui.ToastInfo, ref.Name+" is further back than the history loaded so far.")
	}
}

// refTrailing ends a tree row: a group's count, or a branch's mark when HEAD is on
// it and how far it is ahead of and behind its upstream.
func refTrailing(t *ui.Theme, n ui.TreeNode, byKey map[string]repo.Ref, counts map[string]int, info repo.Info) ui.Element {
	row := ui.Div().Row().Align(ui.Center).Gap(6).FontSize(11.5).TextColor(t.TextSubtle)
	if c, ok := counts[n.Key]; ok {
		return row.Child(ui.Text(itoa(c)))
	}
	ref, ok := byKey[n.Key]
	if !ok {
		return nil
	}
	if ref.Kind == repo.Branch && ref.Name == info.Head {
		row.Child(ui.Icon(icons.Check).Size(12).Color(t.Accent))
	}
	if ref.Ahead > 0 {
		row.Child(ui.Text("↑" + itoa(ref.Ahead)))
	}
	if ref.Behind > 0 {
		row.Child(ui.Text("↓" + itoa(ref.Behind)))
	}
	return row
}

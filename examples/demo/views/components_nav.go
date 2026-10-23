package views

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// navigationSection shows the tabs, the collapsible sections and the tree view. Each
// sample keeps its selection in the section, so only the section re-renders.
func navigationSection() ui.Element {
	// The trees are made once: the section re-renders on each change, and the
	// nodes of the large tree are not made again then.
	files, folders := projectFiles(), bigFolders()
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		plain := ui.Local(ctx, "nav.plain", "alpha")
		disabled := ui.Local(ctx, "nav.disabled", "overview")
		icon := ui.Local(ctx, "nav.icon", "home")
		open := ui.Local(ctx, "nav.open", false)
		email, sms, push := ui.Local(ctx, "nav.email", true), ui.Local(ctx, "nav.sms", false), ui.Local(ctx, "nav.push", false)
		node := ui.Local(ctx, "nav.node", "src")
		big := ui.Local(ctx, "nav.big", "folder-0000")
		note := ui.Local(ctx, "nav.note", "")
		// The project tree is opened and closed from its buttons, and reports its open keys and
		// its last open or close in the status line.
		openKeys := ui.Local(ctx, "nav.treeOpen", []string(nil))
		handle := ui.Local(ctx, "nav.treeHandle", ui.NewTreeHandle()).Peek()
		lastEvent := ui.Local(ctx, "nav.treeEvent", "none")

		samples := []ui.Element{
			sample(t, "Tabs, panels kept", ui.Div().Width(ui.Px(420)).Child(ui.Tabs(plain,
				ui.Tab[string]{Value: "alpha", Label: "Alpha", Panel: func() ui.Element {
					// KeepMounted keeps this panel, so the text in the field stays when another tab is selected.
					return ui.Div().Column().Gap(8).Children(
						panelText(t, "Alpha panel"),
						ui.TextInput(note).Label("Note").Placeholder("Type a note").Width(ui.Full),
						ui.Text("Text typed here is kept when another tab is selected.").FontSize(11).TextColor(t.TextSubtle),
					)
				}},
				ui.Tab[string]{Value: "beta", Label: "Beta", Panel: func() ui.Element { return panelText(t, "Beta panel") }},
				ui.Tab[string]{Value: "gamma", Label: "Gamma", Panel: func() ui.Element { return panelText(t, "Gamma panel") }},
			).Key("nav.plainSections").Label("Plain sections").KeepMounted())),
			sample(t, "Tabs, one disabled", ui.Div().Width(ui.Px(420)).Child(ui.Tabs(disabled,
				ui.Tab[string]{Value: "overview", Label: "Overview", Panel: func() ui.Element { return panelText(t, "Overview panel") }},
				ui.Tab[string]{Value: "usage", Label: "Usage", Disabled: true, Panel: func() ui.Element { return panelText(t, "Usage panel") }},
				ui.Tab[string]{Value: "billing", Label: "Billing", Panel: func() ui.Element { return panelText(t, "Billing panel") }},
			).Key("nav.disabledSections").Label("Sections, one disabled"))),
			sample(t, "Tabs, with icons", ui.Div().Width(ui.Px(420)).Child(ui.Tabs(icon,
				ui.Tab[string]{Value: "home", Label: "Home", Icon: icons.Home, Panel: func() ui.Element { return panelText(t, "Home panel") }},
				ui.Tab[string]{Value: "files", Label: "Files", Icon: icons.Folder, Panel: func() ui.Element { return panelText(t, "Files panel") }},
				ui.Tab[string]{Value: "settings", Label: "Settings", Icon: icons.Settings, Panel: func() ui.Element { return panelText(t, "Settings panel") }},
			).Key("nav.iconSections").Label("Sections with icons"))),
			sample(t, "Collapsible", ui.Div().Width(ui.Px(420)).Child(ui.Collapsible("Advanced options", open,
				panelText(t, "The options are shown while the section is open.")).Key("nav.advanced"))),
			sample(t, "Accordion, one open at a time", ui.Div().Width(ui.Px(420)).Child(ui.Accordion(
				ui.Collapsible("Email alerts", email, panelText(t, "Send an email for each change.")).Key("nav.emailSection"),
				ui.Collapsible("Text alerts", sms, panelText(t, "Send a text message for each change.")).Key("nav.smsSection"),
				ui.Collapsible("Push alerts", push, panelText(t, "Send a push notification for each change.")).Key("nav.pushSection"),
			).Key("nav.accordion").SingleOpen())),
			sample(t, "Tree view", treeFromCode(t, files, node, openKeys, handle, lastEvent)),
			sample(t, "Tree view, 10,000 generated folders", ui.Div().Width(ui.Px(420)).Child(ui.Tree(folders, big).
				Key("nav.bigTree").Label("Generated folders").Style(ui.Style{Height: ui.Px(200)}))),
			manyTabsSample(t),
			closableTabsSample(t),
		}
		// Each sample is a column, not a wrapped row, since the tabs and the tree are wide.
		col := ui.Div().Column().Gap(18)
		col.Children(samples...)
		return Card(t).Gap(14).
			Child(SectionTitle(t, "Navigation and layout")).
			Child(ui.Text("Tab moves to the selected tab. Left and Right select the neighbouring tabs; Ctrl+Tab goes to the next tab from the tab list or the panel.").FontSize(12).TextColor(t.TextMuted)).
			Child(col)
	})
}

// closableTabsSample is a tab list whose tabs the user can close. The app keeps the open
// tabs. OnClose drops the one that was closed, and selects the tab that took its place
// when that one was selected, in the same handler.
func closableTabsSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		open := ui.Local(ctx, "nav.closeOpen", []string{"Notes", "Draft", "Budget", "Read me"})
		picked := ui.Local(ctx, "nav.closePicked", "Notes")
		last := ui.Local(ctx, "nav.closeLast", "none")
		tabs := make([]ui.Tab[string], 0, len(open.Get()))
		for _, name := range open.Get() {
			tabs = append(tabs, ui.Tab[string]{Value: name, Label: name, Closable: true,
				Panel: func() ui.Element { return panelText(t, name+" panel") }})
		}
		return sample(t, "Tabs, closable: close button, middle click, Delete, Ctrl+W", ui.Div().Width(ui.Px(420)).Column().Gap(8).
			Child(ui.Tabs(picked, tabs...).Key("nav.closableTabs").Label("Open documents").
				OnClose(func(v string) {
					list := slices.Clone(open.Peek())
					i := slices.Index(list, v)
					list = slices.Delete(list, i, i+1)
					open.Set(list)
					if picked.Peek() == v && len(list) > 0 {
						picked.Set(list[min(i, len(list)-1)])
					}
					last.Set(v)
				})).
			Child(ui.Text("Last closed: "+last.Get()).FontSize(12).TextColor(t.TextMuted)))
	})
}

// manyTabsSample is a narrow strip of tabs that do not fit. The strip scrolls, with
// arrow buttons at its ends, the overflow menu lists every tab, the long label is cut
// short at TabMaxWidth, and the prefix and the suffix sit outside the strip. The add
// button appends a tab and selects it. The tabs are kept in the component.
func manyTabsSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		names := ui.Local(ctx, "nav.manyNames", []string{
			"Tab 1", "Tab 2", "Tab 3", "Tab 4", "Tab 5", "Tab 6", "Tab 7", "Tab 8", "Tab 9",
			"Quarterly revenue by region, with forecasts",
		})
		picked := ui.Local(ctx, "nav.manyPicked", "Tab 1")
		tabs := make([]ui.Tab[string], 0, len(names.Get()))
		for _, name := range names.Get() {
			tabs = append(tabs, ui.Tab[string]{Value: name, Label: name,
				Panel: func() ui.Element { return panelText(t, name+" panel") }})
		}
		add := ui.IconButton(icons.Plus).Label("Add tab").OnClick(func() {
			next := fmt.Sprintf("Tab %d", len(names.Peek()))
			names.Update(func(ns []string) []string { return append(slices.Clone(ns), next) })
			picked.Set(next)
		})
		return sample(t, "Many tabs: scroll, overflow menu, long label, add", ui.Div().Width(ui.Px(360)).Child(ui.Tabs(picked, tabs...).
			Key("nav.manyTabs").Label("Many tabs").
			Prefix(ui.Text("Sheets").FontSize(12).TextColor(t.TextMuted)).
			Suffix(add).
			OverflowMenu().TabMaxWidth(ui.Px(120))))
	})
}

// treeFromCode is the project tree with buttons that open and close it from code,
// and a status line. The status line shows the open keys, and the last node that was
// opened or closed, by the tree or by a button.
func treeFromCode(t *ui.Theme, files []ui.TreeNode, node *ui.Signal[string], openKeys *ui.Signal[[]string],
	handle *ui.TreeHandle, lastEvent *ui.Signal[string]) ui.Element {
	open := "none"
	if keys := openKeys.Get(); len(keys) > 0 {
		open = strings.Join(keys, ", ")
	}
	return ui.Div().Column().Gap(8).Width(ui.Px(420)).
		Child(ui.Div().Row().Wrap().Gap(8).
			Child(ui.Button("Expand all").Key("nav.treeExpandAll").Secondary().Small().OnClick(handle.ExpandAll)).
			Child(ui.Button("Collapse all").Key("nav.treeCollapseAll").Secondary().Small().OnClick(handle.CollapseAll)).
			Child(ui.Button("Reveal a deep node").Key("nav.treeReveal").Secondary().Small().
				OnClick(func() { handle.Reveal("src/ui/tree.go") }))).
		Child(ui.Tree(files, node).Key("nav.tree").Label("Project files").Style(ui.Style{MaxHeight: ui.Px(200)}).
			Handle(handle).Expanded(openKeys).
			OnExpand(func(key string) { lastEvent.Set("expanded " + key) }).
			OnCollapse(func(key string) { lastEvent.Set("collapsed " + key) })).
		Child(ui.Text(fmt.Sprintf("Open: %s. Last event: %s.", open, lastEvent.Get())).FontSize(12).TextColor(t.TextMuted))
}

// panelText is the content of a sample panel.
func panelText(t *ui.Theme, s string) ui.Element {
	return ui.Text(s).TextColor(t.TextMuted)
}

// projectFiles is a small file tree with a folder that has nested folders, to show
// the keys: Right opens a folder and Left closes it.
func projectFiles() []ui.TreeNode {
	return []ui.TreeNode{
		{Key: "src", Label: "src", Icon: icons.Folder, Children: func() []ui.TreeNode {
			return []ui.TreeNode{
				{Key: "src/main.go", Label: "main.go", Icon: icons.File},
				{Key: "src/ui", Label: "ui", Icon: icons.Folder, Children: func() []ui.TreeNode {
					return []ui.TreeNode{
						{Key: "src/ui/tabs.go", Label: "tabs.go", Icon: icons.File},
						{Key: "src/ui/tree.go", Label: "tree.go", Icon: icons.File},
					}
				}},
			}
		}},
		{Key: "docs", Label: "docs", Icon: icons.Folder, Children: func() []ui.TreeNode {
			return []ui.TreeNode{{Key: "docs/readme.md", Label: "readme.md", Icon: icons.File}}
		}},
		{Key: "LICENSE", Label: "LICENSE", Icon: icons.File},
	}
}

// bigFolders returns the 10,000 folders of the large tree, made on the first use
// and then kept: the page is built again on each visit, and the tree is not.
var bigFolders = sync.OnceValue(func() []ui.TreeNode { return generatedFolders(10000) })

// generatedFolders is n folders, each with three files that are made only when the
// folder opens, so a large tree costs what its visible part costs.
func generatedFolders(n int) []ui.TreeNode {
	nodes := make([]ui.TreeNode, n)
	for i := range nodes {
		key := fmt.Sprintf("folder-%04d", i)
		nodes[i] = ui.TreeNode{Key: key, Label: fmt.Sprintf("folder %04d", i), Icon: icons.Folder,
			Children: func() []ui.TreeNode {
				return []ui.TreeNode{
					{Key: key + "/a", Label: "a.txt", Icon: icons.File},
					{Key: key + "/b", Label: "b.txt", Icon: icons.File},
					{Key: key + "/c", Label: "c.txt", Icon: icons.File},
				}
			}}
	}
	return nodes
}

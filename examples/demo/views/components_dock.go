package views

import (
	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/ui"
)

// dockDefault is the layout the Dock section starts from, and the one its Reset layout
// button returns to: an explorer beside an editor, with an output log under the editor.
var dockDefault = ui.DockLayout{Root: ui.DockNode{
	Sizes: []float64{0.25, 0.75},
	Children: []ui.DockNode{
		{Panels: []string{"explorer"}, Active: "explorer"},
		{Vertical: true, Sizes: []float64{0.7, 0.3}, Children: []ui.DockNode{
			{Panels: []string{"editor", "notes"}, Active: "editor"},
			{Panels: []string{"output"}, Active: "output"},
		}},
	},
}}

// dockSection shows a dock with four panels. Its layout and the text of the editor are
// kept in the section, so they survive the section's re-renders. The demo keeps no settings
// file, so the layout is kept for the session only. A floated group opens in a window of
// its own with app.OpenDockWindow.
func dockSection() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		layout := ui.Local(ctx, "dock.layout", dockDefault)
		text := ui.Local(ctx, "dock.text", "Write the note here.")
		panels := map[string]ui.DockPanel{
			"explorer": {Title: "Explorer", Closable: true, Content: func() ui.Element {
				return ui.Div().Column().Gap(6).Padding(12).
					Child(panelText(t, "Inbox")).Child(panelText(t, "Projects")).Child(panelText(t, "Archive"))
			}},
			"editor": {Title: "Editor", Content: func() ui.Element {
				return ui.Div().Column().Padding(12).Child(ui.TextArea(text).Label("Note text").Grow(1))
			}},
			"notes": {Title: "Notes", Closable: true, Content: func() ui.Element {
				// The count and the line keep their values as the panel moves, floats and docks back.
				return ui.Component(func(ctx *ui.Context) ui.Element {
					count := ui.Local(ctx, "count", 0)
					line := ui.Local(ctx, "line", "")
					return ui.Div().Column().Gap(8).Padding(12).
						Child(panelText(t, "Dock a tab onto an edge of the section.")).
						Child(ui.TextInput(line).Label("Line")).
						Child(ui.Button("Count").Secondary().OnClick(func() { count.Set(count.Peek() + 1) })).
						Child(ui.Textf("Count: %d", count.Get()).TextColor(t.TextMuted))
				})
			}},
			"output": {Title: "Output", Closable: true, Content: func() ui.Element {
				return ui.Div().Column().Padding(12).Gap(4).
					Child(panelText(t, "Ready.")).Child(panelText(t, "Watching the project."))
			}},
		}
		return Card(t).Gap(14).
			Child(SectionTitle(t, "Docking")).
			Child(ui.Text("Drag a tab over a group to join it, to a side of it, or to an edge of the section. Drop a tab outside the section to float its panel in a window of its own. The tab menu moves, splits, floats and closes panels. Ctrl+Shift with an arrow key moves a panel to the group on that side.").FontSize(12).TextColor(t.TextMuted)).
			Child(ui.Div().Width(ui.Px(560)).Height(ui.Px(360)).
				Child(ui.Dock(layout, panels).Floating(app.OpenDockWindow))).
			Child(ui.Button("Reset layout").Secondary().OnClick(func() { layout.Set(dockDefault.Clone()) }))
	})
}

package views

import (
	"github.com/tyler-builds/jamb/ui"
)

// splitSection shows the split panes: side by side, stacked, nested, a pane that
// collapses, a split whose first pane has a size in pixels with limits, and a split of
// three panes. Each sample keeps its sizes in the section, so only the section re-renders.
func splitSection() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		across := ui.Local(ctx, "split.across", ui.Percent(35))
		down := ui.Local(ctx, "split.down", ui.Percent(50))
		outer := ui.Local(ctx, "split.outer", ui.Percent(40))
		inner := ui.Local(ctx, "split.inner", ui.Percent(50))
		collapsedSize := ui.Local(ctx, "split.collapsedSize", ui.Percent(30))
		collapsed := ui.Local(ctx, "split.collapsed", false)
		px := ui.Local(ctx, "split.px", ui.Px(160))

		// splitBox is a box of the given height for a split, as wide as the samples.
		splitBox := func(h float32, s *ui.SplitElement) ui.Element {
			return ui.Div().Width(ui.Px(420)).Height(ui.Px(h)).Child(s)
		}
		samples := []ui.Element{
			sample(t, "Side by side, by a percentage",
				splitBox(160, ui.Split(
					ui.Pane(splitPane(t, "List", "Notes in order")).SizeSignal(across).Min(ui.Px(120)).Label("List"),
					ui.Pane(splitPane(t, "Details", "The selected note")).Min(ui.Px(160)).Label("Details"),
				).Label("Resize list"))),
			sample(t, "Stacked, vertical",
				splitBox(200, ui.Split(
					ui.Pane(splitPane(t, "Editor", "The note text")).SizeSignal(down).Min(ui.Px(60)).Label("Editor"),
					ui.Pane(splitPane(t, "Preview", "The rendered note")).Min(ui.Px(60)).Label("Preview"),
				).Orientation(ui.SplitVertical).Label("Resize editor and preview"))),
			sample(t, "Nested: a stacked split in the second pane",
				splitBox(220, ui.Split(
					ui.Pane(splitPane(t, "Folders", "Project files")).SizeSignal(outer).Min(ui.Px(120)).Label("Folders"),
					ui.Pane(ui.Split(
						ui.Pane(splitPane(t, "Editor", "The note text")).SizeSignal(inner).Min(ui.Px(60)).Label("Editor"),
						ui.Pane(splitPane(t, "Preview", "The rendered note")).Min(ui.Px(60)).Label("Preview"),
					).Orientation(ui.SplitVertical).Label("Resize editor and preview")).Min(ui.Px(200)).Label("Work area"),
				).Label("Resize folders"))),
			sample(t, "Collapsible: Enter or a double-click",
				splitBox(160, ui.Split(
					ui.Pane(splitPane(t, "Files", "Collapse this pane")).SizeSignal(collapsedSize).Collapsible().Collapsed(collapsed).Label("Files"),
					ui.Pane(splitPane(t, "Editor", "Kept while collapsed")).Min(ui.Px(160)).Label("Editor"),
				).Label("Resize files"))),
			sample(t, "Pixels, from 120 to 320 px",
				splitBox(160, ui.Split(
					ui.Pane(splitPane(t, "Sidebar", "Fixed width")).SizeSignal(px).Min(ui.Px(120)).Max(ui.Px(320)).Label("Sidebar"),
					ui.Pane(splitPane(t, "Content", "Takes the rest")).Min(ui.Px(160)).Label("Content"),
				).Label("Resize sidebar"))),
			sample(t, "Three panes: pixels, fill, and a percentage",
				splitBox(160, ui.Split(
					ui.Pane(splitPane(t, "Navigation", "Fixed width")).Size(ui.Px(120)).Min(ui.Px(80)).Label("Navigation"),
					ui.Pane(splitPane(t, "Main", "Takes the rest")).Min(ui.Px(120)).Label("Main"),
					ui.Pane(splitPane(t, "Info", "A fifth of the width")).Size(ui.Percent(20)).Min(ui.Px(100)).Max(ui.Percent(40)).Label("Info"),
				).Label("Resize panes"))),
		}
		col := ui.Div().Row().Wrap().Align(ui.Start).Gap(22)
		col.Children(samples...)
		return Card(t).Gap(14).
			Child(SectionTitle(t, "Split panes")).
			Child(ui.Text("Drag a divider, or focus it and use the arrow keys (Shift for a larger step). Home and End go to its limits. Enter collapses or restores a collapsible pane, as a double-click does.").FontSize(12).TextColor(t.TextMuted)).
			Child(col)
	})
}

// splitPane is the content of a pane of a split sample: a title and a line of text.
func splitPane(t *ui.Theme, title, body string) ui.Element {
	return ui.Div().Padding(12).Column().Gap(6).
		Child(ui.Text(title).Semibold()).
		Child(panelText(t, body))
}

package views

import (
	"github.com/tyler-builds/jamb/ui"
)

// splitSection shows the split panes: side by side, stacked, nested, a pane that
// collapses, and a split whose first pane has a size in pixels with limits. Each
// sample keeps its position in the section, so only the section re-renders.
func splitSection() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		across := ui.Local(ctx, "split.across", 0.35)
		down := ui.Local(ctx, "split.down", 0.5)
		outer := ui.Local(ctx, "split.outer", 0.4)
		inner := ui.Local(ctx, "split.inner", 0.5)
		collapsedPos := ui.Local(ctx, "split.collapsedPos", 0.3)
		collapsed := ui.Local(ctx, "split.collapsed", false)
		px := ui.Local(ctx, "split.px", 160.0)

		// splitBox is a box of the given height for a split, as wide as the samples.
		splitBox := func(h float32, s *ui.SplitElement) ui.Element {
			return ui.Div().Width(ui.Px(420)).Height(ui.Px(h)).Child(s)
		}
		samples := []ui.Element{
			sample(t, "Side by side, by a fraction",
				splitBox(160, ui.Split(splitPane(t, "List", "Notes in order"), splitPane(t, "Details", "The selected note")).
					Position(across).Label("Resize list").FirstLabel("List").SecondLabel("Details").
					MinFirst(ui.Px(120)).MinSecond(ui.Px(160)))),
			sample(t, "Stacked, vertical",
				splitBox(200, ui.Split(splitPane(t, "Editor", "The note text"), splitPane(t, "Preview", "The rendered note")).
					Orientation(ui.SplitVertical).Position(down).Label("Resize editor and preview").
					FirstLabel("Editor").SecondLabel("Preview").MinFirst(ui.Px(60)).MinSecond(ui.Px(60)))),
			sample(t, "Nested: a stacked split in the second pane",
				splitBox(220, ui.Split(splitPane(t, "Folders", "Project files"),
					ui.Split(splitPane(t, "Editor", "The note text"), splitPane(t, "Preview", "The rendered note")).
						Orientation(ui.SplitVertical).Position(inner).Label("Resize editor and preview").
						FirstLabel("Editor").SecondLabel("Preview")).
					Position(outer).Label("Resize folders").FirstLabel("Folders").SecondLabel("Work area").
					MinFirst(ui.Px(120)).MinSecond(ui.Px(200)))),
			sample(t, "Collapsible: Enter or a double-click",
				splitBox(160, ui.Split(splitPane(t, "Files", "Collapse this pane"), splitPane(t, "Editor", "Kept while collapsed")).
					Position(collapsedPos).Collapsed(collapsed).Collapsible(ui.FirstPane).
					Label("Resize files").FirstLabel("Files").SecondLabel("Editor").MinSecond(ui.Px(160)))),
			sample(t, "Pixels, from 120 to 320 px",
				splitBox(160, ui.Split(splitPane(t, "Sidebar", "Fixed width"), splitPane(t, "Content", "Takes the rest")).
					PixelPosition(px).Label("Resize sidebar").FirstLabel("Sidebar").SecondLabel("Content").
					MinFirst(ui.Px(120)).MaxFirst(ui.Px(320)).MinSecond(ui.Px(160)))),
		}
		col := ui.Div().Row().Wrap().Align(ui.Start).Gap(22)
		col.Children(samples...)
		return Card(t).Gap(14).
			Child(SectionTitle(t, "Split panes")).
			Child(ui.Text("Drag the divider, or focus it and use the arrow keys (Shift for a larger step). Home and End go to its limits. Enter collapses or restores a collapsible pane, as a double-click does.").FontSize(12).TextColor(t.TextMuted)).
			Child(col)
	})
}

// splitPane is the content of a pane of a split sample: a title and a line of text.
func splitPane(t *ui.Theme, title, body string) ui.Element {
	return ui.Div().Padding(12).Column().Gap(6).
		Child(ui.Text(title).Semibold()).
		Child(panelText(t, body))
}

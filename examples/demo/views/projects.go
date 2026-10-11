package views

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// Projects is the project list with status filters, search and selection.
func Projects(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		all := s.Projects.Get()
		visible := s.Visible()
		selected := s.Selected.Get()
		open := s.Windows.Get()
		active := 0
		for _, p := range all {
			if p.Status == StatusActive {
				active++
			}
		}

		view := ui.Local(ctx, "view", 0) // 0 for the cards, 1 for the table
		picked := ui.Local(ctx, "picked", []string(nil))
		order := ui.Local(ctx, "order", ui.SortState{})

		list := ui.Div().Gap(10).Role(access.RoleList).SelectionContainer()
		for _, p := range visible {
			list.Child(projectCard(t, s, p, p.ID == selected, open[p.ID]))
		}
		if len(visible) == 0 {
			list.Child(emptyState(t, s))
		}

		var body ui.Element = list
		if view.Get() == 1 {
			body = ui.Div().Gap(10).Child(projectsTableBar(t, s, picked)).Child(projectsTable(t, s, visible, picked, order))
		}

		return ui.Div().Grow(1).ScrollY().
			Child(ui.Div().PaddingX(32).PaddingY(28).Gap(20).MaxWidth(ui.Px(1100)).
				Child(ui.Div().Row().Align(ui.End).Justify(ui.SpaceBetween).Gap(16).
					Child(ui.Div().Gap(4).
						Child(ui.Div().Heading(1).Child(ui.Text("Projects").Size(26).Semibold())).
						Child(ui.Textf("%d projects · %d active", len(all), active).Color(t.TextMuted))).
					Child(ui.Div().Row().Align(ui.Center).Gap(12).
						Child(projectsViewTabs(view)).
						Child(ui.Div().Width(ui.Px(320)).Child(filters(t, s))))).
				Child(body))
	})
}

// filters is the status filter: a segmented tab list. A project dragged onto the
// Active, Paused or Done tab takes that status. The drop target is the content
// of the tab, drawn by the tab's render slot (RenderTab), so the tab keeps its
// selection, keys and keyboard behavior, and the content is what takes the drop.
func filters(t *ui.Theme, s *Store) ui.Element {
	return ui.Tabs(s.Filter,
		ui.Tab[int]{Value: -1, Label: "All"},
		ui.Tab[int]{Value: int(StatusActive), Label: "Active"},
		ui.Tab[int]{Value: int(StatusPaused), Label: "Paused"},
		ui.Tab[int]{Value: int(StatusDone), Label: "Done"},
	).Key("projects-filter").Label("Status filter").Variant(ui.TabsSegmented).
		RenderTab(func(tab ui.Tab[int], _ ui.PartState) ui.Element {
			content := ui.Div().Key("filter-" + tab.Label).Row().Align(ui.Center).Child(ui.Text(tab.Label).NoWrap())
			if tab.Value >= 0 {
				value := tab.Value
				content.OnDrop(ui.Drop(func(id int, e *ui.DropEvent) { s.SetProjectStatus(id, Status(value)) }).
					Over(ui.Style{Background: t.AccentSoft, TextColor: t.Accent}))
			}
			return content
		})
}

// projectCard is one project. Matches of the search query are highlighted in
// its name and description, and open marks a project that has a window of its
// own.
func projectCard(t *ui.Theme, s *Store, p Project, selected, open bool) ui.Element {
	q := s.Query.Peek()
	titles := ui.Div().Row().Align(ui.Center).Gap(10).
		Child(ui.Text(p.Name).Size(15).Semibold().Truncate().HighlightMatches(q, matchHighlight(t))).
		Child(statusBadge(p.Status))
	if open {
		titles.Child(ui.BadgeText("Open in window").Variant(ui.BadgeAccent).Dot())
	}
	star := ui.Div().Size(ui.Px(30), ui.Px(30)).Radius(7).Center().
		TextColor(t.TextSubtle).Transition(fast).
		Hover(ui.Style{Background: t.SurfaceActive, TextColor: t.Text}).
		Label("Star project").Checked(p.Starred).
		OnClick(func() { s.ToggleStar(p.ID) }).
		Child(ui.Icon(starIcon(t, p.Starred)).Size(16).Color(starColor(t, p.Starred)))

	// The card takes focus, so Enter selects it and Shift+F10 opens its menu.
	card := ui.Div().Key(fmt.Sprint(p.ID)).Role(access.RoleListItem).Selected(selected).
		Focusable().FocusVisible(ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: t.FocusRing}}}).Row().Align(ui.Center).Gap(16).Padding(16).Radius(12).
		Background(t.Surface).Border(1, t.Border).Shadow(ui.ShadowSm).Transition(150*time.Millisecond).
		Hover(ui.Style{BorderColor: t.BorderStrong, Background: t.SurfaceHover}).
		When(selected, func(d *ui.DivElement) {
			d.Border(1, t.Accent).Shadow(ui.Shadow{Blur: 0, Spread: 3, Color: t.Accent.Alpha(0.18)})
		}).
		OnClick(func() { s.Selected.Set(p.ID) }).
		OnDoubleClick(func() { OpenProject(s, p.ID) }).
		ContextMenu(func() []ui.MenuItem { return projectMenu(s, p) }).
		OnDrag(ui.Drag(p.ID).Preview(func() ui.Element { return projectPreview(t, p) })).
		Dragging(ui.Style{Opacity: 0.5}).
		OnDrop(ui.Drop(func(id int, e *ui.DropEvent) {
			s.DropMark.Set(dropMark{})
			s.MoveProject(id, p.ID, e.LocalY > e.H/2) // the lower half drops after the card
		}).
			Accept(func(id int) bool { return id != p.ID }).
			Move(func(id int, e *ui.DropEvent) {
				mark := dropMark{ID: p.ID, After: e.LocalY > e.H/2}
				if s.DropMark.Peek() != mark {
					s.DropMark.Set(mark)
				}
			}).
			Leave(func() { s.DropMark.Set(dropMark{}) })).
		Child(IconTile(t, p.Icon, p.Color, 44)).
		Child(ui.Div().Grow(1).Gap(6).
			Child(titles).
			Child(ui.Text(p.Description).Size(13).Color(t.TextMuted).Truncate().HighlightMatches(q, matchHighlight(t))).
			Child(ui.Div().Row().Align(ui.Center).Gap(12).MarginTop(4).
				Child(ui.ProgressBar(float64(p.Progress)).Color(progressColor(t, p.Color)).Label(p.Name + " progress").Width(336)).
				Child(ui.Div().Row().Align(ui.Center).Gap(5).TextColor(t.TextSubtle).
					Child(ui.Icon(icons.Check).Size(13)).
					Child(ui.Textf("%d open tasks", p.Tasks).Size(12))))).
		Child(ui.Div().Align(ui.End).Gap(8).Shrink(0).
			Child(ui.Div().Row().Align(ui.Center).Gap(4).
				Child(ui.IconButton(icons.ArrowUpRight).Label("Open in window").Tooltip("Open in window").OnClick(func() { OpenProject(s, p.ID) })).
				Child(star)).
			Child(ui.Div().Row().Align(ui.Center).Gap(5).TextColor(t.TextSubtle).
				Child(ui.Icon(icons.Clock).Size(13)).
				Child(ui.Text("Updated " + ago(p.Updated)).Size(12).NoWrap())))

	// The insertion line is drawn in the gap on the side the drop would land.
	if mark := s.DropMark.Get(); mark.ID == p.ID {
		card.Child(insertionLine(t, mark.After))
	}
	return card
}

// projectMenu is the context menu of a project card: opening the project, starring
// it and setting its status, as the card's own controls do.
func projectMenu(s *Store, p Project) []ui.MenuItem {
	status := func(st Status) ui.MenuItem {
		return ui.MenuItem{Label: st.String(), Check: ui.MenuCheckRadio, Checked: p.Status == st,
			OnClick: func() { s.SetProjectStatus(p.ID, st) }}
	}
	return []ui.MenuItem{
		{Label: "Open", OnClick: func() { s.Selected.Set(p.ID) }},
		{Label: "Open in window", Icon: icons.ArrowUpRight, OnClick: func() { OpenProject(s, p.ID) }},
		{Label: "Star", Check: ui.MenuCheckBox, Checked: p.Starred, OnClick: func() { s.ToggleStar(p.ID) }},
		{Label: "Status", Submenu: []ui.MenuItem{status(StatusActive), status(StatusPaused), status(StatusDone)}},
		ui.MenuSeparator(),
		{Label: "Archive", Disabled: true},
	}
}

// insertionLine is the 2 px accent line in the gap above or below a card.
func insertionLine(t *ui.Theme, after bool) ui.Element {
	line := ui.Div().Absolute().Left(0).Right(0).Height(ui.Px(2)).Background(t.Accent)
	if after {
		return line.Bottom(-6)
	}
	return line.Top(-6)
}

// projectPreview is the compact card drawn under the pointer while a project
// is dragged.
func projectPreview(t *ui.Theme, p Project) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(8).PaddingX(10).PaddingY(6).Radius(10).
		Background(t.Surface).Border(1, t.Accent).Shadow(ui.ShadowMd).
		Child(IconTile(t, p.Icon, p.Color, 22)).
		Child(ui.Text(p.Name).Size(13).Semibold().NoWrap())
}

func starColor(t *ui.Theme, on bool) ui.Color {
	if on {
		return t.Warning
	}
	return ui.Color{}
}

func emptyState(t *ui.Theme, s *Store) ui.Element {
	return ui.Div().Padding(48).Gap(10).Align(ui.Center).Radius(12).Border(1, t.Border).
		Child(IconTile(t, icons.Search, t.TextMuted, 48)).
		Child(ui.Text("No projects match").Size(15).Semibold()).
		Child(ui.Text("Try a different search or filter.").Color(t.TextMuted)).
		Child(ui.Button("Clear filters").Secondary().Small().Style(ui.Style{Margin: ui.Edges{Top: 6}}).OnClick(func() {
			s.Query.Set("")
			s.Filter.Set(-1)
		}))
}

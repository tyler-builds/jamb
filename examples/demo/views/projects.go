package views

import (
	"fmt"
	"time"

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

		list := ui.Div().Gap(10)
		for _, p := range visible {
			list.Child(projectCard(t, s, p, p.ID == selected, open[p.ID]))
		}
		if len(visible) == 0 {
			list.Child(emptyState(t, s))
		}

		return ui.Div().Grow(1).ScrollY().
			Child(ui.Div().PaddingX(32).PaddingY(28).Gap(20).MaxWidth(ui.Px(1100)).
				Child(ui.Div().Row().Align(ui.End).Justify(ui.SpaceBetween).Gap(16).
					Child(ui.Div().Gap(4).
						Child(ui.Text("Projects").Size(26).Semibold()).
						Child(ui.Textf("%d projects · %d active", len(all), active).Color(t.TextMuted))).
					Child(filters(t, s))).
				Child(list))
	})
}

// filters are the status chips. A project dragged onto Active, Paused or Done
// takes that status.
func filters(t *ui.Theme, s *Store) ui.Element {
	current := s.Filter.Get()
	chip := func(label string, value int) ui.Element {
		on := current == value
		return ui.Div().Key("filter-"+label).Row().PaddingX(12).Height(ui.Px(30)).Radius(8).Center().
			TextColor(t.TextMuted).FontSize(13).FontWeight(ui.FontMedium).Transition(fast).
			Hover(ui.Style{TextColor: t.Text}).
			When(on, func(d *ui.DivElement) {
				d.Background(t.Surface).TextColor(t.Text).Shadow(ui.ShadowSm)
			}).
			OnClick(func() { s.Filter.Set(value) }).
			When(value >= 0, func(d *ui.DivElement) {
				d.OnDrop(ui.Drop(func(id int, e *ui.DropEvent) { s.SetProjectStatus(id, Status(value)) }).
					Over(ui.Style{Background: t.AccentSoft, TextColor: t.Accent}))
			}).
			Child(ui.Text(label))
	}
	return ui.Div().Row().Padding(3).Gap(2).Radius(10).Background(t.SurfaceActive.Alpha(0.6)).
		Child(chip("All", -1)).
		Child(chip("Active", int(StatusActive))).
		Child(chip("Paused", int(StatusPaused))).
		Child(chip("Done", int(StatusDone)))
}

// projectCard is one project. Matches of the search query are highlighted in
// its name and description, and open marks a project that has a window of its
// own.
func projectCard(t *ui.Theme, s *Store, p Project, selected, open bool) ui.Element {
	status := StatusColor(t, p.Status)
	q := s.Query.Peek()
	titles := ui.Div().Row().Align(ui.Center).Gap(10).
		Child(ui.Text(p.Name).Size(15).Semibold().Truncate().HighlightMatches(q, matchHighlight(t))).
		Child(Badge(p.Status.String(), status))
	if open {
		titles.Child(Badge("Open in window", t.Accent))
	}
	star := ui.Div().Size(ui.Px(30), ui.Px(30)).Radius(7).Center().
		TextColor(t.TextSubtle).Transition(fast).
		Hover(ui.Style{Background: t.SurfaceActive, TextColor: t.Text}).
		OnClick(func() { s.ToggleStar(p.ID) }).
		Child(ui.Icon(icons.Star).Size(16).Color(starColor(t, p.Starred)))

	card := ui.Div().Key(fmt.Sprint(p.ID)).Row().Align(ui.Center).Gap(16).Padding(16).Radius(12).
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
		Child(IconTile(p.Icon, p.Color, 44)).
		Child(ui.Div().Grow(1).Gap(6).
			Child(titles).
			Child(ui.Text(p.Description).Size(13).Color(t.TextMuted).Truncate().HighlightMatches(q, matchHighlight(t))).
			Child(ui.Div().Row().Align(ui.Center).Gap(12).MarginTop(4).
				Child(ProgressBar(t, p.Progress, p.Color).MaxWidth(ui.Px(280))).
				Child(ui.Textf("%.0f%%", p.Progress*100).Size(12).Medium().Color(t.TextMuted)).
				Child(ui.Div().Row().Align(ui.Center).Gap(5).TextColor(t.TextSubtle).
					Child(ui.Icon(icons.Check).Size(13)).
					Child(ui.Textf("%d open tasks", p.Tasks).Size(12))))).
		Child(ui.Div().Align(ui.End).Gap(8).Shrink(0).
			Child(ui.Div().Row().Align(ui.Center).Gap(4).
				Child(ui.IconButton(icons.ArrowUpRight).Tooltip("Open in window").OnClick(func() { OpenProject(s, p.ID) })).
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
		Child(IconTile(p.Icon, p.Color, 22)).
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
		Child(IconTile(icons.Search, t.TextMuted, 48)).
		Child(ui.Text("No projects match").Size(15).Semibold()).
		Child(ui.Text("Try a different search or filter.").Color(t.TextMuted)).
		Child(ui.Button("Clear filters").Secondary().Small().Style(ui.Style{Margin: ui.Edges{Top: 6}}).OnClick(func() {
			s.Query.Set("")
			s.Filter.Set(-1)
		}))
}

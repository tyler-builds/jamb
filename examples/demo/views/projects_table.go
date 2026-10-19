package views

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// projectsTable is the table view of the Projects page: the visible projects in a
// DataGrid, sorted by a header and selected by the keyboard or the mouse. Enter or
// a double-click opens a project in a window, as its card does, and the Open button
// of a row does the same. Open tasks is edited in place: a whole number from 0 to 999.
// The selected projects are starred together from the bar above the table.
func projectsTable(t *ui.Theme, s *Store, visible []Project, picked *ui.Signal[[]string], order *ui.Signal[ui.SortState]) ui.Element {
	cols := []ui.DataColumn[Project]{
		{Header: "Name", Key: "name", Text: func(p Project) string { return p.Name },
			Compare: ui.SortByText(func(p Project) string { return p.Name }),
			Cell:    func(p Project) ui.Element { return projectNameCell(t, p) },
			Grow:    2, MinWidth: 160, Resizable: true},
		{Header: "Status", Key: "status", Text: func(p Project) string { return p.Status.String() },
			Compare: ui.SortBy(func(p Project) Status { return p.Status }),
			Cell:    func(p Project) ui.Element { return statusBadge(p.Status) },
			Width:   120, Resizable: true},
		{Header: "Progress", Key: "progress", Text: func(p Project) string { return fmt.Sprintf("%d%%", int(p.Progress*100+0.5)) },
			Compare: ui.SortBy(func(p Project) float32 { return p.Progress }),
			Cell: func(p Project) ui.Element {
				// The bar is at most 160 px, so its readout stays in view in a wide column too.
				return ui.ProgressBar(float64(p.Progress)).Color(progressColor(t, p.Color)).Label(p.Name + " progress").MaxWidth(ui.Px(160))
			},
			Width: 200, Resizable: true},
		{Header: "Open tasks", Key: "tasks", Text: func(p Project) string { return strconv.Itoa(p.Tasks) },
			Compare: ui.SortBy(func(p Project) int { return p.Tasks }),
			Edit: func(p Project, v string) error {
				n, err := strconv.Atoi(strings.TrimSpace(v))
				if err != nil || n < 0 || n > 999 {
					return errors.New("Open tasks is a whole number from 0 to 999")
				}
				s.SetTasks(p.ID, n)
				return nil
			},
			Align: ui.End, Width: 120, Resizable: true},
		{Header: "Due", Key: "due", Text: func(p Project) string { return dueText(p.Due) },
			Compare: func(a, b Project) int { return a.Due.Compare(b.Due) },
			Width:   130, Resizable: true},
		{Header: "Updated", Key: "updated", Text: func(p Project) string { return ago(p.Updated) },
			Compare: func(a, b Project) int { return a.Updated.Compare(b.Updated) },
			Width:   140, Resizable: true},
		{Header: "Open", Key: "open", Interactive: true,
			Cell: func(p Project) ui.Element {
				return ui.Button("Open").Ghost().Small().Label("Open " + p.Name).OnClick(func() { OpenProject(s, p.ID) })
			},
			Width: 110, Resizable: true},
	}
	return ui.DataGrid(visible, func(p Project) string { return strconv.Itoa(p.ID) }, cols...).
		Label("Projects").Selection(ui.SelectionMultiple, picked).Sort(order).Striped().
		Empty(ui.Empty("No projects match").Icon(icons.Search).Description("Try a different search or filter.")).
		OnActivate(func(p Project) { OpenProject(s, p.ID) }).
		Style(ui.Style{Height: ui.Px(480)})
}

// projectNameCell is the name of a project in the table: its icon, its name and
// a star when it is starred. The name is also the cell's text for assistive
// technology, so the star is a sighted cue only.
func projectNameCell(t *ui.Theme, p Project) ui.Element {
	row := ui.Div().Row().Align(ui.Center).Gap(10).
		Child(IconTile(t, p.Icon, p.Color, 24)).
		Child(ui.Text(p.Name).Semibold().Truncate())
	if p.Starred {
		row.Child(ui.Icon(starIcon(t, true)).Size(14).Color(starColor(t, true)))
	}
	return row
}

// projectsTableBar is the line above the table: how many projects are selected,
// and the button that stars them.
func projectsTableBar(t *ui.Theme, s *Store, picked *ui.Signal[[]string]) ui.Element {
	keys := picked.Get()
	return ui.Div().Row().Align(ui.Center).Gap(12).
		Child(ui.Text(fmt.Sprintf("%d selected", len(keys))).FontSize(13).TextColor(t.TextMuted)).
		Child(ui.Button("Star selected").Secondary().Small().Disabled(len(keys) == 0).
			OnClick(func() { s.StarKeys(picked.Peek()) }))
}

// StarKeys stars the projects whose keys (their IDs) are in keys, and announces how
// many were starred.
func (s *Store) StarKeys(keys []string) {
	ids := make(map[int]bool, len(keys))
	for _, k := range keys {
		if id, err := strconv.Atoi(k); err == nil {
			ids[id] = true
		}
	}
	n := 0
	s.Projects.Update(func(ps []Project) []Project {
		out := append([]Project(nil), ps...)
		for i := range out {
			if ids[out[i].ID] && !out[i].Starred {
				out[i].Starred = true
				n++
			}
		}
		return out
	})
	ui.Announce(fmt.Sprintf("%d projects starred", n), access.Polite)
}

// SetTasks sets the number of open tasks of the project with the ID id.
func (s *Store) SetTasks(id, tasks int) {
	s.Projects.Update(func(ps []Project) []Project {
		out := append([]Project(nil), ps...)
		for i := range out {
			if out[i].ID == id {
				out[i].Tasks = tasks
			}
		}
		return out
	})
}

// projectsViewTabs switches the Projects page between its cards and its table.
func projectsViewTabs(view *ui.Signal[int]) ui.Element {
	return ui.Tabs(view,
		ui.Tab[int]{Value: 0, Label: "Cards"},
		ui.Tab[int]{Value: 1, Label: "Table"},
	).Key("projects-view").Label("Projects view").Variant(ui.TabsSegmented)
}

// dueText is the Due column's text: the due date, or "None" for a project with none. Sorting
// puts the projects with no due date first, since their date is the zero time.
func dueText(d time.Time) string {
	if d.IsZero() {
		return "None"
	}
	return d.Format("Jan 2, 2006")
}

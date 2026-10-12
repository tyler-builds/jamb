package views

import (
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// TestProjectsTableShowsTheVisibleProjects checks the Table view of the Projects
// page: the grid has a row for each visible project, and Enter opens the focused
// project in its own window, as a double-click on its card does.
func TestProjectsTableShowsTheVisibleProjects(t *testing.T) {
	r := newDemoRig(t)
	r.s.Page.Set(PageProjects)
	tab := accesstest.Find(t, r.tree(), access.RoleTab, "Table")
	r.bridge.Perform(tab.ID, access.ActionClick)
	tr := r.tree()
	g := accesstest.Find(t, tr, access.RoleDataGrid, "Projects")
	if want := len(r.s.Visible()); g.GridRows != want {
		t.Fatalf("the grid has %d rows, want the %d visible projects", g.GridRows, want)
	}
	if g.GridColumns != 6 {
		t.Errorf("the grid has %d columns, want 6", g.GridColumns)
	}
	r.bridge.Perform(g.ID, access.ActionFocus)
	r.frame()
	r.rt.KeyDown(input.KeyDown, 0, false)
	r.rt.KeyUp(input.KeyDown, 0)
	r.frame()
	r.rt.KeyDown(input.KeyEnter, 0, false)
	r.rt.KeyUp(input.KeyEnter, 0)
	r.frame()
	if open := r.s.Windows.Get(); len(open) == 0 {
		t.Error("Enter on a project row opened no window")
	}
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

// TestStarKeysStarsTheSelectedProjects checks the store operation behind "Star
// selected": it stars the projects named by their keys and no others.
func TestStarKeysStarsTheSelectedProjects(t *testing.T) {
	s := NewStore()
	var unstarred []string
	for _, p := range s.Projects.Get() {
		if !p.Starred {
			unstarred = append(unstarred, strconv.Itoa(p.ID))
		}
	}
	if len(unstarred) < 2 {
		t.Skip("the seed projects have fewer than two unstarred projects")
	}
	s.StarKeys(unstarred[:2])
	starred := 0
	for _, p := range s.Projects.Get() {
		if p.Starred {
			starred++
		}
	}
	want := 0
	for _, p := range NewStore().Projects.Get() {
		if p.Starred {
			want++
		}
	}
	if starred != want+2 {
		t.Errorf("starred %d projects, want %d (two more than the seed)", starred, want+2)
	}
}

// TestComponentsDataGridBoundsTheRows checks the Components page's grid of 100,000
// deploy runs: the access tree reports every row, only the rows in view are built,
// and the empty toggle shows the empty message.
func TestComponentsDataGridBoundsTheRows(t *testing.T) {
	r := newDemoRig(t)
	r.s.Page.Set(PageComponents)
	tr := r.tree()
	g := accesstest.Find(t, tr, access.RoleDataGrid, "Deploy runs")
	if g.GridRows != 100000 || g.GridColumns != 8 {
		t.Fatalf("the grid is %d by %d, want 100,000 rows of 8 columns", g.GridRows, g.GridColumns)
	}
	built := countNodes(tr, access.RoleListItem, func(n *access.Node) bool { return n.GridRows == 0 && n.PosInSet > 0 })
	if built == 0 || built > 60 {
		t.Errorf("%d rows are built; want the rows in view only", built)
	}
	none := accesstest.Find(t, tr, access.RoleCheckBox, "No rows")
	r.bridge.Perform(none.ID, access.ActionToggle)
	tr = r.tree()
	accesstest.Find(t, tr, access.RoleStaticText, "No deploy runs match")
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

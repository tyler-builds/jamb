package views

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/ui"
)

// demoRig is the demo's runtime with its accessibility bridge activated, so that
// announcements are published, and the development warnings it printed.
type demoRig struct {
	rt       *ui.Runtime
	s        *Store
	bridge   *access.Bridge
	warnings []string
	now      time.Time
	updates  []access.Update
}

func newDemoRig(t *testing.T) *demoRig {
	t.Helper()
	r := &demoRig{s: NewStore(), now: time.Unix(1_000_000, 0)}
	r.rt = ui.NewRuntime(App(r.s), ui.DarkTheme, nil)
	r.rt.SetWarnFunc(func(msg string) { r.warnings = append(r.warnings, msg) })
	r.rt.SetAccessWarnings(true)
	r.rt.SetViewport(1200, 760, 1)
	r.frame()
	r.bridge = r.rt.AccessBridge()
	r.bridge.Activate()
	r.bridge.Listen(func(u access.Update) { r.updates = append(r.updates, u) })
	return r
}

func (r *demoRig) frame() {
	r.now = r.now.Add(time.Second)
	r.rt.Frame(r.now)
}

// tree returns the published snapshot after a frame.
func (r *demoRig) tree() *access.Tree {
	r.frame()
	return r.bridge.Tree()
}

// countNodes returns the number of nodes with the role for which keep returns true.
func countNodes(t *access.Tree, role access.Role, keep func(*access.Node) bool) int {
	n := 0
	t.Walk(func(node *access.Node) bool {
		if node.Role == role && (keep == nil || keep(node)) {
			n++
		}
		return true
	})
	return n
}

// TestDemoControlsReportTheirStates checks the toggles, selections and expandable
// rows of each page, and that the actions on them change the app.
func TestDemoControlsReportTheirStates(t *testing.T) {
	r := newDemoRig(t)

	// Settings: switches labelled by their row titles.
	r.s.Page.Set(PageSettings)
	tr := r.tree()
	dark := accesstest.Find(t, tr, access.RoleSwitch, "Dark mode")
	accesstest.Check(t, dark, access.RoleSwitch, "Dark mode", access.StateToggle|access.StateChecked)
	title := accesstest.Find(t, tr, access.RoleStaticText, "Dark mode")
	if len(dark.LabelledBy) != 1 || dark.LabelledBy[0] != title.ID {
		t.Fatalf("the Dark mode switch is labelled by %v, want the title %d", dark.LabelledBy, title.ID)
	}
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleSwitch, "Compact activity"), access.RoleSwitch,
		"Compact activity", access.StateToggle)
	r.bridge.Perform(accesstest.Find(t, tr, access.RoleSwitch, "Compact activity").ID, access.ActionToggle)
	r.frame()
	if !r.s.Compact.Get() {
		t.Fatal("toggling the switch through the bridge did not change its setting")
	}
	if n := countNodes(r.tree(), access.RoleHeading, nil); n < 2 {
		t.Fatalf("%d headings on Settings, want the page title and the section titles", n)
	}

	// Projects: the status filter is a selection group, and the list a selection container.
	r.s.Page.Set(PageProjects)
	tr = r.tree()
	group := accesstest.Find(t, tr, access.RoleGroup, "Status filter")
	if !group.States.Has(access.StateSelectionContainer) {
		t.Fatalf("the status filter is not a selection container: [%s]", group.States)
	}
	all := accesstest.Find(t, tr, access.RoleButton, "All")
	accesstest.Check(t, all, access.RoleButton, "All", access.StateSelectable|access.StateSelected)
	done := accesstest.Find(t, tr, access.RoleButton, "Done")
	if !done.States.Has(access.StateSelectable) || done.States.Has(access.StateSelected) {
		t.Fatalf("the unchosen Done chip has states [%s]", done.States)
	}
	r.bridge.Perform(done.ID, access.ActionSelect)
	r.frame()
	if r.s.Filter.Get() != int(StatusDone) {
		t.Fatalf("selecting the Done chip set the filter to %d", r.s.Filter.Get())
	}
	accesstest.Check(t, accesstest.Find(t, r.tree(), access.RoleButton, "Done"), access.RoleButton, "Done", access.StateSelected)

	// Each project card is a list item, and the selected one is selected.
	selected := 0
	tr = r.tree()
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleListItem && n.States.Has(access.StateSelected) {
			selected++
		}
		return true
	})
	if selected > 1 {
		t.Fatalf("%d project cards are selected, want at most one", selected)
	}
	stars := countNodes(tr, access.RoleButton, func(n *access.Node) bool { return n.Name == "Star project" && n.States.Has(access.StateToggle) })
	if stars == 0 {
		t.Fatal("no star toggles on the Projects page")
	}

	// Activity: a build row expands and collapses through the bridge.
	r.s.Page.Set(PageActivity)
	tr = r.tree()
	var row *access.Node
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleButton && n.States.Has(access.StateExpandable) {
			row = n
			return false
		}
		return true
	})
	if row == nil {
		t.Fatalf("no expandable build row on the Activity page:\n%s", tr.Dump())
	}
	if row.States.Has(access.StateExpanded) {
		t.Fatal("a build row starts expanded")
	}
	r.bridge.Perform(row.ID, access.ActionExpand)
	r.frame()
	if !expandedRowOpen(r.tree(), row.ID) {
		t.Fatal("Expand did not open the build row")
	}
	r.bridge.Perform(row.ID, access.ActionCollapse)
	r.frame()
	if expandedRowOpen(r.tree(), row.ID) {
		t.Fatal("Collapse did not close the build row")
	}

	// The sidebar shows the current page as selected, and the theme switch the theme.
	r.s.Page.Set(PageSettings)
	tr = r.tree()
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Settings"), access.RoleButton, "Settings",
		access.StateSelectable|access.StateSelected)
	if n := accesstest.Find(t, tr, access.RoleButton, "Dashboard"); n.States.Has(access.StateSelected) {
		t.Fatal("Dashboard is selected while Settings is the page")
	}
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Dark"), access.RoleButton, "Dark",
		access.StateSelectable|access.StateSelected)

	if len(r.warnings) != 0 {
		t.Fatalf("the demo produced accessibility warnings:\n%s", strings.Join(r.warnings, "\n"))
	}
}

// expandedRowOpen reports whether the node id is expanded in the tree.
func expandedRowOpen(t *access.Tree, id access.ID) bool {
	n := t.Node(id)
	return n != nil && n.States.Has(access.StateExpanded)
}

// TestDemoAnnouncesCreatedProjects checks that creating a project announces it,
// as the bridge delivers the announcement with the next published tree.
func TestDemoAnnouncesCreatedProjects(t *testing.T) {
	r := newDemoRig(t)
	r.s.OpenNewProject()
	r.s.NewName.Set("Sample project")
	tr := r.tree()
	create := accesstest.Find(t, tr, access.RoleButton, "Create project")
	r.bridge.Perform(create.ID, access.ActionClick)
	r.frame()
	found := false
	for _, u := range r.updates {
		for _, a := range u.Announcements {
			found = found || (a.Text == "Project created" && a.Politeness == access.Polite)
		}
	}
	if !found {
		t.Fatal("creating a project did not announce it")
	}
}

// TestEventCountText checks the count that the Activity page shows and announces.
func TestEventCountText(t *testing.T) {
	cases := []struct {
		n     int
		query string
		want  string
	}{
		{1, "", "1 event"},
		{2000, "", "2,000 events"},
		{1, "build", "1 matching event"},
		{12, "build", "12 matching events"},
	}
	for _, c := range cases {
		if got := eventCountText(c.n, c.query); got != c.want {
			t.Errorf("eventCountText(%d, %q) = %q, want %q", c.n, c.query, got, c.want)
		}
	}
}

// TestDemoProjectCardsTakeFocus checks that a project card can be focused, so that
// a keyboard user can select it and open its context menu.
func TestDemoProjectCardsTakeFocus(t *testing.T) {
	r := newDemoRig(t)
	r.s.Page.Set(PageProjects)
	tr := r.tree()
	cards := 0
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleListItem {
			cards++
			if !n.States.Has(access.StateFocusable) {
				t.Errorf("the project card %q cannot take focus", n.Name)
			}
		}
		return true
	})
	if cards == 0 {
		t.Fatal("no project cards on the Projects page")
	}
}

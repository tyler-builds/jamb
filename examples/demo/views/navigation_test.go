package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/ui"
)

// countRoleIn returns the number of nodes with role in the tree.
func countRoleIn(tr *access.Tree, role access.Role) int {
	n := 0
	tr.Walk(func(nd *access.Node) bool {
		if nd.Role == role {
			n++
		}
		return true
	})
	return n
}

// TestComponentsNavigationReportsRolesAndStates checks the navigation and layout
// samples of the Components page: the tab lists, the disabled tab, the collapsible
// header, the accordion's headers and the tree views, and that the page builds
// without development warnings.
func TestComponentsNavigationReportsRolesAndStates(t *testing.T) {
	r := newDemoRig(t)
	r.s.Page.Set(PageComponents)
	tr := r.tree()
	if len(r.warnings) != 0 {
		t.Fatalf("warnings on the Components page: %v", r.warnings)
	}

	list := accesstest.Find(t, tr, access.RoleTabList, "Plain sections")
	if !list.States.Has(access.StateSelectionContainer) {
		t.Fatalf("the Plain sections list has states [%s], want a selection container", list.States)
	}
	alpha := accesstest.Find(t, tr, access.RoleTab, "Alpha")
	if !alpha.States.Has(access.StateSelected) || !alpha.States.Has(access.StateSelectable) {
		t.Fatalf("Alpha has states [%s], want selected", alpha.States)
	}
	usage := accesstest.Find(t, tr, access.RoleTab, "Usage")
	if !usage.States.Has(access.StateDisabled) || usage.States.Has(access.StateSelected) {
		t.Fatalf("Usage has states [%s], want disabled and not selected", usage.States)
	}
	accesstest.Find(t, tr, access.RoleTab, "Home")

	header := accesstest.Find(t, tr, access.RoleButton, "Advanced options")
	if !header.States.Has(access.StateExpandable) || header.States.Has(access.StateExpanded) {
		t.Fatalf("Advanced options has states [%s], want expandable and closed", header.States)
	}
	email := accesstest.Find(t, tr, access.RoleButton, "Email alerts")
	if !email.States.Has(access.StateExpanded) {
		t.Fatalf("the first accordion section has states [%s], want expanded", email.States)
	}
	if accesstest.Find(t, tr, access.RoleButton, "Text alerts").States.Has(access.StateExpanded) {
		t.Fatalf("a second accordion section is open at once in a single-open accordion")
	}

	files := accesstest.Find(t, tr, access.RoleTree, "Project files")
	if !files.States.Has(access.StateSelectionContainer) {
		t.Fatalf("Project files has states [%s], want a selection container", files.States)
	}
	src := accesstest.Find(t, tr, access.RoleTreeItem, "src")
	if src.Level != 1 || src.PosInSet != 1 || src.SetSize != 3 || !src.States.Has(access.StateExpandable) {
		t.Fatalf("src has level %d, position %d of %d, states [%s]", src.Level, src.PosInSet, src.SetSize, src.States)
	}
	if !src.States.Has(access.StateSelected) {
		t.Fatalf("src is not selected, though it is the first selection of the sample: [%s]", src.States)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings after the navigation samples were read: %v", r.warnings)
	}
}

// TestComponentsLargeTreeBuildsOnlyVisibleRows checks that the 10,000-node
// sample builds the rows in view and no more.
func TestComponentsLargeTreeBuildsOnlyVisibleRows(t *testing.T) {
	r := newDemoRig(t)
	r.s.Page.Set(PageComponents)
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleTree, "Generated folders")
	if n := countRoleIn(tr, access.RoleTreeItem); n == 0 || n > 120 {
		t.Fatalf("%d tree items are built for the page; the 10,000 folders should build only the rows in view", n)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// TestProjectWindowOpensOnTheOverview checks that the project window shows the
// overview first, and that the comments are in their own tab.
func TestProjectWindowOpensOnTheOverview(t *testing.T) {
	s := NewStore()
	rt := ui.NewRuntime(ProjectDetail(s, s.Projects.Peek()[0].ID), DemoDarkTheme, nil)
	rt.SetViewport(680, 640, 1)
	rt.Frame(time.Now())
	tr := rt.AccessTree()
	overview := accesstest.Find(t, tr, access.RoleTab, "Overview")
	if !overview.States.Has(access.StateSelected) {
		t.Fatalf("the project window is not on the Overview tab: [%s]", overview.States)
	}
	comments := accesstest.Find(t, tr, access.RoleTab, "Comments")
	if comments.States.Has(access.StateSelected) {
		t.Fatalf("the Comments tab is selected on the first view")
	}
}

package views

import (
	"strconv"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/ui"
)

// TestDemoAccessibility checks the names and roles that the demo exposes to
// assistive technology on each page, and that no page produces a development
// warning.
func TestDemoAccessibility(t *testing.T) {
	s := NewStore()
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	var warnings []string
	rt.SetWarnFunc(func(msg string) { warnings = append(warnings, msg) })
	rt.SetAccessWarnings(true)
	rt.SetViewport(1200, 760, 1)
	now := time.Unix(1_000_000, 0)
	frame := func() {
		now = now.Add(time.Second)
		rt.Frame(now)
	}
	frame()

	for _, p := range []Page{PageDashboard, PageProjects, PageActivity, PageComponents, PageSettings} {
		s.Page.Set(p)
		frame()
		frame()
		tr := rt.AccessTree()
		for _, nav := range []string{"Dashboard", "Activity", "Components", "Settings"} {
			accesstest.Find(t, tr, access.RoleButton, nav)
		}
		// The Projects item names its project count, as a badge on the item does.
		if n := tr.Find(access.RoleButton, "Projects, "+strconv.Itoa(len(s.Projects.Get()))+" items"); n == nil {
			t.Fatalf("no Projects navigation button:\n%s", tr.Dump())
		}
		accesstest.Find(t, tr, access.RoleButton, "Minimize")
		accesstest.Find(t, tr, access.RoleButton, "Maximize")
		accesstest.Find(t, tr, access.RoleButton, "Close")
		accesstest.Find(t, tr, access.RoleButton, "New Project")
		accesstest.Find(t, tr, access.RoleButton, "Notifications")
		accesstest.Find(t, tr, access.RoleTextField, "Search projects")
		accesstest.Find(t, tr, access.RoleImage, "Account avatar")
		if p == PageActivity {
			accesstest.Find(t, tr, access.RoleTextField, "Filter events")
		}
		if p == PageComponents {
			// The Split panes section: a divider named for its list, and a collapsible one.
			accesstest.Find(t, tr, access.RoleSeparator, "Resize list")
			accesstest.Find(t, tr, access.RoleSeparator, "Resize files")
		}
	}

	s.OpenNewProject()
	frame()
	tr := rt.AccessTree()
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleDialog, "New project"), access.RoleDialog, "New project", 0)
	accesstest.Find(t, tr, access.RoleTextField, "Name")
	accesstest.Find(t, tr, access.RoleTextArea, "Description")

	if len(warnings) != 0 {
		t.Fatalf("the demo produced accessibility warnings:\n%v", warnings)
	}
}

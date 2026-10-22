package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui/uitest"
)

// TestChurnLeavesNoExtraNodes checks that moving between pages and typing in the
// filters leaves the window with the nodes a fresh window has: a node that is
// rebuilt in place (see ui.newNode) is the same node, and one that is no longer
// built is retired, so the count does not grow with the churn.
func TestChurnLeavesNoExtraNodes(t *testing.T) {
	pages := []Page{PageDashboard, PageProjects, PageActivity, PageComponents, PageSettings, PageMarkdown}
	s := NewStore()
	u := uitest.New(t, App(s), uitest.Size(1200, 760), uitest.Theme(DemoDarkTheme))
	for i := range 120 {
		s.Page.Set(pages[i%len(pages)])
		u.Advance(16 * time.Millisecond)
		if i%6 == 3 {
			s.SectionField.Focus()
			u.Advance(16 * time.Millisecond)
			u.Type("b")
			u.Advance(16 * time.Millisecond)
			u.Press("backspace")
			u.Advance(16 * time.Millisecond)
		}
	}
	s.Page.Set(PageProjects)
	u.Settle()
	got := u.Runtime().Stats().Nodes

	s2 := NewStore()
	u2 := uitest.New(t, App(s2), uitest.Size(1200, 760), uitest.Theme(DemoDarkTheme))
	u2.Settle()
	if want := u2.Runtime().Stats().Nodes; got != want {
		t.Errorf("nodes after churn %d, fresh window %d", got, want)
	}
}

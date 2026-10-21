package views

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// The many-tabs sample keeps its long label as the tab's name, and its add button
// appends a tab that the strip then lists, without development warnings.
func TestManyTabsAddsATabAndKeepsItsFullLabel(t *testing.T) {
	r := newDemoRig(t)
	showComponents(r)
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleTab, "Tab 1")
	accesstest.Find(t, tr, access.RoleTab, "Quarterly revenue by region, with forecasts")

	add := accesstest.Find(t, tr, access.RoleButton, "Add tab")
	r.bridge.Perform(add.ID, access.ActionClick)
	tr = r.tree()
	accesstest.Find(t, tr, access.RoleTab, "Tab 10")
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

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

// The tree buttons open the project tree from code, and the status line reports the
// open keys and the last node that was opened or closed.
func TestTreeButtonsOpenRevealAndReportFromCode(t *testing.T) {
	r := newDemoRig(t)
	showComponents(r)
	clickButton(r, t, "Reveal a deep node")
	accesstest.Find(t, r.tree(), access.RoleTreeItem, "tree.go")
	if !hasText(r, "Open: src, src/ui. Last event: expanded src/ui.") {
		t.Error("after Reveal, the status line does not show the opened nodes and the last one")
	}
	clickButton(r, t, "Collapse all")
	if !hasText(r, "Open: none. Last event: collapsed src/ui.") {
		t.Error("after Collapse all, the status line does not show that no node is open")
	}
	clickButton(r, t, "Expand all")
	if !hasText(r, "Open: src, src/ui, docs. Last event: expanded docs.") {
		t.Error("after Expand all, the status line does not show every node open")
	}
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

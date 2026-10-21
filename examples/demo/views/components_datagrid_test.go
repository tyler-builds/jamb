package views

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// The shipments grid of the Data grid section has its 60 rows loaded, its 31 columns
// (the order, six data columns, 24 week columns and the actions), and no development
// warnings. The cell selection grid beside it is there too.
func TestShipmentsGridShowsItsRowsAndColumns(t *testing.T) {
	r := newDemoRig(t)
	showComponents(r)
	tr := r.tree()
	g := accesstest.Find(t, tr, access.RoleDataGrid, "Shipments")
	if g.GridRows != 60 || g.GridColumns != 31 {
		t.Fatalf("the grid is %d by %d, want 60 rows of 31 columns", g.GridRows, g.GridColumns)
	}
	accesstest.Find(t, tr, access.RoleDataGrid, "Cell selection")
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// TestDataGridAddAndRemoveSelectionKeepTheOthers checks AddToSelection and
// RemoveFromSelection on the rows of a multiple selection: adding keeps the chosen
// rows, removing takes one out, and a row that is already chosen is not added twice.
func TestDataGridAddAndRemoveSelectionKeepTheOthers(t *testing.T) {
	sel := NewSignal([]string(nil))
	r := dgRig(dgItems(10), SelectionMultiple, sel, NewSignal(SortState{}), nil)
	row := func(name string) *access.Node {
		return accesstest.Find(t, r.tree(), access.RoleListItem, name)
	}
	r.rt.runAccess(row("Item 000001, 1").ID, access.Request{Action: access.ActionSelect})
	r.frame()
	r.rt.runAccess(row("Item 000002, 2").ID, access.Request{Action: access.ActionAddToSelection})
	r.frame()
	if got := strings.Join(sel.Peek(), ","); got != "i1,i2" {
		t.Fatalf("after AddToSelection of i2: selection %q, want i1,i2", got)
	}
	r.rt.runAccess(row("Item 000002, 2").ID, access.Request{Action: access.ActionAddToSelection})
	r.frame()
	if got := strings.Join(sel.Peek(), ","); got != "i1,i2" {
		t.Errorf("AddToSelection of a chosen row changed the selection to %q", got)
	}
	r.rt.runAccess(row("Item 000001, 1").ID, access.Request{Action: access.ActionRemoveFromSelection})
	r.frame()
	if got := strings.Join(sel.Peek(), ","); got != "i2" {
		t.Errorf("after RemoveFromSelection of i1: selection %q, want i2", got)
	}
}

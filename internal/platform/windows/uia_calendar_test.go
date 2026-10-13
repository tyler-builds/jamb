//go:build windows

package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// A calendar is the Calendar control type, with the Grid and Table patterns, and its days are
// the cells of the grid (see the Calendar control type of UI Automation).
func TestCalendarControlTypeAndPatterns(t *testing.T) {
	if got := controlType(access.RoleCalendar); got != 50001 || ctCalendar != 50001 {
		t.Fatalf("a calendar's control type is %d, want 50001 (Calendar)", got)
	}
	cal := &access.Node{Role: access.RoleCalendar}
	if !supports(kindGrid, cal) || !supports(kindTable, cal) {
		t.Error("a calendar does not support the Grid and Table patterns")
	}
	if supports(kindGrid, &access.Node{Role: access.RoleComboBox}) {
		t.Error("a combo box supports the Grid pattern")
	}
}

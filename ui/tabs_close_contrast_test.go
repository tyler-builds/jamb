package ui

import "testing"

// TestTabsCloseButtonIsDrawnUnderHighContrast checks that the close button of a tab
// that is neither selected nor under the pointer is hidden, as it is by default, and
// drawn under high contrast, where the system draws its buttons.
func TestTabsCloseButtonIsDrawnUnderHighContrast(t *testing.T) {
	sel := NewSignal("a")
	var closed []string
	r := closeTabs(sel, &closed, true)
	if op := closeButton(t, r, "Close Gamma").style.Opacity; op == 0 {
		t.Fatalf("the close button of an unselected tab is drawn at full opacity, want it hidden until hover")
	}
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	if op := closeButton(t, r, "Close Gamma").style.Opacity; op != 0 {
		t.Fatalf("high contrast: the close button has opacity %v, want it drawn", op)
	}
}

//go:build windows

package windows

import "testing"

// TestShownState checks which window states are on screen: a window is shown when
// it has the visible style and is not minimized. A hidden window has no style, and
// a minimized one keeps it.
func TestShownState(t *testing.T) {
	for name, tc := range map[string]struct {
		visible, minimized, want bool
	}{
		"shown":             {true, false, true},
		"hidden":            {false, false, false},
		"minimized":         {true, true, false},
		"hidden, minimized": {false, true, false},
	} {
		if got := shownState(tc.visible, tc.minimized); got != tc.want {
			t.Errorf("%s: shownState = %t; want %t", name, got, tc.want)
		}
	}
}

// TestWakeTimerWanted checks that timed wake-ups are scheduled only for a shown
// window: the caret, tips and animations of a hidden or minimized window wait.
func TestWakeTimerWanted(t *testing.T) {
	if !wakeTimerWanted(true) {
		t.Error("a shown window schedules no wake-up")
	}
	if wakeTimerWanted(false) {
		t.Error("a hidden or minimized window schedules a wake-up")
	}
}

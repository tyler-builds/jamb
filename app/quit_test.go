package app

import (
	"strings"
	"sync/atomic"
	"testing"
)

func TestQuitsOnDestroy(t *testing.T) {
	for _, c := range []struct {
		name                                 string
		left                                 int
		keepRunning, quitting, quitWhenClose bool
		want                                 bool
	}{
		{"the last window", 0, false, false, false, true},
		{"a window of several", 1, false, false, false, false},
		{"the last window, keeping running", 0, true, false, false, false},
		{"the last window Quit closed, keeping running", 0, true, true, false, true},
		{"a window Quit closed, others left", 1, true, true, false, false},
		{"a QuitWhenClosed window, keeping running", 1, true, false, true, true},
		{"a QuitWhenClosed window, closing the others", 1, false, true, true, false},
	} {
		if got := quitsOnDestroy(c.left, c.keepRunning, c.quitting, c.quitWhenClose); got != c.want {
			t.Errorf("%s: quits %v; want %v", c.name, got, c.want)
		}
	}
}

// Run runs once per process: a second call fails, and says how to show windows
// again instead, rather than returning at once without a window.
func TestRunOnce(t *testing.T) {
	var ran atomic.Bool
	if err := runOnce(&ran); err != nil {
		t.Fatalf("the first run failed: %v", err)
	}
	err := runOnce(&ran)
	if err == nil || !strings.Contains(err.Error(), "KeepRunning") {
		t.Errorf("the second run gave %v; want an error that names KeepRunning", err)
	}
}

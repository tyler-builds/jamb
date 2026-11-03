package app

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tyler-builds/jamb/ui"
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

// A Window without a Root starts the app with no window when it keeps running,
// and is an error that says how to start with no window otherwise.
func TestFirstWindow(t *testing.T) {
	root := func(*ui.Context) ui.Element { return nil }
	if open, err := firstWindow(Window{Root: root}, false); !open || err != nil {
		t.Errorf("a window with a Root: open %v, %v; want true, nil", open, err)
	}
	if open, err := firstWindow(Window{Root: root}, true); !open || err != nil {
		t.Errorf("a window with a Root, keeping running: open %v, %v; want true, nil", open, err)
	}
	if open, err := firstWindow(Window{}, true); open || err != nil {
		t.Errorf("no Root, keeping running: open %v, %v; want false, nil", open, err)
	}
	if _, err := firstWindow(Window{}, false); err == nil || !strings.Contains(err.Error(), "KeepRunning") {
		t.Errorf("no Root: %v; want an error that names KeepRunning", err)
	}
}

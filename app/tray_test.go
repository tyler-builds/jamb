package app

import (
	"image"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// formatUUID writes a UUID in the 8-4-4-4-12 form.
func formatUUID(u [16]byte) string {
	const hex = "0123456789abcdef"
	var b strings.Builder
	for i, c := range u {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			b.WriteByte('-')
		}
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

// TestTrayGUID checks that the identity of a tray icon is stable across runs (Windows
// keeps the user's choice to show an icon by it, so it must never change), that it is a
// version 8 UUID of the RFC 9562 variant, and that it differs by app and by icon.
func TestTrayGUID(t *testing.T) {
	a := trayGUID("JambExamples.Notes", 1)
	if b := trayGUID("JambExamples.Notes", 1); a != b {
		t.Errorf("trayGUID changed between calls: %s and %s", formatUUID(a), formatUUID(b))
	}
	if a[6]>>4 != 8 || a[8]>>6 != 2 {
		t.Errorf("trayGUID %s is not a version 8 UUID of the RFC 9562 variant", formatUUID(a))
	}
	if got, want := formatUUID(a), "8545fe0d-06a4-82d8-b916-ab2dc4790a39"; got != want {
		t.Errorf("trayGUID(JambExamples.Notes, 1) = %s, want %s: a changed identity loses the users' choices", got, want)
	}
	for name, other := range map[string][16]byte{
		"another icon of the app": trayGUID("JambExamples.Notes", 2),
		"another app":             trayGUID("JambExamples.Other", 1),
	} {
		if other == a {
			t.Errorf("%s has the same identity %s", name, formatUUID(other))
		}
	}
}

func TestTrayIDsCount(t *testing.T) {
	first, second := NewTray(TrayOptions{}), NewTray(TrayOptions{})
	if second.id != first.id+1 {
		t.Errorf("tray IDs %d and %d; want consecutive", first.id, second.id)
	}
	if first.native != nil {
		t.Error("a new tray has an icon before Show")
	}
}

func TestTraySeparator(t *testing.T) {
	if !TraySeparator().separator {
		t.Error("TraySeparator is not a separator")
	}
	if (TrayItem{Label: "Quit"}).separator {
		t.Error("a labelled item is a separator")
	}
}

func TestClampProgress(t *testing.T) {
	for in, want := range map[float64]float64{-0.5: 0, 0: 0, 0.4: 0.4, 1: 1, 1.2: 1, nanValue(): 0} {
		if got := clampProgress(in); got != want {
			t.Errorf("clampProgress(%v) = %v; want %v", in, got, want)
		}
	}
}

// nanValue returns a NaN, which is not a constant expression in Go.
func nanValue() float64 {
	zero := 0.0
	return zero / zero
}

func TestProgressStateNative(t *testing.T) {
	for state, want := range map[ui.TaskbarState]uint8{
		ui.TaskbarNone:          0,
		ui.TaskbarNormal:        1,
		ui.TaskbarIndeterminate: 2,
		ui.TaskbarPaused:        3,
		ui.TaskbarError:         4,
		ui.TaskbarState(200):    0,
	} {
		if got := uint8(nativeState(state)); got != want {
			t.Errorf("nativeState(%d) = %d; want %d", state, got, want)
		}
	}
}

func TestCheckBadge(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := checkBadge(img, "  "); err == nil {
		t.Error("a badge with a blank description passed")
	}
	if err := checkBadge(img, "3 unread"); err != nil {
		t.Errorf("a badge with a description: %v", err)
	}
	if err := checkBadge(nil, ""); err != nil {
		t.Errorf("clearing a badge: %v", err)
	}
}

// A close the user asks for hides the window only while CloseToTray holds true and a tray
// icon is shown; the app's own close always closes.
func TestCloseHides(t *testing.T) {
	on, off := ui.NewSignal(true), ui.NewSignal(false)
	for _, tc := range []struct {
		name   string
		sig    *ui.Signal[bool]
		shown  int
		asked  bool
		hidden bool
	}{
		{"on, icon shown", on, 1, false, true},
		{"on, no icon", on, 0, false, false},
		{"off", off, 1, false, false},
		{"nil", nil, 1, false, false},
		{"the app's own close", on, 1, true, false},
	} {
		if got := closeHides(tc.sig, tc.shown, tc.asked); got != tc.hidden {
			t.Errorf("%s: closeHides = %v, want %v", tc.name, got, tc.hidden)
		}
	}
}

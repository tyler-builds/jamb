package app

import (
	"image"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// The namespace of the DNS names in RFC 4122.
var namespaceDNS = [16]byte{0x6b, 0xa7, 0xb8, 0x10, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8}

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

// TestUUIDV5 checks the name-based UUID against a known value: the version 5 UUID
// of "python.org" in the DNS namespace, as the Python documentation gives it.
func TestUUIDV5(t *testing.T) {
	got := uuidV5(namespaceDNS, "python.org")
	if s := formatUUID(got); s != "886313e1-3b8a-5372-9b90-0c9aee199e5d" {
		t.Errorf("uuidV5 = %s; want 886313e1-3b8a-5372-9b90-0c9aee199e5d", s)
	}
}

// TestTrayGUID checks that the identity of a tray icon is stable across runs, that
// it is a version 5 UUID, and that it differs by app and by icon.
func TestTrayGUID(t *testing.T) {
	a := trayGUID("JambExamples.Notes", 1)
	if b := trayGUID("JambExamples.Notes", 1); a != b {
		t.Errorf("trayGUID changed between calls: %s and %s", formatUUID(a), formatUUID(b))
	}
	if a[6]>>4 != 5 || a[8]>>6 != 2 {
		t.Errorf("trayGUID %s is not a version 5 UUID of the RFC 4122 variant", formatUUID(a))
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
	for state, want := range map[ui.ProgressState]uint8{
		ui.TaskbarNone:          0,
		ui.TaskbarNormal:        1,
		ui.TaskbarIndeterminate: 2,
		ui.TaskbarPaused:        3,
		ui.TaskbarError:         4,
		ui.ProgressState(200):   0,
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

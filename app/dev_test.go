package app

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

func TestParseKey(t *testing.T) {
	cases := []struct {
		spec string
		key  input.Key
		mods input.Modifiers
	}{
		{"enter", input.KeyEnter, 0},
		{"z", input.KeyZ, 0},
		{"ctrl+z", input.KeyZ, input.ModCtrl},
		{"Ctrl+Shift+Left", input.KeyLeft, input.ModCtrl | input.ModShift},
		{"0", input.Key0, 0},
		{"ctrl+5", input.Key5, input.ModCtrl},
		{"9", input.Key9, 0},
		{"f", input.KeyF, 0},
		{"f1", input.KeyF1, 0},
		{"alt+f12", input.KeyF12, input.ModAlt},
		{"insert", input.KeyInsert, 0},
		{"super+d", input.KeyD, input.ModSuper},
		{"ctrl+plus", input.KeyEqual, input.ModCtrl},
	}
	for _, c := range cases {
		k, mods, ok := parseKey(c.spec)
		if !ok || k != c.key || mods != c.mods {
			t.Errorf("parseKey(%q) = %v, %v, %v; want %v, %v, true", c.spec, k, mods, ok, c.key, c.mods)
		}
	}
}

func TestParseKeyRejectsUnknown(t *testing.T) {
	for _, spec := range []string{"cmd+k", "ctrl+", "f0", "f13", "10", "ctrl+enterr", "+"} {
		if _, _, ok := parseKey(spec); ok {
			t.Errorf("parseKey(%q) succeeded; want an error", spec)
		}
	}
}

// A key the script cannot press stops the script when it is read, rather
// than being skipped while the script runs.
func TestScriptRejectsUnknownKey(t *testing.T) {
	if _, err := parseScript("idle; key ctrl+4; exit"); err != nil {
		t.Fatalf("ctrl+4: %v", err)
	}
	_, err := parseScript("idle; key cmd+k; exit")
	if err == nil || !strings.Contains(err.Error(), "cmd+k") {
		t.Fatalf("cmd+k: err = %v; want an error naming the key", err)
	}
}

//go:build js

package web

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

func TestKeyCodes(t *testing.T) {
	for code, want := range map[string]input.Key{
		"KeyA": input.KeyA, "KeyZ": input.KeyZ, "Digit0": input.Key0, "Digit9": input.Key9,
		"F1": input.KeyF1, "F12": input.KeyF12, "Enter": input.KeyEnter, "NumpadEnter": input.KeyEnter,
		"ArrowLeft": input.KeyLeft, "Backquote": input.KeyBacktick, "ContextMenu": input.KeyMenu,
	} {
		if got := keyCodes[code]; got != want {
			t.Errorf("keyCodes[%q] = %v, want %v", code, got, want)
		}
	}
	if k, ok := keyCodes["F13"]; ok {
		t.Errorf("F13 maps to %v; jamb has no such key", k)
	}
}

func TestBrowserShortcut(t *testing.T) {
	for _, c := range []struct {
		k    input.Key
		m    input.Modifiers
		want bool
	}{
		{input.KeyR, input.ModCtrl, true},                  // reload
		{input.KeyW, input.ModSuper, true},                 // close the tab, on macOS
		{input.KeyI, input.ModCtrl | input.ModShift, true}, // developer tools
		{input.KeyF5, 0, true},
		{input.KeyC, input.ModCtrl, false}, // copy is the app's
		{input.KeyI, input.ModCtrl, false}, // italic is the app's
		{input.KeyTab, 0, false},           // focus moves in the app
		{input.KeyR, 0, false},             // typing
	} {
		if got := browserShortcut(c.k, c.m); got != c.want {
			t.Errorf("browserShortcut(%v, %v) = %v, want %v", c.k, c.m, got, c.want)
		}
	}
}

func TestClipboardKey(t *testing.T) {
	for _, c := range []struct {
		k    input.Key
		m    input.Modifiers
		want bool
	}{
		{input.KeyC, input.ModCtrl, true},
		{input.KeyX, input.ModCtrl, true},
		{input.KeyV, input.ModCtrl, true},
		{input.KeyV, input.ModCtrl | input.ModShift, false}, // the app's own shortcut
		{input.KeyC, input.ModCtrl | input.ModAlt, false},
		{input.KeyC, 0, false}, // typing
		{input.KeyA, input.ModCtrl, false},
	} {
		if got := clipboardKey(c.k, c.m); got != c.want {
			t.Errorf("clipboardKey(%v, %v) = %v, want %v", c.k, c.m, got, c.want)
		}
	}
}

func TestPrintable(t *testing.T) {
	for _, c := range []struct {
		key  string
		m    input.Modifiers
		want bool
	}{
		{"a", 0, true},
		{"A", input.ModShift, true},
		{"é", 0, true},
		{"a", input.ModCtrl, false},
		{"Enter", 0, false},
		{"Dead", 0, false},
	} {
		if got := printable(c.key, c.m); got != c.want {
			t.Errorf("printable(%q, %v) = %v, want %v", c.key, c.m, got, c.want)
		}
	}
}

package app

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// TestParseKeyPunctuation checks that the script key names read the punctuation keys, with and
// without modifiers, at their physical positions.
func TestParseKeyPunctuation(t *testing.T) {
	cases := []struct {
		spec string
		key  input.Key
		mods input.Modifiers
	}{
		{"/", input.KeySlash, 0},
		{"ctrl+/", input.KeySlash, input.ModCtrl},
		{"\\", input.KeyBackslash, 0},
		{"[", input.KeyLeftBracket, 0},
		{"ctrl+shift+[", input.KeyLeftBracket, input.ModCtrl | input.ModShift},
		{"ctrl+shift+]", input.KeyRightBracket, input.ModCtrl | input.ModShift},
		{";", input.KeySemicolon, 0},
		{"'", input.KeyApostrophe, 0},
		{",", input.KeyComma, 0},
		{".", input.KeyPeriod, 0},
		{"`", input.KeyBacktick, 0},
	}
	for _, c := range cases {
		k, m, ok := parseKey(c.spec)
		if !ok || k != c.key || m != c.mods {
			t.Errorf("parseKey(%q) = %v, %v, %v; want %v, %v", c.spec, k, m, ok, c.key, c.mods)
		}
	}
	if _, _, ok := parseKey("ctrl+nope"); ok {
		t.Error("an unknown key name parsed")
	}
}

package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// punctuationKeys maps each punctuation key name to its key. The names are the characters the US
// layout prints on the keys.
var punctuationKeys = []struct {
	name string
	key  input.Key
}{
	{"-", input.KeyMinus},
	{"=", input.KeyEqual},
	{"/", input.KeySlash},
	{"\\", input.KeyBackslash},
	{"[", input.KeyLeftBracket},
	{"]", input.KeyRightBracket},
	{";", input.KeySemicolon},
	{"'", input.KeyApostrophe},
	{",", input.KeyComma},
	{".", input.KeyPeriod},
	{"`", input.KeyBacktick},
}

// TestParsePunctuationKeyNames checks that each punctuation name parses to its key, alone and with
// modifiers, and that an unknown name is an error that lists the names.
func TestParsePunctuationKeyNames(t *testing.T) {
	a := NewAction("test.punct", "Punct")
	for _, tc := range punctuationKeys {
		for _, keys := range []string{tc.name, "ctrl+" + tc.name, "ctrl+shift+" + tc.name} {
			b, err := ParseBinding("", keys, a)
			if err != nil {
				t.Errorf("ParseBinding(%q): %v", keys, err)
				continue
			}
			if got := b.strokes[0].key; got != tc.key {
				t.Errorf("ParseBinding(%q) key = %d, want %d", keys, got, tc.key)
			}
		}
		b, _ := ParseBinding("", "ctrl+shift+"+tc.name, a)
		if want := input.ModCtrl | input.ModShift; b.strokes[0].mods != want {
			t.Errorf("ParseBinding(ctrl+shift+%s) mods = %d, want %d", tc.name, b.strokes[0].mods, want)
		}
	}
	_, err := ParseBinding("", "ctrl+~", a)
	if err == nil || !strings.Contains(err.Error(), `unknown key "~"`) || !strings.Contains(err.Error(), "`") {
		t.Errorf("ParseBinding(ctrl+~) error = %v, want an unknown key that lists the punctuation names", err)
	}
}

// TestPunctuationKeysLabelAsTheyAreNamed checks that KeysFor shows each punctuation key as the
// character it is named with, so the label reads back as the binding that was written.
func TestPunctuationKeysLabelAsTheyAreNamed(t *testing.T) {
	for _, tc := range punctuationKeys {
		a := NewAction("test.punct.label", "Label")
		r := newKeyRig(func(ctx *Context) Element { return Div() })
		r.rt.AddBindings(Bind("ctrl+shift+"+tc.name, a))
		if want := "Ctrl+Shift+" + tc.name; r.rt.KeysFor(a) != want {
			t.Errorf("KeysFor(ctrl+shift+%s) = %q, want %q", tc.name, r.rt.KeysFor(a), want)
		}
	}
}

// TestPunctuationKeysTypeTextWithoutModifiers checks that a single-stroke binding on a punctuation key
// consumes text the way = and - do: the key types text on its own, and not with Ctrl, Alt or Super.
func TestPunctuationKeysTypeTextWithoutModifiers(t *testing.T) {
	for _, tc := range punctuationKeys {
		s := keystroke{key: tc.key}
		if !s.typesText() {
			t.Errorf("%s does not type text on its own", tc.name)
		}
		if (keystroke{key: tc.key, mods: input.ModCtrl}).typesText() {
			t.Errorf("ctrl+%s types text", tc.name)
		}
	}
}

package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// The Pause/Break key binds by name, as "pause" or "break", and a menu shows it as Pause.
func TestPauseKeyBinds(t *testing.T) {
	cancelQuery := NewAction("test.cancel-query", "Cancel query")
	for _, keys := range []string{"alt+pause", "alt+break", "ALT+Break"} {
		b, err := ParseBinding("", keys, cancelQuery)
		if err != nil {
			t.Fatalf("ParseBinding(%q): %v", keys, err)
		}
		if want := (keystroke{input.KeyPause, input.ModAlt}); len(b.strokes) != 1 || b.strokes[0] != want {
			t.Errorf("ParseBinding(%q) = %v, want %v", keys, b.strokes, want)
		}
	}

	ran := 0
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Key("editor").Size(Px(200), Px(100)).Focusable().OnAction(cancelQuery, func() { ran++ })
	}, 400, 300)
	r.rt.AddBindings(Bind("alt+break", cancelQuery))
	if got := r.rt.KeysFor(cancelQuery); got != "Alt+Pause" {
		t.Errorf("KeysFor = %q, want %q", got, "Alt+Pause")
	}
	r.focus(t, "editor")
	r.key(input.KeyPause, input.ModAlt)
	if ran != 1 {
		t.Errorf("Alt+Pause ran the action %d times, want 1", ran)
	}
}

// Alt held while Pause is pressed is not a tap of Alt, so releasing Alt first does not
// focus the menu bar.
func TestAltPauseDoesNotFocusMenuBar(t *testing.T) {
	r := menuBarRig()
	r.focus(t, "editor")
	r.rt.KeyDown(input.KeyUnknown, input.ModAlt, false)
	r.rt.KeyDown(input.KeyPause, input.ModAlt, false)
	r.rt.KeyUp(input.KeyUnknown, 0) // Alt is released before Pause
	r.rt.KeyUp(input.KeyPause, 0)
	r.frame()
	if r.focusKey() != "editor" {
		t.Errorf("Alt+Pause moved focus to %q; want it to stay on the editor", r.focusKey())
	}
}

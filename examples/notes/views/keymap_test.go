package views

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/keymap"
)

// A keymap file names the notes actions, and its bindings work like any other.
func TestKeymapFileBindsNewNote(t *testing.T) {
	r := newRig(t)
	bs, err := keymap.Parse([]byte(`[{"bindings": {"ctrl+shift+n": "notes.new"}}]`), Actions())
	if err != nil {
		t.Fatalf("keymap.Parse: %v", err)
	}
	r.rt.AddBindings(bs...)
	before := len(r.s.Notes.Peek())
	r.key(input.KeyN, input.ModCtrl|input.ModShift)
	if len(r.s.Notes.Peek()) != before+1 {
		t.Errorf("Ctrl+Shift+N from the keymap file: %d notes, want %d", len(r.s.Notes.Peek()), before+1)
	}
}

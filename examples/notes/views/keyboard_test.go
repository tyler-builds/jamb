package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/ui"
)

// focusRingOn reports whether the focus ring is drawn around the element with
// key: a ring quad covers the element's center.
func focusRingOn(r *rig, key string) bool {
	r.t.Helper()
	box, ok := r.rt.Find(key)
	if !ok {
		r.t.Fatalf("no element with key %q", key)
	}
	cx, cy := box.X+box.W/2, box.Y+box.H/2
	for _, q := range r.rt.Frame(time.Now()).Quads {
		if q.Background.A == 0 && q.BorderWidth > 0 && q.BorderColor == scene.Color(ui.DarkTheme().FocusRing) && q.Bounds.Contains(cx, cy) {
			return true
		}
	}
	return false
}

// tabTo presses Tab until the focus ring is on the element with key. It gives
// up after a bounded number of presses.
func tabTo(r *rig, key string) bool {
	for range 60 {
		if focusRingOn(r, key) {
			return true
		}
		r.key(input.KeyTab, 0)
	}
	return focusRingOn(r, key)
}

// Tab reaches a tag chip and a note row. Enter toggles the chip's filter, Space
// too, and Enter opens the note. A click removes the ring.
func TestNotesRowsAndChipsByKeyboard(t *testing.T) {
	r := seededRig(t, 3)
	notes := r.s.Notes.Peek()
	r.s.AddTag(notes[1], "work")
	r.frame()

	if !tabTo(r, "chip-work") {
		t.Fatal("Tab does not reach the tag chip")
	}
	r.key(input.KeyEnter, 0)
	if r.s.TagFilter.Peek() != "work" {
		t.Errorf("Enter on the chip: filter %q, want work", r.s.TagFilter.Peek())
	}
	r.key(input.KeySpace, 0)
	if r.s.TagFilter.Peek() != "" {
		t.Errorf("Space on the chip: filter %q, want none", r.s.TagFilter.Peek())
	}

	id := notes[0].ID
	if !tabTo(r, id) {
		t.Fatal("Tab does not reach a note row")
	}
	r.key(input.KeyEnter, 0)
	if r.s.Selected.Peek() != id {
		t.Errorf("Enter on a note row: selected %q, want %q", r.s.Selected.Peek(), id)
	}

	r.clickKey(id)
	if focusRingOn(r, id) {
		t.Error("the ring stayed on the note row after a click")
	}
}

// Ctrl+Enter is not a click on a focused row, so a modified Enter opens nothing.
func TestModifiedEnterDoesNotOpenNoteRow(t *testing.T) {
	r := seededRig(t, 2)
	notes := r.s.Notes.Peek()
	r.s.Selected.Set(notes[1].ID)
	if !tabTo(r, notes[0].ID) {
		t.Fatal("Tab does not reach a note row")
	}
	before := r.s.Selected.Peek()
	r.key(input.KeyEnter, input.ModCtrl)
	if r.s.Selected.Peek() != before {
		t.Errorf("Ctrl+Enter changed the selection to %q", r.s.Selected.Peek())
	}
}

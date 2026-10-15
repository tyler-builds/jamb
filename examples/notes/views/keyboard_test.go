package views

import (
	"testing"

	"github.com/tyler-builds/jamb/ui/uitest"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

// newUI renders the app over s, with its key bindings, in a window of the
// app's default size.
func newUI(t *testing.T, s *store.Store) *uitest.UI {
	return uitest.New(t, Root(s), uitest.Bindings(Bindings()...), uitest.Size(1100, 720))
}

// tabTo presses Tab until the focus ring is drawn around the element with key.
// It gives up after a bounded number of presses.
func tabTo(u *uitest.UI, key string) bool {
	for range 60 {
		if u.Get(key).FocusRing() {
			return true
		}
		u.Press("tab")
	}
	return u.Get(key).FocusRing()
}

// Tab reaches a tag chip and a note row. Enter toggles the chip's filter, Space
// too, and Enter opens the note. A click removes the ring.
func TestNotesRowsAndChipsByKeyboard(t *testing.T) {
	s := seededStore(t, 3)
	notes := s.Notes.Peek()
	s.AddTag(notes[1], "work")
	u := newUI(t, s)

	if !tabTo(u, "chip-work") {
		t.Fatal("Tab does not reach the tag chip")
	}
	u.Press("enter")
	if s.TagFilter.Peek() != "work" {
		t.Errorf("Enter on the chip: filter %q, want work", s.TagFilter.Peek())
	}
	u.Press("space")
	if s.TagFilter.Peek() != "" {
		t.Errorf("Space on the chip: filter %q, want none", s.TagFilter.Peek())
	}

	id := notes[0].ID
	row := u.Get("row-" + id) // the row draws the ring; its text is keyed by the ID
	if !tabTo(u, row.Key()) {
		t.Fatal("Tab does not reach a note row")
	}
	u.Press("enter")
	if s.Selected.Peek() != id {
		t.Errorf("Enter on a note row: selected %q, want %q", s.Selected.Peek(), id)
	}
	if !row.Selected() {
		t.Error("the opened note's row does not report itself selected")
	}

	row.Click()
	if row.FocusRing() {
		t.Error("the ring stayed on the note row after a click")
	}
}

// Ctrl+Enter is not a click on a focused row, so a modified Enter opens nothing.
func TestModifiedEnterDoesNotOpenNoteRow(t *testing.T) {
	s := seededStore(t, 2)
	notes := s.Notes.Peek()
	s.Selected.Set(notes[1].ID)
	u := newUI(t, s)
	if !tabTo(u, "row-"+notes[0].ID) {
		t.Fatal("Tab does not reach a note row")
	}
	before := s.Selected.Peek()
	u.Press("ctrl+enter")
	if s.Selected.Peek() != before {
		t.Errorf("Ctrl+Enter changed the selection to %q", s.Selected.Peek())
	}
}

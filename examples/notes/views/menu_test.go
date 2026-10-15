package views

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// rightClickKey presses the right button at the middle of the element with key,
// which opens its context menu.
func (r *rig) rightClickKey(t *testing.T, key string) {
	t.Helper()
	x, y := r.keyCenter(t, key)
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonRight, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonRight, 0)
	r.frame()
}

// menuOpen reports whether a menu is on screen.
func (r *rig) menuOpen() bool { return r.shown("jamb.menu") }

// dump returns the laid-out tree as text, for checking what a menu shows.
func (r *rig) dump() string {
	var buf bytes.Buffer
	r.rt.Dump(&buf, 96)
	return buf.String()
}

// The menu bar sits over the top of the window, and the content starts below it.
func TestMenuBarIsAboveContent(t *testing.T) {
	r := newRig(t)
	bar, ok := r.rt.Find("jamb.menubar.0")
	if !ok {
		t.Fatal("no menu bar")
	}
	if bar.Y < 0 || bar.Y+bar.H > menuBarHeight {
		t.Errorf("File label at y %v..%v, want inside the bar's %d px", bar.Y, bar.Y+bar.H, menuBarHeight)
	}
	sidebar, ok := r.rt.Find("sidebar")
	if !ok {
		t.Fatal("no sidebar list")
	}
	if sidebar.Y < menuBarHeight {
		t.Errorf("sidebar starts at y %v, under the %d px bar", sidebar.Y, menuBarHeight)
	}
}

// File's New note runs the app's own action, which creates a note.
func TestFileMenuNewNote(t *testing.T) {
	r := newRig(t)
	before := len(r.s.Notes.Peek())
	r.clickKey("jamb.menubar.0")
	if !r.menuOpen() {
		t.Fatal("clicking File did not open its menu")
	}
	r.clickKey("jamb.menu.0.0")
	if got := len(r.s.Notes.Peek()); got != before+1 {
		t.Errorf("New note from File: %d notes, want %d", got, before+1)
	}
	if r.menuOpen() {
		t.Error("the menu is still open after New note")
	}
}

// Edit and View show the shortcuts of the window's real bindings.
func TestMenuBarShowsShortcuts(t *testing.T) {
	r := newRig(t)
	r.clickKey("jamb.menubar.1")
	if d := r.dump(); !strings.Contains(d, "Ctrl+Z") || !strings.Contains(d, "Ctrl+A") {
		t.Errorf("Edit does not show Ctrl+Z and Ctrl+A")
	}
	r.clickKey("jamb.menubar.2")
	if d := r.dump(); !strings.Contains(d, "Ctrl+Shift+P") || !strings.Contains(d, "Ctrl+=") {
		t.Errorf("View does not show Ctrl+Shift+P and Ctrl+=")
	}
}

// Zoom in from the View menu steps the zoom, and Ctrl+0 resets it.
func TestViewMenuZoom(t *testing.T) {
	r := newRig(t)
	r.clickKey("jamb.menubar.2")
	r.clickKey("jamb.menu.0.3") // Zoom in
	if z := r.rt.Zoom(); z != 1.1 {
		t.Fatalf("zoom after Zoom in = %v, want 1.1", z)
	}
	r.key(input.KeyEqual, input.ModCtrl)
	if z := r.rt.Zoom(); z != 1.25 {
		t.Errorf("zoom after Ctrl+= = %v, want 1.25", z)
	}
	r.key(input.Key0, input.ModCtrl)
	if z := r.rt.Zoom(); z != 1 {
		t.Errorf("zoom after Ctrl+0 = %v, want 1", z)
	}
}

// The menu of a note row pins and unpins it, and deletes it after selecting it.
func TestNoteRowMenuPinsAndDeletes(t *testing.T) {
	r := newRig(t)
	n := r.s.Create()
	r.frame()
	r.rightClickKey(t, n.ID)
	if !r.menuOpen() {
		t.Fatal("right-click on the note row did not open its menu")
	}
	r.clickKey("jamb.menu.0.0") // Pin
	if !n.Pinned.Peek() {
		t.Error("Pin did not pin the note")
	}
	r.rightClickKey(t, n.ID)
	if !strings.Contains(r.dump(), "Unpin") {
		t.Error("the menu of a pinned note does not offer Unpin")
	}
	r.key(input.KeyEscape, 0)
	if r.menuOpen() {
		t.Error("Escape did not close the note's menu")
	}

	r.s.Create() // a second note, so the first is not the selected one
	r.frame()
	r.rightClickKey(t, n.ID)
	r.clickKey("jamb.menu.0.2") // Delete
	if r.s.Selected.Peek() != n.ID {
		t.Errorf("Delete selected %q, want the note %q", r.s.Selected.Peek(), n.ID)
	}
}

// Delete asks for confirmation in the editor, with Cancel and Delete buttons:
// Cancel keeps the note, and Delete removes it.
func TestDeleteAsksForConfirmation(t *testing.T) {
	r := newRig(t)
	n := r.s.Create()
	r.s.Create() // a second note, so the first is not the selected one
	r.frame()
	before := len(r.s.Notes.Peek())

	r.rightClickKey(t, n.ID)
	r.clickKey("jamb.menu.0.2") // Delete: asks, and deletes nothing yet
	if !strings.Contains(r.dump(), "Delete this note?") || len(r.s.Notes.Peek()) != before {
		t.Fatalf("Delete did not ask first: notes %d (was %d)", len(r.s.Notes.Peek()), before)
	}
	r.clickKey("confirm-cancel")
	if len(r.s.Notes.Peek()) != before || strings.Contains(r.dump(), "Delete this note?") {
		t.Errorf("Cancel changed the notes (%d, was %d) or left the question open", len(r.s.Notes.Peek()), before)
	}

	r.rightClickKey(t, n.ID)
	r.clickKey("jamb.menu.0.2")
	r.clickKey("confirm-delete")
	if len(r.s.Notes.Peek()) != before-1 || r.s.Find(n.ID) != nil {
		t.Errorf("Delete left %d notes (was %d) or kept the note", len(r.s.Notes.Peek()), before)
	}
}

// Alt by itself focuses the menu bar, and Escape from its menu gives focus back.
func TestAltFocusesMenuBarInNotes(t *testing.T) {
	r := newRig(t)
	r.rt.KeyDown(input.KeyUnknown, input.ModAlt, false)
	r.rt.KeyUp(input.KeyUnknown, 0)
	r.frame()
	if _, ok := r.rt.Find("jamb.menubar.0"); !ok {
		t.Fatal("no bar")
	}
	r.key(input.KeyDown, 0)
	if !r.menuOpen() {
		t.Fatal("Down on the focused bar did not open File")
	}
	r.key(input.KeyEscape, 0)
	if r.menuOpen() {
		t.Error("Escape did not close File")
	}
}

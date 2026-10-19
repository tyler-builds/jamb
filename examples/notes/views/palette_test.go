package views

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

func openPaletteKey(r *rig) {
	r.key(input.KeyP, input.ModCtrl|input.ModShift)
}

func TestPaletteOpensAndFilters(t *testing.T) {
	r := newRig(t)
	if _, ok := r.rt.Find("jamb.palette"); ok {
		t.Fatal("palette is open before any key was pressed")
	}
	openPaletteKey(r)
	if _, ok := r.rt.Find("jamb.palette"); !ok {
		t.Fatal("Ctrl+Shift+P did not open the palette")
	}
	r.typeText("theme")
	if _, ok := r.rt.Find("jamb.palette/notes.theme"); !ok {
		t.Error(`typing "theme" does not list Toggle theme`)
	}
	if _, ok := r.rt.Find("jamb.palette/notes.new"); ok {
		t.Error(`typing "theme" still lists New note`)
	}
}

func TestPaletteEnterRunsSelectedCommand(t *testing.T) {
	r := newRig(t)
	dark := r.rt.Theme().Dark
	openPaletteKey(r)
	r.typeText("theme")
	r.key(input.KeyEnter, 0)
	if r.rt.Theme().Dark == dark {
		t.Error("Enter did not toggle the theme")
	}
	if _, ok := r.rt.Find("jamb.palette"); ok {
		t.Error("the palette is still open after running a command")
	}
}

func TestPaletteEscapeCloses(t *testing.T) {
	r := newRig(t)
	openPaletteKey(r)
	r.key(input.KeyEscape, 0)
	if _, ok := r.rt.Find("jamb.palette"); ok {
		t.Error("Escape did not close the palette")
	}
}

func TestPaletteDownThenEnterRunsSecondCommand(t *testing.T) {
	r := newRig(t)
	openPaletteKey(r)
	r.key(input.KeyDown, 0)
	r.key(input.KeyEnter, 0) // the second listed command is Search notes
	r.typeText("zz")
	if r.s.Query.Peek() != "zz" {
		t.Errorf("search query %q, want \"zz\": Search notes did not run", r.s.Query.Peek())
	}
}

func TestChordTogglesThemeWithoutTyping(t *testing.T) {
	r := newRig(t)
	r.key(input.KeyN, input.ModCtrl) // the title field has focus
	dark := r.rt.Theme().Dark
	r.key(input.KeyK, input.ModCtrl)
	// The platform sends the key, then its text, then the release.
	r.rt.KeyDown(input.KeyT, input.ModCtrl, false)
	r.rt.TextInput("t")
	r.rt.KeyUp(input.KeyT, input.ModCtrl)
	r.frame()
	if r.rt.Theme().Dark == dark {
		t.Error("Ctrl+K Ctrl+T did not toggle the theme")
	}
	if title := r.selected().Title.Peek(); title != "" {
		t.Errorf("the chord typed %q into the title", title)
	}
}

func TestAltDownWorksInTitleField(t *testing.T) {
	r := newRig(t)
	r.key(input.KeyN, input.ModCtrl) // new note, title focused
	created := r.selected()
	r.key(input.KeyDown, input.ModAlt)
	if r.selected() == created {
		t.Error("Alt+Down with the title focused did not move to the next note")
	}
}

// Escape closes the palette and puts focus back in the note body the user was
// typing in.
func TestPaletteEscapeRestoresBodyFocus(t *testing.T) {
	r := newRig(t)
	r.key(input.KeyN, input.ModCtrl)
	r.typeText("Trip")
	r.key(input.KeyEnter, 0) // moves to the body
	openPaletteKey(r)
	r.key(input.KeyEscape, 0)
	r.typeText("x")
	if got := r.selected().Body.Peek(); got != "x" {
		t.Errorf("body after Escape and typing %q, want \"x\": focus did not return to the body", got)
	}
}

// Running a command from the palette runs it where the user was: the pin
// toggles, and the body has focus afterwards.
func TestPaletteRunRestoresBodyFocus(t *testing.T) {
	r := newRig(t)
	r.key(input.KeyN, input.ModCtrl)
	r.typeText("Trip")
	r.key(input.KeyEnter, 0)
	n := r.selected()
	pinned := n.Pinned.Peek()
	openPaletteKey(r)
	r.typeText("pin")
	r.key(input.KeyEnter, 0)
	if n.Pinned.Peek() == pinned {
		t.Error("running Pin or unpin note from the palette did not toggle the pin")
	}
	r.typeText("y")
	if got := n.Body.Peek(); got != "y" {
		t.Errorf("body after the palette ran a command and typing %q, want \"y\"", got)
	}
}

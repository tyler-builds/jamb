package views

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/input"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

// keyCenter returns the middle of the element with key.
func (r *rig) keyCenter(t *testing.T, key string) (float32, float32) {
	t.Helper()
	rect, ok := r.rt.Find(key)
	if !ok {
		t.Fatalf("no element with key %q", key)
	}
	return rect.X + rect.W/2, rect.Y + rect.H/2
}

func TestDraggingNoteOntoTagChipAddsTag(t *testing.T) {
	r := newRig(t)
	var n *store.Note
	for _, cand := range r.s.Notes.Peek() {
		if !slices.Contains(cand.Tags.Peek(), "home") {
			n = cand
			break
		}
	}
	if n == nil {
		t.Fatal("every seeded note has the home tag; the test needs one without it")
	}
	sx, sy := r.keyCenter(t, n.ID)
	cx, cy := r.keyCenter(t, "chip-home")
	r.rt.PointerMove(sx, sy, 0)
	r.frame()
	r.rt.PointerDown(sx, sy, input.ButtonLeft, 1, 0)
	r.rt.PointerMove(sx, sy+6, 0)
	r.frame()
	r.rt.PointerMove(cx, cy, 0)
	r.frame()
	r.rt.PointerUp(cx, cy, input.ButtonLeft, 0)
	r.frame()
	if !slices.Contains(n.Tags.Peek(), "home") {
		t.Errorf("tags after the drop = %v; want home added", n.Tags.Peek())
	}
}

func TestDroppedFilesImportOnlyTextNotes(t *testing.T) {
	r := newRig(t)
	dir := t.TempDir()
	md := filepath.Join(dir, "Lisbon trip.md")
	png := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(md, []byte("Packing list"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(png, []byte("not text"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := len(r.s.Notes.Peek())
	r.rt.DropFiles(600, 400, []string{md, png})
	r.frame()
	if got := len(r.s.Notes.Peek()); got != before+1 {
		t.Fatalf("notes after the drop = %d; want %d (the .md only)", got, before+1)
	}
	n := r.selected()
	if n == nil || n.Title.Peek() != "Lisbon trip" || n.Body.Peek() != "Packing list" {
		t.Errorf("imported note = %+v; want the file name without extension as title and its text as body", n)
	}
	if want := "Imported 1 note, skipped 1 file"; importNotice.Peek() != want {
		t.Errorf("status = %q; want %q", importNotice.Peek(), want)
	}
}

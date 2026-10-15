package views

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

type rig struct {
	t  *testing.T
	s  *store.Store
	rt *ui.Runtime
}

func newRig(t *testing.T) *rig {
	s, err := openStore(t, filepath.Join(t.TempDir(), "notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	rt := ui.NewRuntime(Root(s), ui.DarkTheme(), nil)
	rt.AddBindings(Bindings()...)
	rt.SetViewport(1100, 720, 1)
	r := &rig{t, s, rt}
	r.frame()
	return r
}

// openStore opens a store for a test. Its pending changes are saved before
// the test's temporary directories are removed (cleanups run last first), so
// the background saver can't recreate a directory afterwards.
func openStore(t *testing.T, path string) (*store.Store, error) {
	s, err := store.Open(path)
	if err == nil {
		t.Cleanup(func() { s.Flush() })
	}
	return s, err
}

func (r *rig) frame() { r.rt.Frame(time.Now()) }

// frameScene brings rt up to date and returns the scene it draws, for the tests
// that read the quads of a frame.
func frameScene(rt *ui.Runtime, now time.Time) *scene.Scene {
	return uihost.Of(rt).(interface{ Frame(time.Time) *scene.Scene }).Frame(now)
}

func (r *rig) key(k input.Key, mods input.Modifiers) {
	r.rt.KeyDown(k, mods, false)
	r.rt.KeyUp(k, mods)
	r.frame()
}

func (r *rig) typeText(s string) {
	for _, c := range s {
		r.rt.TextInput(string(c))
	}
	r.frame()
}

func (r *rig) click(x, y float32) {
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
}

func (r *rig) selected() *store.Note { return r.s.Find(r.s.Selected.Peek()) }

func TestNewNoteFocusesTitleThenBody(t *testing.T) {
	r := newRig(t)
	before := len(r.s.Notes.Peek())
	r.key(input.KeyN, input.ModCtrl)
	n := r.selected()
	if len(r.s.Notes.Peek()) != before+1 || n == nil || n.Title.Peek() != "" {
		t.Fatalf("Ctrl+N did not create and select an empty note")
	}
	r.typeText("Trip to Lisbon")
	r.key(input.KeyEnter, 0) // Enter in the title moves to the body
	r.typeText("Book tram 28.")
	r.key(input.KeyEnter, 0)
	r.typeText("Try pastéis de nata.")
	if n.Title.Peek() != "Trip to Lisbon" {
		t.Errorf("title = %q", n.Title.Peek())
	}
	if n.Body.Peek() != "Book tram 28.\nTry pastéis de nata." {
		t.Errorf("body = %q", n.Body.Peek())
	}
	if r.selected() != n {
		t.Errorf("selection moved to %q while typing", r.selected().Title.Peek())
	}
	if r.s.Notes.Peek()[1] != n { // first after the pinned welcome note
		t.Errorf("the edited note is not at the top of its section")
	}
}

func TestSearchFiltersAndEnterOpensMatch(t *testing.T) {
	r := newRig(t)
	r.key(input.KeyF, input.ModCtrl)
	r.typeText("groc")
	if r.s.Query.Peek() != "groc" {
		t.Fatalf("Ctrl+F did not focus search: query %q", r.s.Query.Peek())
	}
	r.key(input.KeyEnter, 0)
	if n := r.selected(); n == nil || n.Title.Peek() != "Groceries" {
		t.Fatalf("Enter in search should open Groceries")
	}
	// Focus moved to the body: typing edits the note.
	r.typeText("!")
	if !strings.HasPrefix(r.selected().Body.Peek(), "!") {
		t.Errorf("body after typing: %q", r.selected().Body.Peek())
	}
}

func TestAltArrowsAndDelete(t *testing.T) {
	r := newRig(t)
	first := r.selected()
	r.key(input.KeyDown, input.ModAlt)
	second := r.selected()
	if second == first {
		t.Fatal("Alt+Down did not move to the next note")
	}
	r.s.Delete(second)
	r.frame()
	if r.s.Find(second.ID) != nil || r.selected() == nil {
		t.Error("delete did not remove the note and select a neighbor")
	}
}

func TestEditsAreSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	s, err := openStore(t, path)
	if err != nil {
		t.Fatal(err)
	}
	n := s.Create()
	n.Title.Set("Persisted")
	s.Changed(n)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	s2, err := openStore(t, path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range s2.Notes.Peek() {
		found = found || m.Title.Peek() == "Persisted"
	}
	if !found {
		t.Error("note not found after reopening the file")
	}
}

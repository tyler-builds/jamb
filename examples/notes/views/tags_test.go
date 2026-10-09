package views

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/input"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

// clickKey clicks the element with the given key.
func (r *rig) clickKey(key string) {
	r.t.Helper()
	if err := r.rt.ClickKey(key); err != nil {
		r.t.Fatal(err)
	}
}

// shown reports whether an element with the given key is on screen.
func (r *rig) shown(key string) bool {
	_, ok := r.rt.Find(key)
	return ok
}

// listed reports whether a note's entry is in the sidebar list (entries
// are keyed by note ID).
func (r *rig) listed(title string) bool {
	for _, n := range r.s.Notes.Peek() {
		if n.Title.Peek() == title {
			return r.shown(n.ID)
		}
	}
	return false
}

func (r *rig) selectTitle(title string) *store.Note {
	r.t.Helper()
	for _, n := range r.s.Notes.Peek() {
		if n.Title.Peek() == title {
			r.s.Selected.Set(n.ID)
			r.frame()
			return n
		}
	}
	r.t.Fatalf("no note %q", title)
	return nil
}

// addTag clicks the "Add tag" field, types text and presses Enter.
func (r *rig) addTag(text string) {
	r.t.Helper()
	r.clickKey("add-tag")
	r.typeText(text)
	r.key(input.KeyEnter, 0)
}

func (r *rig) visibleTitles() []string {
	var out []string
	for _, n := range r.s.Notes.Peek() {
		if store.Visible(n, r.s.Query.Peek(), r.s.TagFilter.Peek()) {
			out = append(out, n.Title.Peek())
		}
	}
	return out
}

func TestSeededTags(t *testing.T) {
	r := newRig(t)
	n := r.selectTitle("Groceries")
	if !slices.Equal(n.Tags.Peek(), []string{"home"}) {
		t.Errorf("Groceries tags = %v", n.Tags.Peek())
	}
}

func TestAddTagWithFieldAndEnter(t *testing.T) {
	r := newRig(t)
	n := r.selectTitle("Groceries")
	body := n.Body.Peek()
	r.addTag("  Travel ")
	if !slices.Equal(n.Tags.Peek(), []string{"home", "travel"}) {
		t.Fatalf("tags = %v, want [home travel]", n.Tags.Peek())
	}
	// The field was cleared and keeps focus: the next tag goes straight in.
	r.typeText("HOME") // duplicate after lowercasing
	r.key(input.KeyEnter, 0)
	r.typeText("   ") // empty after trimming
	r.key(input.KeyEnter, 0)
	if !slices.Equal(n.Tags.Peek(), []string{"home", "travel"}) {
		t.Errorf("duplicate or empty tag was added: %v", n.Tags.Peek())
	}
	if !r.shown("pill-travel") || !r.shown("chip-travel") {
		t.Errorf("travel pill or chip missing")
	}
	if n.Body.Peek() != body || n.Title.Peek() != "Groceries" {
		t.Errorf("typing in the tag field leaked into the note: %q / %q", n.Title.Peek(), n.Body.Peek())
	}
}

func TestRemoveTagWithPillX(t *testing.T) {
	r := newRig(t)
	n := r.selectTitle("Ideas for the framework")
	r.clickKey("remove-jamb")
	if !slices.Equal(n.Tags.Peek(), []string{"work"}) {
		t.Fatalf("after clicking jamb's x: tags = %v", n.Tags.Peek())
	}
	// Removing the last use of the active filter's tag drops its chip and
	// clears the filter.
	r.clickKey("chip-work")
	if r.s.TagFilter.Peek() != "work" {
		t.Fatalf("filter = %q", r.s.TagFilter.Peek())
	}
	r.clickKey("remove-work")
	if len(n.Tags.Peek()) != 0 || r.s.TagFilter.Peek() != "" {
		t.Errorf("tags = %v, filter = %q", n.Tags.Peek(), r.s.TagFilter.Peek())
	}
	if r.shown("chip-work") {
		t.Error("the work chip is still shown")
	}
}

func TestTagChipFiltersList(t *testing.T) {
	r := newRig(t)
	r.clickKey("chip-jamb")
	if r.s.TagFilter.Peek() != "jamb" {
		t.Fatalf("filter = %q", r.s.TagFilter.Peek())
	}
	want := []string{"Welcome to Jamb Notes", "Languages", "Ideas for the framework"}
	if got := r.visibleTitles(); !slices.Equal(got, want) {
		t.Errorf("visible = %v, want %v", got, want)
	}
	if r.listed("Groceries") {
		t.Error("Groceries is still listed under the jamb filter")
	}
	// Combined with search.
	r.key(input.KeyF, input.ModCtrl)
	r.typeText("framework")
	if got := r.visibleTitles(); !slices.Equal(got, []string{"Ideas for the framework"}) {
		t.Errorf("jamb + search: visible = %v", got)
	}
	if r.listed("Languages") {
		t.Error("Languages is still listed under jamb + search")
	}
	r.s.Query.Set("")
	r.frame()
	// Clicking the active chip clears the filter.
	r.clickKey("chip-jamb")
	if r.s.TagFilter.Peek() != "" || len(r.visibleTitles()) != 4 {
		t.Errorf("filter = %q, visible = %v", r.s.TagFilter.Peek(), r.visibleTitles())
	}
	if !r.listed("Groceries") {
		t.Error("Groceries is not listed after clearing the filter")
	}
}

func TestTagChipsWrapInsideSidebar(t *testing.T) {
	r := newRig(t)
	for _, tag := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"} {
		r.addTag(tag)
	}
	first, _ := r.rt.Find("chip-alpha")
	last, ok := r.rt.Find("chip-foxtrot")
	if !ok {
		t.Fatal("foxtrot chip not shown")
	}
	if last.Y <= first.Y {
		t.Errorf("chips did not wrap onto another line (y %v vs %v)", last.Y, first.Y)
	}
	if last.Right() > 300 {
		t.Errorf("a chip extends to x=%v, past the 300px sidebar", last.Right())
	}
}

func TestTagsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	s, err := openStore(t, path)
	if err != nil {
		t.Fatal(err)
	}
	n := s.Create()
	n.Title.Set("Lisbon")
	s.AddTag(n, "Travel")
	s.AddTag(n, "work")
	s.RemoveTag(n, "work")
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	s2, err := openStore(t, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range s2.Notes.Peek() {
		if m.Title.Peek() == "Lisbon" {
			if !slices.Equal(m.Tags.Peek(), []string{"travel"}) {
				t.Errorf("tags after reopening = %v", m.Tags.Peek())
			}
			return
		}
	}
	t.Error("note not found after reopening the file")
}

func TestOldFileWithoutTagsLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	old := `[{"id":"a","title":"Old","body":"x","created":"2025-01-01T00:00:00Z","updated":"2025-01-01T00:00:00Z"}]`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := openStore(t, path)
	if err != nil {
		t.Fatal(err)
	}
	if n := s.Find("a"); n == nil || len(n.Tags.Peek()) != 0 || len(s.AllTags()) != 0 {
		t.Errorf("old file did not load cleanly")
	}
}

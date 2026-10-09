// Package store holds the notes, keeps them ordered and saves them to disk.
//
// Every note field is its own signal, so typing in a note re-renders only
// the views that show that note. The list order is a separate signal that
// changes only when a note actually moves.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// Note is one note. Its fields are signals; read them with Get in render
// functions and Peek elsewhere.
type Note struct {
	ID      string
	Created time.Time
	Title   *ui.Signal[string]
	Body    *ui.Signal[string]
	Updated *ui.Signal[time.Time]
	Pinned  *ui.Signal[bool]
	Tags    *ui.Signal[[]string] // normalized (lowercase), no duplicates; never mutated in place
}

// SaveState describes the persistence status shown in the sidebar.
type SaveState struct {
	Saving bool
	Err    string
	At     time.Time
}

// Store is the application state.
type Store struct {
	// Notes in display order: pinned first, then most recently edited.
	Notes    *ui.Signal[[]*Note]
	Selected *ui.Signal[string] // note ID
	Query    *ui.Signal[string]
	// TagFilter limits the note list to notes with this tag ("" = all).
	TagFilter *ui.Signal[string]
	Save      *ui.Signal[SaveState]

	path    string
	nextID  int
	mu      sync.Mutex
	pending bool
	kick    chan struct{}
}

type fileNote struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	Pinned  bool      `json:"pinned,omitempty"`
	Tags    []string  `json:"tags,omitempty"`
}

// DefaultPath is where notes are kept: $JAMB_NOTES_FILE, or the user's
// config directory.
func DefaultPath() string {
	if p := os.Getenv("JAMB_NOTES_FILE"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	} else if err := migrateLegacyDir(dir); err != nil {
		fmt.Fprintln(os.Stderr, "jamb-notes: cannot move the goui-notes directory:", err)
	}
	return filepath.Join(dir, dirName, "notes.json")
}

// The notes directory was named goui-notes before the project became jamb.
// migrateLegacyDir renames it to jamb-notes, so the notes, settings and keymap
// carry over. It does nothing when jamb-notes already exists or goui-notes
// does not.
const (
	dirName       = "jamb-notes"
	legacyDirName = "goui-notes"
)

func migrateLegacyDir(configDir string) error {
	dst := filepath.Join(configDir, dirName)
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	src := filepath.Join(configDir, legacyDirName)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// Open loads notes from path (seeding a few on first run) and starts the
// background saver.
func Open(path string) (*Store, error) {
	s := &Store{
		Notes:     ui.NewSignal[[]*Note](nil),
		Selected:  ui.NewSignal(""),
		Query:     ui.NewSignal(""),
		TagFilter: ui.NewSignal(""),
		Save:      ui.NewSignal(SaveState{}),
		path:      path,
		kick:      make(chan struct{}, 1),
	}
	var notes []*Note
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		var file []fileNote
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("notes file %s is not valid JSON (%v); move it aside to start fresh", path, err)
		}
		for _, f := range file {
			notes = append(notes, s.newNote(f))
		}
	case os.IsNotExist(err):
		for _, f := range seedNotes() {
			notes = append(notes, s.newNote(f))
		}
		s.pending = true
	default:
		return nil, fmt.Errorf("cannot read notes file %s: %v", path, err)
	}
	s.Notes.Set(sorted(notes))
	if len(notes) > 0 {
		s.Selected.Set(s.Notes.Peek()[0].ID)
	}
	go s.saver()
	if s.pending {
		s.kickSave()
	}
	return s, nil
}

func (s *Store) newNote(f fileNote) *Note {
	if f.ID == "" {
		s.nextID++
		f.ID = fmt.Sprintf("n%d-%d", time.Now().UnixNano(), s.nextID)
	}
	return &Note{
		ID:      f.ID,
		Created: f.Created,
		Title:   ui.NewSignal(f.Title),
		Body:    ui.NewSignal(f.Body),
		Updated: ui.NewSignal(f.Updated),
		Pinned:  ui.NewSignal(f.Pinned),
		Tags:    ui.NewSignal(normalizeTags(f.Tags)),
	}
}

func sorted(notes []*Note) []*Note {
	out := slices.Clone(notes)
	slices.SortStableFunc(out, func(a, b *Note) int {
		if pa, pb := a.Pinned.Peek(), b.Pinned.Peek(); pa != pb {
			if pa {
				return -1
			}
			return 1
		}
		return b.Updated.Peek().Compare(a.Updated.Peek())
	})
	return out
}

// Find returns the note with the given ID, or nil.
func (s *Store) Find(id string) *Note {
	for _, n := range s.Notes.Peek() {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// Create adds an empty note at the top and selects it.
func (s *Store) Create() *Note {
	now := time.Now()
	n := s.newNote(fileNote{Created: now, Updated: now})
	s.Notes.Set(sorted(append(s.Notes.Peek(), n)))
	s.Selected.Set(n.ID)
	s.Query.Set("")
	s.Changed(n)
	return n
}

// Import adds a note with the given title and body at the top, without
// selecting it, and schedules a save. Files dropped into the app use it.
func (s *Store) Import(title, body string) *Note {
	now := time.Now()
	n := s.newNote(fileNote{Title: title, Body: body, Created: now, Updated: now})
	s.Notes.Set(sorted(append(s.Notes.Peek(), n)))
	s.kickSave()
	return n
}

// Changed records an edit: it bumps the note's time, moves it up if it is
// not already first in its group, and schedules a save.
func (s *Store) Changed(n *Note) {
	n.Updated.Set(time.Now())
	notes := s.Notes.Peek()
	if re := sorted(notes); !slices.Equal(re, notes) {
		s.Notes.Set(re)
	}
	s.kickSave()
}

// TogglePin pins or unpins a note.
func (s *Store) TogglePin(n *Note) {
	n.Pinned.Set(!n.Pinned.Peek())
	s.Notes.Set(sorted(s.Notes.Peek()))
	s.kickSave()
}

// Delete removes a note and selects its neighbor.
func (s *Store) Delete(n *Note) {
	notes := s.Notes.Peek()
	i := slices.Index(notes, n)
	if i < 0 {
		return
	}
	notes = slices.Delete(slices.Clone(notes), i, i+1)
	s.Notes.Set(notes)
	if s.Selected.Peek() == n.ID {
		next := ""
		if len(notes) > 0 {
			next = notes[min(i, len(notes)-1)].ID
		}
		s.Selected.Set(next)
	}
	s.pruneTagFilter()
	s.kickSave()
}

// ---- tags ----

// NormalizeTag trims and lowercases a tag; "" means it is not a valid tag.
func NormalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

func normalizeTags(tags []string) []string {
	var out []string
	for _, t := range tags {
		if t = NormalizeTag(t); t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// AddTag adds a tag to a note. It returns false if the tag is empty or the
// note already has it.
func (s *Store) AddTag(n *Note, tag string) bool {
	tag = NormalizeTag(tag)
	tags := n.Tags.Peek()
	if tag == "" || slices.Contains(tags, tag) {
		return false
	}
	n.Tags.Set(append(slices.Clone(tags), tag))
	s.Changed(n)
	return true
}

// RemoveTag removes a tag from a note. If no note has the tag any more and
// it was the active filter, the filter is cleared.
func (s *Store) RemoveTag(n *Note, tag string) {
	tags := n.Tags.Peek()
	i := slices.Index(tags, tag)
	if i < 0 {
		return
	}
	n.Tags.Set(slices.Delete(slices.Clone(tags), i, i+1))
	s.pruneTagFilter()
	s.Changed(n)
}

// ToggleTagFilter makes tag the active filter, or clears it if it already is.
func (s *Store) ToggleTagFilter(tag string) {
	if s.TagFilter.Peek() == tag {
		tag = ""
	}
	s.TagFilter.Set(tag)
}

// AllTags returns every tag in use, sorted. It reads the notes' tag signals
// with Get, so a view calling it updates when tags change.
func (s *Store) AllTags() []string {
	var all []string
	for _, n := range s.Notes.Get() {
		for _, t := range n.Tags.Get() {
			if !slices.Contains(all, t) {
				all = append(all, t)
			}
		}
	}
	slices.Sort(all)
	return all
}

func (s *Store) pruneTagFilter() {
	f := s.TagFilter.Peek()
	if f == "" {
		return
	}
	for _, n := range s.Notes.Peek() {
		if slices.Contains(n.Tags.Peek(), f) {
			return
		}
	}
	s.TagFilter.Set("")
}

// Visible reports whether a note matches both the search query and the tag
// filter ("" = any). Like Matches, it subscribes the caller to the note.
func Visible(n *Note, q, tag string) bool {
	if tag != "" && !slices.Contains(n.Tags.Get(), tag) {
		return false
	}
	return Matches(n, q)
}

// Matches reports whether a note matches the search query. It reads the
// note's signals with Get, so a filtered list updates as notes change.
func Matches(n *Note, q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(n.Title.Get()), q) || strings.Contains(strings.ToLower(n.Body.Get()), q)
}

// ---- saving ----

func (s *Store) kickSave() {
	s.mu.Lock()
	s.pending = true
	s.mu.Unlock()
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// saver runs on its own goroutine: it waits for edits to pause for 400 ms,
// then writes the file atomically. It updates the Save signal from this
// goroutine, which the UI picks up on its next frame.
func (s *Store) saver() {
	for range s.kick {
		for {
			select {
			case <-s.kick:
				continue // still typing
			case <-time.After(400 * time.Millisecond):
			}
			break
		}
		s.mu.Lock()
		if !s.pending {
			s.mu.Unlock()
			continue
		}
		s.pending = false
		s.mu.Unlock()
		s.Save.Set(SaveState{Saving: true, At: s.Save.Peek().At})
		err := s.write()
		st := SaveState{At: time.Now()}
		if err != nil {
			st.Err = err.Error()
		}
		s.Save.Set(st)
	}
}

// Flush writes pending changes immediately (call before exiting).
func (s *Store) Flush() error {
	s.mu.Lock()
	pending := s.pending
	s.pending = false
	s.mu.Unlock()
	if !pending {
		return nil
	}
	return s.write()
}

func (s *Store) write() error {
	notes := s.Notes.Peek()
	file := make([]fileNote, len(notes))
	for i, n := range notes {
		file[i] = fileNote{
			ID: n.ID, Title: n.Title.Peek(), Body: n.Body.Peek(),
			Created: n.Created, Updated: n.Updated.Peek(), Pinned: n.Pinned.Peek(),
			Tags: n.Tags.Peek(),
		}
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

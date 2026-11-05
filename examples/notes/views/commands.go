package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/examples/notes/store"
	"github.com/tyler-builds/jamb/ui"
)

// NoteInfo is a note as the commands return it.
type NoteInfo struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Body    string    `json:"body,omitempty" desc:"Only from notes.get"`
	Tags    []string  `json:"tags"`
	Pinned  bool      `json:"pinned"`
	Updated time.Time `json:"updated"`
}

func infoOf(n *store.Note, body bool) NoteInfo {
	i := NoteInfo{ID: n.ID, Title: n.Title.Peek(), Tags: n.Tags.Peek(), Pinned: n.Pinned.Peek(), Updated: n.Updated.Peek()}
	if i.Tags == nil {
		i.Tags = []string{}
	}
	if body {
		i.Body = n.Body.Peek()
	}
	return i
}

// ListIn filters notes.list.
type ListIn struct {
	Query string `json:"query,omitempty" desc:"Only the notes whose title or body contains this, in any case"`
	Tag   string `json:"tag,omitempty" desc:"Only the notes with this tag"`
}

// NoteID names a note.
type NoteID struct {
	ID string `json:"id" desc:"The note's id, from notes.list"`
}

// CreateIn is a new note.
type CreateIn struct {
	Title string   `json:"title"`
	Body  string   `json:"body,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

// UpdateIn changes a note: the fields given replace the note's.
type UpdateIn struct {
	ID    string    `json:"id"`
	Title *string   `json:"title,omitempty"`
	Body  *string   `json:"body,omitempty"`
	Tags  *[]string `json:"tags,omitempty" desc:"Replaces all the tags"`
}

// RegisterCommands registers the commands that coding agents and scripts run
// through the control channel (jamb ctl run). Call it once.
func RegisterCommands(s *store.Store) {
	find := func(id string) (*store.Note, error) {
		if n := s.Find(id); n != nil {
			return n, nil
		}
		return nil, fmt.Errorf("there is no note %q", id)
	}
	ui.NewCommand("notes.list", "List the notes, pinned first and then the most recently edited, without their bodies",
		func(in ListIn) ([]NoteInfo, error) {
			q := strings.ToLower(in.Query)
			tag := store.NormalizeTag(in.Tag)
			out := []NoteInfo{}
			for _, n := range s.Notes.Peek() {
				if q != "" && !strings.Contains(strings.ToLower(n.Title.Peek()+"\n"+n.Body.Peek()), q) {
					continue
				}
				if tag != "" && !containsTag(n.Tags.Peek(), tag) {
					continue
				}
				out = append(out, infoOf(n, false))
			}
			return out, nil
		})
	ui.NewCommand("notes.get", "Get a note, with its body", func(in NoteID) (NoteInfo, error) {
		n, err := find(in.ID)
		if err != nil {
			return NoteInfo{}, err
		}
		return infoOf(n, true), nil
	})
	ui.NewCommand("notes.create", "Create a note and show it in the editor", func(in CreateIn) (NoteInfo, error) {
		n := s.Create()
		n.Title.Set(in.Title)
		n.Body.Set(in.Body)
		for _, t := range in.Tags {
			s.AddTag(n, t)
		}
		s.Changed(n)
		s.Selected.Set(n.ID)
		return infoOf(n, true), nil
	})
	ui.NewCommand("notes.update", "Change a note's title, body or tags", func(in UpdateIn) (NoteInfo, error) {
		n, err := find(in.ID)
		if err != nil {
			return NoteInfo{}, err
		}
		if in.Title != nil {
			n.Title.Set(*in.Title)
		}
		if in.Body != nil {
			n.Body.Set(*in.Body)
		}
		if in.Tags != nil {
			for _, t := range n.Tags.Peek() {
				s.RemoveTag(n, t)
			}
			for _, t := range *in.Tags {
				s.AddTag(n, t)
			}
		}
		s.Changed(n)
		return infoOf(n, true), nil
	})
	ui.NewCommand("notes.delete", "Delete a note", func(in NoteID) (struct{}, error) {
		n, err := find(in.ID)
		if err != nil {
			return struct{}{}, err
		}
		s.Delete(n)
		return struct{}{}, nil
	})
	ui.NewCommand("notes.select", "Show a note in the editor", func(in NoteID) (struct{}, error) {
		if _, err := find(in.ID); err != nil {
			return struct{}{}, err
		}
		s.Selected.Set(in.ID)
		return struct{}{}, nil
	})
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

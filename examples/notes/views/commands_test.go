package views

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/examples/notes/store"
	"github.com/tyler-builds/jamb/ui"
)

func TestCommands(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	RegisterCommands(s)
	run := func(name, args string) any {
		t.Helper()
		out, err := ui.CommandNamed(name).RunJSON([]byte(args))
		if err != nil {
			t.Fatalf("%s %s: %v", name, args, err)
		}
		return out
	}
	created := run("notes.create", `{"title":"Agent note","body":"milk, eggs","tags":["Home"]}`).(NoteInfo)
	if s.Selected.Peek() != created.ID || created.Tags[0] != "home" {
		t.Fatalf("create: %+v, selected %q", created, s.Selected.Peek())
	}
	list := run("notes.list", `{"query":"EGGS"}`).([]NoteInfo)
	if len(list) != 1 || list[0].ID != created.ID || list[0].Body != "" {
		t.Fatalf("list by query: %+v", list)
	}
	if got := run("notes.list", `{"tag":"home"}`).([]NoteInfo); len(got) == 0 {
		t.Error("list by tag found nothing")
	}
	run("notes.update", `{"id":"`+created.ID+`","body":"oat milk","tags":[]}`)
	got := run("notes.get", `{"id":"`+created.ID+`"}`).(NoteInfo)
	if got.Body != "oat milk" || got.Title != "Agent note" || len(got.Tags) != 0 {
		t.Errorf("after update: %+v", got)
	}
	run("notes.delete", `{"id":"`+created.ID+`"}`)
	if _, err := ui.CommandNamed("notes.get").RunJSON([]byte(`{"id":"` + created.ID + `"}`)); err == nil || !strings.Contains(err.Error(), "no note") {
		t.Errorf("get after delete: %v", err)
	}
}

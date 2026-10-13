package views

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// seededRig opens a store with n notes whose snippets vary in length.
func seededRig(t *testing.T, n int) *rig {
	t.Helper()
	notes := make([]map[string]any, n)
	for i := range n {
		notes[i] = map[string]any{
			"id":      fmt.Sprintf("n%05d", i),
			"title":   fmt.Sprintf("Note %d", i),
			"body":    strings.Repeat(fmt.Sprintf("Line %d of a note with a few words. ", i), i%23),
			"created": "2026-01-01T00:00:00Z",
			"updated": time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(time.RFC3339),
		}
	}
	data, err := json.Marshal(notes)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "notes.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := openStore(t, path)
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

// subtreeNodes counts the nodes under the element with the given key, read
// from the tree dump (each line is one node, indented by depth).
func subtreeNodes(r *rig, key string) int {
	var b strings.Builder
	r.rt.Dump(&b, 0)
	depth, count := -1, 0
	for _, line := range strings.Split(b.String(), "\n") {
		if line == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if depth < 0 {
			if strings.Contains(line, "key="+key+"  [") {
				depth = indent
				count = 1
			}
			continue
		}
		if indent <= depth {
			break
		}
		count++
	}
	return count
}

// settle runs frames at 60 fps until the scroll animations have ended.
func settle(r *rig, now time.Time) time.Time {
	for range 60 {
		now = now.Add(16 * time.Millisecond)
		r.rt.Frame(now)
	}
	return now
}

func TestSidebarBuildsOnlyVisibleRows(t *testing.T) {
	r := seededRig(t, 2000)
	if got := len(r.s.Notes.Peek()); got != 2000 {
		t.Fatalf("%d notes, want 2000", got)
	}
	n := subtreeNodes(r, "sidebar")
	t.Logf("%d nodes in the sidebar's note list for 2,000 notes", n)
	if n == 0 || n >= 200 {
		t.Errorf("%d nodes in the sidebar's note list for 2,000 notes, want fewer than 200", n)
	}
}

func TestAltDownKeepsSelectedNoteVisible(t *testing.T) {
	r := seededRig(t, 2000)
	now := time.Now()
	want := r.s.Notes.Peek()[30].ID // the notes are sorted by last edit: 30 steps down from the first
	for i := range 30 {
		r.rt.KeyDown(input.KeyDown, input.ModAlt, false)
		r.rt.KeyUp(input.KeyDown, input.ModAlt)
		now = settle(r, now)
		id := r.s.Selected.Peek()
		list, ok := r.rt.Find("sidebar")
		if !ok {
			t.Fatal("sidebar list not found")
		}
		row, ok := r.rt.Find(id)
		if !ok {
			t.Fatalf("step %d: selected note %s is not built", i, id)
		}
		if row.Y < list.Y || row.Y+row.H > list.Y+list.H {
			t.Fatalf("step %d: selected row %+v is outside the list %+v", i, row, list)
		}
	}
	if got := r.s.Selected.Peek(); got != want {
		t.Errorf("selected %s after 30 steps down, want %s", got, want)
	}
}

package views

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// TestNotesAccessibility checks the names and roles that the notes app exposes
// to assistive technology, and that it produces no development warnings. The
// store is a temporary file; the user's notes are never touched.
func TestNotesAccessibility(t *testing.T) {
	s, err := openStore(t, filepath.Join(t.TempDir(), "notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	rt := ui.NewRuntime(Root(s), ui.DarkTheme(), nil)
	rt.AddBindings(Bindings()...)
	var warnings []string
	rt.SetWarnFunc(func(msg string) { warnings = append(warnings, msg) })
	rt.SetAccessWarnings(true)
	rt.SetViewport(1100, 720, 1)
	now := time.Unix(1_000_000, 0)
	frame := func() {
		now = now.Add(time.Second)
		rt.Frame(now)
	}
	frame()

	tr := rt.AccessTree()
	accesstest.Find(t, tr, access.RoleTextField, "Search notes")
	accesstest.Find(t, tr, access.RoleButton, "New note")

	// Creating a note selects it and shows its editor.
	rt.KeyDown(input.KeyN, input.ModCtrl, false)
	rt.KeyUp(input.KeyN, input.ModCtrl)
	frame()
	frame()
	tr = rt.AccessTree()
	accesstest.Find(t, tr, access.RoleTextField, "Title")
	accesstest.Find(t, tr, access.RoleTextArea, "Body")
	accesstest.Find(t, tr, access.RoleButton, "Pin note")
	accesstest.Find(t, tr, access.RoleButton, "Delete note")
	accesstest.Find(t, tr, access.RoleButton, "Toggle theme")

	if len(warnings) != 0 {
		t.Fatalf("the notes app produced accessibility warnings:\n%v", warnings)
	}
}

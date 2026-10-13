package views

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/examples/notes/store"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// TestNotesControlsReportTheirStates checks the sidebar's selection, the pin
// toggle and the tag filter, and that the actions on them change the app. Each
// announcement the app makes is checked on the tree that carries it.
func TestNotesControlsReportTheirStates(t *testing.T) {
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
	bridge := rt.AccessBridge()
	bridge.Activate()
	var updates []access.Update
	bridge.Listen(func(u access.Update) { updates = append(updates, u) })
	announced := func(text string) bool {
		for _, u := range updates {
			for _, a := range u.Announcements {
				if a.Text == text {
					return true
				}
			}
		}
		return false
	}

	// Creating a note selects it: the sidebar's list has exactly one selected row.
	rt.KeyDown(input.KeyN, input.ModCtrl, false)
	rt.KeyUp(input.KeyN, input.ModCtrl)
	frame()
	tr := bridge.Tree()
	if n := countNodes(tr, access.RoleListItem, func(n *access.Node) bool { return n.States.Has(access.StateSelected) }); n != 1 {
		t.Fatalf("%d sidebar rows are selected after a note was created, want 1", n)
	}
	if n := countNodes(tr, access.RoleList, nil); n != 1 {
		t.Fatalf("%d lists, want the notes sidebar", n)
	}
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Pin note"), access.RoleButton, "Pin note",
		access.StateToggle)

	// The pin is a toggle: pressing it through the bridge pins the note and reports it.
	pin := accesstest.Find(t, tr, access.RoleButton, "Pin note")
	bridge.Perform(pin.ID, access.ActionToggle)
	frame()
	if n := s.Find(s.Selected.Peek()); n == nil || !n.Pinned.Get() {
		t.Fatal("toggling the pin did not pin the selected note")
	}
	accesstest.Check(t, accesstest.Find(t, bridge.Tree(), access.RoleButton, "Pin note"), access.RoleButton,
		"Pin note", access.StateToggle|access.StateChecked)

	// Tag chips are a selection group: choosing one filters the sidebar.
	note := s.Find(s.Selected.Peek())
	s.AddTag(note, "work")
	frame()
	tr = bridge.Tree()
	accesstest.Find(t, tr, access.RoleGroup, "Filter by tag")
	chip := accesstest.Find(t, tr, access.RoleButton, "work")
	if !chip.States.Has(access.StateSelectable) || chip.States.Has(access.StateSelected) {
		t.Fatalf("the unchosen tag chip has states [%s]", chip.States)
	}
	bridge.Perform(chip.ID, access.ActionSelect)
	frame()
	if s.TagFilter.Get() != "work" {
		t.Fatalf("selecting the chip set the tag filter to %q", s.TagFilter.Get())
	}
	accesstest.Check(t, accesstest.Find(t, bridge.Tree(), access.RoleButton, "work"), access.RoleButton, "work",
		access.StateSelected)

	// The save status is announced once per save that finishes.
	s.Save.Set(store.SaveState{At: now})
	frame()
	if !announced("Saved") {
		t.Fatal("a finished save was not announced")
	}
	before := len(updates)
	frame()
	for _, u := range updates[before:] {
		for _, a := range u.Announcements {
			if a.Text == "Saved" {
				t.Fatal("the same save was announced twice")
			}
		}
	}

	// A notice from an import or an export is announced.
	showNotice("Imported 2 notes")
	frame()
	if !announced("Imported 2 notes") {
		t.Fatal("the import result was not announced")
	}

	// Deleting a note through its confirmation announces it.
	bridge.Perform(accesstest.Find(t, bridge.Tree(), access.RoleButton, "Delete note").ID, access.ActionClick)
	frame()
	bridge.Perform(accesstest.Find(t, bridge.Tree(), access.RoleButton, "Delete").ID, access.ActionClick)
	frame()
	if !announced("Note deleted") {
		t.Fatal("deleting a note was not announced")
	}

	if len(warnings) != 0 {
		t.Fatalf("the notes app produced accessibility warnings:\n%v", warnings)
	}
}

// countNodes returns the number of nodes with the role for which keep returns true.
func countNodes(t *access.Tree, role access.Role, keep func(*access.Node) bool) int {
	n := 0
	t.Walk(func(node *access.Node) bool {
		if node.Role == role && (keep == nil || keep(node)) {
			n++
		}
		return true
	})
	return n
}

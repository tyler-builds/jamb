package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// appRig shows the whole demo window for a fresh store.
func appRig(t *testing.T) (*Store, *ui.Runtime) {
	t.Helper()
	s := NewStore()
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 900, 1)
	rt.Frame(time.Now())
	return s, rt
}

// openNewProject clicks the header's New Project button, and checks that it
// opened the form and added nothing yet.
func openNewProject(t *testing.T, s *Store, rt *ui.Runtime) {
	t.Helper()
	before := len(s.Projects.Peek())
	if err := rt.ClickKey("new-project"); err != nil {
		t.Fatal(err)
	}
	if !s.NewOpen.Peek() {
		t.Fatal("New Project did not open the form")
	}
	if n := len(s.Projects.Peek()); n != before {
		t.Fatalf("opening the form added %d project(s); want none until Create", n-before)
	}
}

// New Project opens a form. Create adds the project with the trimmed name, puts
// it first, selects it and shows the Projects page. An empty description gets
// the default text.
func TestNewProjectFormCreatesProject(t *testing.T) {
	s, rt := appRig(t)
	before := len(s.Projects.Peek())
	openNewProject(t, s, rt)
	rt.Type("  Apollo  ")
	rt.Frame(time.Now())
	if err := rt.ClickKey("project-create"); err != nil {
		t.Fatal(err)
	}
	ps := s.Projects.Peek()
	if len(ps) != before+1 || ps[0].Name != "Apollo" {
		t.Fatalf("after Create: %d projects, first %q; want %d, Apollo", len(ps), ps[0].Name, before+1)
	}
	if ps[0].Description != defaultDescription {
		t.Errorf("description %q, want the default", ps[0].Description)
	}
	if s.Selected.Peek() != ps[0].ID || s.Page.Peek() != PageProjects || s.Filter.Peek() != -1 {
		t.Errorf("after Create: selected %d, page %v, filter %d; want the new project on Projects with no filter",
			s.Selected.Peek(), s.Page.Peek(), s.Filter.Peek())
	}
	if s.NewOpen.Peek() {
		t.Error("the form is still open after Create")
	}
}

// Enter in the name field runs Create, and Create is disabled while the name is
// blank, so Enter does nothing then.
func TestNewProjectEnterCreatesOnlyWithAName(t *testing.T) {
	s, rt := appRig(t)
	before := len(s.Projects.Peek())
	openNewProject(t, s, rt)
	rt.Press(input.KeyEnter, 0)
	if len(s.Projects.Peek()) != before || !s.NewOpen.Peek() {
		t.Fatal("Enter with a blank name created a project or closed the form")
	}
	rt.Type("Gamma")
	rt.Press(input.KeyEnter, 0)
	if len(s.Projects.Peek()) != before+1 || s.NewOpen.Peek() {
		t.Errorf("Enter with a name: %d projects (was %d), form open %v", len(s.Projects.Peek()), before, s.NewOpen.Peek())
	}
}

// Tab moves to the description, whose text is kept with the project.
func TestNewProjectKeepsDescription(t *testing.T) {
	s, rt := appRig(t)
	openNewProject(t, s, rt)
	rt.Type("Delta")
	rt.Press(input.KeyTab, 0)
	rt.Type("Scratch notes\nsecond line")
	if err := rt.ClickKey("project-create"); err != nil {
		t.Fatal(err)
	}
	if ps := s.Projects.Peek(); ps[0].Description != "Scratch notes\nsecond line" {
		t.Errorf("description %q", ps[0].Description)
	}
}

// Cancel, Escape and a press on the backdrop close the form without adding a
// project.
func TestNewProjectCloseWithoutCreating(t *testing.T) {
	s, rt := appRig(t)
	before := len(s.Projects.Peek())

	openNewProject(t, s, rt)
	if err := rt.ClickKey("project-cancel"); err != nil {
		t.Fatal(err)
	}
	if s.NewOpen.Peek() {
		t.Error("Cancel did not close the form")
	}

	openNewProject(t, s, rt)
	rt.Type("Epsilon")
	rt.Press(input.KeyEscape, 0)
	if s.NewOpen.Peek() {
		t.Error("Escape did not close the form")
	}

	openNewProject(t, s, rt)
	rt.PointerMove(5, 5, 0)
	rt.PointerDown(5, 5, input.ButtonLeft, 1, 0)
	rt.PointerUp(5, 5, input.ButtonLeft, 0)
	rt.Frame(time.Now())
	if s.NewOpen.Peek() {
		t.Error("a press on the backdrop did not close the form")
	}
	if n := len(s.Projects.Peek()); n != before {
		t.Errorf("closing the form added %d project(s)", n-before)
	}
}

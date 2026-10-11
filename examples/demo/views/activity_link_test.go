package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// Clicking the project name in an Activity row opens Projects with that
// project selected.
func TestActivityProjectLinkOpensProjectSelected(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageActivity)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	row, ok := rt.Find("event-1")
	if !ok {
		t.Fatal("the first event is not on screen")
	}
	// The link is somewhere in the row's text; sweep for it. Plain text does
	// nothing, so only the link opens a project.
	y := row.Y + row.H/2
	for x := row.X + 60; x < row.X+row.W-60 && s.Page.Get() == PageActivity; x += 3 {
		rt.PointerMove(x, y, 0)
		rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
		rt.PointerUp(x, y, input.ButtonLeft, 0)
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
	if s.Page.Get() != PageProjects {
		t.Fatal("no point in the first event's row opened Projects")
	}
	var want Project
	for _, p := range s.Projects.Get() {
		if p.Name == events[0].what {
			want = p
		}
	}
	if want.ID == 0 {
		t.Fatalf("no project named %q", events[0].what)
	}
	if got := s.Selected.Get(); got != want.ID {
		t.Errorf("selected project = %d, want %d (%s)", got, want.ID, want.Name)
	}
}

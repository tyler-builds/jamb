package views

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// A project window and the projects page are two runtimes that read the same
// store. Starring in one must re-render the other.
func TestStarInProjectWindowUpdatesProjectsPage(t *testing.T) {
	s := NewStore()
	id := s.Projects.Peek()[1].ID // Project Beta, not starred
	if p, _ := s.Project(id); p.Starred {
		t.Fatal("the test needs a project that starts unstarred")
	}

	page := ui.NewRuntime(func(ctx *ui.Context) ui.Element { return Projects(s) }, DemoDarkTheme, nil)
	page.SetViewport(1200, 900, 1)
	page.Frame(time.Now())

	detailRuns := 0
	detail := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		detailRuns++
		return ProjectDetail(s, id)(ctx)
	}, DemoDarkTheme, nil)
	detail.SetViewport(640, 420, 1)
	detail.Frame(time.Now())

	// The star button of the project window toggles the shared store.
	s.ToggleStar(id)
	if !page.NeedsFrame() || !detail.NeedsFrame() {
		t.Fatalf("starring did not wake both windows (page=%v detail=%v)", page.NeedsFrame(), detail.NeedsFrame())
	}
	page.Frame(time.Now())
	detail.Frame(time.Now())
	if detailRuns != 2 {
		t.Errorf("the project window rendered %d times, want 2", detailRuns)
	}
	if p, _ := s.Project(id); !p.Starred {
		t.Error("the store does not show the star")
	}
}

// A project that is gone shows a message instead of crashing its window.
func TestProjectDetailWithoutProject(t *testing.T) {
	s := NewStore()
	rt := ui.NewRuntime(ProjectDetail(s, -1), DemoDarkTheme, nil)
	rt.SetViewport(640, 420, 1)
	rt.Frame(time.Now())
}

// The main window marks a project that has a window of its own, and the mark
// goes when that window closes.
func TestOpenWindowBadgeFollowsTheWindow(t *testing.T) {
	s := NewStore()
	id := s.Projects.Peek()[0].ID
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())
	dump := func() string {
		var b strings.Builder
		rt.Dump(&b, 40)
		return b.String()
	}
	if strings.Contains(dump(), "Open in window") {
		t.Fatal("a project shows an open window before any was opened")
	}
	OpenProject(s, id)
	rt.Frame(time.Now())
	if !strings.Contains(dump(), "Open in window") {
		t.Fatal("the project does not show that its window is open")
	}
	s.windowClosed(id)
	rt.Frame(time.Now())
	if strings.Contains(dump(), "Open in window") {
		t.Error("the badge is still shown after the window closed")
	}
}

// Opening a project whose window is open focuses that window instead of
// opening a second one.
func TestOpenProjectFocusesItsOpenWindow(t *testing.T) {
	s := NewStore()
	id := s.Projects.Peek()[1].ID
	open := &openWindow{} // not closed: the window is open
	s.details[id] = open
	OpenProject(s, id)
	if s.details[id] != open {
		t.Error("opening a project with an open window replaced its window")
	}
	if !open.focused {
		t.Error("opening a project with an open window did not focus it")
	}
}

// openWindow is an open window that records Focus. Its other methods are not
// called by the test.
type openWindow struct {
	ui.Window
	focused bool
}

func (w *openWindow) Closed() bool                          { return false }
func (w *openWindow) Show()                                 {}
func (w *openWindow) Hide()                                 {}
func (w *openWindow) SetTaskbarProgress(ui.TaskbarProgress) {}
func (w *openWindow) SetTaskbarBadge(image.Image, string)   {}
func (w *openWindow) Focus()                                { w.focused = true }

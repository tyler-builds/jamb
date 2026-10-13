package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/ui"
)

// focusRings counts the focus-ring quads (hollow borders in the theme's
// FocusRing color) in a frame.
func focusRings(sc *scene.Scene) int {
	n := 0
	for _, q := range sc.Quads {
		if q.Background.A == 0 && q.BorderWidth > 0 && q.BorderColor == scene.Color(ui.DarkTheme().FocusRing) {
			n++
		}
	}
	return n
}

// Tab walks the sidebar navigation, a focused item shows a ring, and Enter opens
// its page. A mouse click removes the ring.
func TestSidebarNavigatesByKeyboard(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageDashboard)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())

	rt.Press(input.KeyTab, 0) // Dashboard, the first item
	if n := focusRings(rt.Frame(time.Now())); n != 1 {
		t.Errorf("after Tab: %d focus rings, want 1", n)
	}
	rt.Press(input.KeyTab, 0) // Projects
	rt.Press(input.KeyEnter, 0)
	if s.Page.Get() != PageProjects {
		t.Errorf("page after Tab, Tab, Enter = %v, want projects", s.Page.Get())
	}

	rt.Press(input.KeyTab, 0) // Activity
	rt.Press(input.KeyTab, 0) // Components
	rt.Press(input.KeyTab, 0) // Settings
	rt.Press(input.KeySpace, 0)
	if s.Page.Get() != PageSettings {
		t.Errorf("page after Space on Settings = %v, want settings", s.Page.Get())
	}

	if err := rt.ClickKey("nav-Dashboard"); err != nil {
		t.Fatal(err)
	}
	if n := focusRings(rt.Frame(time.Now())); n != 0 {
		t.Errorf("after a click: %d focus rings, want 0", n)
	}
	if s.Page.Get() != PageDashboard {
		t.Errorf("page after clicking Dashboard = %v", s.Page.Get())
	}
}

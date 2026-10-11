package views

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/scene"
	"github.com/tyler-builds/jamb/ui"
)

// shortcutRig is the main window with its key bindings, drawn once.
func shortcutRig(s *Store) *ui.Runtime {
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.AddBindings(Bindings()...)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())
	return rt
}

// Ctrl+1 to Ctrl+5 switch to the pages in the sidebar's order.
func TestShortcutsSwitchPages(t *testing.T) {
	s := NewStore()
	rt := shortcutRig(s)
	pages := []Page{PageDashboard, PageProjects, PageActivity, PageComponents, PageSettings}
	for i, want := range pages {
		rt.Press(input.Key1+input.Key(i), input.ModCtrl)
		if got := s.Page.Get(); got != want {
			t.Errorf("Ctrl+%d: page %v, want %v", i+1, got, want)
		}
	}
}

// Ctrl+Shift+L switches between the dark and light themes.
func TestShortcutTogglesTheme(t *testing.T) {
	s := NewStore()
	rt := shortcutRig(s)
	rt.Press(input.KeyL, input.ModCtrl|input.ModShift)
	if rt.Theme() != DemoLightTheme {
		t.Error("Ctrl+Shift+L did not switch to the light theme")
	}
	rt.Press(input.KeyL, input.ModCtrl|input.ModShift)
	if rt.Theme() != DemoDarkTheme {
		t.Error("a second Ctrl+Shift+L did not switch back to the dark theme")
	}
}

// Ctrl+K moves the keyboard focus to the header's search field.
func TestShortcutFocusesSearch(t *testing.T) {
	s := NewStore()
	rt := shortcutRig(s)
	rt.Press(input.KeyK, input.ModCtrl)
	if !s.Search.Focused() {
		t.Error("Ctrl+K did not focus the search field")
	}
}

// The Settings page lists the shortcuts with their keys.
func TestSettingsListsShortcuts(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageSettings)
	rt := shortcutRig(s)
	// The shortcuts are in their own tab, which is shown by a click on it.
	if err := rt.ClickKey("settings-tabs/tab:shortcuts"); err != nil {
		t.Fatal(err)
	}
	rt.Frame(time.Now())
	var b strings.Builder
	rt.Dump(&b, 40)
	for _, want := range []string{"Keyboard shortcuts", "Go to Projects", "Ctrl+2", "Toggle theme", "Ctrl+Shift+L"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the Settings page does not show %q", want)
		}
	}
}

// Searching highlights the matches in the project names and descriptions.
func TestProjectsHighlightSearchMatches(t *testing.T) {
	s := NewStore()
	rt := shortcutRig(s)
	soft := scene.Color(matchHighlight(ui.DarkTheme).Background)
	count := func() int {
		n := 0
		for _, q := range rt.Frame(time.Now()).Quads {
			if q.Background == soft {
				n++
			}
		}
		return n
	}
	base := count()
	s.Query.Set("renderer")
	rt.Frame(time.Now())
	if got := count(); got <= base {
		t.Errorf("searching for renderer painted %d highlight quads, want more than the %d before", got, base)
	}
}

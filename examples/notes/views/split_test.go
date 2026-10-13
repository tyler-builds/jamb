package views

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

// splitWindow returns a window of the notes app over a store in dir, and its
// sidebar width signal.
func splitWindow(t *testing.T, dir string, width float64) (*ui.Runtime, *ui.Signal[float64], func()) {
	t.Helper()
	s, err := openStore(t, filepath.Join(dir, "notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	sidebar := ui.NewSignal(width)
	rt := ui.NewRuntime(RootWith(s, Layout{Sidebar: sidebar}), ui.DarkTheme, nil)
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
	rt.AccessBridge().Activate()
	frame()
	frame() // the split learns its length one frame after its first render
	t.Cleanup(func() {
		if len(warnings) != 0 {
			t.Errorf("the window printed development warnings: %v", warnings)
		}
	})
	return rt, sidebar, frame
}

func TestSidebarDividerIsAnAccessibleSplitter(t *testing.T) {
	rt, _, _ := splitWindow(t, t.TempDir(), DefaultSidebarWidth)
	sep := accesstest.Find(t, rt.AccessTree(), access.RoleSeparator, "Resize sidebar")
	if sep.Range == nil {
		t.Fatal("the divider has no RangeValue")
	}
	if sep.Range.Value != DefaultSidebarWidth {
		t.Fatalf("the divider is at %v, want the sidebar's default %d", sep.Range.Value, DefaultSidebarWidth)
	}
	if !sep.States.Has(access.StateFocusable) {
		t.Fatal("the divider is not focusable")
	}
}

// TestSidebarWidthIsKeptAcrossRestarts sets the sidebar's width through the
// divider's RangeValue, as an assistive technology does, saves it the way main
// does, and reads it back into a new window of the same notes file.
func TestSidebarWidthIsKeptAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	rt, sidebar, frame := splitWindow(t, dir, DefaultSidebarWidth)
	sep := accesstest.Find(t, rt.AccessTree(), access.RoleSeparator, "Resize sidebar")
	rt.AccessBridge().Do(sep.ID, access.Request{Action: access.ActionSetRangeValue, X: 360})
	frame()
	if sidebar.Peek() != 360 {
		t.Fatalf("setting the divider to 360 left the sidebar at %v", sidebar.Peek())
	}

	settingsPath := store.SettingsPath(filepath.Join(dir, "notes.json"))
	if err := store.SaveSettings(settingsPath, store.Settings{SidebarWidth: sidebar.Peek()}); err != nil {
		t.Fatal(err)
	}
	got := store.LoadSettings(settingsPath).SidebarWidth
	if got != 360 {
		t.Fatalf("the saved width is %v, want 360", got)
	}
	// A new window with the loaded width starts with the divider there.
	rt2, _, _ := splitWindow(t, dir, got)
	if sep2 := accesstest.Find(t, rt2.AccessTree(), access.RoleSeparator, "Resize sidebar"); sep2.Range.Value != 360 {
		t.Fatalf("the restored divider is at %v, want 360", sep2.Range.Value)
	}
}

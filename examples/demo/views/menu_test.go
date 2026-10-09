package views

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// rightClickAt presses the right button at (x, y) and lets the frame run.
func rightClickAt(rt *ui.Runtime, x, y float32) {
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonRight, 1, 0)
	rt.PointerUp(x, y, input.ButtonRight, 0)
	rt.Frame(time.Now())
}

// clickAt presses and releases the left button at (x, y) and lets the frame run.
func clickAt(rt *ui.Runtime, x, y float32) {
	rt.PointerMove(x, y, 0)
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	rt.Frame(time.Now())
}

// center returns the middle of the element with key.
func center(t *testing.T, rt *ui.Runtime, key string) (float32, float32) {
	t.Helper()
	r, ok := rt.Find(key)
	if !ok {
		t.Fatalf("no element with key %q", key)
	}
	return r.X + r.W/2, r.Y + r.H/2
}

// clickKey presses and releases the left button in the middle of the element
// with key.
func clickKey(t *testing.T, rt *ui.Runtime, key string) {
	t.Helper()
	x, y := center(t, rt, key)
	clickAt(rt, x, y)
}

// hoverOpen moves the pointer onto the element with key and waits out the
// submenu delay, so the submenu of that row opens.
func hoverOpen(t *testing.T, rt *ui.Runtime, key string) {
	t.Helper()
	x, y := center(t, rt, key)
	rt.PointerMove(x, y, 0)
	rt.Frame(time.Now().Add(time.Second))
}

// The card's menu stars the project, and its Status submenu sets the status.
func TestProjectCardMenuStarsAndSetsStatus(t *testing.T) {
	s, rt := projectRig(t)
	var id int
	for _, p := range s.Visible() {
		if p.Name == "Project Beta" {
			id = p.ID
		}
	}
	x, y := center(t, rt, fmt.Sprint(id))
	rightClickAt(rt, x, y)
	if _, ok := rt.Find("jamb.menu"); !ok {
		t.Fatal("right-click on the card did not open its menu")
	}
	clickKey(t, rt, "jamb.menu.0.2") // Star
	if p, _ := s.Project(id); !p.Starred {
		t.Error("Star did not star the project")
	}

	rightClickAt(rt, x, y)
	hoverOpen(t, rt, "jamb.menu.0.3") // Status
	if _, ok := rt.Find("jamb.menu.1.1"); !ok {
		t.Fatal("hovering Status did not open its submenu")
	}
	clickKey(t, rt, "jamb.menu.1.1") // Paused
	if p, _ := s.Project(id); p.Status != StatusPaused {
		t.Errorf("status = %v, want paused", p.Status)
	}
	if _, ok := rt.Find("jamb.menu"); ok {
		t.Error("the menu is still open after choosing a status")
	}
}

// The dropdown and the context menu on the Components page show the demo's own
// shortcuts, and choosing a submenu item in the context menu closes it.
func TestComponentsMenusShowBindingShortcuts(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageComponents)
	rt := ui.NewRuntime(App(s), ui.DarkTheme, nil)
	rt.AddBindings(Bindings()...)
	rt.SetViewport(1200, 3000, 1) // the whole page, so every sample is on screen
	rt.Frame(time.Now())

	clickKey(t, rt, "project-actions")
	if _, ok := rt.Find("jamb.menu"); !ok {
		t.Fatal("the dropdown did not open")
	}
	var dump strings.Builder
	rt.Dump(&dump, 96)
	for _, want := range []string{"Ctrl+2", "Ctrl+K", "Ctrl+Shift+L"} {
		if !strings.Contains(dump.String(), want) {
			t.Errorf("dropdown does not show %s", want)
		}
	}
	rt.Press(input.KeyEscape, 0)

	x, y := center(t, rt, "context-menu-sample")
	rightClickAt(rt, x, y)
	hoverOpen(t, rt, "jamb.menu.0.3") // Layout
	if _, ok := rt.Find("jamb.menu.1.1"); !ok {
		t.Fatal("hovering Layout did not open its submenu")
	}
	clickKey(t, rt, "jamb.menu.1.1") // List
	if _, ok := rt.Find("jamb.menu"); ok {
		t.Error("choosing a layout did not close the context menu")
	}
}

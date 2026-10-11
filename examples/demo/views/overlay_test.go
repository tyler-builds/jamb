package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

func TestUserMenu(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageDashboard)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())

	if err := rt.ClickKey("user-card"); err != nil {
		t.Fatal(err)
	}
	card, ok := rt.Find("user-card")
	if !ok {
		t.Fatal("no user card")
	}
	menu, ok := rt.Find("user-menu")
	if !ok {
		t.Fatal("menu did not open")
	}
	// The menu enters from a few pixels below its place; let it settle first.
	for i := 0; i < 100 && rt.NeedsFrame(); i++ {
		time.Sleep(10 * time.Millisecond)
		rt.Frame(time.Now())
	}
	menu, _ = rt.Find("user-menu")
	if menu.Y+menu.H > card.Y || menu.Y < 0 || menu.X < 0 || menu.X+menu.W > 1200 {
		t.Errorf("menu %+v should sit above the card %+v inside the window", menu, card)
	}
	if err := rt.ClickKey("menu-settings"); err != nil {
		t.Fatal(err)
	}
	if s.Page.Get() != PageSettings {
		t.Errorf("page = %v, want settings", s.Page.Get())
	}
	if _, ok := rt.Find("user-menu"); ok {
		t.Error("menu still open after choosing Settings")
	}
}

// Escape closes the user menu, which dismisses on an outside click too.
func TestUserMenuClosesOnEscape(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageDashboard)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())

	if err := rt.ClickKey("user-card"); err != nil {
		t.Fatal(err)
	}
	if _, ok := rt.Find("user-menu"); !ok {
		t.Fatal("menu did not open")
	}
	rt.Press(input.KeyEscape, 0)
	if _, ok := rt.Find("user-menu"); ok {
		t.Error("menu still open after Escape")
	}
}

// Escape closes a placement popover of the Components page.
func TestComponentsPopoverClosesOnEscape(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageComponents)
	rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())

	if err := rt.ClickKey("place-BelowStart"); err != nil {
		t.Fatal(err)
	}
	if _, ok := rt.Find("popover"); !ok {
		t.Fatal("popover did not open")
	}
	rt.Press(input.KeyEscape, 0)
	if _, ok := rt.Find("popover"); ok {
		t.Error("popover still open after Escape")
	}
}

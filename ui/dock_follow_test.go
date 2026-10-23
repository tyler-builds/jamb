package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// floatRig shows a dock with one floating group, and the runtime of the group's window.
type floatRig struct {
	main, float *accessRig
	win         DockWindow
	ran         int // how many times the main window's action ran
}

var testFollowAction = NewAction("test.follow", "Follow")

func newFloatRig(t *testing.T) *floatRig {
	t.Helper()
	f := &floatRig{}
	var opened []DockWindow
	open := func(w DockWindow) Window {
		opened = append(opened, w)
		return &dockTestWindow{x: w.X, y: w.Y, w: w.Width, h: w.Height}
	}
	sig := NewSignal(dockSplitLayout().Float("tree", 30, 40, 300, 200))
	f.main = newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).OnAction(testFollowAction, func() { f.ran++ }).
			Child(Dock(sig, testDockPanels()).Key("dock").Floating(open))
	})
	f.main.rt.AddBindings(Bind("ctrl+j", testFollowAction))
	f.main.frame()
	if len(opened) != 1 {
		t.Fatalf("%d windows opened, want one for the floating group", len(opened))
	}
	f.win = opened[0]
	f.float = newAccessRig(f.win.Root)
	return f
}

// TestDockFloatingWindowTakesTheLookOfItsDock checks that the window of a floating group
// shows the theme, text and zoom of the dock's window, and keeps them in step both ways.
func TestDockFloatingWindowTakesTheLookOfItsDock(t *testing.T) {
	custom := LightTheme()
	custom.Name = "Custom"
	f := newFloatRig(t)
	main, float := f.main.rt, f.float.rt

	if float.Theme() != main.Theme() {
		t.Errorf("the floating window's theme is %q, want the dock window's %q", float.Theme().Name, main.Theme().Name)
	}
	main.SetTheme(custom)
	if float.Theme() != custom {
		t.Errorf("after the dock window's SetTheme the floating window has %q, want Custom", float.Theme().Name)
	}
	other := DarkTheme()
	other.Name = "Other"
	float.SetTheme(other)
	if main.Theme() != other || float.Theme() != other {
		t.Errorf("a SetTheme in the floating window gave %q and %q, want Other in both", main.Theme().Name, float.Theme().Name)
	}

	var zooms []float32
	main.hostOnZoom(func(z float32) { zooms = append(zooms, z) })
	main.SetZoom(1.25)
	if float.Zoom() != 1.25 {
		t.Errorf("the floating window's zoom is %v, want the dock window's 1.25", float.Zoom())
	}
	float.ZoomIn()
	if main.Zoom() != 1.5 || float.Zoom() != 1.5 {
		t.Errorf("a zoom in the floating window gave %v and %v, want 1.5 in both", main.Zoom(), float.Zoom())
	}
	if !slices.Equal(zooms, []float32{1.25, 1.5}) {
		t.Errorf("the dock window's OnZoom heard %v, want [1.25 1.5]", zooms)
	}

	s := EnglishStrings()
	s.OK = "Fine"
	float.SetStrings(s)
	if main.strings.OK != "Fine" || float.strings.OK != "Fine" {
		t.Errorf("SetStrings in the floating window gave %q and %q, want Fine in both", main.strings.OK, float.strings.OK)
	}
}

// TestDockFloatingWindowUsesTheBindingsAndActionsOfItsDock checks that a key bound in the
// dock's window runs there from the floating window, through an action that only the dock's
// window handles, and that the action is listed as available in the floating window.
func TestDockFloatingWindowUsesTheBindingsAndActionsOfItsDock(t *testing.T) {
	f := newFloatRig(t)
	f.float.press(input.KeyJ, input.ModCtrl)
	if f.ran != 1 {
		t.Errorf("Ctrl+J in the floating window ran the dock window's action %d times, want 1", f.ran)
	}
	if !slices.Contains(f.float.rt.AvailableActions(), testFollowAction) {
		t.Errorf("the floating window does not list the dock window's action")
	}
	if got := f.float.rt.KeysFor(testFollowAction); got != "Ctrl+J" {
		t.Errorf("KeysFor in the floating window = %q, want Ctrl+J", got)
	}
	if !f.float.rt.Dispatch(testFollowAction) || f.ran != 2 {
		t.Errorf("Dispatch in the floating window did not reach the dock window's handler")
	}
}

// TestDockFloatingWindowLetsGoWhenItCloses checks that a closed floating window no longer
// follows the dock's window.
func TestDockFloatingWindowLetsGoWhenItCloses(t *testing.T) {
	f := newFloatRig(t)
	main, float := f.main.rt, f.float.rt
	before := float.Theme()
	float.hostClose()
	if len(main.followers) != 0 {
		t.Fatalf("the dock window still has %d followers", len(main.followers))
	}
	custom := LightTheme()
	custom.Name = "Custom"
	main.SetTheme(custom)
	if float.Theme() != before {
		t.Errorf("a closed floating window took the dock window's new theme")
	}
	f.float.press(input.KeyJ, input.ModCtrl)
	if f.ran != 0 {
		t.Errorf("a closed floating window still ran the dock window's action")
	}
}

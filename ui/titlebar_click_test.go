package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// In a window whose title bar the app draws, the platform runs the title-bar buttons'
// pointer clicks, so the runtime does not run their handlers as well: that would minimize
// twice, toggle maximize back, or close as the app instead of the user (which skips
// CloseToTray). Assistive technology still invokes them, and Close asks for the user's
// close.
func TestTitleBarButtonsLeavePointerClicksToThePlatform(t *testing.T) {
	win := &fakeWindow{}
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Row().Child(Div().Grow(1)).Child(TitleBarButtons())
	})
	r.rt.SetWindow(win)
	r.rt.nativeCaption = true
	r.frame()
	for _, k := range []string{"Minimize", "Maximize", "Close"} {
		rc, ok := r.rt.Find(k)
		if !ok {
			t.Fatalf("no %s button", k)
		}
		x, y := rc.X+rc.W/2, rc.Y+rc.H/2
		r.rt.PointerMove(x, y, 0)
		r.frame()
		r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
		r.frame()
		r.rt.PointerUp(x, y, input.ButtonLeft, 0)
		r.frame()
	}
	if len(win.calls) != 0 {
		t.Fatalf("pointer clicks on the title-bar buttons ran %v; the platform runs them", win.calls)
	}
	close := accesstest.Find(t, r.tree(), access.RoleButton, "Close")
	r.bridge.Perform(close.ID, access.ActionClick)
	r.frame()
	if len(win.calls) != 1 || win.calls[0] != "request close" {
		t.Fatalf("invoking Close ran %v, want the user's close (request close)", win.calls)
	}
}

// The title-bar buttons are the platform's areas although they have an OnClick for
// assistive technology, so the platform presses them and Windows can show Snap Layouts
// over maximize; a control inside a drag area still takes its own input.
func TestTitleBarButtonAreasAreThePlatforms(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Row().WindowArea(WindowAreaDrag).
			Child(Button("Search").Key("search")).
			Child(Div().Grow(1)).
			Child(TitleBarButtons())
	})
	r.rt.SetWindow(&fakeWindow{})
	r.frame()
	for key, want := range map[string]WindowArea{
		"Minimize": WindowAreaMinimize, "Maximize": WindowAreaMaximize, "Close": WindowAreaClose,
		"search": WindowAreaNone,
	} {
		rc, ok := r.rt.Find(key)
		if !ok {
			t.Fatalf("no %s", key)
		}
		if got := r.rt.hostWindowAreaAt(rc.X+rc.W/2, rc.Y+rc.H/2); got != want {
			t.Errorf("%s: area %d, want %d", key, got, want)
		}
	}
}

// The title-bar buttons take the height of their bar, so in a tall bar they reach
// its top and bottom edges, even in a bar that centers its children. A bar sized
// by its content (bar 0 here) gets buttons 32 px high.
func TestTitleBarButtonsFillTheBar(t *testing.T) {
	for _, c := range []struct{ bar, want float32 }{{44, 44}, {32, 32}, {0, 32}} {
		r := newAccessRig(func(ctx *Context) Element {
			bar := Div().Row().Align(Center).Child(Div().Grow(1)).Child(TitleBarButtons())
			if c.bar > 0 {
				bar.Height(Px(c.bar))
			}
			return Div().Column().Child(bar)
		})
		for _, k := range []string{"Minimize", "Maximize", "Close"} {
			rc, ok := r.rt.Find(k)
			if !ok {
				t.Fatalf("no %s button", k)
			}
			if rc.Y != 0 || rc.H != c.want || rc.W != 46 {
				t.Errorf("in a %v px bar, %s is %vx%v at y %v, want 46x%v at y 0", c.bar, k, rc.W, rc.H, rc.Y, c.want)
			}
		}
	}
}

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

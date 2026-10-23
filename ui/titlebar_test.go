package ui

import (
	"math"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// TestTitleBarCentersOnTheWindow checks that the centered element sits in the middle
// of the bar's whole width though the sides differ, and that it never covers a side.
func TestTitleBarCentersOnTheWindow(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(
			TitleBar().Buttons(false).Key("bar").
				Child(Div().Key("left").Width(Px(120)).Height(Px(20))).
				Trailing(Div().Key("right").Width(Px(200)).Height(Px(20))).
				Center(Div().Key("mid").Width(Px(80)).Height(Px(20))))
	}, 600, 400)
	bar, left, mid, right := r.find(t, "bar"), r.find(t, "left"), r.find(t, "mid"), r.find(t, "right")
	if bar.W != 600 {
		t.Fatalf("bar %+v, want the window's width", bar)
	}
	if c := mid.X + mid.W/2; math.Abs(float64(c-300)) > 0.5 {
		t.Errorf("centered element's middle at %v, want 300 (the window's middle)", c)
	}
	if left.X+left.W > mid.X || mid.X+mid.W > right.X {
		t.Errorf("centered element %+v covers a side: left %+v, right %+v", mid, left, right)
	}
}

// TestTitleBarMovesTheCenterWhenASideGrows checks that a change of a side's width,
// made outside the bar, moves the room of the centered element: the bar renders
// again after the layout that measured the side.
func TestTitleBarMovesTheCenterWhenASideGrows(t *testing.T) {
	side := NewSignal(120.0)
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(
			TitleBar().Buttons(false).Key("bar").
				Child(Component(func(ctx *Context) Element {
					return Div().Key("left").Width(Px(float32(side.Get()))).Height(Px(20))
				})).
				Center(Div().Key("mid").Width(Px(40)).Height(Px(20))))
	}, 600, 400)
	if x := r.find(t, "titlebar.center").X; x != 120 {
		t.Fatalf("centered room starts at %v, want 120, the start side's width", x)
	}
	side.Set(200)
	r.frame()
	room := r.find(t, "titlebar.center")
	if room.X != 200 || room.W != 200 {
		t.Errorf("centered room %+v, want x 200 and width 200 after the side grew to 200", room)
	}
}

// TestTitleBarButtonsAreOptional checks that the caption buttons are at the end of
// the bar by default and are left out by Buttons(false).
func TestTitleBarButtonsAreOptional(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(TitleBar().Key("shown").Child(Text("Shown")))
	})
	accesstest.Find(t, r.tree(), access.RoleButton, "Close")

	r2 := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(TitleBar().Buttons(false).Child(Text("Hidden")))
	})
	if _, ok := r2.rt.Find("Close"); ok {
		t.Error("a title bar with Buttons(false) has a Close button")
	}
}

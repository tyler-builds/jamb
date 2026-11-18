package uitest_test

import (
	"fmt"
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// pad is a 200 by 100 box keyed "pad" at 20, 10 that logs its presses: the point in
// the box and the modifiers held.
func pad(log *[]string) func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		return ui.Div().PaddingTop(10).PaddingLeft(20).Child(
			ui.Div().Key("pad").Label("Pad").Size(ui.Px(200), ui.Px(100)).
				OnMouseDown(func(e *ui.MouseEvent) {
					*log = append(*log, fmt.Sprintf("%.0f,%.0f mods=%d", e.LocalX, e.LocalY, e.Mods))
				}))
	}
}

// ClickWith clicks the center with the modifiers held; ClickAt clicks a point in the
// element, and UI.ClickAt a point in the window. Each runs a frame that a recording
// marks as following an input, as Click's does.
func TestClickWithModifiersAndAtAPoint(t *testing.T) {
	var log []string
	u := uitest.New(t, pad(&log), uitest.Size(300, 200))
	rec := u.Record()
	u.Get("pad").ClickWith(input.ModCtrl)
	u.Get("pad").ClickAt(5, 7)
	u.Get("pad").ClickAt(150, 90, input.ModShift, input.ModCtrl)
	u.ClickAt(30, 20, input.ModAlt)
	rec.Stop()
	want := []string{
		fmt.Sprintf("100,50 mods=%d", input.ModCtrl),
		"5,7 mods=0",
		fmt.Sprintf("150,90 mods=%d", input.ModShift|input.ModCtrl),
		fmt.Sprintf("10,10 mods=%d", input.ModAlt),
	}
	if fmt.Sprint(log) != fmt.Sprint(want) {
		t.Errorf("presses = %q, want %q", log, want)
	}
	inputs := 0
	for _, f := range rec.Frames() {
		if f.Input {
			inputs++
		}
	}
	if inputs != 4 {
		t.Errorf("%d recorded frames follow an input, want 4", inputs)
	}
}

// The points are in the logical pixels of Bounds, at any zoom.
func TestClickAtFollowsZoom(t *testing.T) {
	var log []string
	u := uitest.New(t, pad(&log), uitest.Size(600, 400))
	u.Runtime().SetZoom(1.5)
	u.Frame()
	u.Get("pad").ClickAt(40, 30)
	u.ClickAt(25, 15)
	if want := []string{"40,30 mods=0", "5,5 mods=0"}; fmt.Sprint(log) != fmt.Sprint(want) {
		t.Errorf("presses at zoom 1.5 = %q, want %q", log, want)
	}
}

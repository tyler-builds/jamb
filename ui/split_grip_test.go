package ui

import "testing"

// The grip is drawn always by default, only while the divider is hovered with SplitGripOnHover,
// and never with SplitGripHidden; the bar is drawn in every mode.
func TestSplitGripModes(t *testing.T) {
	for _, tc := range []struct {
		name           string
		grip           SplitGrip
		idle, hovering bool
	}{
		{"shown", SplitGripShown, true, true},
		{"on hover", SplitGripOnHover, false, true},
		{"hidden", SplitGripHidden, false, false},
	} {
		r := splitRig(t, func() Element {
			return Split(Pane(Text("Left")), Pane(Text("Right"))).Grip(tc.grip)
		})
		r.frame()
		if got := partNodeOrNil(r.rt, "Split.Grip") != nil; got != tc.idle {
			t.Errorf("%s: grip drawn while idle %v, want %v", tc.name, got, tc.idle)
		}
		if partNodeOrNil(r.rt, "Split.Divider") == nil {
			t.Errorf("%s: no bar", tc.name)
		}
		hoverNode(r, handleNode(t, r))
		r.frame()
		if got := partNodeOrNil(r.rt, "Split.Grip") != nil; got != tc.hovering {
			t.Errorf("%s: grip drawn while hovered %v, want %v", tc.name, got, tc.hovering)
		}
		r.rt.PointerMove(-100, -100, 0)
		r.frame()
		focusNode(r, handleNode(t, r))
		if got := partNodeOrNil(r.rt, "Split.Grip") != nil; got != tc.hovering {
			t.Errorf("%s: grip drawn with keyboard focus %v, want %v", tc.name, got, tc.hovering)
		}
	}
}

// The divider's bar runs the divider's whole length in both orientations, so the split
// draws a line between its panes; the divider centers the bar, so a bar with only a
// thickness had no length and drew nothing.
func TestSplitBarRunsTheDividerLength(t *testing.T) {
	for _, o := range []SplitOrientation{SplitHorizontal, SplitVertical} {
		r := splitRig(t, func() Element { return Split(Pane(Text("Left")), Pane(Text("Right"))).Orientation(o) })
		r.frame()
		bar, hit := r.rt.nodeRect(partNodeOrNil(r.rt, "Split.Divider")), r.rt.nodeRect(handleNode(t, r))
		thick, length, want := bar.W, bar.H, hit.H
		if o == SplitVertical {
			thick, length, want = bar.H, bar.W, hit.W
		}
		if thick != 1 || length != want {
			t.Errorf("orientation %d: the bar is %v thick and %v long, want 1 and %v", o, thick, length, want)
		}
	}
}

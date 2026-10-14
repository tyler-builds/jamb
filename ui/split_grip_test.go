package ui

import "testing"

// The grip is drawn always by default, only while the divider is hovered with GripOnHover,
// and never with GripHidden; the bar is drawn in every mode.
func TestSplitGripModes(t *testing.T) {
	for _, tc := range []struct {
		name           string
		grip           SplitGrip
		idle, hovering bool
	}{
		{"shown", GripShown, true, true},
		{"on hover", GripOnHover, false, true},
		{"hidden", GripHidden, false, false},
	} {
		r := splitRig(t, func() Element {
			return Split(Text("Left"), Text("Right")).Grip(tc.grip)
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

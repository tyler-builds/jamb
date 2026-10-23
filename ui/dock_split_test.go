package ui

import (
	"math"
	"testing"
)

// TestDockThreeChildrenShareOneSplit checks that a split node with three children is one split of three
// panes, with a divider between each two. A drag of the first divider moves the space between its two
// children only, stores their shares, and saves once at the release.
func TestDockThreeChildrenShareOneSplit(t *testing.T) {
	saves := 0
	sig := NewSignal(DockLayout{Root: sized(false, []float64{1, 2, 1}, grp("notes", "notes"), grp("log", "log"), grp("tree", "tree"))})
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).Child(Dock(sig, testDockPanels()).Key("dock").
			OnChange(func(DockLayout) { saves++ }))
	})
	r.frame()
	handles := partNodes(r.rt, "Split.Handle")
	if len(handles) != 2 {
		t.Fatalf("three children have %d dividers, want 2", len(handles))
	}
	if len(partNodes(r.rt, "Split.Pane")) != 3 {
		t.Fatalf("three children have %d panes, want 3", len(partNodes(r.rt, "Split.Pane")))
	}
	// The dock is 500 px wide, so the panes share the 498 px less the two dividers as 1:2:1.
	checkSizes(t, "widths before the drag", paneSizes(r, false), []float64{124.5, 249, 124.5})

	dragDivider(r, handlesOf(r, false)[0], 40, 0)
	if saves != 1 {
		t.Errorf("OnChange ran %d times at the end of the drag, want 1 at once", saves)
	}
	checkSizes(t, "widths after the drag", paneSizes(r, false), []float64{164.5, 209, 124.5})
	got := sig.Peek().Root.Sizes
	if len(got) != 3 {
		t.Fatalf("the stored shares are %v, want three", got)
	}
	total := 0.0
	for _, s := range got {
		total += s
	}
	if math.Abs(total-1) > 1e-9 {
		t.Errorf("the stored shares %v sum to %v, want 1", got, total)
	}
	if math.Abs(got[0]-164.5/498) > 0.001 || math.Abs(got[1]-209.0/498) > 0.001 || math.Abs(got[2]-0.25) > 0.001 {
		t.Errorf("the stored shares are %v, want %v, %v and 0.25", got, 164.5/498, 209.0/498)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

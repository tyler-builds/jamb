package ui

import "testing"

// TestCollapsibleHeaderCornersFollowTheSection checks that the header's hover fill
// does not cover the section's border at a rounded corner: the header takes the
// section's inner corners on its top, and on its bottom too while the section is
// closed, when the header is the section's bottom edge.
func TestCollapsibleHeaderCornersFollowTheSection(t *testing.T) {
	open := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Collapsible("Sync", open, Text("Body"))
	})
	sec := partNode(t, r.rt, "Collapsible.Section", "")
	want := innerCorners(sec.style.radii(), sec.style.borderEdges())
	if want.TopLeft == 0 {
		t.Fatal("the section has no rounded corner to follow")
	}
	head := partNode(t, r.rt, "Collapsible.Header", "").style.radii()
	if head.TopLeft != want.TopLeft || head.TopRight != want.TopRight ||
		head.BottomLeft != want.BottomLeft || head.BottomRight != want.BottomRight {
		t.Errorf("closed: the header's corners are %+v, want the section's inner corners %+v", head, want)
	}
	open.Set(true)
	r.frame()
	head = partNode(t, r.rt, "Collapsible.Header", "").style.radii()
	if head.TopLeft != want.TopLeft || head.TopRight != want.TopRight || head.BottomLeft != 0 || head.BottomRight != 0 {
		t.Errorf("open: the header's corners are %+v, want the top corners %+v and square bottom ones", head, want)
	}
}

package layout

import "testing"

// expectInk checks n's ink rectangle, to within float32 rounding.
func expectInk(t *testing.T, n *Node, x0, y0, x1, y1 float32) {
	t.Helper()
	near := func(a, b float32) bool { return a-b < 1e-3 && b-a < 1e-3 }
	if !near(n.InkX0, x0) || !near(n.InkY0, y0) || !near(n.InkX1, x1) || !near(n.InkY1, y1) {
		t.Errorf("ink (%v,%v)-(%v,%v), want (%v,%v)-(%v,%v)",
			n.InkX0, n.InkY0, n.InkX1, n.InkY1, x0, y0, x1, y1)
	}
}

func TestInkOfPlainNesting(t *testing.T) {
	inner := box(Style{Width: Px(20), Height: Px(10)})
	root := box(Style{Padding: Edges{5, 5, 5, 5}}, box(Style{Width: Px(100), Height: Px(50)}, inner))
	var e Engine
	e.Compute(root, 200, 200)
	// Nothing draws past its box, so the root's ink is its own box.
	expectInk(t, root, 0, 0, 200, 200)
	expectInk(t, root.Children[0], 0, 0, 100, 50)
}

func TestInkIncludesAbsoluteChildOutsideParent(t *testing.T) {
	badge := box(Style{Position: Absolute, Top: Px(-30), Left: Px(80), Width: Px(40), Height: Px(40)})
	card := box(Style{Width: Px(100), Height: Px(100)}, badge)
	root := box(Style{}, card)
	var e Engine
	e.Compute(root, 300, 300)
	// The badge reaches 30 px above the card and 20 px past its right edge.
	expectInk(t, card, 0, -30, 120, 100)
}

func TestInkClippingParentLimitsChildren(t *testing.T) {
	badge := box(Style{Position: Absolute, Top: Px(-30), Left: Px(80), Width: Px(40), Height: Px(40)})
	card := box(Style{Width: Px(100), Height: Px(100), OverflowY: OverflowHidden}, badge)
	root := box(Style{}, card)
	var e Engine
	e.Compute(root, 300, 300)
	// The card clips vertically, so the badge's overflow above it is cut off.
	// Horizontally the card does not clip, so the badge still counts.
	expectInk(t, card, 0, 0, 120, 100)
}

func TestInkClippedChildContributesItsBox(t *testing.T) {
	clipper := box(Style{Width: Px(100), Height: Px(100), OverflowX: OverflowHidden},
		box(Style{Position: Absolute, Left: Px(-50), Top: Px(0), Width: Px(40), Height: Px(40)}))
	root := box(Style{Padding: Edges{0, 0, 0, 0}}, clipper)
	var e Engine
	e.Compute(root, 300, 300)
	// The clipper's child lies entirely left of the clipper; its ink is cut
	// to the clipper's own box on X, so the parent does not grow to the left.
	expectInk(t, clipper, 0, 0, 100, 100)
	expectInk(t, root, 0, 0, 300, 300)
}

func TestInkFloatingChildIsExcluded(t *testing.T) {
	menu := box(Style{Position: Floating, Width: Px(200), Height: Px(200)})
	card := box(Style{Width: Px(50), Height: Px(50)}, menu)
	root := box(Style{}, card)
	var e Engine
	e.Compute(root, 300, 300)
	expectInk(t, card, 0, 0, 50, 50)
}

func TestInkOverhangGrowsTheBox(t *testing.T) {
	n := box(Style{Width: Px(100), Height: Px(40)})
	n.Overhang = 12
	root := box(Style{}, n)
	var e Engine
	e.Compute(root, 300, 300)
	expectInk(t, n, -12, -12, 112, 52)
}

func TestInkGrowScalesAboutCenter(t *testing.T) {
	n := box(Style{Width: Px(100), Height: Px(40)})
	n.GrowOverhang = 0.1 // drawn at 1.1x about (50, 20)
	root := box(Style{}, n)
	var e Engine
	e.Compute(root, 300, 300)
	expectInk(t, n, -5, -2, 105, 42)
}

func TestInkScalesChildrenOverflowingAGrowingParent(t *testing.T) {
	// A child far outside a scaled parent is pushed further out by the
	// scale, so the parent's ink must reach it scaled, not merely by its box.
	far := box(Style{Position: Absolute, Left: Px(400), Top: Px(0), Width: Px(10), Height: Px(10)})
	n := box(Style{Width: Px(100), Height: Px(100)}, far)
	n.GrowOverhang = 0.1
	root := box(Style{}, n)
	var e Engine
	e.Compute(root, 600, 600)
	// Scaled about (50, 50): the child's right edge at 410 maps to 50+1.1*360.
	if n.InkX1 < 50+1.1*360 {
		t.Errorf("ink right edge %v does not reach the scaled child at %v", n.InkX1, 50+1.1*360)
	}
}

func TestInkOfLeafCoversOverflowingContent(t *testing.T) {
	// Text wider than its box (a word that cannot wrap) draws past it.
	leaf := &Node{Style: Style{Width: Px(30), Height: Px(20)}, Measure: fixedText{words: 1, wordW: 50}}
	root := box(Style{}, leaf)
	var e Engine
	e.Compute(root, 300, 300)
	expectInk(t, leaf, 0, 0, 50, 20)
}

func TestInkUpdatesWhenOverhangChanges(t *testing.T) {
	n := box(Style{Width: Px(100), Height: Px(40)})
	root := box(Style{}, n)
	var e Engine
	e.Compute(root, 300, 300)
	expectInk(t, n, 0, 0, 100, 40)
	// A changed overhang needs Invalidate, as any other change of the node.
	n.Overhang = 8
	n.Invalidate()
	root.Invalidate()
	e.Compute(root, 300, 300)
	expectInk(t, n, -8, -8, 108, 48)
}

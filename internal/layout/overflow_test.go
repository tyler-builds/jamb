package layout

import "testing"

// overflowText is a fake text that does not wrap and draws past its box.
type overflowText struct{ noWrapText }

func (overflowText) Overflows() bool { return true }

// A NoWrap text stretched across a scroll container that is narrower than its
// line counts its line in the container's scrollable width, padding and all, as
// CSS counts the overflow of a child that does not clip.
func TestScrollWidthCountsOverflowingText(t *testing.T) {
	txt := &Node{Measure: overflowText{noWrapText{w: 500}}}
	scroller := box(Style{Width: Px(200), OverflowX: OverflowScroll, OverflowY: OverflowHidden,
		Padding: Edges{Top: 4, Right: 10, Bottom: 4, Left: 10}}, txt)
	root := box(Style{AlignItems: AlignStart}, scroller)
	var e Engine
	e.Compute(root, 300, 300)
	if txt.W != 180 {
		t.Fatalf("the text is %v wide; want it stretched to the scroller's 180", txt.W)
	}
	if txt.ContentW != 500 {
		t.Errorf("the text's content is %v wide; want its 500 line", txt.ContentW)
	}
	if scroller.ContentW != 520 {
		t.Errorf("the scroller's content is %v wide; want 520: the line between its paddings", scroller.ContentW)
	}
}

// Overflow reaches a scroll container through children that do not clip, and not
// through one that does.
func TestScrollWidthStopsAtClippingChild(t *testing.T) {
	line := func() *Node { return &Node{Measure: overflowText{noWrapText{w: 500}}} }
	through := box(Style{}, line())
	clipped := box(Style{OverflowX: OverflowHidden}, line())
	open := box(Style{Width: Px(200), OverflowX: OverflowScroll}, through)
	shut := box(Style{Width: Px(200), OverflowX: OverflowScroll}, clipped)
	root := box(Style{AlignItems: AlignStart}, open, shut)
	var e Engine
	e.Compute(root, 300, 300)
	if open.ContentW != 500 {
		t.Errorf("through a plain box the scroller's content is %v wide; want 500", open.ContentW)
	}
	if shut.ContentW != 200 {
		t.Errorf("through a clipping box the scroller's content is %v wide; want its own 200", shut.ContentW)
	}
}

// A leaf that does not overflow, such as an image scaled into its box, keeps its
// box as its content however large its natural size.
func TestScrollWidthIgnoresLeafThatDoesNotOverflow(t *testing.T) {
	img := &Node{Style: Style{Width: Px(100), Height: Px(50)}, Measure: fixedImage{w: 1000, h: 500}}
	scroller := box(Style{Width: Px(200), OverflowX: OverflowScroll}, img)
	root := box(Style{AlignItems: AlignStart}, scroller)
	var e Engine
	e.Compute(root, 300, 300)
	if scroller.ContentW != 200 {
		t.Errorf("the scroller's content is %v wide; want its own 200", scroller.ContentW)
	}
}

// An absolute child that fills its parent ends where the parent's box does: the
// trailing padding follows in-flow content only, as in CSS, so the parent does
// not overflow by its padding and widen a scroll container around it.
func TestAbsoluteFillDoesNotOverflowPadding(t *testing.T) {
	fill := box(Style{Position: Absolute, Left: Px(0), Right: Px(0), Top: Px(0), Bottom: Px(0)})
	tab := box(Style{Padding: Edges{Top: 4, Right: 12, Bottom: 4, Left: 12}},
		&Node{Measure: overflowText{noWrapText{w: 40}}}, fill)
	strip := box(Style{Direction: Row, OverflowX: OverflowScroll}, tab)
	root := box(Style{AlignItems: AlignStart}, strip)
	var e Engine
	e.Compute(root, 300, 300)
	if tab.ContentW != tab.W || tab.ContentH != tab.H {
		t.Errorf("the tab's content is %vx%v; want its box, %vx%v", tab.ContentW, tab.ContentH, tab.W, tab.H)
	}
	if strip.ContentW != strip.W {
		t.Errorf("the strip's content is %v wide; want its own %v", strip.ContentW, strip.W)
	}
}

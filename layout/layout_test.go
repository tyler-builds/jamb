package layout

import (
	"fmt"
	"testing"
)

// fixedText is a fake text leaf: n characters of 10px wide words, 20px lines.
type fixedText struct{ words, wordW float32 }

func (t fixedText) Measure(maxW float32) (float32, float32) {
	perLine := t.words
	if maxW < t.words*t.wordW {
		perLine = max(1, float32(int(maxW/t.wordW)))
	}
	if perLine > t.words {
		perLine = t.words
	}
	lines := float32(int((t.words + perLine - 1) / perLine))
	return perLine * t.wordW, lines * 20
}

func box(s Style, kids ...*Node) *Node { return &Node{Style: s, Children: kids} }

func expect(t *testing.T, n *Node, x, y, w, h float32) {
	t.Helper()
	if n.X != x || n.Y != y || n.W != w || n.H != h {
		t.Errorf("got (%v,%v %vx%v), want (%v,%v %vx%v)", n.X, n.Y, n.W, n.H, x, y, w, h)
	}
}

func TestRowGrowAndGap(t *testing.T) {
	a := box(Style{Width: Px(100)})
	b := box(Style{Grow: 1})
	c := box(Style{Width: Px(50)})
	root := box(Style{Direction: Row, Gap: 10, Padding: Edges{5, 5, 5, 5}}, a, b, c)
	var e Engine
	e.Compute(root, 400, 100)
	expect(t, a, 5, 5, 100, 90)
	expect(t, b, 115, 5, 220, 90)
	expect(t, c, 345, 5, 50, 90)
}

func TestColumnJustifyAlign(t *testing.T) {
	a := box(Style{Width: Px(40), Height: Px(20)})
	b := box(Style{Width: Px(60), Height: Px(20)})
	root := box(Style{Justify: JustifyCenter, AlignItems: AlignCenter}, a, b)
	var e Engine
	e.Compute(root, 200, 100)
	expect(t, a, 80, 30, 40, 20)
	expect(t, b, 70, 50, 60, 20)
}

func TestSpaceBetween(t *testing.T) {
	a := box(Style{Width: Px(10)})
	b := box(Style{Width: Px(10)})
	root := box(Style{Direction: Row, Justify: JustifySpaceBetween}, a, b)
	var e Engine
	e.Compute(root, 100, 10)
	expect(t, a, 0, 0, 10, 10)
	expect(t, b, 90, 0, 10, 10)
}

func TestTextWrapsInColumn(t *testing.T) {
	txt := &Node{Measure: fixedText{words: 10, wordW: 10}}
	card := box(Style{Padding: Edges{10, 10, 10, 10}}, txt)
	root := box(Style{}, card)
	var e Engine
	e.Compute(root, 60, 500)
	// 40px of content width → 4 words per line → 3 lines.
	expect(t, card, 0, 0, 60, 80)
	expect(t, txt, 10, 10, 40, 60)
}

func TestRowShrinksText(t *testing.T) {
	icon := box(Style{Width: Px(20), Height: Px(20), Shrink: 1})
	txt := &Node{Measure: fixedText{words: 10, wordW: 10}, Style: Style{Shrink: 1}}
	root := box(Style{Direction: Row, AlignItems: AlignStart}, icon, txt)
	var e Engine
	e.Compute(root, 70, 200)
	expect(t, icon, 0, 0, 20, 20)
	expect(t, txt, 20, 0, 50, 40)
}

func TestScrollListKeepsRowHeights(t *testing.T) {
	var rows []*Node
	for i := 0; i < 10; i++ {
		rows = append(rows, box(Style{Height: Px(50), Shrink: 1}))
	}
	list := box(Style{OverflowY: OverflowScroll, Grow: 1, Shrink: 1}, rows...)
	header := box(Style{Height: Px(40)})
	root := box(Style{}, header, list)
	var e Engine
	e.Compute(root, 100, 300)
	expect(t, list, 0, 40, 100, 260)
	expect(t, rows[9], 0, 450, 100, 50)
	if list.ContentH != 500 {
		t.Errorf("content height %v, want 500", list.ContentH)
	}
}

func TestAbsolute(t *testing.T) {
	badge := box(Style{Position: Absolute, Top: Px(4), Right: Px(4), Width: Px(10), Height: Px(10)})
	fill := box(Style{Position: Absolute, Left: Px(0), Right: Px(0), Top: Px(0), Bottom: Px(0)})
	root := box(Style{}, badge, fill)
	var e Engine
	e.Compute(root, 100, 50)
	expect(t, badge, 86, 4, 10, 10)
	expect(t, fill, 0, 0, 100, 50)
}

func TestFloating(t *testing.T) {
	pop := box(Style{Position: Floating}, box(Style{Width: Px(200), Height: Px(30)}))
	parent := box(Style{Width: Px(40), Height: Px(20)}, pop)
	root := box(Style{AlignItems: AlignStart}, parent)
	var e Engine
	e.Compute(root, 300, 100)
	if pop.W != 200 || pop.H != 30 {
		t.Errorf("floating size %vx%v, want 200x30", pop.W, pop.H)
	}
	if parent.ContentW != 40 || parent.ContentH != 20 {
		t.Errorf("parent content %vx%v, want 40x20", parent.ContentW, parent.ContentH)
	}
}

func TestPercentAndMinMax(t *testing.T) {
	a := box(Style{Width: Percent(50), MaxWidth: Px(80), Height: Px(10)})
	b := box(Style{Width: Percent(10), MinWidth: Px(30), Height: Px(10)})
	root := box(Style{Direction: Row, AlignItems: AlignStart}, a, b)
	var e Engine
	e.Compute(root, 200, 100)
	expect(t, a, 0, 0, 80, 10)
	expect(t, b, 80, 0, 30, 10)
}

func TestAutoHeightFromChildren(t *testing.T) {
	inner := box(Style{Height: Px(30)})
	mid := box(Style{Padding: Edges{5, 0, 5, 0}, Margin: Edges{Top: 7}}, inner)
	root := box(Style{AlignItems: AlignStart}, mid)
	var e Engine
	e.Compute(root, 100, 100)
	expect(t, mid, 0, 7, 0, 40)
}

func buildTree(depth, fan int) *Node {
	if depth == 0 {
		return &Node{Measure: fixedText{words: 8, wordW: 9}, Style: Style{Shrink: 1}}
	}
	n := &Node{Style: Style{Direction: Direction(depth % 2), Gap: 4, Padding: Edges{4, 4, 4, 4}, Shrink: 1}}
	for i := 0; i < fan; i++ {
		n.Children = append(n.Children, buildTree(depth-1, fan))
	}
	return n
}

func BenchmarkLayout(b *testing.B) {
	for _, size := range []struct{ depth, fan int }{{3, 6}, {4, 6}, {6, 3}} {
		root := buildTree(size.depth, size.fan)
		count := 0
		var walk func(*Node)
		walk = func(n *Node) {
			count++
			for _, c := range n.Children {
				walk(c)
			}
		}
		walk(root)
		b.Run(fmt.Sprintf("nodes=%d", count), func(b *testing.B) {
			var e Engine
			b.ReportAllocs()
			for b.Loop() {
				e.Compute(root, 1200, 800)
			}
		})
	}
}

type fixedImage struct{ w, h float32 }

func (i fixedImage) Measure(float32) (float32, float32) { return i.w, i.h }

func TestPercentImageIsCompressible(t *testing.T) {
	img := &Node{Measure: fixedImage{960, 240}, Style: Style{Width: Percent(100), Height: Percent(100)}}
	banner := box(Style{Height: Px(150)}, img)
	main := box(Style{Grow: 1, Shrink: 1}, banner)
	side := box(Style{Width: Px(200), Shrink: 0})
	root := box(Style{Direction: Row}, side, main)
	var e Engine
	e.Compute(root, 700, 400)
	expect(t, main, 200, 0, 500, 400)
	expect(t, img, 0, 0, 500, 150)
}

func TestIncrementalLayoutSkipsCleanSubtrees(t *testing.T) {
	var rows []*Node
	for i := 0; i < 50; i++ {
		cell := &Node{Measure: fixedText{words: 3, wordW: 10}, Style: Style{Shrink: 1}}
		rows = append(rows, box(Style{Direction: Row, Padding: Edges{4, 4, 4, 4}}, box(Style{}, cell)))
	}
	root := box(Style{}, rows...)
	var e Engine
	e.Compute(root, 400, 3000)
	full := e.Arranged

	e.Compute(root, 400, 3000)
	if e.Arranged != 0 {
		t.Fatalf("unchanged tree re-arranged %d nodes", e.Arranged)
	}

	// Change one leaf's content without changing its size: only the path to it is revisited.
	leaf := rows[10].Children[0].Children[0]
	leaf.Measure = fixedText{words: 3, wordW: 10}
	leaf.Invalidate()
	rows[10].Children[0].Invalidate()
	rows[10].Invalidate()
	root.Invalidate()
	e.Compute(root, 400, 3000)
	if e.Arranged > 4 {
		t.Errorf("one-leaf change arranged %d of %d nodes, want only its ancestor path", e.Arranged, full)
	}
	expect(t, rows[49], 0, 49*28, 400, 28)

	// A size change of the root re-lays out everything that depends on width.
	e.Compute(root, 300, 3000)
	expect(t, rows[49], 0, 49*28, 300, 28)
}

func TestWrapBreaksIntoLines(t *testing.T) {
	var chips []*Node
	for i := 0; i < 4; i++ {
		chips = append(chips, box(Style{Width: Px(30), Height: Px(20)}))
	}
	tall := box(Style{Width: Px(30), Height: Px(40)})
	chips = append(chips, tall)
	row := box(Style{Direction: Row, Wrap: true, Gap: 5, AlignItems: AlignStart}, chips...)
	root := box(Style{AlignItems: AlignStart}, box(Style{Width: Px(100)}, row))
	var e Engine
	e.Compute(root, 400, 400)
	// 30+5+30+5+30 = 100 fits; the 4th chip wraps.
	expect(t, chips[0], 0, 0, 30, 20)
	expect(t, chips[2], 70, 0, 30, 20)
	expect(t, chips[3], 0, 25, 30, 20)
	expect(t, tall, 35, 25, 30, 40)
	if row.H != 65 {
		t.Errorf("wrapped row height %v, want 20+5+40", row.H)
	}
}

func TestWrapStretchesToLineAndMeasuresInAutoParent(t *testing.T) {
	a := box(Style{Width: Px(60)}) // stretches to its line (cross auto)
	b := box(Style{Width: Px(60), Height: Px(30)})
	c := box(Style{Width: Px(60), Height: Px(10)})
	row := box(Style{Direction: Row, Wrap: true}, a, b, c)
	card := box(Style{Padding: Edges{5, 5, 5, 5}}, row) // auto height from content
	root := box(Style{AlignItems: AlignStart}, box(Style{Width: Px(135), AlignItems: AlignStart}, card))
	var e Engine
	e.Compute(root, 400, 400)
	// Content width 125: a and b share line 1 (cross 30), c is alone on line 2.
	expect(t, a, 0, 0, 60, 30)
	expect(t, c, 0, 30, 60, 10)
	if card.H != 50 {
		t.Errorf("card height %v, want 5+30+10+5", card.H)
	}
}

func TestWrapJustifiesEachLine(t *testing.T) {
	var items []*Node
	for i := 0; i < 3; i++ {
		items = append(items, box(Style{Width: Px(40), Height: Px(10)}))
	}
	row := box(Style{Direction: Row, Wrap: true, Justify: JustifyCenter}, items...)
	root := box(Style{}, row)
	var e Engine
	e.Compute(root, 100, 100)
	expect(t, items[0], 10, 0, 40, 10) // line 1: 80 wide, centered in 100
	expect(t, items[2], 30, 10, 40, 10)
}

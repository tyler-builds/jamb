package layout

import (
	"math"
	"testing"
)

// wordText is a fake wrapping leaf: words of the given widths, with no spaces.
// A line holds whole words, and a word wider than the line is broken into pieces
// of the line width. It implements MinContenter, which is the widest word.
type wordText struct {
	words []float32
	lineH float32
}

func (t wordText) MinContentWidth() (float32, bool) {
	var w float32
	for _, x := range t.words {
		w = max(w, x)
	}
	return w, true
}

func (t wordText) Measure(maxW float32) (float32, float32) {
	var width, cur float32
	lines := 1
	for _, w := range t.words {
		if cur > 0 && cur+w > maxW {
			lines++
			cur = 0
		}
		if w > maxW && maxW > 0 {
			pieces := int(math.Ceil(float64(w / maxW)))
			lines += pieces - 1
			width = max(width, maxW)
			cur = w - float32(pieces-1)*maxW
			continue
		}
		cur += w
		width = max(width, cur)
	}
	return width, float32(lines) * t.lineH
}

// noWrapText is a fake NoWrap leaf: one line of its full width, however narrow.
type noWrapText struct{ w float32 }

func (t noWrapText) MinContentWidth() (float32, bool)   { return t.w, false }
func (t noWrapText) Measure(float32) (float32, float32) { return t.w, 20 }

func text(lineH float32, words ...float32) *Node {
	return &Node{Style: Style{Shrink: 1}, Measure: wordText{words: words, lineH: lineH}}
}

// A text in a shrinking row keeps its longest word: it does not shrink into the
// breaking of its words, and the row overflows when a fixed item beside it takes
// the room, as CSS does.
func TestTextKeepsItsLongestWordInShrinkingRow(t *testing.T) {
	icon := box(Style{Width: Px(20), Height: Px(20), Shrink: 1})
	txt := text(20, 30, 10, 10)
	root := box(Style{Direction: Row, AlignItems: AlignStart}, icon, txt)
	var e Engine
	e.Compute(root, 40, 200)
	if txt.W != 30 {
		t.Errorf("text is %v wide, want its longest word, 30", txt.W)
	}
	if txt.H != 40 {
		t.Errorf("text is %v tall, want two lines of 20 at 30 wide", txt.H)
	}
	if icon.W != 20 {
		t.Errorf("icon is %v wide, want its fixed 20", icon.W)
	}
}

// A word that fits its container does not break, however little room its siblings
// leave: beside a fixed icon in a 40 row the 30 word stays whole.
func TestWordThatFitsItsContainerDoesNotBreak(t *testing.T) {
	icon := box(Style{Width: Px(20), Height: Px(20), Shrink: 0})
	txt := text(20, 30, 10, 10)
	root := box(Style{Direction: Row, AlignItems: AlignStart}, icon, txt)
	var e Engine
	e.Compute(root, 40, 200)
	if txt.W != 30 {
		t.Errorf("text is %v wide beside a 20 icon in a 40 row, want its word, 30", txt.W)
	}
}

// The longest word is the floor of a shrinking text: it stops at 30 in a 30 row,
// where its natural width is 40. A row narrower than the word breaks the word.
func TestTextMinimumIsItsWordNotItsNaturalWidth(t *testing.T) {
	txt := text(20, 30, 10)
	root := box(Style{Direction: Row}, txt)
	var e Engine
	e.Compute(root, 35, 100)
	if txt.W != 35 {
		t.Errorf("text is %v wide in a 35 row, want 35 (its natural width is 40, its word 30)", txt.W)
	}
	e.Compute(root, 30, 100)
	if txt.W != 30 {
		t.Errorf("text is %v wide in a 30 row, want its longest word, 30", txt.W)
	}
	e.Compute(root, 20, 100)
	if txt.W != 20 {
		t.Errorf("text is %v wide in a 20 row, want 20: the 30 word breaks", txt.W)
	}
}

// A NoWrap leaf is one line, so it keeps its full width.
func TestNoWrapLeafKeepsItsFullWidth(t *testing.T) {
	nw := &Node{Style: Style{Shrink: 1}, Measure: noWrapText{w: 50}}
	root := box(Style{Direction: Row}, nw)
	var e Engine
	e.Compute(root, 30, 100)
	if nw.W != 50 {
		t.Errorf("NoWrap leaf is %v wide, want its full 50", nw.W)
	}
}

// An explicit MinWidth replaces the automatic minimum, so the text may shrink
// below its longest word, as Truncate's zero MinWidth lets it.
func TestExplicitMinWidthOptsOut(t *testing.T) {
	txt := text(20, 30)
	txt.Style.MinWidth = Px(0)
	root := box(Style{Direction: Row}, txt)
	var e Engine
	e.Compute(root, 20, 100)
	if txt.W != 20 {
		t.Errorf("text with MinWidth 0 is %v wide, want 20", txt.W)
	}
}

// A word wider than the row breaks, so the text stays within the row.
func TestWordWiderThanRowBreaksInsideTheWord(t *testing.T) {
	txt := text(20, 200)
	root := box(Style{Direction: Row, AlignItems: AlignStart}, txt)
	var e Engine
	e.Compute(root, 100, 100)
	if txt.W != 100 {
		t.Errorf("text is %v wide in a 100 row, want 100", txt.W)
	}
	if txt.H != 40 {
		t.Errorf("text is %v tall, want the 200 word broken into two lines", txt.H)
	}
}

// The same holds for a column inside the row: its minimum is held to the row too.
func TestColumnOfLongWordStaysInRow(t *testing.T) {
	col := box(Style{Shrink: 1}, text(20, 200))
	root := box(Style{Direction: Row}, col)
	var e Engine
	e.Compute(root, 100, 100)
	if col.W != 100 {
		t.Errorf("column is %v wide in a 100 row, want 100", col.W)
	}
	if col.Children[0].W != 100 {
		t.Errorf("text in the column is %v wide, want 100", col.Children[0].W)
	}
}

// A container of texts in a row takes the sum of their minimums, and a text in a
// column the largest of them.
func TestNestedRowAndColumnMinimums(t *testing.T) {
	a := text(20, 30, 10) // natural 40, longest word 30
	b := text(20, 20)     // natural 20, longest word 20
	inner := box(Style{Direction: Row, Gap: 10, Shrink: 1}, a, b)
	root := box(Style{Direction: Row}, inner)
	var e Engine
	e.Compute(root, 65, 100)
	if inner.W != 65 {
		t.Errorf("inner row is %v wide in a 65 row, want 65", inner.W)
	}
	if math.Abs(float64(a.W-35)) > 0.01 || b.W != 20 {
		t.Errorf("texts are %v and %v wide, want 35 and 20", a.W, b.W)
	}

	c := text(20, 30, 10) // longest word 30
	d := text(20, 20)     // longest word 20
	col := box(Style{Shrink: 1}, c, d)
	root = box(Style{Direction: Row}, col)
	e.Compute(root, 30, 100)
	if col.W != 30 {
		t.Errorf("column of texts is %v wide in a 30 row, want the largest minimum, 30", col.W)
	}
	icon := box(Style{Width: Px(20), Shrink: 0})
	root = box(Style{Direction: Row}, col, icon)
	e.Compute(root, 40, 100)
	if col.W != 30 {
		t.Errorf("column beside a 20 icon in a 40 row is %v wide, want its largest minimum, 30", col.W)
	}
}

// A container whose overflow would hide a long word keeps the automatic minimum at
// zero, as any clipping item does.
func TestClippingItemHasNoAutomaticMinimum(t *testing.T) {
	clip := box(Style{Shrink: 1, OverflowX: OverflowHidden}, text(20, 30))
	root := box(Style{Direction: Row}, clip)
	var e Engine
	e.Compute(root, 20, 100)
	if clip.W != 20 {
		t.Errorf("clipping item is %v wide in a 20 row, want 20", clip.W)
	}
}

// minOf measures the min-content width of n.
func minOf(e *Engine, n *Node) float32 { w, _ := e.minContentSize(n, nan); return w }

// The min-content width of containers: a column takes the largest child, a row
// the sum of its children and gaps, a wrapping row the largest child. Padding,
// margins, hidden and absolute children follow the same rules as the flex layout.
func TestMinContentOfContainers(t *testing.T) {
	var e Engine
	col := box(Style{Padding: Edges{Left: 5, Right: 5}}, text(20, 30, 10), text(20, 20))
	if got := minOf(&e, col); got != 40 {
		t.Errorf("column min-content = %v, want 30 + 5 + 5 padding = 40", got)
	}
	row := box(Style{Direction: Row, Gap: 10}, text(20, 30, 10), text(20, 20))
	if got := minOf(&e, row); got != 60 {
		t.Errorf("row min-content = %v, want 30 + 10 + 20 = 60", got)
	}
	wrap := box(Style{Direction: Row, Wrap: true, Gap: 10}, text(20, 30, 10), text(20, 20))
	if got := minOf(&e, wrap); got != 30 {
		t.Errorf("wrapping row min-content = %v, want the largest child, 30", got)
	}
	m := box(Style{Direction: Row}, &Node{Style: Style{Width: Px(12), Margin: Edges{Left: 3, Right: 4}}})
	if got := minOf(&e, m); got != 19 {
		t.Errorf("row of a fixed child with margins = %v, want 12 + 3 + 4 = 19", got)
	}
	hidden := box(Style{Direction: Row}, text(20, 50), &Node{Style: Style{Hidden: true, Width: Px(90)}})
	if got := minOf(&e, hidden); got != 50 {
		t.Errorf("row with a hidden child = %v, want 50", got)
	}
	abs := box(Style{Direction: Row}, text(20, 50), &Node{Style: Style{Position: Absolute, Width: Px(90)}})
	if got := minOf(&e, abs); got != 50 {
		t.Errorf("row with an absolute child = %v, want 50", got)
	}
}

// A wrapping leaf is measured by its min-content width, so a percentage width of
// an indefinite size contributes nothing, as in computeSize.
func TestMinContentPercentIsCompressible(t *testing.T) {
	var e Engine
	p := &Node{Style: Style{Width: Percent(100)}, Measure: wordText{words: []float32{80}, lineH: 20}}
	if got := minOf(&e, p); got != 0 {
		t.Errorf("percent-width text min-content = %v, want 0 while its container is indefinite", got)
	}
}

// BenchmarkShrinkingRows lays out 100 rows, each a fixed item and a wrapping text
// that shrinks, at two widths in turn, so every pass measures the minimums.
func BenchmarkShrinkingRows(b *testing.B) {
	rows := make([]*Node, 100)
	for i := range rows {
		rows[i] = box(Style{Direction: Row, Gap: 12}, box(Style{Width: Px(60), Shrink: 1}), text(20, 30, 10, 10, 25))
	}
	root := box(Style{Gap: 8}, rows...)
	var e Engine
	e.Compute(root, 120, 4000)
	b.ResetTimer()
	for i := range b.N {
		root.Invalidate()
		e.Compute(root, float32(100+i%2*20), 4000)
	}
}

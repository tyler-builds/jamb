package layout

import (
	"math"
	"testing"
)

func leaf(w, h float32) *Node { return &Node{Measure: fixedText{1, w}, Style: Style{Height: Px(h)}} }

func gridOf(cols []Length, rows []Length, s Style, kids ...*Node) *Node {
	s.Grid = &Grid{Columns: cols, Rows: rows}
	return box(s, kids...)
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.01 }

func expectNear(t *testing.T, n *Node, x, y, w, h float32) {
	t.Helper()
	if !near(n.X, x) || !near(n.Y, y) || !near(n.W, w) || !near(n.H, h) {
		t.Errorf("got (%v,%v %vx%v), want (%v,%v %vx%v)", n.X, n.Y, n.W, n.H, x, y, w, h)
	}
}

// Px columns take their size, Fr columns share the rest by their values, and
// items fill the cells row by row.
func TestGridFixedAndFr(t *testing.T) {
	kids := []*Node{box(Style{Height: Px(20)}), box(Style{Height: Px(30)}), box(Style{Height: Px(10)}), box(Style{Height: Px(40)})}
	root := gridOf([]Length{Px(100), Fr(1), Fr(2)}, nil, Style{Gap: 10, Padding: Edges{5, 5, 5, 5}}, kids...)
	var e Engine
	e.Compute(root, 410, 300)
	// 400 inside, less 100 and two gaps: 280 for 3 shares.
	// Items with a height keep it, as in CSS; the row is as tall as the tallest.
	expectNear(t, kids[0], 5, 5, 100, 20)
	expectNear(t, kids[1], 115, 5, 280.0/3, 30)
	expectNear(t, kids[2], 125+280.0/3, 5, 560.0/3, 10)
	expectNear(t, kids[3], 5, 45, 100, 40) // the second row
}

// An Auto column is as wide as its widest item; Fr columns take the rest.
func TestGridAutoColumn(t *testing.T) {
	a, b := leaf(70, 20), leaf(40, 20)
	c, d := box(Style{Height: Px(20)}), box(Style{Height: Px(20)})
	root := gridOf([]Length{Auto, Fr(1)}, nil, Style{Gap: 10}, a, c, b, d)
	var e Engine
	e.Compute(root, 300, 200)
	expectNear(t, a, 0, 0, 70, 20)
	expectNear(t, c, 80, 0, 220, 20)
	expectNear(t, b, 0, 30, 70, 20) // stretched to the column
}

// An Auto column of wrapping text shrinks when the row does not fit, towards
// the text's narrowest width.
func TestGridAutoShrinks(t *testing.T) {
	text := &Node{Measure: fixedText{20, 10}} // 200 wide on one line, 10 at least
	fixed := box(Style{Height: Px(10)})
	root := gridOf([]Length{Auto, Px(100)}, nil, Style{}, text, fixed)
	var e Engine
	e.Compute(root, 150, 200)
	if !near(text.W, 50) || !near(fixed.X, 50) {
		t.Errorf("text %vx%v, fixed at %v; want the text 50 wide", text.W, text.H, fixed.X)
	}
	if text.H != 80 { // 5 words a line, 4 lines
		t.Errorf("text height %v, want 80", text.H)
	}
}

// A spanning item takes several columns, and moves to the next row when the
// rest of the row is too narrow; a row span keeps later items out of its cells.
func TestGridSpans(t *testing.T) {
	a := box(Style{Height: Px(10)})
	wide := box(Style{Height: Px(10), ColSpan: 2})
	tall := box(Style{RowSpan: 2})
	b, c, d := box(Style{Height: Px(10)}), box(Style{Height: Px(10)}), box(Style{Height: Px(10)})
	root := gridOf([]Length{Px(50), Px(50), Px(50)}, nil, Style{}, a, b, wide, tall, c, d)
	var e Engine
	e.Compute(root, 150, 200)
	expectNear(t, a, 0, 0, 50, 10)
	expectNear(t, b, 50, 0, 50, 10)
	expectNear(t, wide, 0, 10, 100, 10) // did not fit after b
	expectNear(t, tall, 100, 10, 50, 20)
	expectNear(t, c, 0, 20, 50, 10)
	expectNear(t, d, 50, 20, 50, 10) // the third column is the tall item's
}

// Rows takes Px and Fr sizes when the grid has a height; the rest fit content.
// AlignSelf places an item in its row, and items without a height stretch.
func TestGridRowsAndAlign(t *testing.T) {
	head := box(Style{ColSpan: 2})
	body, side := box(Style{}), box(Style{})
	foot := box(Style{Height: Px(10), AlignSelf: AlignEnd})
	tall := box(Style{Height: Px(30)})
	root := gridOf([]Length{Fr(1), Fr(1)}, []Length{Px(40), Fr(1)}, Style{}, head, body, side, foot, tall)
	var e Engine
	e.Compute(root, 100, 200)
	expectNear(t, head, 0, 0, 100, 40)
	// Fr(1) takes what the 40 and the auto row of 30 leave.
	expectNear(t, body, 0, 40, 50, 130)
	expectNear(t, side, 50, 40, 50, 130)
	expectNear(t, foot, 0, 190, 50, 10) // at the end of its 30 tall row
	expectNear(t, tall, 50, 170, 50, 30)
}

// A grid with an automatic size is as big as its tracks and gaps, and its
// min-content width counts each column.
func TestGridIntrinsicSize(t *testing.T) {
	g := gridOf([]Length{Px(40), Auto}, nil, Style{Gap: 5}, box(Style{Height: Px(10)}), leaf(30, 15))
	root := box(Style{Direction: Row, AlignItems: AlignStart}, g)
	var e Engine
	e.Compute(root, 500, 500)
	expectNear(t, g, 0, 0, 75, 15)
	if w, _ := e.minContentSize(g, nan); !near(w, 75) {
		t.Errorf("min-content width %v, want 75", w)
	}
}

// Styles with equal grids are equal; with other tracks they are not.
func TestStyleEqual(t *testing.T) {
	a := Style{Grid: &Grid{Columns: []Length{Px(1), Fr(1)}}}
	b := Style{Grid: &Grid{Columns: []Length{Px(1), Fr(1)}}}
	c := Style{Grid: &Grid{Columns: []Length{Px(1), Fr(2)}}}
	if !a.Equal(b) || a.Equal(c) || a.Equal(Style{}) || !(Style{Gap: 1}).Equal(Style{Gap: 1}) {
		t.Error("Equal compared grids wrongly")
	}
}

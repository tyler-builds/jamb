package layout

import "testing"

// A column item with Basis(0) in a column of no height keeps its content's
// height, as CSS's min-height: auto does, so the next item starts below it.
func TestBasisZeroInAutoHeightColumnKeepsContentHeight(t *testing.T) {
	content := box(Style{Height: Px(200)})
	main := box(Style{Direction: Column, Grow: 1, Basis: Px(0), MinWidth: Px(0)}, content)
	after := box(Style{Height: Px(50)})
	page := box(Style{Direction: Column}, main, after)
	root := box(Style{Direction: Column, Width: Percent(100), Height: Percent(100), OverflowY: OverflowScroll}, page)
	e := &Engine{}
	e.Compute(root, 800, 600)
	expect(t, main, 0, 0, 800, 200)
	expect(t, after, 0, 200, 800, 50)
}

// Items with Grow(1) and Basis(0) share a row equally while each share holds
// its content, and an item whose longest word is wider than its share keeps it,
// leaving the rest to the others.
func TestBasisZeroItemsShareEquallyAboveTheirMinimum(t *testing.T) {
	a := text(20, 30)
	b := text(20, 30)
	a.Style, b.Style = Style{Grow: 1, Shrink: 1, Basis: Px(0)}, Style{Grow: 1, Shrink: 1, Basis: Px(0)}
	root := box(Style{Direction: Row, AlignItems: AlignStart, Width: Px(300)}, a, b)
	e := &Engine{}
	e.Compute(root, 300, 100)
	expect(t, a, 0, 0, 150, 20)
	expect(t, b, 150, 0, 150, 20)

	wide := text(20, 200)
	wide.Style = Style{Grow: 1, Shrink: 1, Basis: Px(0)}
	c := text(20, 30)
	c.Style = Style{Grow: 1, Shrink: 1, Basis: Px(0)}
	root = box(Style{Direction: Row, AlignItems: AlignStart, Width: Px(300)}, wide, c)
	e = &Engine{}
	e.Compute(root, 300, 100)
	expect(t, wide, 0, 0, 200, 20)
	expect(t, c, 200, 0, 100, 20)
}

// A fit-content row sizes a growing Basis(0) item from its content, as CSS does,
// so a centered page is as wide as its text and not just its fixed items.
func TestFitContentRowSizesBasisZeroItemFromContent(t *testing.T) {
	icon := box(Style{Width: Px(96), Height: Px(96)})
	txt := text(20, 100, 100, 100)
	body := box(Style{Direction: Column, Grow: 1, Basis: Px(0), MinWidth: Px(0)}, txt)
	row := box(Style{Direction: Row, Gap: 20}, icon, body)
	page := box(Style{Direction: Column, MaxWidth: Px(960)}, row)
	root := box(Style{Direction: Column, AlignItems: AlignCenter, Width: Percent(100), Height: Percent(100)}, page)
	e := &Engine{}
	e.Compute(root, 1000, 600)
	expect(t, page, 292, 0, 416, 96)
	expect(t, body, 116, 0, 300, 96)
}

// An AutoFill grid has as many equal columns as fit at the minimum width, and
// gains one as it widens.
func TestGridAutoFillColumns(t *testing.T) {
	for _, c := range []struct {
		width float32
		cols  int
	}{{250, 1}, {600, 2}, {880, 3}, {1000, 3}, {1180, 4}} {
		kids := make([]*Node, 5)
		for i := range kids {
			kids[i] = box(Style{Height: Px(50)})
		}
		g := box(Style{Direction: Column, Gap: 20, Grid: &Grid{Columns: []Length{AutoFill(280)}}}, kids...)
		root := box(Style{Direction: Column}, g)
		e := &Engine{}
		e.Compute(root, c.width, 600)
		wantW := (c.width - 20*float32(c.cols-1)) / float32(c.cols)
		if c.cols == 1 {
			wantW = c.width
		}
		expect(t, kids[0], 0, 0, wantW, 50)
		if c.cols < len(kids) {
			expect(t, kids[c.cols], 0, 70, wantW, 50) // the first item of the second row
		}
	}
}

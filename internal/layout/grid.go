package layout

import "slices"

// Grid layout: a node with Columns lays out its in-flow children in a grid.
// Children fill the cells row by row, in order, each covering ColSpan columns
// and RowSpan rows; an item that does not fit in what is left of a row starts
// the next one, and cells that earlier items skipped stay empty (CSS's sparse
// auto-placement). Columns are sized in three steps: Px and Percent columns
// take their size; Auto columns take the widest content of the items that sit
// in them alone (shrinking towards their narrowest when the row does not fit);
// and Fr columns share what is left in proportion to their values. Rows fit
// their tallest item unless Rows sizes them, the same way. Items span the
// width of their cells unless they have a width of their own, and their
// height unless AlignSelf (or the grid's AlignItems) says otherwise; a
// SoftWidth is not a width of their own here. Gap
// separates rows and columns.
//
// Items that span several tracks do not size Auto tracks, except that a row
// grows to fit an item spanning it and auto rows before it.

// Fr returns a flexible grid track that takes v shares of the space the other
// tracks leave.
func Fr(v float32) Length { return Length{v, UnitFr} }

// isGrid reports whether n lays out its children as a grid.
func (n *Node) isGrid() bool { return n.Style.Grid != nil && len(n.Style.Grid.Columns) > 0 }

type gridItem struct {
	n          *Node
	r, c       int // the first row and column it covers
	rows, cols int // how many it covers
}

// place puts the in-flow children of n in cells, and returns them and the
// number of rows.
func (n *Node) place() ([]gridItem, int) {
	nc := len(n.Style.Grid.Columns)
	var items []gridItem
	var occ [][]bool
	free := func(r, c, rs, cs int) bool {
		for i := r; i < r+rs && i < len(occ); i++ {
			for j := c; j < c+cs; j++ {
				if occ[i][j] {
					return false
				}
			}
		}
		return true
	}
	r, c := 0, 0
	for _, ch := range n.Children {
		cs := &ch.Style
		if cs.Position != Relative || cs.Hidden {
			continue
		}
		span := min(max(cs.ColSpan, 1), nc)
		rspan := max(cs.RowSpan, 1)
		for {
			if c+span > nc {
				r, c = r+1, 0
				continue
			}
			if free(r, c, rspan, span) {
				break
			}
			c++
		}
		for len(occ) < r+rspan {
			occ = append(occ, make([]bool, nc))
		}
		for i := r; i < r+rspan; i++ {
			for j := c; j < c+span; j++ {
				occ[i][j] = true
			}
		}
		items = append(items, gridItem{ch, r, c, rspan, span})
		c += span
	}
	return items, len(occ)
}

// tracks sizes the tracks of one axis. defs are the sizes asked for (Auto
// past the end), space the content-box length (NaN when indefinite); maxC and
// minC are the widest and narrowest content of the items that sit in each
// track alone.
func tracks(defs []Length, count int, space, gap float32, maxC, minC []float32) []float32 {
	size := make([]float32, count)
	var fr float32
	isFr := make([]bool, count)
	used := gap * float32(max(count-1, 0))
	var autos []int
	for i := range count {
		d := Auto
		if i < len(defs) {
			d = defs[i]
		}
		switch {
		case d.Unit == UnitPx:
			size[i] = d.Value
		case d.Unit == UnitPercent && !isNaN(space):
			size[i] = space * d.Value / 100
		case d.Unit == UnitFr && !isNaN(space) && space < inf:
			isFr[i] = true
			fr += d.Value
			continue
		default:
			size[i] = maxC[i]
			autos = append(autos, i)
		}
		used += size[i]
	}
	if isNaN(space) || space >= inf {
		return size
	}
	if over := used - space; over > 0 && len(autos) > 0 {
		// Shrink the content-sized tracks towards their narrowest content, each
		// in proportion to how far it can go.
		var room float32
		for _, i := range autos {
			room += size[i] - minC[i]
		}
		if room > 0 {
			k := min(over/room, 1)
			for _, i := range autos {
				d := (size[i] - minC[i]) * k
				size[i] -= d
				used -= d
			}
		}
	}
	if fr > 0 {
		left := max(space-used, 0)
		for i := range count {
			if isFr[i] {
				d := defs[i]
				size[i] = left * d.Value / fr
			}
		}
	}
	return size
}

// grid lays out n's in-flow children as a grid; see the top of this file and
// flex, whose arguments and results it shares.
func (e *Engine) grid(n *Node, innerW, innerH, availInnerW float32, arrange bool) (float32, float32) {
	s := &n.Style
	items, nr := n.place()
	nc := len(s.Grid.Columns)

	// Columns, from the widest and narrowest content of their own items.
	maxW, minW := make([]float32, nc), make([]float32, nc)
	for _, it := range items {
		if it.cols != 1 {
			continue
		}
		cs := &it.n.Style
		m := cs.Margin.Left + cs.Margin.Right
		w, _ := e.measure(it.n, nan, nan, inf, nan, nan)
		mc, _ := e.minContentSize(it.n, nan)
		maxW[it.c] = max(maxW[it.c], w+m)
		minW[it.c] = max(minW[it.c], min(mc, w)+m)
	}
	width := innerW
	if isNaN(width) {
		width = availInnerW
	}
	colW := tracks(s.Grid.Columns, nc, width, s.Gap, maxW, minW)
	colX := make([]float32, nc+1)
	for i := range nc {
		colX[i+1] = colX[i] + colW[i] + s.Gap
	}
	cellW := func(it gridItem) float32 { return colX[it.c+it.cols] - colX[it.c] - s.Gap }

	// Rows, from the items' heights at their cells' widths.
	itemW := func(it gridItem) float32 {
		cs := &it.n.Style
		cw := cellW(it)
		if (cs.Width.IsAuto() || cs.SoftWidth) && !cs.FitWidth {
			return max(cw-cs.Margin.Left-cs.Margin.Right, 0)
		}
		return nan // its own width
	}
	heights := make([]float32, len(items))
	widths := make([]float32, len(items))
	rowMax := make([]float32, nr)
	for i, it := range items {
		cs := &it.n.Style
		cw := cellW(it)
		w, h := e.measure(it.n, cw, nan, cw-cs.Margin.Left-cs.Margin.Right, itemW(it), nan)
		widths[i], heights[i] = w, h
		if it.rows == 1 {
			rowMax[it.r] = max(rowMax[it.r], h+cs.Margin.Top+cs.Margin.Bottom)
		}
	}
	rowH := tracks(s.Grid.Rows, nr, innerH, s.Gap, rowMax, rowMax)
	// An item spanning rows grows the last auto row it covers to fit it.
	for i, it := range items {
		if it.rows == 1 {
			continue
		}
		cs := &it.n.Style
		need := heights[i] + cs.Margin.Top + cs.Margin.Bottom
		have := s.Gap * float32(it.rows-1)
		last := -1
		for r := it.r; r < it.r+it.rows; r++ {
			have += rowH[r]
			if r >= len(s.Grid.Rows) || s.Grid.Rows[r].IsAuto() {
				last = r
			}
		}
		if need > have && last >= 0 {
			rowH[last] += need - have
		}
	}
	rowY := make([]float32, nr+1)
	for i := range nr {
		rowY[i+1] = rowY[i] + rowH[i] + s.Gap
	}
	contentW := max(colX[nc]-s.Gap, 0)
	contentH := max(rowY[nr]-s.Gap, 0)
	if !arrange {
		return contentW, contentH
	}

	padX, padY := s.Padding.Left+s.Border.Left, s.Padding.Top+s.Border.Top
	for i, it := range items {
		c := it.n
		cs := &c.Style
		cw, ch := cellW(it), rowY[it.r+it.rows]-rowY[it.r]-s.Gap
		w, h := widths[i], heights[i]
		align := cs.AlignSelf
		if align == AlignAuto {
			align = s.AlignItems
			if align == AlignAuto {
				align = AlignStretch
			}
		}
		mv := cs.Margin.Top + cs.Margin.Bottom
		y := rowY[it.r] + cs.Margin.Top
		switch {
		case align == AlignStretch && cs.Height.IsAuto():
			h = max(ch-mv, 0)
			if w2, _ := e.measure(c, cw, ch, cw-cs.Margin.Left-cs.Margin.Right, itemW(it), h); !isNaN(w2) {
				w = w2
			}
		case align == AlignCenter:
			y += (ch - mv - h) / 2
		case align == AlignEnd:
			y += ch - mv - h
		}
		c.X, c.Y, c.W, c.H = padX+colX[it.c]+cs.Margin.Left, padY+y, w, h
		e.arrange(c)
	}
	return 0, 0
}

// gridMinContent is a grid's min-content width: each column's narrowest
// content, or its size when it has one, and the gaps.
func (e *Engine) gridMinContent(n *Node) float32 {
	s := &n.Style
	items, _ := n.place()
	nc := len(s.Grid.Columns)
	col := make([]float32, nc)
	for _, it := range items {
		if it.cols != 1 {
			continue
		}
		cs := &it.n.Style
		mc, _ := e.minContentSize(it.n, nan)
		col[it.c] = max(col[it.c], mc+cs.Margin.Left+cs.Margin.Right)
	}
	w := s.Gap * float32(max(nc-1, 0))
	for i, d := range s.Grid.Columns {
		if d.Unit == UnitPx {
			col[i] = d.Value
		}
		w += col[i]
	}
	return w
}

// Equal reports whether s and o lay out the same: as ==, but comparing the
// grid tracks by value.
func (s Style) Equal(o Style) bool {
	sg, og := s.Grid, o.Grid
	s.Grid, o.Grid = nil, nil
	if s != o {
		return false
	}
	if sg == nil || og == nil {
		return sg == og
	}
	return slices.Equal(sg.Columns, og.Columns) && slices.Equal(sg.Rows, og.Rows)
}

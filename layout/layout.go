package layout

// Node is one box in the layout tree.
type Node struct {
	Style    Style
	Children []*Node
	// Measure, when set, makes the node a leaf whose content size comes from it.
	Measure Measurer

	// Results, written by Compute. X and Y are relative to the parent's
	// border box (before any scrolling); W and H are the border-box size.
	X, Y, W, H float32
	// ContentW and ContentH are the extent of the children including this
	// node's padding — the scrollable size when it exceeds W/H.
	ContentW, ContentH float32

	// Overhang is how far the node's own drawing may extend beyond its box on
	// every side. The caller sets it. GrowOverhang is the extra scale the node
	// is drawn at, 0.05 for 1.05, applied about the box's center.
	Overhang, GrowOverhang float32
	// Ink is the rectangle, relative to the node's own origin, that the node
	// and its subtree may draw into. Floating children are left out, since
	// overlays are drawn from their own list. A child that clips contributes
	// only its own box (plus its overhang), and a clipping node limits its
	// children to its box on the clipped axes. Written by Compute.
	InkX0, InkY0, InkX1, InkY1 float32

	// Layout is incremental: measurements are cached across Compute calls
	// and a node whose subtree is unchanged and whose size did not change is
	// not re-arranged. Whoever changes a node's Style, Measure output or
	// Children must call Invalidate on it and on each of its ancestors.
	cache      [6]cacheEntry
	cacheN     uint8
	pctReady   bool
	pctW, pctH bool
	arranged   bool    // children positions are valid for arrW x arrH
	arrW, arrH float32 // size at the last arrange
}

// Invalidate discards n's cached measurements and arrangement. Call it on a
// changed node and every ancestor up to the root; new nodes start invalid.
func (n *Node) Invalidate() {
	n.cacheN = 0
	n.pctReady = false
	n.arranged = false
}

type cacheEntry struct {
	cbW, cbH, availW, forcedW, forcedH float32
	w, h                               float32
}

// Engine runs layout passes. Reuse one Engine across frames to keep its
// scratch memory.
type Engine struct {
	scratch [][]item
	depth   int

	viewW, viewH float32 // the viewport, for Floating nodes

	// Stats from the last Compute call.
	Measured int // measure calls that did real work
	CacheHit int
	Arranged int // nodes whose children were (re)positioned
}

type item struct {
	n                 *Node
	basis, hyp        float32
	minMain, maxMain  float32
	main, cross       float32
	marginMain        float32
	marginCross       float32
	grow, shrink      float32
	align             Align
	frozen            bool
	forcedCross       float32
	stretchCrossLater bool
}

// Compute lays out root to fill exactly width × height.
//
// Only invalidated subtrees, and subtrees whose size changed, are recomputed.
func (e *Engine) Compute(root *Node, width, height float32) {
	e.Measured, e.CacheHit, e.Arranged = 0, 0, 0
	root.X, root.Y = 0, 0
	root.W, root.H = width, height
	e.viewW, e.viewH = width, height
	e.arrange(root)
}

func clamp(v, lo, hi float32) float32 {
	if v > hi {
		v = hi
	}
	if v < lo {
		v = lo
	}
	return v
}

func orInf(v float32) float32 {
	if isNaN(v) {
		return inf
	}
	return v
}

func orZero(v float32) float32 {
	if isNaN(v) {
		return 0
	}
	return v
}

func same(a, b float32) bool { return a == b || (isNaN(a) && isNaN(b)) }

func (n *Node) padH() float32 {
	return n.Style.Padding.Left + n.Style.Padding.Right + n.Style.Border.Left + n.Style.Border.Right
}
func (n *Node) padV() float32 {
	return n.Style.Padding.Top + n.Style.Padding.Bottom + n.Style.Border.Top + n.Style.Border.Bottom
}

// measure returns n's border-box size.
//
//   - cbW, cbH: containing block content size for percentages (NaN = indefinite)
//   - availW: width available to the margin box, for fit-content sizing (may be Inf)
//   - forcedW, forcedH: exact size imposed by the parent (NaN = not imposed)
func (e *Engine) measure(n *Node, cbW, cbH, availW, forcedW, forcedH float32) (float32, float32) {
	if !n.pctReady {
		n.pctW, n.pctH = n.Style.usesPercent()
		n.pctReady = true
	}
	// The containing block only matters to percentage lengths; dropping it
	// from the key otherwise lets the measure and arrange passes share entries.
	if !n.pctW {
		cbW = nan
	}
	if !n.pctH {
		cbH = nan
	}
	// A forced width makes the available width irrelevant.
	if !isNaN(forcedW) {
		availW = nan
	}
	for i := 0; i < int(n.cacheN); i++ {
		c := &n.cache[i]
		if same(c.cbW, cbW) && same(c.cbH, cbH) && same(c.availW, availW) && same(c.forcedW, forcedW) && same(c.forcedH, forcedH) {
			e.CacheHit++
			return c.w, c.h
		}
	}
	e.Measured++
	w, h := e.computeSize(n, cbW, cbH, availW, forcedW, forcedH)
	slot := n.cacheN
	if slot < uint8(len(n.cache)) {
		n.cacheN++
	} else {
		slot = uint8(e.Measured) % uint8(len(n.cache))
	}
	n.cache[slot] = cacheEntry{cbW, cbH, availW, forcedW, forcedH, w, h}
	return w, h
}

func (e *Engine) computeSize(n *Node, cbW, cbH, availW, forcedW, forcedH float32) (float32, float32) {
	s := &n.Style
	minW, maxW := orZero(s.MinWidth.resolve(cbW)), orInf(s.MaxWidth.resolve(cbW))
	minH, maxH := orZero(s.MinHeight.resolve(cbH)), orInf(s.MaxHeight.resolve(cbH))
	w, h := forcedW, forcedH
	if isNaN(w) {
		w = s.Width.resolve(cbW)
		if !isNaN(w) {
			w = clamp(w, minW, maxW)
		}
	}
	if isNaN(h) {
		h = s.Height.resolve(cbH)
		if !isNaN(h) {
			h = clamp(h, minH, maxH)
		}
	}
	if !isNaN(w) && !isNaN(h) {
		return w, h
	}
	padH, padV := n.padH(), n.padV()
	innerAvail := availW - padH
	if !isNaN(w) {
		innerAvail = w - padH
	} else if !isNaN(maxW) && maxW < inf {
		innerAvail = min(innerAvail, maxW-padH)
	}
	innerAvail = max(innerAvail, 0)

	var cw, ch float32
	if n.Measure != nil {
		cw, ch = n.Measure.Measure(innerAvail)
		// Percentage-sized leaves are compressible: while the containing
		// block is still being sized they contribute nothing (as in CSS),
		// otherwise an image at 100% would force its natural width upward.
		if s.Width.Unit == UnitPercent && isNaN(cbW) {
			cw = 0
		}
		if s.Height.Unit == UnitPercent && isNaN(cbH) {
			ch = 0
		}
	} else {
		innerW, innerH := nan, nan
		if !isNaN(w) {
			innerW = max(w-padH, 0)
		}
		if !isNaN(h) {
			innerH = max(h-padV, 0)
		}
		cw, ch = e.flex(n, innerW, innerH, innerAvail, false)
	}
	if isNaN(w) {
		w = clamp(cw+padH, minW, maxW)
	}
	if isNaN(h) {
		h = clamp(ch+padV, minH, maxH)
	}
	return w, h
}

// arrange positions n's children given n's final W and H.
func (e *Engine) arrange(n *Node) {
	if n.arranged && n.arrW == n.W && n.arrH == n.H {
		return // nothing inside changed and the box is the same size
	}
	n.arranged, n.arrW, n.arrH = true, n.W, n.H
	e.Arranged++
	if n.Measure != nil || len(n.Children) == 0 {
		n.ContentW, n.ContentH = n.W, n.H
		// A leaf's content can run past its box (an unbreakable word, text
		// that does not wrap), so it counts at the width it is drawn in.
		x1, y1 := n.W, n.H
		if n.Measure != nil {
			cw, ch := n.Measure.Measure(max(n.W-n.padH(), 0))
			x1 = max(x1, n.Style.Padding.Left+n.Style.Border.Left+cw)
			y1 = max(y1, n.Style.Padding.Top+n.Style.Border.Top+ch)
		}
		n.setInk(0, 0, x1, y1)
		return
	}
	innerW, innerH := max(n.W-n.padH(), 0), max(n.H-n.padV(), 0)
	e.flex(n, innerW, innerH, innerW, true)
	e.arrangeAbsolute(n)

	// Scrollable extent: farthest child edge plus trailing padding.
	cw, ch := n.W, n.H
	for _, c := range n.Children {
		if c.Style.Hidden {
			c.X, c.Y, c.W, c.H = 0, 0, 0, 0
			continue
		}
		if c.Style.Position == Floating {
			continue
		}
		cw = max(cw, c.X+c.W+c.Style.Margin.Right+n.Style.Padding.Right+n.Style.Border.Right)
		ch = max(ch, c.Y+c.H+c.Style.Margin.Bottom+n.Style.Padding.Bottom+n.Style.Border.Bottom)
	}
	n.ContentW, n.ContentH = cw, ch

	// Ink: the box, and every in-flow or absolute child's ink. A clipping
	// node's children are cut to its box on the clipped axes.
	clipX, clipY := n.Style.OverflowX != OverflowVisible, n.Style.OverflowY != OverflowVisible
	x0, y0, x1, y1 := float32(0), float32(0), n.W, n.H
	for _, c := range n.Children {
		if c.Style.Hidden || c.Style.Position == Floating {
			continue
		}
		if !clipX {
			x0, x1 = min(x0, c.X+c.InkX0), max(x1, c.X+c.InkX1)
		}
		if !clipY {
			y0, y1 = min(y0, c.Y+c.InkY0), max(y1, c.Y+c.InkY1)
		}
	}
	n.setInk(x0, y0, x1, y1)
}

// setInk sets n's ink from the rectangle x0,y0..x1,y1 that its box and
// children cover. The overhang is added on every side, then the result is
// scaled about the box's center, as paint scales the node and everything
// drawn in it.
func (n *Node) setInk(x0, y0, x1, y1 float32) {
	o := n.Overhang
	x0, y0, x1, y1 = x0-o, y0-o, x1+o, y1+o
	if g := n.GrowOverhang; g > 0 {
		k := 1 + g
		cx, cy := n.W/2, n.H/2
		x0, x1 = cx+(x0-cx)*k, cx+(x1-cx)*k
		y0, y1 = cy+(y0-cy)*k, cy+(y1-cy)*k
	}
	n.InkX0, n.InkY0, n.InkX1, n.InkY1 = x0, y0, x1, y1
}

// flex runs the flex algorithm over n's in-flow children. innerW/innerH are
// n's content-box size (NaN if not yet known). When arrange is true both are
// definite and children are positioned and recursively arranged. It returns
// the content size the children need.
func (e *Engine) flex(n *Node, innerW, innerH, availInnerW float32, arrange bool) (float32, float32) {
	s := &n.Style
	row := s.Direction == Row

	e.depth++
	if len(e.scratch) < e.depth {
		e.scratch = append(e.scratch, nil)
	}
	items := e.scratch[e.depth-1][:0]
	defer func() {
		e.scratch[e.depth-1] = items[:0]
		e.depth--
	}()

	mainSize, crossSize := innerH, innerW
	cbMain := innerH
	if row {
		mainSize, crossSize = innerW, innerH
		cbMain = innerW
	}
	crossAvail := crossSize
	if isNaN(crossAvail) {
		if row {
			crossAvail = inf
		} else {
			crossAvail = availInnerW
		}
	}

	// 1. Flex basis and hypothetical main size of each item.
	for _, c := range n.Children {
		cs := &c.Style
		if cs.Position != Relative || cs.Hidden {
			continue
		}
		it := item{n: c, grow: cs.Grow, shrink: cs.Shrink, forcedCross: nan}
		if row {
			it.marginMain = cs.Margin.Left + cs.Margin.Right
			it.marginCross = cs.Margin.Top + cs.Margin.Bottom
		} else {
			it.marginMain = cs.Margin.Top + cs.Margin.Bottom
			it.marginCross = cs.Margin.Left + cs.Margin.Right
		}
		it.align = cs.AlignSelf
		if it.align == AlignAuto {
			it.align = s.AlignItems
			if it.align == AlignAuto {
				it.align = AlignStretch
			}
		}
		crossLen := cs.Width
		mainLen := cs.Height
		minL, maxL := cs.MinHeight, cs.MaxHeight
		if row {
			crossLen, mainLen = cs.Height, cs.Width
			minL, maxL = cs.MinWidth, cs.MaxWidth
		}
		if it.align == AlignStretch && crossLen.IsAuto() {
			// Wrapped items stretch to their line, which is only known later.
			if !isNaN(crossSize) && !s.Wrap {
				it.forcedCross = max(crossSize-it.marginCross, 0)
			} else {
				it.stretchCrossLater = true
			}
		}

		it.basis = cs.Basis.resolve(cbMain)
		if isNaN(it.basis) {
			it.basis = mainLen.resolve(cbMain)
		}
		contentBasis := isNaN(it.basis)
		if contentBasis {
			it.basis = e.mainContent(c, row, innerW, innerH, crossAvail-it.marginCross, it.forcedCross)
		}
		it.minMain = minL.resolve(cbMain)
		if isNaN(it.minMain) {
			it.minMain = 0
			// Automatic minimum size: items that do not clip keep at least
			// their content size, so scrollable lists don't squash their rows.
			if minL.IsAuto() && c.overflowMain(row) == OverflowVisible {
				it.minMain = -1 // computed lazily, only if shrinking happens
			}
		}
		it.maxMain = orInf(maxL.resolve(cbMain))
		it.hyp = clamp(it.basis, max(it.minMain, 0), it.maxMain)
		items = append(items, it)
	}

	// 2. Break items into lines (one line unless Wrap is set).
	limit := mainSize
	if isNaN(limit) {
		limit = inf
		if row {
			limit = availInnerW
		}
	}
	var lineBuf [4]flexLine
	lines := lineBuf[:0]
	if !s.Wrap {
		lines = append(lines, flexLine{0, len(items), 0, 0})
	} else {
		a, acc := 0, float32(0)
		for i := range items {
			size := items[i].hyp + items[i].marginMain
			if i > a && acc+s.Gap+size > limit+0.01 {
				lines = append(lines, flexLine{a, i, 0, 0})
				a, acc = i, 0
			}
			if i > a {
				acc += s.Gap
			}
			acc += size
		}
		if a < len(items) || len(lines) == 0 {
			lines = append(lines, flexLine{a, len(items), 0, 0})
		}
	}

	// 3. Per line: resolve flexible lengths, then cross sizes.
	var contentMain, contentCross float32
	for li := range lines {
		ln := &lines[li]
		its := items[ln.a:ln.b]
		gaps := s.Gap * float32(max(len(its)-1, 0))
		used := gaps
		for i := range its {
			used += its[i].hyp + its[i].marginMain
		}
		target := mainSize
		if isNaN(target) {
			target = used
			if row && used > availInnerW {
				target = availInnerW // fit-content: shrink into the available width
			}
		}
		free := target - used
		for i := range its {
			its[i].main = its[i].hyp
		}
		if free > 0 {
			e.grow(its, free)
		} else if free < 0 {
			for i := range its {
				it := &its[i]
				if it.minMain < 0 {
					it.minMain = e.autoMin(it.n, row, innerW, innerH, crossAvail-it.marginCross, it.forcedCross, it.basis)
				}
			}
			e.shrink(its, -free)
		}
		for i := range its {
			it := &its[i]
			if !isNaN(it.forcedCross) {
				it.cross = it.forcedCross
			} else {
				it.cross = e.crossContent(it.n, row, innerW, innerH, it.main, crossAvail-it.marginCross)
			}
			ln.main += it.main + it.marginMain
			ln.cross = max(ln.cross, it.cross+it.marginCross)
		}
		ln.main += gaps
		if !s.Wrap && !isNaN(crossSize) {
			ln.cross = crossSize // a single line spans the container
		}
		for i := range its {
			it := &its[i]
			if it.stretchCrossLater {
				it.cross = max(ln.cross-it.marginCross, 0)
				it.forcedCross = it.cross
			}
		}
		contentMain = max(contentMain, ln.main)
		contentCross += ln.cross
	}
	contentCross += s.Gap * float32(max(len(lines)-1, 0)) * boolf(s.Wrap)

	if !arrange {
		if row {
			return contentMain, contentCross
		}
		return contentCross, contentMain
	}

	// 4. Position, line by line.
	padMain, padCross := s.Padding.Top+s.Border.Top, s.Padding.Left+s.Border.Left
	if row {
		padMain, padCross = s.Padding.Left+s.Border.Left, s.Padding.Top+s.Border.Top
	}
	lineStart := padCross
	for _, ln := range lines {
		its := items[ln.a:ln.b]
		freeMain := mainSize - ln.main
		offset, between := float32(0), s.Gap
		if freeMain > 0 && len(its) > 0 {
			k := float32(len(its))
			switch s.Justify {
			case JustifyCenter:
				offset = freeMain / 2
			case JustifyEnd:
				offset = freeMain
			case JustifySpaceBetween:
				if len(its) > 1 {
					between += freeMain / (k - 1)
				}
			case JustifySpaceAround:
				offset = freeMain / k / 2
				between += freeMain / k
			case JustifySpaceEvenly:
				offset = freeMain / (k + 1)
				between += freeMain / (k + 1)
			}
		}
		cursor := padMain + offset
		for i := range its {
			it := &its[i]
			c := it.n
			cs := &c.Style
			mStart, cStart := cs.Margin.Top, cs.Margin.Left
			if row {
				mStart, cStart = cs.Margin.Left, cs.Margin.Top
			}
			crossPos := lineStart + cStart
			switch it.align {
			case AlignCenter:
				crossPos += (ln.cross - it.cross - it.marginCross) / 2
			case AlignEnd:
				crossPos += ln.cross - it.cross - it.marginCross
			}
			if row {
				c.X, c.Y, c.W, c.H = cursor+mStart, crossPos, it.main, it.cross
			} else {
				c.X, c.Y, c.W, c.H = crossPos, cursor+mStart, it.cross, it.main
			}
			cursor += it.main + it.marginMain + between
			e.arrange(c)
		}
		lineStart += ln.cross + s.Gap
	}
	return 0, 0
}

// flexLine is a range of items laid out on one line, with its total main
// size and its cross size.
type flexLine struct {
	a, b        int
	main, cross float32
}

func boolf(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

func (n *Node) overflowMain(row bool) Overflow {
	if row {
		return n.Style.OverflowX
	}
	return n.Style.OverflowY
}

// mainContent measures an item's content size along the main axis.
func (e *Engine) mainContent(c *Node, row bool, innerW, innerH, crossAvail, forcedCross float32) float32 {
	if row {
		w, _ := e.measure(c, innerW, innerH, inf, nan, forcedCross)
		return w
	}
	_, h := e.measure(c, innerW, innerH, crossAvail, forcedCross, nan)
	return h
}

// autoMin is the content-based minimum main size of an item.
func (e *Engine) autoMin(c *Node, row bool, innerW, innerH, crossAvail, forcedCross, basis float32) float32 {
	if row {
		// min-content width: the widest unbreakable piece.
		w, _ := e.measure(c, innerW, innerH, 0, nan, forcedCross)
		if !c.Style.Width.IsAuto() {
			w = min(w, basis)
		}
		return w
	}
	_, h := e.measure(c, innerW, innerH, crossAvail, forcedCross, nan)
	if !c.Style.Height.IsAuto() {
		h = min(h, basis)
	}
	return h
}

// crossContent measures an item's cross size once its main size is fixed.
func (e *Engine) crossContent(c *Node, row bool, innerW, innerH, main, crossAvail float32) float32 {
	if row {
		_, h := e.measure(c, innerW, innerH, inf, main, nan)
		return h
	}
	w, _ := e.measure(c, innerW, innerH, crossAvail, nan, main)
	return w
}

func (e *Engine) grow(items []item, free float32) {
	for i := range items {
		items[i].frozen = items[i].grow <= 0
	}
	for iter := 0; iter < 4 && free > 0.01; iter++ {
		var total float32
		for i := range items {
			if !items[i].frozen {
				total += items[i].grow
			}
		}
		if total <= 0 {
			return
		}
		share := free
		if total < 1 {
			share = free * total // fractional grow sums take only that fraction
			total = 1
		}
		free = 0
		for i := range items {
			it := &items[i]
			if it.frozen {
				continue
			}
			want := it.main + share*it.grow/total
			if want > it.maxMain {
				free += want - it.maxMain
				want = it.maxMain
				it.frozen = true
			}
			it.main = want
		}
	}
}

func (e *Engine) shrink(items []item, overflow float32) {
	for i := range items {
		items[i].frozen = items[i].shrink <= 0 || items[i].main <= items[i].minMain
	}
	for iter := 0; iter < 4 && overflow > 0.01; iter++ {
		var total float32
		for i := range items {
			if !items[i].frozen {
				total += items[i].shrink * items[i].basis
			}
		}
		if total <= 0 {
			return
		}
		remaining := overflow
		overflow = 0
		for i := range items {
			it := &items[i]
			if it.frozen {
				continue
			}
			want := it.main - remaining*it.shrink*it.basis/total
			if want < it.minMain {
				overflow += it.minMain - want
				want = it.minMain
				it.frozen = true
			}
			it.main = want
		}
	}
}

func (e *Engine) arrangeAbsolute(n *Node) {
	s := &n.Style
	pbW := n.W - s.Border.Left - s.Border.Right
	pbH := n.H - s.Border.Top - s.Border.Bottom
	for _, c := range n.Children {
		cs := &c.Style
		if cs.Hidden {
			continue
		}
		if cs.Position == Floating {
			w, h := e.measure(c, e.viewW, e.viewH, e.viewW, nan, nan)
			c.X, c.Y, c.W, c.H = 0, 0, w, h
			e.arrange(c)
			continue
		}
		if cs.Position != Absolute {
			continue
		}
		left, right := cs.Left.resolve(pbW), cs.Right.resolve(pbW)
		top, bottom := cs.Top.resolve(pbH), cs.Bottom.resolve(pbH)
		mh := cs.Margin.Left + cs.Margin.Right
		mv := cs.Margin.Top + cs.Margin.Bottom
		fw, fh := nan, nan
		if cs.Width.IsAuto() && !isNaN(left) && !isNaN(right) {
			fw = max(pbW-left-right-mh, 0)
		}
		if cs.Height.IsAuto() && !isNaN(top) && !isNaN(bottom) {
			fh = max(pbH-top-bottom-mv, 0)
		}
		avail := pbW - mh
		if !isNaN(left) {
			avail -= left
		}
		w, h := e.measure(c, pbW, pbH, max(avail, 0), fw, fh)
		x := s.Border.Left + s.Padding.Left + cs.Margin.Left
		switch {
		case !isNaN(left):
			x = s.Border.Left + left + cs.Margin.Left
		case !isNaN(right):
			x = s.Border.Left + pbW - right - w - cs.Margin.Right
		}
		y := s.Border.Top + s.Padding.Top + cs.Margin.Top
		switch {
		case !isNaN(top):
			y = s.Border.Top + top + cs.Margin.Top
		case !isNaN(bottom):
			y = s.Border.Top + pbH - bottom - h - cs.Margin.Bottom
		}
		c.X, c.Y, c.W, c.H = x, y, w, h
		e.arrange(c)
	}
}

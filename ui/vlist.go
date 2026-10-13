package ui

import (
	"fmt"
	"math"
	"strconv"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
)

// ListElement is a scrolling list whose rows may differ in height. Create one
// with ListOf.
type ListElement struct {
	ident      unsafe.Pointer // backing array of the items, compared to detect a change
	count      int
	keyAt      func(i int) string
	rowAt      func(i int) Element
	key        string
	selectable bool // see SelectionContainer
	style      Style
	handle     *ListHandle
	bottom     bool
	onAtEnd    func(atEnd bool)
	size       sizing // see the sizing methods
}

// The sizing methods give the list's outer box a size, over its Style. See Sizing
// in docs/styling.md.
func (l *ListElement) Width(v Length) *ListElement          { l.size.setWidth(v); return l }
func (l *ListElement) Height(v Length) *ListElement         { l.size.setHeight(v); return l }
func (l *ListElement) MinWidth(v Length) *ListElement       { l.size.setMinWidth(v); return l }
func (l *ListElement) MinHeight(v Length) *ListElement      { l.size.setMinHeight(v); return l }
func (l *ListElement) MaxWidth(v Length) *ListElement       { l.size.setMaxWidth(v); return l }
func (l *ListElement) MaxHeight(v Length) *ListElement      { l.size.setMaxHeight(v); return l }
func (l *ListElement) Grow(f float32) *ListElement          { l.size.setGrow(f); return l }
func (l *ListElement) GridSpan(cols, rows int) *ListElement { l.size.setSpan(cols, rows); return l }

// ListOf creates a scrolling list of items whose rows may differ in height.
// Only rows near the viewport are built. Each row's height is measured once
// it is built and remembered by its key; rows not yet seen are estimated from
// the mean of the measured ones. The scroll position stays on the item at the
// top of the viewport, so measurements, inserts and removals above it do not
// make the view jump.
//
// row gets the item and its index in items, as Map's function does, for
// striping or a position such as "3 of 10".
//
// Changes are detected by slice identity: pass a new slice to change the
// items, as with signals. Changing a slice in place is not detected:
//
//	ui.ListOf(notes, func(n *store.Note) string { return n.ID }, func(_ int, n *store.Note) ui.Element {
//		return noteRow(n)
//	}).Style(ui.Style{Grow: 1})
func ListOf[T any](items []T, key func(T) string, row func(i int, item T) Element) *ListElement {
	return &ListElement{
		ident: unsafe.Pointer(unsafe.SliceData(items)),
		count: len(items),
		keyAt: func(i int) string { return key(items[i]) },
		rowAt: func(i int) Element { return row(i, items[i]) },
	}
}

func (*ListElement) isElement()           {}
func (l *ListElement) elementKey() string { return l.key }

// Key gives the list a stable identity among its siblings.
func (l *ListElement) Key(k string) *ListElement { l.key = k; return l }

// Style is merged into the scroll container: size it with Height, Grow or MaxHeight.
func (l *ListElement) Style(s Style) *ListElement { l.style = l.style.Merge(s); return l }

// Handle connects h so the list can be scrolled from code.
func (l *ListElement) Handle(h *ListHandle) *ListElement { l.handle = h; return l }

// SelectionContainer makes the list the group that holds its rows as selection
// items, such as the notes in a sidebar. Rows that call AccessSelected are the items.
func (l *ListElement) SelectionContainer() *ListElement { l.selectable = true; return l }

// Bottom is for chat-like lists. When the rows are shorter than the viewport
// they sit at its bottom. While the list is at its end, appended items (and
// rows growing) keep it there.
func (l *ListElement) Bottom() *ListElement { l.bottom = true; return l }

// OnAtEnd calls fn when the list moves to its end or away from it, during the
// paint that changes it. Read it into a signal to show a control only while the
// list is away from its end (AtEnd does not notify).
func (l *ListElement) OnAtEnd(fn func(atEnd bool)) *ListElement { l.onAtEnd = fn; return l }

// defaultRowHeight is the estimate for a row before any row has been measured.
const defaultRowHeight = 32

// varState is the persistent side of a ListOf. It lives in the state of the
// list's scroll container, so the scroll position and the row window survive
// rebuilds.
type varState struct {
	// ident is the backing array of the items last indexed. Holding it as an
	// unsafe.Pointer keeps that array alive, so a new slice cannot reuse its
	// address and pass for the same items.
	ident   unsafe.Pointer
	count   int
	keys    []string           // key of each item; a duplicate gets an index-based key
	index   map[string]int     // item position by key
	heights map[string]float32 // measured heights of the keys in the list
	hidx    *heightIndex
	inst    *componentInstance
	handle  *ListHandle
	bottom  bool
	onAtEnd func(atEnd bool)

	from, to         int // rows to build, [from, to); set when the window moves
	windowSet        bool
	builtFrom, built int     // rows the last expand built, [builtFrom, built)
	inner            float32 // viewport height inside the padding, from the last paint
	top              float32 // offset of the spacer in the scroll container
	pad              float32 // space above the rows (Bottom)
	atEnd            bool    // the list was at its end at the last paint
	overflow         bool    // the rows were taller than the viewport at the last paint
	endKnown         bool    // a paint has recorded atEnd
	warned           bool
	rebuilds         int // O(n) index rebuilds, for tests and benchmarks
}

// rebuild reads every item's key and sets up the height index. Heights
// measured for keys still in the list are reused.
func (vs *varState) rebuild(rt *Runtime, l *ListElement) {
	vs.rebuilds++
	n := l.count
	vs.keys = make([]string, n)
	vs.index = make(map[string]int, n)
	dup := ""
	for i := range n {
		k := l.keyAt(i)
		if _, ok := vs.index[k]; ok {
			if dup == "" {
				dup = k
			}
			k = "jamb.dup#" + strconv.Itoa(i)
		}
		vs.keys[i] = k
		vs.index[k] = i
	}
	// The measured heights are kept for the keys still in the list, in a new
	// map: heights of items that left are dropped, so the table stays the size
	// of the list however much it churns.
	hs := make([]float32, n)
	heights := make(map[string]float32, n)
	for i, k := range vs.keys {
		if h, ok := vs.heights[k]; ok {
			hs[i], heights[k] = h, h
		}
	}
	vs.heights = heights
	vs.hidx = newHeightIndex(hs)
	vs.hidx.def = defaultRowHeight * float64(rt.textFactor) // rows grow with the Text size
	vs.ident, vs.count = l.ident, n
	if dup != "" && !vs.warned && rt.warn != nil {
		vs.warned = true
		rt.warn(fmt.Sprintf("duplicate key %q among the items of a ListOf; keys must be unique. "+
			"Later duplicates are identified by their position, so their heights and scroll "+
			"anchor do not follow them when items move. Derive keys from a unique field such as an ID.", dup))
	}
}

// sameItems reports whether l's items are the ones last indexed.
func (vs *varState) sameItems(l *ListElement) bool {
	return vs.hidx != nil && vs.ident == l.ident && vs.count == l.count
}

// measure records the height of item i as it was laid out.
func (vs *varState) measure(i int, h float32) {
	vs.heights[vs.keys[i]] = h
	vs.hidx.set(i, h)
}

// end is the scroll offset at which the list is at its end: the rows and the
// Bottom padding, less the viewport.
func (vs *varState) end() float64 {
	return max(vs.hidx.total()+float64(vs.pad)-float64(vs.inner), 0)
}

// vAnchor is the item at the top of the viewport, taken before the items or
// their heights change. restore moves the scroll position so that the item
// keeps its place on screen.
type vAnchor struct {
	key   string
	idx   int
	off   float32 // distance from the item's top to the scroll offset
	ok    bool
	atEnd bool    // the list was at its end: it stays there
	end   float64 // its end offset, see varState.end
}

// capture takes the anchor from the current scroll offset.
func (vs *varState) capture(st *nodeState, atEnd bool) vAnchor {
	// A list that fits its viewport is at its end trivially. It follows the
	// end only when it is a Bottom list or it overflows; otherwise growing it
	// would scroll it to the end and hide its first rows.
	a := vAnchor{atEnd: atEnd && (vs.bottom || vs.overflow)}
	if vs.hidx == nil {
		return a
	}
	a.end = vs.end()
	if vs.count == 0 || vs.pad > 0 {
		return a // nothing to anchor: the rows fit in the viewport
	}
	y := float64(st.targetY - vs.top)
	idx := vs.hidx.find(y)
	a.idx, a.key, a.ok = idx, vs.keys[idx], true
	a.off = float32(y - vs.hidx.prefix(idx))
	return a
}

// restore moves the scroll offset (and its target) by the amount the anchor
// item moved, or to the end if the list was at its end. An anchor whose key is
// gone resolves to the item now at its old index.
func (vs *varState) restore(st *nodeState, a vAnchor) {
	var t float32
	switch {
	case vs.count == 0:
		t = 0
	case a.atEnd:
		t = st.targetY + float32(vs.end()-a.end)
	case a.ok:
		idx, ok := vs.index[a.key]
		if !ok {
			idx = min(a.idx, vs.count-1)
		}
		t = vs.top + float32(vs.hidx.prefix(idx)) + a.off
	default:
		return
	}
	if d := t - st.targetY; d != 0 {
		st.targetY += d
		st.scrollY += d
	}
}

func (l *ListElement) expand(ctx *Context) Element {
	rt := ctx.rt
	st := rt.state(expandedID(ctx.id))
	if st.varList == nil {
		st.varList = &varState{heights: map[string]float32{}}
	}
	vs := st.varList
	vs.inst, vs.handle, vs.bottom, vs.onAtEnd = ctx.inst, l.handle, l.bottom, l.onAtEnd
	if l.handle != nil {
		l.handle.rt, l.handle.id = rt, expandedID(ctx.id)
	}
	changed := !vs.sameItems(l)
	var a vAnchor
	if changed {
		a = vs.capture(st, vs.atEnd)
		vs.rebuild(rt, l)
	}
	vs.pad = 0
	if vs.bottom {
		vs.pad = max(vs.inner-float32(vs.hidx.total()), 0)
	}
	if changed {
		vs.restore(st, a)
	}

	// Until the window is known, only row 0 is built: it is measured first.
	from, to := 0, min(vs.count, 1)
	if vs.windowSet {
		from = min(max(vs.from, 0), vs.count)
		to = max(min(vs.to, vs.count), from)
	}
	spacer := Div().Key("jamb.list.spacer").Shrink(0).Height(Px(vs.pad + float32(vs.hidx.total())))
	for i := from; i < to; i++ {
		top := vs.pad + float32(vs.hidx.prefix(i))
		r := l.rowAt(i)
		wrap := Div().Key("jamb.vrow:" + vs.keys[i]).Absolute().Top(top).Left(0).Right(0).Child(r)
		setListPosition(wrap, r, i+1, vs.count)
		spacer.Child(wrap)
	}
	vs.builtFrom, vs.built = from, to
	container := l.size.apply(Div().ScrollY().Style(l.style)).Role(access.RoleList)
	container.access().container = l.selectable
	container.vrows = vs
	return container.Child(spacer)
}

// setAtEnd records whether the list is at its end. onAtEnd is called on the
// first paint and then on each change.
func (vs *varState) setAtEnd(atEnd bool) {
	if vs.endKnown && atEnd == vs.atEnd {
		return
	}
	vs.endKnown, vs.atEnd = true, atEnd
	if vs.onAtEnd != nil {
		vs.onAtEnd(atEnd)
	}
}

// markVList schedules a rebuild of the rows before the current Frame returns.
func (rt *Runtime) markVList(vs *varState) {
	vs.inst.invalidate()
	rt.listStale = true
}

// checkVList runs after a ListOf's scroll container is painted at offset sy.
// It measures the rows just laid out. If any height changed, the scroll
// position is moved to keep the anchor item in place and the rows are rebuilt.
// It then applies a pending ScrollToItem or ScrollToEnd, and works out the
// window of rows in view, including the one a scroll is heading toward.
func (rt *Runtime) checkVList(n *node, sy float32) {
	vs := n.div.vrows
	st := n.st
	if len(n.children) == 0 {
		return
	}
	spacer := n.children[0]
	be := n.style.borderEdges()
	inner := n.lay.H - be.Top - be.Bottom - n.style.Padding.Top - n.style.Padding.Bottom
	if inner != vs.inner {
		vs.inner = inner
		rt.markVList(vs)
	}
	vs.top = spacer.lay.Y
	maxY := n.lay.ContentH - n.lay.H
	vs.overflow = maxY > 0
	if vs.count == 0 {
		vs.setAtEnd(st.targetY >= maxY-1)
		if h := vs.handle; h != nil {
			h.end, h.has = false, false // nothing to scroll to
		}
		return
	}

	// Measure the built rows. Their positions depend on the heights above
	// them, so the anchor is taken first and restored after.
	a := vs.capture(st, st.targetY >= maxY-1)
	changed := false
	snapped := false
	for k, c := range spacer.children {
		i := vs.builtFrom + k
		if h := c.lay.H; h > 0 && math.Abs(float64(h-vs.hidx.hs[i])) > 0.5 {
			vs.measure(i, h)
			changed = true
		}
	}
	if changed {
		vs.restore(st, a)
		sy = st.scrollY // the restore moved the offset too; the window follows it
		rt.markVList(vs)
	}

	// A pending scroll waits until the rows are laid out at their heights.
	if h := vs.handle; h != nil && !changed {
		if h.end {
			snapped = seekY(n, maxY)
			h.end = false
			rt.animating = true
		}
		if h.has {
			i := min(max(h.pending, 0), vs.count-1)
			top := vs.top + vs.pad + float32(vs.hidx.prefix(i))
			snapped = revealRange(n, top, top+float32(vs.hidx.height(i)))
			// Keep the scroll pending until the row is built and measured, so
			// that its height (not the estimate) decides where it lands. A row
			// that is built but has no height is done: waiting would never end.
			built := i >= vs.builtFrom && i < vs.built
			if vs.hidx.hs[i] > 0 || built {
				h.has = false
			}
			rt.animating = true // the target moved: the next frame eases toward it
		}
	}

	// Rows in view, covering both the current offset and the scroll target. A
	// jump of more than a viewport (ScrollToItem far away) covers only the
	// target: building every row in between would cost a frame per row.
	base := vs.top + vs.pad
	lo, hi := viewSpan(n, sy, snapped)
	vf := vs.hidx.find(float64(lo - base))
	vt := min(vs.hidx.find(float64(hi-base))+1, vs.count)
	ov := max(listOverscan, (vt-vf)/2)
	if !vs.windowSet || vf < vs.builtFrom || vt > vs.built || vs.builtFrom < vf-3*ov || vs.built > vt+3*ov {
		vs.from = max(vf-ov, 0)
		vs.to = min(vt+ov, vs.count)
		vs.windowSet = true
		rt.markVList(vs)
	}
	vs.setAtEnd(st.targetY >= maxY-1)
}

// heightIndex holds the height of every item of a ListOf: the measured height
// where one is known and the estimate elsewhere. Two Fenwick (binary indexed)
// trees hold the measured heights and their count, so prefix sums and
// searches take O(log n). Unmeasured items count at the estimate inside each
// query, so a new estimate, which every measurement can change, does not
// rewrite the tree.
type heightIndex struct {
	n      int
	hs     []float32 // measured height per item; 0 when not measured
	sum    []float64 // Fenwick tree of measured heights, 1-based
	cnt    []int     // Fenwick tree of measured counts
	totSum float64
	totCnt int
	def    float64 // the estimate before any item is measured (see defaultRowHeight)
}

// newHeightIndex takes ownership of hs. Its entries are 0 for unmeasured items.
func newHeightIndex(hs []float32) *heightIndex {
	n := len(hs)
	h := &heightIndex{n: n, hs: hs, sum: make([]float64, n+1), cnt: make([]int, n+1), def: defaultRowHeight}
	for i, v := range hs {
		if v > 0 {
			h.sum[i+1], h.cnt[i+1] = float64(v), 1
			h.totSum += float64(v)
			h.totCnt++
		}
	}
	for j := 1; j <= n; j++ {
		if p := j + j&-j; p <= n {
			h.sum[p] += h.sum[j]
			h.cnt[p] += h.cnt[j]
		}
	}
	return h
}

// est is the estimate for unmeasured items: the mean of the measured ones.
func (h *heightIndex) est() float64 {
	if h.totCnt == 0 {
		return h.def
	}
	return h.totSum / float64(h.totCnt)
}

// set records the measured height v (> 0) of item i.
func (h *heightIndex) set(i int, v float32) {
	dv, dc := float64(v), 0
	if old := h.hs[i]; old > 0 {
		dv -= float64(old)
	} else {
		dc = 1
	}
	h.hs[i] = v
	h.totSum += dv
	h.totCnt += dc
	for j := i + 1; j <= h.n; j += j & -j {
		h.sum[j] += dv
		h.cnt[j] += dc
	}
}

// prefix is the top of item i relative to the first item: the heights of the
// items before i.
func (h *heightIndex) prefix(i int) float64 {
	var s float64
	c := 0
	for j := i; j > 0; j -= j & -j {
		s += h.sum[j]
		c += h.cnt[j]
	}
	return s + h.est()*float64(i-c)
}

// total is the height of all the items.
func (h *heightIndex) total() float64 { return h.prefix(h.n) }

// height is item i's measured height, or the estimate.
func (h *heightIndex) height(i int) float64 {
	if h.hs[i] > 0 {
		return float64(h.hs[i])
	}
	return h.est()
}

// find returns the item containing offset y: the last item whose top is at or
// above y. It is 0 for offsets before the first item and n-1 past the end.
func (h *heightIndex) find(y float64) int {
	if h.n == 0 {
		return 0
	}
	est := h.est()
	pos, s, c := 0, 0.0, 0
	step := 1
	for step*2 <= h.n {
		step *= 2
	}
	for ; step > 0; step /= 2 {
		next := pos + step
		if next > h.n {
			continue
		}
		ns, nc := s+h.sum[next], c+h.cnt[next]
		if ns+est*float64(next-nc) <= y {
			pos, s, c = next, ns, nc
		}
	}
	return min(pos, h.n-1)
}

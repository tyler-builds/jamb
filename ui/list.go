package ui

import (
	"fmt"
	"math"

	"github.com/tyler-builds/jamb/access"
)

// UniformListElement is a scrolling list that builds only the rows in view.
// Every row must have the same height: the list measures one row and
// positions the rest from that.
type UniformListElement struct {
	count      int
	row        func(i int) Element
	style      Style
	key        string
	handle     *ListHandle
	selectable bool // see SelectionContainer
	// decorate, when set, adjusts the scroll container. A Tree sets its role,
	// keys and focus with it.
	decorate func(c *DivElement)
	// ownPos leaves the positions of the rows to the rows. A tree item's position
	// is among its siblings, not among the visible rows.
	ownPos bool
	// header is drawn over the top of the rows. It keeps the top of the viewport
	// as the list scrolls, so it stays in view (see DataGrid).
	header *DivElement
	// rowWidth is the width of the rows' content, which is the width of the list
	// or more, so that the list scrolls horizontally as well (see DataGrid).
	rowWidth float32
	// before, when set, runs at paint before the offset of the list is stepped.
	before func(n *node)
	// pin, when set, runs at paint after the offset of the list is stepped to sx,
	// before the rows are painted, so that it can keep cells in place as the rows
	// scroll sideways (see DataGrid's pinned columns).
	pin func(n *node, sx float32)
	// tail, when set, is drawn after the rows, as one more row of their height. It is
	// not one of the items: it has no position in the list (see DataGrid.LoadingMore).
	tail Element
	size sizing // see the sizing methods
}

// The sizing methods give the list's outer box a size, over its Style. See Sizing
// in docs/styling.md.
func (l *UniformListElement) Width(v Length) *UniformListElement { l.size.setWidth(v); return l }

// Height sets the height of the uniform list.
func (l *UniformListElement) Height(v Length) *UniformListElement { l.size.setHeight(v); return l }

// MinWidth sets the smallest width of the uniform list.
func (l *UniformListElement) MinWidth(v Length) *UniformListElement { l.size.setMinWidth(v); return l }

// MinHeight sets the smallest height of the uniform list.
func (l *UniformListElement) MinHeight(v Length) *UniformListElement {
	l.size.setMinHeight(v)
	return l
}

// MaxWidth sets the largest width of the uniform list.
func (l *UniformListElement) MaxWidth(v Length) *UniformListElement { l.size.setMaxWidth(v); return l }

// MaxHeight sets the largest height of the uniform list.
func (l *UniformListElement) MaxHeight(v Length) *UniformListElement {
	l.size.setMaxHeight(v)
	return l
}

// Grow makes the uniform list take a share of the free space on its parent's main axis.
func (l *UniformListElement) Grow(f float32) *UniformListElement { l.size.setGrow(f); return l }

// GridSpan sets how many columns and rows the uniform list covers in its parent's grid.
func (l *UniformListElement) GridSpan(cols, rows int) *UniformListElement {
	l.size.setSpan(cols, rows)
	return l
}

// UniformList creates a list of count rows. row(i) is called only for rows
// near the viewport, so count can be large:
//
//	ui.UniformList(len(events), func(i int) ui.Element { return eventRow(events[i]) }).
//		Style(ui.Style{Grow: 1})
func UniformList(count int, row func(i int) Element) *UniformListElement {
	return &UniformListElement{count: count, row: row}
}

func (*UniformListElement) isElement()           {}
func (l *UniformListElement) elementKey() string { return l.key }

// Key gives the list a stable identity among its siblings.
func (l *UniformListElement) Key(k string) *UniformListElement { l.key = k; return l }

// Style is merged into the scroll container: size it with Height, Grow or MaxHeight.
func (l *UniformListElement) Style(s Style) *UniformListElement { l.style = l.style.Merge(s); return l }

// Handle connects h so the list can be scrolled from code.
func (l *UniformListElement) Handle(h *ListHandle) *UniformListElement { l.handle = h; return l }

// SelectionContainer makes the list the group that holds its rows as selection
// items, such as the notes in a sidebar. Rows that call AccessSelected are the items.
func (l *UniformListElement) SelectionContainer() *UniformListElement { l.selectable = true; return l }

// ListHandle scrolls a UniformList or a ListOf from code.
type ListHandle struct {
	rt      *Runtime
	id      uint64
	pending int
	has     bool
	end     bool // ScrollToEnd, applied by the next paint
}

// NewListHandle creates a handle to pass to UniformList.Handle.
func NewListHandle() *ListHandle { return &ListHandle{} }

// ScrollToItem scrolls the least amount that shows item i completely
// (nothing if it already is). Takes effect on the next frame.
func (h *ListHandle) ScrollToItem(i int) {
	h.pending, h.has, h.end = i, true, false
	h.request()
}

// ScrollToEnd scrolls to the last item. Takes effect on the next frame.
func (h *ListHandle) ScrollToEnd() {
	h.end, h.has = true, false
	h.request()
}

// AtEnd reports whether the list is scrolled to its end, to within a pixel.
// It is false before the first paint.
func (h *ListHandle) AtEnd() bool {
	if h.rt == nil {
		return false
	}
	n := h.rt.byID[h.id]
	if n == nil || n.paintGen == 0 || n.st == nil {
		return false
	}
	return n.st.targetY >= n.lay.ContentH-n.lay.H-1
}

func (h *ListHandle) request() {
	if h.rt != nil {
		h.rt.needPaint = true
		h.rt.requestFrame()
	}
}

// seekY sets the scroll target of container n to y, for a ListHandle. A seek
// of more than a viewport snaps: the offset moves to y at once. Easing across
// that distance would paint frames showing a place where no rows are built.
// Nearer seeks ease as usual. seekY reports whether it snapped.
func seekY(n *node, y float32) bool {
	st := n.st
	st.targetY = max(y, 0)
	if math.Abs(float64(st.targetY-st.scrollY)) > float64(n.lay.H) {
		st.scrollY = st.targetY
		return true
	}
	return false
}

// revealRange scrolls the least amount that shows the content from top to
// bottom in the scroll container n. Nothing moves if it is already shown. It
// reports whether the seek snapped (see seekY).
func revealRange(n *node, top, bottom float32) bool {
	st := n.st
	be := n.style.borderEdges()
	t0 := be.Top + n.style.Padding.Top
	t1 := be.Bottom + n.style.Padding.Bottom
	y := st.targetY
	if top < y+t0 {
		y = top - t0
	} else if bottom > y+n.lay.H-t1 {
		y = bottom - n.lay.H + t1
	}
	if max(y, 0) == st.targetY {
		return false
	}
	return seekY(n, y)
}

// viewSpan is the span of content whose rows are built: the viewport at offset
// sy, and also the scroll target when a scroll is heading there. After a
// snapped seek the span is the target alone, so the rows between are not
// built.
func viewSpan(n *node, sy float32, snapped bool) (lo, hi float32) {
	st := n.st
	if snapped {
		return st.targetY, st.targetY + n.lay.H
	}
	return min(sy, st.targetY), max(sy+n.lay.H, st.targetY+n.lay.H)
}

// listState is the persistent side of a UniformList. It lives in the state of
// the list's scroll container, so the scroll position and the row window
// survive rebuilds.
type listState struct {
	rowH     float32 // height of one row; 0 until measured
	from, to int     // rows to build, [from, to)
	count    int
	handle   *ListHandle
	inst     *componentInstance
	sticky   bool // the header is kept at the top of the viewport
	before   func(n *node)
	pin      func(n *node, sx float32)
}

// listOverscan is the number of rows built beyond the viewport on each side.
const listOverscan = 4

func (l *UniformListElement) expand(ctx *Context) Element {
	st := ctx.rt.state(expandedID(ctx.id))
	if st.list == nil {
		st.list = &listState{}
	}
	ls := st.list
	ls.count, ls.inst, ls.handle = l.count, ctx.inst, l.handle
	ls.sticky, ls.before, ls.pin = l.header != nil, l.before, l.pin
	if l.handle != nil {
		l.handle.rt, l.handle.id = ctx.rt, expandedID(ctx.id)
	}
	// Until a row has been measured, only row 0 is built.
	from, to := 0, min(l.count, 1)
	if ls.rowH > 0 {
		from = min(max(ls.from, 0), l.count)
		to = max(min(ls.to, l.count), from)
	}
	slots := l.count
	if l.tail != nil {
		slots++
	}
	spacer := Div().Key("jamb.list.spacer").Shrink(0).Height(Px(float32(slots) * ls.rowH))
	if l.rowWidth > 0 {
		spacer.Width(Px(l.rowWidth))
	}
	for i := from; i < to; i++ {
		r := l.row(i)
		key := fmt.Sprintf("jamb.row#%d", i)
		if k, ok := r.(keyed); ok && k.elementKey() != "" {
			key = "jamb.row:" + k.elementKey()
		}
		wrap := Div().Key(key).Absolute().Top(float32(i) * ls.rowH).Left(0).Right(0).Child(r)
		if !l.ownPos {
			setListPosition(wrap, r, i+1, l.count)
		}
		spacer.Child(wrap)
	}
	if l.tail != nil && to == l.count {
		spacer.Child(Div().Key("jamb.list.tail").Absolute().Top(float32(l.count) * ls.rowH).Left(0).Right(0).Child(l.tail))
	}
	container := l.size.apply(Div().ScrollY().Style(l.style)).Role(access.RoleList)
	container.access().container = l.selectable
	container.vlist = ls
	if l.decorate != nil {
		l.decorate(container)
	}
	container.Child(spacer)
	if l.header != nil {
		container.Child(l.header)
	}
	return container
}

// markList schedules a rebuild of the list's rows before the current Frame
// returns.
func (rt *Runtime) markList(ls *listState) {
	ls.inst.invalidate()
	rt.listStale = true
}

// checkList runs after the list's scroll container is painted at offset sy.
// It measures a row, applies a pending ScrollToItem, and works out the rows
// in view (including the rows a scroll is heading toward). If the built rows
// do not cover them, the list is marked for a rebuild.
func (rt *Runtime) checkList(n *node, sy float32) {
	if n.div.vrows != nil {
		rt.checkVList(n, sy)
		return
	}
	ls := n.div.vlist
	if ls.count == 0 || len(n.children) == 0 {
		return
	}
	st := n.st
	spacer := n.children[0]
	snapped := false
	if ls.handle != nil && ls.handle.end {
		snapped = seekY(n, n.lay.ContentH-n.lay.H)
		ls.handle.end = false
		rt.animating = true
	}

	// Measure one built row. A new height changes which rows are in view, so
	// fall through and work out the window with it; the list is rebuilt
	// before anything is shown.
	if len(spacer.children) > 0 {
		h := spacer.children[0].lay.H
		if h > 0 && (ls.rowH == 0 || float32(math.Abs(float64(h-ls.rowH))) > 0.5) {
			old := ls.rowH
			ls.rowH = h
			if old > 0 {
				// Keep the same row at the top of the viewport.
				// An offset above the rows (in the padding, such as a DataGrid's header) is
				// not scaled: the rows start at the padding, not at the top of the list.
				rescale := func(y float32) float32 {
					if y <= spacer.lay.Y {
						return max(y, 0)
					}
					return spacer.lay.Y + (y-spacer.lay.Y)*h/old
				}
				st.scrollY, st.targetY = rescale(st.scrollY), rescale(st.targetY)
				sy = st.scrollY
			}
			rt.markList(ls)
		}
	}
	if ls.rowH == 0 {
		return
	}

	if ls.handle != nil && ls.handle.has {
		i := min(max(ls.handle.pending, 0), ls.count-1)
		top := spacer.lay.Y + float32(i)*ls.rowH
		snapped = revealRange(n, top, top+ls.rowH)
		ls.handle.has = false
		rt.animating = true
	}

	// Rows in view, covering both the current offset and the scroll target.
	lo, hi := viewSpan(n, sy, snapped)
	vf := min(max(int(math.Floor(float64((lo-spacer.lay.Y)/ls.rowH))), 0), ls.count)
	vt := min(max(int(math.Ceil(float64((hi-spacer.lay.Y)/ls.rowH))), 0), ls.count)
	ov := max(listOverscan, (vt-vf)/2)
	if vf < ls.from || vt > ls.to || ls.from < vf-3*ov || ls.to > vt+3*ov {
		ls.from = max(vf-ov, 0)
		ls.to = min(vt+ov, ls.count)
		rt.markList(ls)
	}
}

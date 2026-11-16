package ui

import (
	"math"

	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
)

// Selection handles. A finger selects text with a long press (see
// hostLongPress), and the selection then shows a handle at each end, as a
// phone's does: a thin bar the height of the line, with a round knob above the
// start and below the end, in the selection's color. A finger that goes
// down on a handle drags it: that end of the selection follows the finger, by
// characters, past the other end too, and across the texts of a SelectAcross
// group; a text field, or a group in a scroll container, scrolls when the end
// reaches the edge, as a mouse's drag does. When the finger lifts, the text's
// menu opens again, with Copy. The handles go when the selection is cleared,
// when the focus moves to another element, and when the mouse, a key or a
// finger anywhere else is pressed. Only a window that delivers touches (the web
// platform's) shows them: a mouse never does.

const (
	handleBar  = 2  // the width of a handle's bar
	handleKnob = 5  // the radius of its knob
	handleGrab = 22 // how far around the knob and the bar a finger grabs a handle: a target 44 wide
)

// handleState is the selection handles of the runtime.
type handleState struct {
	// owner is the text whose selection shows handles: an editor, a selectable
	// text, or the SelectAcross group of one (see handleKey); 0 for none.
	owner uint64
	// pending is set when a touch went down on a handle (see hostDragsAt),
	// whose press follows.
	pending bool
	// dragging is set while a finger drags a handle; dx and dy are from the
	// finger to the point that the dragged end follows, the middle of its line.
	dragging bool
	dx, dy   float32
	// ends are where the last paint drew the start's and the end's handle.
	ends [2]selEnd
}

// selEnd is an end of a selection as drawn: the caret line at it, in logical
// window coordinates. ok is false for an end not in view, which shows no handle.
type selEnd struct {
	x, top, h float32
	ok        bool
}

// knobY returns the middle of the knob of the start's handle (start set) or
// the end's.
func (e selEnd) knobY(start bool) float32 {
	if start {
		return e.top - handleKnob
	}
	return e.top + e.h + handleKnob
}

// handleKey returns the owner of the selection of n, for handleState: the
// SelectAcross group of a selectable text in one, else n.
func handleKey(n *node) uint64 {
	if n.textEl != nil {
		if g := selectGroup(n); g != nil {
			return g.id
		}
	}
	return n.id
}

// showHandles has the selection of the focused text show handles, after a
// finger selected in it.
func (rt *Runtime) showHandles() {
	if n := rt.byID[rt.focusID]; n != nil && (n.editor != nil || n.textEl != nil && n.st != nil && n.st.sel != nil) {
		rt.handles.owner = handleKey(n)
		rt.needPaint = true
		rt.requestFrame()
	}
}

// hideHandles takes the handles away, as a press that grabs none of them, or a
// key, does.
func (rt *Runtime) hideHandles() {
	if rt.handles.owner != 0 {
		rt.handles.owner = 0
		rt.handles.ends = [2]selEnd{}
		rt.needPaint = true
		rt.requestFrame()
	}
}

// handleOwner returns the text whose selection shows handles: the focused
// one, or the one whose context menu has the focus. nil when there is none.
func (rt *Runtime) handleOwner() *node {
	if rt.handles.owner == 0 {
		return nil
	}
	for _, id := range [2]uint64{rt.focusID, rt.menuHeld} {
		n := rt.byID[id]
		if n == nil || n.st == nil || (n.editor == nil || n.st.edit == nil) && (n.textEl == nil || n.st.sel == nil) {
			continue
		}
		if handleKey(n) == rt.handles.owner {
			return n
		}
	}
	return nil
}

// selectionEnds returns where the selection of n, an editor or a selectable
// text, starts and ends as the last paint laid it out. ok is false when n has
// no selection, or shows none.
func (rt *Runtime) selectionEnds(n *node) (start, end selEnd, ok bool) {
	if n.editor != nil {
		es := n.st.edit
		if !es.hasSelection() || es.layout == nil || n.paintGen != rt.paintGen || rt.composing == n.id && es.comp.text != "" {
			return selEnd{}, selEnd{}, false
		}
		a, b := es.selection()
		rt.selRects = es.layout.SelectionRects(es.drawn.toShown(a), es.drawn.endShown(b), rt.selRects[:0])
		// A single-line field scrolls its text inside its box.
		view := n.clip.Intersect(n.abs)
		return rectEnds(rt.selRects, n.abs.X+es.originX, n.abs.Y+es.originY, func(float32) float32 { return 0 }, view, view)
	}
	if g := selectGroup(n); g != nil {
		return rt.groupEnds(g)
	}
	s := n.st.sel
	a, b := s.bounds()
	if a == b || s.text != n.textEl.text || s.layout == nil || n.paintGen != rt.paintGen {
		return selEnd{}, selEnd{}, false
	}
	l := s.layout
	rt.selRects = l.SelectionRects(a, b, rt.selRects[:0])
	return rectEnds(rt.selRects, n.abs.X+n.style.Padding.Left, n.abs.Y+n.style.Padding.Top,
		func(y float32) float32 { return n.lineOffset(l, lineAtY(l.LineHeight, len(l.Lines), y)) }, n.clip, n.clip)
}

// groupEnds is selectionEnds for the SelectAcross group g: the start in the
// first text the selection covers part of, and the end in the last.
func (rt *Runtime) groupEnds(g *node) (start, end selEnd, ok bool) {
	members := rt.groupMembers(g)
	lo, hi, ok := rt.groupSpan(g, members)
	if !ok {
		return selEnd{}, selEnd{}, false
	}
	first := true
	for i := lo.i; i <= hi.i; i++ {
		a, b := memberRange(members, i, lo, hi)
		if a == b {
			continue
		}
		m := members[i]
		l := rt.memberLayout(m)
		if l == nil || len(l.Lines) == 0 {
			continue
		}
		r := rt.nodeRect(m)
		view := m.clip
		if m.paintGen != rt.paintGen {
			view = scene.Rect{} // not drawn: out of view
		}
		rt.selRects = l.SelectionRects(a, b, rt.selRects[:0])
		s, e, any := rectEnds(rt.selRects, r.X+m.style.Padding.Left, r.Y+m.style.Padding.Top,
			func(y float32) float32 { return m.lineOffsetIn(l, lineAtY(l.LineHeight, len(l.Lines), y), r.W) }, view, view)
		if !any {
			continue
		}
		if first {
			start, first = s, false
		}
		end = e
	}
	return start, end, !first
}

// lineAtY returns the line of a layout of n lines, each lh high, at y.
func lineAtY(lh float32, n int, y float32) int {
	if lh <= 0 || n == 0 {
		return 0
	}
	return min(max(int(y/lh+0.5), 0), n-1)
}

// rectEnds returns the ends of a selection drawn as rects, relative to x, y,
// each line moved right by off of its top: the start of the first rect and
// the end of the last. Each end is ok when its line's middle is in view.
func rectEnds(rects []text.Rect, x, y float32, off func(top float32) float32, startView, endView scene.Rect) (start, end selEnd, ok bool) {
	if len(rects) == 0 {
		return selEnd{}, selEnd{}, false
	}
	f, l := rects[0], rects[len(rects)-1]
	start = selEnd{x: x + off(f.Y) + f.X, top: y + f.Y, h: f.H}
	end = selEnd{x: x + off(l.Y) + l.X + l.W, top: y + l.Y, h: l.H}
	start.ok = endInView(startView, start)
	end.ok = endInView(endView, end)
	return start, end, true
}

// endInView reports whether the middle of the caret line e is in view, with half
// a pixel to spare for an end at the edge of a box.
func endInView(view scene.Rect, e selEnd) bool {
	mx, my := e.x, e.top+e.h/2
	return !view.Empty() && mx >= view.X-0.5 && mx <= view.Right()+0.5 && my >= view.Y && my <= view.Bottom()
}

// handlesInOverlay reports whether the text with handles is inside an overlay,
// such as a popover, whose paint comes after the rest of the window's: its
// handles are drawn after it, and other handles under the overlays.
func (rt *Runtime) handlesInOverlay() bool {
	n := rt.handleOwner()
	for a := n; a != nil; a = a.parent {
		if a.flags&flagOverlay != 0 {
			return true
		}
	}
	return false
}

// paintHandles draws the handles of the selection that shows them, and
// records where, for handleAt. A selection that has gone takes its handles
// with it, unless a handle is being dragged, whose end may meet the other.
func (rt *Runtime) paintHandles() {
	h := &rt.handles
	h.ends = [2]selEnd{}
	if h.owner == 0 {
		return
	}
	n := rt.handleOwner()
	var start, end selEnd
	ok := false
	if n != nil {
		start, end, ok = rt.selectionEnds(n)
	}
	if !ok {
		if !h.dragging {
			h.owner = 0
		}
		return
	}
	// The selection's color, solid: a field's own, or the accent of a text's.
	col := rt.theme.Accent
	if n.editor != nil && !n.editor.selection.IsZero() {
		col = n.editor.selection
	}
	col.A = 1
	c := col.scene()
	for i, e := range [2]selEnd{start, end} {
		if !e.ok {
			continue
		}
		h.ends[i] = e
		rt.scene.AddQuad(scene.Quad{Bounds: rt.px(scene.Rect{X: e.x - handleBar/2, Y: e.top, W: handleBar, H: e.h}),
			Background: c, Radii: scene.Uniform(handleBar / 2 * rt.scale)})
		ky := e.knobY(i == 0)
		knob := scene.Rect{X: e.x - handleKnob, Y: ky - handleKnob, W: 2 * handleKnob, H: 2 * handleKnob}
		// Not snapped to the pixel grid, which would make the knob an oval.
		s := rt.scale
		rt.scene.AddQuad(scene.Quad{Bounds: scene.Rect{X: knob.X * s, Y: knob.Y * s, W: knob.W * s, H: knob.H * s},
			Background: c, Radii: scene.Uniform(handleKnob * s)})
	}
}

// handleAt returns the handle a finger at x, y, in logical window
// coordinates, grabs: 0 for the start's, 1 for the end's, -1 for none. A
// finger grabs a handle within handleGrab of its knob or its bar; between two
// that both reach, the one with the nearer knob.
func (rt *Runtime) handleAt(x, y float32) int {
	best, bestD := -1, float32(math.Inf(1))
	for i, e := range rt.handles.ends {
		if !e.ok {
			continue
		}
		ky := e.knobY(i == 0)
		top, bottom := ky-handleGrab, e.top+e.h
		if i == 1 {
			top, bottom = e.top, ky+handleGrab
		}
		if x < e.x-handleGrab || x > e.x+handleGrab || y < top || y > bottom {
			continue
		}
		if d := (x-e.x)*(x-e.x) + (y-ky)*(y-ky); d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// startHandleDrag starts the drag of the handle under a finger that went down
// at x, y, in logical window coordinates, and reports whether there was one.
// The other end of the selection stays where it is, as its anchor, and the
// dragged end follows the finger, as the caret of a mouse's drag does: the
// text's own handlers move it (see editorMouseMove, selectMouseMove), with
// the press captured by the text. An open context menu closes, and opens
// again when the finger lifts (see endHandleDrag).
func (rt *Runtime) startHandleDrag(x, y float32) bool {
	i := rt.handleAt(x, y)
	n := rt.handleOwner()
	if i < 0 || n == nil {
		return false
	}
	e := rt.handles.ends[i]
	rt.closeContextMenu(true)
	if rt.focusID != n.id {
		rt.setFocus(n.id)
	}
	switch g := selectGroup(n); {
	case n.editor != nil:
		es := n.st.edit
		a, b := es.selection()
		es.anchor, es.caret = a, b
		if i == 0 {
			es.anchor, es.caret = b, a
		}
		es.dragging, es.havePrefX = true, false
		es.wordA, es.wordB = 0, 0
	case n.textEl != nil && g != nil:
		gs := rt.groupState(g)
		members := rt.groupMembers(g)
		lo, hi, ok := rt.groupSpan(g, members)
		if !ok {
			return false
		}
		at := func(p memberPos) selPoint { return selPoint{members[p.i].id, p.off} }
		gs.anchor, gs.caret = at(lo), at(hi)
		if i == 0 {
			gs.anchor, gs.caret = at(hi), at(lo)
		}
		gs.wordA, gs.wordB = selPoint{}, selPoint{}
		n.st.sel.dragging = true
		n.st.sel.wordA, n.st.sel.wordB = 0, 0
	default:
		s := n.st.sel
		a, b := s.bounds()
		s.anchor, s.caret = a, b
		if i == 0 {
			s.anchor, s.caret = b, a
		}
		s.dragging = true
		s.wordA, s.wordB = 0, 0
	}
	h := &rt.handles
	h.dragging = true
	h.dx, h.dy = e.x-x, e.top+e.h/2-y
	rt.captureID = n.id
	rt.needPaint = true
	rt.requestFrame()
	return true
}

// endHandleDrag ends the drag of a handle by a finger that lifted at x, y:
// the text's menu opens there, as after the long press that selected.
func (rt *Runtime) endHandleDrag(x, y float32) {
	h := &rt.handles
	h.dragging = false
	n := rt.byID[rt.captureID]
	rt.captureID = 0
	if n != nil && n.div != nil && n.div.h.onMouseUp != nil {
		n.div.h.onMouseUp(rt.mouseEvent(n, x, y)) // ends the text's drag
	}
	if n == nil || rt.ctxMenu != nil {
		return
	}
	var path []*node
	for a := n; a != nil; a = a.parent {
		path = append(path, a)
	}
	rt.openTouchMenu(path, x, y)
	rt.needPaint = true
	rt.requestFrame()
}

// openTouchMenu opens the menu of the text a finger selected in, the nearest
// element of path with one, for a finger that lifted at x, y: below the end's
// handle, so that the menu covers neither the selection nor its handles, or
// at x, y when that handle is not in view.
func (rt *Runtime) openTouchMenu(path []*node, x, y float32) {
	t := menuTarget(path)
	if t == nil {
		return
	}
	// The menu hangs from the selection with its handles: below it, or above
	// it where there is no room below, as a phone's does.
	h := float32(0)
	if s, e := rt.handles.ends[0], rt.handles.ends[1]; rt.handles.owner != 0 && (s.ok || e.ok) {
		top, bottom := s.knobY(true)-handleKnob-4, e.knobY(false)+handleKnob+4
		switch {
		case !s.ok:
			top = e.top - 4
		case !e.ok:
			bottom = s.top + s.h + 4
		}
		x, y, h = e.x, top, bottom-top
		if !e.ok {
			x = s.x
		}
	}
	rt.pointerFocus = rt.focusID
	rt.openContextMenu(t, t.div.contextMenu, x, y, false, "")
	if rt.ctxMenu != nil {
		rt.ctxMenu.h = h
	}
}

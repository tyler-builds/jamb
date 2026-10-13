package ui

import "github.com/tyler-builds/jamb/internal/scene"

// Scrollbars are drawn by the runtime for every ScrollX/ScrollY container
// whose content overflows. The thumb can be dragged, and clicking the track
// pages toward the click. The bar thickens while the pointer is over it.

type scrollAxis uint8

const (
	axisY scrollAxis = iota
	axisX
)

const (
	sbTrack    = 12 // hit area thickness
	sbThin     = 6  // thumb thickness at rest
	sbThick    = 9  // thumb thickness while hovered or dragged
	sbInset    = 2
	sbMinThumb = 28
)

type scrollbar struct {
	n            *node
	axis         scrollAxis
	track, thumb scene.Rect
	max          float32 // maximum scroll offset
}

type scrollbarID struct {
	id   uint64
	axis scrollAxis
}

type scrollDrag struct {
	scrollbarID
	grab   float32 // pointer offset into the thumb when the drag started
	active bool
}

// scrollbars returns n's visible scrollbars in logical window coordinates.
func (rt *Runtime) scrollbars(n *node, dst []scrollbar) []scrollbar {
	st := n.st
	if st == nil {
		return dst
	}
	s := &n.style
	be := s.borderEdges()
	r := rt.nodeRect(n)
	in := scene.Rect{X: r.X + be.Left, Y: r.Y + be.Top, W: r.W - be.Left - be.Right, H: r.H - be.Top - be.Bottom}
	maxY := n.lay.ContentH - n.lay.H
	maxX := n.lay.ContentW - n.lay.W
	hasY := s.OverflowY == OverflowScroll && maxY > 0.5
	hasX := s.OverflowX == OverflowScroll && maxX > 0.5
	corner := func(has bool) float32 {
		if has {
			return sbTrack
		}
		return 0
	}
	if hasY {
		track := scene.Rect{X: in.Right() - sbTrack, Y: in.Y + sbInset, W: sbTrack, H: in.H - 2*sbInset - corner(hasX)}
		length := min(max(track.H*n.lay.H/n.lay.ContentH, sbMinThumb), track.H)
		pos := (track.H - length) * min(max(st.scrollY/maxY, 0), 1)
		dst = append(dst, scrollbar{n: n, axis: axisY, track: track, max: maxY,
			thumb: scene.Rect{X: track.X, Y: track.Y + pos, W: track.W, H: length}})
	}
	if hasX {
		track := scene.Rect{X: in.X + sbInset, Y: in.Bottom() - sbTrack, W: in.W - 2*sbInset - corner(hasY), H: sbTrack}
		length := min(max(track.W*n.lay.W/n.lay.ContentW, sbMinThumb), track.W)
		pos := (track.W - length) * min(max(st.scrollX/maxX, 0), 1)
		dst = append(dst, scrollbar{n: n, axis: axisX, track: track, max: maxX,
			thumb: scene.Rect{X: track.X + pos, Y: track.Y, W: length, H: track.H}})
	}
	return dst
}

// scrollbarAt finds the innermost scrollbar under (x, y).
func (rt *Runtime) scrollbarAt(x, y float32) (scrollbar, bool) {
	for _, n := range rt.hitTest(x, y) {
		rt.sbScratch = rt.scrollbars(n, rt.sbScratch[:0])
		for _, sb := range rt.sbScratch {
			if sb.track.Contains(x, y) {
				return sb, true
			}
		}
	}
	return scrollbar{}, false
}

func (rt *Runtime) scrollbarByID(id scrollbarID) (scrollbar, bool) {
	n := rt.byID[id.id]
	if n == nil {
		return scrollbar{}, false
	}
	for _, sb := range rt.scrollbars(n, nil) {
		if sb.axis == id.axis {
			return sb, true
		}
	}
	return scrollbar{}, false
}

// scrollbarDown starts a thumb drag or pages along the track. It reports
// whether the press was on a scrollbar (and so is not passed to elements).
func (rt *Runtime) scrollbarDown(x, y float32) bool {
	sb, ok := rt.scrollbarAt(x, y)
	if !ok {
		return false
	}
	st := sb.n.st
	id := scrollbarID{sb.n.id, sb.axis}
	if sb.thumb.Contains(x, y) {
		grab := y - sb.thumb.Y
		if sb.axis == axisX {
			grab = x - sb.thumb.X
		}
		rt.sbDrag = scrollDrag{scrollbarID: id, grab: grab, active: true}
	} else {
		// Page toward the click, animated like a wheel scroll.
		if sb.axis == axisY {
			page := sb.n.lay.H * 0.9
			if y < sb.thumb.Y {
				page = -page
			}
			st.targetY = min(max(st.targetY+page, 0), sb.max)
		} else {
			page := sb.n.lay.W * 0.9
			if x < sb.thumb.X {
				page = -page
			}
			st.targetX = min(max(st.targetX+page, 0), sb.max)
		}
	}
	rt.needPaint, rt.scrolled = true, true
	rt.requestFrame()
	return true
}

// scrollbarDrag moves the dragged thumb to follow the pointer.
func (rt *Runtime) scrollbarDrag(x, y float32) {
	sb, ok := rt.scrollbarByID(rt.sbDrag.scrollbarID)
	if !ok {
		rt.sbDrag.active = false
		return
	}
	st := sb.n.st
	if sb.axis == axisY {
		room := sb.track.H - sb.thumb.H
		if room > 0 {
			st.targetY = min(max((y-sb.track.Y-rt.sbDrag.grab)/room*sb.max, 0), sb.max)
			st.scrollY = st.targetY // follow the pointer exactly
		}
	} else {
		room := sb.track.W - sb.thumb.W
		if room > 0 {
			st.targetX = min(max((x-sb.track.X-rt.sbDrag.grab)/room*sb.max, 0), sb.max)
			st.scrollX = st.targetX
		}
	}
	rt.needPaint, rt.scrolled = true, true
	rt.requestFrame()
}

func (rt *Runtime) paintScrollbars(n *node, opacity float32) {
	rt.sbScratch = rt.scrollbars(n, rt.sbScratch[:0])
	for _, sb := range rt.sbScratch {
		id := scrollbarID{n.id, sb.axis}
		dragging := rt.sbDrag.active && rt.sbDrag.scrollbarID == id
		hot := dragging || rt.sbHot == id
		c := rt.theme.Scrollbar
		switch {
		case dragging:
		case hot:
			c = c.Alpha(0.85)
		case rt.isHovered(n.id):
			c = c.Alpha(0.6)
		default:
			c = c.Alpha(0.5)
		}
		thick := float32(sbThin)
		if hot {
			thick = sbThick
			rt.scene.AddQuad(scene.Quad{ // faint track while interacting
				Bounds:     rt.px(sb.track),
				Background: rt.theme.Scrollbar.Alpha(0.12 * opacity).scene(),
				Radii:      scene.Uniform(sbTrack / 2 * rt.scale),
			})
		}
		r := sb.thumb
		if sb.axis == axisY {
			r = scene.Rect{X: r.Right() - sbInset - thick, Y: r.Y, W: thick, H: r.H}
		} else {
			r = scene.Rect{X: r.X, Y: r.Bottom() - sbInset - thick, W: r.W, H: thick}
		}
		rt.scene.AddQuad(scene.Quad{
			Bounds:     rt.px(r),
			Background: c.Alpha(opacity).scene(),
			Radii:      scene.Uniform(thick / 2 * rt.scale),
		})
	}
}

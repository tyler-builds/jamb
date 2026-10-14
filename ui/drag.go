package ui

import (
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
)

// Drag and drop is typed: a drag carries a Go value, and a drop target
// receives it only when the value's type matches the target's. Files dropped
// from the operating system arrive as Files.
//
// A drag starts when the pointer moves dragThreshold logical pixels with the
// primary button held on an element with OnDrag. Until then the press is an
// ordinary click. While the drag runs, the source takes its Dragging style,
// the target under the pointer takes its Over style, and an optional Preview
// is drawn under the pointer. Releasing the button drops the payload on the
// target that accepts it; Escape, or the window losing focus, cancels.
const (
	dragThreshold   = 4   // logical pixels the pointer moves before a press becomes a drag
	dragScrollZone  = 32  // logical pixels from a scroll container's edge that scroll it during a drag
	dragScrollSpeed = 900 // logical pixels per second at the very edge of the zone
)

// dragPreviewID is the identity of the component that renders a drag's
// Preview. It hangs off no node of the tree.
var dragPreviewID = mix(rootID, 0xd4a6)

// DragSpec describes a drag: the payload it carries and what is shown while it
// runs. Create one with Drag.
type DragSpec struct {
	payload any
	preview func() Element
	end     func(dropped bool)
	endAt   func(DragEnd)
}

// DragEnd says how a drag ended, for DragSpec.EndAt. Dropped is set when a target took the
// payload, and Cancelled when Escape or the window losing focus ended the drag. Otherwise
// the button was released with no target under the pointer, and X and Y are the pointer's
// position on the screen in logical pixels, which may lie outside the window. They are zero
// when the drag was cancelled.
type DragEnd struct {
	Dropped   bool
	Cancelled bool
	X, Y      float32
}

// Drag makes a payload of type T that an element can be dragged with. Drop
// targets accept it by its Go type, so a Drop[*Note] only receives *Note.
func Drag[T any](v T) *DragSpec {
	if any(v) == nil {
		panic("ui.Drag(nil): the payload is nil; drag a value whose type a ui.Drop[T] target names")
	}
	return &DragSpec{payload: v}
}

// Preview draws fn under the pointer while the drag runs. It is optional, and
// it is not hit-tested: targets under it are found as if it were not there.
func (s *DragSpec) Preview(fn func() Element) *DragSpec { s.preview = fn; return s }

// End runs fn after the drag ends. dropped is true when a target took the
// payload, and false when the drag was cancelled or released over nothing.
func (s *DragSpec) End(fn func(dropped bool)) *DragSpec { s.end = fn; return s }

// EndAt is like End, and also reports where the drag ended, on the screen (see DragEnd).
// Use it to float a panel where the user let go of it.
func (s *DragSpec) EndAt(fn func(DragEnd)) *DragSpec { s.endAt = fn; return s }

// OnDrag makes the element a drag source. The drag starts once the pointer
// moves 4 px with the primary button held; a press that does not move is a
// click, as before.
func (d *DivElement) OnDrag(s *DragSpec) *DivElement { d.dragSpec = s; return d }

// Dragging merges s on top while this element is the source of a drag. It is
// the feedback for an element that is dragged in place, such as dimming it.
func (d *DivElement) Dragging(s Style) *DivElement { d.dragging = mergeInto(d.dragging, s); return d }

// DropEvent describes a drop, or a pointer position over a target during a
// drag. Coordinates are logical pixels; X and Y are relative to the window,
// and LocalX and LocalY relative to the target. W and H are the target's size,
// so a list row can tell which half the pointer is over:
//
//	after := e.LocalY > e.H/2
type DropEvent struct {
	X, Y           float32
	LocalX, LocalY float32
	W, H           float32
	Mods           input.Modifiers
}

// DropTarget is a drop target for one payload type. *DropSpec[T] implements
// it; create one with Drop and attach it with OnDrop.
type DropTarget interface {
	dropTarget() // unexported: only Drop makes a target

	accepts(payload any) bool
	drop(payload any, e *DropEvent)
	move(payload any, e *DropEvent)
	leave()
	overStyle() *Style
}

// DropSpec is a drop target for payloads of type T. Create one with Drop.
type DropSpec[T any] struct {
	onDrop   func(v T, e *DropEvent)
	acceptFn func(v T) bool
	over     *Style
	onMove   func(v T, e *DropEvent)
	onLeave  func()
}

// Drop makes a drop target for payloads of type T (matched with a type
// assertion, so interface types work too). fn receives the payload when it is
// dropped on the target.
func Drop[T any](fn func(v T, e *DropEvent)) *DropSpec[T] {
	return &DropSpec[T]{onDrop: fn}
}

// Accept decides whether the target takes a payload of type T that is being
// dragged over it. It is like GPUI's can_drop. Without it, every payload of
// type T is accepted.
func (s *DropSpec[T]) Accept(fn func(v T) bool) *DropSpec[T] { s.acceptFn = fn; return s }

// Over merges st on top of the target while an accepted drag is over it.
func (s *DropSpec[T]) Over(st Style) *DropSpec[T] { s.over = mergeInto(s.over, st); return s }

// Move runs when the pointer moves over the target during an accepted drag,
// for example to show an insertion line where the payload would go.
func (s *DropSpec[T]) Move(fn func(v T, e *DropEvent)) *DropSpec[T] { s.onMove = fn; return s }

// Leave runs when an accepted drag leaves the target, or ends.
func (s *DropSpec[T]) Leave(fn func()) *DropSpec[T] { s.onLeave = fn; return s }

func (s *DropSpec[T]) dropTarget() {}

func (s *DropSpec[T]) accepts(payload any) bool {
	v, ok := payload.(T)
	return ok && (s.acceptFn == nil || s.acceptFn(v))
}

// drop, move and call only run for a payload that accepts has taken.
func (s *DropSpec[T]) drop(payload any, e *DropEvent) {
	if s.onDrop != nil {
		s.onDrop(payload.(T), e)
	}
}

func (s *DropSpec[T]) move(payload any, e *DropEvent) {
	if s.onMove != nil {
		s.onMove(payload.(T), e)
	}
}

func (s *DropSpec[T]) leave() {
	if s.onLeave != nil {
		s.onLeave()
	}
}

func (s *DropSpec[T]) overStyle() *Style { return s.over }

// OnDrop makes the element a drop target. It may be called several times, for
// payloads of different types; a drop goes to the first target that accepts
// it on the deepest element.
func (d *DivElement) OnDrop(t DropTarget) *DivElement {
	if t == nil {
		panic("ui.OnDrop(nil): the drop target is nil; create one with ui.Drop[T]")
	}
	d.drops = append(d.drops, t)
	return d
}

// Files is the payload of files dropped from the operating system: their
// paths. Drop[Files] receives them; see Runtime.DropFiles.
type Files []string

// dragState is the runtime's drag. A press on a drag source is pending until
// the pointer has moved dragThreshold; then the drag is active until the
// button is released or the drag is cancelled.
type dragState struct {
	pending bool
	active  bool

	source         uint64 // the drag source element
	spec           *DragSpec
	payload        any
	startX, startY float32 // logical pointer position at the press
	x, y           float32 // logical pointer position now
	mods           input.Modifiers

	target     uint64     // element under the pointer that accepts the payload, or 0
	targetSpec DropTarget // its target
	preview    *componentInstance
	// previewStale is set when the preview was rebuilt and must be laid out.
	previewStale bool
}

// notePress records a press on the deepest drag source in path.
func (rt *Runtime) notePress(path []*node, x, y float32) {
	for _, n := range path {
		if n.flags&flagDrag != 0 {
			rt.drag = dragState{pending: true, source: n.id, spec: n.div.dragSpec, startX: x, startY: y}
			return
		}
	}
}

// startDrag makes a pending press an active drag.
func (rt *Runtime) startDrag(x, y float32) {
	d := &rt.drag
	d.pending, d.active = false, true
	d.payload = d.spec.payload
	// While the drag runs, nothing is pressed or hovered, and no element is
	// captured. Hover resumes when the drag ends.
	rt.setPressed(nil)
	rt.captureID = 0
	hover := rt.hoverIDs
	rt.hoverIDs = nil
	for _, id := range hover {
		rt.restyle(id)
	}
	rt.showTip(0)
	rt.tipCandidate = 0
	rt.restyle(d.source)
	if d.spec.preview != nil {
		rt.showPreview(d.spec.preview)
	}
	rt.needPaint = true
	rt.requestFrame()
}

// dragMoved handles pointer motion during an active drag.
func (rt *Runtime) dragMoved(x, y float32, mods input.Modifiers) {
	rt.drag.x, rt.drag.y, rt.drag.mods = x, y, mods
	rt.dropOver()
	if rt.drag.preview != nil {
		rt.needPaint = true
	}
	rt.requestFrame()
}

// dropOver finds the target under the pointer and reports the pointer over it.
func (rt *Runtime) dropOver() {
	rt.retarget()
	if t := rt.drag.targetSpec; t != nil {
		t.move(rt.drag.payload, rt.dropEvent(rt.drag.target, rt.drag.x, rt.drag.y, rt.drag.mods))
	}
}

// retarget finds the element under the pointer that accepts the payload. When
// the target changes, the old one hears Leave and both are restyled. The cursor
// says whether the pointer is over a target.
func (rt *Runtime) retarget() {
	id, t := rt.dropTargetAt(rt.drag.x, rt.drag.y, rt.drag.payload)
	rt.cursor = input.CursorNotAllowed
	if t != nil {
		rt.cursor = input.CursorDefault
	}
	if id == rt.drag.target {
		return
	}
	old := rt.drag.target
	if rt.drag.targetSpec != nil {
		rt.drag.targetSpec.leave()
	}
	rt.drag.target, rt.drag.targetSpec = id, t
	rt.restyle(old)
	rt.restyle(id)
}

// dropTargetAt returns the deepest element under (x, y) with a target that
// accepts payload, and the target. Only the nodes painted by the latest paint
// are considered, so the preview is never a target.
func (rt *Runtime) dropTargetAt(x, y float32, payload any) (uint64, DropTarget) {
	for _, n := range rt.hitTest(x, y) {
		if n.flags&flagDrop == 0 {
			continue
		}
		for _, t := range n.div.drops {
			if t.accepts(payload) {
				return n.id, t
			}
		}
	}
	return 0, nil
}

// dropEvent describes a drop on the element id at window position (x, y).
func (rt *Runtime) dropEvent(id uint64, x, y float32, mods input.Modifiers) *DropEvent {
	var r scene.Rect
	if n := rt.byID[id]; n != nil {
		r = rt.nodeRect(n)
	}
	return &DropEvent{X: x, Y: y, LocalX: x - r.X, LocalY: y - r.Y, W: r.W, H: r.H, Mods: mods}
}

// finishDrag handles the release of the button during a drag: the payload is
// dropped on the target under the pointer, if one accepts it.
func (rt *Runtime) finishDrag(x, y float32, mods input.Modifiers) {
	rt.drag.x, rt.drag.y, rt.drag.mods = x, y, mods
	rt.retarget()
	dropped := false
	if t := rt.drag.targetSpec; t != nil {
		t.drop(rt.drag.payload, rt.dropEvent(rt.drag.target, x, y, mods))
		dropped = true
	}
	rt.endDrag(dropped, false)
}

// cancelDrag ends a drag without a drop (Escape, the window losing focus), or
// forgets a press that has not become a drag.
func (rt *Runtime) cancelDrag() {
	if rt.drag.active {
		rt.endDrag(false, true)
		return
	}
	rt.drag = dragState{}
}

// endDrag ends the active drag: the target hears Leave, the source and target
// restyle, the preview goes, hover resumes, and the source's End runs. A release
// is reported with the pointer's screen position, which the window's position
// turns into; a cancelled drag has none.
func (rt *Runtime) endDrag(dropped, cancelled bool) {
	d := rt.drag
	if d.targetSpec != nil {
		d.targetSpec.leave()
	}
	rt.drag = dragState{}
	rt.restyle(d.target)
	rt.restyle(d.source)
	if d.preview != nil {
		rt.disposePreview(d.preview)
	}
	rt.updateHover()
	rt.needPaint = true
	rt.requestFrame()
	if d.spec.end != nil {
		d.spec.end(dropped)
	}
	if d.spec.endAt != nil {
		e := DragEnd{Dropped: dropped, Cancelled: cancelled}
		if !cancelled {
			ox, oy := rt.screenOrigin()
			e.X, e.Y = d.x+ox, d.y+oy
		}
		d.spec.endAt(e)
	}
}

// screenOrigin returns the top-left corner of the window's client area on the screen, in
// logical pixels. The host gives it (see SetClientOrigin); without a host, the window's own
// position stands in for it, and a runtime with no window is at the origin.
func (rt *Runtime) screenOrigin() (x, y float32) {
	if rt.clientOrigin != nil {
		ox, oy := rt.clientOrigin()
		return float32(ox), float32(oy)
	}
	if c := rt.windowCtl; c != nil {
		ox, oy := c.Position()
		return float32(ox), float32(oy)
	}
	return 0, 0
}

// dropAt delivers a payload released at (x, y), in this window's logical pixels, to the
// target under the pointer that accepts it, as a drop from another window. It reports
// whether a target took it.
func (rt *Runtime) dropAt(x, y float32, payload any) bool {
	id, t := rt.dropTargetAt(x, y, payload)
	if t == nil {
		return false
	}
	t.drop(payload, rt.dropEvent(id, x, y, 0))
	rt.needPaint = true
	rt.requestFrame()
	return true
}

// showPreview builds the drag's preview. It is a component of its own, with
// no parent node, so a signal it reads re-renders only the preview.
func (rt *Runtime) showPreview(preview func() Element) {
	inst := rt.instance(dragPreviewID, Component(func(ctx *Context) Element {
		return Div().Child(Div().Absolute().Left(0).Top(0).Child(preview()))
	}), nil, nil, rt.rootTextStyle())
	rt.drag.preview = inst
	rt.drag.previewStale = true
	rt.layoutPreview()
}

// layoutPreview lays out the preview when it was built or rebuilt.
func (rt *Runtime) layoutPreview() {
	if rt.drag.preview == nil || !rt.drag.previewStale {
		return
	}
	rt.drag.previewStale = false
	rt.engine.Compute(&rt.drag.preview.root.lay, rt.viewW, rt.viewH)
	rt.needPaint = true
}

// disposePreview forgets the preview's component and nodes.
func (rt *Runtime) disposePreview(inst *componentInstance) {
	rt.disposeInstance(inst)
	if inst.root != nil {
		rt.retire(inst.root)
	}
}

// paintDrag paints the preview over everything, centered on the pointer. A
// preview is usually smaller than its source (a chip for a card), so keeping
// the offset at which the source was grabbed would put it far from the pointer.
// Inside a ghost, nothing it draws can be hit, registers a list, or is culled.
func (rt *Runtime) paintDrag(view scene.Rect, dt float32) {
	p := rt.drag.preview
	if p == nil || p.root == nil || len(p.root.children) == 0 {
		return
	}
	n := p.root.children[0]
	x, y := rt.drag.x-n.lay.W/2, rt.drag.y-n.lay.H/2
	prev := rt.inGhost
	rt.inGhost = true
	rt.paintNode(n, x-n.lay.X, y-n.lay.Y, view, 1, rt.theme.Text, dt)
	rt.inGhost = prev
}

// autoscrollDrag scrolls the innermost scroll container under the pointer
// when the pointer is within dragScrollZone of its top or bottom edge (left or
// right for a horizontal one). It reports whether anything scrolled, so the
// caller can re-find the target. Outside the zone nothing animates.
func (rt *Runtime) autoscrollDrag(dt float32) bool {
	if !rt.drag.active {
		return false
	}
	scrolled := false
	vertical, horizontal := false, false
	for _, n := range rt.hitTest(rt.drag.x, rt.drag.y) {
		if n.st == nil {
			continue
		}
		if !vertical && n.style.OverflowY == OverflowScroll {
			vertical = true
			scrolled = rt.scrollAtEdge(n, false, dt) || scrolled
		}
		if !horizontal && n.style.OverflowX == OverflowScroll {
			horizontal = true
			scrolled = rt.scrollAtEdge(n, true, dt) || scrolled
		}
		if vertical && horizontal {
			break
		}
	}
	return scrolled
}

// scrollAtEdge scrolls n toward the pointer's edge, if the pointer is near one.
func (rt *Runtime) scrollAtEdge(n *node, horizontal bool, dt float32) bool {
	r := rt.nodeRect(n)
	be := n.style.borderEdges()
	pos, lo, hi := rt.drag.y, r.Y+be.Top, r.Bottom()-be.Bottom
	maxScroll := max(n.lay.ContentH-n.lay.H, 0)
	cur, scroll := &n.st.targetY, &n.st.scrollY
	if horizontal {
		pos, lo, hi = rt.drag.x, r.X+be.Left, r.Right()-be.Right
		maxScroll = max(n.lay.ContentW-n.lay.W, 0)
		cur, scroll = &n.st.targetX, &n.st.scrollX
	}
	// near is how close to an edge the pointer is, from 0 (out of the zone) to 1.
	near := func(d float32) float32 {
		if d >= dragScrollZone {
			return 0
		}
		return (dragScrollZone - max(d, 0)) / dragScrollZone
	}
	dir := near(hi-pos) - near(pos-lo)
	if dir == 0 {
		return false
	}
	next := min(max(*cur+dir*dragScrollSpeed*dt, 0), maxScroll)
	if next == *cur {
		return false
	}
	*cur, *scroll = next, next
	rt.animating = true
	rt.needPaint = true
	rt.scrolled = true
	return true
}

// DropFiles delivers files dropped from the operating system at (x, y), in
// device-logical pixels like the Pointer methods. The files go to the deepest
// element with a Drop[Files] target. There are no Over, Move or Leave
// callbacks for them: the platform does not report the pointer during a drop
// from outside the app.
func (rt *Runtime) DropFiles(x, y float32, paths []string) {
	x, y = x/rt.zoom, y/rt.zoom
	payload := Files(paths)
	id, t := rt.dropTargetAt(x, y, payload)
	if t == nil {
		return
	}
	t.drop(payload, rt.dropEvent(id, x, y, 0))
}

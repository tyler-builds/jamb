package ui

import (
	"cmp"
	"slices"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// hitTest returns the nodes under (x, y), deepest first.
func (rt *Runtime) hitTest(x, y float32) []*node {
	rt.hitPath = rt.hitPath[:0]
	// Overlays first, topmost last painted. Events on one bubble through its
	// anchor and ancestors, as in the tree.
	for i := len(rt.overlays) - 1; i >= 0; i-- {
		if ov := rt.overlays[i]; !ov.ghost && rt.hit(ov.n, x, y) {
			for p := ov.n.parent; p != nil; p = p.parent {
				rt.hitPath = append(rt.hitPath, p)
			}
			return rt.hitPath
		}
	}
	if rt.tree != nil {
		rt.hit(rt.tree, x, y)
	}
	return rt.hitPath
}

// hit adds n and the element under (x, y) to the hit path, if any. Only the
// nodes painted by the latest paint can be hit; the others were culled or
// have not been painted yet, and their rectangles may be stale. Overlays are
// not in n.painted, since they are hit through the overlay list.
func (rt *Runtime) hit(n *node, x, y float32) bool {
	if n.paintGen != rt.paintGen || !n.clip.Contains(x, y) {
		return false
	}
	for i := len(n.painted) - 1; i >= 0; i-- {
		if rt.hit(n.painted[i], x, y) {
			rt.hitPath = append(rt.hitPath, n)
			return true
		}
	}
	if n.abs.Contains(x, y) {
		rt.hitPath = append(rt.hitPath, n)
		return true
	}
	return false
}

// updateHover recomputes the hovered elements and fires enter/leave handlers.
// During a drag nothing is hovered: the hover state is cleared when the drag
// starts and recomputed when it ends.
func (rt *Runtime) updateHover() {
	if rt.drag.active {
		return
	}
	var path []*node
	hot := scrollbarID{}
	if rt.mouseIn {
		if sb, ok := rt.scrollbarAt(rt.mouseX, rt.mouseY); ok {
			hot = scrollbarID{sb.n.id, sb.axis}
		}
		path = rt.hitTest(rt.mouseX, rt.mouseY)
		if hot.id != 0 {
			// Over a scrollbar: its container is the deepest hovered element.
			for i, n := range path {
				if n.id == hot.id {
					path = path[i:]
					break
				}
			}
		}
	}
	if hot != rt.sbHot {
		rt.sbHot = hot
		rt.needPaint = true
		rt.requestFrame()
	}
	// The rich text segment under the pointer: repainted when it changes, and
	// it sets the cursor when clickable.
	if seg := rt.segmentAt(path, rt.mouseX, rt.mouseY); seg != rt.hoverSeg {
		rt.hoverSeg = seg
		rt.needPaint = true
		rt.requestFrame()
	}
	rt.updateTooltip(path)
	changed := len(path) != len(rt.hoverIDs)
	if !changed {
		for i, n := range path {
			if n.id != rt.hoverIDs[i] {
				changed = true
				break
			}
		}
	}
	if !changed {
		rt.updateCursor(path)
		return
	}
	newIDs := make([]uint64, len(path))
	for i, n := range path {
		newIDs[i] = n.id
	}
	for _, id := range rt.hoverIDs {
		if !contains(newIDs, id) {
			if n := rt.byID[id]; n != nil && n.div != nil && n.div.h.onMouseLeave != nil {
				n.div.h.onMouseLeave()
			}
		}
	}
	for _, n := range path {
		if !contains(rt.hoverIDs, n.id) && n.div != nil && n.div.h.onMouseEnter != nil {
			n.div.h.onMouseEnter()
		}
	}
	old := rt.hoverIDs
	rt.hoverIDs = newIDs
	// Restyle only the elements whose hover state flipped.
	for _, id := range old {
		if !contains(newIDs, id) {
			rt.restyle(id)
		}
	}
	for _, id := range newIDs {
		if !contains(old, id) {
			rt.restyle(id)
		}
	}
	rt.updateCursor(path)
}

// updateTooltip tracks which element with a tooltip the pointer rests on,
// starting the delay when it changes and hiding a tooltip that no longer
// applies.
func (rt *Runtime) updateTooltip(path []*node) {
	id := uint64(0)
	if len(rt.pressIDs) == 0 {
		for _, n := range path {
			if n.div != nil && n.div.tooltip != "" {
				id = n.id
				break
			}
		}
	}
	if id == rt.tipCandidate {
		return
	}
	rt.tipCandidate = id
	rt.tipDismissed = 0
	rt.tipAt = rt.now().Add(tooltipDelay)
	if id != 0 {
		rt.wakeAt(rt.tipAt)
	}
	if rt.tipShown != 0 && rt.tipShown != id {
		rt.showTip(0)
	}
}

// clickOutside tells every listener that the press at path was outside it.
// For an overlay, its anchor counts as inside.
func (rt *Runtime) clickOutside(path []*node) {
	in := func(id uint64) bool {
		for _, n := range path {
			if n.id == id {
				return true
			}
		}
		return false
	}
	for _, o := range append([]*node(nil), rt.outside...) {
		if in(o.id) || (o.div.overlay != 0 && o.parent != nil && in(o.parent.id)) {
			continue
		}
		if fn := o.div.h.onClickOutside; fn != nil {
			fn()
		}
	}
}

func (rt *Runtime) updateCursor(path []*node) {
	rt.cursor = input.CursorDefault
	if rt.sbHot.id != 0 {
		return // arrow over scrollbars
	}
	if rt.clickable(rt.hoverSeg) {
		rt.cursor = CursorPointer // a link wins over a clickable row around it
		return
	}
	for _, n := range path {
		if n.style.Cursor != CursorDefault {
			rt.cursor = n.style.Cursor
			break
		}
	}
}

// setPressed replaces the pressed set, restyling elements that changed.
func (rt *Runtime) setPressed(ids []uint64) {
	old := append([]uint64(nil), rt.pressIDs...)
	rt.pressIDs = append(rt.pressIDs[:0], ids...)
	for _, id := range old {
		if !contains(rt.pressIDs, id) {
			rt.restyle(id)
		}
	}
	for _, id := range rt.pressIDs {
		if !contains(old, id) {
			rt.restyle(id)
		}
	}
}

func (rt *Runtime) mouseEvent(n *node, x, y float32) *MouseEvent {
	r := rt.nodeRect(n)
	return &MouseEvent{X: x, Y: y, LocalX: x - r.X, LocalY: y - r.Y}
}

// PointerMove handles pointer motion in logical pixels.
func (rt *Runtime) PointerMove(x, y float32, mods input.Modifiers) {
	x, y = x/rt.zoom, y/rt.zoom
	rt.mouseX, rt.mouseY, rt.mouseIn = x, y, true
	if rt.sbDrag.active {
		rt.scrollbarDrag(x, y)
		return
	}
	if rt.drag.pending && !rt.drag.active {
		dx, dy := x-rt.drag.startX, y-rt.drag.startY
		if dx*dx+dy*dy >= dragThreshold*dragThreshold {
			rt.startDrag(x, y)
		}
	}
	if rt.drag.active {
		rt.dragMoved(x, y, mods)
		return
	}
	rt.updateHover()
	if n := rt.byID[rt.captureID]; n != nil {
		// A drag in progress: only the capturing element hears about it.
		if n.div.h.onMouseMove != nil {
			e := rt.mouseEvent(n, x, y)
			e.Mods = mods
			n.div.h.onMouseMove(e)
		}
		return
	}
	for _, n := range rt.hitTest(x, y) {
		if n.div != nil && n.div.h.onMouseMove != nil {
			e := rt.mouseEvent(n, x, y)
			e.Mods = mods
			n.div.h.onMouseMove(e)
			if e.stopped {
				break
			}
		}
	}
}

// PointerLeave handles the pointer leaving the window.
func (rt *Runtime) PointerLeave() {
	rt.mouseIn = false
	rt.updateHover()
}

// PointerDown handles a button press.
func (rt *Runtime) PointerDown(x, y float32, b input.MouseButton, clicks int, mods input.Modifiers) {
	x, y = x/rt.zoom, y/rt.zoom
	rt.mouseX, rt.mouseY, rt.mouseIn = x, y, true
	rt.setKeyboardModality(false)
	rt.pointerFocus, rt.altArmed = rt.focusID, false
	rt.updateHover()
	if b == input.ButtonLeft && rt.scrollbarDown(x, y) {
		return
	}
	if rt.drag.active {
		return // a second button during a drag is ignored
	}
	rt.drag = dragState{}
	path := append([]*node(nil), rt.hitTest(x, y)...)
	rt.showTip(0)
	rt.tipCandidate = 0
	rt.clickOutside(path)
	if b == input.ButtonRight && rt.ctxMenu == nil {
		// A context menu that survived the press was pressed inside: it stays.
		if n := menuTarget(path); n != nil {
			e := rt.mouseEvent(n, x, y)
			rt.openContextMenu(n, e.LocalX, e.LocalY, false)
		}
	}
	if b == input.ButtonLeft {
		rt.notePress(path, x, y)
		rt.pressSeg = rt.segmentAt(path, x, y)
		rt.setPressed(rt.hoverIDs)
		focus := uint64(0)
		for _, n := range path {
			if n.div != nil && n.div.focusable {
				focus = n.id
				break
			}
		}
		rt.setFocus(focus)
		for _, n := range path {
			if n.div != nil && n.div.capture {
				rt.captureID = n.id
				break
			}
		}
	}
	for _, n := range path {
		if n.div != nil && n.div.h.onMouseDown != nil {
			e := rt.mouseEvent(n, x, y)
			e.Button, e.Clicks, e.Mods = b, clicks, mods
			n.div.h.onMouseDown(e)
			if e.stopped {
				break
			}
		}
	}
	if clicks == 2 && b == input.ButtonLeft {
		for _, n := range path {
			if n.div != nil && n.div.h.onDoubleClick != nil {
				n.div.h.onDoubleClick()
				break
			}
			if n.div != nil && n.div.overlay != 0 {
				break
			}
		}
	}
}

// PointerUp handles a button release, firing OnClick when the release
// happens over an element that was pressed.
func (rt *Runtime) PointerUp(x, y float32, b input.MouseButton, mods input.Modifiers) {
	x, y = x/rt.zoom, y/rt.zoom
	rt.mouseX, rt.mouseY = x, y
	if rt.sbDrag.active && b == input.ButtonLeft {
		rt.sbDrag.active = false
		rt.needPaint = true
		rt.updateHover()
		rt.requestFrame()
		return
	}
	if rt.drag.active {
		// The release of a drag drops it: no click or mouse-up handlers run.
		if b == input.ButtonLeft {
			rt.finishDrag(x, y, mods)
		}
		return
	}
	if b == input.ButtonLeft {
		rt.drag = dragState{} // a press that never became a drag is a click
	}
	if n := rt.byID[rt.captureID]; n != nil && b == input.ButtonLeft {
		rt.captureID = 0
		if n.div.h.onMouseUp != nil {
			e := rt.mouseEvent(n, x, y)
			e.Button, e.Mods = b, mods
			n.div.h.onMouseUp(e)
		}
	}
	path := append([]*node(nil), rt.hitTest(x, y)...)
	for _, n := range path {
		if n.div != nil && n.div.h.onMouseUp != nil {
			e := rt.mouseEvent(n, x, y)
			e.Button, e.Mods = b, mods
			n.div.h.onMouseUp(e)
			if e.stopped {
				break
			}
		}
	}
	if b != input.ButtonLeft {
		return
	}
	// A link inside a clickable row gets the click, not the row. Only the
	// segment that was pressed counts.
	pressed := rt.pressSeg
	rt.pressSeg = segRef{}
	if seg := rt.segmentAt(path, x, y); seg.node != 0 && seg == pressed && rt.clickable(seg) {
		rt.setPressed(nil)
		rt.byID[seg.node].rich.segs[seg.seg].onClick()
		return
	}
	for _, n := range path {
		if n.div != nil && n.div.h.onClick != nil && rt.isPressed(n.id) {
			n.div.h.onClick()
			break
		}
		if n.div != nil && n.div.overlay != 0 {
			break // a click inside an overlay never clicks its anchor
		}
	}
	rt.setPressed(nil)
}

// Scroll handles wheel/touchpad scrolling at (x, y).
func (rt *Runtime) Scroll(x, y, dx, dy float32, precise bool, mods input.Modifiers) {
	if rt.zoomKeys && mods.Has(input.ModCtrl) && dy != 0 {
		// Ctrl+wheel zooms: one step per wheel notch (60 px), accumulated
		// for touchpads.
		rt.wheelZoom += dy
		for rt.wheelZoom <= -60 {
			rt.wheelZoom += 60
			rt.ZoomIn()
		}
		for rt.wheelZoom >= 60 {
			rt.wheelZoom -= 60
			rt.ZoomOut()
		}
		return
	}
	x, y = x/rt.zoom, y/rt.zoom
	rt.mouseX, rt.mouseY, rt.mouseIn = x, y, true
	path := rt.hitTest(x, y)
	e := &ScrollEvent{DeltaX: dx, DeltaY: dy, Mods: mods}
	for _, n := range path {
		if n.div != nil && n.div.h.onScroll != nil {
			n.div.h.onScroll(e)
			if e.stopped {
				return
			}
		}
	}
	for _, n := range path {
		st := n.st
		if st == nil {
			continue
		}
		maxX := max(n.lay.ContentW-n.lay.W, 0)
		maxY := max(n.lay.ContentH-n.lay.H, 0)
		canY := n.style.OverflowY == OverflowScroll && maxY > 0 && ((dy > 0 && st.targetY < maxY) || (dy < 0 && st.targetY > 0))
		canX := n.style.OverflowX == OverflowScroll && maxX > 0 && ((dx > 0 && st.targetX < maxX) || (dx < 0 && st.targetX > 0))
		if !canX && !canY {
			continue
		}
		if canY {
			st.targetY = min(max(st.targetY+dy, 0), maxY)
		}
		if canX {
			st.targetX = min(max(st.targetX+dx, 0), maxX)
		}
		if precise {
			st.scrollX, st.scrollY = st.targetX, st.targetY
		}
		rt.needPaint = true
		rt.scrolled = true
		rt.requestFrame()
		return
	}
}

// focusPath returns the focused node and its ancestors, or just the root.
func (rt *Runtime) focusPath() []*node {
	var path []*node
	n := rt.byID[rt.focusID]
	if n == nil {
		n = rt.tree
	}
	for ; n != nil; n = n.parent {
		path = append(path, n)
	}
	return path
}

// keyPath is the path keys travel: the focus path, or, while a modal is open,
// only its part inside the modal. The app's bindings and key handlers outside
// the modal then don't run behind it; those inside it (a field's editing
// keys, the modal's own) still do.
func (rt *Runtime) keyPath() []*node {
	path := rt.focusPath()
	m := rt.topModal()
	if m == nil {
		return path
	}
	for i, n := range path {
		if n == m {
			return path[:i+1]
		}
	}
	return []*node{m}
}

// chordTimeout is how long a chord waits for its next stroke.
const chordTimeout = time.Second

// pendingStroke is a stroke held back while a chord waits, with the text its
// key typed (the platform sends that text after the key press).
type pendingStroke struct {
	s    keystroke
	text string
}

// KeyDown handles a key press. Bindings and element handlers run in this
// order, which the README describes:
//
//  1. a chord in progress: a key that starts a longer binding waits, for up
//     to chordTimeout, for the rest of the chord; when the keys held back
//     match nothing, they are replayed and this key is handled again
//  2. all bindings, contextual (BindIn) and global (Bind), in precedence
//     order; the first whose action has a handler on the focus path runs
//  3. OnKeyDown, from the focused element up to the root; then Escape closes
//     the topmost overlay with an OnClickOutside, or hides a tooltip, in place
//     of the field's Cancel (see escapeDismisses); the auto-repeats of that
//     press are ignored, so holding Escape closes one overlay
//  4. Enter or Space (without modifiers, not a repeat) runs the focused
//     element's OnClick; a text field keeps these keys
//  5. Tab (which also reveals the element it moves to), the Menu key and
//     Shift+F10 (which open the focused element's context menu), and the zoom
//     keys
//
// So a key that is both a binding and the start of a longer chord waits up to
// a second before it runs. A chord that starts with a printable key, such as
// "g g", delays typing that key in fields by up to a second; prefer modifiers,
// or a context such as "!Editor".
func (rt *Runtime) KeyDown(k input.Key, mods input.Modifiers, repeat bool) {
	rt.swallowText = false
	rt.altArmed = k == input.KeyUnknown && mods == input.ModAlt
	if k != input.KeyUnknown {
		rt.setKeyboardModality(true)
	}
	if rt.drag.active && k == input.KeyEscape {
		rt.cancelDrag() // Escape cancels a drag before any binding sees it
		return
	}
	if k == input.KeyEscape {
		if repeat && rt.escapeDismissed {
			return
		}
		rt.escapeDismissed = false
	}
	rt.expirePending(rt.now())
	// Modifier keys on their own (and unmapped keys) arrive as KeyUnknown.
	// They never match a binding and must not end a chord: users often let go
	// of Ctrl and press it again between the strokes of Ctrl+K Ctrl+T.
	if k == input.KeyUnknown || !rt.bindingKey(keystroke{k, mods}) {
		rt.keyDown(k, mods, repeat)
	}
}

// setKeyboardModality records whether input is coming from the keyboard, and
// restyles the focused element and repaints its component when that changes,
// so its FocusVisible style and KeyboardFocus follow.
func (rt *Runtime) setKeyboardModality(keyboard bool) {
	if keyboard == rt.keyboardModality {
		return
	}
	rt.keyboardModality = keyboard
	if n := rt.byID[rt.focusID]; n != nil {
		rt.restyle(n.id)
		if n.owner != nil && n.owner.watchesFocus {
			n.owner.invalidate()
		}
	}
}

// keyDown runs the element handlers for a key that no binding took: OnKeyDown
// from the focused element up to the root, then Enter or Space as a click on
// the focused element, then Tab and the zoom keys, unless a handler stops the
// key.
func (rt *Runtime) keyDown(k input.Key, mods input.Modifiers, repeat bool) {
	e := &KeyEvent{Key: k, Mods: mods, Repeat: repeat}
	for _, n := range rt.keyPath() {
		if n.div != nil && n.div.h.onKeyDown != nil {
			n.div.h.onKeyDown(e)
			if e.stopped {
				return
			}
		}
	}
	if k == input.KeyEscape && mods == 0 && rt.escapeDismisses() {
		// A repeat (Escape held since before the overlay opened) does nothing.
		if !repeat {
			rt.escapeDismissed = rt.dismissEscape()
		}
		return
	}
	if !repeat && mods == 0 && (k == input.KeyEnter || k == input.KeySpace) && rt.activateFocused() {
		return
	}
	if !repeat && mods == 0 && k == input.KeyEnter && rt.modalDefault() {
		return
	}
	switch {
	case k == input.KeyTab:
		rt.cycleFocus(mods.Has(input.ModShift))
	case !repeat && (k == input.KeyMenu || (k == input.KeyF10 && mods == input.ModShift)):
		rt.openFocusedMenu()
	case rt.zoomKeys && mods.Has(input.ModCtrl) && k == input.KeyEqual:
		rt.ZoomIn()
	case rt.zoomKeys && mods.Has(input.ModCtrl) && k == input.KeyMinus:
		rt.ZoomOut()
	case rt.zoomKeys && mods.Has(input.ModCtrl) && k == input.Key0:
		rt.ResetZoom()
	}
}

// KeyUp handles a key release. The text a key types arrives between its press
// and its release, so the release also ends any swallowing of text.
func (rt *Runtime) KeyUp(k input.Key, mods input.Modifiers) {
	rt.swallowText = false
	if rt.altArmed {
		rt.altArmed = false
		if k == input.KeyUnknown && !mods.Has(input.ModAlt) && rt.topModal() == nil {
			rt.activateMenuBar() // Alt pressed and released alone
		}
	}
	e := &KeyEvent{Key: k, Mods: mods}
	for _, n := range rt.keyPath() {
		if n.div != nil && n.div.h.onKeyUp != nil {
			n.div.h.onKeyUp(e)
			if e.stopped {
				return
			}
		}
	}
}

// bindingKey handles stroke s with the bindings and any chord in progress. It
// reports whether a binding took the key: it ran, or it started or continued a
// chord. When the strokes held back by a chord match nothing, they are
// replayed, and s is handled again from scratch.
func (rt *Runtime) bindingKey(s keystroke) bool {
	if len(rt.pending) > 0 && rt.focusID != rt.chordFocus {
		rt.pending = nil // focus moved: the chord is dropped, not replayed
	}
	path := rt.keyPath()
	bs := rt.bindingsOn(path)
	if s == (keystroke{input.KeyEscape, 0}) && rt.escapeDismisses() {
		// The field's own Cancel would blur it. While an overlay or tooltip is
		// open, Escape is left to keyDown, which closes that instead.
		bs = slices.DeleteFunc(bs, func(b Binding) bool { return b.action == Cancel })
	}
	seq := make([]keystroke, 0, len(rt.pending)+1)
	for _, p := range rt.pending {
		seq = append(seq, p.s)
	}
	seq = append(seq, s)
	if chordPending(bs, seq) {
		rt.pending = append(rt.pending, pendingStroke{s: s})
		rt.pendingAt, rt.chordFocus = rt.now(), rt.focusID
		rt.swallowText = true
		rt.wakeAt(rt.pendingAt.Add(chordTimeout))
		return true
	}
	if fn := exactRun(path, bs, seq); fn != nil {
		rt.pending = nil
		rt.swallowText = len(seq) > 1 || s.typesText()
		fn()
		return true
	}
	if len(rt.pending) == 0 {
		return false
	}
	held := rt.pending
	rt.pending = nil
	rt.replay(held)
	return rt.bindingKey(s)
}

// chordPending reports whether a binding longer than seq starts with seq and
// no NoAction or Unbind above it turns it off. The turn-offs themselves do not
// count as longer bindings.
func chordPending(bs []Binding, seq []keystroke) bool {
	for i, b := range bs {
		if len(b.strokes) > len(seq) && hasStrokePrefix(b.strokes, seq) && !b.action.disables() && !turnedOff(bs[:i], b) {
			return true
		}
	}
	return false
}

// turnedOff reports whether a NoAction, or an Unbind of b's action, with b's
// keys ranks above b. above is the bindings ranked above b, in order.
func turnedOff(above []Binding, b Binding) bool {
	for _, u := range above {
		if slices.Equal(u.strokes, b.strokes) && (u.action == NoAction || u.action.unbinds == b.action) {
			return true
		}
	}
	return false
}

// exactRun returns the handler of the binding that runs for exactly the
// strokes seq, or nil. bs is in precedence order. A NoAction match stops the
// walk, and an Unbind match removes the later bindings of its action for the
// same keys.
func exactRun(path []*node, bs []Binding, seq []keystroke) func() {
	var unbound []*Action
	for _, b := range bs {
		if !slices.Equal(b.strokes, seq) {
			continue
		}
		switch {
		case b.action == NoAction:
			return nil
		case b.action.unbinds != nil:
			unbound = append(unbound, b.action.unbinds)
		case slices.Contains(unbound, b.action):
			// turned off by an Unbind above
		default:
			if fn := handlerOn(path, b.action); fn != nil {
				return fn
			}
		}
	}
	return nil
}

// expirePending ends a chord that has waited past chordTimeout. If focus is
// still where the chord started, its strokes are replayed; otherwise they are
// dropped.
func (rt *Runtime) expirePending(now time.Time) {
	if len(rt.pending) == 0 || now.Before(rt.pendingAt.Add(chordTimeout)) {
		return
	}
	held := rt.pending
	rt.pending = nil
	if rt.focusID == rt.chordFocus {
		rt.replay(held)
	}
}

// replay runs keys that a chord held back and that did not complete it, as
// GPUI's replay_prefix does. The longest run of them that exactly matches a
// binding runs as that binding. A key that matches nothing goes to the element
// handlers and built-in keys, and its text to the focused field, as if the
// chord had never started.
func (rt *Runtime) replay(held []pendingStroke) {
	for len(held) > 0 {
		if fn, n := rt.longestRun(held); fn != nil {
			fn()
			held = held[n:]
			continue
		}
		first := held[0]
		rt.keyDown(first.s.key, first.s.mods, false)
		if first.text != "" {
			rt.deliverText(first.text)
		}
		held = held[1:]
	}
}

// longestRun returns the handler of the longest prefix of held that exactly
// matches a binding, and the prefix's length, or nil.
func (rt *Runtime) longestRun(held []pendingStroke) (func(), int) {
	for n := len(held); n > 0; n-- {
		seq := make([]keystroke, n)
		for i := range seq {
			seq[i] = held[i].s
		}
		path := rt.keyPath()
		if fn := exactRun(path, rt.bindingsOn(path), seq); fn != nil {
			return fn, n
		}
	}
	return nil, 0
}

// hasStrokePrefix reports whether strokes begins with prefix.
func hasStrokePrefix(strokes, prefix []keystroke) bool {
	return len(strokes) >= len(prefix) && slices.Equal(strokes[:len(prefix)], prefix)
}

// TextInput delivers typed text to the focused element. Text produced by a
// key that a binding consumed is dropped, unless the key is part of a chord:
// then the text is kept with that stroke, to be typed if the chord is replayed.
func (rt *Runtime) TextInput(s string) {
	if rt.swallowText {
		if n := len(rt.pending); n > 0 {
			rt.pending[n-1].text += s
		}
		return
	}
	rt.deliverText(s)
}

// deliverText gives text to the focused element, or else to the nearest
// ancestor that takes text.
func (rt *Runtime) deliverText(s string) {
	for _, n := range rt.keyPath() {
		if n.div != nil && n.div.h.onTextInput != nil {
			n.div.h.onTextInput(s)
			return
		}
	}
}

// WindowFocus reports the window gaining or losing focus.
func (rt *Runtime) WindowFocus(focused bool) {
	rt.focused.Set(focused)
	if !focused {
		rt.setPressed(nil)
		rt.cancelDrag()
	}
}

func (rt *Runtime) setFocus(id uint64) {
	if id == rt.focusID {
		return
	}
	rt.markAccess()
	oldID := rt.focusID
	if old := rt.byID[oldID]; old != nil && old.div != nil && old.div.h.onBlur != nil && rt.blurring != oldID {
		// Focus stays on the old element while its OnBlur runs. A focus change
		// the handler makes then finds blurring set, so it does not call OnBlur
		// for the same element again.
		rt.blurring = oldID
		old.div.h.onBlur()
		rt.blurring = 0
		// OnBlur moved focus on. That change ran the focus callbacks and restyles
		// for its own elements, so this one stops here.
		if rt.focusID != oldID {
			return
		}
	}
	rt.focusID = id
	if n := rt.byID[id]; n != nil && n.div != nil && n.div.h.onFocus != nil {
		n.div.h.onFocus()
	}
	for _, x := range [2]uint64{oldID, id} {
		n := rt.byID[x]
		if n == nil {
			continue
		}
		// The element and its ancestors (FocusWithin styles).
		for a := n; a != nil; a = a.parent {
			rt.restyle(a.id)
		}
		// Composite elements that render differently when focused (the text
		// input's caret) asked to be told.
		if n.owner != nil && n.owner.watchesFocus {
			n.owner.invalidate()
		}
	}
	rt.requestFrame()
}

// tabOrder lists the elements Tab moves through, as HTML and GPUI order them:
// positive TabIndex values first in ascending order (ties in tree order), then
// the rest in tree order. Elements with a negative TabIndex or TabStop(false)
// are left out, though they stay focusable by click or handle. One walk of the
// tree; only the positive indexes are sorted.
func (rt *Runtime) tabOrder() []uint64 {
	type indexed struct {
		index int
		id    uint64
	}
	var first []indexed
	var rest []uint64
	var walk func(n *node)
	walk = func(n *node) {
		if d := n.div; d != nil && d.focusable && !d.noTabStop && d.tabIndex >= 0 {
			if d.tabIndex > 0 {
				first = append(first, indexed{d.tabIndex, n.id})
			} else {
				rest = append(rest, n.id)
			}
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	// While a modal is open, Tab stays inside it.
	root := rt.tree
	if m := rt.topModal(); m != nil {
		root = m
	}
	if root != nil {
		walk(root)
	}
	if len(first) == 0 {
		return rest
	}
	slices.SortStableFunc(first, func(a, b indexed) int { return cmp.Compare(a.index, b.index) })
	ids := make([]uint64, 0, len(first)+len(rest))
	for _, f := range first {
		ids = append(ids, f.id)
	}
	return append(ids, rest...)
}

// cycleFocus moves focus to the next (or, with back, the previous) element of
// the tab order, wrapping around, and reveals it.
func (rt *Runtime) cycleFocus(back bool) {
	ids := rt.tabOrder()
	if len(ids) == 0 {
		return
	}
	i := -1
	for j, id := range ids {
		if id == rt.focusID {
			i = j
		}
	}
	if back {
		i--
		if i < 0 {
			i = len(ids) - 1
		}
	} else {
		i = (i + 1) % len(ids)
	}
	rt.setFocus(ids[i])
	rt.revealFocus = true
	rt.needPaint = true
	rt.requestFrame()
}

// revealTabFocus scrolls the focused element into view after Tab moved focus
// to it. It sets the rectangle that revealCaret scrolls to, unless a text
// field's caret already did. It runs at the end of a paint, when the
// element's position is known, even if the element was culled.
func (rt *Runtime) revealTabFocus() {
	if !rt.revealFocus {
		return
	}
	rt.revealFocus = false
	if n := rt.byID[rt.focusID]; n != nil && rt.revealFrom == nil {
		rt.reveal, rt.revealFrom = n.abs, n
	}
}

// activateFocused runs the OnClick of the focused element, as Enter or Space
// does for a focused button. It reports whether there was one to run. Text
// fields are not elements with OnClick, so their keys are left to typing.
func (rt *Runtime) activateFocused() bool {
	n := rt.byID[rt.focusID]
	if n == nil || n.kind != kindDiv || n.div == nil || n.div.h.onClick == nil {
		return false
	}
	n.div.h.onClick()
	return true
}

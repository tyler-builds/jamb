package ui

import (
	"slices"
)

// A retained component keeps its instance, and with it its Local state, its signal
// subscriptions, its OnDispose functions, its node states and the widgets' state (text,
// scroll offsets, focus), when its element moves to another place in a tree. Its
// identity is its key in a retainHub, not its place: the runtime finds the instance by
// the key wherever the element is built, so a Dock keeps a panel while the panel moves
// between groups and between windows. Each window is a runtime of its own, but all
// runtimes run on the UI thread, so an instance moves from one runtime to another by
// changing the runtime pointers and the maps that hold its state.
//
// An instance is in one of three places: in the tree of a runtime (attached), parked
// (in no tree, while the hub still wants its key), or disposed. An attached instance that
// a runtime's next build does not claim is parked when the hub still keeps its key, and
// disposed when it does not. A parked instance is claimed by the runtime that builds its
// key next; the claim takes it out of the tree it was in, if any. The hub disposes the
// instances of keys it no longer keeps (see sweep). An element without a retainSlot is not
// affected, and costs nothing beyond a nil check.
//
// The hub is used on the UI thread only.

// retainSlot is the retained identity of a ComponentElement: the key that names its
// instance in hub.
type retainSlot struct {
	hub *retainHub
	key string
}

// retainHub holds the retained instances of one dock. live maps each key to its instance,
// attached or parked. keep reports whether a key is still wanted: a parked or attached
// instance whose key is not wanted is disposed.
type retainHub struct {
	live map[string]*componentInstance
	keep func(key string) bool
}

// newRetainHub returns an empty hub that keeps the keys keep reports.
func newRetainHub(keep func(key string) bool) *retainHub {
	return &retainHub{live: map[string]*componentInstance{}, keep: keep}
}

// claim returns the live instance of key, now attached to rt under the node parent: it is
// taken out of the runtime it was attached to, or from its parked state, and put into rt's
// maps. It returns nil when there is no live instance.
func (h *retainHub) claim(rt *Runtime, key string, parent *node) *componentInstance {
	inst := h.live[key]
	if inst == nil {
		return nil
	}
	if !inst.parked && inst.rt != rt {
		inst.rt.park(inst)
	}
	rt.adopt(inst, parent)
	return inst
}

// sweep disposes the live instances whose keys the hub no longer keeps, such as the panels
// that a layout closed. Their OnDispose functions run once.
func (h *retainHub) sweep() {
	for key, inst := range h.live {
		if !h.keep(key) {
			inst.rt.disposeInstance(inst)
		}
	}
}

// retireChild gives up the instance c, which the build of its owner did not claim. A
// retained instance whose key the hub keeps is parked; anything else is disposed.
func (rt *Runtime) retireChild(c *componentInstance) {
	if r := c.retain; r != nil && !c.disposed && r.hub.keep(r.key) {
		rt.park(c)
		return
	}
	rt.disposeInstance(c)
}

// park takes the instance inst out of rt's maps, and out of the focus, the pointer and
// the animation lists, while its nodes and states stay as they are. Its nodes are still
// children of the stale nodes of rt, which the next build of rt replaces; the retire
// skips them, since the instance is no longer rt's (see retireNode).
func (rt *Runtime) park(inst *componentInstance) {
	rt.topModalOK = false
	ids := map[uint64]bool{}
	insts := map[*componentInstance]bool{}
	forNodes(inst.root, func(n *node) { ids[n.id] = true })
	forInstances(inst, func(c *componentInstance) {
		insts[c] = true
		if rt.instances[c.id] == c {
			delete(rt.instances, c.id)
		}
	})
	rt.animInsts = slices.DeleteFunc(rt.animInsts, func(c *componentInstance) bool { return insts[c] })
	for c := range insts {
		delete(rt.pausedAnims, c)
		delete(rt.graceAnims, c)
	}
	forNodes(inst.root, func(n *node) {
		if rt.byID[n.id] == n {
			delete(rt.byID, n.id)
		}
		if n.flags&flagOutside != 0 {
			rt.outside = slices.DeleteFunc(rt.outside, func(o *node) bool { return o.id == n.id })
		}
	})
	if ids[rt.focusID] {
		inst.refocus = rt.focusID
		rt.focusID = 0
	}
	if ids[rt.captureID] {
		rt.captureID = 0
	}
	if ids[rt.composing] {
		rt.composing = 0
	}
	rt.hoverIDs = slices.DeleteFunc(rt.hoverIDs, func(id uint64) bool { return ids[id] })
	rt.pressIDs = slices.DeleteFunc(rt.pressIDs, func(id uint64) bool { return ids[id] })
	rt.ghosts = slices.DeleteFunc(rt.ghosts, func(n *node) bool { return ids[n.id] })
	inst.parked = true
	inst.parent = nil // parentNode stays: adopt finds the instances that render into it by it
	rt.markAccess()
}

// adopt attaches the parked or attached instance inst to rt under the node parent: its nodes
// and instances point to rt, and its states move there from the runtime it was in. The
// instances that render straight into the parent's children are given the new parent. A
// focused node that was inside it keeps the focus when the runtime has it.
func (rt *Runtime) adopt(inst *componentInstance, parent *node) {
	rt.topModalOK = false
	from := inst.rt
	from.topModalOK = false
	moved := from != rt
	inst.reparent(parent)
	move := func(id uint64) {
		if !moved {
			return
		}
		if st := from.states[id]; st != nil {
			delete(from.states, id)
			st.gen = rt.gen
			rt.states[id] = st
		}
	}
	// The build generations are counted per runtime. The moved instances and nodes carry the
	// one before this build, so that no build of rt takes them for one it has made already.
	stale := rt.gen - 1
	forInstances(inst, func(c *componentInstance) {
		move(c.id)
		c.rt = rt
		c.parked = false
		c.gen = stale
		rt.instances[c.id] = c
		if moved {
			// The handlers of the elements these instances built hold the runtime that built
			// them, so the subtree renders once more in rt. Its state stays where it is.
			c.dirty.Store(true)
		}
	})
	forNodes(inst.root, func(n *node) {
		move(n.id)
		n.rt = rt
		n.gen = stale
		rt.byID[n.id] = n
		if n.flags&flagOutside != 0 {
			rt.listenOutside(n)
		}
		if moved {
			if d := n.div; d != nil && d.handle != nil && d.handle.rt == from {
				d.handle.rt = rt
			}
			if n.st != nil && n.st.focus != nil && n.st.focus.rt == from {
				n.st.focus.rt = rt
			}
		}
	})
	if inst.refocus != 0 {
		if rt.byID[inst.refocus] != nil {
			rt.focusID = inst.refocus
		}
		inst.refocus = 0
	}
	rt.markAccess()
	rt.needLayout, rt.needPaint = true, true
	rt.requestFrame()
}

// reparent sets the node that inst is attached to. The instances that realize into the same
// node, those whose output is inst's output, are attached to it too, so that a re-render of
// any of them splices into the tree that holds it.
func (inst *componentInstance) reparent(parent *node) {
	old := inst.parentNode
	inst.parentNode = parent
	if old == nil {
		return
	}
	for _, c := range inst.children {
		if c.parentNode == old {
			c.reparent(parent)
		}
	}
}

// keepOutput marks the instances that realize into inst's node as claimed in the build gen,
// as inst is when it is reused: the instances whose output is inst's output, which the
// node's boundary is one of. A retire of the old parent then skips the reused subtree (see
// retireNode).
func (inst *componentInstance) keepOutput(gen uint32) {
	inst.gen = gen
	for _, c := range inst.children {
		if c.root == inst.root && c.gen != gen {
			c.keepOutput(gen)
		}
	}
}

// forNodes calls fn for n and every node under it, parents first.
func forNodes(n *node, fn func(*node)) {
	if n == nil {
		return
	}
	fn(n)
	for _, c := range n.children {
		forNodes(c, fn)
	}
}

// forInstances calls fn for inst and every instance under it, parents first.
func forInstances(inst *componentInstance, fn func(*componentInstance)) {
	fn(inst)
	for _, c := range inst.children {
		forInstances(c, fn)
	}
}

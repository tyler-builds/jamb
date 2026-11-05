package ui

import "slices"

// The IDs of the host's layers' components. The IDs of the app's elements
// derive from rootID, so they do not meet them.
const (
	hostLayerID     = 0x1a7e5
	titleBarLayerID = 0x1a7e6
)

// hostLayer is a component of the host's, attached beside the app's root
// element (see SetHostLayer).
type hostLayer struct {
	id   uint64
	el   *ComponentElement
	inst *componentInstance
}

// SetHostLayer gives the host of the runtime, such as package app, a layer of
// its own above the app's UI. fn is a component that renders the layer: it runs
// again when the signals it reads change, as any component does. It renders
// overlays, such as an AlertDialog or a Modal, or nil for nothing. What it
// renders joins the window's tree after the app's root element, so a modal in it
// takes the focus, traps Tab and is found by screen readers and the control
// channel, as one the app opened is. The app's elements keep their IDs and their
// layout: the layer is attached beside them, not around them. It needs a root
// element that lays out children, a Div or a component that renders one, as
// nearly every app's root is; HostLayerShown reports whether it has one.
//
// The app package uses it for the prompt about unsaved changes when a window
// closes (see app.Window.Unsaved). Call it on the UI thread. A nil fn removes
// the layer.
func (rt *Runtime) SetHostLayer(fn func(ctx *Context) Element) {
	rt.setLayer(hostLayerID, fn)
}

// setLayer sets, replaces or, with a nil fn, removes the host layer with the
// ID. Layers are attached in the order of their IDs, so the default title bar
// comes before the host's layer, whose dialogs paint over it.
func (rt *Runtime) setLayer(id uint64, fn func(ctx *Context) Element) {
	rt.detachLayer()
	i := slices.IndexFunc(rt.layers, func(l *hostLayer) bool { return l.id == id })
	switch {
	case fn == nil && i >= 0:
		if inst := rt.layers[i].inst; inst != nil {
			rt.retireChild(inst)
		}
		rt.layers = slices.Delete(rt.layers, i, i+1)
	case fn != nil && i >= 0:
		rt.layers[i].el = Component(fn)
	case fn != nil:
		rt.layers = append(rt.layers, &hostLayer{id: id, el: Component(fn)})
		slices.SortFunc(rt.layers, func(a, b *hostLayer) int { return int(a.id) - int(b.id) })
	}
	rt.attachLayer()
	rt.needLayout = true
	rt.requestFrame()
}

// attachLayer builds the host layers, those not yet built, and adds their
// nodes as the last children of the tree's root node.
func (rt *Runtime) attachLayer() {
	root := rt.tree
	if root == nil || root.kind != kindDiv {
		return // a Text or another leaf lays out no children
	}
	for _, l := range rt.layers {
		l.inst = rt.instance(l.id, l.el, nil, root, rt.rootTextStyle())
		n := l.inst.root
		n.parent = root
		root.children = append(root.children, n)
		root.lay.Children = append(root.lay.Children, &n.lay)
	}
	if len(rt.layers) > 0 {
		rt.reorder(root)
		rt.invalidateLayout(root)
		rt.needLayout = true
	}
}

// detachLayer takes the host layers' nodes out of their parent, so that a
// rebuild of the tree's root does not retire them with the rest of the old tree.
func (rt *Runtime) detachLayer() {
	for _, l := range rt.layers {
		inst := l.inst
		if inst == nil || inst.root == nil || inst.parentNode == nil {
			continue
		}
		p := inst.parentNode
		if i := slices.Index(p.children, inst.root); i >= 0 {
			p.children = slices.Delete(p.children, i, i+1)
			p.lay.Children = slices.Delete(p.lay.Children, i, i+1)
			rt.reorder(p)
			rt.invalidateLayout(p)
		}
	}
}

// HostLayerShown reports whether the host layer is attached to the window's
// tree, so that what it renders shows. It is false before SetHostLayer, before
// the first frame, and while the app's root element is not a Div.
func (rt *Runtime) HostLayerShown() bool {
	i := slices.IndexFunc(rt.layers, func(l *hostLayer) bool { return l.id == hostLayerID })
	if i < 0 {
		return false
	}
	inst := rt.layers[i].inst
	return inst != nil && inst.root != nil && inst.parentNode != nil && inst.parentNode == rt.tree &&
		slices.Contains(rt.tree.children, inst.root)
}

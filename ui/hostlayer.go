package ui

import "slices"

// hostLayerID is the ID of the host layer's component. The IDs of the app's
// elements derive from rootID, so they do not meet it.
const hostLayerID = 0x1a7e5

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
	rt.detachLayer()
	if fn == nil {
		if inst := rt.layerInst; inst != nil {
			rt.layerInst = nil
			rt.retireChild(inst)
		}
		rt.layerEl = nil
		rt.needLayout = true
		rt.requestFrame()
		return
	}
	rt.layerEl = Component(fn)
	rt.attachLayer()
	rt.requestFrame()
}

// attachLayer builds the host layer, if it is set and not yet built, and adds
// its node as the last child of the tree's root node.
func (rt *Runtime) attachLayer() {
	root := rt.tree
	if rt.layerEl == nil || root == nil || root.kind != kindDiv {
		return // a Text or another leaf lays out no children
	}
	inst := rt.instance(hostLayerID, rt.layerEl, nil, root, rt.rootTextStyle())
	rt.layerInst = inst
	n := inst.root
	n.parent = root
	root.children = append(root.children, n)
	root.lay.Children = append(root.lay.Children, &n.lay)
	rt.reorder(root)
	rt.invalidateLayout(root)
	rt.needLayout = true
}

// detachLayer takes the host layer's node out of its parent, so that a rebuild
// of the tree's root does not retire it with the rest of the old tree.
func (rt *Runtime) detachLayer() {
	inst := rt.layerInst
	if inst == nil || inst.root == nil || inst.parentNode == nil {
		return
	}
	p := inst.parentNode
	if i := slices.Index(p.children, inst.root); i >= 0 {
		p.children = slices.Delete(p.children, i, i+1)
		p.lay.Children = slices.Delete(p.lay.Children, i, i+1)
		rt.reorder(p)
		rt.invalidateLayout(p)
	}
}

// HostLayerShown reports whether the host layer is attached to the window's
// tree, so that what it renders shows. It is false before SetHostLayer, before
// the first frame, and while the app's root element is not a Div.
func (rt *Runtime) HostLayerShown() bool {
	inst := rt.layerInst
	return inst != nil && inst.root != nil && inst.parentNode != nil && inst.parentNode == rt.tree &&
		slices.Contains(rt.tree.children, inst.root)
}

package ui

import "slices"

// undisplayed reports whether n is hidden by Display, itself or through an
// ancestor. It is false for nil.
func (rt *Runtime) undisplayed(n *node) bool {
	for ; n != nil; n = n.parent {
		if n.flags&flagNoDisplay != 0 {
			return true
		}
	}
	return false
}

// moveFocusOutOfHidden moves focus off an element that a build hid (see
// Display) to its nearest focusable ancestor, or clears focus when there is
// none. The ancestors above the outermost hidden element are displayed, so
// focus can go to them.
func (rt *Runtime) moveFocusOutOfHidden() {
	n := rt.byID[rt.focusID]
	if n == nil || !rt.undisplayed(n) {
		return
	}
	top := n
	for a := n; a != nil; a = a.parent {
		if a.flags&flagNoDisplay != 0 {
			top = a
		}
	}
	var to uint64
	for a := top.parent; a != nil && to == 0; a = a.parent {
		if a.div != nil && a.div.focusable {
			to = a.id
		}
	}
	rt.setFocus(to)
}

// holdAnim records an instance whose repeating or running animation is hidden
// by Display. It asks for no frame; revealAnims renders it again once shown.
func (rt *Runtime) holdAnim(inst *componentInstance) {
	if !slices.Contains(rt.hiddenAnims, inst) {
		rt.hiddenAnims = append(rt.hiddenAnims, inst)
	}
}

// revealAnims renders again the instances whose animations were held while
// hidden and are shown now, so that each shows its progress at this moment and
// runs on from there. An instance that has gone is dropped.
func (rt *Runtime) revealAnims() {
	if len(rt.hiddenAnims) == 0 {
		return
	}
	shown := slices.Clone(rt.hiddenAnims)
	rt.hiddenAnims = slices.DeleteFunc(rt.hiddenAnims, func(inst *componentInstance) bool {
		return inst.disposed || !rt.undisplayed(inst.parentNode)
	})
	for _, inst := range shown {
		if !inst.disposed && !rt.undisplayed(inst.parentNode) {
			rt.rerender(inst)
		}
	}
}

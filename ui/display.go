package ui

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

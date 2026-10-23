package ui

import "slices"

// A window can belong to another, its owner: the window of a floating dock group
// belongs to the window of its dock. The group is still part of that window, so the
// window that holds it shows the owner's theme, text and zoom, and changes them with
// the owner: SetTheme, SetThemes, SetStrings and SetZoom on either one apply to both,
// and the owner's OnZoom hears of a zoom made in the other. The owner's key bindings
// are its bindings, and an action that nothing in the window handles goes on to the
// owner, from the component it belongs to up to the owner's root, as if the window's
// content were in that component's place.

// follow makes rt belong to owner, with its actions going on from the component at.
// It takes the owner's look at once. A window belongs to one owner: following another
// leaves the first, and an owner that belongs to a window passes rt on to that one.
func (rt *Runtime) follow(owner *Runtime, at *componentInstance) {
	for owner.owner != nil {
		owner = owner.owner
	}
	if owner == rt {
		return
	}
	rt.ownerAt = at
	if rt.owner == owner {
		return
	}
	rt.unfollow()
	rt.owner = owner
	owner.followers = append(owner.followers, rt)
	rt.copyLook(owner)
}

// unfollow ends rt's ties: to its owner, and those of the windows that belong to it,
// which keep the look they have. It runs when the window closes.
func (rt *Runtime) unfollow() {
	if o := rt.owner; o != nil {
		o.followers = slices.DeleteFunc(o.followers, func(f *Runtime) bool { return f == rt })
		rt.owner, rt.ownerAt = nil, nil
	}
	for _, f := range rt.followers {
		f.owner, f.ownerAt = nil, nil
	}
	rt.followers = nil
}

// lookChanged gives the windows that belong to rt its new look.
func (rt *Runtime) lookChanged() {
	for _, f := range rt.followers {
		f.copyLook(rt)
	}
}

// copyLook makes rt's theme, text and zoom those of o. The themes are chosen by rt's
// own system settings, which are the same for every window of the app.
func (rt *Runtime) copyLook(o *Runtime) {
	rt.themes = o.themes
	rt.appTheme = o.appTheme
	rt.applyTheme()
	rt.strings = o.strings
	if rt.zoom != o.zoom {
		rt.zoom = o.zoom
		rt.applyZoom()
	}
	rt.hostInvalidateAll()
	rt.requestFrame()
}

// bindings returns the key bindings that apply in rt: its owner's, when it has one.
func (rt *Runtime) bindings() []Binding {
	if rt.owner != nil {
		return rt.owner.keymap
	}
	return rt.keymap
}

// withOwner returns the path an action takes: path, and when it reaches rt's root
// and rt has an owner, the component of the owner that rt belongs to and its
// ancestors. A path that a modal cuts short stays in the modal.
func (rt *Runtime) withOwner(path []*node) []*node {
	o := rt.owner
	if o == nil || len(path) == 0 || path[len(path)-1] != rt.tree {
		return path
	}
	n := o.tree
	if at := rt.ownerAt; at != nil && !at.disposed && at.rt == o && at.root != nil {
		n = at.root
	}
	out := slices.Clip(path)
	for ; n != nil; n = n.parent {
		out = append(out, n)
	}
	return out
}

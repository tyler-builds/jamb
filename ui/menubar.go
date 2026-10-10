package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// MenuBar is a row of top-level menus, such as File and Edit, for a window's
// top edge (including a custom title bar). A click on a label opens its menu,
// and while one is open, the pointer over another label opens that one.
// Pressing and releasing Alt by itself focuses the bar. There, Left and Right
// move between labels, Down, Enter or Space opens the highlighted menu, and
// Escape or Tab leaves the bar. Within an open menu, Left and Right move to the
// neighboring menu. The labels are not in the Tab order. Alt+letter
// mnemonics are not implemented.
func MenuBar(menus ...TopMenu) Element {
	return Component(func(ctx *Context) Element {
		rt, t := ctx.rt, ctx.Theme()
		open := Local(ctx, "open", -1) // index of the open menu, or -1
		hl := Local(ctx, "hl", 0)      // the label that has keyboard focus
		items := Local(ctx, "items", []MenuItem(nil))
		path := Local(ctx, "path", []int{-1})
		// One focus handle per label, made on each render: the handlers of this
		// render are attached to these labels.
		handles := make([]*FocusHandle, len(menus))
		for i := range handles {
			handles[i] = NewFocusHandle()
		}
		show := func(i int, keyboard bool) {
			if i < 0 || i >= len(menus) || open.Peek() == i {
				return
			}
			built := menus[i].Items()
			if len(built) == 0 {
				return
			}
			rt.menuOpened()
			rt.menuCancel()
			items.Set(built)
			path.Set(highlightPath(built, keyboard))
			open.Set(i)
			hl.Set(i)
		}
		hide := func(restore bool) {
			if open.Peek() < 0 {
				return
			}
			open.Set(-1)
			rt.menuClosed(restore)
		}
		// leave runs for a press outside the bar, which also closes its menu.
		leave := func() {
			if open.Peek() >= 0 {
				open.Set(-1)
			}
			rt.menuClosed(false)
		}

		bar := Div().Row().Align(Center).Gap(2).Role(access.RoleMenuBar)
		bar.menuBar = true
		bar.OnClickOutside(leave)
		bar.OnKeyDown(func(e *KeyEvent) {
			if open.Peek() >= 0 || e.Mods&(input.ModCtrl|input.ModAlt|input.ModSuper) != 0 {
				return
			}
			n := len(menus)
			switch e.Key {
			case input.KeyLeft, input.KeyRight:
				dir := 1
				if e.Key == input.KeyLeft {
					dir = -1
				}
				if n > 0 {
					handles[(hl.Peek()+dir+n)%n].FocusNow()
				}
			case input.KeyDown, input.KeyEnter, input.KeySpace:
				if !e.Repeat {
					show(hl.Peek(), true)
				}
			case input.KeyEscape:
				rt.menuClosed(true)
			case input.KeyTab:
				rt.menuClosed(true) // not stopped: focus moves on from where it was
				return
			default:
				return
			}
			e.StopPropagation()
		})

		cur := open.Get()
		var panel *menuView
		if cur >= 0 {
			panel = &menuView{ctx: ctx, t: t, items: items.Get(), path: path, close: hide,
				label: menus[cur].Label,
				move: func(dir int) {
					if at := open.Peek(); at >= 0 {
						show((at+dir+len(menus))%len(menus), true)
					}
				}}
		}
		for i, tm := range menus {
			// The open label keeps its highlight while the pointer is on it.
			hover := t.SurfaceHover
			if i == cur {
				hover = t.SurfaceActive
			}
			lbl := Div().Keyf("jamb.menubar.%d", i).Row().Align(Center).MinHeight(Px(26)).PaddingX(10).Radius(4).Shrink(0).
				FontSize(13).TextColor(t.Text).
				Hover(Style{Background: hover}).
				FocusVisible(Style{Background: t.SurfaceHover}).
				FocusHandle(handles[i]).TabStop(false).
				OnFocus(func() { hl.Set(i) }).
				OnClick(func() {
					if open.Peek() == i {
						hide(true)
					} else {
						show(i, false)
					}
				}).
				OnMouseEnter(func() {
					if open.Peek() >= 0 {
						show(i, false)
					}
				}).
				Child(Text(tm.Label).NoWrap()).Role(access.RoleMenuItem).Expanded(i == cur)
			if i == cur {
				lbl.Background(t.SurfaceActive)
				lbl.Child(panel.top(2))
			}
			bar.Child(lbl)
		}
		return bar
	})
}

// HasMenuBar reports whether the window's tree has a MenuBar. The platform
// asks through the app when Alt is pressed alone, so the system menu stays
// shut over the app's own bar.
func (rt *Runtime) HasMenuBar() bool { return rt.menuBarNode() != nil }

// menuBarNode returns the bar made by MenuBar in the tree, or nil.
func (rt *Runtime) menuBarNode() *node {
	var find func(n *node) *node
	find = func(n *node) *node {
		if n.div != nil && n.div.menuBar {
			return n
		}
		for _, c := range n.children {
			if f := find(c); f != nil {
				return f
			}
		}
		return nil
	}
	if rt.tree == nil {
		return nil
	}
	return find(rt.tree)
}

// activateMenuBar focuses the first label of the menu bar, as a press and
// release of Alt does, and remembers where focus was so that it can return.
// It does nothing when the window has no menu bar.
func (rt *Runtime) activateMenuBar() {
	bar := rt.menuBarNode()
	if bar == nil {
		return
	}
	for _, c := range bar.children {
		if c.div != nil && c.div.focusable {
			if rt.menuReturn == nil {
				rt.menuReturn = rt.FocusedHandle()
			}
			rt.setFocus(c.id)
			return
		}
	}
}

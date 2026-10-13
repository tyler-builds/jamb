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
func MenuBar(menus ...TopMenu) *MenuBarElement {
	return &MenuBarElement{menus: menus}
}

// MenuBarStyle is the look of each part of a menu bar. A part's style merges
// over the built-in defaults, then over the theme's MenuBar, then over the
// Styles of the instance; see docs/styling.md. BarItem is each label on the bar.
// The states are parts of their own: BarItemHover while the pointer is over a
// label, BarItemFocusVisible while the label has keyboard focus, and BarItemOpen
// for the label of the open menu. The menus that a bar opens are styled by
// Theme.Menu, as the menus of any other kind are (see MenuStyle).
type MenuBarStyle struct {
	BarItem             Style // each label on the bar
	BarItemHover        Style // a label the pointer is over
	BarItemFocusVisible Style // a label that has keyboard focus
	BarItemOpen         Style // the label of the open menu
}

// merge returns s with the set fields of o applied on top, part by part.
func (s MenuBarStyle) merge(o MenuBarStyle) MenuBarStyle {
	return MenuBarStyle{
		BarItem:             s.BarItem.Merge(o.BarItem),
		BarItemHover:        s.BarItemHover.Merge(o.BarItemHover),
		BarItemFocusVisible: s.BarItemFocusVisible.Merge(o.BarItemFocusVisible),
		BarItemOpen:         s.BarItemOpen.Merge(o.BarItemOpen),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s MenuBarStyle) plain() MenuBarStyle {
	return MenuBarStyle{
		BarItem:             plainStyle(s.BarItem),
		BarItemHover:        plainStyle(s.BarItemHover),
		BarItemFocusVisible: plainStyle(s.BarItemFocusVisible),
		BarItemOpen:         plainStyle(s.BarItemOpen),
	}
}

// menuBarDefaults returns the built-in look of a menu bar, from the colors of t.
func menuBarDefaults(t *Theme) MenuBarStyle {
	return MenuBarStyle{
		BarItem: Style{Direction: Row, Align: Center, MinHeight: Px(26), Padding: XY(10, 0), Radius: 4,
			NoShrink: true, FontSize: 13, TextColor: t.Text},
		BarItemHover:        Style{Background: t.SurfaceHover},
		BarItemFocusVisible: Style{Background: t.SurfaceHover},
		BarItemOpen:         Style{Background: t.SurfaceActive},
	}
}

// resolveMenuBar returns the look of a menu bar: the defaults, merged with the
// theme's MenuBar and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveMenuBar(t *Theme, inst MenuBarStyle) MenuBarStyle {
	st := menuBarDefaults(t)
	for _, o := range []MenuBarStyle{t.MenuBar, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// MenuBarElement is a row of top-level menus. See MenuBar.
type MenuBarElement struct {
	menus  []TopMenu
	styles MenuBarStyle
}

func (*MenuBarElement) isElement() {}

// Styles sets the look of the labels on the bar, for this instance only. Set
// fields win over the theme's MenuBar and the defaults; see MenuBarStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (mb *MenuBarElement) Styles(st MenuBarStyle) *MenuBarElement {
	mb.styles = mb.styles.merge(st)
	return mb
}

func (mb *MenuBarElement) expand(ctx *Context) Element {
	menus, styles := mb.menus, mb.styles
	return Component(func(ctx *Context) Element {
		rt, t := ctx.rt, ctx.Theme()
		st := resolveMenuBar(t, styles)
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
				label: menus[cur].Label, st: resolveMenu(t, MenuStyle{}),
				move: func(dir int) {
					if at := open.Peek(); at >= 0 {
						show((at+dir+len(menus))%len(menus), true)
					}
				}}
		}
		for i, tm := range menus {
			// The open label keeps its highlight while the pointer is on it.
			hover := st.BarItemHover
			if i == cur {
				hover = st.BarItemOpen
			}
			lbl := Div().asPart("MenuBar.BarItem").Keyf("jamb.menubar.%d", i).Style(st.BarItem).
				Hover(hover).
				FocusVisible(st.BarItemFocusVisible).
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
				Child(Text(tm.Label).NoWrap()).Role(access.RoleMenuItem).AccessExpanded(i == cur)
			if i == cur {
				lbl.Style(st.BarItemOpen)
				lbl.Child(panel.top(2))
			}
			bar.Child(lbl)
		}
		return bar
	})
}

// hostHasMenuBar reports whether the window's tree has a MenuBar. The platform
// asks through the app when Alt is pressed alone, so the system menu stays
// shut over the app's own bar.
func (rt *Runtime) hostHasMenuBar() bool { return rt.menuBarNode() != nil }

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

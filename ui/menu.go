package ui

import (
	"slices"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// Menus are drawn by jamb, not by the operating system, so they look the same
// on every platform and work with a custom title bar. A menu is a list of
// MenuItems. ContextMenu, DropdownMenu and MenuBar open one:
//
//	ui.Div().ContextMenu(func() []ui.MenuItem { return rowMenu() })
//	ui.DropdownMenu(ui.Button("Actions"), func() []ui.MenuItem { return actionMenu() })
//	ui.MenuBar(ui.TopMenu{Label: "File", Items: fileMenu})

// MenuKind says what a MenuItem is.
type MenuKind uint8

const (
	MenuKindItem      MenuKind = iota // a command, check or radio item (the zero value)
	MenuKindSeparator                 // a rule between groups of items
	MenuKindSection                   // a group label that runs nothing
)

// MenuCheck makes an item checkable. MenuCheckBox shows a check mark while
// Checked is set; MenuCheckRadio shows a dot. Radio items in one menu form a
// group, and the selected one is the one that is Checked.
type MenuCheck uint8

const (
	MenuCheckNone MenuCheck = iota
	MenuCheckBox
	MenuCheckRadio
)

// MenuItem is one entry of a menu. Set Action, which shows the action's
// shortcut and runs through the normal action dispatch, or OnClick. If both
// are set, Action runs. A non-empty Submenu opens a menu to the right instead
// of running.
type MenuItem struct {
	Kind     MenuKind
	Label    string // defaults to the Action's label
	Icon     *icons.Icon
	Action   *Action
	OnClick  func()
	Disabled bool       // shown, but cannot be chosen
	Check    MenuCheck  // see MenuCheck
	Checked  bool       // the state of a checkable item
	Submenu  []MenuItem // non-empty: opens a submenu
}

// MenuSeparator is a rule between groups of items.
func MenuSeparator() MenuItem { return MenuItem{Kind: MenuKindSeparator} }

// MenuSection is a group label, such as "Status". It runs nothing.
func MenuSection(label string) MenuItem { return MenuItem{Kind: MenuKindSection, Label: label} }

// accessRole returns the item's accessibility role, as in WAI-ARIA's menuitem,
// menuitemcheckbox, menuitemradio and separator. Checked is the state of a
// check or radio item.
func (m MenuItem) accessRole() access.Role {
	switch {
	case m.Kind == MenuKindSeparator:
		return access.RoleSeparator
	case m.Check == MenuCheckRadio:
		return access.RoleMenuItemRadio
	case m.Check == MenuCheckBox:
		return access.RoleMenuItemCheck
	}
	return access.RoleMenuItem
}

// text is the label an item shows: its Label, or its Action's label.
func (m MenuItem) text() string {
	if m.Label == "" && m.Action != nil {
		return m.Action.Label()
	}
	return m.Label
}

// selectable reports whether a press or a key can choose the item.
func (m MenuItem) selectable() bool { return m.Kind == MenuKindItem && !m.Disabled }

// MenuStyle is the look of each part of a menu: a dropdown, a context menu and
// the menus of a MenuBar. A part's style merges over the built-in defaults, then
// over the theme's Menu, then over the Styles of the instance (a DropdownMenu
// has one; a context menu takes the theme's only); see docs/styling.md. Panel is
// the box of a menu. Item is each row; ItemHighlighted is the row that has the
// keyboard position, and ItemDisabled a row that cannot be chosen. Separator is
// the rule between groups, SectionLabel a group's label, Shortcut the text of an
// action's key binding, Check the slot before a label (its check mark, dot or
// icon is drawn in its TextColor), and SubmenuChevron the arrow of a submenu.
type MenuStyle struct {
	Panel           Style // the box of a menu
	Item            Style // each row
	ItemHighlighted Style // the row that has the keyboard position
	ItemDisabled    Style // a row that cannot be chosen
	Separator       Style // the rule between groups of rows
	SectionLabel    Style // a group's label
	Shortcut        Style // the key binding of an action
	Check           Style // the slot before a label: its check mark, dot or icon
	SubmenuChevron  Style // the arrow of a row that opens a submenu
}

// merge returns s with the set fields of o applied on top, part by part.
func (s MenuStyle) merge(o MenuStyle) MenuStyle {
	return MenuStyle{
		Panel:           s.Panel.Merge(o.Panel),
		Item:            s.Item.Merge(o.Item),
		ItemHighlighted: s.ItemHighlighted.Merge(o.ItemHighlighted),
		ItemDisabled:    s.ItemDisabled.Merge(o.ItemDisabled),
		Separator:       s.Separator.Merge(o.Separator),
		SectionLabel:    s.SectionLabel.Merge(o.SectionLabel),
		Shortcut:        s.Shortcut.Merge(o.Shortcut),
		Check:           s.Check.Merge(o.Check),
		SubmenuChevron:  s.SubmenuChevron.Merge(o.SubmenuChevron),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s MenuStyle) plain() MenuStyle {
	return MenuStyle{
		Panel:           plainStyle(s.Panel),
		Item:            plainStyle(s.Item),
		ItemHighlighted: plainStyle(s.ItemHighlighted),
		ItemDisabled:    plainStyle(s.ItemDisabled),
		Separator:       plainStyle(s.Separator),
		SectionLabel:    plainStyle(s.SectionLabel),
		Shortcut:        plainStyle(s.Shortcut),
		Check:           plainStyle(s.Check),
		SubmenuChevron:  plainStyle(s.SubmenuChevron),
	}
}

// menuDefaults returns the built-in look of a menu, from the colors of t. Under
// high contrast the highlighted row is the Highlight color with HighlightText on
// it, since the soft tint of the normal theme is the window color there.
func menuDefaults(t *Theme) MenuStyle {
	d := MenuStyle{
		Panel: Style{Padding: All(4), MinWidth: Px(200), Background: t.Surface, BorderWidth: 1,
			BorderColor: t.Border, Radius: t.Radius, Shadows: []Shadow{ShadowMd}},
		Item: Style{Direction: Row, Align: Center, Gap: 8, MinHeight: Px(28), Padding: XY(8, 0),
			Radius: 4, NoShrink: true, FontSize: 13, TextColor: t.Text},
		ItemHighlighted: Style{Background: t.AccentSoft},
		ItemDisabled:    Style{TextColor: t.Disabled},
		Separator:       Style{Height: Px(1), NoShrink: true, Margin: Edges{Top: 4, Bottom: 4}, Background: t.Border},
		SectionLabel:    Style{Padding: XY(8, 4), FontSize: 12, FontWeight: FontMedium, TextColor: t.TextSubtle},
		Shortcut:        Style{TextColor: t.TextSubtle},
		Check: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(16), Height: Px(16),
			NoShrink: true, TextColor: t.Text},
		SubmenuChevron: Style{TextColor: t.TextMuted},
	}
	if t.HighContrast {
		d.ItemHighlighted = Style{Background: t.Accent, TextColor: t.AccentText}
	}
	return d
}

// resolveMenu returns the look of a menu: the defaults, merged with the theme's
// Menu and then the instance's Styles. Under high contrast the two overrides are
// plain (see plainStyle).
func resolveMenu(t *Theme, inst MenuStyle) MenuStyle {
	st := menuDefaults(t)
	for _, o := range []MenuStyle{t.Menu, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// TopMenu is one menu of a MenuBar: the label on the bar, and the function that
// builds the menu's items when the menu opens.
type TopMenu struct {
	Label string
	Items func() []MenuItem
}

// submenuDelay is how long the pointer rests on an item before its submenu
// opens, or a submenu it has left closes. The delay lets the pointer cross
// other items on its way into a submenu.
const submenuDelay = 150 * time.Millisecond

// menuStep returns the index of the next selectable item after from, moving
// dir (+1 or -1) and wrapping around. From -1 (nothing highlighted) it starts
// at the first item going forward, or the last going back. It returns -1 when
// no item can be chosen.
func menuStep(items []MenuItem, from, dir int) int {
	n := len(items)
	if n == 0 {
		return -1
	}
	i := from
	if i < 0 || i >= n {
		i = n - 1 // so the first step goes to 0
		if dir < 0 {
			i = 0 // so the first step goes to n-1
		}
	}
	for range n {
		i = (i + dir + n) % n
		if items[i].selectable() {
			return i
		}
	}
	return -1
}

// menuTypeahead returns the next selectable item, after from and wrapping
// around, whose label starts with r (case-insensitive), or -1.
func menuTypeahead(items []MenuItem, from int, r rune) int {
	n := len(items)
	for k := 1; k <= n; k++ {
		i := (from + k) % n
		first, _ := utf8.DecodeRuneInString(items[i].text())
		if items[i].selectable() && unicode.ToLower(first) == unicode.ToLower(r) {
			return i
		}
	}
	return -1
}

// menuLevel returns the items shown at level k of an open menu, following the
// highlighted item of each level above it. It returns nil when the path does
// not reach level k.
func menuLevel(items []MenuItem, path []int, k int) []MenuItem {
	for j := 0; j < k; j++ {
		if j >= len(path) || path[j] < 0 || path[j] >= len(items) || len(items[path[j]].Submenu) == 0 {
			return nil
		}
		items = items[path[j]].Submenu
	}
	return items
}

// highlightPath is the path of a menu that has just opened: nothing highlighted,
// or the first item when the keyboard opened it.
func highlightPath(items []MenuItem, first bool) []int {
	if first {
		return []int{menuStep(items, -1, 1)}
	}
	return []int{-1}
}

// menuView draws one open menu chain: the top panel and the submenus open from
// it. The state is a path with one entry per open level: path[k] is the
// highlighted item of level k, and a level k+1 panel is open when there is a
// path entry after path[k]. The path lives in a signal of the component that
// owns the menu. Only the top panel takes keyboard focus, and it moves the path.
type menuView struct {
	ctx   *Context
	t     *Theme
	items []MenuItem
	path  *Signal[[]int]
	// close ends the whole chain. It restores focus when restore is set.
	close func(restore bool)
	// move, set by a menu bar, opens the neighboring top-level menu dir steps
	// away (+1 or -1) when Left or Right has nothing else to do here.
	move func(dir int)
	// label is the accessible name of the menu: the trigger's name, or the
	// menu's title in a menu bar. It is left out when empty.
	label string
	// st is the look of the menu's parts (see MenuStyle).
	st MenuStyle
}

// set replaces the path, dropping a submenu change that is still waiting.
func (m *menuView) set(p []int) {
	m.ctx.rt.menuCancel()
	if !slices.Equal(p, m.path.Peek()) {
		m.path.Set(p)
	}
}

// highlight highlights item i at level n, the deepest open level.
func (m *menuView) highlight(n, i int) {
	if i < 0 {
		return
	}
	p := slices.Clone(m.path.Peek())
	p[n] = i
	m.set(p)
}

// openSub opens the submenu of item i at level k, highlighting its first item
// when first is set.
func (m *menuView) openSub(k, i int, first bool) {
	items := menuLevel(m.items, m.path.Peek(), k)
	p := slices.Clone(m.path.Peek()[:k+1])
	p[k] = i
	sub := -1
	if first {
		sub = menuStep(items[i].Submenu, -1, 1)
	}
	m.set(append(p, sub))
}

// hover highlights item i at level k when the pointer enters it. Opening or
// closing a submenu waits submenuDelay, so that the pointer can move
// diagonally into a submenu past the items in between.
func (m *menuView) hover(k, i int) {
	rt := m.ctx.rt
	rt.menuCancel()
	path := m.path.Peek()
	items := menuLevel(m.items, path, k)
	if k >= len(path) || i < 0 || i >= len(items) {
		return // the row is stale: the path changed before the next frame
	}
	p := slices.Clone(path[:k+1])
	p[k] = -1
	if it := items[i]; it.selectable() {
		p[k] = i
		if len(it.Submenu) > 0 {
			p = append(p, -1)
		}
	}
	if slices.Equal(p, path) {
		return
	}
	if len(path) > k+1 || len(p) > k+1 {
		rt.menuLater(submenuDelay, func() { m.path.Set(p) })
		return
	}
	m.set(p)
}

// activate chooses item i at level k. A submenu opens. Anything else closes the
// whole menu, returning focus, and then runs.
func (m *menuView) activate(k, i int) {
	items := menuLevel(m.items, m.path.Peek(), k)
	if k >= len(m.path.Peek()) || i < 0 || i >= len(items) || !items[i].selectable() {
		return
	}
	it := items[i]
	if len(it.Submenu) > 0 {
		m.openSub(k, i, true)
		return
	}
	m.close(true)
	switch {
	case it.Action != nil:
		m.ctx.Dispatch(it.Action)
	case it.OnClick != nil:
		it.OnClick()
	}
}

// keyDown moves the highlight for a key press in the menu. Up and Down wrap and
// skip items that cannot be chosen. Right opens a submenu and Left closes one,
// or moves to the neighboring menu of a menu bar. Escape closes one level.
// Enter and Space choose. Tab closes the menu and lets focus move on.
func (m *menuView) keyDown(e *KeyEvent) {
	if e.Mods&(input.ModCtrl|input.ModAlt|input.ModSuper) != 0 {
		return
	}
	if e.Key == input.KeyMenu || (e.Key == input.KeyF10 && e.Mods == input.ModShift) {
		e.StopPropagation() // a menu is open: the context menu key does not open another
		return
	}
	path := m.path.Peek()
	n := len(path) - 1
	items := menuLevel(m.items, path, n)
	cur := -1
	if items != nil {
		cur = path[n]
	}
	switch e.Key {
	case input.KeyEscape:
		e.StopPropagation()
		if e.Repeat {
			return
		}
		if n > 0 {
			m.set(path[:n])
		} else {
			m.close(true)
		}
	case input.KeyDown:
		m.highlight(n, menuStep(items, cur, 1))
	case input.KeyUp:
		m.highlight(n, menuStep(items, cur, -1))
	case input.KeyHome:
		m.highlight(n, menuStep(items, -1, 1))
	case input.KeyEnd:
		m.highlight(n, menuStep(items, len(items), -1))
	case input.KeyRight:
		if cur >= 0 && items[cur].selectable() && len(items[cur].Submenu) > 0 {
			m.openSub(n, cur, true)
		} else if n == 0 && m.move != nil {
			m.move(1)
		}
	case input.KeyLeft:
		if n > 0 {
			m.set(path[:n])
		} else if m.move != nil {
			m.move(-1)
		}
	case input.KeyEnter, input.KeySpace:
		if !e.Repeat && cur >= 0 {
			m.activate(n, cur)
		}
	case input.KeyTab:
		m.close(true)
		return // not stopped: the focus moves on from where it was
	default:
		return
	}
	e.StopPropagation()
}

// typeahead highlights the next item whose label starts with the letter or
// digit typed.
func (m *menuView) typeahead(s string) {
	r, _ := utf8.DecodeRuneInString(s)
	if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
		return
	}
	path := m.path.Peek()
	n := len(path) - 1
	m.highlight(n, menuTypeahead(menuLevel(m.items, path, n), path[n], r))
}

// top is the top panel: a floating panel below its anchor that takes keyboard
// focus, and closes when the pointer presses outside it.
func (m *menuView) top(gap float32) *DivElement {
	d := m.box(0, m.items, m.path.Get()).Key("jamb.menu").Overlay(BelowStart).OverlayGap(gap).
		AutoFocus().TabStop(false).OnKeyDown(m.keyDown).OnTextInput(m.typeahead).
		OnClickOutside(func() { m.close(false) })
	if m.label != "" {
		d.Label(m.label)
	}
	return d
}

// box is the surface of one panel at level k, with its rows stacked.
func (m *menuView) box(k int, items []MenuItem, path []int) *DivElement {
	// Rows that have an icon or a check mark leave room for it, so the labels
	// line up.
	gutter := false
	for _, it := range items {
		gutter = gutter || it.Icon != nil || it.Check != MenuCheckNone
	}
	d := Div().asPart("Menu.Panel").Style(m.st.Panel).
		Enter(Fade().Slide(0, -6).Grow(0.98), Animation{Duration: 140 * time.Millisecond}).
		Exit(Fade().Grow(0.98), Animation{Duration: 90 * time.Millisecond}).
		Role(access.RoleMenu)
	d.access().level = k + 1
	for i, it := range items {
		d.Child(m.row(k, i, it, path, gutter))
	}
	return d
}

// row is one entry of the panel at level k. The submenu of a highlighted row
// with one open hangs from it, to the right.
func (m *menuView) row(k, i int, it MenuItem, path []int, gutter bool) Element {
	switch it.Kind {
	case MenuKindSeparator:
		return Div().asPart("Menu.Separator").Style(m.st.Separator).Role(access.RoleSeparator)
	case MenuKindSection:
		return Div().asPart("Menu.SectionLabel").Style(m.st.SectionLabel).
			Child(Text(it.text()).NoWrap())
	}
	on := it.selectable()
	hl := k < len(path) && path[k] == i
	r := Div().asPart("Menu.Item").Keyf("jamb.menu.%d.%d", k, i).Style(m.st.Item)
	// The name is the label alone: the shortcut is shown, but assistive technology
	// reads the label, as a menu does.
	a := r.access()
	a.role, a.disabled, a.checked, a.checkedSet = it.accessRole(), it.Disabled, it.Checked, true
	a.name = it.text()
	a.highlighted = hl && on
	a.level = k + 1
	if len(it.Submenu) > 0 {
		// Expand opens the submenu and Collapse closes it, as the keys do.
		a.expandable, a.expanded = true, k+1 < len(path) && path[k] == i
		a.setExpanded = func(open bool) {
			if open {
				m.openSub(k, i, true)
			} else if p := m.path.Peek(); k+1 < len(p) {
				m.set(p[:k+1])
			}
		}
	}
	if !on {
		r.Style(m.st.ItemDisabled)
	}
	if hl && on {
		r.Style(m.st.ItemHighlighted)
	}
	if gutter {
		r.Child(m.mark(it))
	}
	r.Child(Text(it.text()).NoWrap()).Child(Spacer())
	if sc := m.shortcut(it); sc != "" {
		r.Child(Div().asPart("Menu.Shortcut").Style(m.st.Shortcut).Child(Text(sc).NoWrap()))
	}
	if len(it.Submenu) > 0 {
		r.Child(Div().asPart("Menu.SubmenuChevron").Style(m.st.SubmenuChevron).Child(Icon(icons.ChevronRight).Size(14)))
	}
	r.OnMouseEnter(func() { m.hover(k, i) })
	if on {
		r.OnClick(func() { m.activate(k, i) })
	}
	if hl && k < len(path)-1 && len(it.Submenu) > 0 {
		r.Child(m.box(k+1, it.Submenu, path).Overlay(RightStart).OverlayGap(2))
	}
	return r
}

// mark is the 16 px slot before an item's label: its check mark or dot, or its
// icon.
func (m *menuView) mark(it MenuItem) Element {
	t := m.t
	slot := Div().asPart("Menu.Check").Style(m.st.Check)
	switch {
	case it.Check == MenuCheckRadio && it.Checked:
		slot.Child(Div().Size(Px(6), Px(6)).Radius(3).Background(t.Text))
	case it.Check == MenuCheckBox && it.Checked:
		slot.Child(Icon(icons.Check).Size(14))
	case it.Icon != nil:
		slot.Child(Icon(it.Icon).Size(16).Color(t.TextMuted))
	}
	return slot
}

// shortcut is the text of the action's binding, such as "Ctrl+N", or "".
func (m *menuView) shortcut(it MenuItem) string {
	if it.Action == nil {
		return ""
	}
	return m.ctx.KeysFor(it.Action)
}

// contextMenu is an open context menu: the element it is open on, the point
// on that element where it opened (in its local coordinates), and its items.
type contextMenu struct {
	id       uint64
	x, y     float32
	items    []MenuItem
	keyboard bool   // opened from the keyboard: the first item is highlighted
	label    string // the name of the element it is open on, for the menu's name
}

// ContextMenu opens a menu when the element is right-clicked, or when it has
// focus and Shift+F10 or the Menu key is pressed. The menu opens at the pointer,
// or below the focused element for the keyboard. build runs when the menu
// opens; an empty result opens nothing. The menu closes as the menus of a menu
// bar do.
func (d *DivElement) ContextMenu(build func() []MenuItem) *DivElement {
	d.contextMenu = build
	return d
}

// DropdownMenu wraps trigger so that a click opens a menu below it. Enter,
// Space or Down opens it too, while trigger or a control in it has focus. build
// runs when the menu opens; an empty result opens nothing. The wrapper keeps
// the size of the trigger. The menu closes on a second click, on a press
// outside it, or with Escape, and focus returns to where it was.
func DropdownMenu(trigger Element, build func() []MenuItem) *DropdownMenuElement {
	return &DropdownMenuElement{trigger: trigger, build: build}
}

// DropdownMenuElement is a trigger and the menu it opens. See DropdownMenu.
type DropdownMenuElement struct {
	trigger Element
	build   func() []MenuItem
	styles  MenuStyle
	key     string // the element key, see Key
}

// Key identifies the menu among its siblings, so it keeps its state when the
// siblings around it change (see DivElement.Key).
func (dm *DropdownMenuElement) Key(k string) *DropdownMenuElement { dm.key = k; return dm }

func (dm *DropdownMenuElement) elementKey() string { return dm.key }

func (*DropdownMenuElement) isElement() {}

// Styles sets the look of the parts of the menu, for this instance only. Set
// fields win over the theme's Menu and the defaults; see MenuStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (dm *DropdownMenuElement) Styles(st MenuStyle) *DropdownMenuElement {
	dm.styles = dm.styles.merge(st)
	return dm
}

func (dm *DropdownMenuElement) expand(ctx *Context) Element {
	trigger, build, styles := dm.trigger, dm.build, dm.styles
	return Component(func(ctx *Context) Element {
		rt, t := ctx.rt, ctx.Theme()
		open := Local(ctx, "open", false)
		items := Local(ctx, "items", []MenuItem(nil))
		path := Local(ctx, "path", []int{-1})
		closeMenu := func(restore bool) {
			if !open.Peek() {
				return
			}
			open.Set(false)
			rt.menuClosed(restore)
		}
		openMenu := func(keyboard bool) {
			if open.Peek() {
				return
			}
			built := build()
			if len(built) == 0 {
				return
			}
			rt.menuOpened()
			items.Set(built)
			path.Set(highlightPath(built, keyboard))
			open.Set(true)
		}
		d := Div().AlignSelf(Start).Shrink(0).AccessExpanded(open.Get()).
			OnClick(func() {
				if open.Peek() {
					closeMenu(true)
				} else {
					openMenu(false)
				}
			}).
			OnKeyDown(func(e *KeyEvent) {
				if open.Peek() || e.Repeat || e.Mods != 0 {
					return
				}
				if e.Key == input.KeyEnter || e.Key == input.KeySpace || e.Key == input.KeyDown {
					openMenu(true)
					e.StopPropagation()
				}
			}).
			Child(trigger)
		var name string
		if b, ok := trigger.(*ButtonElement); ok {
			// The wrapper is the control that assistive technology sees: it takes
			// the button's name, and the button is left out of the tree. Focus goes
			// to the button, so the wrapper takes focus for it.
			b.inert = true
			name = b.accessName()
			d.Label(name)
			d.access().focusInside = !b.disabled
		}
		if open.Get() {
			m := &menuView{ctx: ctx, t: t, items: items.Get(), path: path, close: closeMenu, label: name,
				st: resolveMenu(t, styles)}
			d.Child(m.top(4))
		}
		return d
	})
}

// menuTarget returns the nearest element on path (deepest first) that has a
// context menu, or nil.
func menuTarget(path []*node) *node {
	for _, n := range path {
		if n.div != nil && n.div.contextMenu != nil {
			return n
		}
	}
	return nil
}

// menuOpened records where focus returns when the menu closes. The first menu
// of a chain sets it: the focus before the press when the pointer opened the
// menu, else the focused element. Later menus of the chain keep it.
func (rt *Runtime) menuOpened() {
	if rt.menuReturn != nil {
		return
	}
	if rt.keyboardModality {
		rt.menuReturn = rt.FocusedHandle()
		return
	}
	rt.menuReturn = rt.handleFor(rt.pointerFocus)
}

// handleFor returns a handle to element id, or to "nothing focused" for 0.
func (rt *Runtime) handleFor(id uint64) *FocusHandle {
	if id == 0 {
		return &FocusHandle{rt: rt, none: true}
	}
	return &FocusHandle{rt: rt, id: id}
}

// menuClosed ends the menu chain: it drops the focus to return to and any
// hover change still waiting, and focuses the return element when restore is
// set. A menu sets its own closed state before it calls this, so the focus
// change does not close it again.
func (rt *Runtime) menuClosed(restore bool) {
	r := rt.menuReturn
	rt.menuReturn = nil
	rt.menuFn = nil
	if restore && r != nil {
		r.FocusNow()
	}
}

// menuLater runs fn after d, unless a later call or menuClosed replaces it.
func (rt *Runtime) menuLater(d time.Duration, fn func()) {
	rt.menuAt, rt.menuFn = rt.now().Add(d), fn
	rt.wakeAt(rt.menuAt)
}

// menuCancel drops the pending hover change, if any.
func (rt *Runtime) menuCancel() { rt.menuFn = nil }

// openContextMenu opens n's context menu at (x, y) in n's local coordinates.
func (rt *Runtime) openContextMenu(n *node, x, y float32, keyboard bool) {
	items := n.div.contextMenu()
	if len(items) == 0 {
		return
	}
	rt.menuOpened()
	rt.ctxMenu = &contextMenu{id: n.id, x: x, y: y, items: items, keyboard: keyboard, label: menuLabel(n)}
	if n.owner != nil {
		n.owner.invalidate()
	}
}

// closeContextMenu closes the open context menu, if any, and restores focus
// when restore is set.
func (rt *Runtime) closeContextMenu(restore bool) {
	c := rt.ctxMenu
	if c == nil {
		return
	}
	rt.ctxMenu = nil
	if n := rt.byID[c.id]; n != nil && n.owner != nil {
		n.owner.invalidate()
	}
	rt.menuClosed(restore)
}

// openFocusedMenu opens the context menu of the nearest element on the focus
// path that has one, below the focused element. It reports whether it did.
func (rt *Runtime) openFocusedMenu() bool {
	path := rt.focusPath()
	target := menuTarget(path)
	if target == nil {
		return false
	}
	f, tr := rt.nodeRect(path[0]), rt.nodeRect(target)
	rt.openContextMenu(target, f.X-tr.X, f.Y+f.H-tr.Y, true)
	return true
}

// contextMenuAt is the element a context menu hangs from: an empty box at the
// point the menu opened at, inside the element that owns the menu. The menu
// is an overlay of that box, so it is placed at the point.
func contextMenuAt(rt *Runtime, c *contextMenu) Element {
	return Div().Absolute().Left(c.x).Top(c.y).Size(Px(0), Px(0)).Child(
		Component(func(ctx *Context) Element {
			m := &menuView{ctx: ctx, t: ctx.Theme(), items: c.items,
				path:  Local(ctx, "path", highlightPath(c.items, c.keyboard)),
				close: rt.closeContextMenu, label: c.label, st: resolveMenu(ctx.Theme(), MenuStyle{})}
			return m.top(0)
		}))
}

// menuLabel is the name of a context menu: the name of the element it hangs
// from, or "Actions" when that element has none. A menu must have a name, since
// it takes keyboard focus.
func menuLabel(n *node) string {
	role, ok := accessRole(n)
	if s := (&accessBuilder{}).name(n, ok && absorbsText(role)); s != "" {
		return s
	}
	return "Actions"
}

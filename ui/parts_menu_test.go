package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// menuItems is a menu with a section label, a row with a check mark, a row with
// an icon, a row with a submenu, a separator, a disabled row and a row for an
// action with a key binding. The first enabled row is "Open".
func menuItems(note *Action) []MenuItem {
	return []MenuItem{
		MenuSection("Status"),
		{Label: "Open", OnClick: func() {}},
		{Label: "Pinned", Check: MenuCheckBox, Checked: true, OnClick: func() {}},
		{Label: "Files", Icon: icons.Check, OnClick: func() {}},
		{Label: "More", Submenu: []MenuItem{{Label: "Deep", OnClick: func() {}}}},
		MenuSeparator(),
		{Label: "Delete", Disabled: true, OnClick: func() {}},
		{Action: note},
	}
}

// menuTrigger is the element that opens the menu, keyed "trig".
func menuTrigger() Element { return Button("Actions").Key("trig") }

// menuRig returns a themed rig with a dropdown menu (ctxMenu false) or a context
// menu on the trigger, with the menu style st set on the instance (dropdown) or
// on the theme (context menu, which takes the theme's Menu only). The key
// binding of the action is set so that the shortcut is drawn.
func menuRig(ctxMenu bool, st MenuStyle) (*accessRig, *Action) {
	note := NewAction("test.note", "New note")
	th := *DarkTheme()
	if ctxMenu {
		th.Menu = st
	}
	r := newThemedRig(&th, func(ctx *Context) Element {
		if ctxMenu {
			return Div().Padding(20).Child(Div().Key("trig").Size(Px(200), Px(30)).Focusable().
				ContextMenu(func() []MenuItem { return menuItems(note) }))
		}
		return Div().Padding(20).Column().Child(DropdownMenu(menuTrigger(), func() []MenuItem {
			return menuItems(note)
		}).Styles(st))
	})
	r.rt.AddBindings(Bind("ctrl+n", note))
	r.frame()
	return r, note
}

// openMenu opens the menu of a rig from menuRig: by the pointer, or by the
// keyboard, which highlights the first enabled row.
func openMenu(t *testing.T, r *accessRig, ctxMenu, keyboard bool) {
	t.Helper()
	switch {
	case !ctxMenu && !keyboard:
		r.clickKey(t, "trig")
	case !ctxMenu && keyboard:
		focusNode(r, keyNode(t, r.rt, "trig"))
		r.press(input.KeyEnter, 0)
	case ctxMenu && !keyboard:
		rc := r.rt.nodeRect(keyNode(t, r.rt, "trig"))
		x, y := rc.X+rc.W/2, rc.Y+rc.H/2
		r.rt.PointerMove(x, y, 0)
		r.rt.PointerDown(x, y, input.ButtonRight, 1, 0)
		r.rt.PointerUp(x, y, input.ButtonRight, 0)
		r.frame()
	default:
		focusNode(r, keyNode(t, r.rt, "trig"))
		r.press(input.KeyF10, input.ModShift)
	}
}

// menuRowNamed returns the row of the open menu with the accessible name.
func menuRowNamed(t *testing.T, r *accessRig, name string) *node {
	t.Helper()
	return accessNode(t, r, access.RoleMenuItem, name)
}

func testMenuParts(t *testing.T, applied map[string]bool) {
	type menuCase struct {
		part     string
		set      func(*MenuStyle, Style)
		keyboard bool   // the menu is opened by the keyboard, which highlights a row
		row      string // the row with this name draws the part, when the part is a row
		drawn    string // the element that draws a part that is not a row
		text     bool   // the part is colored by its text color
	}
	cases := []menuCase{
		{part: "Menu.Panel", set: func(s *MenuStyle, m Style) { s.Panel = m }, drawn: "Menu.Panel"},
		{part: "Menu.Item", set: func(s *MenuStyle, m Style) { s.Item = m }, row: "Files"},
		{part: "Menu.ItemHighlighted", set: func(s *MenuStyle, m Style) { s.ItemHighlighted = m }, keyboard: true, row: "Open"},
		{part: "Menu.ItemDisabled", set: func(s *MenuStyle, m Style) { s.ItemDisabled = m }, row: "Delete", text: true},
		{part: "Menu.Separator", set: func(s *MenuStyle, m Style) { s.Separator = m }, drawn: "Menu.Separator"},
		{part: "Menu.SectionLabel", set: func(s *MenuStyle, m Style) { s.SectionLabel = m }, drawn: "Menu.SectionLabel"},
		{part: "Menu.Shortcut", set: func(s *MenuStyle, m Style) { s.Shortcut = m }, drawn: "Menu.Shortcut", text: true},
		{part: "Menu.Check", set: func(s *MenuStyle, m Style) { s.Check = m }, drawn: "Menu.Check", text: true},
		{part: "Menu.SubmenuChevron", set: func(s *MenuStyle, m Style) { s.SubmenuChevron = m }, drawn: "Menu.SubmenuChevron", text: true},
	}
	for _, widget := range []string{"DropdownMenu", "ContextMenu"} {
		ctxMenu := widget == "ContextMenu"
		for _, c := range cases {
			name := widget + c.part[len("Menu"):]
			applied[name] = true
			t.Run(name, func(t *testing.T) {
				var st MenuStyle
				c.set(&st, Style{Background: markBG, TextColor: markFG})
				r, _ := menuRig(ctxMenu, st)
				openMenu(t, r, ctxMenu, c.keyboard)
				check := func(n *node) {
					if c.text {
						wantTextColor(t, n, name, markFG)
						return
					}
					wantBackground(t, n, name, markBG)
				}
				if c.row != "" {
					check(menuRowNamed(t, r, c.row))
					return
				}
				if len(partNodes(r.rt, c.drawn)) == 0 {
					t.Fatalf("%s: no node draws %s", name, c.drawn)
				}
				found := false
				for _, n := range partNodes(r.rt, c.drawn) {
					if (c.text && n.style.TextColor == markFG) || (!c.text && n.style.Background == markBG) {
						found = true
					}
				}
				if !found {
					t.Errorf("%s: the style does not reach a node that draws %s", name, c.drawn)
				}
			})
		}
	}
}

// testMenuBarParts checks the parts of a menu bar on its labels: BarItem and its
// states BarItemHover, BarItemFocusVisible and BarItemOpen.
func testMenuBarParts(t *testing.T, applied map[string]bool) {
	type barCase struct {
		part  string
		set   func(*MenuBarStyle, Style)
		act   func(*testing.T, *accessRig)
		label string // the key of the label that draws the part
	}
	cases := []barCase{
		{part: "MenuBar.BarItem", set: func(s *MenuBarStyle, m Style) { s.BarItem = m }, label: "jamb.menubar.1"},
		{part: "MenuBar.BarItemHover", set: func(s *MenuBarStyle, m Style) { s.BarItemHover = m }, label: "jamb.menubar.1",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "jamb.menubar.1")) }},
		{part: "MenuBar.BarItemFocusVisible", set: func(s *MenuBarStyle, m Style) { s.BarItemFocusVisible = m }, label: "jamb.menubar.1",
			act: func(t *testing.T, r *accessRig) { focusNode(r, keyNode(t, r.rt, "jamb.menubar.1")) }},
		{part: "MenuBar.BarItemOpen", set: func(s *MenuBarStyle, m Style) { s.BarItemOpen = m }, label: "jamb.menubar.0",
			act: func(t *testing.T, r *accessRig) { r.clickKey(t, "jamb.menubar.0") }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st MenuBarStyle
			c.set(&st, Style{Background: markBG})
			var sink []string
			r := newThemedRig(DarkTheme(), func(ctx *Context) Element {
				return Div().Column().Child(MenuBar(
					TopMenu{Label: "File", Items: func() []MenuItem { return sampleFile(&sink) }},
					TopMenu{Label: "Edit", Items: func() []MenuItem { return sampleEdit(&sink) }},
				).Styles(st))
			})
			if c.act != nil {
				c.act(t, r)
			}
			wantBackground(t, keyNode(t, r.rt, c.label), c.part, markBG)
		})
	}
}

// TestMenuHighlightUnderHighContrast checks the deliberate high-contrast change:
// the keyboard row of a menu is the Highlight color with HighlightText on it,
// since the soft tint of the normal theme is the window color under high contrast.
func TestMenuHighlightUnderHighContrast(t *testing.T) {
	r, _ := menuRig(false, MenuStyle{})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	openMenu(t, r, false, true)
	th := r.rt.theme
	row := menuRowNamed(t, r, "Open")
	if row.style.Background != th.Accent {
		t.Errorf("high contrast: the highlighted row is %v, want Highlight %v", row.style.Background, th.Accent)
	}
	if got := effectiveTextColor(row); got != th.AccentText {
		t.Errorf("high contrast: the highlighted row's text is %v, want HighlightText %v", got, th.AccentText)
	}
}

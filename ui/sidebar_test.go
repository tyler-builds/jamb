package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// sidebarDemo is a sidebar of two headings, five items and one disabled item. The
// inbox has a badge of 4, and "sent" is disabled. The selection is sel, the
// collapsed state collapsed, the instance style st, and onChange is called with
// each selected key. It has a Toggle, and a Footer that names "Settings".
func sidebarDemo(sel *Signal[string], collapsed *Signal[bool], st SidebarStyle, onChange func(string)) Element {
	return Div().Row().Child(Sidebar(sel,
		SidebarHeading("Mail"),
		SidebarItem{Key: "inbox", Label: "Inbox", Icon: icons.Inbox, Badge: 4},
		SidebarItem{Key: "drafts", Label: "Drafts", Icon: icons.Star},
		SidebarItem{Key: "sent", Label: "Sent", Icon: icons.Home, Disabled: true},
		SidebarHeading("Tags"),
		SidebarItem{Key: "archive", Label: "Archive", Icon: icons.Search},
	).Key("nav").Styles(st).Collapsed(collapsed).Toggle().Footer(Text("Settings")).OnChange(onChange))
}

func TestSidebarClickSelectsAndCallsOnChange(t *testing.T) {
	sel := NewSignal("inbox")
	collapsed := NewSignal(false)
	var picked []string
	r := newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(sel, collapsed, SidebarStyle{}, func(k string) { picked = append(picked, k) })
	})
	r.clickKey(t, "nav/item:archive")
	if sel.Peek() != "archive" {
		t.Fatalf("a click selected %q, want archive", sel.Peek())
	}
	if got := strings.Join(picked, ","); got != "archive" {
		t.Fatalf("OnChange got %q, want archive", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSidebarArrowsMoveFocusWithoutSelecting(t *testing.T) {
	sel := NewSignal("inbox")
	collapsed := NewSignal(false)
	var picked []string
	r := newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(sel, collapsed, SidebarStyle{}, func(k string) { picked = append(picked, k) })
	})
	r.focusKey(t, "nav/item:inbox")
	r.press(input.KeyDown, 0) // drafts; the disabled item and the heading are skipped below it
	if got := r.focusedKey(); got != "nav/item:drafts" {
		t.Fatalf("Down moved focus to %q, want drafts", got)
	}
	if sel.Peek() != "inbox" || len(picked) != 0 {
		t.Fatalf("Down changed the selection to %q (OnChange %v); arrows must not select", sel.Peek(), picked)
	}
	r.press(input.KeyDown, 0) // archive: the disabled Sent and the Tags heading are skipped
	if got := r.focusedKey(); got != "nav/item:archive" {
		t.Fatalf("Down moved focus to %q, want archive", got)
	}
	r.press(input.KeyDown, 0) // the last item does not wrap
	if got := r.focusedKey(); got != "nav/item:archive" {
		t.Fatalf("Down past the last item moved focus to %q", got)
	}
	r.press(input.KeyUp, 0) // drafts again
	if got := r.focusedKey(); got != "nav/item:drafts" {
		t.Fatalf("Up moved focus to %q, want drafts", got)
	}
	r.press(input.KeyUp, 0) // inbox: the Mail heading above it is skipped
	r.press(input.KeyUp, 0) // the first item does not wrap
	if got := r.focusedKey(); got != "nav/item:inbox" {
		t.Fatalf("Up from the first item moved focus to %q, want inbox", got)
	}
	r.press(input.KeyEnd, 0)
	if got := r.focusedKey(); got != "nav/item:archive" {
		t.Fatalf("End moved focus to %q, want archive", got)
	}
	r.press(input.KeyHome, 0)
	if got := r.focusedKey(); got != "nav/item:inbox" {
		t.Fatalf("Home moved focus to %q, want inbox", got)
	}
	if sel.Peek() != "inbox" || len(picked) != 0 {
		t.Fatalf("the keys changed the selection to %q (OnChange %v)", sel.Peek(), picked)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSidebarEnterAndSpaceSelect(t *testing.T) {
	sel := NewSignal("inbox")
	collapsed := NewSignal(false)
	var picked []string
	r := newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(sel, collapsed, SidebarStyle{}, func(k string) { picked = append(picked, k) })
	})
	r.focusKey(t, "nav/item:inbox")
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnter, 0)
	if sel.Peek() != "drafts" {
		t.Fatalf("Enter selected %q, want drafts", sel.Peek())
	}
	r.press(input.KeyDown, 0)
	r.press(input.KeySpace, 0)
	if sel.Peek() != "archive" {
		t.Fatalf("Space selected %q, want archive", sel.Peek())
	}
	if got := strings.Join(picked, ","); got != "drafts,archive" {
		t.Fatalf("OnChange got %q, want drafts,archive", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSidebarOneTabStopAtTheSelectedItem(t *testing.T) {
	inOrder := func(r *accessRig, key string) bool {
		n := r.nodeByKey(t, key)
		for _, id := range r.rt.tabOrder() {
			if id == n.id {
				return true
			}
		}
		return false
	}
	r := newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(NewSignal("drafts"), NewSignal(false), SidebarStyle{}, nil)
	})
	r.frame()
	if !inOrder(r, "nav/item:drafts") || inOrder(r, "nav/item:inbox") || inOrder(r, "nav/item:archive") {
		t.Fatalf("the Tab order is not the selected item alone")
	}
	// A selection that is disabled, or that is none, leaves the first enabled item as the stop.
	r = newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(NewSignal("sent"), NewSignal(false), SidebarStyle{}, nil)
	})
	r.frame()
	if !inOrder(r, "nav/item:inbox") || inOrder(r, "nav/item:sent") {
		t.Fatalf("with a disabled selection the Tab stop is not the first enabled item")
	}
}

func TestSidebarCollapsedShowsIconsAndTooltips(t *testing.T) {
	sel := NewSignal("inbox")
	collapsed := NewSignal(true)
	r := newAccessRig(func(ctx *Context) Element { return sidebarDemo(sel, collapsed, SidebarStyle{}, nil) })
	r.frame()
	if w := keyNode(t, r.rt, "nav").style.Width; w != Px(sidebarCollapsedWidth) {
		t.Fatalf("a collapsed sidebar is %v wide, want %v", w, Px(sidebarCollapsedWidth))
	}
	for _, n := range r.rt.byID {
		if n.kind == kindText && n.textEl != nil && n.textEl.text == "Inbox" {
			t.Fatalf("a collapsed sidebar draws the label %q", n.textEl.text)
		}
	}
	if tip := keyNode(t, r.rt, "nav/item:inbox").div.tooltip; tip != "Inbox" {
		t.Fatalf("the tooltip of the inbox is %q, want its label", tip)
	}
	// The count is not drawn, but the name keeps it.
	accesstest.Check(t, r.stateOf(t, access.RoleListItem, "Inbox, 4"), access.RoleListItem, "Inbox, 4",
		access.StateSelectable|access.StateSelected|access.StateFocusable)
	collapsed.Set(false)
	r.frame()
	if w := keyNode(t, r.rt, "nav").style.Width; w != Px(sidebarWidth) {
		t.Fatalf("an expanded sidebar is %v wide, want %v", w, Px(sidebarWidth))
	}
	if tip := keyNode(t, r.rt, "nav/item:inbox").div.tooltip; tip != "" {
		t.Fatalf("an expanded item has the tooltip %q", tip)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSidebarToggleFlipsCollapsed(t *testing.T) {
	collapsed := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(NewSignal("inbox"), collapsed, SidebarStyle{}, nil)
	})
	btn := r.stateOf(t, access.RoleButton, "Collapse navigation")
	if !btn.States.Has(access.StateExpanded) {
		t.Fatalf("the toggle of an expanded sidebar does not report expanded")
	}
	r.clickKey(t, "nav/toggle")
	if !collapsed.Peek() {
		t.Fatalf("the toggle did not collapse the sidebar")
	}
	btn = r.stateOf(t, access.RoleButton, "Expand navigation")
	if btn.States.Has(access.StateExpanded) {
		t.Fatalf("the toggle of a collapsed sidebar reports expanded")
	}
	r.clickKey(t, "nav/toggle")
	if collapsed.Peek() {
		t.Fatalf("the toggle did not expand the sidebar again")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSidebarAccessibilityTree(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(NewSignal("inbox"), NewSignal(false), SidebarStyle{}, nil)
	})
	pane := r.stateOf(t, access.RoleGroup, "Navigation")
	if pane.Name != "Navigation" {
		t.Fatalf("the pane is named %q", pane.Name)
	}
	accesstest.Check(t, r.stateOf(t, access.RoleHeading, "Mail"), access.RoleHeading, "Mail", 0)
	inbox := r.stateOf(t, access.RoleListItem, "Inbox, 4")
	accesstest.Check(t, inbox, access.RoleListItem, "Inbox, 4",
		access.StateSelectable|access.StateSelected|access.StateFocusable)
	drafts := r.stateOf(t, access.RoleListItem, "Drafts")
	if drafts.States.Has(access.StateSelected) {
		t.Fatalf("Drafts is selected; only the inbox is")
	}
	sent := r.stateOf(t, access.RoleListItem, "Sent")
	if !sent.States.Has(access.StateDisabled) || sent.States.Has(access.StateSelected) {
		t.Fatalf("Sent is %v, want disabled and not selected", sent.States)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSidebarHeaderAndFooterStayOutsideTheItems(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(NewSignal("inbox"), NewSignal(false), SidebarStyle{}, nil)
	})
	r.frame()
	var settings, archive *node
	for _, n := range r.rt.byID {
		if n.kind == kindText && n.textEl != nil {
			switch n.textEl.text {
			case "Settings":
				settings = n
			case "Archive":
				archive = n
			}
		}
	}
	if settings == nil || archive == nil {
		t.Fatalf("the footer or the last item is not drawn")
	}
	sc, ac := r.rt.nodeRect(settings), r.rt.nodeRect(archive)
	if sc.Y < ac.Y+ac.H {
		t.Fatalf("the footer is at y %v, above the last item (bottom %v)", sc.Y, ac.Y+ac.H)
	}
}

func TestSidebarWidthFollowsTextScaleAndSizing(t *testing.T) {
	sel := NewSignal("inbox")
	collapsed := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element { return sidebarDemo(sel, collapsed, SidebarStyle{}, nil) })
	if w := keyNode(t, r.rt, "nav").style.Width; w != Px(sidebarWidth) {
		t.Fatalf("the default width is %v, want %v", w, Px(sidebarWidth))
	}
	r.rt.SetSystemSettings(SystemSettings{TextScale: 225})
	r.frame()
	if w := keyNode(t, r.rt, "nav").style.Width; w != Px(sidebarWidth*2.25) {
		t.Fatalf("at 225 percent the width is %v, want %v", w, Px(sidebarWidth*2.25))
	}
	collapsed.Set(true)
	r.frame()
	if w := keyNode(t, r.rt, "nav").style.Width; w != Px(sidebarCollapsedWidth) {
		t.Fatalf("collapsed at 225 percent the width is %v, want %v", w, Px(sidebarCollapsedWidth))
	}
	// A width the app sets wins over the default when expanded, and not when collapsed.
	wide := NewSignal(false)
	r = newAccessRig(func(ctx *Context) Element {
		return Div().Row().Child(Sidebar(NewSignal("inbox"), SidebarItem{Key: "inbox", Label: "Inbox"}).
			Key("nav").Collapsed(wide).Width(Px(300)).MinWidth(Px(200)))
	})
	if w := keyNode(t, r.rt, "nav").style.Width; w != Px(300) {
		t.Fatalf("the width set by the app is %v, want 300", w)
	}
	wide.Set(true)
	r.frame()
	if w := keyNode(t, r.rt, "nav").style.Width; w != Px(sidebarCollapsedWidth) {
		t.Fatalf("collapsed, the width is %v, want %v", w, Px(sidebarCollapsedWidth))
	}
}

func TestSidebarCollapseIsNotAnimated(t *testing.T) {
	for _, reduced := range []bool{true, false} {
		collapsed := NewSignal(false)
		r := newAccessRig(func(ctx *Context) Element {
			return sidebarDemo(NewSignal("inbox"), collapsed, SidebarStyle{}, nil)
		})
		r.rt.SetSystemSettings(SystemSettings{ReducedMotion: reduced})
		r.frame()
		r.clickKey(t, "nav/toggle")
		// The width is the collapsed one on the first frame after the change, with no
		// width animation, reduced motion or not.
		if w := keyNode(t, r.rt, "nav").style.Width; w != Px(sidebarCollapsedWidth) {
			t.Fatalf("reduced motion %v: the width is %v one frame after the change, want %v",
				reduced, w, Px(sidebarCollapsedWidth))
		}
	}
}

func TestSidebarHighContrastSelectionIsTheHighlight(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(NewSignal("inbox"), NewSignal(false), SidebarStyle{}, nil)
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	inbox := keyNode(t, r.rt, "nav/item:inbox")
	if inbox.style.Background != r.rt.theme.Accent || inbox.style.TextColor != r.rt.theme.AccentText {
		t.Fatalf("the selected item is %v on %v, want AccentText on Accent (the highlight)",
			inbox.style.TextColor, inbox.style.Background)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// partSidebar is sidebarDemo, with the inbox selected and the sidebar expanded,
// for the parts test.
func partSidebar(st SidebarStyle) Element {
	return sidebarDemo(NewSignal("inbox"), NewSignal(false), st, nil)
}

func testSidebarParts(t *testing.T, applied map[string]bool) {
	type sidebarCase struct {
		part  string
		set   func(*SidebarStyle, Style)
		key   string // the element whose node draws the part, "" for the only one
		drawn string // the part whose node draws it
		act   func(*testing.T, *accessRig)
	}
	cases := []sidebarCase{
		{part: "Sidebar.Pane", set: func(s *SidebarStyle, m Style) { s.Pane = m }, key: "nav", drawn: "Sidebar.Pane"},
		{part: "Sidebar.Item", set: func(s *SidebarStyle, m Style) { s.Item = m }, key: "nav/item:drafts", drawn: "Sidebar.Item"},
		{part: "Sidebar.ItemHover", set: func(s *SidebarStyle, m Style) { s.ItemHover = m }, key: "nav/item:drafts", drawn: "Sidebar.Item",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "nav/item:drafts")) }},
		{part: "Sidebar.ItemSelected", set: func(s *SidebarStyle, m Style) { s.ItemSelected = m }, key: "nav/item:inbox", drawn: "Sidebar.Item"},
		{part: "Sidebar.ItemDisabled", set: func(s *SidebarStyle, m Style) { s.ItemDisabled = m }, key: "nav/item:sent", drawn: "Sidebar.Item"},
		{part: "Sidebar.ItemFocusVisible", set: func(s *SidebarStyle, m Style) { s.ItemFocusVisible = m }, key: "nav/item:drafts", drawn: "Sidebar.Item",
			act: func(t *testing.T, r *accessRig) { focusNode(r, keyNode(t, r.rt, "nav/item:drafts")) }},
		{part: "Sidebar.Indicator", set: func(s *SidebarStyle, m Style) { s.Indicator = m }, drawn: "Sidebar.Indicator"},
		{part: "Sidebar.Heading", set: func(s *SidebarStyle, m Style) { s.Heading = m }, key: "nav/heading:0", drawn: "Sidebar.Heading"},
		{part: "Sidebar.Toggle", set: func(s *SidebarStyle, m Style) { s.Toggle = m }, key: "nav/toggle", drawn: "Sidebar.Toggle"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st SidebarStyle
			c.set(&st, Style{Background: markBG})
			r := newAccessRig(func(ctx *Context) Element { return partSidebar(st) })
			if c.act != nil {
				c.act(t, r)
			}
			wantBackground(t, partNode(t, r.rt, c.drawn, c.key), c.part, markBG)
		})
	}
}

func TestSidebarOnChangeIsCalledOnceForAReselect(t *testing.T) {
	sel := NewSignal("inbox")
	var picked []string
	r := newAccessRig(func(ctx *Context) Element {
		return sidebarDemo(sel, NewSignal(false), SidebarStyle{}, func(k string) { picked = append(picked, k) })
	})
	r.clickKey(t, "nav/item:inbox") // already selected: nothing changes
	if len(picked) != 0 {
		t.Fatalf("clicking the selected item called OnChange with %v", picked)
	}
	r.clickKey(t, "nav/item:drafts")
	if got := strings.Join(picked, ","); got != "drafts" {
		t.Fatalf("OnChange got %q, want drafts once", got)
	}
}

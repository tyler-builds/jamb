package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// splitButtonRig is a Save split button between two buttons, with a menu of Save as and
// Save a copy. ran records what ran.
func splitButtonRig(build func(*SplitButtonElement) *SplitButtonElement) (*accessRig, *[]string) {
	ran := &[]string{}
	r := newAccessRig(func(ctx *Context) Element {
		sb := SplitButton("Save", func() { *ran = append(*ran, "Save") }, func() []MenuItem {
			return []MenuItem{
				{Label: "Save as", OnClick: func() { *ran = append(*ran, "Save as") }},
				{Label: "Save a copy", OnClick: func() { *ran = append(*ran, "Save a copy") }},
			}
		}).Key("save")
		if build != nil {
			sb = build(sb)
		}
		return Div().Column().Align(Start).Child(Button("Before").Key("before")).Child(sb)
	})
	return r, ran
}

// Assistive technology sees a group named by the label with two buttons: the action
// and the arrow, which says whether its menu is open.
func TestSplitButtonAccess(t *testing.T) {
	r, _ := splitButtonRig(nil)
	tr := r.tree()
	group := accesstest.Find(t, tr, access.RoleGroup, "Save")
	if kids := accesstest.Children(tr, group); len(kids) != 2 {
		t.Fatalf("the group holds %q, want the action and the arrow", kids)
	}
	main := accesstest.Find(t, tr, access.RoleButton, "Save")
	arrow := accesstest.Find(t, tr, access.RoleButton, "More options")
	for _, n := range []*access.Node{main, arrow} {
		if !n.States.Has(access.StateFocusable) {
			t.Errorf("%s is not a Tab stop", n.Name)
		}
	}
	if !arrow.States.Has(access.StateExpandable) || arrow.States.Has(access.StateExpanded) {
		t.Errorf("the arrow has states %v, want a collapsed expandable button", arrow.States)
	}
}

// A click on the action runs it; a click on the arrow opens the menu, and choosing
// an item runs it and closes the menu.
func TestSplitButtonPointer(t *testing.T) {
	r, ran := splitButtonRig(nil)
	main := partNode(t, r.rt, "SplitButton.Main", "")
	rc := r.rt.nodeRect(main)
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if len(*ran) != 1 || (*ran)[0] != "Save" {
		t.Fatalf("a click on the action ran %q", *ran)
	}
	arrow := partNode(t, r.rt, "SplitButton.Arrow", "")
	rc = r.rt.nodeRect(arrow)
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleMenu, "Save")
	if n := accesstest.Find(t, tr, access.RoleButton, "More options"); !n.States.Has(access.StateExpanded) {
		t.Errorf("the arrow of an open menu has states %v, want expanded", n.States)
	}
	item := accessNode(t, r, access.RoleMenuItem, "Save a copy")
	rc = r.rt.nodeRect(item)
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	r.frame()
	if len(*ran) != 2 || (*ran)[1] != "Save a copy" {
		t.Errorf("choosing the item ran %q", *ran)
	}
	accesstest.Absent(t, r.tree(), access.RoleMenu, "Save")
}

// Down on the arrow, and Alt+Down on either segment, open the menu with its first
// item highlighted; Escape closes it and returns the focus to the segment.
func TestSplitButtonKeys(t *testing.T) {
	r, ran := splitButtonRig(nil)
	r.focusAccess(t, access.RoleButton, "More options")
	r.press(input.KeyDown, 0)
	accesstest.Find(t, r.tree(), access.RoleMenu, "Save")
	r.press(input.KeyEscape, 0)
	accesstest.Absent(t, r.tree(), access.RoleMenu, "Save")
	if !r.focusedAccess(t, access.RoleButton, "More options") {
		t.Error("Escape did not return the focus to the arrow")
	}

	r.focusAccess(t, access.RoleButton, "Save")
	r.press(input.KeyDown, input.ModAlt)
	accesstest.Find(t, r.tree(), access.RoleMenu, "Save")
	r.press(input.KeyEnter, 0) // the first item is highlighted
	r.frame()
	if len(*ran) != 1 || (*ran)[0] != "Save as" {
		t.Errorf("Enter in the menu ran %q, want Save as", *ran)
	}
	if !r.focusedAccess(t, access.RoleButton, "Save") {
		t.Error("choosing an item did not return the focus to the action")
	}
	// Enter on the action runs it.
	r.press(input.KeyEnter, 0)
	if len(*ran) != 2 || (*ran)[1] != "Save" {
		t.Errorf("Enter on the action ran %q", *ran)
	}
}

// A disabled split button takes no focus and opens no menu.
func TestSplitButtonDisabled(t *testing.T) {
	r, ran := splitButtonRig(func(b *SplitButtonElement) *SplitButtonElement { return b.Disabled(true) })
	tr := r.tree()
	for _, name := range []string{"Save", "More options"} {
		n := accesstest.Find(t, tr, access.RoleButton, name)
		if !n.States.Has(access.StateDisabled) || n.States.Has(access.StateFocusable) {
			t.Errorf("%s of a disabled split button has states %v", name, n.States)
		}
	}
	rc := r.rt.nodeRect(partNode(t, r.rt, "SplitButton.Arrow", ""))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	accesstest.Absent(t, r.tree(), access.RoleMenu, "Save")
	if len(*ran) != 0 {
		t.Errorf("a disabled split button ran %q", *ran)
	}
}

// The segments share one line: a Secondary arrow has no left border, and each
// segment rounds only its outer corners.
func TestSplitButtonSegments(t *testing.T) {
	th := DarkTheme()
	st := splitButtonDefaults(th, ButtonSecondary, sizeDefault)
	if st.Arrow.BorderEdges != (Edges{Top: 1, Right: 1, Bottom: 1}) {
		t.Errorf("a Secondary arrow has the borders %v, want no left one", st.Arrow.BorderEdges)
	}
	if st.Main.Corners.TopRight != 0 || st.Main.Corners.TopLeft == 0 || st.Arrow.Corners.TopLeft != 0 || st.Arrow.Corners.TopRight == 0 {
		t.Errorf("the corners are %+v and %+v, want only the outer ones rounded", st.Main.Corners, st.Arrow.Corners)
	}
	st = splitButtonDefaults(th, ButtonPrimary, sizeDefault)
	if st.Arrow.BorderEdges != (Edges{Left: 1}) {
		t.Errorf("a Primary arrow has the borders %v, want the line on its left", st.Arrow.BorderEdges)
	}
	if st.Main.Shadows != nil || st.Root.Shadows == nil {
		t.Error("the shadow is not the row's")
	}
	// The split button is as tall as a button of the same size.
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Row().Align(Start).Child(Button("B").Small().Key("b")).
			Child(SplitButton("S", nil, nil).Small().Icon(icons.Check).Key("s"))
	})
	if bh, sh := r.rt.nodeRect(r.keyed(t, "b")).H, r.rt.nodeRect(partNode(t, r.rt, "SplitButton.Root", "")).H; bh != sh {
		t.Errorf("a small split button is %v tall, want %v as a small button", sh, bh)
	}
}

// partSplitButton is a Secondary split button with an icon, disabled or not, with
// the instance style st.
func partSplitButton(st SplitButtonStyle, disabled bool) Element {
	return Div().Column().Align(Start).Child(SplitButton("Save", func() {}, func() []MenuItem {
		return []MenuItem{{Label: "Save as"}}
	}).Secondary().Icon(icons.Check).Key("save").Disabled(disabled).Styles(st))
}

func testSplitButtonParts(t *testing.T, applied map[string]bool) {
	type splitCase struct {
		part     string
		set      func(*SplitButtonStyle, Style)
		drawn    string
		disabled bool
		text     bool
		act      func(*testing.T, *accessRig)
	}
	seg := func(part string) func(*testing.T, *accessRig) *node {
		return func(t *testing.T, r *accessRig) *node { return partNode(t, r.rt, part, "") }
	}
	main, arrow := seg("SplitButton.Main"), seg("SplitButton.Arrow")
	cases := []splitCase{
		{part: "SplitButton.Root", set: func(s *SplitButtonStyle, m Style) { s.Root = m }, drawn: "SplitButton.Root"},
		{part: "SplitButton.Main", set: func(s *SplitButtonStyle, m Style) { s.Main = m }, drawn: "SplitButton.Main"},
		{part: "SplitButton.MainHover", set: func(s *SplitButtonStyle, m Style) { s.MainHover = m }, drawn: "SplitButton.Main",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, main(t, r)) }},
		{part: "SplitButton.MainPressed", set: func(s *SplitButtonStyle, m Style) { s.MainPressed = m }, drawn: "SplitButton.Main",
			act: func(t *testing.T, r *accessRig) { pressNode(r, main(t, r)) }},
		{part: "SplitButton.MainDisabled", set: func(s *SplitButtonStyle, m Style) { s.MainDisabled = m }, drawn: "SplitButton.Main", disabled: true},
		{part: "SplitButton.MainFocusVisible", set: func(s *SplitButtonStyle, m Style) { s.MainFocusVisible = m }, drawn: "SplitButton.Main",
			act: func(t *testing.T, r *accessRig) { focusNode(r, main(t, r)) }},
		{part: "SplitButton.Arrow", set: func(s *SplitButtonStyle, m Style) { s.Arrow = m }, drawn: "SplitButton.Arrow"},
		{part: "SplitButton.ArrowHover", set: func(s *SplitButtonStyle, m Style) { s.ArrowHover = m }, drawn: "SplitButton.Arrow",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, arrow(t, r)) }},
		{part: "SplitButton.ArrowPressed", set: func(s *SplitButtonStyle, m Style) { s.ArrowPressed = m }, drawn: "SplitButton.Arrow",
			act: func(t *testing.T, r *accessRig) { pressNode(r, arrow(t, r)) }},
		{part: "SplitButton.ArrowDisabled", set: func(s *SplitButtonStyle, m Style) { s.ArrowDisabled = m }, drawn: "SplitButton.Arrow", disabled: true},
		{part: "SplitButton.ArrowFocusVisible", set: func(s *SplitButtonStyle, m Style) { s.ArrowFocusVisible = m }, drawn: "SplitButton.Arrow",
			act: func(t *testing.T, r *accessRig) { focusNode(r, arrow(t, r)) }},
		{part: "SplitButton.Label", set: func(s *SplitButtonStyle, m Style) { s.Label = m }, drawn: "SplitButton.Label", text: true},
		{part: "SplitButton.Icon", set: func(s *SplitButtonStyle, m Style) { s.Icon = m }, drawn: "SplitButton.Icon"},
		{part: "SplitButton.Chevron", set: func(s *SplitButtonStyle, m Style) { s.Chevron = m }, drawn: "SplitButton.Chevron"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st SplitButtonStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return partSplitButton(st, c.disabled) })
			if c.act != nil {
				c.act(t, r)
			}
			n := partNode(t, r.rt, c.drawn, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

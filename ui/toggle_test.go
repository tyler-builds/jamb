package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// partToggle is a toggle with an icon and a label, on or off, disabled or not,
// with the instance style st.
func partToggle(on bool, st ToggleStyle, disabled bool) Element {
	return Div().Column().Child(Toggle("Bold", NewSignal(on)).Icon(icons.Check).Key("bold").Disabled(disabled).Styles(st))
}

func testToggleParts(t *testing.T, applied map[string]bool) {
	type toggleCase struct {
		part     string
		set      func(*ToggleStyle, Style)
		on       bool
		disabled bool
		drawn    string
		text     bool
		act      func(*testing.T, *accessRig)
	}
	cases := []toggleCase{
		{part: "Toggle.Root", set: func(s *ToggleStyle, m Style) { s.Root = m }, drawn: "Toggle.Root"},
		{part: "Toggle.RootChecked", set: func(s *ToggleStyle, m Style) { s.RootChecked = m }, on: true, drawn: "Toggle.Root"},
		{part: "Toggle.RootHover", set: func(s *ToggleStyle, m Style) { s.RootHover = m }, drawn: "Toggle.Root",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, "Toggle.Root", "")) }},
		{part: "Toggle.RootPressed", set: func(s *ToggleStyle, m Style) { s.RootPressed = m }, drawn: "Toggle.Root",
			act: func(t *testing.T, r *accessRig) { pressNode(r, partNode(t, r.rt, "Toggle.Root", "")) }},
		{part: "Toggle.RootDisabled", set: func(s *ToggleStyle, m Style) { s.RootDisabled = m }, disabled: true, drawn: "Toggle.Root"},
		{part: "Toggle.RootFocusVisible", set: func(s *ToggleStyle, m Style) { s.RootFocusVisible = m }, drawn: "Toggle.Root",
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "Toggle.Root", "")) }},
		{part: "Toggle.Label", set: func(s *ToggleStyle, m Style) { s.Label = m }, drawn: "Toggle.Label", text: true},
		{part: "Toggle.Icon", set: func(s *ToggleStyle, m Style) { s.Icon = m }, drawn: "Toggle.Icon"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st ToggleStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return partToggle(c.on, st, c.disabled) })
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

// partToggleGroup is a toggle group of three options with icons, the second
// selected and the third disabled, with the instance style st. disabled disables
// the whole group.
func partToggleGroup(st ToggleGroupStyle, disabled bool) Element {
	return Div().Column().Child(ToggleGroup(NewSignal("center"),
		Option[string]{Value: "left", Label: "Left", Icon: icons.ChevronLeft},
		Option[string]{Value: "center", Label: "Center", Icon: icons.Minus},
		Option[string]{Value: "right", Label: "Right", Icon: icons.ChevronRight, Disabled: true},
	).Key("align").Label("Alignment").Disabled(disabled).Styles(st))
}

func testToggleGroupParts(t *testing.T, applied map[string]bool) {
	type groupCase struct {
		part  string
		set   func(*ToggleGroupStyle, Style)
		drawn string // the part that draws it
		key   string // the key of the node, for an item
		text  bool
		field bool // the group is in a Field that shows an error
		act   func(*testing.T, *accessRig)
	}
	cases := []groupCase{
		{part: "ToggleGroup.Group", set: func(s *ToggleGroupStyle, m Style) { s.Group = m }, drawn: "ToggleGroup.Group"},
		{part: "ToggleGroup.GroupInvalid", set: func(s *ToggleGroupStyle, m Style) { s.GroupInvalid = m }, drawn: "ToggleGroup.Group", field: true},
		{part: "ToggleGroup.Item", set: func(s *ToggleGroupStyle, m Style) { s.Item = m }, drawn: "ToggleGroup.Item", key: "align/left"},
		{part: "ToggleGroup.ItemSelected", set: func(s *ToggleGroupStyle, m Style) { s.ItemSelected = m }, drawn: "ToggleGroup.Item", key: "align/center"},
		{part: "ToggleGroup.ItemHover", set: func(s *ToggleGroupStyle, m Style) { s.ItemHover = m }, drawn: "ToggleGroup.Item", key: "align/left",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "align/left")) }},
		{part: "ToggleGroup.ItemPressed", set: func(s *ToggleGroupStyle, m Style) { s.ItemPressed = m }, drawn: "ToggleGroup.Item", key: "align/left",
			act: func(t *testing.T, r *accessRig) { pressNode(r, keyNode(t, r.rt, "align/left")) }},
		{part: "ToggleGroup.ItemFocusVisible", set: func(s *ToggleGroupStyle, m Style) { s.ItemFocusVisible = m }, drawn: "ToggleGroup.Item", key: "align/center",
			act: func(t *testing.T, r *accessRig) { focusNode(r, keyNode(t, r.rt, "align/center")) }},
		{part: "ToggleGroup.ItemDisabled", set: func(s *ToggleGroupStyle, m Style) { s.ItemDisabled = m }, drawn: "ToggleGroup.Item", key: "align/right"},
		{part: "ToggleGroup.Label", set: func(s *ToggleGroupStyle, m Style) { s.Label = m }, drawn: "ToggleGroup.Label", text: true},
		{part: "ToggleGroup.Icon", set: func(s *ToggleGroupStyle, m Style) { s.Icon = m }, drawn: "ToggleGroup.Icon"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st ToggleGroupStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element {
				if c.field {
					return Field("Alignment", ToggleGroup(NewSignal(""), Option[string]{Value: "a", Label: "A"}).
						Styles(st)).Error("Choose one.")
				}
				return partToggleGroup(st, false)
			})
			r.frame()
			if c.act != nil {
				c.act(t, r)
			}
			var n *node
			switch {
			case c.key != "":
				n = keyNode(t, r.rt, c.key)
			case c.drawn == "ToggleGroup.Label" || c.drawn == "ToggleGroup.Icon":
				n = partNodes(r.rt, c.drawn)[0]
			default:
				n = partNode(t, r.rt, c.drawn, "")
			}
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

func TestToggleClickAndSpaceToggleButNotEnter(t *testing.T) {
	on := NewSignal(false)
	var changes []bool
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Toggle("Bold", on).Key("bold").OnChange(func(v bool) {
			if v != on.Peek() {
				t.Errorf("OnChange(%v) ran before the signal was set", v)
			}
			changes = append(changes, v)
		}))
	})
	n := r.stateOf(t, access.RoleButton, "Bold")
	accesstest.Check(t, n, access.RoleButton, "Bold", access.StateToggle|access.StateFocusable)
	if n.States.Has(access.StateChecked) || !n.Has(access.ActionToggle) {
		t.Fatalf("an off toggle has states [%s] and actions %v", n.States, n.Actions)
	}
	r.clickKey(t, "bold")
	if !on.Peek() {
		t.Fatalf("a click did not turn the toggle on")
	}
	if !r.stateOf(t, access.RoleButton, "Bold").States.Has(access.StateChecked) {
		t.Fatalf("an on toggle is not reported as checked")
	}
	r.focusKey(t, "bold")
	r.press(input.KeySpace, 0)
	if on.Peek() {
		t.Fatalf("Space did not turn the toggle off")
	}
	r.press(input.KeyEnter, 0)
	if on.Peek() {
		t.Fatalf("Enter turned the toggle on; it goes on to the default button")
	}
	if len(changes) != 2 || !changes[0] || changes[1] {
		t.Fatalf("OnChange ran with %v, want [true false]", changes)
	}
	// The assistive technology toggles it through the Toggle pattern.
	r.bridge.Perform(r.stateOf(t, access.RoleButton, "Bold").ID, access.ActionToggle)
	r.frame()
	if !on.Peek() {
		t.Fatalf("the Toggle action did not turn the toggle on")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestToggleIconOnlyKeepsItsLabelAsName(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Toggle("Bold", NewSignal(true)).Icon(icons.Check).IconOnly())
	})
	accesstest.Check(t, r.stateOf(t, access.RoleButton, "Bold"), access.RoleButton, "Bold", access.StateToggle|access.StateChecked)
	if len(partNodes(r.rt, "Toggle.Label")) != 0 || len(partNodes(r.rt, "Toggle.Icon")) != 1 {
		t.Fatalf("an icon-only toggle draws %d labels and %d icons", len(partNodes(r.rt, "Toggle.Label")), len(partNodes(r.rt, "Toggle.Icon")))
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
	// With no icon, IconOnly still shows the label.
	r = newAccessRig(func(ctx *Context) Element { return Div().Column().Child(Toggle("Bold", NewSignal(false)).IconOnly()) })
	if len(partNodes(r.rt, "Toggle.Label")) != 1 {
		t.Fatalf("an icon-only toggle with no icon has no label")
	}
}

func TestToggleIsAsTallAsAButton(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Row().Align(Start).Child(Button("Save").Ghost().Key("b")).Child(Toggle("Bold", NewSignal(true)).Key("t")).
			Child(Button("Save").Small().Key("sb")).Child(Toggle("Bold", NewSignal(false)).Small().Key("st"))
	})
	for _, p := range [][2]string{{"b", "t"}, {"sb", "st"}} {
		if bh, th := r.rt.nodeRect(keyNode(t, r.rt, p[0])).H, r.rt.nodeRect(keyNode(t, r.rt, p[1])).H; bh != th {
			t.Errorf("button %s is %v high, toggle %s %v", p[0], bh, p[1], th)
		}
	}
}

func TestToggleDisabledIsNotFocusable(t *testing.T) {
	on := NewSignal(true)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Toggle("Bold", on).Key("bold").Disabled(true))
	})
	n := r.stateOf(t, access.RoleButton, "Bold")
	if !n.States.Has(access.StateDisabled) || n.States.Has(access.StateFocusable) || !n.States.Has(access.StateChecked) {
		t.Fatalf("a disabled on toggle has states [%s]", n.States)
	}
	rc := r.rt.nodeRect(partNode(t, r.rt, "Toggle.Root", ""))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if !on.Peek() {
		t.Fatalf("a click changed a disabled toggle")
	}
}

// TestToggleStateKeepsTheWeight checks that turning a toggle on, and the pointer
// over it, change its fill and border and never the weight of its text.
func TestToggleStateKeepsTheWeight(t *testing.T) {
	for _, th := range []*Theme{DarkTheme(), LightTheme(), HighContrastTheme(HighContrastBlack(), DarkTheme())} {
		for _, v := range []ButtonVariant{ButtonGhost, ButtonSecondary} {
			off := toggleDefaults(th, v, sizeDefault, false, false, false)
			on := toggleDefaults(th, v, sizeDefault, false, true, false)
			for _, s := range []Style{off.RootChecked, off.RootHover, off.RootPressed, off.RootFocusVisible,
				on.RootChecked, on.RootHover, on.RootPressed} {
				if s.FontWeight != 0 {
					t.Errorf("%s variant %d: a state sets the weight %v", th.Name, v, s.FontWeight)
				}
			}
			if on.RootChecked.Background.IsZero() || on.RootChecked.BorderColor.IsZero() {
				t.Errorf("%s variant %d: an on toggle has no fill or border: %+v", th.Name, v, on.RootChecked)
			}
		}
	}
}

func TestToggleUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Toggle = ToggleStyle{Root: Style{Background: markBG}, RootChecked: Style{Background: markBG}}
	theme.ToggleGroup = ToggleGroupStyle{ItemSelected: Style{Background: markBG}}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Div().Column().Child(Toggle("Bold", NewSignal(true)).Key("bold")).
			Child(ToggleGroup(NewSignal("a"), Option[string]{Value: "a", Label: "Alpha"}, Option[string]{Value: "b", Label: "Beta"}).
				Key("g").Label("Letters"))
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	if n := partNode(t, r.rt, "Toggle.Root", ""); n.style.Background != th.Accent || n.style.TextColor != th.AccentText {
		t.Errorf("high contrast: the on toggle is %v on %v, want HighlightText on Highlight", n.style.TextColor, n.style.Background)
	}
	if n := keyNode(t, r.rt, "g/a"); n.style.Background != th.Accent {
		t.Errorf("high contrast: the selected item is %v, want Highlight %v", n.style.Background, th.Accent)
	}
	focusNode(r, partNode(t, r.rt, "Toggle.Root", ""))
	if len(partNode(t, r.rt, "Toggle.Root", "").style.Shadows) == 0 {
		t.Errorf("high contrast: the focused toggle has no focus ring")
	}
}

// toggleGroupRig builds a group of three alignments, the middle one disabled,
// bound to value and keyed "align".
func toggleGroupRig(value *Signal[string], changes *[]string) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(ToggleGroup(value,
			Option[string]{Value: "left", Label: "Left", Icon: icons.ChevronLeft},
			Option[string]{Value: "center", Label: "Center", Icon: icons.Minus, Disabled: true},
			Option[string]{Value: "right", Label: "Right", Icon: icons.ChevronRight},
		).Key("align").Label("Alignment").OnChange(func(v string) { *changes = append(*changes, v) }))
	})
}

func TestToggleGroupRovingFocusAndArrows(t *testing.T) {
	value := NewSignal("left")
	var changes []string
	r := toggleGroupRig(value, &changes)
	tr := r.tree()
	group := accesstest.Find(t, tr, access.RoleGroup, "Alignment")
	if !group.States.Has(access.StateSelectionContainer) {
		t.Fatalf("the group has states [%s], want a selection container", group.States)
	}
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleRadioButton, "Left"), access.RoleRadioButton, "Left",
		access.StateSelectable|access.StateSelected|access.StateFocusable)
	if c := accesstest.Find(t, tr, access.RoleRadioButton, "Center"); !c.States.Has(access.StateDisabled) {
		t.Fatalf("the disabled option has states [%s]", c.States)
	}
	if r.nodeByKey(t, "align/left").div.noTabStop || !r.nodeByKey(t, "align/right").div.noTabStop {
		t.Fatalf("the Tab stop is not the selected option")
	}
	r.focusKey(t, "align/left")
	r.press(input.KeyRight, 0)
	if value.Peek() != "right" || r.focusedKey() != "align/right" {
		t.Fatalf("Right chose %q with focus on %q, want right, past the disabled option", value.Peek(), r.focusedKey())
	}
	r.press(input.KeyRight, 0)
	if value.Peek() != "left" {
		t.Fatalf("Right past the last option chose %q, want it to wrap to left", value.Peek())
	}
	r.press(input.KeyUp, 0)
	if value.Peek() != "right" {
		t.Fatalf("Up from the first option chose %q, want right", value.Peek())
	}
	r.clickKey(t, "align/left")
	if value.Peek() != "left" {
		t.Fatalf("a click chose %q, want left", value.Peek())
	}
	r.press(input.KeyEnter, 0)
	if value.Peek() != "left" {
		t.Fatalf("Enter changed the value to %q", value.Peek())
	}
	r.bridge.Perform(accesstest.Find(t, r.tree(), access.RoleRadioButton, "Right").ID, access.ActionSelect)
	r.frame()
	if value.Peek() != "right" {
		t.Fatalf("Select on Right chose %q", value.Peek())
	}
	if want := []string{"right", "left", "right", "left", "right"}; len(changes) != len(want) {
		t.Fatalf("OnChange ran with %v, want %v", changes, want)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestToggleGroupIconOnlyAndJoinedCorners(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(ToggleGroup(NewSignal("a"),
			Option[string]{Value: "a", Label: "Alpha", Icon: icons.Check},
			Option[string]{Value: "b", Label: "Beta", Icon: icons.Minus},
			Option[string]{Value: "c", Label: "Gamma"},
		).Key("g").Label("Letters").IconOnly())
	})
	if n := len(partNodes(r.rt, "ToggleGroup.Label")); n != 1 {
		t.Fatalf("an icon-only group draws %d labels, want 1 for the option with no icon", n)
	}
	accesstest.Find(t, r.tree(), access.RoleRadioButton, "Beta") // the label stays the name
	first, mid, last := keyNode(t, r.rt, "g/a").style, keyNode(t, r.rt, "g/b").style, keyNode(t, r.rt, "g/c").style
	if first.Corners.TopLeft == 0 || first.Corners.TopRight != 0 || !mid.Corners.isZero() ||
		last.Corners.BottomRight == 0 || last.Corners.BottomLeft != 0 {
		t.Fatalf("corners: first %+v, middle %+v, last %+v", first.Corners, mid.Corners, last.Corners)
	}
	if first.BorderEdges.Left != 0 || mid.BorderEdges.Left != 1 || last.BorderEdges.Left != 1 {
		t.Fatalf("the lines between the items: %v %v %v", first.BorderEdges, mid.BorderEdges, last.BorderEdges)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestToggleGroupDisabledHasNoTabStop(t *testing.T) {
	value := NewSignal("a")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(ToggleGroup(value, Option[string]{Value: "a", Label: "Alpha"}, Option[string]{Value: "b", Label: "Beta"}).
			Key("g").Label("Letters").Disabled(true))
	})
	for _, name := range []string{"Alpha", "Beta"} {
		if n := r.stateOf(t, access.RoleRadioButton, name); !n.States.Has(access.StateDisabled) || n.States.Has(access.StateFocusable) {
			t.Fatalf("%s in a disabled group has states [%s]", name, n.States)
		}
	}
	rc := r.rt.nodeRect(keyNode(t, r.rt, "g/b"))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if value.Peek() != "a" {
		t.Fatalf("a click changed a disabled group to %q", value.Peek())
	}
}

// TestToggleGroupInAField checks that a Field names the group, gives it its focus
// handle, and checks its value with Required.
func TestToggleGroupInAField(t *testing.T) {
	value := NewSignal("")
	form := NewForm()
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(form.Field("align", "Alignment", ToggleGroup(value,
			Option[string]{Value: "left", Label: "Left"}, Option[string]{Value: "right", Label: "Right"}).Key("g").
			Validate(Required[string]())).Required())
	})
	g := r.stateOf(t, access.RoleGroup, "Alignment")
	if !g.States.Has(access.StateRequired) {
		t.Fatalf("the group in a required field has states [%s]", g.States)
	}
	form.Submit()
	r.frame()
	if g := r.stateOf(t, access.RoleGroup, "Alignment"); !g.States.Has(access.StateInvalid) {
		t.Fatalf("an empty required group is not invalid after submit: [%s]", g.States)
	}
	if r.focusedKey() != "g/left" {
		t.Fatalf("focus is on %q after a failed submit, want the group's first option", r.focusedKey())
	}
	r.press(input.KeySpace, 0)
	if value.Peek() != "left" {
		t.Fatalf("Space chose %q, want left", value.Peek())
	}
	if g := r.stateOf(t, access.RoleGroup, "Alignment"); g.States.Has(access.StateInvalid) {
		t.Fatalf("the group is still invalid with a value: [%s]", g.States)
	}
}

package ui

import (
	"cmp"
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// checkoutSteps are the steps of the tests' stepper.
var checkoutSteps = []Step{
	{Title: "Account", Description: "Sign in"},
	{Title: "Shipping", Description: "Where to send it"},
	{Title: "Payment", Icon: icons.Star},
	{Title: "Review"},
}

// orderedParts returns the nodes drawn for part, from the top left: by their place
// on the screen, top to bottom and then left to right.
func orderedParts(rt *Runtime, part string) []*node {
	out := partNodes(rt, part)
	slices.SortFunc(out, func(a, b *node) int {
		ra, rb := rt.nodeRect(a), rt.nodeRect(b)
		if ra.Y != rb.Y {
			return cmp.Compare(ra.Y, rb.Y)
		}
		return cmp.Compare(ra.X, rb.X)
	})
	return out
}

// The stepper is a list named by its Label, and each step a list item named by its
// position, title and state. The current step is selected. A stepper without
// OnSelect takes no focus.
func TestStepperAccessNamesAndStates(t *testing.T) {
	cur := NewSignal(1)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Stepper(cur, checkoutSteps...).Key("steps").Label("Checkout steps"))
	})
	tr := r.tree()
	list := accesstest.Find(t, tr, access.RoleList, "Checkout steps")
	if !list.States.Has(access.StateSelectionContainer) {
		t.Errorf("the list is not a selection container: %v", list.States)
	}
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleListItem, "Step 1 of 4: Account, completed"),
		access.RoleListItem, "Step 1 of 4: Account, completed", 0)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleListItem, "Step 2 of 4: Shipping, current"),
		access.RoleListItem, "Step 2 of 4: Shipping, current", access.StateSelected)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleListItem, "Step 3 of 4: Payment, not started"),
		access.RoleListItem, "Step 3 of 4: Payment, not started", 0)
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.focusable {
			t.Errorf("a display-only stepper has a focusable node %q", n.key)
		}
	}
	if len(r.warnings) != 0 {
		t.Errorf("accessibility warnings: %v", r.warnings)
	}
	// The default name follows the window's Strings.
	r2 := newAccessRig(func(ctx *Context) Element { return Stepper(cur, checkoutSteps...) })
	accesstest.Find(t, r2.tree(), access.RoleList, "Steps")
}

// The indicator of a complete step shows a check mark, the others their number or
// icon. A current step past the end marks every step complete.
func TestStepperIndicators(t *testing.T) {
	cur := NewSignal(2)
	r := newAccessRig(func(ctx *Context) Element { return Stepper(cur, checkoutSteps...).Key("steps") })
	inds := orderedParts(r.rt, "Stepper.Indicator")
	if len(inds) != 4 {
		t.Fatalf("%d indicators, want 4", len(inds))
	}
	content := func(n *node) string {
		c := n.children[0]
		switch c.kind {
		case kindText:
			return c.textEl.text
		case kindIcon:
			return "icon"
		}
		return "?"
	}
	got := []string{content(inds[0]), content(inds[1]), content(inds[2]), content(inds[3])}
	if want := []string{"icon", "icon", "icon", "4"}; !slices.Equal(got, want) {
		t.Errorf("indicators %v, want %v (two checks, the step's icon, a number)", got, want)
	}
	cur.Set(9)
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleListItem, "Step 4 of 4: Review, completed")
	accesstest.Absent(t, tr, access.RoleListItem, "Step 4 of 4: Review, current")
}

// With OnSelect the stepper is one Tab stop, at the current step. The arrow keys move
// the focus without choosing, Enter, Space and a click choose, and the signal is
// left to OnSelect.
func TestStepperKeyboardAndClicks(t *testing.T) {
	cur := NewSignal(1)
	var got []int
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Stepper(cur, checkoutSteps...).Key("steps").OnSelect(func(i int) { got = append(got, i) }))
	})
	stops := 0
	for i := range 4 {
		d := keyNode(t, r.rt, "steps/step:"+string(rune('0'+i))).div
		if !d.focusable {
			t.Fatalf("step %d is not focusable", i)
		}
		if !d.noTabStop {
			stops++
			if i != 1 {
				t.Errorf("the Tab stop is step %d, want the current step 1", i)
			}
		}
	}
	if stops != 1 {
		t.Errorf("%d Tab stops, want 1", stops)
	}
	focusNode(r, keyNode(t, r.rt, "steps/step:1"))
	r.press(input.KeyRight, 0)
	if k := r.focusedKey(); k != "steps/step:2" {
		t.Fatalf("Right moved the focus to %q, want step 2", k)
	}
	if len(got) != 0 || cur.Peek() != 1 {
		t.Fatalf("Right chose a step: OnSelect %v, current %d", got, cur.Peek())
	}
	r.press(input.KeyEnd, 0)
	r.press(input.KeyRight, 0) // no wrap
	if k := r.focusedKey(); k != "steps/step:3" {
		t.Errorf("End, Right: focus on %q, want the last step", k)
	}
	r.press(input.KeyEnter, 0)
	r.press(input.KeyHome, 0)
	r.press(input.KeySpace, 0)
	if k := r.focusedKey(); k != "steps/step:0" {
		t.Errorf("Home: focus on %q, want the first step", k)
	}
	r.press(input.KeyDown, 0) // a horizontal stepper takes Left and Right only
	if k := r.focusedKey(); k != "steps/step:0" {
		t.Errorf("Down moved the focus of a horizontal stepper to %q", k)
	}
	r.clickKey(t, "steps/step:2")
	if want := []int{3, 0, 2}; !slices.Equal(got, want) {
		t.Errorf("OnSelect %v, want %v", got, want)
	}
	if cur.Peek() != 1 {
		t.Errorf("the current step is %d; OnSelect sets it, not the stepper", cur.Peek())
	}
	tr := r.tree()
	n := accesstest.Find(t, tr, access.RoleListItem, "Step 3 of 4: Payment, not started")
	if !slices.Contains(n.Actions, access.ActionClick) || !n.States.Has(access.StateFocusable|access.StateSelectable) {
		t.Errorf("a choosable step: actions %v, states %v", n.Actions, n.States)
	}
	if len(r.warnings) != 0 {
		t.Errorf("accessibility warnings: %v", r.warnings)
	}
}

// A vertical stepper moves with Up and Down, and draws the line to the next step
// inside each step but the last.
func TestStepperVertical(t *testing.T) {
	cur := NewSignal(0)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Stepper(cur, checkoutSteps...).Key("steps").Vertical().OnSelect(func(int) {}))
	})
	if n := len(partNodes(r.rt, "Stepper.Connector")); n != 3 {
		t.Errorf("%d connectors, want 3", n)
	}
	if partNode(t, r.rt, "Stepper.Stepper", "").style.Direction != Column {
		t.Error("a vertical stepper is not a column")
	}
	focusNode(r, keyNode(t, r.rt, "steps/step:0"))
	r.press(input.KeyDown, 0)
	if k := r.focusedKey(); k != "steps/step:1" {
		t.Errorf("Down moved the focus to %q, want step 1", k)
	}
	r.press(input.KeyUp, 0)
	if k := r.focusedKey(); k != "steps/step:0" {
		t.Errorf("Up moved the focus to %q, want step 0", k)
	}
	if n := len(partNodes(r.rt, "Stepper.Connector")); n != 3 {
		t.Errorf("%d connectors after the keys, want 3", n)
	}
}

// A horizontal stepper narrower than its steps stays inside its width: the steps
// shrink and their texts end with an ellipsis, instead of running over what is
// beside the stepper.
func TestStepperFitsANarrowWidth(t *testing.T) {
	cur := NewSignal(1)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Row().
			Child(Div().Width(Px(320)).Child(Stepper(cur, checkoutSteps...).Key("steps"))).
			Child(Div().Width(Px(40)).Height(Px(20)).Key("beside"))
	})
	beside := r.rt.nodeRect(keyNode(t, r.rt, "beside"))
	for i, n := range orderedParts(r.rt, "Stepper.Step") {
		if rr := r.rt.nodeRect(n); rr.X+rr.W > beside.X+0.5 {
			t.Errorf("step %d ends at %.1f, past the element beside the stepper at %.1f", i, rr.X+rr.W, beside.X)
		}
	}
	for _, n := range partNodes(r.rt, "Stepper.Indicator") {
		if rr := r.rt.nodeRect(n); rr.W < 27.5 {
			t.Errorf("an indicator shrank to %.1f, want 28", rr.W)
		}
	}
}

// A disabled stepper with OnSelect shows its steps as disabled: they take no focus
// and no click.
func TestStepperDisabled(t *testing.T) {
	cur := NewSignal(1)
	chosen := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Stepper(cur, checkoutSteps...).Key("steps").OnSelect(func(int) { chosen++ }).Disabled(true))
	})
	n := keyNode(t, r.rt, "steps/step:2")
	if n.div.focusable {
		t.Error("a step of a disabled stepper is focusable")
	}
	if n.style.Opacity != 0.5 {
		t.Errorf("a disabled step has opacity %v, want 0.5", n.style.Opacity)
	}
	rc := r.rt.nodeRect(n)
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if chosen != 0 {
		t.Error("a click chose a step of a disabled stepper")
	}
	accesstest.Check(t, accesstest.Find(t, r.tree(), access.RoleListItem, "Step 3 of 4: Payment, not started"),
		access.RoleListItem, "Step 3 of 4: Payment, not started", access.StateDisabled)
}

// The state of a step changes its fills and borders, never its font weight.
func TestStepperStatesKeepTheFontWeight(t *testing.T) {
	cur := NewSignal(1)
	r := newAccessRig(func(ctx *Context) Element { return Stepper(cur, checkoutSteps...).Key("steps").OnSelect(func(int) {}) })
	focusNode(r, keyNode(t, r.rt, "steps/step:1"))
	hoverNode(r, keyNode(t, r.rt, "steps/step:2"))
	for _, part := range []string{"Stepper.Step", "Stepper.Indicator", "Stepper.Title"} {
		var weights []FontWeight
		for _, n := range partNodes(r.rt, part) {
			if !slices.Contains(weights, n.style.FontWeight) {
				weights = append(weights, n.style.FontWeight)
			}
		}
		if len(weights) != 1 {
			t.Errorf("%s: font weights %v across the states, want one", part, weights)
		}
	}
}

// Under high contrast a complete indicator is Highlight with HighlightText, the
// current one has a thicker Highlight border, and an app's colors are not used.
func TestStepperUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Stepper = StepperStyle{IndicatorComplete: Style{Background: markBG}, IndicatorCurrent: Style{BorderColor: markBG}}
	cur := NewSignal(1)
	r := newThemedRig(&theme, func(ctx *Context) Element { return Stepper(cur, checkoutSteps...).Key("steps") })
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	inds := orderedParts(r.rt, "Stepper.Indicator")
	if inds[0].style.Background != th.Accent || inds[0].style.TextColor != th.AccentText {
		t.Errorf("a complete indicator: %v on %v, want HighlightText on Highlight", inds[0].style.TextColor, inds[0].style.Background)
	}
	if inds[1].style.BorderColor != th.Accent || inds[1].style.BorderWidth <= inds[2].style.BorderWidth {
		t.Errorf("the current indicator: border %v %v, want a thicker Highlight border than %v",
			inds[1].style.BorderWidth, inds[1].style.BorderColor, inds[2].style.BorderWidth)
	}
	if inds[2].style.BorderColor != th.Text {
		t.Errorf("an upcoming indicator's border is %v, want the text color %v", inds[2].style.BorderColor, th.Text)
	}
}

func testStepperParts(t *testing.T, applied map[string]bool) {
	type stepCase struct {
		part     string
		set      func(*StepperStyle, Style)
		drawn    string
		key      string // the key of the step that holds the part, or "" for every node
		index    int    // the index among the drawn nodes, or -1 for every node
		vertical bool
		act      func(*testing.T, *accessRig)
	}
	cases := []stepCase{
		{part: "Stepper.Stepper", set: func(s *StepperStyle, m Style) { s.Stepper = m }, drawn: "Stepper.Stepper", index: -1},
		{part: "Stepper.Step", set: func(s *StepperStyle, m Style) { s.Step = m }, drawn: "Stepper.Step", index: -1},
		{part: "Stepper.StepHover", set: func(s *StepperStyle, m Style) { s.StepHover = m }, drawn: "Stepper.Step", key: "steps/step:2",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "steps/step:2")) }},
		{part: "Stepper.StepFocusVisible", set: func(s *StepperStyle, m Style) { s.StepFocusVisible = m }, drawn: "Stepper.Step", key: "steps/step:1",
			act: func(t *testing.T, r *accessRig) { focusNode(r, keyNode(t, r.rt, "steps/step:1")) }},
		{part: "Stepper.Indicator", set: func(s *StepperStyle, m Style) { s.Indicator = m }, drawn: "Stepper.Indicator", index: 2},
		{part: "Stepper.IndicatorComplete", set: func(s *StepperStyle, m Style) { s.IndicatorComplete = m }, drawn: "Stepper.Indicator", index: 0},
		{part: "Stepper.IndicatorCurrent", set: func(s *StepperStyle, m Style) { s.IndicatorCurrent = m }, drawn: "Stepper.Indicator", index: 1},
		{part: "Stepper.Title", set: func(s *StepperStyle, m Style) { s.Title = m }, drawn: "Stepper.Title", index: -1},
		{part: "Stepper.Description", set: func(s *StepperStyle, m Style) { s.Description = m }, drawn: "Stepper.Description", index: -1},
		{part: "Stepper.Connector", set: func(s *StepperStyle, m Style) { s.Connector = m }, drawn: "Stepper.Connector", index: 2},
		{part: "Stepper.Connector", set: func(s *StepperStyle, m Style) { s.Connector = m }, drawn: "Stepper.Connector", index: 2, vertical: true},
		{part: "Stepper.ConnectorComplete", set: func(s *StepperStyle, m Style) { s.ConnectorComplete = m }, drawn: "Stepper.Connector", index: 0},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st StepperStyle
			c.set(&st, Style{Background: markBG})
			cur := NewSignal(1)
			r := newAccessRig(func(ctx *Context) Element {
				s := Stepper(cur, checkoutSteps...).Key("steps").OnSelect(func(int) {}).Styles(st)
				if c.vertical {
					s.Vertical()
				}
				return Div().Column().Child(s)
			})
			if c.act != nil {
				c.act(t, r)
			}
			var found []*node
			switch {
			case c.key != "":
				found = []*node{partNode(t, r.rt, c.drawn, c.key)}
			case c.index >= 0:
				if all := orderedParts(r.rt, c.drawn); len(all) > c.index {
					found = []*node{all[c.index]}
				}
			default:
				found = partNodes(r.rt, c.drawn)
			}
			if len(found) == 0 {
				t.Fatalf("%s: no node drawn for it", c.part)
			}
			for _, n := range found {
				wantBackground(t, n, c.part, markBG)
			}
		})
	}
	applied["Stepper.StepDisabled"] = true
	t.Run("Stepper.StepDisabled", func(t *testing.T) {
		st := StepperStyle{StepDisabled: Style{Background: markBG}}
		r := newAccessRig(func(ctx *Context) Element {
			return Stepper(NewSignal(1), checkoutSteps...).Key("steps").OnSelect(func(int) {}).Disabled(true).Styles(st)
		})
		wantBackground(t, partNode(t, r.rt, "Stepper.Step", "steps/step:2"), "Stepper.StepDisabled", markBG)
	})
}

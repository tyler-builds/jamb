package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/layout"
)

// press sends a key press and its release, and lets the frame run.
func (r *accessRig) press(k input.Key, mods input.Modifiers) {
	r.rt.KeyDown(k, mods, false)
	r.rt.KeyUp(k, mods)
	r.frame()
}

// typeText types s, one character at a time.
func (r *accessRig) typeText(s string) {
	for _, c := range s {
		r.rt.TextInput(string(c))
	}
	r.frame()
}

// keyed returns the element with key that takes focus: the focusable element
// under the keyed one, which a control's key is on.
func (r *accessRig) keyed(t *testing.T, key string) *node {
	t.Helper()
	for _, n := range r.rt.byID {
		if n.key == key {
			if f := firstFocusable(n); f != nil {
				return f
			}
		}
	}
	t.Fatalf("no focusable element %q", key)
	return nil
}

// firstFocusable returns n or the first focusable element under it, or nil.
func firstFocusable(n *node) *node {
	if n.div != nil && n.div.focusable {
		return n
	}
	for _, c := range n.children {
		if f := firstFocusable(c); f != nil {
			return f
		}
	}
	return nil
}

// clickKey presses and releases the mouse on the center of the element with key.
func (r *accessRig) clickKey(t *testing.T, key string) {
	t.Helper()
	rc := r.rt.nodeRect(r.keyed(t, key))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
}

// clickAt presses and releases the left button at (x, y), in logical pixels.
func (r *accessRig) clickAt(x, y float32) {
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
}

// focusKey gives keyboard focus to the focusable element of the element with key.
func (r *accessRig) focusKey(t *testing.T, key string) {
	t.Helper()
	r.rt.setFocus(r.keyed(t, key).id)
	r.frame()
}

// focusedKey returns the key of the element with keyboard focus, or of its
// nearest keyed ancestor, or "".
func (r *accessRig) focusedKey() string {
	for n := r.rt.byID[r.rt.focusID]; n != nil; n = n.parent {
		if n.key != "" {
			return n.key
		}
	}
	return ""
}

// focusAccess gives keyboard focus to the element whose access node has the role and name.
func (r *accessRig) focusAccess(t *testing.T, role access.Role, name string) {
	t.Helper()
	n := accesstest.Find(t, r.tree(), role, name)
	for _, nd := range r.rt.byID {
		if accID(nd.id) == n.ID {
			r.rt.setFocus(nd.id)
			r.frame()
			return
		}
	}
	t.Fatalf("no element for %s %q", role, name)
}

// stateOf returns the access node of role and name, from the current tree.
func (r *accessRig) stateOf(t *testing.T, role access.Role, name string) *access.Node {
	t.Helper()
	return accesstest.Find(t, r.tree(), role, name)
}

func TestCheckboxClickAndSpaceToggle(t *testing.T) {
	on := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Checkbox("Email alerts", on).Key("email"))
	})
	n := r.stateOf(t, access.RoleCheckBox, "Email alerts")
	accesstest.Check(t, n, access.RoleCheckBox, "Email alerts", access.StateToggle|access.StateFocusable)
	if !n.Has(access.ActionToggle) {
		t.Fatalf("a check box has no toggle action: %v", n.Actions)
	}
	r.clickKey(t, "email")
	if !on.Peek() {
		t.Fatalf("a click did not turn the box on")
	}
	r.focusKey(t, "email")
	r.press(input.KeySpace, 0)
	if on.Peek() {
		t.Fatalf("Space did not turn the box off")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestCheckboxTriMixedStateAndClick(t *testing.T) {
	state := NewSignal(CheckMixed)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(CheckboxTri("All channels", state).Key("all"))
	})
	n := r.stateOf(t, access.RoleCheckBox, "All channels")
	if !n.States.Has(access.StateMixed) || n.States.Has(access.StateChecked) {
		t.Fatalf("a mixed box has states [%s]", n.States)
	}
	r.clickKey(t, "all")
	if state.Peek() != CheckOn {
		t.Fatalf("a click on a mixed box made it %v, want on", state.Peek())
	}
	accesstest.Check(t, r.stateOf(t, access.RoleCheckBox, "All channels"), access.RoleCheckBox, "All channels",
		access.StateToggle|access.StateChecked)
	r.clickKey(t, "all")
	if state.Peek() != CheckOff {
		t.Fatalf("a click on an on box made it %v, want off", state.Peek())
	}
}

func TestCheckboxDisabledDoesNotChange(t *testing.T) {
	on := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Checkbox("Locked", on).Key("locked").Disabled(true))
	})
	n := r.stateOf(t, access.RoleCheckBox, "Locked")
	if !n.States.Has(access.StateDisabled) || n.Has(access.ActionToggle) {
		t.Fatalf("a disabled box has states [%s] and actions %v", n.States, n.Actions)
	}
	rc, _ := r.rt.Find("locked")
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if on.Peek() {
		t.Fatalf("a disabled box changed")
	}
}

func TestRadioGroupRovingFocusAndArrows(t *testing.T) {
	value := NewSignal("daily")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(RadioGroup(value,
			Option[string]{Value: "daily", Label: "Daily"},
			Option[string]{Value: "weekly", Label: "Weekly", Disabled: true},
			Option[string]{Value: "monthly", Label: "Monthly"},
		).Key("freq").Label("Frequency"))
	})
	tr := r.tree()
	group := accesstest.Find(t, tr, access.RoleGroup, "Frequency")
	if !group.States.Has(access.StateSelectionContainer) {
		t.Fatalf("the group has states [%s], want a selection container", group.States)
	}
	daily := accesstest.Find(t, tr, access.RoleRadioButton, "Daily")
	accesstest.Check(t, daily, access.RoleRadioButton, "Daily", access.StateSelectable|access.StateSelected|access.StateFocusable)
	weekly := accesstest.Find(t, tr, access.RoleRadioButton, "Weekly")
	if !weekly.States.Has(access.StateDisabled) {
		t.Fatalf("the disabled option has states [%s]", weekly.States)
	}
	// Only the selected option is a Tab stop. The others are reached with the arrows.
	if r.nodeByKey(t, "freq/daily").div.noTabStop || !r.nodeByKey(t, "freq/monthly").div.noTabStop {
		t.Fatalf("the Tab stop is not the selected option")
	}
	r.focusKey(t, "freq/daily")
	r.press(input.KeyDown, 0)
	if value.Peek() != "monthly" || r.focusedKey() != "freq/monthly" {
		t.Fatalf("Down chose %q with focus on %q, want monthly", value.Peek(), r.focusedKey())
	}
	r.press(input.KeyDown, 0)
	if value.Peek() != "daily" {
		t.Fatalf("Down past the last option chose %q, want it to wrap to daily", value.Peek())
	}
	r.press(input.KeyUp, 0)
	if value.Peek() != "monthly" {
		t.Fatalf("Up from the first option chose %q, want monthly", value.Peek())
	}
	// The assistive technology selects an option through its SelectionItem.
	r.bridge.Perform(accesstest.Find(t, r.tree(), access.RoleRadioButton, "Daily").ID, access.ActionSelect)
	r.frame()
	if value.Peek() != "daily" {
		t.Fatalf("Select on Daily chose %q", value.Peek())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestSwitchTogglesAndIsAToggle(t *testing.T) {
	on := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Switch("Dark mode", on).Key("dark"))
	})
	accesstest.Check(t, r.stateOf(t, access.RoleSwitch, "Dark mode"), access.RoleSwitch, "Dark mode",
		access.StateToggle|access.StateFocusable)
	r.clickKey(t, "dark")
	if !on.Peek() {
		t.Fatalf("a click did not turn the switch on")
	}
	r.focusKey(t, "dark")
	r.press(input.KeySpace, 0)
	if on.Peek() {
		t.Fatalf("Space did not turn the switch off")
	}
}

// selectRig builds a Select of three colors, bound to value, keyed "channel".
func selectRig(t *testing.T, value *Signal[string]) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Select(value,
			Option[string]{Value: "red", Label: "Red"},
			Option[string]{Value: "green", Label: "Green"},
			Option[string]{Value: "blue", Label: "Blue", Disabled: true},
			Option[string]{Value: "teal", Label: "Teal"},
		).Key("channel").Label("Channel").Placeholder("Pick one"))
	})
}

func TestSelectOpensAsAListAndChoosesWithEnter(t *testing.T) {
	value := NewSignal("green")
	r := selectRig(t, value)
	field := r.stateOf(t, access.RoleComboBox, "Channel")
	if !field.States.Has(access.StateExpandable) || field.States.Has(access.StateExpanded) || field.Value != "Green" {
		t.Fatalf("closed field: states [%s], value %q", field.States, field.Value)
	}
	if !field.States.Has(access.StateReadOnly) || !field.Has(access.ActionExpand) {
		t.Fatalf("the field is not read-only with an expand action: states [%s], actions %v", field.States, field.Actions)
	}
	r.clickKey(t, "channel")
	tr := r.tree()
	list := accesstest.Find(t, tr, access.RoleList, "Channel")
	if !list.States.Has(access.StateSelectionContainer) {
		t.Fatalf("the list has states [%s]", list.States)
	}
	green := accesstest.Find(t, tr, access.RoleListItem, "Green")
	if !green.States.Has(access.StateSelectable) || !green.States.Has(access.StateSelected) {
		t.Fatalf("the chosen option has states [%s]", green.States)
	}
	if tr.Focus != green.ID {
		t.Fatalf("accessibility focus is on %d, want the highlighted option %d", tr.Focus, green.ID)
	}
	r.press(input.KeyDown, 0) // Blue is disabled: the highlight goes to Teal
	if tr := r.tree(); tr.Focus != accesstest.Find(t, tr, access.RoleListItem, "Teal").ID {
		t.Fatalf("Down did not move the highlight past the disabled option")
	}
	r.press(input.KeyEnter, 0)
	if value.Peek() != "teal" {
		t.Fatalf("Enter chose %q, want teal", value.Peek())
	}
	if !r.focusedAccess(t, access.RoleComboBox, "Channel") {
		t.Fatalf("focus is on %q after the list closed, want the field", r.focusedKey())
	}
	if f := r.stateOf(t, access.RoleComboBox, "Channel"); f.States.Has(access.StateExpanded) || f.Value != "Teal" {
		t.Fatalf("after Enter the field is expanded=%v with value %q", f.States.Has(access.StateExpanded), f.Value)
	}
}

func TestSelectEscapeClosesWithoutChange(t *testing.T) {
	value := NewSignal("red")
	r := selectRig(t, value)
	r.focusKey(t, "channel")
	r.press(input.KeyDown, input.ModAlt) // Alt+Down opens the list
	if r.stateOf(t, access.RoleList, "Channel") == nil {
		t.Fatalf("Alt+Down did not open the list")
	}
	r.press(input.KeyDown, 0)
	r.press(input.KeyEscape, 0)
	if value.Peek() != "red" {
		t.Fatalf("Escape changed the value to %q", value.Peek())
	}
	if !r.focusedAccess(t, access.RoleComboBox, "Channel") {
		t.Fatalf("focus is on %q after Escape, want the field", r.focusedKey())
	}
}

func TestSelectClosedKeysChooseAndTypeAhead(t *testing.T) {
	value := NewSignal("red")
	r := selectRig(t, value)
	r.focusKey(t, "channel")
	r.press(input.KeyDown, 0)
	if value.Peek() != "green" {
		t.Fatalf("Down with the field closed chose %q, want green", value.Peek())
	}
	r.press(input.KeyUp, 0)
	if value.Peek() != "red" {
		t.Fatalf("Up with the field closed chose %q, want red", value.Peek())
	}
	r.press(input.KeyEnd, 0)
	if value.Peek() != "teal" {
		t.Fatalf("End chose %q, want the last option teal", value.Peek())
	}
	r.press(input.KeyHome, 0)
	if value.Peek() != "red" {
		t.Fatalf("Home chose %q, want the first option red", value.Peek())
	}
	r.typeText("t")
	if value.Peek() != "teal" {
		t.Fatalf("typing t chose %q, want teal", value.Peek())
	}
	r.press(input.KeyHome, 0)
	r.now = r.now.Add(2 * time.Second) // a pause ends the type-ahead search
	r.typeText("g")
	if value.Peek() != "green" {
		t.Fatalf("typing g chose %q, want green", value.Peek())
	}
	if r.stateOf(t, access.RoleComboBox, "Channel").States.Has(access.StateExpanded) {
		t.Fatalf("typing opened the list")
	}
}

func TestSelectClickOutsideAndF4(t *testing.T) {
	value := NewSignal("red")
	r := selectRig(t, value)
	r.focusKey(t, "channel")
	r.press(input.KeyF4, 0)
	if r.stateOf(t, access.RoleList, "Channel") == nil {
		t.Fatalf("F4 did not open the list")
	}
	r.clickAt(480, 390) // outside the field and the list
	if value.Peek() != "red" {
		t.Fatalf("a press outside changed the value to %q", value.Peek())
	}
	if _, ok := r.rt.Find("jamb.listbox"); ok {
		t.Fatalf("a press outside did not close the list")
	}
}

func TestSelectWithoutALabelWarns(t *testing.T) {
	value := NewSignal("red")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Select(value, Option[string]{Value: "red", Label: "Red"}))
	})
	r.tree()
	if got := warningsContaining(r.warnings, "combo box has no accessible name"); len(got) != 1 {
		t.Fatalf("warnings = %v, want one for the combo box without a name", r.warnings)
	}
}

func TestSliderKeysSteps(t *testing.T) {
	v := NewSignal(40.0)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Slider(v, 0, 100, 5).Label("Volume").Key("vol"))
	})
	n := r.stateOf(t, access.RoleSlider, "Volume")
	if n.Range == nil || n.Range.Value != 40 || n.Range.Min != 0 || n.Range.Max != 100 ||
		n.Range.SmallChange != 5 || n.Range.LargeChange != 10 {
		t.Fatalf("slider range = %+v", n.Range)
	}
	r.focusKey(t, "vol")
	r.press(input.KeyRight, 0)
	if v.Peek() != 45 {
		t.Fatalf("Right chose %v, want 45", v.Peek())
	}
	r.press(input.KeyPageUp, 0)
	if v.Peek() != 55 {
		t.Fatalf("Page Up chose %v, want 55", v.Peek())
	}
	r.press(input.KeyLeft, 0)
	if v.Peek() != 50 {
		t.Fatalf("Left chose %v, want 50", v.Peek())
	}
	r.press(input.KeyPageDown, 0)
	if v.Peek() != 40 {
		t.Fatalf("Page Down chose %v, want 40", v.Peek())
	}
	r.press(input.KeyEnd, 0)
	if v.Peek() != 100 {
		t.Fatalf("End chose %v, want 100", v.Peek())
	}
	r.press(input.KeyHome, 0)
	if v.Peek() != 0 {
		t.Fatalf("Home chose %v, want 0", v.Peek())
	}
}

func TestSliderPointerAndRangeValueSet(t *testing.T) {
	v := NewSignal(0.0)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Slider(v, 0, 100, 5).Label("Volume").Key("vol").Width(Px(300)))
	})
	rc := r.rt.nodeRect(r.keyed(t, "vol"))
	// A press a quarter of the way along the thumb travel sets a quarter of the range.
	r.clickAt(rc.X+8+0.25*(rc.W-16), rc.Y+rc.H/2)
	if v.Peek() != 25 {
		t.Fatalf("a press at a quarter chose %v, want 25", v.Peek())
	}
	n := r.stateOf(t, access.RoleSlider, "Volume")
	if !n.Has(access.ActionSetRangeValue) {
		t.Fatalf("the slider has no set-range-value action: %v", n.Actions)
	}
	r.bridge.Do(n.ID, access.Request{Action: access.ActionSetRangeValue, X: 33})
	r.frame()
	if v.Peek() != 35 {
		t.Fatalf("SetValue(33) gave %v, want 35 (the nearest step)", v.Peek())
	}
	r.bridge.Do(n.ID, access.Request{Action: access.ActionSetRangeValue, X: 500})
	r.frame()
	if v.Peek() != 100 {
		t.Fatalf("SetValue(500) gave %v, want the maximum 100", v.Peek())
	}
}

func TestSliderDisabledIsNotFocusable(t *testing.T) {
	v := NewSignal(10.0)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Slider(v, 0, 100, 5).Label("Volume").Disabled(true))
	})
	n := r.stateOf(t, access.RoleSlider, "Volume")
	if !n.States.Has(access.StateDisabled) || n.States.Has(access.StateFocusable) || len(n.Actions) != 0 {
		t.Fatalf("a disabled slider has states [%s] and actions %v", n.States, n.Actions)
	}
}

// numberRig builds a NumberInput on the int or float signal, named "Quiet hours from".
func intRig(t *testing.T, value *Signal[int]) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(NumberInput(value, 0, 23, 1).Label("Quiet hours from").SpinButtons(SpinInline).Key("quiet"))
	})
}

func TestNumberInputStepsWithUpAndDown(t *testing.T) {
	value := NewSignal(9)
	r := intRig(t, value)
	n := r.stateOf(t, access.RoleSpinner, "Quiet hours from")
	if n.Range == nil || n.Range.Value != 9 || n.Range.Min != 0 || n.Range.Max != 23 || n.Range.SmallChange != 1 {
		t.Fatalf("spinner range = %+v", n.Range)
	}
	r.focusAccess(t, access.RoleTextField, "Quiet hours from")
	r.press(input.KeyUp, 0)
	if value.Peek() != 10 {
		t.Fatalf("Up gave %d, want 10", value.Peek())
	}
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0)
	if value.Peek() != 8 {
		t.Fatalf("Down twice gave %d, want 8", value.Peek())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestNumberInputClampsAndRejectsText(t *testing.T) {
	value := NewSignal(9)
	r := intRig(t, value)
	r.focusAccess(t, access.RoleTextField, "Quiet hours from")
	for range 3 {
		r.press(input.KeyBackspace, 0)
	}
	r.typeText("50")
	r.press(input.KeyEnter, 0)
	if value.Peek() != 23 {
		t.Fatalf("50 committed as %d, want it clamped to 23", value.Peek())
	}
	for range 2 {
		r.press(input.KeyBackspace, 0)
	}
	r.typeText("x")
	r.press(input.KeyEnter, 0)
	if value.Peek() != 23 {
		t.Fatalf("invalid text changed the value to %d", value.Peek())
	}
	if !r.stateOf(t, access.RoleSpinner, "Quiet hours from").States.Has(access.StateInvalid) {
		t.Fatalf("the field is not marked invalid after Enter on bad text")
	}
	// Leaving the field reverts the text to the value.
	r.rt.setFocus(0)
	r.frame()
	if r.stateOf(t, access.RoleSpinner, "Quiet hours from").States.Has(access.StateInvalid) {
		t.Fatalf("the field is still invalid after it was left")
	}
	if got := accesstest.Find(t, r.tree(), access.RoleTextField, "Quiet hours from").Value; got != "23" {
		t.Fatalf("the text after leaving is %q, want the value 23", got)
	}
}

func TestNumberInputButtonsAndFloats(t *testing.T) {
	value := NewSignal(0.5)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(NumberInput(value, 0.0, 2.0, 0.5).Label("Gain").SpinButtons(SpinInline).Key("gain"))
	})
	if got := accesstest.Find(t, r.tree(), access.RoleTextField, "Gain").Value; got != "0.5" {
		t.Fatalf("the text is %q, want 0.5 (one decimal, from the step)", got)
	}
	r.focusAccess(t, access.RoleButton, "Increase")
	r.press(input.KeyEnter, 0)
	if value.Peek() != 1.0 {
		t.Fatalf("Increase gave %v, want 1", value.Peek())
	}
	// A value set from outside the field replaces the text.
	value.Set(1.5)
	r.frame()
	if got := accesstest.Find(t, r.tree(), access.RoleTextField, "Gain").Value; got != "1.5" {
		t.Fatalf("the text after an outside change is %q, want 1.5", got)
	}
}

// focusedAccess reports whether the access tree's focus is on the node of role and name.
func (r *accessRig) focusedAccess(t *testing.T, role access.Role, name string) bool {
	t.Helper()
	tr := r.tree()
	return tr.Focus == accesstest.Find(t, tr, role, name).ID
}

// TestFormControlsInHighContrastAndAtLargeText builds one of each control under
// high contrast and at 225% text. Each must report no development warnings, keep
// its roles, and have no fixed height on the fields that hold text.
func TestFormControlsInHighContrastAndAtLargeText(t *testing.T) {
	settings := map[string]SystemSettings{
		"high contrast": {HighContrast: true, Colors: HighContrastWhite()},
		"225% text":     {TextScale: 225},
	}
	for name, sys := range settings {
		on, state, choice := NewSignal(false), NewSignal(CheckMixed), NewSignal("a")
		level, count, pick := NewSignal(30.0), NewSignal(4), NewSignal("red")
		r := newAccessRig(func(ctx *Context) Element {
			return Div().Column().Gap(12).
				Child(Checkbox("Plain", on)).
				Child(CheckboxTri("Mixed", state)).
				Child(Switch("Switch", on)).
				Child(RadioGroup(choice, Option[string]{Value: "a", Label: "Alpha"}, Option[string]{Value: "b", Label: "Beta"}).Label("Group")).
				Child(Select(pick, Option[string]{Value: "red", Label: "Red"}).Label("Pick").Placeholder("Choose")).
				Child(Slider(level, 0, 100, 5).Label("Level")).
				Child(NumberInput(count, 0, 9, 1).Label("Count").SpinButtons(SpinInline))
		})
		r.rt.SetSystemSettings(sys)
		tr := r.tree()
		if len(r.warnings) != 0 {
			t.Fatalf("%s: warnings %v", name, r.warnings)
		}
		for _, want := range []struct {
			role access.Role
			name string
		}{
			{access.RoleCheckBox, "Plain"}, {access.RoleCheckBox, "Mixed"}, {access.RoleSwitch, "Switch"},
			{access.RoleGroup, "Group"}, {access.RoleRadioButton, "Alpha"}, {access.RoleComboBox, "Pick"},
			{access.RoleSlider, "Level"}, {access.RoleSpinner, "Count"},
		} {
			accesstest.Find(t, tr, want.role, want.name)
		}
		combo := accessNode(t, r, access.RoleComboBox, "Pick")
		if combo.style.Height != layout.Auto || combo.style.MinHeight == layout.Auto {
			t.Fatalf("%s: the select has a fixed height %v, or no minimum height %v", name, combo.style.Height, combo.style.MinHeight)
		}
	}
}

// accessNode returns the built node of the access node with role and name.
func accessNode(t *testing.T, r *accessRig, role access.Role, name string) *node {
	t.Helper()
	n := accesstest.Find(t, r.tree(), role, name)
	for _, nd := range r.rt.byID {
		if accID(nd.id) == n.ID {
			return nd
		}
	}
	t.Fatalf("no node for %s %q", role, name)
	return nil
}

// TestFormControlsRequestNoFramesUnderReducedMotion opens and closes a select and
// toggles a switch. Under reduced motion none of them asks for animation frames;
// without it, the same actions do, so the test would notice a motion that ignores
// the setting.
func TestFormControlsRequestNoFramesUnderReducedMotion(t *testing.T) {
	for _, reduced := range []bool{true, false} {
		value, on := NewSignal("red"), NewSignal(false)
		r := newAccessRig(func(ctx *Context) Element {
			return Div().Column().Gap(12).
				Child(Select(value,
					Option[string]{Value: "red", Label: "Red"},
					Option[string]{Value: "green", Label: "Green"},
				).Label("Channel")).
				Child(Switch("Sync", on))
		})
		r.rt.SetSystemSettings(SystemSettings{ReducedMotion: reduced})
		r.frame()

		r.focusAccess(t, access.RoleComboBox, "Channel")
		r.press(input.KeyEnter, 0) // opens the list, which fades in
		opened := r.rt.NeedsFrame()
		r.press(input.KeyEscape, 0) // closes it, which fades out
		closed := r.rt.NeedsFrame()

		r.focusAccess(t, access.RoleSwitch, "Sync")
		r.press(input.KeySpace, 0) // the knob and the track ease over 120 ms
		toggled := r.rt.NeedsFrame()

		if reduced && (opened || closed || toggled) {
			t.Fatalf("under reduced motion: open asks for frames %v, close %v, switch %v", opened, closed, toggled)
		}
		if !reduced && !(opened && closed && toggled) {
			t.Fatalf("without reduced motion: open asks for frames %v, close %v, switch %v; each must", opened, closed, toggled)
		}
	}
}

// TestEnterOnCheckBoxAndSwitchGoesOnToTheDefault checks the Windows convention:
// Enter does not toggle a check box or a switch, or choose a radio button, so it
// reaches the modal's default button, and Space toggles or chooses them.
func TestEnterOnCheckBoxAndSwitchGoesOnToTheDefault(t *testing.T) {
	agreed, sync, open := NewSignal(false), NewSignal(false), NewSignal(true)
	size := NewSignal("small")
	oks := 0
	r := newAccessRig(func(ctx *Context) Element {
		page := Div().Column().Child(Text("page"))
		if !open.Get() {
			return page
		}
		ok := Button("OK").OnClick(func() { oks++ })
		body := Div().Column().Gap(8).
			Child(Checkbox("I agree", agreed)).
			Child(Switch("Sync now", sync)).
			Child(RadioGroup(size, Option[string]{Value: "small", Label: "Small"}, Option[string]{Value: "large", Label: "Large"}).Label("Size"))
		return page.Child(Modal("Confirm", body).Footer(ok).Default(ok).OnClose(func() { open.Set(false) }))
	})
	r.focusAccess(t, access.RoleCheckBox, "I agree")
	r.press(input.KeyEnter, 0)
	if agreed.Peek() || oks != 1 {
		t.Fatalf("Enter on the check box: agreed=%v, OK ran %d times; want false and 1", agreed.Peek(), oks)
	}
	r.focusAccess(t, access.RoleSwitch, "Sync now")
	r.press(input.KeyEnter, 0)
	if sync.Peek() || oks != 2 {
		t.Fatalf("Enter on the switch: sync=%v, OK ran %d times; want false and 2", sync.Peek(), oks)
	}
	r.focusAccess(t, access.RoleRadioButton, "Large")
	r.press(input.KeyEnter, 0)
	if size.Peek() != "small" || oks != 3 {
		t.Fatalf("Enter on a radio button: size=%q, OK ran %d times; want \"small\" and 3", size.Peek(), oks)
	}
	r.press(input.KeySpace, 0)
	if size.Peek() != "large" || oks != 3 {
		t.Fatalf("Space on a radio button: size=%q, OK ran %d times; want \"large\" and 3", size.Peek(), oks)
	}
	r.focusAccess(t, access.RoleCheckBox, "I agree")
	r.press(input.KeySpace, 0)
	if !agreed.Peek() || oks != 3 {
		t.Fatalf("Space on the check box: agreed=%v, OK ran %d times; want true and 3", agreed.Peek(), oks)
	}
}

// TestSelectOnChangeRunsForChoicesOnly checks that OnChange runs with the chosen
// value when the user chooses an option, and not when the list is closed without
// a change.
func TestSelectOnChangeRunsForChoicesOnly(t *testing.T) {
	value := NewSignal("red")
	var changes []string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Select(value,
			Option[string]{Value: "red", Label: "Red"},
			Option[string]{Value: "green", Label: "Green"},
			Option[string]{Value: "blue", Label: "Blue"},
		).Label("Channel").OnChange(func(v string) { changes = append(changes, v) }))
	})
	r.focusAccess(t, access.RoleComboBox, "Channel")
	r.press(input.KeyDown, 0)  // closed: chooses green
	r.press(input.KeyEnter, 0) // opens the list
	r.press(input.KeyDown, 0)  // highlights blue
	r.press(input.KeyEscape, 0)
	if fmtList(changes) != "green" {
		t.Fatalf("OnChange ran with %v, want [green] (Escape changes nothing)", changes)
	}
	r.press(input.KeyEnter, 0)
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnter, 0) // chooses blue from the list
	if fmtList(changes) != "green,blue" {
		t.Fatalf("OnChange ran with %v, want [green blue]", changes)
	}
}

// TestRadioGroupOnChangeRunsForSelection checks OnChange on the arrow keys and on
// the selection of an option through the access tree.
func TestRadioGroupOnChangeRunsForSelection(t *testing.T) {
	value := NewSignal("a")
	var changes []string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(RadioGroup(value,
			Option[string]{Value: "a", Label: "Alpha"},
			Option[string]{Value: "b", Label: "Beta"},
		).Label("Letters").OnChange(func(v string) { changes = append(changes, v) }))
	})
	r.focusAccess(t, access.RoleRadioButton, "Alpha")
	r.press(input.KeyDown, 0)
	r.bridge.Perform(accesstest.Find(t, r.tree(), access.RoleRadioButton, "Alpha").ID, access.ActionSelect)
	r.frame()
	if fmtList(changes) != "b,a" {
		t.Fatalf("OnChange ran with %v, want [b a]", changes)
	}
}

// fmtList joins the values for a comparison.
func fmtList(vs []string) string { return strings.Join(vs, ",") }

package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// dialogFormRig is the New Project form: a summary, a required name without a key
// (as the demo's dialog has it), and a create button.
func dialogFormRig(name *Signal[string]) *accessRig {
	form := NewForm().OnSubmit(func() error { return nil })
	return newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(12).
			Child(form.Summary()).
			Child(form.Field("name", "Name", TextInput(name).Placeholder("Project name").
				Validate(Required[string](), MaxLength(80))).Required()).
			Child(Button("Create project").Key("create").OnClick(form.Submit))
	})
}

// TestTypingAfterAFailedSubmitKeepsFocusInTheField checks that a field keeps its
// focus when its error goes away: after a failed submit, the first typed character
// clears the required error, and the field must keep focus and take the rest of the
// text. The field's text input has no key, as in the demo's dialog.
func TestTypingAfterAFailedSubmitKeepsFocusInTheField(t *testing.T) {
	name := NewSignal("")
	r := dialogFormRig(name)
	r.clickKey(t, "create")
	if !r.stateOf(t, access.RoleTextField, "Name").States.Has(access.StateFocused) {
		t.Fatal("focus is not on Name after a failed submit")
	}
	r.typeText("L")
	if got := name.Peek(); got != "L" {
		t.Fatalf("name = %q after one character, want L", got)
	}
	if r.stateOf(t, access.RoleTextField, "Name").States.Has(access.StateInvalid) {
		t.Fatal("Name is still invalid with a value in the field")
	}
	if !r.stateOf(t, access.RoleTextField, "Name").States.Has(access.StateFocused) {
		t.Fatal("Name lost focus when its error cleared")
	}
	r.typeText("ab")
	if got := name.Peek(); got != "Lab" {
		t.Fatalf("name = %q after typing on, want Lab", got)
	}
	if !r.stateOf(t, access.RoleTextField, "Name").States.Has(access.StateFocused) {
		t.Error("Name lost focus while the user typed")
	}
}

// TestFieldMessageComingAndGoingKeepsTheControl checks that a control keeps its node
// when the messages of its field come and go: the help text and an app error are
// added and removed around it, and the editor keeps its focus through each change.
func TestFieldMessageComingAndGoingKeepsTheControl(t *testing.T) {
	text := NewSignal("")
	appErr := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(8).
			Child(Field("Notes", TextInput(text)).Description("Help.").Error(appErr.Get())).
			Child(Button("Other").Key("other"))
	})
	r.focusAccess(t, access.RoleTextField, "Notes")
	id := r.rt.focusID
	appErr.Set("Too short.")
	r.frame()
	r.typeText("a")
	if r.rt.focusID != id {
		t.Errorf("the control was rebuilt when its error appeared: focus %d, was %d", r.rt.focusID, id)
	}
	appErr.Set("")
	r.frame()
	r.typeText("b")
	if r.rt.focusID != id {
		t.Errorf("the control was rebuilt when its error went away: focus %d, was %d", r.rt.focusID, id)
	}
	if text.Peek() != "ab" {
		t.Errorf("text = %q, want ab", text.Peek())
	}
}

// TestNumberInputMessageComingAndGoingKeepsFocus checks that a number input keeps
// its focus when its own message appears and goes away: an invalid entry shows the
// range, and a valid one clears it, with the field focused through both.
func TestNumberInputMessageComingAndGoingKeepsFocus(t *testing.T) {
	v := NewSignal(8)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(NumberInput(v, 0, 23, 1).Label("Hours").Key("hours"))
	})
	focusNode(r, editorNode(t, r.rt))
	r.typeText("x")
	r.press(input.KeyEnter, 0)
	if !hasName(r, "Enter a number from 0 to 23.") {
		t.Fatal("the range message does not show for an invalid entry")
	}
	if r.rt.focusID != editorNode(t, r.rt).id {
		t.Fatal("the field lost focus when its message appeared")
	}
	// Select the draft and type over it.
	r.press(input.KeyA, input.ModCtrl)
	r.typeText("12")
	r.press(input.KeyEnter, 0)
	if v.Peek() != 12 {
		t.Fatalf("value = %d, want 12", v.Peek())
	}
	if hasName(r, "Enter a number from 0 to 23.") {
		t.Error("the range message still shows for a valid entry")
	}
	if r.rt.focusID != editorNode(t, r.rt).id {
		t.Error("the field lost focus when its message went away")
	}
}

// TestTextAreaAndSelectKeepFocusWhenTheirErrorClears checks the other controls of a
// field: a text area and a select keep focus when the change that clears their
// required error is made with it.
func TestTextAreaAndSelectKeepFocusWhenTheirErrorClears(t *testing.T) {
	desc, pick := NewSignal(""), NewSignal("")
	form := NewForm().OnSubmit(func() error { return nil })
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(12).
			Child(form.Summary()).
			Child(form.Field("desc", "Description", TextArea(desc).Validate(Required[string]())).Required()).
			Child(form.Field("pick", "Channel", Select(pick, Option[string]{Value: "red", Label: "Red"},
				Option[string]{Value: "green", Label: "Green"}).Validate(Required[string]())).Required()).
			Child(Button("Create").Key("create").OnClick(form.Submit))
	})
	r.clickKey(t, "create")
	if r.rt.focusID != editorNode(t, r.rt).id {
		t.Fatal("focus is not on the description after a failed submit")
	}
	r.typeText("a")
	if desc.Peek() != "a" || r.rt.focusID != editorNode(t, r.rt).id {
		t.Fatalf("the text area lost focus when its error cleared (value %q)", desc.Peek())
	}
	// Focus moves to the select, the field after the text area. Down chooses an option.
	trigger := partNode(t, r.rt, "Select.Trigger", "")
	focusNode(r, trigger)
	r.press(input.KeyDown, 0)
	if pick.Peek() == "" {
		t.Fatal("Down did not choose an option")
	}
	if r.rt.focusID != partNode(t, r.rt, "Select.Trigger", "").id {
		t.Error("the select lost focus when its error cleared")
	}
}

package ui

import (
	"errors"
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// groupRig is a form with a group of two check boxes, checked by a Check of the
// field, and a Save button. The first check box takes the field's focus.
func groupRig(email, text *Signal[bool], form *Form, h *FocusHandle) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		group := Div().Column().
			Child(Checkbox("Email", email).Key("email").FocusHandle(h)).
			Child(Checkbox("Text message", text).Key("text"))
		return Div().Column().Child(form.Summary()).
			Child(form.Field("channels", "Send alerts by", group).
				Check(func() error {
					if !email.Get() && !text.Get() {
						return errors.New("Choose at least one channel.")
					}
					return nil
				}).
				Description("Choose how alerts reach you.").Required().Focus(h)).
			Child(Button("Save").Key("save").OnClick(form.Submit))
	})
}

func TestGroupFieldIsNamedDescribedAndChecked(t *testing.T) {
	email, text := NewSignal(false), NewSignal(false)
	form := NewForm().OnSubmit(func() error { return nil })
	r := groupRig(email, text, form, NewFocusHandle())
	g := r.stateOf(t, access.RoleGroup, "Send alerts by")
	if !g.States.Has(access.StateRequired) {
		t.Error("the group is not required")
	}
	if g.Description != "Choose how alerts reach you." {
		t.Errorf("Description = %q, want the help text", g.Description)
	}
	if hasName(r, "Choose at least one channel.") {
		t.Fatal("the group shows an error before the form is submitted")
	}
	r.clickKey(t, "save")
	if !hasName(r, "Choose at least one channel.") {
		t.Fatal("no error for a group with no channel checked")
	}
	if !r.stateOf(t, access.RoleGroup, "Send alerts by").States.Has(access.StateInvalid) {
		t.Error("the group does not report StateInvalid")
	}
	r.clickKey(t, "email")
	if hasName(r, "Choose at least one channel.") {
		t.Error("the error stays after a channel is checked")
	}
}

func TestGroupFieldFocusesTheHandleItWasGiven(t *testing.T) {
	email, text := NewSignal(false), NewSignal(false)
	h := NewFocusHandle()
	form := NewForm().OnSubmit(func() error { return nil })
	r := groupRig(email, text, form, h)
	r.clickKey(t, "save")
	if got := r.focusedKey(); got != "email" {
		t.Fatalf("focus = %q, want the first check box of the group", got)
	}
}

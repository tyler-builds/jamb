package ui

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// hasName reports whether the access tree has a node named s.
func hasName(r *accessRig, s string) bool {
	found := false
	r.tree().Walk(func(n *access.Node) bool {
		if n.Name == s {
			found = true
		}
		return true
	})
	return found
}

// listenAnnouncements records the announcements the access bridge delivers for r.
// The returned function publishes the queue, as the next access update does, and
// returns the texts delivered since its last call. Each announcement must be
// assertive, which the forms announce errors with.
func listenAnnouncements(r *accessRig) func(t *testing.T) []string {
	var got []access.Announcement
	r.bridge.Listen(func(u access.Update) { got = append(got, u.Announcements...) })
	return func(t *testing.T) []string {
		t.Helper()
		r.rt.markAccess()
		r.frame()
		var out []string
		for _, a := range got {
			out = append(out, a.Text)
			if a.Politeness != access.Assertive {
				t.Errorf("announcement %q is not assertive", a.Text)
			}
		}
		got = nil
		return out
	}
}

func TestValidatorsReportTheirMessages(t *testing.T) {
	digits := regexp.MustCompile(`^\d+$`)
	cases := []struct {
		name string
		err  error
		want string // "" when the value is valid
	}{
		{"required text, blank", Required[string]()("  "), msgRequired},
		{"required text", Required[string]()("a"), ""},
		{"required check box, off", Required[bool]()(false), msgRequired},
		{"required check box, on", Required[bool]()(true), ""},
		{"required number, zero", Required[int]()(0), msgRequired},
		{"required number", Required[int]()(3), ""},
		{"min length, empty passes", MinLength(3)(""), ""},
		{"min length, short", MinLength(3)("ab"), "Enter at least 3 characters."},
		{"min length counts runes", MinLength(3)("日本"), "Enter at least 3 characters."},
		{"max length counts runes", MaxLength(2)("日本"), ""},
		{"max length, long", MaxLength(2)("日本語"), "Enter no more than 2 characters."},
		{"pattern, empty passes", Pattern(digits, "Digits only.")(""), ""},
		{"pattern, no match", Pattern(digits, "Digits only.")("12a"), "Digits only."},
		{"range, below", Range(0, 23)(-1), "Enter a value from 0 to 23."},
		{"range, bound", Range(0, 23)(23), ""},
		{"range, float", Range(0.5, 2.0)(2.5), "Enter a value from 0.5 to 2."},
		{"custom", Custom(func(s string) error {
			if s != "x" {
				return errors.New("Enter x.")
			}
			return nil
		})("y"), "Enter x."},
	}
	for _, c := range cases {
		got := ""
		if c.err != nil {
			got = c.err.Error()
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestFirstFailingValidatorWins(t *testing.T) {
	rules := []Validator[string]{MinLength(5), Required[string]()}
	// The empty text passes MinLength, so Required is the one that reports it.
	if err := firstError("", rules); err == nil || err.Error() != msgRequired {
		t.Fatalf("firstError(\"\") = %v, want the required message", err)
	}
	if err := firstError("ab", rules); err == nil || err.Error() != "Enter at least 5 characters." {
		t.Fatalf("firstError(\"ab\") = %v, want the length message", err)
	}
}

func TestFieldWiresItsControl(t *testing.T) {
	name := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Field("Name", TextInput(name).Key("name").Validate(Required[string]())).
			Description("Shown in the project list.").Required()
	})
	f := r.stateOf(t, access.RoleTextField, "Name")
	if !f.States.Has(access.StateRequired) {
		t.Errorf("the field is not required: %v", f.States)
	}
	if f.States.Has(access.StateInvalid) {
		t.Errorf("the field is invalid before the user has touched it: %v", f.States)
	}
	if f.Description != "Shown in the project list." {
		t.Errorf("Description = %q, want the help text", f.Description)
	}
	if len(f.LabelledBy) != 1 || r.tree().Node(f.LabelledBy[0]).Name != "Name" {
		t.Errorf("LabelledBy = %v, want the label text", f.LabelledBy)
	}
	if len(f.DescribedBy) != 1 || r.tree().Node(f.DescribedBy[0]).Name != "Shown in the project list." {
		t.Errorf("DescribedBy = %v, want the help text", f.DescribedBy)
	}
	if n := len(r.warnings); n != 0 {
		t.Errorf("warnings: %v", r.warnings)
	}
}

func TestFieldAddsTheRequiredMarkerOutsideTheAccessTree(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Field("Name", TextInput(NewSignal("")).Key("name")).Required()
	})
	if hasName(r, "*") {
		t.Fatal("the asterisk is exposed to assistive technology")
	}
	if !r.stateOf(t, access.RoleTextField, "Name").States.Has(access.StateRequired) {
		t.Fatal("a required field does not report StateRequired")
	}
}

func TestFieldShowsItsErrorOnlyAfterTheUserEditsAndLeaves(t *testing.T) {
	name := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(8).
			Child(Field("Name", TextInput(name).Key("name").Validate(Required[string]()))).
			Child(Button("Other").Key("other"))
	})
	if hasName(r, msgRequired) {
		t.Fatal("an error shows before the user has touched the field")
	}
	r.focusKey(t, "name")
	r.typeText("a")
	r.press(input.KeyBackspace, 0)
	if hasName(r, msgRequired) {
		t.Fatal("an error shows while the field still has focus")
	}
	r.focusKey(t, "other")
	if !hasName(r, msgRequired) {
		t.Fatal("no error after the user edited the field and left it")
	}
	if !r.stateOf(t, access.RoleTextField, "Name").States.Has(access.StateInvalid) {
		t.Fatal("the shown error does not set StateInvalid")
	}
}

func TestFieldDoesNotShowAnErrorForAFieldTheUserOnlyVisited(t *testing.T) {
	name := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(8).
			Child(Field("Name", TextInput(name).Key("name").Validate(Required[string]()))).
			Child(Button("Other").Key("other"))
	})
	r.focusKey(t, "name")
	r.focusKey(t, "other")
	if hasName(r, msgRequired) {
		t.Fatal("an error shows for a field that was only visited")
	}
}

func TestFieldErrorFromTheAppIsShownAndDescribed(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Field("Name", TextInput(NewSignal("Lab")).Key("name")).Error("That name is in use.")
	})
	f := r.stateOf(t, access.RoleTextField, "Name")
	if !f.States.Has(access.StateInvalid) {
		t.Fatal("an app error does not set StateInvalid")
	}
	if f.Description != "That name is in use." {
		t.Fatalf("Description = %q, want the error", f.Description)
	}
	if !hasName(r, "That name is in use.") {
		t.Fatal("the error is not shown")
	}
}

// formRig is a form with a name (required, by default) and a description with
// help text, a create button and the summary above the fields.
func formRig(t *testing.T, form *Form, name, desc *Signal[string]) *accessRig {
	t.Helper()
	return newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(8).
			Child(form.Summary()).
			Child(form.Field("name", "Name", TextInput(name).Key("name").Validate(Required[string]())).Required()).
			Child(form.Field("desc", "Description", TextArea(desc).Key("desc").Validate(Required[string]())).
				Description("What the project is for.")).
			Child(Button("Create").Key("create").OnClick(form.Submit))
	})
}

func TestSubmitFocusesTheFirstInvalidFieldAndAnnouncesTheCount(t *testing.T) {
	name, desc := NewSignal(""), NewSignal("")
	saved := 0
	form := NewForm().OnSubmit(func() error { saved++; return nil })
	r := formRig(t, form, name, desc)
	said := listenAnnouncements(r)
	said(t)
	r.clickKey(t, "create")
	if saved != 0 {
		t.Fatal("the form was submitted while it had errors")
	}
	if got := r.focusedKey(); got != "name" {
		t.Fatalf("focus = %q, want the first invalid field", got)
	}
	if got := said(t); len(got) != 1 || got[0] != "2 fields need attention." {
		t.Fatalf("announcements = %q, want the count", got)
	}
	if !hasName(r, "2 fields need attention.") {
		t.Fatal("the summary does not show the count")
	}
	if !hasName(r, "Name: "+msgRequired) || !hasName(r, "Description: "+msgRequired) {
		t.Fatal("the summary does not list each error")
	}
}

func TestSummaryLinkMovesFocusToItsField(t *testing.T) {
	name, desc := NewSignal("Lab"), NewSignal("")
	form := NewForm().OnSubmit(func() error { return nil })
	r := formRig(t, form, name, desc)
	r.clickKey(t, "create")
	if got := r.focusedKey(); got != "desc" {
		t.Fatalf("focus = %q, want the description, the only invalid field", got)
	}
	// Focus starts on the button, and a link moves it to the field.
	r.clickKey(t, "create")
	r.clickKey(t, fmt.Sprintf("form-%d/item:desc", form.id))
	if got := r.focusedKey(); got != "desc" {
		t.Fatalf("after the link, focus = %q, want the description", got)
	}
}

func TestSubmitRunsTheFormWhenItIsValid(t *testing.T) {
	name, desc := NewSignal("Lab"), NewSignal("Sample")
	saved := 0
	form := NewForm().OnSubmit(func() error { saved++; return nil })
	r := formRig(t, form, name, desc)
	r.clickKey(t, "create")
	if saved != 1 {
		t.Fatalf("saved %d times, want once", saved)
	}
	if hasName(r, msgRequired) || !form.submitted.Peek() {
		t.Fatal("a valid submit should record that the form was submitted and show no error")
	}
}

func TestSubmitErrorIsTheFormMessage(t *testing.T) {
	name, desc := NewSignal("Taken"), NewSignal("Sample")
	form := NewForm().OnSubmit(func() error { return errors.New("That name is already in use.") })
	r := formRig(t, form, name, desc)
	said := listenAnnouncements(r)
	said(t)
	r.clickKey(t, "create")
	if !hasName(r, "That name is already in use.") {
		t.Fatal("the form's message is not shown in the summary")
	}
	if got := said(t); len(got) != 1 || got[0] != "That name is already in use." {
		t.Fatalf("announcements = %q, want the message", got)
	}
}

func TestEnterInATextFieldSubmitsTheForm(t *testing.T) {
	name, desc := NewSignal("Lab"), NewSignal("Sample")
	saved := 0
	form := NewForm().OnSubmit(func() error { saved++; return nil })
	r := formRig(t, form, name, desc)
	r.focusKey(t, "name")
	r.press(input.KeyEnter, 0)
	if saved != 1 {
		t.Fatalf("Enter in a field saved %d times, want once", saved)
	}
}

func TestEnterOnACheckBoxRunsTheModalDefaultNotTheForm(t *testing.T) {
	accept := NewSignal(false)
	saved, created := 0, 0
	form := NewForm().OnSubmit(func() error { saved++; return nil })
	create := Button("Create").Key("create").OnClick(func() { created++ })
	r := newAccessRig(func(ctx *Context) Element {
		body := Div().Column().Child(form.Field("accept", "Terms", Checkbox("I accept", accept).Key("accept").
			Validate(Required[bool]())))
		return Modal("New", body).Footer(create).Default(create)
	})
	r.focusKey(t, "accept")
	r.press(input.KeyEnter, 0)
	if created != 1 {
		t.Fatalf("Enter on a check box ran the modal default %d times, want once", created)
	}
	if saved != 0 {
		t.Fatalf("Enter on a check box submitted the form %d times, want none", saved)
	}
}

func TestAsyncSubmitDisablesTheButtonAndReportsTheResult(t *testing.T) {
	name := NewSignal("Lab")
	done := make(chan error, 1)
	form := NewForm().OnSubmitAsync(func() <-chan error { return done })
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(form.Summary()).
			Child(form.Field("name", "Name", TextInput(name).Key("name").Validate(Required[string]()))).
			Child(Button("Save").Key("save").Disabled(form.Submitting()).OnClick(form.Submit))
	})
	said := listenAnnouncements(r)
	r.clickKey(t, "save")
	if !form.Submitting() {
		t.Fatal("the form is not submitting after Submit")
	}
	if !r.stateOf(t, access.RoleButton, "Save").States.Has(access.StateDisabled) {
		t.Fatal("the submit button is not disabled while the submit runs")
	}
	form.Submit() // a second submit does nothing while the first runs
	done <- errors.New("Server unavailable.")
	deadline := time.Now().Add(5 * time.Second)
	for form.Submitting() && time.Now().Before(deadline) {
		r.frame()
		time.Sleep(2 * time.Millisecond)
	}
	r.frame()
	if form.Submitting() {
		t.Fatal("the submit did not finish")
	}
	if !hasName(r, "Server unavailable.") {
		t.Fatal("the result of the submit is not shown")
	}
	if got := said(t); len(got) == 0 || got[len(got)-1] != "Server unavailable." {
		t.Fatalf("announcements = %q, want the result", got)
	}
}

func TestResetClearsTheSubmittedState(t *testing.T) {
	name, desc := NewSignal(""), NewSignal("")
	form := NewForm().OnSubmit(func() error { return nil })
	r := formRig(t, form, name, desc)
	r.clickKey(t, "create")
	if !hasName(r, msgRequired) {
		t.Fatal("no error after a failed submit")
	}
	form.Reset()
	r.frame()
	if hasName(r, msgRequired) || hasName(r, "2 fields need attention.") {
		t.Fatal("errors are still shown after Reset")
	}
}

func TestControlsCheckTheirValues(t *testing.T) {
	channel := NewSignal("")
	pick := NewSignal("")
	accept := NewSignal(false)
	form := NewForm().OnSubmit(func() error { return nil })
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(form.Summary()).
			Child(form.Field("channel", "Channel", Select(channel, Option[string]{Value: "email", Label: "Email"}).Key("channel").
				Label("Channel").Validate(Required[string]()))).
			Child(form.Field("pick", "Frequency", RadioGroup(pick, Option[string]{Value: "daily", Label: "Daily"}).Key("pick").
				Validate(Required[string]()))).
			Child(form.Field("accept", "Terms", Checkbox("I accept", accept).Key("accept").Validate(Required[bool]()))).
			Child(Button("Save").Key("save").OnClick(form.Submit))
	})
	said := listenAnnouncements(r)
	said(t)
	r.clickKey(t, "save")
	if got := said(t); len(got) != 1 || got[0] != "3 fields need attention." {
		t.Fatalf("announcements = %q, want three fields", got)
	}
	if got := r.focusedKey(); got != "channel" {
		t.Fatalf("focus = %q, want the select, the first field", got)
	}
	if !r.stateOf(t, access.RoleComboBox, "Channel").States.Has(access.StateInvalid) {
		t.Fatal("the select does not report StateInvalid")
	}
}

func TestEveryControlReportsItsRequiredState(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Field("Name", TextInput(NewSignal("")).Key("name")).Required()).
			Child(Field("Channel", Select(NewSignal(""), Option[string]{Value: "email", Label: "Email"}).Key("channel")).Required())
	})
	if !r.stateOf(t, access.RoleTextField, "Name").States.Has(access.StateRequired) {
		t.Error("the text field is not required")
	}
	if !r.stateOf(t, access.RoleComboBox, "Channel").States.Has(access.StateRequired) {
		t.Error("the select is not required")
	}
}

func TestMessagesLookAlikeInANumberInputAndAField(t *testing.T) {
	if !reflect.DeepEqual(messageStyle(DarkTheme()), numberInputDefaults(DarkTheme(), SpinInline, 1, false, false).Message) {
		t.Fatal("a number input's message and a field's error have different looks")
	}
	if !reflect.DeepEqual(messageStyle(DarkTheme()), formFieldDefaults(DarkTheme()).Error) {
		t.Fatal("a field's error is not the shared message look")
	}
}

func TestFormPartsResolveInOrder(t *testing.T) {
	th := DarkTheme().Clone()
	th.FormField = FormFieldStyle{Label: Style{TextColor: Hex("#111111")}}
	if got := resolveFormField(th, FormFieldStyle{}).Label.TextColor; got != Hex("#111111") {
		t.Fatalf("theme label color = %v, want the theme's", got)
	}
	if got := resolveFormField(th, FormFieldStyle{Label: Style{TextColor: Hex("#222222")}}).Label.TextColor; got != Hex("#222222") {
		t.Fatalf("instance label color = %v, want the instance's", got)
	}
	if got := resolveFormField(th, FormFieldStyle{Label: Style{Clear: ClearTextColor}}).Label.TextColor; !got.IsZero() {
		t.Fatalf("a cleared label color = %v, want none", got)
	}
}

func TestFormPartsFollowHighContrast(t *testing.T) {
	hc := HighContrastTheme(HighContrastBlack(), DarkTheme())
	st := resolveFormField(hc, FormFieldStyle{Label: Style{TextColor: Hex("#222222")}})
	if st.Label.TextColor != hc.Text {
		t.Fatalf("a label under high contrast is %v, want the system text color", st.Label.TextColor)
	}
	if got := messageIconColor(hc, Style{}); got != hc.Text {
		t.Fatalf("the error icon under high contrast is %v, want the system text color", got)
	}
	if got := resolveForm(hc, FormStyle{Summary: Style{BorderColor: Hex("#333333")}}).Summary.BorderColor; got != hc.Text {
		t.Fatalf("the summary border under high contrast is %v, want the system text color", got)
	}
}

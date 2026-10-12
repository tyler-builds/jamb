package views

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// emailPattern is the shape the Forms sample accepts for an address: text, an @
// sign and text, with no spaces.
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)

// formsSection shows the form API in one place: a required field with help text,
// an inline field, the error summary, and an async submit with a spinner. The
// simulated server refuses the name "taken", and the form shows that refusal as its
// own message. Its values are kept in the section.
func formsSection(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		form := ui.Local(ctx, "forms.form", ui.NewForm()).Peek()
		name := ui.Local(ctx, "forms.name", "")
		email := ui.Local(ctx, "forms.email", "")
		result := ui.Local(ctx, "forms.result", "")
		form.OnSubmitAsync(func() <-chan error {
			value := strings.TrimSpace(name.Peek())
			done := make(chan error, 1)
			go func() {
				time.Sleep(2 * time.Second)
				if strings.EqualFold(value, "taken") {
					done <- errors.New("That name is already in use. Choose another.")
					return
				}
				ctx.Post(func() { result.Set("Saved “" + value + "”.") })
				done <- nil
			}()
			return done
		})
		body := ui.Div().Column().Gap(12).Width(ui.Px(420)).
			Child(form.Summary()).
			Child(form.Field("name", "Name", ui.TextInput(name).Key("forms-name").Placeholder("A name for the item").Width(ui.Full).
				Validate(ui.Required[string](), ui.MaxLength(40))).
				Description("Must be unique. Enter “taken” to see a refusal from the server.").Required()).
			Child(form.Field("email", "Email", ui.TextInput(email).Placeholder("name@example.com").Width(ui.Full).
				Validate(ui.Pattern(emailPattern, "Enter an email address like name@example.com."))).Inline())
		actions := ui.Div().Row().Align(ui.Center).Gap(10).
			Child(ui.Button("Save").Key("forms-save").Primary().Disabled(form.Submitting()).OnClick(form.Submit))
		if form.Submitting() {
			actions.Child(ui.ProgressRing().Size(ui.RingSmall))
		}
		return Card(t).Gap(14).
			Child(SectionTitle(t, "Forms")).
			Child(ui.Text("A required name with help text, an email on the same row as its label, and a summary of the errors on submit. Save takes a moment, and a name that is taken is refused by the server.").
				Size(13).Color(t.TextMuted)).
			Child(body).
			Child(actions).
			Child(ui.Text(result.Get()).Size(13).Color(t.TextMuted))
	})
}

package uitest_test

import (
	"fmt"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// The example of docs/testing.md, kept here so that the guide stays true.

type signInForm struct {
	Name     *ui.Signal[string]
	Remember *ui.Signal[bool]
	Status   *ui.Signal[string]
}

func newSignInForm() *signInForm {
	return &signInForm{Name: ui.NewSignal(""), Remember: ui.NewSignal(false), Status: ui.NewSignal("")}
}

func (f *signInForm) Root(ctx *ui.Context) ui.Element {
	name := f.Name.Get()
	return ui.Div().Column().Gap(8).Padding(16).Children(
		ui.TextInput(f.Name).Key("name").Label("Name"),
		ui.Checkbox("Remember me", f.Remember).Key("remember"),
		ui.Button("Sign in").Key("sign-in").Disabled(name == "").
			OnClick(func() { f.Status.Set(fmt.Sprintf("Welcome, %s", f.Name.Peek())) }),
		ui.Div().Key("status").Child(ui.Text(f.Status.Get())),
	)
}

func TestSignIn(t *testing.T) {
	f := newSignInForm()
	u := uitest.New(t, f.Root, uitest.Size(400, 300))

	signIn := u.ByRole(access.RoleButton, "Sign in")
	if !signIn.Disabled() {
		t.Fatal("Sign in is enabled before a name is typed")
	}
	u.Get("name").Type("Ada")
	u.Get("remember").Click()
	signIn.Click()

	if got := u.Get("status").Text(); got != "Welcome, Ada" {
		t.Errorf("status = %q", got)
	}
	if !u.Get("remember").Checked() {
		t.Error("Remember me is not checked")
	}

	u.Press("shift+tab") // focus back to Remember me, shown as keyboard focus
	if !u.Get("remember").FocusRing() {
		t.Error("no focus ring around Remember me after Shift+Tab")
	}
}

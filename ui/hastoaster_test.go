package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// A widget reads ctx.HasToaster to tell whether its toast would show. It re-renders
// when the first Toaster is mounted and when the last one is removed.
func TestHasToasterFollowsMountedToasters(t *testing.T) {
	toasters := ui.NewSignal(0)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		root := ui.Div().Column().Child(ui.Component(func(ctx *ui.Context) ui.Element {
			if ctx.HasToaster() {
				return ui.Text("toast").Key("feedback")
			}
			return ui.Text("inline").Key("feedback")
		}))
		for i := range toasters.Get() {
			root.Child(ui.Toaster().Key(string(rune('a' + i))))
		}
		return root
	})
	check := func(when, want string) {
		t.Helper()
		if got := u.Get("feedback").Text(); got != want {
			t.Errorf("%s: the widget shows %q, want %q", when, got, want)
		}
	}
	check("no Toaster", "inline")
	toasters.Set(1)
	u.Frame()
	check("a Toaster mounted", "toast")
	toasters.Set(2)
	u.Frame()
	check("two Toasters", "toast")
	toasters.Set(1)
	u.Frame()
	check("one of two Toasters removed", "toast")
	toasters.Set(0)
	u.Frame()
	check("the last Toaster removed", "inline")
}

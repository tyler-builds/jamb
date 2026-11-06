package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// These tests record the demo's frames while they act, and check that nothing
// pops, jumps or keeps moving where a person would notice (see the Motion
// section of docs/testing.md). Each allowed finding says why it is intended or
// what it waits for.

// The modal, the drawer and the alert dialog of the Components page enter and
// leave with their motion.
func TestDialogsMotion(t *testing.T) {
	u := demoUI(t, PageComponents, "Dialogs")
	rec := u.Record()
	for _, open := range []string{"Form in a modal", "Open drawer", "Delete sample…"} {
		u.ByRole(access.RoleButton, open).Click()
		u.Settle()
		u.Press("escape")
		u.Settle()
	}
	rec.Check(t)
}

// Toasts slide in and fade out. When the older one times out, the one above it
// drops into its place in one frame.
func TestToastsMotion(t *testing.T) {
	u := demoUI(t, PageComponents, "Feedback")
	rec := u.Record()
	u.ByRole(access.RoleButton, "Info toast").Click()
	u.Settle()
	u.ByRole(access.RoleButton, "Success toast").Click()
	u.Settle()
	u.Advance(8 * time.Second) // both time out
	u.Settle()
	rec.Check(t, uitest.Allow{Kind: uitest.Jump, Element: "toaster/jamb.toast",
		Reason: "a toast that leaves exits as a ghost outside the Toaster's layout, so the toasts after it " +
			"move into its place at once (docs/animation.md, Cost); the Toaster does not yet glide them"})
}

// A tooltip fades in after its delay and out when the pointer leaves.
func TestTooltipMotion(t *testing.T) {
	u := demoUI(t, PageDashboard, "")
	rec := u.Record()
	u.Get("nav-Settings").Hover()
	u.Advance(time.Second)
	u.Get("user-card").Hover()
	u.Settle()
	rec.Check(t)
}

// The date picker's calendar and the time picker's list enter and leave with
// their motion.
func TestPickersMotion(t *testing.T) {
	u := demoUI(t, PageComponents, "Date and time")
	rec := u.Record()
	for _, open := range []string{"Choose date", "Show times"} {
		u.ByRole(access.RoleButton, open).Click()
		u.Settle()
		u.Press("escape")
		u.Settle()
	}
	rec.Check(t)
}

// A page replaces the one before it at once, by design.
func TestPageSwitchMotion(t *testing.T) {
	u := demoUI(t, PageDashboard, "")
	rec := u.Record()
	u.Click("nav-Projects")
	u.Settle()
	const why = "a click on the sidebar replaces the page at once, by design"
	rec.Check(t, uitest.Allow{Kind: uitest.PopIn, Reason: why}, uitest.Allow{Kind: uitest.PopOut, Reason: why})
}

// Tabs switch their panel at once, and a Collapsible, an Accordion and a
// collapsing Sidebar show and hide what they hold at once: they have no
// open and close motion yet.
func TestDisclosureMotion(t *testing.T) {
	const notYet = "Collapsible, Accordion and Sidebar open and close with no motion; " +
		"docs/comparison-gpui.md lists their animated open and close as a gap"
	u := demoUI(t, PageComponents, "Navigation and layout")
	rec := u.Record()
	u.ByRole(access.RoleTab, "Beta").Click()
	u.Settle()
	u.ByRole(access.RoleButton, "Advanced options").Click()
	u.Settle()
	u.ByRole(access.RoleButton, "Email alerts").Click()
	u.Settle()
	rec.Check(t,
		uitest.Allow{Kind: uitest.PopIn, Element: "panel:beta", Reason: "a tab shows its panel at once, by design"},
		uitest.Allow{Kind: uitest.PopOut, Element: "panel:alpha", Reason: "a tab shows its panel at once, by design"},
		uitest.Allow{Kind: uitest.PopIn, Element: "nav.advanced", Reason: notYet},
		uitest.Allow{Kind: uitest.PopOut, Element: "nav.emailSection", Reason: notYet})

	u = demoUI(t, PageComponents, "More widgets")
	rec = u.Record()
	u.ByRole(access.RoleButton, "Collapse navigation").Click()
	u.Settle()
	rec.Check(t, uitest.Allow{Kind: uitest.PopOut, Element: "demo-sidebar", Reason: notYet})
}

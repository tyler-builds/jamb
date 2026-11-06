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

// Toasts slide in and fade out. When one times out, the others glide into its
// place (the theme's Reflow spring).
func TestToastsMotion(t *testing.T) {
	u := demoUI(t, PageComponents, "Feedback")
	rec := u.Record()
	u.ByRole(access.RoleButton, "Info toast").Click()
	u.Settle()
	u.ByRole(access.RoleButton, "Success toast").Click()
	u.Settle()
	u.Advance(8 * time.Second) // both time out
	u.Settle()
	rec.Check(t)
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

// Tabs switch their panel at once. A Collapsible and an Accordion spring open
// and closed, revealing and covering what they hold, and a collapsing Sidebar
// springs to its new width (the theme's Reflow spring).
func TestDisclosureMotion(t *testing.T) {
	u := demoUI(t, PageComponents, "Navigation and layout")
	rec := u.Record()
	u.ByRole(access.RoleTab, "Beta").Click()
	u.Settle()
	for _, b := range []string{"Advanced options", "Email alerts", "Text alerts", "Advanced options", "Text alerts"} {
		u.ByRole(access.RoleButton, b).Click()
		u.Settle()
	}
	rec.Check(t,
		uitest.Allow{Kind: uitest.PopIn, Element: "panel:beta", Reason: "a tab shows its panel at once, by design"},
		uitest.Allow{Kind: uitest.PopOut, Element: "panel:alpha", Reason: "a tab shows its panel at once, by design"})

	u = demoUI(t, PageComponents, "More widgets")
	rec = u.Record()
	u.ByRole(access.RoleButton, "Collapse navigation").Click()
	u.Settle()
	u.ByRole(access.RoleButton, "Expand navigation").Click()
	u.Settle()
	rec.Check(t)
}

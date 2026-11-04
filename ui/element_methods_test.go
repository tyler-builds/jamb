package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// Text takes Basis and margins as a Div does, so it needs no wrapping Div to
// share a row or keep its distance.
func TestTextBasisAndMargins(t *testing.T) {
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Column().
			Child(ui.Div().Row().Width(ui.Px(400)).
				Child(ui.Text("a").Key("a").Grow(1).Basis(ui.Px(0))).
				Child(ui.Text("bbbbbb").Key("b").Grow(1).Basis(ui.Px(0)))).
			Child(ui.Text("c").Key("c").MarginTop(6).MarginLeft(4))
	})
	a, b, c := u.Get("a").Bounds(), u.Get("b").Bounds(), u.Get("c").Bounds()
	if a.W != 200 || b.X != 200 || b.W != 200 {
		t.Errorf("Basis(0) texts: a %v, b %v; want two 200 px halves", a, b)
	}
	if c.X != 4 || c.Y != a.H+6 {
		t.Errorf("text with margins at (%v, %v), want (4, %v)", c.X, c.Y, a.H+6)
	}
}

// A badge's tooltip is its accessible description.
func TestBadgeTooltipIsItsDescription(t *testing.T) {
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Div().Child(ui.BadgeText("Verified").Tooltip("Signed by the publisher"))
	})
	if d := u.ByName("Verified").Node().Description; d != "Signed by the publisher" {
		t.Errorf("badge description %q, want the tooltip", d)
	}
}

// HideValue, or a Format that returns "", leaves the track the whole width.
func TestProgressBarHideValue(t *testing.T) {
	for name, bar := range map[string]func() *ui.ProgressBarElement{
		"HideValue": func() *ui.ProgressBarElement { return ui.ProgressBar(0.84).Label("Trust").HideValue() },
		"empty Format": func() *ui.ProgressBarElement {
			return ui.ProgressBar(0.84).Label("Trust").Format(func(float64) string { return "" })
		},
	} {
		u := uitest.New(t, func(*ui.Context) ui.Element {
			return ui.Div().Column().Child(ui.Div().Column().Width(ui.Px(300)).Child(bar().Key("bar")))
		})
		track := u.ByRole(access.RoleProgressBar, "Trust").Bounds()
		if track.W != 300 {
			t.Errorf("%s: track %v wide, want 300", name, track.W)
		}
		if v := u.ByRole(access.RoleProgressBar, "Trust").Node().Range; v == nil || v.Value != 0.84 {
			t.Errorf("%s: range %+v, want the value 0.84 kept for assistive technology", name, v)
		}
	}
}

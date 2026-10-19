package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// TestAlertGroup checks that an alert is a group named by its title and
// described by its description, and that it has no close button without OnClose.
func TestAlertGroup(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Alert("Sync paused").Description("Reconnect to resume.").Warning()
	})
	tr := r.tree()
	n := accesstest.Find(t, tr, access.RoleGroup, "Sync paused")
	if n.Description != "Reconnect to resume." {
		t.Errorf("description %q, want the alert's description", n.Description)
	}
	accesstest.Absent(t, tr, access.RoleButton, "Close")
}

// TestAlertWithoutTitle checks that an alert with only a description is named
// by it, and draws no title.
func TestAlertWithoutTitle(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return Alert("").Description("Saved.") })
	accesstest.Find(t, r.tree(), access.RoleGroup, "Saved.")
	if n := partNodes(r.rt, "Alert.Title"); len(n) != 0 {
		t.Errorf("%d titles drawn, want none", len(n))
	}
}

// TestAlertIsNotAnnounced checks that an alert is not a live region: building it
// announces nothing.
func TestAlertIsNotAnnounced(t *testing.T) {
	takeAnnouncements()
	r := newAccessRig(func(ctx *Context) Element { return Alert("Disk nearly full").Danger() })
	r.tree()
	if got := takeAnnouncements(); len(got) != 0 {
		t.Errorf("announced %q, want nothing", got)
	}
}

// TestAlertClose checks that the close button is a focusable button named Close,
// and that it calls OnClose when it is clicked, and when Enter or Space is pressed
// on it.
func TestAlertClose(t *testing.T) {
	closed := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Alert("Update ready").OnClose(func() { closed++ })
	})
	accesstest.Check(t, accesstest.Find(t, r.tree(), access.RoleButton, "Close"),
		access.RoleButton, "Close", access.StateFocusable)
	clickPart(t, r, "Alert.Close")
	if closed != 1 {
		t.Fatalf("a click calls OnClose %d times, want 1", closed)
	}
	r.focusAccess(t, access.RoleButton, "Close")
	r.press(input.KeyEnter, 0)
	r.press(input.KeySpace, 0)
	if closed != 3 {
		t.Errorf("Enter and Space call OnClose %d times in all, want 3", closed)
	}
}

// TestAlertIcon checks the icon: each variant draws one by default, Icon replaces
// it, and Icon(nil) hides it.
func TestAlertIcon(t *testing.T) {
	icon := func(a *AlertElement) *icons.Icon {
		r := newAccessRig(func(ctx *Context) Element { return a })
		ns := partNodes(r.rt, "Alert.Icon")
		if len(ns) == 0 {
			return nil
		}
		return ns[0].children[0].icon.icon
	}
	for v, want := range map[AlertVariant]*icons.Icon{AlertInfo: icons.Info, AlertSuccess: icons.Check,
		AlertWarning: icons.Alert, AlertDanger: icons.Alert} {
		if got := icon(Alert("A").Variant(v)); got != want {
			t.Errorf("variant %d: icon %v, want %v", v, got, want)
		}
	}
	if got := icon(Alert("A").Icon(icons.Bell)); got != icons.Bell {
		t.Errorf("Icon(Bell): icon %v, want Bell", got)
	}
	if got := icon(Alert("A").Icon(nil)); got != nil {
		t.Errorf("Icon(nil): icon %v, want none", got)
	}
}

// TestAlertVariantStyles checks the theme's Alert, which applies to every
// variant, and its Alerts, which applies to one variant with the instance over
// both.
func TestAlertVariantStyles(t *testing.T) {
	all, success, inst := Hex("#111213"), Hex("#121314"), Hex("#141516")
	bg := func(th *Theme, v AlertVariant, st AlertStyle) Color {
		r := newThemedRig(th, func(ctx *Context) Element { return Alert("A").Variant(v).Styles(st) })
		return partNode(t, r.rt, "Alert.Alert", "").style.Background
	}
	th := *DarkTheme()
	th.Alert.Alert.Background = all
	th.Alerts.Success.Alert.Background = success
	if got := bg(&th, AlertInfo, AlertStyle{}); got != all {
		t.Errorf("info: background %v, want the theme's Alert %v", got, all)
	}
	if got := bg(&th, AlertSuccess, AlertStyle{}); got != success {
		t.Errorf("success: background %v, want the variant's %v", got, success)
	}
	if got := bg(&th, AlertSuccess, AlertStyle{Alert: Style{Background: inst}}); got != inst {
		t.Errorf("success with an instance: background %v, want %v", got, inst)
	}
}

// TestAlertUnderHighContrast checks that an app color does not reach an alert
// under high contrast: it is the window color outlined in the text color.
func TestAlertUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Alert.Alert = Style{Background: markBG}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Alert("A").Danger().Styles(AlertStyle{Alert: Style{Background: markBG}})
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	n := partNode(t, r.rt, "Alert.Alert", "")
	if n.style.Background != th.Surface {
		t.Errorf("high contrast: the alert is %v, want the window color %v", n.style.Background, th.Surface)
	}
	if n.style.BorderColor != th.Text {
		t.Errorf("high contrast: the border is %v, want the text color %v", n.style.BorderColor, th.Text)
	}
}

func testAlertParts(t *testing.T, applied map[string]bool) {
	type alertCase struct {
		part string
		set  func(*AlertStyle, Style)
		text bool
	}
	cases := []alertCase{
		{part: "Alert.Alert", set: func(s *AlertStyle, m Style) { s.Alert = m }},
		{part: "Alert.Icon", set: func(s *AlertStyle, m Style) { s.Icon = m }, text: true},
		{part: "Alert.Title", set: func(s *AlertStyle, m Style) { s.Title = m }, text: true},
		{part: "Alert.Description", set: func(s *AlertStyle, m Style) { s.Description = m }, text: true},
		{part: "Alert.Close", set: func(s *AlertStyle, m Style) { s.Close = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st AlertStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element {
				return Alert("Sync paused").Description("Reconnect to resume.").OnClose(func() {}).Styles(st)
			})
			n := partNode(t, r.rt, c.part, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

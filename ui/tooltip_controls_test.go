package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// Every interactive control takes a Tooltip: resting the pointer on it shows the
// tooltip, also on a disabled control, and assistive technology reads it as the
// control's description.
func TestControlsTakeATooltip(t *testing.T) {
	opts := []Option[string]{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}
	day := time.Date(2026, 1, 2, 9, 0, 0, 0, time.Local)
	cases := []struct {
		name string
		role access.Role
		el   Element
	}{
		{"TextInput", access.RoleTextField, TextInput(NewSignal("")).Label("C").Tooltip("Help")},
		{"TextArea", access.RoleTextArea, TextArea(NewSignal("")).Label("C").Tooltip("Help")},
		{"NumberInput", access.RoleSpinner, NumberInput(NewSignal(1), 0, 9, 1).Label("C").Tooltip("Help")},
		{"Combobox", access.RoleComboBox, Combobox(NewSignal(""), func(string) []Option[string] { return opts }).Label("C").Tooltip("Help")},
		{"DatePicker", access.RoleTextField, DatePicker(NewSignal(day)).Label("C").Tooltip("Help")},
		{"DateRangePicker", access.RoleTextField, DateRangePicker(NewSignal(day), NewSignal(day.AddDate(0, 0, 2))).Label("C").Tooltip("Help")},
		{"TimePicker", access.RoleComboBox, TimePicker(NewSignal(day)).Label("C").Tooltip("Help")},
		{"Select", access.RoleComboBox, Select(NewSignal("a"), opts...).Label("C").Tooltip("Help")},
		{"Checkbox", access.RoleCheckBox, Checkbox("C", NewSignal(false)).Tooltip("Help")},
		{"Checkbox disabled", access.RoleCheckBox, Checkbox("C", NewSignal(false)).Disabled(true).Tooltip("Help")},
		{"Switch", access.RoleSwitch, Switch("C", NewSignal(false)).Tooltip("Help")},
		{"Slider", access.RoleSlider, Slider(NewSignal(1.0), 0, 9, 1).Label("C").Tooltip("Help")},
		{"RadioGroup", access.RoleGroup, RadioGroup(NewSignal("a"), opts...).Label("C").Tooltip("Help")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newOverlayRig(func(ctx *Context) Element {
				return Div().Padding(20).Align(Start).Child(Div().Key("c").Child(c.el))
			}, 800, 600)
			rc := r.find(t, "c")
			r.rt.PointerMove(rc.X+8, rc.Y+rc.H/2, 0)
			r.frame()
			r.now = r.now.Add(600 * time.Millisecond)
			r.frame()
			if _, ok := r.rt.Find("jamb.tooltip"); !ok {
				t.Fatal("no tooltip after resting on the control")
			}
			if c.name == "Checkbox disabled" || c.name == "RadioGroup" {
				return
			}
			ar := newAccessRig(func(ctx *Context) Element { return c.el })
			if n := accesstest.Find(t, ar.tree(), c.role, "C"); n.Description != "Help" {
				t.Errorf("description %q, want the tooltip", n.Description)
			}
		})
	}
}

// The navigation and display widgets take a Tooltip too, on their root: resting the
// pointer on the widget shows it, and assistive technology reads it as the root's
// description.
func TestWidgetsTakeATooltip(t *testing.T) {
	link := func() {}
	cases := []struct {
		name string
		role access.Role
		as   string // the accessible name of the root
		el   Element
	}{
		{"Pagination", access.RoleGroup, "C", Pagination(NewSignal(2), 5).Label("C").Tooltip("Help")},
		{"Breadcrumbs", access.RoleGroup, "C",
			Breadcrumbs(Crumb{Label: "Home", OnClick: link}, Crumb{Label: "Records"}).Label("C").Tooltip("Help")},
		{"Sidebar", access.RoleGroup, "C",
			Div().Row().Height(Px(200)).Child(Sidebar(NewSignal("a"), SidebarItem{Key: "a", Label: "A"}).Label("C").Tooltip("Help"))},
		{"Tag", access.RoleGroup, "Sales", Tag("Sales").Tooltip("Help")},
		{"Avatar", access.RoleImage, "Sample User", Avatar("Sample User").Tooltip("Help")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newOverlayRig(func(ctx *Context) Element {
				return Div().Padding(20).Align(Start).Child(Div().Key("c").Child(c.el))
			}, 800, 600)
			rc := r.find(t, "c")
			r.rt.PointerMove(rc.X+4, rc.Y+rc.H/2, 0)
			r.frame()
			r.now = r.now.Add(600 * time.Millisecond)
			r.frame()
			if _, ok := r.rt.Find("jamb.tooltip"); !ok {
				t.Fatal("no tooltip after resting on the widget")
			}
			ar := newAccessRig(func(ctx *Context) Element { return c.el })
			if n := accesstest.Find(t, ar.tree(), c.role, c.as); n.Description != "Help" {
				t.Errorf("description %q, want the tooltip", n.Description)
			}
		})
	}
}

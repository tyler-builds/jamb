package views

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// TestComponentsFormControlsReportTheirStates checks the form controls of the
// Components page: their roles, states and values, and that the page builds
// without development warnings.
func TestComponentsFormControlsReportTheirStates(t *testing.T) {
	r := newDemoRig(t)
	r.s.Page.Set(PageComponents)
	tr := r.tree()
	if len(r.warnings) != 0 {
		t.Fatalf("warnings on the Components page: %v", r.warnings)
	}

	email := accesstest.Find(t, tr, access.RoleCheckBox, "Email alerts")
	if !email.States.Has(access.StateChecked) || !email.States.Has(access.StateToggle) {
		t.Fatalf("Email alerts has states [%s], want checked and toggle", email.States)
	}
	mixed := accesstest.Find(t, tr, access.RoleCheckBox, "Some channels")
	if !mixed.States.Has(access.StateMixed) || mixed.States.Has(access.StateChecked) {
		t.Fatalf("Some channels has states [%s], want mixed and not checked", mixed.States)
	}
	group := accesstest.Find(t, tr, access.RoleGroup, "Frequency")
	if !group.States.Has(access.StateSelectionContainer) {
		t.Fatalf("the Frequency group has states [%s], want a selection container", group.States)
	}
	daily := accesstest.Find(t, tr, access.RoleRadioButton, "Daily digest")
	if !daily.States.Has(access.StateSelected) {
		t.Fatalf("Daily digest has states [%s], want selected", daily.States)
	}
	combo := accesstest.Find(t, tr, access.RoleComboBox, "Channel")
	if combo.Value != "Email" || !combo.States.Has(access.StateExpandable) {
		t.Fatalf("the Channel select has value %q and states [%s]", combo.Value, combo.States)
	}
	slider := accesstest.Find(t, tr, access.RoleSlider, "Volume")
	if slider.Range == nil || slider.Range.Value != 40 || slider.Range.Max != 100 {
		t.Fatalf("the Volume slider range is %+v", slider.Range)
	}
	spin := accesstest.Find(t, tr, access.RoleSpinner, "Sample quiet hours")
	if spin.Range == nil || spin.Range.Value != 22 || spin.Range.Max != 23 {
		t.Fatalf("the quiet hours spinner range is %+v", spin.Range)
	}
	if accesstest.Find(t, tr, access.RoleSpinner, "Quiet hours from").Range == nil {
		t.Fatalf("the preferences quiet hours have no range")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings after the form was read: %v", r.warnings)
	}
}

// TestSettingsSwitchChangesTheTheme checks that the Dark mode switch, which is a
// switch of the framework, changes the theme and reports the new state.
func TestSettingsSwitchChangesTheTheme(t *testing.T) {
	r := newDemoRig(t)
	r.s.Page.Set(PageSettings)
	dark := accesstest.Find(t, r.tree(), access.RoleSwitch, "Dark mode")
	if !dark.States.Has(access.StateChecked) {
		t.Fatalf("Dark mode is not on in the dark theme: [%s]", dark.States)
	}
	r.bridge.Perform(dark.ID, access.ActionToggle)
	r.frame()
	if accesstest.Find(t, r.tree(), access.RoleSwitch, "Dark mode").States.Has(access.StateChecked) {
		t.Fatalf("toggling Dark mode did not turn it off")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

//go:build windows

package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// TestFormPropertyIDsMatchTheSDK checks the two form properties against the values
// in the Windows SDK header UIAutomationCoreApi.h (10.0.26100.0):
// UIA_IsRequiredForFormPropertyId is 30025 and UIA_IsDataValidForFormPropertyId is
// 30103.
func TestFormPropertyIDsMatchTheSDK(t *testing.T) {
	if propIsRequiredForForm != 30025 {
		t.Errorf("propIsRequiredForForm = %d, want 30025", propIsRequiredForForm)
	}
	if propIsDataValidForForm != 30103 {
		t.Errorf("propIsDataValidForForm = %d, want 30103", propIsDataValidForForm)
	}
}

// TestRequiredAndValidForForm checks what a form field reports: IsRequiredForForm
// for a required field, and IsDataValidForForm false for an invalid one. An element
// that is neither leaves IsDataValidForForm empty.
func TestRequiredAndValidForForm(t *testing.T) {
	cases := []struct {
		name     string
		states   access.State
		required bool
		valid    int16 // variantBoolOf of IsDataValidForForm (-1 true, 0 false)
		empty    bool  // IsDataValidForForm is VT_EMPTY
	}{
		{name: "required, valid", states: access.StateRequired, required: true, valid: -1},
		{name: "required, invalid", states: access.StateRequired | access.StateInvalid, required: true, valid: 0},
		{name: "optional, valid", states: 0, required: false, empty: true},
		{name: "optional, invalid", states: access.StateInvalid, required: false, valid: 0},
	}
	for _, c := range cases {
		n := &access.Node{ID: 9, Role: access.RoleTextField, States: c.states}
		req := nodeProperty(propIsRequiredForForm, n, false)
		if req.vt != vtBool || (variantBoolOf(req) == -1) != c.required {
			t.Errorf("%s: IsRequiredForForm = tag %d value %d, want required %v", c.name, req.vt, variantBoolOf(req), c.required)
		}
		valid := nodeProperty(propIsDataValidForForm, n, false)
		if c.empty {
			if valid.vt != vtEmpty {
				t.Errorf("%s: IsDataValidForForm = tag %d, want empty", c.name, valid.vt)
			}
			continue
		}
		if valid.vt != vtBool || variantBoolOf(valid) != c.valid {
			t.Errorf("%s: IsDataValidForForm = tag %d value %d, want %d", c.name, valid.vt, variantBoolOf(valid), c.valid)
		}
	}
}

package views

import "testing"

// The input format samples show the raw value the field's signal holds: a typed phone
// number is read back as its digits, and the Accept sample refuses an uppercase letter
// as it is typed, so only the lowercase letter after it is kept.
func TestInputFormatsShowTheRawValue(t *testing.T) {
	r := newDemoRig(t)
	showComponents(r)
	clickOK(t, r, "format-phone")
	r.rt.Type("5551234567")
	r.frame()
	if !hasText(r, `Value: raw "5551234567"`) {
		t.Fatal("the phone readout does not show the digits typed")
	}
	clickOK(t, r, "format-handle")
	r.rt.Type("Ab")
	r.frame()
	if !hasText(r, `Value: raw "b"`) {
		t.Fatal("the handle readout does not show the lowercase letter kept after Accept refused the uppercase one")
	}
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

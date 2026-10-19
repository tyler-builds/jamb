//go:build windows

package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// TestValuePatternFollowsTheNodeOrItsRole checks which nodes the Value pattern is
// offered on: a text field and a combo box by their role, and an element that takes
// a value through the Set Value action. A read-only data item has no Value pattern,
// since Microsoft's DataItem guidance requires it only for an editable primary text
// (uiauto-supportdataitemcontroltype).
func TestValuePatternFollowsTheNodeOrItsRole(t *testing.T) {
	cases := []struct {
		name string
		n    access.Node
		want bool
	}{
		{"text field", access.Node{Role: access.RoleTextField}, true},
		{"combo box", access.Node{Role: access.RoleComboBox}, true},
		{"editable data item", access.Node{Role: access.RoleDataItem,
			Actions: []access.Action{access.ActionFocus, access.ActionSetValue}}, true},
		{"read-only data item", access.Node{Role: access.RoleDataItem,
			Actions: []access.Action{access.ActionFocus}}, false},
		{"button", access.Node{Role: access.RoleButton}, false},
		{"slider", access.Node{Role: access.RoleSlider, Range: &access.Range{Max: 10}}, false},
		{"slider with a value text", access.Node{Role: access.RoleSlider, Range: &access.Range{Max: 5}, Value: "3 of 5 stars"}, true},
	}
	for _, c := range cases {
		n := c.n
		if got := hasValueNode(&n); got != c.want {
			t.Errorf("%s: Value pattern %v, want %v", c.name, got, c.want)
		}
	}
}

// TestSliderValueTextIsReadOnly checks that the Value of a slider with a value text
// is read-only: the slider is set through RangeValue, and its RangeValue stays
// writable unless the slider itself is read-only.
func TestSliderValueTextIsReadOnly(t *testing.T) {
	n := access.Node{Role: access.RoleSlider, Range: &access.Range{Max: 5}, Value: "3 of 5 stars"}
	if !valueReadOnly(&n) || n.States.Has(access.StateReadOnly) {
		t.Fatalf("the value text of a slider is not read-only, or the slider is")
	}
	if field := (access.Node{Role: access.RoleTextField}); valueReadOnly(&field) {
		t.Fatalf("an editable text field is read-only")
	}
}

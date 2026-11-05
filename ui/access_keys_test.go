package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
)

// keyedRoles returns the roles of the nodes whose key is key.
func keyedRoles(t *testing.T, e Element, key string) []string {
	t.Helper()
	rt := NewRuntime(func(*Context) Element { return Div().Child(e) }, nil, nil)
	rt.SetViewport(400, 300, 1)
	rt.Frame(time.Now())
	var roles []string
	rt.AccessTree().Walk(func(n *access.Node) bool {
		if n.Key == key {
			roles = append(roles, n.Role.String())
		}
		return true
	})
	return roles
}

// A control's key is its node's automation ID, also when the control renders
// a frame around that node, as a TextInput does.
func TestAccessKeysOfControls(t *testing.T) {
	s, on, n, v := NewSignal(""), NewSignal(false), NewSignal(3), NewSignal(0.5)
	when, color := NewSignal(time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)), NewSignal(Color{})
	opt := Option[string]{Value: "a", Label: "A"}
	for _, c := range []struct {
		name string
		e    Element
		role string
	}{
		{"TextInput", TextInput(s).Key("k").Label("L"), "text field"},
		{"TextArea", TextArea(s).Key("k").Label("L"), "text area"},
		{"Slider", Slider(v, 0, 1, 0.1).Key("k").Label("L"), "slider"},
		{"ColorPicker", ColorPicker(color).Key("k").Label("L"), "button"},
		{"Select", Select(s, opt).Key("k").Label("L"), "combo box"},
		{"Combobox", Combobox(s, func(string) []Option[string] { return nil }).Key("k").Label("L"), "combo box"},
		{"DatePicker", DatePicker(when).Key("k").Label("L"), "combo box"},
		{"Checkbox", Checkbox("L", on).Key("k"), "check box"},
		{"Switch", Switch("L", on).Key("k"), "switch"},
		{"Rating", Rating("L", n).Key("k"), "slider"},
		{"OTPInput", OTPInput("L", s, 4).Key("k"), "text field"},
		{"RadioGroup", RadioGroup(s, opt).Key("k").Label("L"), "group"},
		{"Button", Button("L").Key("k"), "button"},
	} {
		if got := keyedRoles(t, c.e, "k"); len(got) != 1 || got[0] != c.role {
			t.Errorf("%s: key on %v; want one %s", c.name, got, c.role)
		}
	}
}

// A layout container passes its key on only to a node it alone contributes
// that has no key of its own.
func TestAccessKeysOfContainers(t *testing.T) {
	if got := keyedRoles(t, Div().Key("row").Children(Button("A"), Button("B")), "row"); len(got) != 0 {
		t.Errorf("a row of two buttons gave its key to %v", got)
	}
	if got := keyedRoles(t, Div().Key("card").Child(Button("A").Key("a")), "card"); len(got) != 0 {
		t.Errorf("a card gave its key to %v, over the button's own", got)
	}
	if got := keyedRoles(t, Div().Key("card").Child(Button("A")), "card"); len(got) != 1 || got[0] != "button" {
		t.Errorf("a card around one button: key on %v; want the button", got)
	}
}

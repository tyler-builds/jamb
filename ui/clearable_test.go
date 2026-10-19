package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// The clear button shows while the field has text. A click clears it, runs
// OnChange and leaves the focus in the field.
func TestClearableButton(t *testing.T) {
	sig := NewSignal("")
	var changes []string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(TextInput(sig).Label("Search").Key("q").Clearable().
			OnChange(func(s string) { changes = append(changes, s) }))
	})
	accesstest.Absent(t, r.tree(), access.RoleButton, "Clear")
	r.focusKey(t, "q")
	r.typeText("ab")
	n := accesstest.Find(t, r.tree(), access.RoleButton, "Clear")
	if n.States.Has(access.StateFocusable) {
		t.Error("the clear button is a Tab stop")
	}
	changes = nil
	rc := r.rt.nodeRect(partNode(t, r.rt, "TextInput.Button", ""))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if sig.Get() != "" {
		t.Errorf("after the click the value is %q, want it empty", sig.Get())
	}
	if len(changes) != 1 || changes[0] != "" {
		t.Errorf("OnChange ran with %q, want once with the empty text", changes)
	}
	if r.focusedKey() != "q" {
		t.Errorf("after the click the focus is on %q, want the field", r.focusedKey())
	}
	accesstest.Absent(t, r.tree(), access.RoleButton, "Clear")
}

// Escape clears a clearable field that has text, and can be undone; the next
// Escape leaves the field, as in any field.
func TestClearableEscape(t *testing.T) {
	sig := NewSignal("")
	var changes []string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(TextInput(sig).Label("Search").Key("q").Clearable().
			OnChange(func(s string) { changes = append(changes, s) }))
	})
	r.focusKey(t, "q")
	r.typeText("abc")
	changes = nil
	r.press(input.KeyEscape, 0)
	if sig.Get() != "" || len(changes) != 1 || changes[0] != "" {
		t.Errorf("Escape left %q and ran OnChange with %q, want the text cleared once", sig.Get(), changes)
	}
	if r.focusedKey() != "q" {
		t.Errorf("the first Escape moved the focus to %q", r.focusedKey())
	}
	r.press(input.KeyZ, input.ModCtrl)
	if sig.Get() != "abc" {
		t.Errorf("Ctrl+Z after Escape gave %q, want the text back", sig.Get())
	}
	r.press(input.KeyEscape, 0)
	r.press(input.KeyEscape, 0)
	if r.rt.focusID != 0 {
		t.Errorf("the Escape on an empty field kept the focus on %q", r.focusedKey())
	}
}

// A field that is not clearable keeps its text on Escape, and a read-only one
// shows no clear button.
func TestEscapeWithoutClearable(t *testing.T) {
	sig := NewSignal("keep")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().
			Child(TextInput(sig).Label("Plain").Key("plain")).
			Child(TextInput(NewSignal("x")).Label("Locked").ReadOnly(true).Clearable())
	})
	accesstest.Absent(t, r.tree(), access.RoleButton, "Clear")
	r.focusKey(t, "plain")
	r.press(input.KeyEscape, 0)
	if sig.Get() != "keep" {
		t.Errorf("Escape changed a field that is not clearable to %q", sig.Get())
	}
}

// The clear button comes before the reveal button, and both before the app's
// Trailing element.
func TestClearableOrder(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(TextInput(NewSignal("x")).Label("Password").Masked().Revealable().Clearable().
			Trailing(Text("tail").Key("tail")))
	})
	tr := r.tree()
	var names []string
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleButton {
			names = append(names, n.Name)
		}
		return true
	})
	if len(names) != 2 || names[0] != "Clear" || names[1] != "Show password" {
		t.Errorf("the buttons are %q, want Clear and then Show password", names)
	}
	trail := partNode(t, r.rt, "TextInput.Trailing", "")
	if len(trail.children) != 3 || trail.children[2].key != "tail" {
		t.Errorf("the Trailing part does not end with the app's element")
	}
}

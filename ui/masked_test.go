package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// maskedRig is a masked text input named Password, focused, holding value.
func maskedRig(t *testing.T, value string, build func(*TextInputElement) *TextInputElement) (*accessRig, *Signal[string]) {
	t.Helper()
	sig := NewSignal(value)
	r := newAccessRig(func(ctx *Context) Element {
		ti := TextInput(sig).Label("Password").Key("pw").Masked()
		if build != nil {
			ti = build(ti)
		}
		return Div().Column().Child(ti)
	})
	r.focusKey(t, "pw")
	return r, sig
}

// A masked field draws a bullet for each character, and assistive technology reads
// a password field whose value and text are the bullets.
func TestMaskedFieldShowsBullets(t *testing.T) {
	// "e" with a combining accent is one character of two runes.
	r, sig := maskedRig(t, "", nil)
	r.typeText("ab")
	r.rt.TextInput("é")
	r.frame()
	if got := sig.Get(); got != "abé" {
		t.Fatalf("the value is %q, want the typed text", got)
	}
	if got := editorNode(t, r.rt).st.edit.layout.Text; got != "•••" {
		t.Errorf("the field draws %q, want three bullets", got)
	}
	n := r.stateOf(t, access.RoleTextField, "Password")
	if n.Value != "•••" {
		t.Errorf("the accessible value is %q, want three bullets", n.Value)
	}
	if !n.States.Has(access.StatePassword) {
		t.Errorf("the field has states %v, want the password state", n.States)
	}
	if n.Text == nil || n.Text.Content.Read(0, n.Text.Content.Len(), -1) != "•••" {
		t.Errorf("the text content is not the bullets: %+v", n.Text)
	}
	if n.Text != nil && n.Text.Caret != 3 {
		t.Errorf("the text caret is at point %d, want 3", n.Text.Caret)
	}

	// Left moves over the accented character as one.
	r.press(input.KeyLeft, 0)
	if es := editorNode(t, r.rt).st.edit; es.caret != 2 {
		t.Errorf("after Left the caret is at byte %d, want 2", es.caret)
	}
	// Ctrl+Left goes to the start, since the words are not shown.
	r.press(input.KeyRight, 0)
	r.press(input.KeyLeft, input.ModCtrl)
	if es := editorNode(t, r.rt).st.edit; es.caret != 0 {
		t.Errorf("after Ctrl+Left the caret is at byte %d, want 0", es.caret)
	}
	// Ctrl+Delete deletes the whole text after the caret.
	r.press(input.KeyDelete, input.ModCtrl)
	if got := sig.Get(); got != "" {
		t.Errorf("after Ctrl+Delete the value is %q, want it empty", got)
	}
}

// Copy and cut of a masked field do nothing; paste works.
func TestMaskedFieldKeepsTextOffTheClipboard(t *testing.T) {
	r, sig := maskedRig(t, "secret", nil)
	r.rt.memClipboard = "before"
	r.press(input.KeyA, input.ModCtrl)
	r.press(input.KeyC, input.ModCtrl)
	if r.rt.memClipboard != "before" {
		t.Errorf("Ctrl+C copied %q from a masked field", r.rt.memClipboard)
	}
	r.press(input.KeyX, input.ModCtrl)
	if r.rt.memClipboard != "before" || sig.Get() != "secret" {
		t.Errorf("Ctrl+X cut a masked field: clipboard %q, value %q", r.rt.memClipboard, sig.Get())
	}
	r.press(input.KeyV, input.ModCtrl)
	if got := sig.Get(); got != "before" {
		t.Errorf("Ctrl+V over the selection gave %q, want the clipboard's text", got)
	}
}

// A masked field shows no composition: the text the input method confirms is
// typed, as in an OTPInput.
func TestMaskedFieldShowsNoComposition(t *testing.T) {
	r, sig := maskedRig(t, "", nil)
	r.rt.hostCompose(composition{text: "ka", cursor: 2})
	r.frame()
	if r.rt.composing != 0 {
		t.Error("a masked field took a composition")
	}
	r.rt.TextInput("か")
	r.frame()
	if got := sig.Get(); got != "か" {
		t.Errorf("the confirmed text gave %q, want it typed", got)
	}
}

// A double-click in a masked field selects the whole text.
func TestMaskedFieldDoubleClickSelectsAll(t *testing.T) {
	r, _ := maskedRig(t, "two words", nil)
	ed := editorNode(t, r.rt)
	rc := r.rt.nodeRect(ed)
	x, y := rc.X+4, rc.Y+rc.H/2
	r.rt.PointerMove(x, y, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.rt.PointerDown(x, y, input.ButtonLeft, 2, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
	if a, b := ed.st.edit.selection(); a != 0 || b != len("two words") {
		t.Errorf("a double-click selected %d to %d, want the whole text", a, b)
	}
}

// The reveal button shows and hides the text, says which by its name rather than
// by a pressed state, and leaves the focus in the field; Alt+F8 does the same. It comes before
// the app's Trailing element.
func TestRevealableField(t *testing.T) {
	r, _ := maskedRig(t, "pw1", func(ti *TextInputElement) *TextInputElement {
		return ti.Revealable().Trailing(Text("tail").Key("tail"))
	})
	tr := r.tree()
	show := accesstest.Find(t, tr, access.RoleButton, "Show password")
	if show.States.Has(access.StateToggle) || show.States.Has(access.StateChecked) {
		t.Errorf("the reveal button has states %v, want a plain button", show.States)
	}
	if show.States.Has(access.StateFocusable) {
		t.Error("the reveal button is a Tab stop")
	}

	// The button is before the app's element in the Trailing part.
	trail := partNode(t, r.rt, "TextInput.Trailing", "")
	if len(trail.children) != 2 || trail.children[0].div == nil || trail.children[0].div.part != "TextInput.Button" ||
		trail.children[1].key != "tail" {
		t.Errorf("the Trailing part does not hold the button and then the element")
	}

	button := partNode(t, r.rt, "TextInput.Button", "")
	rc := r.rt.nodeRect(button)
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if r.focusedKey() != "pw" {
		t.Errorf("after the click the focus is on %q, want the field", r.focusedKey())
	}
	tr = r.tree()
	hide := accesstest.Find(t, tr, access.RoleButton, "Hide password")
	if hide.States.Has(access.StateToggle) || hide.States.Has(access.StateChecked) {
		t.Errorf("the reveal button has states %v while the text shows, want a plain button", hide.States)
	}
	field := accesstest.Find(t, tr, access.RoleTextField, "Password")
	if field.Value != "pw1" || !field.States.Has(access.StatePassword) {
		t.Errorf("a shown field has value %q and states %v, want its text and the password state", field.Value, field.States)
	}
	if got := editorNode(t, r.rt).st.edit.layout.Text; got != "pw1" {
		t.Errorf("a shown field draws %q, want its text", got)
	}
	// Copy still does nothing.
	r.rt.memClipboard = ""
	r.press(input.KeyA, input.ModCtrl)
	r.press(input.KeyC, input.ModCtrl)
	if r.rt.memClipboard != "" {
		t.Errorf("a shown masked field copied %q", r.rt.memClipboard)
	}

	r.press(input.KeyF8, input.ModAlt)
	if got := editorNode(t, r.rt).st.edit.layout.Text; got != "•••" {
		t.Errorf("after Alt+F8 the field draws %q, want the bullets again", got)
	}
	accesstest.Find(t, r.tree(), access.RoleButton, "Show password")
}

// Revealable does nothing without Masked, and a disabled field has no button.
func TestRevealableNeedsMasked(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().
			Child(TextInput(NewSignal("a")).Label("Plain").Revealable()).
			Child(TextInput(NewSignal("b")).Label("Off").Masked().Revealable().Disabled(true))
	})
	tr := r.tree()
	accesstest.Absent(t, tr, access.RoleButton, "Show password")
	if n := accesstest.Find(t, tr, access.RoleTextField, "Off"); n.Value != "•" || !strings.Contains(n.States.String(), "password") {
		t.Errorf("a disabled masked field has value %q and states %v", n.Value, n.States)
	}
}

// A masked field in a Field is named by the Field's label and still masked.
func TestMaskedFieldInAField(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Field("Password", TextInput(NewSignal("abc")).Masked().Revealable()))
	})
	n := accesstest.Find(t, r.tree(), access.RoleTextField, "Password")
	if n.Value != "•••" || !n.States.Has(access.StatePassword) {
		t.Errorf("a masked field in a Field has value %q and states %v", n.Value, n.States)
	}
}

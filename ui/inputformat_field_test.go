package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// formatRig is a text input named Phone with the format f, focused, holding value,
// and a button named Other, with the field's OnChange calls.
func formatRig(t *testing.T, f InputFormat, value string, build func(*TextInputElement) *TextInputElement) (*accessRig, *Signal[string], *[]string) {
	t.Helper()
	useLocale(t, testLocaleEN())
	sig := NewSignal(value)
	changes := &[]string{}
	r := newAccessRig(func(ctx *Context) Element {
		ti := TextInput(sig).Label("Phone").Key("phone").Format(f).
			OnChange(func(s string) { *changes = append(*changes, s) })
		if build != nil {
			ti = build(ti)
		}
		return Div().Column().Child(ti).Child(Button("Other").Key("other"))
	})
	r.focusKey(t, "phone")
	return r, sig, changes
}

// fieldEdit returns the editor state of the rig's only field.
func fieldEdit(t *testing.T, r *accessRig) *editState {
	t.Helper()
	return editorNode(t, r.rt).st.edit
}

// Typing fills the slots: the signal holds the digits, and the field draws, and
// assistive technology reads, the number in its shape without the prompts.
func TestFormatFieldTyping(t *testing.T) {
	r, sig, _ := formatRig(t, phoneFormat, "", nil)
	r.typeText("555")
	if got := editorNode(t, r.rt).st.edit.layout.Text; got != "(555) " {
		t.Errorf("after three digits the field draws %q", got)
	}
	// The next literal is skipped, and the letter is refused.
	r.typeText(") a1234567")
	if sig.Get() != "5551234567" {
		t.Fatalf("the value is %q", sig.Get())
	}
	n := r.stateOf(t, access.RoleTextField, "Phone")
	if n.Value != "(555) 123-4567" {
		t.Errorf("the accessible value is %q", n.Value)
	}
	if n.Text == nil || n.Text.Content.Read(0, n.Text.Content.Len(), -1) != "(555) 123-4567" || n.Text.Caret != n.Text.Content.Len() {
		t.Errorf("the text content is %+v", n.Text)
	}
	r.typeText("8")
	if sig.Get() != "5551234567" {
		t.Errorf("typing into a full field gave %q", sig.Get())
	}
}

// Left moves one slot at a time, and Backspace deletes the slot before the caret.
func TestFormatFieldLeftAndBackspace(t *testing.T) {
	r, sig, _ := formatRig(t, phoneFormat, "", nil)
	r.typeText("5551234567")
	for range 4 {
		r.press(input.KeyLeft, 0)
	}
	if es := fieldEdit(t, r); es.caret != 6 {
		t.Fatalf("after four Lefts the caret is at %d, want 6", es.caret)
	}
	r.press(input.KeyBackspace, 0)
	if sig.Get() != "555124567" || fieldEdit(t, r).caret != 5 {
		t.Errorf("Backspace gave %q, caret %d", sig.Get(), fieldEdit(t, r).caret)
	}
	r.press(input.KeyEnd, 0)
	r.press(input.KeyBackspace, 0)
	if sig.Get() != "55512456" {
		t.Errorf("Backspace at the end gave %q", sig.Get())
	}
	// Ctrl+Left goes to the start of the group of slots.
	r.press(input.KeyLeft, input.ModCtrl)
	if es := fieldEdit(t, r); es.caret != 6 {
		t.Errorf("Ctrl+Left put the caret at %d, want 6", es.caret)
	}
	if got := r.stateOf(t, access.RoleTextField, "Phone").Value; got != "(555) 124-56" {
		t.Errorf("the field shows %q", got)
	}
}

// A click on a literal puts the caret before the slot after it.
func TestFormatFieldClickOnALiteral(t *testing.T) {
	r, _, _ := formatRig(t, phoneFormat, "5551234567", nil)
	ed := editorNode(t, r.rt)
	es := ed.st.edit
	rc := r.rt.nodeRect(ed)
	a, _ := es.layout.Caret(len("(555"))
	b, _ := es.layout.Caret(len("(555)"))
	r.clickAt(rc.X+es.originX+(a+b)/2, rc.Y+rc.H/2)
	if es.caret != 3 || es.anchor != 3 {
		t.Errorf("a click on ) put the caret at %d, want 3", es.caret)
	}
	// A double-click selects the group of slots.
	x, _ := es.layout.Caret(len("(555) 1"))
	x += rc.X + es.originX
	r.rt.PointerDown(x, rc.Y+rc.H/2, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, rc.Y+rc.H/2, input.ButtonLeft, 0)
	r.rt.PointerDown(x, rc.Y+rc.H/2, input.ButtonLeft, 2, 0)
	r.rt.PointerUp(x, rc.Y+rc.H/2, input.ButtonLeft, 0)
	r.frame()
	if a, b := es.selection(); a != 3 || b != 6 {
		t.Errorf("a double-click selected %d to %d, want the group 3 to 6", a, b)
	}
}

// Typing over a selection is one undo step, and undo restores the raw value and
// the caret.
func TestFormatFieldUndo(t *testing.T) {
	r, sig, _ := formatRig(t, phoneFormat, "", nil)
	r.typeText("5551234567")
	r.now = r.now.Add(2 * time.Second)
	es := fieldEdit(t, r)
	es.anchor, es.caret = 3, 6
	r.typeText("9")
	if sig.Get() != "55594567" || es.caret != 4 {
		t.Fatalf("typing over the selection gave %q, caret %d", sig.Get(), es.caret)
	}
	r.press(input.KeyZ, input.ModCtrl)
	if sig.Get() != "5551234567" || es.anchor != 3 || es.caret != 6 {
		t.Errorf("undo gave %q, selection %d to %d", sig.Get(), es.anchor, es.caret)
	}
	r.press(input.KeyY, input.ModCtrl)
	if sig.Get() != "55594567" || es.caret != 4 {
		t.Errorf("redo gave %q, caret %d", sig.Get(), es.caret)
	}
}

// An edit that Accept refuses changes nothing: not the value, not the caret, no
// OnChange and no undo step. The format's own refusals are the same.
func TestFormatFieldAccept(t *testing.T) {
	r, sig, changes := formatRig(t, phoneFormat, "", func(ti *TextInputElement) *TextInputElement {
		return ti.Accept(func(next string) bool { return !strings.HasPrefix(next, "0") })
	})
	r.typeText("0")
	r.typeText(")")
	r.typeText("x")
	es := fieldEdit(t, r)
	if sig.Get() != "" || len(*changes) != 0 || len(es.undo) != 0 || es.caret != 0 {
		t.Fatalf("refused edits gave %q, OnChange %q, %d undo steps, caret %d", sig.Get(), *changes, len(es.undo), es.caret)
	}
	r.typeText("55")
	r.press(input.KeyHome, 0)
	r.typeText("0")
	if sig.Get() != "55" || es.caret != 0 || len(*changes) != 2 {
		t.Errorf("a refused insert gave %q, caret %d, OnChange %q", sig.Get(), es.caret, *changes)
	}
	// Accept without a format filters what plain text can become.
	digits := NewSignal("")
	r2 := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(TextInput(digits).Label("Digits").Key("d").
			Accept(func(next string) bool { return strings.Trim(next, "0123456789") == "" }))
	})
	r2.focusKey(t, "d")
	r2.typeText("1a2")
	if digits.Get() != "12" {
		t.Errorf("Accept without a format let through %q", digits.Get())
	}
}

// Paste takes what fits from the clipboard; copy copies the text as shown; cut
// removes the raw range.
func TestFormatFieldClipboard(t *testing.T) {
	r, sig, _ := formatRig(t, phoneFormat, "", nil)
	r.rt.memClipboard = "555.123.4567"
	r.press(input.KeyV, input.ModCtrl)
	if sig.Get() != "5551234567" {
		t.Fatalf("paste gave %q", sig.Get())
	}
	r.press(input.KeyA, input.ModCtrl)
	r.press(input.KeyC, input.ModCtrl)
	if r.rt.memClipboard != "(555) 123-4567" {
		t.Errorf("copy put %q on the clipboard", r.rt.memClipboard)
	}
	es := fieldEdit(t, r)
	es.anchor, es.caret = 3, 6
	r.press(input.KeyX, input.ModCtrl)
	if r.rt.memClipboard != "123" || sig.Get() != "5554567" {
		t.Errorf("cut put %q on the clipboard and left %q", r.rt.memClipboard, sig.Get())
	}
	r.rt.memClipboard = "abc"
	r.press(input.KeyV, input.ModCtrl)
	if sig.Get() != "5554567" {
		t.Errorf("a paste with nothing that fits gave %q", sig.Get())
	}
}

// Assistive technology sets the value as a paste of the whole text, and selects in
// the offsets of the text shown.
func TestFormatFieldAccess(t *testing.T) {
	r, sig, _ := formatRig(t, phoneFormat, "", nil)
	field := accesstest.Find(t, r.tree(), access.RoleTextField, "Phone")
	r.bridge.Do(field.ID, access.Request{Action: access.ActionSetValue, Value: "(555) 123-4567"})
	r.frame()
	if sig.Get() != "5551234567" {
		t.Fatalf("SetValue gave %q", sig.Get())
	}
	r.bridge.Do(field.ID, access.Request{Action: access.ActionSetValue, Value: "none"})
	r.frame()
	if sig.Get() != "5551234567" {
		t.Errorf("a SetValue with nothing that fits gave %q", sig.Get())
	}
	// "123" is bytes 6 to 9 of the text shown, and 3 to 6 of the value.
	r.bridge.Do(field.ID, access.Request{Action: access.ActionSetTextSelection, Anchor: 6, Caret: 9})
	r.frame()
	if es := fieldEdit(t, r); es.anchor != 3 || es.caret != 6 {
		t.Errorf("the selection is %d to %d, want 3 to 6", es.anchor, es.caret)
	}
	// The caret at the end of it is drawn before the slot after it, after the "-".
	ti := textOf(t, r, access.RoleTextField, "Phone")
	if sel := ti.Selection(); sel.Start != 6 || sel.End != 10 {
		t.Errorf("the text selection is points %d to %d, want 6 to 10", sel.Start, sel.End)
	}
	r.bridge.Do(field.ID, access.Request{Action: access.ActionSetValue, Value: ""})
	r.frame()
	if sig.Get() != "" {
		t.Errorf("SetValue with the empty text gave %q", sig.Get())
	}
}

// An input method's composition shows in the text shown, at the caret, and its
// full-width digits are typed as ASCII; a result that doesn't fit types nothing.
func TestFormatFieldComposition(t *testing.T) {
	r, sig, _ := formatRig(t, phoneFormat, "555", nil)
	r.press(input.KeyEnd, 0)
	r.rt.hostCompose(composition{text: "１", cursor: len("１")})
	r.frame()
	if got := fieldEdit(t, r).layout.Text; got != "(555) １" {
		t.Errorf("the composition shows as %q", got)
	}
	r.rt.commitComposition()
	r.frame()
	if sig.Get() != "5551" {
		t.Fatalf("the committed composition gave %q", sig.Get())
	}
	r.rt.hostCompose(composition{text: "５５x", cursor: len("５５x")})
	r.rt.commitComposition()
	r.frame()
	if sig.Get() != "5551" {
		t.Errorf("a composition that doesn't fit gave %q", sig.Get())
	}
	r.rt.hostCompose(composition{text: "２３", cursor: len("２３")})
	r.rt.commitComposition()
	r.frame()
	if sig.Get() != "555123" {
		t.Errorf("full-width digits gave %q", sig.Get())
	}
}

// A masked field with a format takes only what fits, and draws a bullet for each
// character without the literals.
func TestFormatFieldMasked(t *testing.T) {
	r, sig, _ := formatRig(t, TemplateFormat("99-99"), "", func(ti *TextInputElement) *TextInputElement { return ti.Masked() })
	r.typeText("12")
	r.typeText("a")
	r.typeText("345")
	if sig.Get() != "1234" {
		t.Fatalf("the value is %q", sig.Get())
	}
	if got := fieldEdit(t, r).layout.Text; got != "••••" {
		t.Errorf("the field draws %q, want four bullets", got)
	}
	if got := r.stateOf(t, access.RoleTextField, "Phone").Value; got != "••••" {
		t.Errorf("the accessible value is %q", got)
	}
	if got := editorNode(t, r.rt).editor.prompt(sig.Get(), true); got != "" {
		t.Errorf("a masked field has the prompt %q", got)
	}
}

// The clear button shows while the raw value has text, and Escape clears it.
func TestFormatFieldClearable(t *testing.T) {
	r, sig, _ := formatRig(t, phoneFormat, "", func(ti *TextInputElement) *TextInputElement { return ti.Clearable() })
	accesstest.Absent(t, r.tree(), access.RoleButton, "Clear")
	r.typeText("55")
	accesstest.Find(t, r.tree(), access.RoleButton, "Clear")
	r.press(input.KeyEscape, 0)
	if sig.Get() != "" {
		t.Errorf("Escape left %q", sig.Get())
	}
}

// In a Field, Required and Complete see the raw value: a partly filled number is
// incomplete when the user leaves the field.
func TestFormatFieldValidation(t *testing.T) {
	useLocale(t, testLocaleEN())
	sig := NewSignal("")
	const msg = "Enter the whole phone number."
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(8).
			Child(Field("Phone", TextInput(sig).Key("phone").Format(phoneFormat).
				Validate(Required[string](), phoneFormat.Complete(msg)))).
			Child(Button("Other").Key("other"))
	})
	r.focusKey(t, "phone")
	r.typeText("555")
	r.focusKey(t, "other")
	if !hasName(r, msg) {
		t.Fatal("no error for a partly filled phone number")
	}
	r.focusKey(t, "phone")
	r.press(input.KeyEnd, 0)
	r.typeText("1234567")
	r.focusKey(t, "other")
	if hasName(r, msg) || hasName(r, englishStrings().Required) {
		t.Error("an error for a whole phone number")
	}
}

// Leaving a number field puts the number in its shortest form, as an undo step.
func TestFormatFieldNormalizesOnBlur(t *testing.T) {
	r, sig, changes := formatRig(t, moneyFormat, "", nil)
	r.typeText("007")
	r.focusKey(t, "other")
	if sig.Get() != "7" || (*changes)[len(*changes)-1] != "7" {
		t.Fatalf("leaving the field gave %q, OnChange %q", sig.Get(), *changes)
	}
	r.focusKey(t, "phone")
	r.press(input.KeyZ, input.ModCtrl)
	if sig.Get() != "007" {
		t.Errorf("undo after leaving gave %q", sig.Get())
	}
	r.press(input.KeyA, input.ModCtrl)
	r.typeText("-")
	r.focusKey(t, "other")
	if sig.Get() != "" {
		t.Errorf("leaving the field with a minus alone gave %q", sig.Get())
	}
}

// A focused empty template shows the prompts; an unfocused one shows the
// placeholder, or the prompts when it has none.
func TestFormatFieldPlaceholderAndPrompts(t *testing.T) {
	r, _, _ := formatRig(t, phoneFormat, "", func(ti *TextInputElement) *TextInputElement { return ti.Placeholder("Phone number") })
	e := editorNode(t, r.rt).editor
	if got := e.prompt("", true); got != "(___) ___-____" {
		t.Errorf("focused and empty, the prompt is %q", got)
	}
	if got := e.prompt("", false); got != "" {
		t.Errorf("unfocused with a placeholder, the prompt is %q", got)
	}
	if got := e.prompt("55", false); got != "_) ___-____" {
		t.Errorf("unfocused with text, the prompt is %q", got)
	}
	r2, _, _ := formatRig(t, phoneFormat, "", nil)
	if got := editorNode(t, r2.rt).editor.prompt("", false); got != "(___) ___-____" {
		t.Errorf("unfocused without a placeholder, the prompt is %q", got)
	}
	// The prompts are not in what assistive technology reads, or in the layout.
	if got := r.stateOf(t, access.RoleTextField, "Phone").Value; got != "" {
		t.Errorf("the empty field reads as %q", got)
	}
}

// A number field groups as the user types, and the minus toggles the sign.
func TestFormatFieldNumber(t *testing.T) {
	r, sig, _ := formatRig(t, moneyFormat, "", nil)
	r.typeText("1234")
	r.typeText(",")
	r.typeText("5")
	if sig.Get() != "12345" {
		t.Fatalf("the value is %q", sig.Get())
	}
	if got := fieldEdit(t, r).layout.Text; got != "12,345" {
		t.Errorf("the field draws %q", got)
	}
	r.press(input.KeyLeft, 0)
	r.typeText("-")
	if sig.Get() != "-12345" || fieldEdit(t, r).caret != 5 {
		t.Errorf("minus gave %q, caret %d", sig.Get(), fieldEdit(t, r).caret)
	}
	r.press(input.KeyEnd, 0)
	r.typeText(".256")
	if sig.Get() != "-12345.25" {
		t.Errorf("the decimals gave %q", sig.Get())
	}
	if got := r.stateOf(t, access.RoleTextField, "Phone").Value; got != "-12,345.25" {
		t.Errorf("the accessible value is %q", got)
	}
}

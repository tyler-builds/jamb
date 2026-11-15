package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// textHost is the host side of a runtime that a window whose input methods read
// and edit the text around the caret uses (see ui.hostTextContext).
type textHost interface {
	TextContext() uihost.TextContext
	TextEdit(start, end int, s string)
	TextSelect(anchor, caret int)
}

func hostText(u *uitest.UI) textHost { return uihost.Of(u.Runtime()).(textHost) }

// The purposes and Enter keys as platform.TextPurpose and platform.EnterKey
// number them (app checks that they agree).
const (
	purposeText, purposeVerbatim, purposePassword, purposeEmail = 0, 1, 2, 3
	purposeDecimal, purposeNumber                               = 6, 5
	enterNewLine, enterDone, enterGo, enterSearch               = 0, 1, 2, 4
)

// The focused field's text and selection, and what it holds, are reported for
// input methods; a field whose text an input method must not edit as it is
// reports what it holds but not its text.
func TestTextContextOfFields(t *testing.T) {
	msg, name, secret, phone, query := ui.NewSignal("Hello there"), ui.NewSignal("jamb"), ui.NewSignal("hunter2"),
		ui.NewSignal("555"), ui.NewSignal("")
	n := ui.NewSignal(1.5)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Children(
			ui.TextArea(msg).Key("msg").Label("Message"),
			ui.TextInput(name).Key("name").Label("Name").Purpose(ui.PurposeVerbatim).OnSubmit(func(string) {}),
			ui.TextInput(secret).Key("secret").Label("Password").Masked(),
			ui.TextInput(phone).Key("phone").Label("Phone").Format(ui.TemplateFormat("(999) 999-9999")),
			ui.TextInput(query).Key("query").Label("Search").Purpose(ui.PurposeSearch),
			ui.NumberInput(n, 0, 10, 0.5).Key("n").Label("Amount"),
		)
	})
	h := hostText(u)
	if c := h.TextContext(); c.Focused {
		t.Errorf("with no field focused, the context is %+v", c)
	}
	u.Get("msg").Focus()
	c := h.TextContext()
	if !c.Focused || !c.HasText || c.Text != "Hello there" || c.Purpose != purposeText || !c.Multiline || c.Enter != enterNewLine {
		t.Errorf("a text area's context is %+v; want its text, as prose, taking new lines", c)
	}
	for _, f := range []struct {
		key           string
		hasText       bool
		purpose, ente uint8
	}{
		{"name", true, purposeVerbatim, enterGo},
		{"secret", false, purposePassword, enterDone},
		{"phone", false, purposeVerbatim, enterDone},
		{"query", true, purposeText + 8, enterSearch}, // PurposeSearch
	} {
		u.Get(f.key).Focus()
		c := h.TextContext()
		if c.HasText != f.hasText || c.Purpose != f.purpose || c.Enter != f.ente || c.Multiline {
			t.Errorf("%s: context %+v; want text shown %v, purpose %d, Enter %d", f.key, c, f.hasText, f.purpose, f.ente)
		}
	}
	u.Get("n").Click()
	if c := h.TextContext(); c.Purpose != purposeDecimal {
		t.Errorf("a float NumberInput's purpose is %d, want a decimal number", c.Purpose)
	}
}

// An input method's edits replace a range of the field's text and put the caret
// after the new text: a correction, the period of a double space, a suggestion
// that completes a word, a deletion of a word. Its selection moves the caret.
func TestTextEditsFromInputMethods(t *testing.T) {
	v := ui.NewSignal("")
	changes := 0
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Child(ui.TextArea(v).Key("msg").Label("Message").OnChange(func(string) { changes++ }))
	})
	u.Get("msg").Focus()
	h := hostText(u)
	step := func(what string, edit func(), want string, caret int) {
		t.Helper()
		edit()
		u.Frame()
		c := h.TextContext()
		if v.Get() != want || c.Text != want || c.Anchor != caret || c.Caret != caret {
			t.Fatalf("%s: text %q, selection %d-%d; want %q with the caret at %d", what, c.Text, c.Anchor, c.Caret, want, caret)
		}
	}
	step("typing", func() { h.TextEdit(0, 0, "I went to teh") }, "I went to teh", 13)
	step("a correction", func() { h.TextEdit(10, 13, "the") }, "I went to the", 13)
	step("a space", func() { h.TextEdit(13, 13, " ") }, "I went to the ", 14)
	step("a double space", func() { h.TextEdit(13, 14, ". ") }, "I went to the. ", 15)
	step("a suggestion", func() { h.TextEdit(15, 15, "Wor") }, "I went to the. Wor", 18)
	step("its completion", func() { h.TextEdit(15, 18, "World ") }, "I went to the. World ", 21)
	step("a word deleted", func() { h.TextEdit(15, 21, "") }, "I went to the. ", 15)
	step("the caret moved", func() { h.TextSelect(2, 2) }, "I went to the. ", 2)
	if changes != 7 {
		t.Errorf("OnChange ran %d times, want once for each edit", changes)
	}
	h.TextSelect(2, 6)
	u.Frame()
	if c := h.TextContext(); c.Anchor != 2 || c.Caret != 6 {
		t.Errorf("a selection made %d-%d, want 2-6", c.Anchor, c.Caret)
	}
	// Out of range and inside a character, an edit is clamped to the text and
	// to whole characters.
	v.Set("é")
	u.Frame()
	h.TextEdit(1, 99, "e")
	u.Frame()
	if v.Get() != "e" {
		t.Errorf("an edit inside a character gave %q, want the character replaced", v.Get())
	}
}

// Undo takes back an input method's correction on its own, and its typing with
// the typing around it.
func TestTextEditsUndo(t *testing.T) {
	v := ui.NewSignal("")
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Child(ui.TextArea(v).Key("msg").Label("Message"))
	})
	u.Get("msg").Focus()
	h := hostText(u)
	for i, c := range "teh" {
		h.TextEdit(i, i, string(c))
	}
	h.TextEdit(0, 3, "the")
	u.Frame()
	u.Press("ctrl+z")
	if v.Get() != "teh" {
		t.Errorf("undo after a correction gave %q, want the word as typed", v.Get())
	}
	u.Press("ctrl+z")
	if v.Get() != "" {
		t.Errorf("a second undo gave %q, want the typing undone", v.Get())
	}
}

// A single-line field types a new line of an input method's edit as a space, and
// a masked or read-only field takes no edits.
func TestTextEditsInSingleLineAndLockedFields(t *testing.T) {
	one, secret, ro := ui.NewSignal(""), ui.NewSignal("abc"), ui.NewSignal("fixed")
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Children(
			ui.TextInput(one).Key("one").Label("One"),
			ui.TextInput(secret).Key("secret").Label("Secret").Masked(),
			ui.TextInput(ro).Key("ro").Label("Read-only").ReadOnly(true),
		)
	})
	h := hostText(u)
	u.Get("one").Focus()
	h.TextEdit(0, 0, "a\nb")
	u.Get("secret").Focus()
	h.TextEdit(0, 3, "xyz")
	u.Get("ro").Focus()
	h.TextEdit(0, 5, "changed")
	u.Frame()
	if one.Get() != "a b" || secret.Get() != "abc" || ro.Get() != "fixed" {
		t.Errorf("fields %q, %q, %q; want a b, and the masked and read-only ones unchanged", one.Get(), secret.Get(), ro.Get())
	}
}

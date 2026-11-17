package ui_test

import (
	"strings"
	"testing"
	"unicode/utf8"

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

// The focused field reports where its text is and the faces it is drawn
// with, for a window that lays the same text out (a page's textarea on iOS):
// the box, its text size as jamb lays it out, whole pixels, and its face,
// with the missing-glyph font of the face.
func TestTextContextGeometryAndFaces(t *testing.T) {
	msg := ui.NewSignal("Hello there")
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(10).Child(ui.TextArea(msg).Key("msg").Label("Message").Style(ui.Style{FontSize: 13.5}))
	})
	u.Get("msg").Focus()
	u.Frame()
	c := hostText(u).TextContext()
	if c.Box[2] <= 0 || c.Box[3] <= 0 || c.LineHeight <= 0 {
		t.Fatalf("the text area reports the box %v and line height %v", c.Box, c.LineHeight)
	}
	if c.FontSize != 14 {
		t.Errorf("text of 13.5 reports the size %v, want 14, as it is laid out", c.FontSize)
	}
	if len(c.Faces) != 1 || len(c.Faces[0].Data) == 0 || !strings.HasPrefix(c.Faces[0].Key, "jamb-face-") ||
		!strings.HasPrefix(c.Faces[0].NotdefKey, "jamb-notdef-") || len(c.Faces[0].Notdef) == 0 {
		t.Errorf("the text area reports %d faces, want its own, with its file and its missing-glyph font", len(c.Faces))
	}
}

// A field reports its text as runs that cover it in order: the syntax that its
// marks hide as runs of no face and no width, a tab as a run of its own, and
// the rest in the faces it is drawn in; and where it puts the caret.
func TestTextContextRuns(t *testing.T) {
	v := "Use `code` and **bold**\ttab\nnext 😀 日本"
	msg := ui.NewSignal(v)
	marks := func(s string) []ui.TextMark {
		var out []ui.TextMark
		if i := strings.Index(s, "`code`"); i >= 0 {
			out = append(out, ui.TextMark{Start: i, End: i + 1, Syntax: true, ScopeStart: i, ScopeEnd: i + 6},
				ui.TextMark{Start: i + 1, End: i + 5, Style: ui.Highlight{Mono: true}},
				ui.TextMark{Start: i + 5, End: i + 6, Syntax: true, ScopeStart: i, ScopeEnd: i + 6})
		}
		if i := strings.Index(s, "**bold**"); i >= 0 {
			out = append(out, ui.TextMark{Start: i, End: i + 2, Syntax: true, ScopeStart: i, ScopeEnd: i + 8},
				ui.TextMark{Start: i + 2, End: i + 6, Style: ui.Highlight{Weight: ui.FontBold}},
				ui.TextMark{Start: i + 6, End: i + 8, Syntax: true, ScopeStart: i, ScopeEnd: i + 8})
		}
		return out
	}
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(10).Child(ui.TextArea(msg).Key("msg").Label("Message").Marks(marks))
	})
	u.Get("msg").Focus()
	h := hostText(u)
	h.TextSelect(len(v), len(v)) // the caret at the end: the syntax before it hides
	u.Frame()
	c := h.TextContext()
	if len(c.Runs) == 0 || c.Carets == nil {
		t.Fatalf("the field reports %d runs, carets %v", len(c.Runs), c.Carets != nil)
	}
	at := 0
	byText := map[string]uihost.TextRun{}
	for _, r := range c.Runs {
		if r.Start != at || r.End <= r.Start {
			t.Fatalf("a run covers %d to %d after the runs reached %d", r.Start, r.End, at)
		}
		at = r.End
		byText[v[r.Start:r.End]] = r
	}
	if at != len(v) {
		t.Fatalf("the runs end at %d of %d", at, len(v))
	}
	for _, s := range []string{"`", "**"} {
		if r, ok := byText[s]; !ok || r.Face != -1 || r.Width != 0 {
			t.Errorf("the hidden %q is the run %+v (found %v), want one of no face and no width", s, r, ok)
		}
	}
	if r := byText["\t"]; r.Kind != 1 || r.Width <= 0 {
		t.Errorf("the tab is the run %+v, want a tab run with its width", r)
	}
	code, bold, plain := byText["code"], byText["bold"], byText["Use "]
	if code.Face == plain.Face || bold.Face == plain.Face || code.Face < 0 || code.Face >= len(c.Faces) {
		t.Errorf("code is in face %d, bold in %d, plain text in %d of %d; want each in its own", code.Face, bold.Face, plain.Face, len(c.Faces))
	}
	if r := byText["\n"]; r.Kind != 2 {
		t.Errorf("the line break is the run %+v, want a break", r)
	}
	carets := c.Carets(0, len(v))
	if n := utf8.RuneCountInString(v) + 1; len(carets) != n || carets[len(carets)-1].Line != 1 {
		t.Errorf("the field gives %d carets, the last on line %d; want one at each of %d boundaries, the last on the second line",
			len(carets), carets[len(carets)-1].Line, n)
	}
	if c.WrapWidth <= 0 {
		t.Errorf("a text area's lines wrap at %v", c.WrapWidth)
	}
}

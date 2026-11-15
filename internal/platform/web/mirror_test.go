//go:build js

package web

import (
	"strings"
	"syscall/js"
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
)

func TestUTF16Offsets(t *testing.T) {
	s := "aé😀b" // 1, 2, 4 and 1 bytes; 1, 1, 2 and 1 UTF-16 code units
	if n := utf16Len(s); n != 5 {
		t.Errorf("utf16Len = %d, want 5", n)
	}
	for _, c := range []struct{ bytes, units int }{{0, 0}, {1, 1}, {3, 2}, {7, 4}, {8, 5}} {
		if got := utf16Offset(s, c.bytes); got != c.units {
			t.Errorf("utf16Offset(%d) = %d, want %d", c.bytes, got, c.units)
		}
		if got := byteOffset(s, c.units); got != c.bytes {
			t.Errorf("byteOffset(%d) = %d, want %d", c.units, got, c.bytes)
		}
	}
	if got := byteOffset(s, 3); got != 3 {
		t.Errorf("an offset inside a surrogate pair is byte %d, want the start of its character", got)
	}
	if got := byteOffset(s, 99); got != len(s) {
		t.Errorf("an offset past the end is byte %d, want the end", got)
	}
}

func TestDiffText(t *testing.T) {
	for _, c := range []struct {
		old, new string
		at       int // the caret in new
		a, b     int
		ins      string
	}{
		{"hello", "hello!", 6, 5, 5, "!"},
		{"teh ", "the ", 4, 1, 4, "he "},                      // a correction, with the space after it
		{"Hello ", "Hello. ", 7, 5, 6, ". "},                  // a double space: ". " for " "
		{"aa", "aaa", 1, 0, 0, "a"},                           // typed at the start of a run
		{"abbb", "abb", 2, 2, 3, ""},                          // the second b deleted, as the caret says
		{"a😀b", "a😁b", 5, 1, 5, "😁"},                          // whole characters, though they share bytes
		{"résumé", "resume", 6, 1, 8, "esume"},                // é and e differ in their first byte
		{"hello world", "hello ", 6, 6, 11, ""},               // a word deleted
		{"see omw", "see On my way!", 14, 4, 7, "On my way!"}, // a text replacement
	} {
		a, b, ins, same := diffText(c.old, c.new, c.at)
		if same || a != c.a || b != c.b || ins != c.ins {
			t.Errorf("diffText(%q, %q) = %d, %d, %q; want %d, %d, %q", c.old, c.new, a, b, ins, c.a, c.b, c.ins)
		}
		if got := c.old[:a] + ins + c.old[b:]; got != c.new {
			t.Errorf("the edit of %q gives %q, want %q", c.old, got, c.new)
		}
	}
	if _, _, _, same := diffText("x", "x", 1); !same {
		t.Error("the same text differs")
	}
}

func TestMirrorWindow(t *testing.T) {
	var m mirror
	short := "a short message"
	if lo, hi := m.window(short, 3, 3); lo != 0 || hi != len(short) {
		t.Errorf("a short text's window is %d-%d, want all of it", lo, hi)
	}
	long := strings.Repeat("0123456789", 1000)
	lo, hi := m.window(long, 5000, 5000)
	if lo != 5000-mirrorBefore || hi != 5000+mirrorAfter {
		t.Fatalf("a long text's window is %d-%d, want %d either side of the caret", lo, hi, mirrorBefore)
	}
	// Typing moves the caret on: the window stays, so the textarea is not written.
	typed := long[:5000] + "x" + long[5000:]
	m = mirror{on: true, valid: true, base: lo, text: typed[lo : hi+1]} // as syncText leaves it
	if l, h := m.window(typed, 5001, 5001); l != lo || h != hi+1 {
		t.Errorf("after typing, the window is %d-%d; want it where it was, %d-%d", l, h, lo, hi+1)
	}
	// A caret near the window's edge moves it.
	if l, _ := m.window(long, lo+10, lo+10); l == lo {
		t.Errorf("a caret %d bytes from the window's start kept it", 10)
	}
	// The window starts and ends at whole characters.
	wide := strings.Repeat("é", 3000)
	l, h := (&mirror{}).window(wide, 3001, 3001)
	if l%2 != 0 || h%2 != 0 {
		t.Errorf("a window of %d-%d splits a character", l, h)
	}
}

func TestTextAttrs(t *testing.T) {
	for _, c := range []struct {
		ctx  platform.TextContext
		want textAttrs
	}{
		{platform.TextContext{Purpose: platform.PurposeText, Multiline: true}, textAttrs{"on", "sentences", "true", "text", "enter"}},
		{platform.TextContext{Purpose: platform.PurposeSearch, Enter: platform.EnterSearch}, textAttrs{"on", "sentences", "true", "search", "search"}},
		{platform.TextContext{Purpose: platform.PurposeVerbatim, Enter: platform.EnterGo}, textAttrs{"off", "none", "false", "text", "go"}},
		{platform.TextContext{Purpose: platform.PurposePassword, Enter: platform.EnterDone}, textAttrs{"off", "none", "false", "text", "done"}},
		{platform.TextContext{Purpose: platform.PurposeEmail}, textAttrs{"off", "none", "false", "email", "enter"}},
		{platform.TextContext{Purpose: platform.PurposeDecimal, Enter: platform.EnterSend}, textAttrs{"off", "none", "false", "decimal", "send"}},
	} {
		if got := attrsOf(c.ctx); got != c.want {
			t.Errorf("attrsOf(%+v) = %v, want %v", c.ctx, got, c.want)
		}
	}
}

// fakeArea is a textarea for tests: its value counts its writes, and replace
// edits it as the browser does for typing, a correction or a deletion.
const fakeArea = `({
	_v: "", writes: 0, selectionStart: 0, selectionEnd: 0, selectionDirection: "forward", attrs: {}, focuses: 0,
	get value() { return this._v },
	set value(v) { this._v = v; this.writes++; this.selectionStart = this.selectionEnd = v.length },
	setAttribute(n, v) { this.attrs[n] = v },
	setSelectionRange(s, e, d) { this.selectionStart = s; this.selectionEnd = e; this.selectionDirection = d || "forward" },
	focus() { this.focuses++ }, blur() {},
	replace(s, e, t) {
		this._v = this._v.slice(0, s) + t + this._v.slice(e);
		this.selectionStart = this.selectionEnd = s + t.length;
	},
})`

// textApp is an app with one text field: it applies the window's edits and
// selections to it, and keeps its composition, as ui's editor does.
type textApp struct {
	platform.Handler
	text          string
	anchor, caret int
	comp          string
	keys          []input.Key
	purpose       platform.TextPurpose
	noText        bool // a masked field's: its text is not shown
}

func (a *textApp) OnEvent(_ platform.Window, e platform.Event) {
	switch e.Kind {
	case platform.EventTextEdit:
		a.text = a.text[:e.Start] + e.Text + a.text[e.End:]
		a.anchor, a.caret = e.Start+len(e.Text), e.Start+len(e.Text)
	case platform.EventTextSelect:
		a.anchor, a.caret = e.Start, e.End
	case platform.EventComposition:
		a.comp = e.Text
	case platform.EventTextInput:
		a.text = a.text[:a.caret] + e.Text + a.text[a.caret:]
		a.caret += len(e.Text)
		a.anchor = a.caret
	case platform.EventKeyDown:
		a.keys = append(a.keys, e.Key)
	}
}

func (a *textApp) OnFrame(platform.Window) bool { return false }

func (a *textApp) context() platform.TextContext {
	return platform.TextContext{Focused: true, Text: a.text, Anchor: a.anchor, Caret: a.caret, HasText: !a.noText, Purpose: a.purpose,
		Multiline: true}
}

// textPage is a page whose textarea has the focus, for a text field of app.
type textPage struct {
	t    *testing.T
	w    *window
	area js.Value
	app  *textApp
}

func newTextPage(t *testing.T, text string) *textPage {
	w, _ := touchWindow(t)
	withPage(t, w)
	area := js.Global().Call("eval", fakeArea)
	w.text.area, w.text.focused = area, true
	app := &textApp{text: text, anchor: len(text), caret: len(text)}
	w.handler = app
	p := &textPage{t, w, area, app}
	p.frame()
	return p
}

// frame runs what the window posted, and a frame of the app, which tells the
// window of the field.
func (p *textPage) frame() {
	runPosted()
	p.w.SetTextContext(p.app.context())
}

// browser edits the textarea from start to end, in UTF-16 code units, as the
// browser does for its input event, and runs the event and a frame.
func (p *textPage) browser(start, end int, text string) {
	p.area.Call("replace", start, end, text)
	p.input(false)
}

// input runs the textarea's input event, or the end of a composition, and a frame.
func (p *textPage) input(ended bool) {
	a := p.area
	if p.w.areaInput(a.Get("value").String(), a.Get("selectionStart").Int(), a.Get("selectionEnd").Int(),
		a.Get("selectionDirection").String() == "backward", ended) {
		a.Set("value", "")
	}
	p.frame()
}

func (p *textPage) value() string { return p.area.Get("value").String() }
func (p *textPage) writes() int   { return p.area.Get("writes").Int() }

// check fails the test unless the app's field holds want, with the caret at
// its end, and the textarea holds it too, with its caret there.
func (p *textPage) check(what, want string) {
	p.t.Helper()
	if p.app.text != want || p.app.caret != len(want) || p.app.anchor != p.app.caret {
		p.t.Fatalf("%s: the field holds %q with its selection at %d-%d; want %q with the caret at its end",
			what, p.app.text, p.app.anchor, p.app.caret, want)
	}
	if v := p.value(); v != want || p.area.Get("selectionEnd").Int() != utf16Len(want) {
		p.t.Fatalf("%s: the textarea holds %q with its caret at %d; want %q with the caret at its end",
			what, v, p.area.Get("selectionEnd").Int(), want)
	}
}

// The textarea holds the field's text and selection, so a phone's keyboard
// can correct a word, put a period for a double space or complete a word.
// What the keyboard does arrives as an edit of the textarea, which reaches the
// field as the edit of its text that it is; the field's text then comes back
// the same, and the textarea is not written again.
func TestTextareaMirrorsField(t *testing.T) {
	p := newTextPage(t, "")
	if p.writes() != 1 || p.area.Get("attrs").Get("autocorrect").String() != "on" ||
		p.area.Get("attrs").Get("autocapitalize").String() != "sentences" {
		t.Fatalf("a prose field gave the textarea %d writes and attributes %v",
			p.writes(), js.Global().Get("JSON").Call("stringify", p.area.Get("attrs")))
	}
	written := p.writes()
	for _, s := range []struct {
		what       string
		start, end int
		text, want string
	}{
		{"typing", 0, 0, "I", "I"},
		{"typing", 1, 1, " went to teh", "I went to teh"},
		{"an autocorrection, as insertReplacementText", 10, 13, "the", "I went to the"},
		{"a space", 13, 13, " ", "I went to the "},
		{"a double space's period", 13, 14, ". ", "I went to the. "},
		{"a capital, which the keyboard's shift typed", 15, 15, "W", "I went to the. W"},
		{"a word completed from the suggestions", 15, 16, "Wonderful ", "I went to the. Wonderful "},
		{"a word deleted, as deleteWordBackward", 15, 25, "", "I went to the. "},
		{"a text replacement shortcut", 15, 15, "omw", "I went to the. omw"},
		{"its expansion", 15, 18, "On my way!", "I went to the. On my way!"},
		{"a dictated phrase", 25, 25, " See you soon.", "I went to the. On my way! See you soon."},
		{"an emoji", 39, 39, " 😀", "I went to the. On my way! See you soon. 😀"},
		{"a character after it", 42, 42, "!", "I went to the. On my way! See you soon. 😀!"},
	} {
		p.browser(s.start, s.end, s.text)
		p.check(s.what, s.want)
	}
	if p.writes() != written {
		t.Errorf("the textarea was written %d times as the keyboard edited it; want none", p.writes()-written)
	}

	// The keyboard's cursor movement, as a long press of the space bar, moves
	// the field's caret.
	p.area.Call("setSelectionRange", 2, 6)
	p.w.areaSelect(2, 6, false)
	p.frame()
	if p.app.anchor != 2 || p.app.caret != 6 || p.writes() != written {
		t.Errorf("the keyboard selected 2-6; the field selected %d-%d, and the textarea was written %d times",
			p.app.anchor, p.app.caret, p.writes()-written)
	}
}

// A key that the app handles itself, as Backspace or Enter on a hardware
// keyboard, changes the field, whose text is written to the textarea in the
// next frame; so is a caret the app moves, as a tap does.
func TestTextareaFollowsTheApp(t *testing.T) {
	p := newTextPage(t, "Hello")
	p.app.text, p.app.anchor, p.app.caret = "Hell", 4, 4 // Backspace
	p.frame()
	p.check("Backspace", "Hell")
	p.app.anchor, p.app.caret = 1, 1 // a tap
	p.frame()
	if s := p.area.Get("selectionStart").Int(); s != 1 || p.value() != "Hell" {
		t.Errorf("after a tap at 1 the textarea holds %q with the caret at %d", p.value(), s)
	}
}

// An input method composes in the textarea: the field shows the composition
// at its caret, and the text it commits is typed into the field when it ends.
// A keyboard that composes the word at the caret again, as Android's does when
// the caret comes back to a word, takes the word out of the field while it
// composes it, and types it back when it ends.
func TestTextareaComposition(t *testing.T) {
	p := newTextPage(t, "Hi ")
	w := p.w
	w.composition(compositionStart, "")
	for _, s := range []string{"n", "ni", "nih"} {
		w.composition(compositionUpdate, s)
		p.area.Call("replace", 3, 3+len(s)-1, s)
		p.input(false)
		if p.app.comp != s || p.app.text != "Hi " {
			t.Fatalf("composing %q: the field shows %q at its caret, and holds %q", s, p.app.comp, p.app.text)
		}
	}
	w.composition(compositionUpdate, "你")
	p.area.Call("replace", 3, 6, "你")
	p.input(false)
	w.composition(compositionEnd, "你")
	p.input(true)
	if p.app.comp != "" {
		t.Errorf("the composition shows %q after it ended", p.app.comp)
	}
	p.check("a committed composition", "Hi 你")

	// The word at the caret composed again.
	p = newTextPage(t, "say hello")
	w = p.w
	w.composition(compositionStart, "")
	w.composition(compositionUpdate, "hello")
	p.input(false) // the textarea's text is the same: the composition is over the word
	if p.app.comp != "hello" || p.app.text != "say " || p.app.caret != 4 {
		t.Fatalf("the word composed again: the field holds %q, caret %d, showing %q", p.app.text, p.app.caret, p.app.comp)
	}
	w.composition(compositionUpdate, "help")
	p.area.Call("replace", 4, 9, "help")
	p.input(false)
	w.composition(compositionEnd, "help")
	p.input(true)
	p.check("a word composed again", "say help")
}

// A field that shows its text to no input method, as a masked one, gets an
// empty textarea, whose typed text is delivered and removed, and whose
// deletions are the app's keys.
func TestTextareaEmptyForMaskedField(t *testing.T) {
	w, _ := touchWindow(t)
	withPage(t, w)
	area := js.Global().Call("eval", fakeArea)
	w.text.area, w.text.focused = area, true
	app := &textApp{noText: true, purpose: platform.PurposePassword}
	w.handler = app
	w.SetTextContext(app.context())
	if area.Get("attrs").Get("autocorrect").String() != "off" {
		t.Error("a password's textarea corrects words")
	}
	area.Call("replace", 0, 0, "pw")
	if !w.areaInput(area.Get("value").String(), 2, 2, false, false) {
		t.Error("the textarea of a masked field is not emptied after typing")
	}
	if w.beforeInput("deleteContentBackward") != true {
		t.Error("a deletion in an empty textarea is not prevented")
	}
	runPosted()
	if app.text != "pw" || len(app.keys) != 1 || app.keys[0] != input.KeyBackspace {
		t.Errorf("the field got %q and keys %v; want the typed text and a Backspace", app.text, app.keys)
	}
}

// A mirroring textarea takes the deletions of a phone's keyboard itself, as of
// a word, and its new lines and undo are the app's keys.
func TestTextareaBeforeInput(t *testing.T) {
	p := newTextPage(t, "one two")
	for _, typ := range []string{"deleteContentBackward", "deleteWordBackward", "deleteSoftLineBackward",
		"insertText", "insertReplacementText"} {
		if p.w.beforeInput(typ) {
			t.Errorf("a mirroring textarea prevents %s", typ)
		}
	}
	for _, typ := range []string{"insertLineBreak", "historyUndo", "formatBold"} {
		if !p.w.beforeInput(typ) {
			t.Errorf("a mirroring textarea takes %s", typ)
		}
	}
	runPosted()
	if len(p.app.keys) != 2 || p.app.keys[0] != input.KeyEnter || p.app.keys[1] != input.KeyZ {
		t.Errorf("the app got keys %v, want Enter and Ctrl+Z", p.app.keys)
	}
}

// A long text's textarea holds the part around the caret, and the edits there
// reach the field at their place in the whole text.
func TestTextareaWindowOfLongText(t *testing.T) {
	text := strings.Repeat("word ", 2000) // 10,000 bytes
	p := newTextPage(t, text)
	base := p.w.text.mirror.base
	if base == 0 || len(p.value()) > mirrorBefore+mirrorAfter {
		t.Fatalf("the textarea holds %d bytes from %d of a %d-byte text", len(p.value()), base, len(text))
	}
	n := utf16Len(p.value())
	p.browser(n, n, "ok")
	if p.app.text != text+"ok" || p.app.caret != len(text)+2 {
		t.Errorf("typing at the end of a long text gave %q... with the caret at %d", p.app.text[len(text)-5:], p.app.caret)
	}
}

// After the app types the composition itself and cancels the input method's,
// the textarea's edits are ignored until the app's text is written to it.
func TestTextareaAfterCancelComposition(t *testing.T) {
	p := newTextPage(t, "ab")
	p.w.composition(compositionStart, "")
	p.w.composition(compositionUpdate, "c")
	p.browser(2, 2, "c")
	p.w.CancelComposition()
	p.app.text, p.app.anchor, p.app.caret, p.app.comp = "abc", 3, 3, "" // the app typed it
	p.area.Call("replace", 3, 3, "")                                    // the blur's input event
	p.w.areaInput(p.value(), 3, 3, false, false)
	runPosted()
	if p.app.text != "abc" {
		t.Errorf("the field holds %q after the composition was canceled, want abc", p.app.text)
	}
	p.frame()
	p.check("the next frame", "abc")
}

// A field of another purpose gives the textarea its attributes. A phone's
// keyboard reads them when the textarea takes the focus, so the textarea takes
// it again, but only in a tap or a key press, when a page may focus an element
// and keep the keyboard; and not for a focus that leaves the fields.
func TestTextareaAttributesForField(t *testing.T) {
	p := newTextPage(t, "")
	focuses := func() int { return p.area.Get("focuses").Int() }
	attr := func(name string) string { return p.area.Get("attrs").Get(name).String() }
	n := focuses()
	p.app.purpose = platform.PurposeVerbatim
	p.frame() // not in a gesture
	if attr("autocorrect") != "off" || attr("autocapitalize") != "none" || focuses() != n {
		t.Errorf("a verbatim field: autocorrect %s, autocapitalize %s, %d focuses", attr("autocorrect"),
			attr("autocapitalize"), focuses()-n)
	}
	p.app.purpose = platform.PurposeEmail
	inGesture(p.frame) // a tap
	if attr("inputmode") != "email" || focuses() != n+1 {
		t.Errorf("an email field in a tap: inputmode %s, %d focuses, want email and 1", attr("inputmode"), focuses()-n)
	}
	p.w.SetTextContext(platform.TextContext{}) // the focus left the fields
	if attr("inputmode") != "email" {
		t.Errorf("a focus that left the fields changed inputmode to %s", attr("inputmode"))
	}
}

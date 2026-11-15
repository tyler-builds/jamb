//go:build js

package web

import (
	"unicode/utf8"

	"github.com/tyler-builds/jamb/internal/platform"
)

// The textarea mirrors the focused text field: it holds the field's text
// around the caret, with the field's selection, so that a phone's keyboard
// sees what it needs to correct a word, capitalize a sentence, suggest the
// next word or put a period for a double space, and the keyboard's own
// cursor movement moves the field's caret. The browser edits the textarea
// itself, for typed text as for an input method's corrections; after each edit
// the window compares the textarea with what it held and sends the difference
// to the app (see syncText), and the app's text and selection come back in its
// next frame (see SetTextContext), where they are written to the textarea only
// if they differ from it, as after a key the app handled itself. A field that
// shows no text to input methods, as a masked one, gets an empty textarea,
// whose typed text is taken and removed after each edit (see textInput).

// The window of the field's text that the textarea holds: up to mirrorBefore
// bytes before the selection and mirrorAfter after it. The window stays where
// it is while the selection is at least mirrorMargin bytes from its ends, and
// no more than mirrorMargin further than that, so typing does not move it, and
// the textarea is written only when the app's text differs from it.
const (
	mirrorBefore = 2000
	mirrorAfter  = 2000
	mirrorMargin = 500
)

// mirror is what the textarea holds of the focused field.
type mirror struct {
	on bool // the textarea holds the field's text
	// valid is set while text and the selection are what the textarea holds.
	// It is cleared when the textarea changed in a way the window did not
	// follow (see CancelComposition): the textarea's edits are ignored until
	// the app's text is written to it again.
	valid bool
	base  int    // where text starts in the field's text, in bytes
	text  string // the field's text from base
	// anchor and caret are the ends of the selection, as byte offsets in text.
	anchor, caret int
}

// end returns where the mirror ends in the field's text.
func (m *mirror) end() int { return m.base + len(m.text) }

// window returns the part of the field's text s, from lo to hi, that the
// textarea holds for the selection from anchor to caret, and keeps where the
// mirror m is when the selection is still well inside it.
func (m *mirror) window(s string, anchor, caret int) (lo, hi int) {
	a, b := min(anchor, caret), max(anchor, caret)
	lo, hi = runeStart(s, max(a-mirrorBefore, 0)), runeStart(s, min(b+mirrorAfter, len(s)))
	if hi < len(s) && hi < b {
		hi = b // b is within s; runeStart moved hi before it only in a broken text
	}
	if !m.on || !m.valid {
		return lo, hi
	}
	if old := m.base; old <= a && old <= len(s) && runeStart(s, old) == old &&
		(old == 0 || a-old >= mirrorMargin) && a-old <= mirrorBefore+mirrorMargin {
		lo = old
	}
	if old := m.end(); old >= b && old <= len(s) && runeStart(s, old) == old &&
		(old == len(s) || old-b >= mirrorMargin) && old-b <= mirrorAfter+mirrorMargin {
		hi = old
	}
	return lo, hi
}

// runeStart returns i, moved back to the start of the character it is in, and
// clamped to s.
func runeStart(s string, i int) int {
	i = min(max(i, 0), len(s))
	for i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}

// utf16Len returns the length of s in UTF-16 code units, as JavaScript counts
// the length of a string and the offsets in it.
func utf16Len(s string) int { return utf16Offset(s, len(s)) }

// utf16Offset returns the byte offset i of s as an offset in UTF-16 code units.
func utf16Offset(s string, i int) int {
	n := 0
	for _, r := range s[:runeStart(s, i)] {
		n++
		if r >= 0x10000 {
			n++
		}
	}
	return n
}

// byteOffset returns the offset u of s in UTF-16 code units as a byte offset,
// at the start of a character: an offset inside a surrogate pair is the start
// of its character. It is clamped to s.
func byteOffset(s string, u int) int {
	n := 0
	for i, r := range s {
		w := 1
		if r >= 0x10000 {
			w = 2
		}
		if n+w > u {
			return i
		}
		n += w
	}
	return len(s)
}

// diffText returns how new differs from old: new is old with the bytes from a
// to b replaced by ins. The changed part is on whole characters, and ends at
// at, an offset in new where the caret is after the edit, when it can: that
// tells where in a run of the same character one was typed or deleted, and
// makes a double space's period the replacement of the space by ". " that it
// is. same reports that new is old.
func diffText(old, new string, at int) (a, b int, ins string, same bool) {
	if old == new {
		return 0, 0, "", true
	}
	at = min(max(at, 0), len(new))
	// The prefix ends where the new text before the caret starts, at most.
	n := min(len(old), len(new), max(at-max(len(new)-len(old), 0), 0))
	p := 0
	for p < n && old[p] == new[p] {
		p++
	}
	p = runeStart(new, p) // the same bytes before it in old: the same start
	// The suffix starts at the caret, at the earliest.
	sfx := 0
	for sfx < len(old)-p && sfx < len(new)-max(p, at) && old[len(old)-1-sfx] == new[len(new)-1-sfx] {
		sfx++
	}
	// The suffix starts at a character in both, as the bytes are the same.
	for sfx > 0 && !utf8.RuneStart(new[len(new)-sfx]) {
		sfx--
	}
	return p, len(old) - sfx, new[p : len(new)-sfx], false
}

// compositionIn returns where the composition data is in the textarea's
// text s, whose caret is at caret, and whose text before the composition was
// old: the composition ends at the caret, where an input method keeps it, or
// else it is what changed from old.
func compositionIn(s, old, data string, caret int) (start, end int) {
	if c := runeStart(s, caret); c >= len(data) && s[c-len(data):c] == data {
		return c - len(data), c
	}
	a, _, ins, same := diffText(old, s, caret)
	if same {
		return caret, caret
	}
	return a, a + len(ins)
}

// textAttrs are the textarea's attributes for the field it mirrors, which a
// phone's keyboard follows: autocorrect, autocapitalize, spellcheck, inputmode
// and enterkeyhint.
type textAttrs [5]string

// textAttrNames are the names of the attributes of textAttrs, in order.
var textAttrNames = [5]string{"autocorrect", "autocapitalize", "spellcheck", "inputmode", "enterkeyhint"}

// attrsOf returns the textarea's attributes for the field c.
func attrsOf(c platform.TextContext) textAttrs {
	enter := [...]string{platform.EnterNewLine: "enter", platform.EnterDone: "done", platform.EnterGo: "go",
		platform.EnterSend: "send", platform.EnterSearch: "search"}
	hint := "enter"
	if int(c.Enter) < len(enter) {
		hint = enter[c.Enter]
	}
	mode := "text"
	switch c.Purpose {
	case platform.PurposeText:
		return textAttrs{"on", "sentences", "true", mode, hint}
	case platform.PurposeSearch:
		return textAttrs{"on", "sentences", "true", "search", hint}
	case platform.PurposeEmail:
		mode = "email"
	case platform.PurposeURL:
		mode = "url"
	case platform.PurposeNumber:
		mode = "numeric"
	case platform.PurposeDecimal:
		mode = "decimal"
	case platform.PurposeTel:
		mode = "tel"
	}
	return textAttrs{"off", "none", "false", mode, hint}
}

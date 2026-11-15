package ui

import (
	"strings"

	"github.com/tyler-builds/jamb/internal/uihost"
)

// TextPurpose is what a text field holds. A phone's keyboard follows it: it
// corrects words and capitalizes sentences in prose, and types the rest as it
// is, with keys for what the field takes (see TextInputElement.Purpose).
type TextPurpose uint8

// The purposes of a text field. A Masked field is a password whatever its
// purpose: nothing is corrected, suggested or learned from it.
const (
	// PurposeText is prose, as a message or a note: words are corrected and
	// suggested, and sentences start with a capital. It is the default.
	PurposeText TextPurpose = iota
	// PurposeVerbatim is text typed as it is, such as a name, a command, a
	// path or code: nothing is corrected or capitalized.
	PurposeVerbatim
	purposePassword // a Masked field's
	// PurposeEmail is an email address, PurposeURL a web address, PurposeNumber
	// a whole number, PurposeDecimal a number with a decimal point and
	// PurposeTel a phone number: typed as they are, with their keys.
	PurposeEmail
	PurposeURL
	PurposeNumber
	PurposeDecimal
	PurposeTel
	// PurposeSearch is a search's query: prose, with a search key for Enter.
	PurposeSearch
)

// The keys that Enter is labeled with on a phone's keyboard, as
// platform.EnterKey numbers them.
const (
	enterNewLine uint8 = iota
	enterDone
	enterGo
	enterSend
	enterSearch
)

// Purpose says what the field holds, for a phone's keyboard: whether it corrects
// words and capitalizes sentences, and which keys it shows (see TextPurpose).
// The default is PurposeText. A field with a Format types as PurposeVerbatim
// unless it is given another.
func (ti *TextInputElement) Purpose(p TextPurpose) *TextInputElement {
	ti.purpose, ti.purposeSet = p, true
	return ti
}

// Purpose says what the text area holds; see TextInputElement.Purpose.
func (ta *TextAreaElement) Purpose(p TextPurpose) *TextAreaElement {
	ta.purpose, ta.purposeSet = p, true
	return ta
}

// hostTextContext reports the focused text field, its text and its selection,
// and what it holds, for a window whose input methods read and edit the text
// around the caret (see platform.TextContextWindow). A field shows its text
// only when an input method's edits can go into it as they are: not a masked
// field, one with a format, which changes what is typed, or a read-only one.
// An element that takes text input itself (see IME), such as a code editor,
// shows none and types verbatim.
func (rt *Runtime) hostTextContext() uihost.TextContext {
	n := rt.byID[rt.focusID]
	if n == nil {
		return uihost.TextContext{}
	}
	if n.editor == nil || n.st == nil || n.st.edit == nil {
		if n.div != nil && n.div.h.onTextInput != nil {
			return uihost.TextContext{Focused: true, Purpose: uint8(PurposeVerbatim), Multiline: true}
		}
		return uihost.TextContext{}
	}
	e := n.editor
	c := uihost.TextContext{Focused: true, Purpose: uint8(e.purpose), Multiline: e.multiline}
	switch {
	case e.multiline:
		c.Enter = enterNewLine
	case e.purpose == PurposeSearch:
		c.Enter = enterSearch
	case e.onSubmit != nil:
		c.Enter = enterGo
	default:
		c.Enter = enterDone
	}
	if e.password {
		c.Purpose = uint8(purposePassword)
	}
	if e.password || e.format != nil || e.locked() {
		return c
	}
	v := e.value.Peek()
	es := n.st.edit
	es.sync(v)
	c.Text, c.Anchor, c.Caret, c.HasText = v, es.anchor, es.caret, true
	return c
}

// hostTextEdit replaces the bytes from start to end of the focused field's text
// with s, and puts the caret after it: an input method's edit, such as a word it
// corrected or the period of a double space (see platform.EventTextEdit). The
// range is clamped to the text and to whole characters. Text typed at the caret
// groups with typing for undo, a deletion with deleting, and a replacement is an
// undo step of its own; the field's Accept is asked, as for typing.
func (rt *Runtime) hostTextEdit(start, end int, s string) {
	n, es, v := rt.editorOf(rt.focusID)
	if n == nil || n.editor.password || n.editor.format != nil || n.editor.locked() {
		return
	}
	start, end = clampToRune(v, start), clampToRune(v, end)
	if start > end {
		start, end = end, start
	}
	if !n.editor.multiline {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	kind := editOther
	switch {
	case start == end:
		kind = editTyping
	case s == "":
		kind = editDeleting
	}
	if start == end && s == "" {
		return
	}
	rt.editorSet(n.id, v[:start]+s+v[end:], start+len(s), kind)
	if strings.HasSuffix(s, " ") {
		es.lastKind = editOther // words are undo steps, as in editorInput
	}
}

// hostTextSelect selects from anchor to caret in the focused field, as byte
// offsets in its text, as an input method moves the caret (see
// platform.EventTextSelect). It does nothing while an input method composes.
func (rt *Runtime) hostTextSelect(anchor, caret int) {
	n, es, v := rt.editorOf(rt.focusID)
	if n == nil || rt.composing != 0 {
		return
	}
	anchor, caret = clampToRune(v, anchor), clampToRune(v, caret)
	if es.anchor == anchor && es.caret == caret {
		return
	}
	es.anchor, es.caret = anchor, caret
	es.havePrefX = false
	rt.editorTouched(n.id)
}

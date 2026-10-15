package text

import (
	"slices"
	"unicode"
)

// Word units follow the Windows convention, which Notepad, WordPad, Word and the
// edit controls of Edge and Chrome share (checked against Notepad, see
// docs/accessibility-design-phases-b-c.md, "Phase C"). A word unit is a run of word characters
// (letters, digits, underscore and combining marks), or a run of other visible
// characters such as punctuation, followed by any spaces and tabs after it. A line
// break is a unit of its own. So "A small note-taking app. Next" has the units
// "A ", "small ", "note", "-", "taking ", "app", ". ", "Next". The spaces before a
// line break belong to the word before it, and a line break is not a space of the
// unit before it.
//
// The editor moves the caret to the start of the next unit with Ctrl+Right, and to
// the start of the unit before the caret with Ctrl+Left. Ctrl+Delete and
// Ctrl+Backspace delete to the same places. UI Automation's word units are the same
// units, so a degenerate range moved by one word lands where Ctrl+Right lands.

// WordStarts returns the byte offsets at which the word units of s start, in
// order. It is empty for an empty s; otherwise its first element is 0.
func WordStarts(s string) []int {
	if s == "" {
		return nil
	}
	starts := []int{0}
	var prev rune
	for i, r := range s {
		if i > 0 && startsWordUnit(prev, r) {
			starts = append(starts, i)
		}
		prev = r
	}
	return starts
}

// startsWordUnit reports whether a character r that follows prev begins a new word
// unit. Spaces never begin one: they belong to the unit before them. A line break is
// a unit of its own, and a CR LF pair is one line break. A visible character begins
// a unit after a space, a line break, or a character of the other class.
func startsWordUnit(prev, r rune) bool {
	if prev == '\r' && r == '\n' {
		return false
	}
	if isLineBreak(r) || isLineBreak(prev) {
		return true
	}
	if unicode.IsSpace(r) {
		return false
	}
	if unicode.IsSpace(prev) {
		return true
	}
	return isWordRune(r) != isWordRune(prev)
}

// isLineBreak reports whether r ends a line.
func isLineBreak(r rune) bool { return r == '\n' || r == '\r' }

// NextWordStart returns the byte offset of the start of the first word unit after pos,
// or len(s) when there is none. From inside a unit it goes to the unit's end, which
// is where Ctrl+Right and Ctrl+Delete stop on Windows.
func NextWordStart(s string, pos int) int {
	starts := WordStarts(s)
	// Every start is a whole byte offset, so the first start after pos is the first
	// start at or after pos+1.
	if i, _ := slices.BinarySearch(starts, pos+1); i < len(starts) {
		return starts[i]
	}
	return len(s)
}

// PrevWordStart returns the byte offset of the start of the word unit that holds
// the character before pos, or 0 when pos is at the start of the text. It is Ctrl+Left
// on Windows: from inside a unit it goes to the unit's start, and from the start of a
// unit to the start of the unit before it.
func PrevWordStart(s string, pos int) int {
	if pos <= 0 {
		return 0
	}
	starts := WordStarts(s)
	i, _ := slices.BinarySearch(starts, pos)
	if i == 0 {
		return 0
	}
	return starts[i-1]
}

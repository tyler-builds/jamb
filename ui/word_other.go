//go:build !windows

package ui

import "github.com/tyler-builds/jamb/internal/text"

// The editor's word keys follow the convention of the platform. Outside Windows the
// keys stop at the end of the next word and at the start of the word before the
// caret, which is the convention that macOS and Linux text fields use, and the one
// the text package's NextWord and PrevWord implement (see text.WordStarts for the
// Windows convention).

// wordForward returns where Ctrl+Right, and Ctrl+Shift+Right, move from pos, and
// where Ctrl+Delete deletes to.
func wordForward(s string, pos int) int { return text.NextWord(s, pos) }

// wordBackward returns where Ctrl+Left, and Ctrl+Shift+Left, move from pos, and
// where Ctrl+Backspace deletes to.
func wordBackward(s string, pos int) int { return text.PrevWord(s, pos) }

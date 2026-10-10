//go:build windows

package ui

import "github.com/tyler-builds/jamb/text"

// The editor's word keys follow the convention of the platform. On Windows, as in
// Notepad, WordPad and the edit controls of Edge and Chrome, Ctrl+Right and
// Ctrl+Delete stop at the start of the next word unit, and Ctrl+Left and
// Ctrl+Backspace at the start of the unit before the caret (see text.WordStarts).

// wordForward returns where Ctrl+Right, and Ctrl+Shift+Right, move from pos, and
// where Ctrl+Delete deletes to.
func wordForward(s string, pos int) int { return text.NextWordStart(s, pos) }

// wordBackward returns where Ctrl+Left, and Ctrl+Shift+Left, move from pos, and
// where Ctrl+Backspace deletes to.
func wordBackward(s string, pos int) int { return text.PrevWordStart(s, pos) }

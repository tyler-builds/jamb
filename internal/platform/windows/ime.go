//go:build windows

package windows

import (
	"unicode/utf16"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

// Text fields draw what an input method is composing themselves, in place
// (see platform.EventComposition), as IMM32 lets a window do: the window hides
// the input method's composition window (WM_IME_SETCONTEXT), reads the
// composition on each WM_IME_COMPOSITION, and types the confirmed text itself,
// so the default window procedure does not also send it as WM_CHAR. The
// candidate list is still the input method's, placed next to the clause being
// converted (see SetTextInputRect).

// The ATTR_* values of a composition's attributes: how each unit of the text
// stands in the conversion.
const (
	attrTargetConverted    = 1
	attrTargetNotConverted = 3
)

// onComposition handles WM_IME_COMPOSITION, whose lParam is flags.
func (w *window) onComposition(flags uint32) {
	if w.imeCanceling {
		return
	}
	c := win32.ReadComposition(w.hwnd, flags)
	if flags&win32.GCS_RESULTSTR != 0 && len(c.Result) > 0 {
		w.emit(platform.Event{Kind: platform.EventTextInput, Text: string(utf16.Decode(c.Result))})
	}
	// The composition goes on, ends with the result just typed, or was
	// canceled (no flags at all).
	w.emit(compositionEvent(c.Text, c.Attrs, c.Cursor))
}

// onEndComposition handles WM_IME_ENDCOMPOSITION.
func (w *window) onEndComposition() {
	if !w.imeCanceling {
		w.emit(platform.Event{Kind: platform.EventComposition})
	}
}

// CancelComposition implements platform.Window.
func (w *window) CancelComposition() {
	w.imeCanceling = true
	win32.CancelComposition(w.hwnd)
	w.imeCanceling = false
}

// compositionEvent returns the EventComposition for a composition of UTF-16
// text, with attrs and the caret at unit cursor, converting the units to byte
// offsets. The clause is the run of units that the input method marks as the
// conversion's target.
func compositionEvent(text []uint16, attrs []byte, cursor int) platform.Event {
	e := platform.Event{Kind: platform.EventComposition}
	var b []byte
	start, end := -1, -1
	for i := 0; i < len(text); {
		r, n := rune(text[i]), 1
		if utf16.IsSurrogate(r) {
			r = utf8.RuneError
			if i+1 < len(text) {
				if d := utf16.DecodeRune(rune(text[i]), rune(text[i+1])); d != utf8.RuneError {
					r, n = d, 2
				}
			}
		}
		from := len(b)
		b = utf8.AppendRune(b, r)
		if i <= cursor && cursor < i+n {
			e.Cursor = from
		}
		if i < len(attrs) && (attrs[i] == attrTargetConverted || attrs[i] == attrTargetNotConverted) {
			if start < 0 {
				start = from
			}
			if end == from || end < 0 {
				end = len(b)
			}
		}
		i += n
	}
	if cursor >= len(text) {
		e.Cursor = len(b)
	}
	e.Text = string(b)
	if start >= 0 {
		e.ClauseStart, e.ClauseEnd = start, end
	}
	return e
}

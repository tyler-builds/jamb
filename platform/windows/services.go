//go:build windows

package windows

import (
	"strings"
	"time"

	"github.com/tyler-builds/jamb/internal/win32"
	"github.com/tyler-builds/jamb/platform"
)

const wakeTimerID = 1

// Clipboard implements platform.Window.
func (w *window) Clipboard() (string, bool) {
	s, ok := win32.ReadClipboard(w.hwnd)
	if !ok {
		return "", false
	}
	return strings.ReplaceAll(s, "\r\n", "\n"), true
}

// SetClipboard implements platform.Window.
func (w *window) SetClipboard(text string) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")
	win32.WriteClipboard(w.hwnd, text)
}

// WakeAt implements platform.Window with a single Win32 timer that always
// targets the earliest requested time.
func (w *window) WakeAt(t time.Time) {
	if !w.wakeAt.IsZero() && !t.Before(w.wakeAt) {
		return
	}
	w.wakeAt = t
	ms := time.Until(t).Milliseconds()
	win32.SetTimer(w.hwnd, wakeTimerID, uint32(max(ms, 1)))
}

// onTimer runs on WM_TIMER.
func (w *window) onTimer() {
	if w.wakeAt.IsZero() {
		win32.KillTimer(w.hwnd, wakeTimerID)
		return
	}
	if remaining := time.Until(w.wakeAt); remaining > time.Millisecond {
		win32.SetTimer(w.hwnd, wakeTimerID, uint32(remaining.Milliseconds()+1)) // fired early
		return
	}
	win32.KillTimer(w.hwnd, wakeTimerID)
	w.wakeAt = time.Time{}
	w.frameWanted = true
}

// SetTextInputRect implements platform.Window.
func (w *window) SetTextInputRect(r platform.Rect) {
	if r == w.imeRect {
		return
	}
	w.imeRect = r
	if r.H > 0 {
		win32.SetIMEPosition(w.hwnd, int32(r.X), int32(r.Y), int32(r.H))
	}
}

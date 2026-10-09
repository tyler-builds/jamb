//go:build windows

package windows

import (
	"errors"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/internal/win32"
)

// dialogTimerID is the timer that renders frames while a native dialog runs
// its own message loop; see modal.
const dialogTimerID = 2

// dialogFrameMS is the interval of that timer, in milliseconds: about 60 frames
// a second while a dialog is open.
const dialogFrameMS = 16

// Message box flags (MessageBoxW). The icon and button bits combine with the
// default-button bits.
const (
	mbOK              = 0x00000000
	mbOKCancel        = 0x00000001
	mbYesNoCancel     = 0x00000003
	mbYesNo           = 0x00000004
	mbIconError       = 0x00000010
	mbIconQuestion    = 0x00000020
	mbIconWarning     = 0x00000030
	mbIconInformation = 0x00000040
	mbDefButton2      = 0x00000100
	mbDefButton3      = 0x00000200
)

// defaultButtonFlags are the MB_DEFBUTTONn flags for the button at index n of
// a set: the first button is the default unless another one is chosen.
var defaultButtonFlags = [3]uint32{0, mbDefButton2, mbDefButton3}

// ShowFileDialog implements platform.Window. The dialog opens after the
// current event, never inside it, so the app's handler has returned first.
func (w *window) ShowFileDialog(kind dialog.FileKind, o dialog.FileOptions, done func(dialog.FileResult)) {
	Post(func() {
		if !win32.IsWindow(w.hwnd) {
			done(dialog.FileResult{Err: errors.New("platform/windows: the window closed before its dialog could open")})
			return
		}
		var res dialog.FileResult
		w.modal(func() { res = showFileDialog(w.hwnd, kind, o) })
		done(res)
	})
}

// ShowMessageBox implements platform.Window. Like ShowFileDialog, it opens
// after the current event.
func (w *window) ShowMessageBox(o dialog.MessageOptions, done func(dialog.Button)) {
	Post(func() {
		if !win32.IsWindow(w.hwnd) {
			done(dialog.NoButton)
			return
		}
		var b dialog.Button
		w.modal(func() { b = showMessageBox(w.hwnd, o) })
		done(b)
	})
}

// modal runs show, which opens a native dialog that runs its own message loop
// until the user closes it. Windows disables the owner while the dialog is
// open, so the owner takes no input. The loop of Run does not run meanwhile,
// so a timer on the owner renders each window that wants a frame and whose
// swap chain is ready, as runLoop does. Frames are drawn, and animations go on.
func (w *window) modal(show func()) {
	win32.SetTimer(w.hwnd, dialogTimerID, dialogFrameMS)
	show()
	win32.KillTimer(w.hwnd, dialogTimerID)
	// The dialog's loop may have taken a posted function's wake-up message, so
	// run the queue now rather than wait for the next message.
	runPosted()
}

// pumpFrames renders the windows that want a frame and whose swap chains can
// take one now. It never waits, so the dialog's loop is not held up.
func pumpFrames() {
	for _, w := range windows {
		if !w.ready || !(w.frameWanted || w.wakePending.Load()) {
			continue
		}
		if h := w.waitable(); h != 0 && w.width > 0 && w.height > 0 && win32.WaitForSingleObject(h, 0) != win32.WAIT_OBJECT_0 {
			continue
		}
		w.render()
	}
}

// messageButtons lists, for each button set, the MessageBoxW flags and the
// buttons in the order their default-button bits count them.
var messageButtons = map[dialog.ButtonSet]struct {
	flags uint32
	order []dialog.Button
}{
	dialog.ButtonsOK:          {mbOK, []dialog.Button{dialog.OK}},
	dialog.ButtonsOKCancel:    {mbOKCancel, []dialog.Button{dialog.OK, dialog.Cancel}},
	dialog.ButtonsYesNo:       {mbYesNo, []dialog.Button{dialog.Yes, dialog.No}},
	dialog.ButtonsYesNoCancel: {mbYesNoCancel, []dialog.Button{dialog.Yes, dialog.No, dialog.Cancel}},
}

// messageFlags returns the MessageBoxW flags for o: its icon, its buttons and
// the default button. A default that is not in the set means the first button.
func messageFlags(o dialog.MessageOptions) uint32 {
	var f uint32
	switch o.Icon {
	case dialog.IconInfo:
		f |= mbIconInformation
	case dialog.IconWarning:
		f |= mbIconWarning
	case dialog.IconError:
		f |= mbIconError
	case dialog.IconQuestion:
		f |= mbIconQuestion
	}
	set, ok := messageButtons[o.Buttons]
	if !ok {
		set = messageButtons[dialog.ButtonsOK]
	}
	f |= set.flags
	for i, b := range set.order {
		if b == o.Default {
			f |= defaultButtonFlags[i]
		}
	}
	return f
}

// showMessageBox shows o as a message box owned by owner and returns the
// button chosen.
func showMessageBox(owner uintptr, o dialog.MessageOptions) dialog.Button {
	switch win32.MessageBox(owner, o.Text, o.Title, messageFlags(o)) {
	case win32.IDOK:
		return dialog.OK
	case win32.IDCANCEL:
		return dialog.Cancel
	case win32.IDYES:
		return dialog.Yes
	case win32.IDNO:
		return dialog.No
	}
	return dialog.NoButton
}

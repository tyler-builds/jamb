//go:build windows

package win32

import "unsafe"

// Bindings for the text pattern of UI Automation and for the system caret, which
// screen magnifiers follow. See platform/windows/uiatext.go and services.go.

var (
	procCreateCaret                   = user32.NewProc("CreateCaret")
	procDestroyCaret                  = user32.NewProc("DestroyCaret")
	procHideCaret                     = user32.NewProc("HideCaret")
	procSetCaretPos                   = user32.NewProc("SetCaretPos")
	procUiaGetReservedNotSupportedVal = uiautomationcore.NewProc("UiaGetReservedNotSupportedValue")
	procSafeArrayAccessData           = oleaut32.NewProc("SafeArrayAccessData")
	procSafeArrayUnaccessData         = oleaut32.NewProc("SafeArrayUnaccessData")
)

// CreateCaret makes the system caret of the thread, owned by hwnd. A solid
// caret of width w and height h is made; it is hidden by HideCaret.
func CreateCaret(hwnd uintptr, w, h int32) bool {
	r, _, _ := procCreateCaret.Call(hwnd, 0, uintptr(w), uintptr(h))
	return r != 0
}

// DestroyCaret destroys the system caret of the thread, if it has one.
func DestroyCaret() bool {
	r, _, _ := procDestroyCaret.Call()
	return r != 0
}

// HideCaret hides the system caret of hwnd, without moving it. A hidden caret
// does not draw, but its position is still the one that GetGUIThreadInfo and the
// magnifiers read.
func HideCaret(hwnd uintptr) bool {
	r, _, _ := procHideCaret.Call(hwnd)
	return r != 0
}

// SetCaretPos moves the system caret to (x, y), in client coordinates of its
// window.
func SetCaretPos(x, y int32) bool {
	r, _, _ := procSetCaretPos.Call(uintptr(x), uintptr(y))
	return r != 0
}

// UiaGetReservedNotSupportedValue writes to out the IUnknown that UI Automation
// returns for an attribute a text range does not have. The caller owns the
// reference. It returns the HRESULT.
func UiaGetReservedNotSupportedValue(out uintptr) uintptr {
	r, _, _ := procUiaGetReservedNotSupportedVal.Call(out)
	return r
}

// SafeArrayAccessData locks the SAFEARRAY psa and writes the address of its data
// to data. Each call must be paired with SafeArrayUnaccessData.
func SafeArrayAccessData(psa uintptr, data *uintptr) uintptr {
	r, _, _ := procSafeArrayAccessData.Call(psa, uintptr(unsafe.Pointer(data)))
	return r
}

// SafeArrayUnaccessData unlocks a SAFEARRAY locked by SafeArrayAccessData.
func SafeArrayUnaccessData(psa uintptr) uintptr {
	r, _, _ := procSafeArrayUnaccessData.Call(psa)
	return r
}

// SysAllocStringN returns a new BSTR of n characters, which the caller fills in
// place and frees with SysFreeString. Its contents are not set.
func SysAllocStringN(n int) uintptr {
	r, _, _ := procSysAllocStringLen.Call(0, uintptr(n))
	return r
}

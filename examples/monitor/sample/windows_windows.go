package sample

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procGetWindow           = user32.NewProc("GetWindow")
	procGetWindowLongPtr    = user32.NewProc("GetWindowLongPtrW")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
)

const (
	gwOwner         = 4
	gwlExStyle      = ^uintptr(19) // -20
	wsExToolWindow  = 0x00000080
	wsExAppWindow   = 0x00040000
	dwmwaCloaked    = 14
	enumWindowsNext = 1
)

// windowPIDs returns the processes that have a window on the taskbar, as Task
// Manager lists them as apps: a top-level window that is shown, not owned by
// another, not a tool window unless it asks for a taskbar button, not cloaked
// (as the windows of a suspended app and of another virtual desktop are), and
// with a title.
func windowPIDs() map[uint32]bool {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumFound = map[uint32]bool{}
	windows.EnumWindows(enumCallback, nil)
	return enumFound
}

// The callback is made once, as syscall.NewCallback keeps each one for the life of
// the process, and the windows it finds go to enumFound under enumMu.
var (
	enumMu       sync.Mutex
	enumFound    map[uint32]bool
	enumCallback = syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		if !windows.IsWindowVisible(hwnd) {
			return enumWindowsNext
		}
		if owner, _, _ := procGetWindow.Call(uintptr(hwnd), gwOwner); owner != 0 {
			return enumWindowsNext
		}
		ex, _, _ := procGetWindowLongPtr.Call(uintptr(hwnd), gwlExStyle)
		if ex&wsExToolWindow != 0 && ex&wsExAppWindow == 0 {
			return enumWindowsNext
		}
		var cloaked uint32
		if windows.DwmGetWindowAttribute(hwnd, dwmwaCloaked, unsafe.Pointer(&cloaked), 4) == nil && cloaked != 0 {
			return enumWindowsNext
		}
		if n, _, _ := procGetWindowTextLength.Call(uintptr(hwnd)); n == 0 {
			return enumWindowsNext
		}
		var pid uint32
		if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err == nil && pid != 0 {
			enumFound[pid] = true
		}
		return enumWindowsNext
	})
)

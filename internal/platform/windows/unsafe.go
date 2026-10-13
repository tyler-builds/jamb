//go:build windows

package windows

import (
	"unsafe"

	"github.com/tyler-builds/jamb/internal/win32"
)

var sizeofWndClass = unsafe.Sizeof(win32.WNDCLASSEXW{})

// ptr converts an LPARAM that Windows documents as a pointer.
func ptr(v uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&v)) }

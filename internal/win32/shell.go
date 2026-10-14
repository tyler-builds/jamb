//go:build windows

package win32

import (
	"syscall"
	"unsafe"
)

// This file binds the notification area (Shell_NotifyIconW), the native popup
// menu and the icon calls behind the tray icon and the taskbar badge. The
// structs follow shellapi.h, winuser.h and wingdi.h of the Windows SDK, with the
// offsets of 64-bit Windows; shell_test.go checks the sizes.

var (
	gdi32 = syscall.NewLazyDLL("gdi32.dll")

	procShellNotifyIconW       = shell32.NewProc("Shell_NotifyIconW")
	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procTrackPopupMenuEx       = user32.NewProc("TrackPopupMenuEx")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procCreateIconIndirect     = user32.NewProc("CreateIconIndirect")
	procDestroyIcon            = user32.NewProc("DestroyIcon")
	procIsWindowVisible        = user32.NewProc("IsWindowVisible")
	procLoadImageW             = user32.NewProc("LoadImageW")
	procCreateBitmap           = gdi32.NewProc("CreateBitmap")
	procCreateDIBSection       = gdi32.NewProc("CreateDIBSection")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
)

const (
	WM_USER          = 0x0400
	WM_NULL          = 0x0000
	WM_CONTEXTMENU   = 0x007B
	WM_DISPLAYCHANGE = 0x007E

	// NIN_SELECT is sent when the icon is clicked, and NIN_KEYSELECT when Enter
	// or Space selects it, with NOTIFYICON_VERSION_4.
	NIN_SELECT    = WM_USER + 0
	NIN_KEYSELECT = NIN_SELECT | 0x1

	NIM_ADD              = 0x00000000
	NIM_MODIFY           = 0x00000001
	NIM_DELETE           = 0x00000002
	NIM_SETVERSION       = 0x00000004
	NOTIFYICON_VERSION_4 = 4

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004
	NIF_GUID    = 0x00000020
	NIF_SHOWTIP = 0x00000080 // with NOTIFYICON_VERSION_4, the standard tooltip shows only with this flag

	MF_STRING    = 0x00000000
	MF_GRAYED    = 0x00000001
	MF_CHECKED   = 0x00000008
	MF_POPUP     = 0x00000010
	MF_SEPARATOR = 0x00000800

	TPM_LEFTALIGN   = 0x0000
	TPM_TOPALIGN    = 0x0000
	TPM_RIGHTBUTTON = 0x0002
	TPM_BOTTOMALIGN = 0x0020
	TPM_RETURNCMD   = 0x0100

	SM_CXSMICON = 49

	IMAGE_ICON      = 1
	LR_DEFAULTCOLOR = 0x0000
	LR_SHARED       = 0x8000
	IDI_APPLICATION = 32512

	SW_HIDE = 0

	WS_POPUP         = 0x80000000
	WS_EX_TOOLWINDOW = 0x00000080

	BI_RGB         = 0
	DIB_RGB_COLORS = 0
)

// NOTIFYICONDATAW is the NOTIFYICONDATAW of shellapi.h, at its full size. GuidItem
// is the GUID as its bytes lie in memory (see platform/windows).
type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32 // the union with uTimeout, which balloons use
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

// ICONINFO is the ICONINFO of winuser.h.
type ICONINFO struct {
	FIcon    uint32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

// BITMAPINFOHEADER is the BITMAPINFOHEADER of wingdi.h.
type BITMAPINFOHEADER struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// RegisterWindowMessage returns the message that name identifies for every
// process, such as "TaskbarCreated".
func RegisterWindowMessage(name string) uint32 {
	p, _ := syscall.UTF16PtrFromString(name)
	r, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(p)))
	return uint32(r)
}

// ShellNotifyIcon sends msg (NIM_ADD, NIM_MODIFY, NIM_DELETE or NIM_SETVERSION)
// with the icon data to the notification area. It reports whether the shell
// accepted it.
func ShellNotifyIcon(msg uint32, nid *NOTIFYICONDATAW) bool {
	r, _, _ := procShellNotifyIconW.Call(uintptr(msg), uintptr(unsafe.Pointer(nid)))
	return r != 0
}

// CreatePopupMenu returns a new empty menu. The caller destroys it with DestroyMenu.
func CreatePopupMenu() uintptr {
	r, _, _ := procCreatePopupMenu.Call()
	return r
}

// AppendMenu appends an item to menu. With MF_POPUP, id is the handle of the
// submenu; otherwise it is the command ID that TrackPopupMenu returns.
func AppendMenu(menu uintptr, flags uint32, id uintptr, text string) bool {
	t, _ := syscall.UTF16PtrFromString(text)
	r, _, _ := procAppendMenuW.Call(menu, uintptr(flags), id, uintptr(unsafe.Pointer(t)))
	return r != 0
}

// DestroyMenu destroys a menu, and the submenus that were appended to it.
func DestroyMenu(menu uintptr) { procDestroyMenu.Call(menu) }

// TrackPopupMenu shows menu at the screen point (x, y) and runs it until the
// user chooses an item or dismisses it. It returns the command ID chosen, or 0.
// hwnd owns the menu, and should be in the foreground (see SetForegroundWindow).
func TrackPopupMenu(menu uintptr, flags uint32, x, y int32, hwnd uintptr) uint32 {
	r, _, _ := procTrackPopupMenuEx.Call(menu, uintptr(flags), uintptr(x), uintptr(y), hwnd, 0)
	return uint32(r)
}

// LoadIcon loads the icon resource name (an ID, as MAKEINTRESOURCE gives it) of
// module, scaled to size pixels square. A module of 0 with name IDI_APPLICATION
// is the standard application icon. A shared icon belongs to the system and is
// not destroyed; any other is the caller's, to destroy with DestroyIcon. It
// returns 0 when there is no such icon.
func LoadIcon(module, name uintptr, size int, shared bool) uintptr {
	flags := uintptr(LR_DEFAULTCOLOR)
	if shared {
		flags = LR_SHARED
	}
	r, _, _ := procLoadImageW.Call(module, name, IMAGE_ICON, uintptr(size), uintptr(size), flags)
	return r
}

// CreateIcon makes an icon of size pixels square from bgra, which holds size
// rows of size 32-bit pixels, top row first, in blue, green, red, alpha order
// with straight (not premultiplied) alpha. The caller destroys the icon with
// DestroyIcon. It returns 0 on failure.
func CreateIcon(size int, bgra []byte) uintptr {
	if size <= 0 || len(bgra) != size*size*4 {
		return 0
	}
	bmi := BITMAPINFOHEADER{
		Size: uint32(unsafe.Sizeof(BITMAPINFOHEADER{})), Width: int32(size), Height: -int32(size),
		Planes: 1, BitCount: 32, Compression: BI_RGB,
	}
	var bits unsafe.Pointer
	color, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 {
		return 0
	}
	copy(unsafe.Slice((*byte)(bits), len(bgra)), bgra)
	// The AND mask is one bit per pixel, rows padded to 16 bits. Its zeros leave
	// the alpha channel of the color bitmap to decide each pixel.
	mask := make([]byte, ((size+15)/16)*2*size)
	hbm, _, _ := procCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	info := ICONINFO{FIcon: 1, HbmMask: hbm, HbmColor: color}
	icon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	procDeleteObject.Call(hbm)
	procDeleteObject.Call(color)
	return icon
}

// IsWindowVisible reports whether the window has the WS_VISIBLE style, which
// ShowWindow(SW_HIDE) clears. A minimized window still has it.
func IsWindowVisible(hwnd uintptr) bool {
	r, _, _ := procIsWindowVisible.Call(hwnd)
	return r != 0
}

// DestroyIcon destroys an icon made by CreateIcon, or loaded without LR_SHARED.
func DestroyIcon(icon uintptr) {
	if icon != 0 {
		procDestroyIcon.Call(icon)
	}
}

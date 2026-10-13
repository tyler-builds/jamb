//go:build windows

// Package win32 is a minimal, cgo-free binding to the Win32 calls the Windows
// adapter needs. Only what is used is declared.
package win32

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	imm32    = syscall.NewLazyDLL("imm32.dll")

	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
	oleaut32          = syscall.NewLazyDLL("oleaut32.dll")

	uiautomationcore = syscall.NewLazyDLL("UIAutomationCore.dll")

	procOpenClipboard           = user32.NewProc("OpenClipboard")
	procCloseClipboard          = user32.NewProc("CloseClipboard")
	procEmptyClipboard          = user32.NewProc("EmptyClipboard")
	procGetClipboardData        = user32.NewProc("GetClipboardData")
	procSetClipboardData        = user32.NewProc("SetClipboardData")
	procSetTimer                = user32.NewProc("SetTimer")
	procKillTimer               = user32.NewProc("KillTimer")
	procGlobalAlloc             = kernel32.NewProc("GlobalAlloc")
	procGlobalFree              = kernel32.NewProc("GlobalFree")
	procGlobalLock              = kernel32.NewProc("GlobalLock")
	procGlobalUnlock            = kernel32.NewProc("GlobalUnlock")
	procGlobalSize              = kernel32.NewProc("GlobalSize")
	procRegisterClipboardFormat = user32.NewProc("RegisterClipboardFormatW")
	procIsClipboardFormatAvail  = user32.NewProc("IsClipboardFormatAvailable")
	procImmGetContext           = imm32.NewProc("ImmGetContext")
	procImmReleaseContext       = imm32.NewProc("ImmReleaseContext")
	procImmSetCompositionWindow = imm32.NewProc("ImmSetCompositionWindow")
	procImmSetCandidateWindow   = imm32.NewProc("ImmSetCandidateWindow")
	procImmGetCompositionString = imm32.NewProc("ImmGetCompositionStringW")
	procImmNotifyIME            = imm32.NewProc("ImmNotifyIME")

	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procPeekMessageW                  = user32.NewProc("PeekMessageW")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procPostMessageW                  = user32.NewProc("PostMessageW")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procGetClientRect                 = user32.NewProc("GetClientRect")
	procSetWindowTextW                = user32.NewProc("SetWindowTextW")
	procLoadCursorW                   = user32.NewProc("LoadCursorW")
	procSetCursor                     = user32.NewProc("SetCursor")
	procSetCapture                    = user32.NewProc("SetCapture")
	procReleaseCapture                = user32.NewProc("ReleaseCapture")
	procTrackMouseEvent               = user32.NewProc("TrackMouseEvent")
	procScreenToClient                = user32.NewProc("ScreenToClient")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procAdjustWindowRectExForDpi      = user32.NewProc("AdjustWindowRectExForDpi")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procGetWindowLongPtrW             = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW             = user32.NewProc("SetWindowLongPtrW")
	procGetWindowPlacement            = user32.NewProc("GetWindowPlacement")
	procSetWindowPlacement            = user32.NewProc("SetWindowPlacement")
	procGetKeyState                   = user32.NewProc("GetKeyState")
	procMonitorFromWindow             = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW               = user32.NewProc("GetMonitorInfoW")
	procValidateRect                  = user32.NewProc("ValidateRect")
	procGetWindowRect                 = user32.NewProc("GetWindowRect")
	procGetModuleHandleW              = kernel32.NewProc("GetModuleHandleW")
	procDwmSetWindowAttribute         = dwmapi.NewProc("DwmSetWindowAttribute")
	procOleInitialize                 = ole32.NewProc("OleInitialize")
	procDragAcceptFiles               = shell32.NewProc("DragAcceptFiles")
	procDragQueryFileW                = shell32.NewProc("DragQueryFileW")
	procDragQueryPoint                = shell32.NewProc("DragQueryPoint")
	procDragFinish                    = shell32.NewProc("DragFinish")
	procIsZoomed                      = user32.NewProc("IsZoomed")
	procIsIconic                      = user32.NewProc("IsIconic")
	procGetSystemMetricsForDpi        = user32.NewProc("GetSystemMetricsForDpi")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	procGetCurrentThreadId            = kernel32.NewProc("GetCurrentThreadId")
	procPostThreadMessageW            = user32.NewProc("PostThreadMessageW")
	procDwmExtendFrameIntoClientArea  = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	procIsWindow                      = user32.NewProc("IsWindow")
	procMessageBoxW                   = user32.NewProc("MessageBoxW")
	procCoCreateInstance              = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree                 = ole32.NewProc("CoTaskMemFree")
	procSHCreateItemFromParsingName   = shell32.NewProc("SHCreateItemFromParsingName")

	procFindFirstChangeNotificationW = kernel32.NewProc("FindFirstChangeNotificationW")
	procFindNextChangeNotification   = kernel32.NewProc("FindNextChangeNotification")
	procFindCloseChangeNotification  = kernel32.NewProc("FindCloseChangeNotification")
	procCreateEventW                 = kernel32.NewProc("CreateEventW")
	procSetEvent                     = kernel32.NewProc("SetEvent")
	procWaitForMultipleObjects       = kernel32.NewProc("WaitForMultipleObjects")
	procCloseHandle                  = kernel32.NewProc("CloseHandle")
	procWaitForSingleObject          = kernel32.NewProc("WaitForSingleObject")
	procMsgWaitForMultipleObjectsEx  = user32.NewProc("MsgWaitForMultipleObjectsEx")

	procClientToScreen        = user32.NewProc("ClientToScreen")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW  = user32.NewProc("GetWindowTextLengthW")
	procGetProcessHeap        = kernel32.NewProc("GetProcessHeap")
	procHeapAlloc             = kernel32.NewProc("HeapAlloc")
	procHeapFree              = kernel32.NewProc("HeapFree")
	procSysAllocStringLen     = oleaut32.NewProc("SysAllocStringLen")
	procSysFreeString         = oleaut32.NewProc("SysFreeString")
	procSysStringLen          = oleaut32.NewProc("SysStringLen")
	procSafeArrayCreateVector = oleaut32.NewProc("SafeArrayCreateVector")
	procSafeArrayPutElement   = oleaut32.NewProc("SafeArrayPutElement")
	procSafeArrayGetElement   = oleaut32.NewProc("SafeArrayGetElement")
	procSafeArrayDestroy      = oleaut32.NewProc("SafeArrayDestroy")
	procSafeArrayGetUBound    = oleaut32.NewProc("SafeArrayGetUBound")

	procUiaReturnRawElementProvider            = uiautomationcore.NewProc("UiaReturnRawElementProvider")
	procUiaHostProviderFromHwnd                = uiautomationcore.NewProc("UiaHostProviderFromHwnd")
	procUiaRaiseAutomationEvent                = uiautomationcore.NewProc("UiaRaiseAutomationEvent")
	procUiaRaiseAutomationPropertyChangedEvent = uiautomationcore.NewProc("UiaRaiseAutomationPropertyChangedEvent")
	procUiaRaiseNotificationEvent              = uiautomationcore.NewProc("UiaRaiseNotificationEvent")
	procUiaRaiseStructureChangedEvent          = uiautomationcore.NewProc("UiaRaiseStructureChangedEvent")
	procUiaDisconnectProvider                  = uiautomationcore.NewProc("UiaDisconnectProvider")
	procUiaClientsAreListening                 = uiautomationcore.NewProc("UiaClientsAreListening")
)

const (
	CS_HREDRAW = 0x0002
	CS_VREDRAW = 0x0001
	CS_DBLCLKS = 0x0008

	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_EX_APPWINDOW     = 0x00040000
	CW_USEDEFAULT       = ^uintptr(0x7fffffff) // 0x80000000 sign-extended

	SW_SHOW = 5

	WM_CREATE               = 0x0001
	WM_DESTROY              = 0x0002
	WM_SIZE                 = 0x0005
	SIZE_RESTORED           = 0
	SIZE_MAXIMIZED          = 2
	WM_SETFOCUS             = 0x0007
	WM_KILLFOCUS            = 0x0008
	WM_PAINT                = 0x000F
	WM_CLOSE                = 0x0010
	WM_QUIT                 = 0x0012
	WM_ERASEBKGND           = 0x0014
	WM_SETCURSOR            = 0x0020
	WM_GETMINMAXINFO        = 0x0024
	WM_KEYDOWN              = 0x0100
	WM_KEYUP                = 0x0101
	WM_CHAR                 = 0x0102
	WM_SYSKEYDOWN           = 0x0104
	WM_SYSKEYUP             = 0x0105
	WM_SYSCOMMAND           = 0x0112
	WM_CANCELMODE           = 0x001F
	WM_MOUSEMOVE            = 0x0200
	WM_LBUTTONDOWN          = 0x0201
	WM_LBUTTONUP            = 0x0202
	WM_LBUTTONDBLCLK        = 0x0203
	WM_RBUTTONDOWN          = 0x0204
	WM_RBUTTONUP            = 0x0205
	WM_RBUTTONDBLCLK        = 0x0206
	WM_MBUTTONDOWN          = 0x0207
	WM_MBUTTONUP            = 0x0208
	WM_MOUSEWHEEL           = 0x020A
	WM_MOUSEHWHEEL          = 0x020E
	WM_ENTERSIZEMOVE        = 0x0231
	WM_EXITSIZEMOVE         = 0x0232
	WM_ENTERMENULOOP        = 0x0211
	WM_EXITMENULOOP         = 0x0212
	WM_DPICHANGED           = 0x02E0
	WM_DROPFILES            = 0x0233
	WM_MOUSELEAVE           = 0x02A3
	WM_NCCREATE             = 0x0081
	WM_NCCALCSIZE           = 0x0083
	WM_NCHITTEST            = 0x0084
	WM_NCMOUSEMOVE          = 0x00A0
	WM_NCLBUTTONDOWN        = 0x00A1
	WM_NCLBUTTONUP          = 0x00A2
	WM_NCLBUTTONDBLCLK      = 0x00A3
	WM_GETOBJECT            = 0x003D
	WM_NCMOUSELEAVE         = 0x02A2
	WM_TIMER                = 0x0113
	WM_IME_STARTCOMPOSITION = 0x010D
	WM_IME_ENDCOMPOSITION   = 0x010E
	WM_IME_COMPOSITION      = 0x010F
	WM_IME_SETCONTEXT       = 0x0281
	WM_APP                  = 0x8000
	// WM_OCCLUSION is posted to a window by DXGI when the window's occlusion
	// state changes (see render/d3d11).
	WM_OCCLUSION = WM_APP + 3

	PM_REMOVE = 0x0001

	HTCLIENT = 1

	TME_LEAVE = 0x00000002

	SWP_NOZORDER     = 0x0004
	SWP_NOACTIVATE   = 0x0010
	SWP_NOSIZE       = 0x0001
	SWP_NOMOVE       = 0x0002
	SWP_FRAMECHANGED = 0x0020
	// SWP_NOOWNERZORDER leaves the owner window's place in the z-order alone.
	SWP_NOOWNERZORDER = 0x0200

	SW_MINIMIZE   = 6
	SW_MAXIMIZE   = 3
	SW_RESTORE    = 9
	SW_SHOWNORMAL = 1

	SM_CXSIZEFRAME    = 32
	SM_CYSIZEFRAME    = 33
	SM_CXPADDEDBORDER = 92

	HTCAPTION     = 2
	HTMINBUTTON   = 8
	HTMAXBUTTON   = 9
	HTLEFT        = 10
	HTRIGHT       = 11
	HTTOP         = 12
	HTTOPLEFT     = 13
	HTTOPRIGHT    = 14
	HTBOTTOM      = 15
	HTBOTTOMLEFT  = 16
	HTBOTTOMRIGHT = 17
	HTCLOSE       = 20

	TME_NONCLIENT = 0x00000010

	MONITOR_DEFAULTTONEAREST = 2

	IDC_ARROW    = 32512
	IDC_IBEAM    = 32513
	IDC_HAND     = 32649
	IDC_SIZEWE   = 32644
	IDC_SIZENS   = 32645
	IDC_NO       = 32648
	WHEEL_DELTA  = 120
	VK_SHIFT     = 0x10
	VK_CONTROL   = 0x11
	VK_MENU      = 0x12
	VK_F10       = 0x79
	VK_LWIN      = 0x5B
	VK_RWIN      = 0x5C
	VK_APPS      = 0x5D
	SC_KEYMENU   = 0xF100
	DPI_AWARE_V2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == -4

	DWMWA_USE_IMMERSIVE_DARK_MODE  = 20
	DWMWA_WINDOW_CORNER_PREFERENCE = 33
)

type POINT struct{ X, Y int32 }

type RECT struct{ Left, Top, Right, Bottom int32 }

type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type TRACKMOUSEEVENT struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   uintptr
	DwHoverTime uint32
}

type MINMAXINFO struct {
	PtReserved     POINT
	PtMaxSize      POINT
	PtMaxPosition  POINT
	PtMinTrackSize POINT
	PtMaxTrackSize POINT
}

type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

func RegisterClassEx(wc *WNDCLASSEXW) (uint16, error) {
	r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(wc)))
	if r == 0 {
		return 0, err
	}
	return uint16(r), nil
}

func CreateWindowEx(exStyle uint32, className, title *uint16, style uint32, x, y, w, h int32, parent, menu, instance uintptr) (uintptr, error) {
	r, _, err := procCreateWindowExW.Call(uintptr(exStyle), uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, menu, instance, 0)
	if r == 0 {
		return 0, err
	}
	return r, nil
}

func DefWindowProc(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), w, l)
	return r
}

func DestroyWindow(hwnd uintptr) { procDestroyWindow.Call(hwnd) }

// GetMessage blocks until a message arrives. It returns false on WM_QUIT.
func GetMessage(m *MSG) bool {
	r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(m)), 0, 0, 0)
	return int32(r) > 0
}

func PeekMessage(m *MSG) bool {
	r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(m)), 0, 0, 0, PM_REMOVE)
	return r != 0
}

func TranslateMessage(m *MSG) { procTranslateMessage.Call(uintptr(unsafe.Pointer(m))) }
func DispatchMessage(m *MSG)  { procDispatchMessageW.Call(uintptr(unsafe.Pointer(m))) }

func PostMessage(hwnd uintptr, msg uint32, w, l uintptr) {
	procPostMessageW.Call(hwnd, uintptr(msg), w, l)
}

func PostQuitMessage(code int) { procPostQuitMessage.Call(uintptr(code)) }

func ShowWindow(hwnd uintptr, cmd int) { procShowWindow.Call(hwnd, uintptr(cmd)) }

func GetClientRect(hwnd uintptr) RECT {
	var r RECT
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

// GetClientRectTo writes the client rectangle of hwnd to r. Unlike GetClientRect,
// it does not make a local that escapes to the heap, so a caller that keeps r
// in a long-lived place pays nothing per call.
func GetClientRectTo(hwnd uintptr, r *RECT) {
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(r)))
}

func SetWindowText(hwnd uintptr, s string) {
	p, _ := syscall.UTF16PtrFromString(s)
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(p)))
}

func LoadCursor(id uintptr) uintptr {
	r, _, _ := procLoadCursorW.Call(0, id)
	return r
}

func SetCursor(c uintptr)       { procSetCursor.Call(c) }
func SetCapture(hwnd uintptr)   { procSetCapture.Call(hwnd) }
func ReleaseCapture()           { procReleaseCapture.Call() }
func ValidateRect(hwnd uintptr) { procValidateRect.Call(hwnd, 0) }
func OleInitialize()            { procOleInitialize.Call(0) }
func GetKeyState(vk int) int16  { r, _, _ := procGetKeyState.Call(uintptr(vk)); return int16(r) }
func GetModuleHandle() uintptr  { r, _, _ := procGetModuleHandleW.Call(0); return r }

// TrackMouseLeave asks for WM_MOUSELEAVE, and for WM_NCMOUSELEAVE when
// nonClient is set (flags TME_NONCLIENT), once the pointer leaves the window.
func TrackMouseLeave(hwnd uintptr, nonClient bool) {
	flags := uint32(TME_LEAVE)
	if nonClient {
		flags |= TME_NONCLIENT
	}
	t := TRACKMOUSEEVENT{DwFlags: flags, HwndTrack: hwnd}
	t.CbSize = uint32(unsafe.Sizeof(t))
	procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&t)))
}

func ScreenToClient(hwnd uintptr, p *POINT) {
	procScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(p)))
}

// EnablePerMonitorDPI opts the process into per-monitor v2 DPI awareness.
// It is a no-op on systems that do not support it.
func EnablePerMonitorDPI() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		procSetProcessDpiAwarenessContext.Call(DPI_AWARE_V2)
	}
}

func GetDpiForWindow(hwnd uintptr) uint32 {
	if procGetDpiForWindow.Find() != nil {
		return 96
	}
	r, _, _ := procGetDpiForWindow.Call(hwnd)
	if r == 0 {
		return 96
	}
	return uint32(r)
}

func GetDpiForSystem() uint32 {
	if procGetDpiForSystem.Find() != nil {
		return 96
	}
	r, _, _ := procGetDpiForSystem.Call()
	if r == 0 {
		return 96
	}
	return uint32(r)
}

func AdjustWindowRectExForDpi(r *RECT, style, exStyle uint32, dpi uint32) {
	if procAdjustWindowRectExForDpi.Find() != nil {
		return
	}
	procAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(r)), uintptr(style), 0, uintptr(exStyle), uintptr(dpi))
}

func GetWindowRect(hwnd uintptr) RECT {
	var r RECT
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

func SetWindowPos(hwnd uintptr, x, y, w, h int32, flags uint32) {
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(flags))
}

// gwlStyle is GWL_STYLE (-16) as the unsigned argument the call takes.
const gwlStyle = ^uintptr(15)

// WindowStyle returns the window's style bits (GWL_STYLE).
func WindowStyle(hwnd uintptr) uintptr {
	r, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlStyle)
	return r
}

// SetWindowStyle replaces the window's style bits (GWL_STYLE). Call
// SetWindowPos with SWP_FRAMECHANGED afterwards so the frame is redrawn.
func SetWindowStyle(hwnd, style uintptr) { procSetWindowLongPtrW.Call(hwnd, gwlStyle, style) }

// WINDOWPLACEMENT is a window's normal, minimized and maximized positions.
type WINDOWPLACEMENT struct {
	Length           uint32
	Flags            uint32
	ShowCmd          uint32
	PtMinPosition    POINT
	PtMaxPosition    POINT
	RcNormalPosition RECT
}

// GetWindowPlacement returns the window's placement, to restore it later.
func GetWindowPlacement(hwnd uintptr) WINDOWPLACEMENT {
	wp := WINDOWPLACEMENT{}
	wp.Length = uint32(unsafe.Sizeof(wp))
	procGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))
	return wp
}

// SetWindowPlacement restores a placement from GetWindowPlacement.
func SetWindowPlacement(hwnd uintptr, wp *WINDOWPLACEMENT) {
	procSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(wp)))
}

// MonitorRect returns the whole area of the monitor nearest to hwnd, taskbar
// included (see WorkArea for the area without it).
func MonitorRect(hwnd uintptr) RECT {
	mon, _, _ := procMonitorFromWindow.Call(hwnd, MONITOR_DEFAULTTONEAREST)
	mi := MONITORINFO{}
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	return mi.RcMonitor
}

// WorkArea returns the work area of the monitor nearest to hwnd.
func WorkArea(hwnd uintptr) RECT {
	mon, _, _ := procMonitorFromWindow.Call(hwnd, MONITOR_DEFAULTTONEAREST)
	mi := MONITORINFO{}
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	return mi.RcWork
}

// SetDarkTitleBar asks DWM for dark (or light) window decorations.
func SetDarkTitleBar(hwnd uintptr, dark bool) {
	if procDwmSetWindowAttribute.Find() != nil {
		return
	}
	var v int32
	if dark {
		v = 1
	}
	procDwmSetWindowAttribute.Call(hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, uintptr(unsafe.Pointer(&v)), 4)
}

// ---- drag and drop of files ----

// DragAcceptFiles lets the window receive WM_DROPFILES.
func DragAcceptFiles(hwnd uintptr, accept bool) {
	var v uintptr
	if accept {
		v = 1
	}
	procDragAcceptFiles.Call(hwnd, v)
}

// DroppedFiles reads the paths and the drop point (in client pixels) of a
// WM_DROPFILES message's drop handle, then releases the handle.
func DroppedFiles(hdrop uintptr) ([]string, POINT) {
	const all = 0xFFFFFFFF // DragQueryFileW index that asks for the count
	n, _, _ := procDragQueryFileW.Call(hdrop, all, 0, 0)
	paths := make([]string, 0, n)
	for i := uintptr(0); i < n; i++ {
		l, _, _ := procDragQueryFileW.Call(hdrop, i, 0, 0) // length without the terminating NUL
		buf := make([]uint16, l+1)
		procDragQueryFileW.Call(hdrop, i, uintptr(unsafe.Pointer(&buf[0])), l+1)
		paths = append(paths, syscall.UTF16ToString(buf))
	}
	var p POINT
	procDragQueryPoint.Call(hdrop, uintptr(unsafe.Pointer(&p)))
	procDragFinish.Call(hdrop)
	return paths, p
}

// ---- clipboard ----

// Standard clipboard formats.
const (
	CF_DIB         = 8
	cfUnicodeText  = 13
	CF_DIBV5       = 17
	gmemMoveable   = 0x0002
	maxClipboardSz = 1 << 30 // larger data is refused rather than copied
)

// RegisterClipboardFormat returns the number of the clipboard format called
// name, such as "PNG", registering it if no program has; 0 on failure.
func RegisterClipboardFormat(name string) uint32 {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0
	}
	r, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(p)))
	return uint32(r)
}

// ClipboardHas reports whether the clipboard holds data in one of formats,
// including formats the system would make from the data it holds. It does not
// open the clipboard.
func ClipboardHas(formats ...uint32) bool {
	for _, f := range formats {
		if r, _, _ := procIsClipboardFormatAvail.Call(uintptr(f)); f != 0 && r != 0 {
			return true
		}
	}
	return false
}

// ReadClipboard returns the clipboard's text, if any.
func ReadClipboard(hwnd uintptr) (string, bool) {
	_, b, ok := ReadClipboardData(hwnd, cfUnicodeText)
	if !ok {
		return "", false
	}
	units := unsafe.Slice((*uint16)(unsafe.Pointer(unsafe.SliceData(b))), len(b)/2)
	for i, u := range units {
		if u == 0 {
			units = units[:i]
			break
		}
	}
	return string(utf16.Decode(units)), true
}

// WriteClipboard replaces the clipboard's contents with text.
func WriteClipboard(hwnd uintptr, text string) bool {
	units := utf16.Encode([]rune(text))
	units = append(units, 0)
	b := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(units))), len(units)*2)
	return WriteClipboardData(hwnd, ClipboardData{Format: cfUnicodeText, Data: b})
}

// ClipboardData is the clipboard's contents in one format.
type ClipboardData struct {
	Format uint32
	Data   []byte
}

// ReadClipboardData returns a copy of the clipboard's contents in the first of
// formats that it holds, and that format. The copy may be longer than the
// data that was put there: the system rounds memory blocks up.
func ReadClipboardData(hwnd uintptr, formats ...uint32) (uint32, []byte, bool) {
	if r, _, _ := procOpenClipboard.Call(hwnd); r == 0 {
		return 0, nil, false
	}
	defer procCloseClipboard.Call()
	for _, f := range formats {
		if f == 0 {
			continue
		}
		h, _, _ := procGetClipboardData.Call(uintptr(f))
		if h == 0 {
			continue
		}
		n, _, _ := procGlobalSize.Call(h)
		if n == 0 || n > maxClipboardSz {
			continue
		}
		p, _, _ := procGlobalLock.Call(h)
		if p == 0 {
			continue
		}
		b := make([]byte, n)
		copy(b, unsafe.Slice((*byte)(ptrAt(p)), n))
		procGlobalUnlock.Call(h)
		return f, b, true
	}
	return 0, nil, false
}

// WriteClipboardData replaces the clipboard's contents with data, which holds
// the same contents in one or more formats. It reports whether the clipboard
// took at least one of them.
func WriteClipboardData(hwnd uintptr, data ...ClipboardData) bool {
	handles := make([]uintptr, len(data))
	defer func() {
		for _, h := range handles {
			if h != 0 {
				procGlobalFree.Call(h) // ownership passes to the system only on success
			}
		}
	}()
	for i, d := range data {
		if d.Format == 0 || len(d.Data) == 0 {
			continue
		}
		h, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(len(d.Data)))
		if h == 0 {
			continue
		}
		p, _, _ := procGlobalLock.Call(h)
		if p == 0 {
			procGlobalFree.Call(h)
			continue
		}
		copy(unsafe.Slice((*byte)(ptrAt(p)), len(d.Data)), d.Data)
		procGlobalUnlock.Call(h)
		handles[i] = h
	}
	if r, _, _ := procOpenClipboard.Call(hwnd); r == 0 {
		return false
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	ok := false
	for i, h := range handles {
		if h == 0 {
			continue
		}
		if r, _, _ := procSetClipboardData.Call(uintptr(data[i].Format), h); r != 0 {
			handles[i] = 0
			ok = true
		}
	}
	return ok
}

// ---- file change notifications ----

const (
	FILE_NOTIFY_CHANGE_FILE_NAME  = 0x00000001
	FILE_NOTIFY_CHANGE_SIZE       = 0x00000008
	FILE_NOTIFY_CHANGE_LAST_WRITE = 0x00000010

	INFINITE      = 0xFFFFFFFF
	WAIT_OBJECT_0 = 0x00000000
	WAIT_TIMEOUT  = 0x00000102
	WAIT_FAILED   = 0xFFFFFFFF

	QS_ALLINPUT = 0x04FF
)

// FindFirstChangeNotification starts watching dir for the changes in filter.
func FindFirstChangeNotification(dir string, watchSubtree bool, filter uint32) (uintptr, error) {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var sub uintptr
	if watchSubtree {
		sub = 1
	}
	h, _, err := procFindFirstChangeNotificationW.Call(uintptr(unsafe.Pointer(p)), sub, uintptr(filter))
	if h == ^uintptr(0) {
		return 0, err
	}
	return h, nil
}

// FindNextChangeNotification re-arms a change handle after a change.
func FindNextChangeNotification(h uintptr) bool {
	r, _, _ := procFindNextChangeNotification.Call(h)
	return r != 0
}

func FindCloseChangeNotification(h uintptr) { procFindCloseChangeNotification.Call(h) }

// CreateEvent makes a manual-reset event that starts unsignaled.
func CreateEvent() (uintptr, error) {
	h, _, err := procCreateEventW.Call(0, 1, 0, 0)
	if h == 0 {
		return 0, err
	}
	return h, nil
}

func SetEvent(h uintptr) { procSetEvent.Call(h) }

func CloseHandle(h uintptr) { procCloseHandle.Call(h) }

// WaitForAny waits until one of the handles is signaled or ms milliseconds
// pass. It returns the index of the signaled handle, WAIT_TIMEOUT or
// WAIT_FAILED.
func WaitForAny(handles []uintptr, ms uint32) uint32 {
	r, _, _ := procWaitForMultipleObjects.Call(uintptr(len(handles)), uintptr(unsafe.Pointer(&handles[0])), 0, uintptr(ms))
	return uint32(r)
}

// ptrAt converts a pointer returned by Windows (not Go memory).
func ptrAt(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

// ---- window state and custom frames ----

func IsZoomed(hwnd uintptr) bool {
	r, _, _ := procIsZoomed.Call(hwnd)
	return r != 0
}

func IsIconic(hwnd uintptr) bool {
	r, _, _ := procIsIconic.Call(hwnd)
	return r != 0
}

func SetForegroundWindow(hwnd uintptr) { procSetForegroundWindow.Call(hwnd) }

// SystemMetricsForDpi returns a system metric (SM_*) scaled to dpi. It falls
// back to the unscaled metric on systems before Windows 10 1607.
func SystemMetricsForDpi(index int, dpi uint32) int32 {
	if procGetSystemMetricsForDpi.Find() != nil {
		r, _, _ := procGetSystemMetrics.Call(uintptr(index))
		return int32(r)
	}
	r, _, _ := procGetSystemMetricsForDpi.Call(uintptr(index), uintptr(dpi))
	return int32(r)
}

// Margins is the MARGINS struct of DwmExtendFrameIntoClientArea.
type Margins struct{ Left, Right, Top, Bottom int32 }

// ExtendFrameIntoClientArea makes DWM draw the window's shadow (and frame
// effects) into the given margins. A 1 px margin keeps the drop shadow of a
// window whose client area covers its frame; see applyCustomFrame in
// platform/windows for why the top margin stays 0.
func ExtendFrameIntoClientArea(hwnd uintptr, m Margins) {
	if procDwmExtendFrameIntoClientArea.Find() != nil {
		return
	}
	procDwmExtendFrameIntoClientArea.Call(hwnd, uintptr(unsafe.Pointer(&m)))
}

func GetCurrentThreadID() uint32 {
	r, _, _ := procGetCurrentThreadId.Call()
	return uint32(r)
}

// PostThreadMessage queues a message for the thread's message loop. It has
// no window, so the loop must handle it itself.
func PostThreadMessage(thread uint32, msg uint32) {
	procPostThreadMessageW.Call(uintptr(thread), uintptr(msg), 0, 0)
}

// ---- dialogs ----

// The button IDs MessageBox returns.
const (
	IDOK     = 1
	IDCANCEL = 2
	IDYES    = 6
	IDNO     = 7
)

// MessageBox shows a message box owned by owner, which it makes modal to that
// window. It returns the button's ID, or 0 if the box could not be shown.
func MessageBox(owner uintptr, text, caption string, flags uint32) int {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(caption)
	r, _, _ := procMessageBoxW.Call(owner, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), uintptr(flags))
	return int(r)
}

// IsWindow reports whether hwnd is still a window.
func IsWindow(hwnd uintptr) bool {
	r, _, _ := procIsWindow.Call(hwnd)
	return r != 0
}

// CoCreateInstance creates an in-process COM object of class clsid, and stores
// its interface iid in obj. It returns the HRESULT.
func CoCreateInstance(clsid, iid unsafe.Pointer, obj *uintptr) uintptr {
	const clsctxInprocServer = 1
	r, _, _ := procCoCreateInstance.Call(uintptr(clsid), 0, clsctxInprocServer, uintptr(iid), uintptr(unsafe.Pointer(obj)))
	return r
}

// SHCreateItemFromParsingName creates a shell item for the path, which must be
// NUL-terminated UTF-16, and stores its IShellItem interface iid in item. It
// returns the HRESULT.
func SHCreateItemFromParsingName(path *uint16, iid unsafe.Pointer, item *uintptr) uintptr {
	r, _, _ := procSHCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(path)), 0, uintptr(iid), uintptr(unsafe.Pointer(item)))
	return r
}

// CoTaskMemFree frees memory that a COM call returned, such as a path string.
func CoTaskMemFree(p uintptr) { procCoTaskMemFree.Call(p) }

// ---- timers ----

func SetTimer(hwnd, id uintptr, ms uint32) { procSetTimer.Call(hwnd, id, uintptr(ms), 0) }
func KillTimer(hwnd, id uintptr)           { procKillTimer.Call(hwnd, id) }

// ---- IME ----

type compositionForm struct {
	Style uint32
	Pos   POINT
	Area  RECT
}

type candidateForm struct {
	Index uint32
	Style uint32
	Pos   POINT
	Area  RECT
}

// SetIMEPosition places the IME composition window at the caret (x, y is the
// caret's top in client pixels, h its height) and keeps the candidate list
// from covering the line being typed.
func SetIMEPosition(hwnd uintptr, x, y, h int32) {
	if procImmGetContext.Find() != nil {
		return
	}
	ctx, _, _ := procImmGetContext.Call(hwnd)
	if ctx == 0 {
		return
	}
	defer procImmReleaseContext.Call(hwnd, ctx)
	const cfsPoint, cfsCandidatePos, cfsExclude = 0x0002, 0x0040, 0x0080
	cf := compositionForm{Style: cfsPoint, Pos: POINT{x, y}}
	procImmSetCompositionWindow.Call(ctx, uintptr(unsafe.Pointer(&cf)))
	cand := candidateForm{Style: cfsExclude, Pos: POINT{x, y + h}, Area: RECT{x, y, x + 1, y + h}}
	procImmSetCandidateWindow.Call(ctx, uintptr(unsafe.Pointer(&cand)))
	_ = cfsCandidatePos
}

// The flags of WM_IME_COMPOSITION, and of WM_IME_SETCONTEXT, that the window
// uses.
const (
	GCS_COMPSTR                 = 0x0008
	GCS_COMPATTR                = 0x0010
	GCS_CURSORPOS               = 0x0080
	GCS_RESULTSTR               = 0x0800
	ISC_SHOWUICOMPOSITIONWINDOW = 0x80000000
)

// Composition is what an input method reports with WM_IME_COMPOSITION. The
// strings are UTF-16. Attrs has one ATTR_* value per unit of Text, and Cursor
// is the caret as a unit index in Text.
type Composition struct {
	Result []uint16 // the confirmed text, when the flags have GCS_RESULTSTR
	Text   []uint16 // the text being composed, when they have GCS_COMPSTR
	Attrs  []byte
	Cursor int
}

// ReadComposition reads the parts of the window's input method composition
// that flags, the lParam of WM_IME_COMPOSITION, says changed.
func ReadComposition(hwnd uintptr, flags uint32) Composition {
	var c Composition
	if procImmGetContext.Find() != nil || procImmGetCompositionString.Find() != nil {
		return c
	}
	ctx, _, _ := procImmGetContext.Call(hwnd)
	if ctx == 0 {
		return c
	}
	defer procImmReleaseContext.Call(hwnd, ctx)
	read := func(index uint32) []byte {
		n, _, _ := procImmGetCompositionString.Call(ctx, uintptr(index), 0, 0)
		if int32(n) <= 0 {
			return nil
		}
		b := make([]byte, int32(n))
		procImmGetCompositionString.Call(ctx, uintptr(index), uintptr(unsafe.Pointer(&b[0])), n)
		return b
	}
	units := func(b []byte) []uint16 {
		if len(b) < 2 {
			return nil
		}
		return unsafe.Slice((*uint16)(unsafe.Pointer(&b[0])), len(b)/2)
	}
	if flags&GCS_RESULTSTR != 0 {
		c.Result = units(read(GCS_RESULTSTR))
	}
	if flags&GCS_COMPSTR != 0 {
		c.Text = units(read(GCS_COMPSTR))
		c.Attrs = read(GCS_COMPATTR)
		if r, _, _ := procImmGetCompositionString.Call(ctx, GCS_CURSORPOS, 0, 0); int32(r) >= 0 {
			c.Cursor = int(int32(r))
		} else {
			c.Cursor = len(c.Text)
		}
	}
	return c
}

// CancelComposition makes the window's input method drop the text it is
// composing. The input method may send its composition messages before this
// returns.
func CancelComposition(hwnd uintptr) {
	if procImmGetContext.Find() != nil || procImmNotifyIME.Find() != nil {
		return
	}
	ctx, _, _ := procImmGetContext.Call(hwnd)
	if ctx == 0 {
		return
	}
	defer procImmReleaseContext.Call(hwnd, ctx)
	const niCompositionStr, cpsCancel = 0x0015, 0x0004
	procImmNotifyIME.Call(ctx, niCompositionStr, cpsCancel, 0)
}

func LOWORD(v uintptr) uint16 { return uint16(v & 0xffff) }
func HIWORD(v uintptr) uint16 { return uint16((v >> 16) & 0xffff) }

// PointFromLParam decodes signed client coordinates from a mouse message.
func PointFromLParam(l uintptr) (int32, int32) {
	return int32(int16(LOWORD(l))), int32(int16(HIWORD(l)))
}

// NCCalcSizeParams is the NCCALCSIZE_PARAMS of WM_NCCALCSIZE: the proposed
// window rectangle, the rectangles that follow it, and the WINDOWPOS.
type NCCalcSizeParams struct {
	Rgrc [3]RECT
	LPos uintptr
}

// WaitForSingleObject waits up to ms milliseconds for h to be signaled. It
// returns WAIT_OBJECT_0 when it was, WAIT_TIMEOUT when it was not.
func WaitForSingleObject(h uintptr, ms uint32) uint32 {
	r, _, _ := procWaitForSingleObject.Call(h, uintptr(ms))
	return uint32(r)
}

// MsgWaitForAny sleeps until one of the handles is signaled or a message
// arrives in the thread's queue. It returns the index of the handle (less than
// len(handles)), len(handles) for a message, or WAIT_FAILED.
func MsgWaitForAny(handles []uintptr) uint32 {
	r, _, _ := procMsgWaitForMultipleObjectsEx.Call(uintptr(len(handles)), uintptr(unsafe.Pointer(&handles[0])),
		INFINITE, QS_ALLINPUT, 0)
	if r == WAIT_FAILED {
		return WAIT_FAILED
	}
	return uint32(r) - WAIT_OBJECT_0
}

// ---- UI Automation ----

// HEAP_ZERO_MEMORY makes HeapAlloc clear the block it returns.
const HEAP_ZERO_MEMORY = 0x00000008

// ClientToScreen converts the client point p of hwnd to screen coordinates, in
// place. The result is in physical pixels for a per-monitor aware process.
func ClientToScreen(hwnd uintptr, p *POINT) {
	procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(p)))
}

// GetWindowText returns the title of hwnd, or "" if it has none.
func GetWindowText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	buf := make([]uint16, n+1)
	r, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:r])
}

// GetProcessHeap returns the process's default heap.
func GetProcessHeap() uintptr {
	r, _, _ := procGetProcessHeap.Call()
	return r
}

// HeapAlloc allocates size bytes on heap, with flags such as HEAP_ZERO_MEMORY,
// and returns the address, or 0 on failure. The memory is outside the Go heap.
func HeapAlloc(heap uintptr, flags uint32, size uintptr) uintptr {
	r, _, _ := procHeapAlloc.Call(heap, uintptr(flags), size)
	return r
}

// HeapFree frees a block that HeapAlloc returned.
func HeapFree(heap uintptr, flags uint32, p uintptr) {
	procHeapFree.Call(heap, uintptr(flags), p)
}

// SysAllocStringLen returns a new BSTR holding a copy of s, which may be empty.
// The caller frees it with SysFreeString.
func SysAllocStringLen(s []uint16) uintptr {
	var p uintptr
	if len(s) > 0 {
		p = uintptr(unsafe.Pointer(&s[0]))
	}
	r, _, _ := procSysAllocStringLen.Call(p, uintptr(len(s)))
	return r
}

// SysFreeString frees a BSTR. It does nothing for 0.
func SysFreeString(bstr uintptr) {
	if bstr != 0 {
		procSysFreeString.Call(bstr)
	}
}

// SysStringLen returns the number of characters in a BSTR.
func SysStringLen(bstr uintptr) uint32 {
	r, _, _ := procSysStringLen.Call(bstr)
	return uint32(r)
}

// SafeArrayCreateVector creates a one-dimensional SAFEARRAY of n elements of
// type vt, indexed from lbound. It returns 0 on failure.
func SafeArrayCreateVector(vt uint16, lbound int32, n uint32) uintptr {
	r, _, _ := procSafeArrayCreateVector.Call(uintptr(vt), uintptr(lbound), uintptr(n))
	return r
}

// SafeArrayPutInt32 stores v at index i of the SAFEARRAY of 4-byte integers psa,
// and returns the HRESULT.
func SafeArrayPutInt32(psa uintptr, i, v int32) uintptr {
	r, _, _ := procSafeArrayPutElement.Call(psa, uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(&v)))
	return r
}

// SafeArrayPutUnknown stores p, an interface pointer, at index i of the SAFEARRAY
// of IUnknown pointers psa, and returns the HRESULT. For an interface array the
// element is the pointer itself, not the address of a variable that holds it. The
// array AddRefs the element, so the caller keeps its own reference.
func SafeArrayPutUnknown(psa uintptr, i int32, p uintptr) uintptr {
	r, _, _ := procSafeArrayPutElement.Call(psa, uintptr(unsafe.Pointer(&i)), p)
	return r
}

// SafeArrayGetInt32 reads the element at index i of the SAFEARRAY of 4-byte
// integers psa. It returns the HRESULT.
func SafeArrayGetInt32(psa uintptr, i int32, v *int32) uintptr {
	r, _, _ := procSafeArrayGetElement.Call(psa, uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(v)))
	return r
}

// SafeArrayUBound returns the upper bound of the first dimension of the SAFEARRAY
// psa. For a vector, that is one less than its length.
func SafeArrayUBound(psa uintptr) int32 {
	var ub int32
	procSafeArrayGetUBound.Call(psa, 1, uintptr(unsafe.Pointer(&ub)))
	return ub
}

// SafeArrayGetUnknown reads the interface pointer at index i of the SAFEARRAY of
// IUnknown pointers psa into out. The element is AddRef-ed, so the caller
// releases it. It returns the HRESULT.
func SafeArrayGetUnknown(psa uintptr, i int32, out *uintptr) uintptr {
	r, _, _ := procSafeArrayGetElement.Call(psa, uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(out)))
	return r
}

// SafeArrayDestroy frees a SAFEARRAY and its elements. It does nothing for 0.
func SafeArrayDestroy(psa uintptr) {
	if psa != 0 {
		procSafeArrayDestroy.Call(psa)
	}
}

// UiaReturnRawElementProvider answers a WM_GETOBJECT message with the provider
// el, and returns the LRESULT to return from the window procedure.
func UiaReturnRawElementProvider(hwnd, wp, lp, el uintptr) uintptr {
	r, _, _ := procUiaReturnRawElementProvider.Call(hwnd, wp, lp, el)
	return r
}

// UiaHostProviderFromHwnd writes to out the host provider of the window hwnd,
// the native element that UI Automation makes for a window. It returns the
// HRESULT.
func UiaHostProviderFromHwnd(hwnd, out uintptr) uintptr {
	r, _, _ := procUiaHostProviderFromHwnd.Call(hwnd, out)
	return r
}

// UiaRaiseAutomationEvent raises the event id for provider, an
// IRawElementProviderSimple.
func UiaRaiseAutomationEvent(provider uintptr, id int32) uintptr {
	r, _, _ := procUiaRaiseAutomationEvent.Call(provider, uintptr(id))
	return r
}

// UiaRaiseStructureChangedEvent raises a structure-changed event of kind change
// for provider, an IRawElementProviderSimple. The runtime ID arguments are
// optional: pass 0 and 0.
func UiaRaiseStructureChangedEvent(provider uintptr, change int32, runtimeID uintptr, n int32) uintptr {
	r, _, _ := procUiaRaiseStructureChangedEvent.Call(provider, uintptr(change), runtimeID, uintptr(n))
	return r
}

// UiaDisconnectProvider disconnects provider, an IRawElementProviderSimple,
// from UI Automation.
func UiaDisconnectProvider(provider uintptr) uintptr {
	r, _, _ := procUiaDisconnectProvider.Call(provider)
	return r
}

// UiaClientsAreListening reports whether an assistive technology is listening
// for UI Automation events.
func UiaClientsAreListening() bool {
	r, _, _ := procUiaClientsAreListening.Call()
	return r != 0
}

// UiaRaiseAutomationPropertyChangedEvent raises a property-changed event for the
// property id of provider, an IRawElementProviderSimple. oldValue and newValue
// are the addresses of VARIANTs. A VARIANT is larger than a register, so the
// ABI passes it by address; UI Automation copies them, and the caller keeps
// ownership of any BSTR in them.
func UiaRaiseAutomationPropertyChangedEvent(provider uintptr, id int32, oldValue, newValue uintptr) uintptr {
	r, _, _ := procUiaRaiseAutomationPropertyChangedEvent.Call(provider, uintptr(id), oldValue, newValue)
	return r
}

// UiaRaiseNotificationEvent raises a notification event for provider, an
// IRawElementProviderSimple, with the kind and processing of the notification
// and the BSTRs display and activity. The caller keeps ownership of the BSTRs.
func UiaRaiseNotificationEvent(provider uintptr, kind, processing int32, display, activity uintptr) uintptr {
	r, _, _ := procUiaRaiseNotificationEvent.Call(provider, uintptr(kind), uintptr(processing), display, activity)
	return r
}

// ShellOpen opens target with its default handler, as a double-click in
// Explorer does (ShellExecuteW with the "open" verb), owned by hwnd. It
// reports false when the shell could not open it.
func ShellOpen(hwnd uintptr, target string) bool {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return false
	}
	r, _, _ := procShellExecuteW.Call(hwnd, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, SW_SHOWNORMAL)
	return r > 32 // values up to 32 are errors
}

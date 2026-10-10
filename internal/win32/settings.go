//go:build windows

package win32

import (
	"syscall"
	"unsafe"
)

// The system settings that the platform reads. Every call here only reads a
// setting; none of them changes one.

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procGetSysColor           = user32.NewProc("GetSysColor")
	procGetCaretBlinkTime     = user32.NewProc("GetCaretBlinkTime")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	procRegGetValueW          = advapi32.NewProc("RegGetValueW")
)

const (
	WM_SETTINGCHANGE  = 0x001A
	WM_SYSCOLORCHANGE = 0x0015

	SPI_GETHIGHCONTRAST        = 0x0042
	SPI_GETCLIENTAREAANIMATION = 0x1042
	SPI_GETCARETWIDTH          = 0x2006

	HCF_HIGHCONTRASTON = 0x00000001

	HKEY_CURRENT_USER = 0x80000001
	RRF_RT_REG_DWORD  = 0x00000010
	ERROR_SUCCESS     = 0
)

// highContrast is the HIGHCONTRASTW struct of SPI_GETHIGHCONTRAST.
type highContrast struct {
	CbSize        uint32
	Flags         uint32
	DefaultScheme uintptr
}

// SysColor returns the color of the system color index (a COLOR_* value) as a
// COLORREF, 0x00BBGGRR. An index that does not exist gives 0.
func SysColor(index int) uint32 {
	r, _, _ := procGetSysColor.Call(uintptr(index))
	return uint32(r)
}

// CaretBlinkTime returns the caret blink interval in milliseconds, or INFINITE
// when the caret does not blink.
func CaretBlinkTime() uint32 {
	r, _, _ := procGetCaretBlinkTime.Call()
	return uint32(r)
}

// HighContrastFlags returns the dwFlags of SPI_GETHIGHCONTRAST (HCF_* bits). It
// reports whether the call succeeded.
func HighContrastFlags() (uint32, bool) {
	hc := highContrast{CbSize: uint32(unsafe.Sizeof(highContrast{}))}
	r, _, _ := procSystemParametersInfoW.Call(SPI_GETHIGHCONTRAST, uintptr(hc.CbSize), uintptr(unsafe.Pointer(&hc)), 0)
	return hc.Flags, r != 0
}

// ClientAreaAnimation returns the BOOL of SPI_GETCLIENTAREAANIMATION, which is
// 0 when the animation effects are off. It reports whether the call succeeded.
func ClientAreaAnimation() (int32, bool) {
	var on int32
	r, _, _ := procSystemParametersInfoW.Call(SPI_GETCLIENTAREAANIMATION, 0, uintptr(unsafe.Pointer(&on)), 0)
	return on, r != 0
}

// CaretWidth returns SPI_GETCARETWIDTH, the caret thickness in device pixels. It
// reports whether the call succeeded.
func CaretWidth() (uint32, bool) {
	var w uint32
	r, _, _ := procSystemParametersInfoW.Call(SPI_GETCARETWIDTH, 0, uintptr(unsafe.Pointer(&w)), 0)
	return w, r != 0
}

// CurrentUserDWORD reads the REG_DWORD value of the key subkey under
// HKEY_CURRENT_USER. It reports whether the value was read.
func CurrentUserDWORD(subkey, value string) (uint32, bool) {
	k, err := syscall.UTF16PtrFromString(subkey)
	if err != nil {
		return 0, false
	}
	v, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return 0, false
	}
	var data uint32
	size := uint32(unsafe.Sizeof(data))
	r, _, _ := procRegGetValueW.Call(HKEY_CURRENT_USER, uintptr(unsafe.Pointer(k)), uintptr(unsafe.Pointer(v)),
		RRF_RT_REG_DWORD, 0, uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&size)))
	return data, r == ERROR_SUCCESS
}

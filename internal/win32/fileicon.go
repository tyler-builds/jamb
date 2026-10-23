//go:build windows

package win32

import (
	"errors"
	"image"
	"runtime"
	"syscall"
	"unsafe"
)

// This file binds the calls behind app.FileIcon: the icon of a file, from its own
// icon resources or from its type, and the pixels of an icon. The structs follow
// shellapi.h and wingdi.h, with the offsets of 64-bit Windows; fileicon_test.go
// checks the sizes.

var (
	procSHDefExtractIconW  = shell32.NewProc("SHDefExtractIconW")
	procSHGetFileInfoW     = shell32.NewProc("SHGetFileInfoW")
	procCoInitializeEx     = ole32.NewProc("CoInitializeEx")
	procCoUninitialize     = ole32.NewProc("CoUninitialize")
	procGetIconInfo        = user32.NewProc("GetIconInfo")
	procGetObjectW         = gdi32.NewProc("GetObjectW")
	procGetDIBits          = gdi32.NewProc("GetDIBits")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
)

const (
	shgfiIcon      = 0x000000100
	shgfiLargeIcon = 0x000000000
	shgfiSmallIcon = 0x000000001

	coinitApartmentThreaded = 0x2
	coinitDisableOLE1DDE    = 0x4
	rpcEChangedMode         = 0x80010106
)

// SHFILEINFOW is the SHFILEINFOW of shellapi.h.
type SHFILEINFOW struct {
	HIcon         uintptr
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [260]uint16
	SzTypeName    [80]uint16
}

// BITMAP is the BITMAP of wingdi.h.
type BITMAP struct {
	BmType       int32
	BmWidth      int32
	BmHeight     int32
	BmWidthBytes int32
	BmPlanes     uint16
	BmBitsPixel  uint16
	BmBits       uintptr
}

// FileIcon returns the icon of the file at path, about size pixels square: the
// file's own icon at exactly that size when the file holds icons (an executable,
// a library or an .ico file), or else the icon of its type, at the small or the
// large shell size, whichever is nearer. The caller destroys it with
// DestroyIcon. It returns 0 when the shell gives no icon.
//
// The shell needs COM on the calling thread, so FileIcon locks the goroutine to
// its thread and initializes COM for the call when the thread has none.
func FileIcon(path string, size int) uintptr {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded|coinitDisableOLE1DDE); uint32(hr) != rpcEChangedMode {
		defer procCoUninitialize.Call()
	}
	var icon uintptr
	if hr, _, _ := procSHDefExtractIconW.Call(uintptr(unsafe.Pointer(p)), 0, 0,
		uintptr(unsafe.Pointer(&icon)), 0, uintptr(size)); hr == 0 && icon != 0 {
		return icon
	}
	flags := uintptr(shgfiIcon | shgfiLargeIcon)
	if size <= 24 {
		flags = shgfiIcon | shgfiSmallIcon
	}
	var info SHFILEINFOW
	if r, _, _ := procSHGetFileInfoW.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info), flags); r == 0 {
		return 0
	}
	return info.HIcon
}

// IconImage returns the pixels of icon, with straight alpha. An icon without an
// alpha channel takes its transparency from its mask.
func IconImage(icon uintptr) (*image.NRGBA, error) {
	var ii ICONINFO
	if r, _, _ := procGetIconInfo.Call(icon, uintptr(unsafe.Pointer(&ii))); r == 0 {
		return nil, errors.New("win32: GetIconInfo failed")
	}
	defer procDeleteObject.Call(ii.HbmMask)
	if ii.HbmColor == 0 {
		return nil, errors.New("win32: the icon is monochrome")
	}
	defer procDeleteObject.Call(ii.HbmColor)
	var bm BITMAP
	if r, _, _ := procGetObjectW.Call(ii.HbmColor, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm))); r == 0 ||
		bm.BmWidth <= 0 || bm.BmHeight <= 0 {
		return nil, errors.New("win32: GetObject failed for the icon's bitmap")
	}
	w, h := int(bm.BmWidth), int(bm.BmHeight)
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return nil, errors.New("win32: CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(dc)
	color, ok := dibBits(dc, ii.HbmColor, w, h)
	if !ok {
		return nil, errors.New("win32: GetDIBits failed for the icon's colors")
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	hasAlpha := false
	for i := 3; i < len(color); i += 4 {
		if color[i] != 0 {
			hasAlpha = true
			break
		}
	}
	var mask []byte
	if !hasAlpha {
		// The mask is the top half of a monochrome bitmap twice the height, or a
		// bitmap of the icon's size; a white pixel in it is transparent.
		if mask, ok = dibBits(dc, ii.HbmMask, w, h); !ok {
			return nil, errors.New("win32: GetDIBits failed for the icon's mask")
		}
	}
	for i := 0; i < w*h*4; i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = color[i+2], color[i+1], color[i]
		switch {
		case hasAlpha:
			img.Pix[i+3] = color[i+3]
		case mask[i] == 0:
			img.Pix[i+3] = 0xff
		}
	}
	return img, nil
}

// dibBits returns the top w×h of the bitmap hbm as 32-bit blue, green, red and
// unused or alpha bytes, top row first.
func dibBits(dc, hbm uintptr, w, h int) ([]byte, bool) {
	var bmi struct {
		Header BITMAPINFOHEADER
		Colors [256]uint32 // room for a color table, which 32-bit bitmaps do not use
	}
	bmi.Header = BITMAPINFOHEADER{Size: uint32(unsafe.Sizeof(BITMAPINFOHEADER{})), Width: int32(w),
		Height: -int32(h), Planes: 1, BitCount: 32, Compression: BI_RGB}
	bits := make([]byte, w*h*4)
	r, _, _ := procGetDIBits.Call(dc, hbm, 0, uintptr(h), uintptr(unsafe.Pointer(&bits[0])),
		uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS)
	return bits, r != 0
}

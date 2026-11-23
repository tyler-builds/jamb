//go:build windows

package windows

import (
	"errors"
	"image"

	"golang.org/x/image/draw"

	"github.com/tyler-builds/jamb/internal/win32"
)

// exeIconResource is the ID of the icon that the packaging tool embeds in the
// executable (cmd/jamb, which writes the icon as resource 1).
const exeIconResource = 1

// smallIconMetric is the system's small icon size at dpi.
func smallIconMetric(dpi uint32) int32 { return win32.SystemMetricsForDpi(win32.SM_CXSMICON, dpi) }

// bigIconMetric is the system's big icon size at dpi.
func bigIconMetric(dpi uint32) int32 { return win32.SystemMetricsForDpi(win32.SM_CXICON, dpi) }

// iconPixels returns img scaled to size pixels square, as the rows CreateIcon
// takes: 32-bit blue, green, red, alpha, top row first, straight alpha.
func iconPixels(img image.Image, size int) []byte {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	out := make([]byte, len(dst.Pix))
	for i := 0; i < len(out); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = dst.Pix[i+2], dst.Pix[i+1], dst.Pix[i], dst.Pix[i+3]
	}
	return out
}

// newIcon returns an icon of img at size pixels square. The caller destroys it.
func newIcon(img image.Image, size int) (uintptr, error) {
	if img.Bounds().Empty() {
		return 0, errors.New("platform/windows: the icon image is empty")
	}
	icon := win32.CreateIcon(size, iconPixels(img, size))
	if icon == 0 {
		return 0, errors.New("platform/windows: CreateIconIndirect failed")
	}
	return icon, nil
}

// makeIcon returns the icon for img at size pixels square, or the executable's
// icon when img is nil. owned reports whether the caller destroys the icon; the
// standard application icon, used when the executable has none, is shared.
func makeIcon(img image.Image, size int) (icon uintptr, owned bool, err error) {
	if img != nil {
		icon, err = newIcon(img, size)
		return icon, err == nil, err
	}
	if icon := win32.LoadIcon(win32.GetModuleHandle(), exeIconResource, size, false); icon != 0 {
		return icon, true, nil
	}
	return win32.LoadIcon(0, win32.IDI_APPLICATION, size, true), false, nil
}

// windowIcon returns the icon of a window at size pixels square: img when it is
// set, or else the executable's icon. The caller destroys it. It returns 0 when
// img is nil and the executable has no icon, as a program built without jamb
// package has none: such a window keeps the icon Windows gives it.
func windowIcon(img image.Image, size int) uintptr {
	if img != nil {
		if icon, err := newIcon(img, size); err == nil {
			return icon
		}
	}
	return win32.LoadIcon(win32.GetModuleHandle(), exeIconResource, size, false)
}

// windowIcons are the small and big icons a window has been given with
// WM_SETICON, and the sizes they were made at. The window made them, and
// destroys them when it gets others or is destroyed.
type windowIcons struct {
	small, big         uintptr
	smallSize, bigSize int
}

// updateIcons gives the window its small and big icons at the sizes of its DPI,
// from WindowOptions.Icon or the executable's icon, each made at exactly its
// size rather than one size stretched. It does nothing when the sizes have not
// changed, so it runs again cheaply on every DPI change. A window without an
// icon to give keeps the one Windows gives it.
func (w *window) updateIcons() {
	small, big := iconSize(w.dpi, smallIconMetric), bigIconSize(w.dpi, bigIconMetric)
	if small == w.icons.smallSize && big == w.icons.bigSize {
		return
	}
	w.icons.smallSize, w.icons.bigSize = small, big
	w.icons.small = w.replaceIcon(win32.ICON_SMALL, w.icons.small, windowIcon(w.opts.Icon, small))
	w.icons.big = w.replaceIcon(win32.ICON_BIG, w.icons.big, windowIcon(w.opts.Icon, big))
}

// replaceIcon gives the window icon as its small or big icon, destroys old, and
// returns the icon the window now has. An icon of 0 leaves old in place.
func (w *window) replaceIcon(which int, old, icon uintptr) uintptr {
	if icon == 0 {
		return old
	}
	win32.SetWindowIcon(w.hwnd, which, icon)
	win32.DestroyIcon(old)
	return icon
}

// destroyIcons takes the window's icons from it and destroys them, as the
// window is destroyed.
func (w *window) destroyIcons() {
	if w.icons.small != 0 {
		win32.SetWindowIcon(w.hwnd, win32.ICON_SMALL, 0)
	}
	if w.icons.big != 0 {
		win32.SetWindowIcon(w.hwnd, win32.ICON_BIG, 0)
	}
	win32.DestroyIcon(w.icons.small)
	win32.DestroyIcon(w.icons.big)
	w.icons = windowIcons{}
}

// classIcons returns the big and small icons of the window class, from the
// executable's icon at the sizes of the system DPI, or 0 when the executable has
// none. The class keeps them for the life of the process. Each window gets its
// own at its DPI (see updateIcons); the class's stand in where Windows reads the
// class rather than the window.
func classIcons() (big, small uintptr) {
	dpi := win32.GetDpiForSystem()
	inst := win32.GetModuleHandle()
	big = win32.LoadIcon(inst, exeIconResource, bigIconSize(dpi, bigIconMetric), false)
	small = win32.LoadIcon(inst, exeIconResource, iconSize(dpi, smallIconMetric), false)
	return big, small
}

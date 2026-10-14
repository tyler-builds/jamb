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

// iconSize returns the side, in pixels, of a small icon at dpi, such as a
// notification area icon or a taskbar overlay. metric is
// GetSystemMetricsForDpi(SM_CXSMICON); when it reports nothing, the size is
// 16 pixels at 96 DPI, scaled.
func iconSize(dpi uint32, metric func(dpi uint32) int32) int {
	if s := int(metric(dpi)); s > 0 {
		return s
	}
	return max(16, (16*int(dpi)+48)/96)
}

// smallIconMetric is the system's small icon size at dpi.
func smallIconMetric(dpi uint32) int32 { return win32.SystemMetricsForDpi(win32.SM_CXSMICON, dpi) }

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

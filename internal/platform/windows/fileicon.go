//go:build windows

package windows

import (
	"errors"
	"image"

	"golang.org/x/image/draw"

	"github.com/tyler-builds/jamb/internal/win32"
)

// FileIcon returns the icon of the file at path, size pixels square: its own icon
// when it holds one, or the icon of its type, scaled to size when the shell gives
// another size.
func FileIcon(path string, size int) (image.Image, error) {
	if size <= 0 {
		return nil, errors.New("platform/windows: the icon size must be above zero")
	}
	icon := win32.FileIcon(path, size)
	if icon == 0 {
		return nil, errors.New("platform/windows: Windows has no icon for " + path)
	}
	defer win32.DestroyIcon(icon)
	img, err := win32.IconImage(icon)
	if err != nil {
		return nil, err
	}
	if img.Bounds().Dx() == size && img.Bounds().Dy() == size {
		return img, nil
	}
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst, nil
}

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

const (
	// minIconSize is the smallest source the icon is accepted at: the 48 px
	// image of the .ico must not be an upscale.
	minIconSize = 48
	// recommendedIconSize is the size below which packaging warns that the
	// icon will look soft at high DPI.
	recommendedIconSize = 256
)

// icoSizes are the images of the .ico file, one per size Windows shows
// (small icons, taskbar, Explorer and the large views).
var icoSizes = []int{16, 24, 32, 48, 64, 128, 256}

// msixLogos are the logo files of the MSIX package, each with its size at
// 100% scale. Every logo is written at 100%, 200% and 400% scale.
var msixLogos = []struct {
	name string
	size int
}{
	{"StoreLogo", 50},
	{"Square44x44Logo", 44},
	{"Square150x150Logo", 150},
}

// msixScales are the scale factors, in percent, of the MSIX logo files.
var msixScales = []int{100, 200, 400}

// loadIcon reads the application icon, which must be a square PNG of at least
// minIconSize pixels.
func loadIcon(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("icon %s must be a PNG file: %v", path, err)
	}
	b := img.Bounds()
	if b.Dx() != b.Dy() {
		return nil, fmt.Errorf("icon %s is %dx%d; it must be square", path, b.Dx(), b.Dy())
	}
	if b.Dx() < minIconSize {
		return nil, fmt.Errorf("icon %s is %dx%d; it must be at least %dx%d", path, b.Dx(), b.Dy(), minIconSize, minIconSize)
	}
	return img, nil
}

// scaled returns src resized to a size×size image.
func scaled(src image.Image, size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// iconImages returns src at each size of icoSizes, in that order.
func iconImages(src image.Image) []image.Image {
	images := make([]image.Image, 0, len(icoSizes))
	for _, size := range icoSizes {
		images = append(images, scaled(src, size))
	}
	return images
}

// writeLogos writes the MSIX logo files of the package into dir. The base file
// of each logo is its 100% image, and the scaled files sit beside it, as the
// MSIX packaging rules expect.
func writeLogos(dir string, src image.Image) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, logo := range msixLogos {
		for _, scale := range msixScales {
			name := logo.name + ".png"
			if scale != 100 {
				name = fmt.Sprintf("%s.scale-%d.png", logo.name, scale)
			}
			if err := writePNG(filepath.Join(dir, name), scaled(src, logo.size*scale/100)); err != nil {
				return err
			}
		}
	}
	return nil
}

// writePNG encodes img to a new file at path.
func writePNG(path string, img image.Image) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return png.Encode(f, img)
}

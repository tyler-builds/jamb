package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // register decoders for ImageFromBytes
	_ "image/png"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/scene"
)

// IconElement draws a vector icon from the icons package.
type IconElement struct {
	icon  *icons.Icon
	size  float32
	color Color
	style Style
	key   string
}

// Icon creates an icon element. Size defaults to 16 and the color is
// inherited like text.
func Icon(ic *icons.Icon) *IconElement { return &IconElement{icon: ic, size: 16} }

func (*IconElement) isElement()           {}
func (i *IconElement) elementKey() string { return i.key }

// Size sets the icon's width and height in logical pixels.
func (i *IconElement) Size(px float32) *IconElement { i.size = px; return i }

// Color sets the icon color.
func (i *IconElement) Color(c Color) *IconElement { i.color = c; return i }

// Style merges layout fields (margins, alignment...) of s.
func (i *IconElement) Style(s Style) *IconElement { i.style = i.style.Merge(s); return i }

// ImageSource is decoded pixel data ready for the GPU. Create it once (for
// example at package init) and reuse it.
type ImageSource struct {
	tex    *scene.Texture
	width  int
	height int
}

// Size returns the image size in pixels.
func (s *ImageSource) Size() (int, int) { return s.width, s.height }

// ImageFromBytes decodes PNG or JPEG data.
func ImageFromBytes(data []byte) (*ImageSource, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ui.ImageFromBytes: cannot decode image (%v); supported formats are PNG and JPEG", err)
	}
	_ = format
	return ImageFromImage(img), nil
}

// MustImage is ImageFromBytes that panics on error; meant for embedded assets.
func MustImage(data []byte) *ImageSource {
	src, err := ImageFromBytes(data)
	if err != nil {
		panic(err)
	}
	return src
}

// ImageFromImage converts any image.Image.
func ImageFromImage(img image.Image) *ImageSource {
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy())) // premultiplied, as the renderer expects
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	tex := scene.NewTexture(b.Dx(), b.Dy(), scene.FormatRGBA8)
	copy(tex.Pix, rgba.Pix)
	return &ImageSource{tex: tex, width: b.Dx(), height: b.Dy()}
}

// ImageFit controls how an image fills its box.
type ImageFit uint8

const (
	// FitCover fills the box, cropping the image (the default).
	FitCover ImageFit = iota
	// FitContain shows the whole image, letterboxed.
	FitContain
	// FitFill stretches the image to the box.
	FitFill
)

// ImageElement draws an ImageSource.
type ImageElement struct {
	src   *ImageSource
	style Style
	fit   ImageFit
	key   string
}

// Image creates an image element. Without an explicit size it uses the
// image's pixel size as logical pixels.
func Image(src *ImageSource) *ImageElement { return &ImageElement{src: src} }

func (*ImageElement) isElement()           {}
func (i *ImageElement) elementKey() string { return i.key }

// Size sets width and height.
func (i *ImageElement) Size(w, h Length) *ImageElement {
	i.style.Width, i.style.Height = w, h
	return i
}

// Radius rounds the image corners.
func (i *ImageElement) Radius(r float32) *ImageElement { i.style.Radius = r; return i }

// Fit chooses cover (default), contain or fill.
func (i *ImageElement) Fit(f ImageFit) *ImageElement { i.fit = f; return i }

// Opacity fades the image.
func (i *ImageElement) Opacity(o float32) *ImageElement { i.style.Opacity = o; return i }

// Style merges layout fields of s.
func (i *ImageElement) Style(s Style) *ImageElement { i.style = i.style.Merge(s); return i }

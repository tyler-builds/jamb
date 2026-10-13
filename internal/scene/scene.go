// Package scene holds the small set of GPU-friendly primitives the ui package
// paints into and a renderer draws from.
//
// All coordinates are physical pixels with the origin at the top-left of the
// window. A Scene is retained: it is rebuilt only when the UI changes and its
// backing slices are reused between rebuilds, so steady-state painting does
// not allocate.
package scene

import "sync/atomic"

// Rect is an axis-aligned rectangle.
type Rect struct{ X, Y, W, H float32 }

func (r Rect) Right() float32  { return r.X + r.W }
func (r Rect) Bottom() float32 { return r.Y + r.H }
func (r Rect) Empty() bool     { return r.W <= 0 || r.H <= 0 }

// Contains reports whether the point is inside r.
func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}

// Intersect returns the overlap of r and o (possibly empty).
func (r Rect) Intersect(o Rect) Rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.Right(), o.Right()), min(r.Bottom(), o.Bottom())
	if x1 < x0 {
		x1 = x0
	}
	if y1 < y0 {
		y1 = y0
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// Color is a straight-alpha sRGB color with components in [0,1].
type Color struct{ R, G, B, A float32 }

// Corners holds per-corner radii (top-left, top-right, bottom-right, bottom-left).
type Corners struct{ TL, TR, BR, BL float32 }

// Uniform returns corners that all have radius r.
func Uniform(r float32) Corners { return Corners{r, r, r, r} }

// Sides holds one width per side, for borders: top, right, bottom, left.
type Sides struct{ Top, Right, Bottom, Left float32 }

// Quad is the workhorse primitive: a filled, optionally rounded and bordered
// rectangle. With Blur > 0 it is a soft drop shadow of the same shape instead,
// using Background as the shadow color.
type Quad struct {
	Bounds      Rect
	Background  Color
	BorderColor Color
	BorderWidth float32
	// BorderSides, when not zero, gives each side its own width in place of
	// BorderWidth. A rounded corner curves from one side into the next.
	BorderSides Sides
	Radii       Corners
	// Blur is the Gaussian sigma of a shadow in pixels. 0 = a crisp quad.
	Blur float32

	// Gradient replaces the fill with a linear gradient from Background to
	// Color2. Angle is in degrees, CSS convention: 0 runs bottom to top, 90
	// left to right, 180 top to bottom. Stop0 and Stop1 are the positions
	// (0..1 along the gradient line) where Background and Color2 apply; the
	// gradient is clamped outside them. Oklab interpolates in Oklab rather
	// than sRGB. The border and the shadow stay solid.
	Gradient bool
	Color2   Color
	Angle    float32
	Stop0    float32
	Stop1    float32
	Oklab    bool
}

// Glyph is one coverage-mask sprite (a text glyph or an icon) from a mask texture.
type Glyph struct {
	Bounds Rect
	// Src is the source rectangle in texels.
	Src   Rect
	Color Color
}

// GlyphRun is a batch of glyphs sharing one mask texture.
type GlyphRun struct {
	Texture *Texture
	Glyphs  []Glyph
}

// ImageDraw draws (part of) an RGBA texture.
type ImageDraw struct {
	Bounds  Rect
	Texture *Texture
	// Src is the source rectangle in texels; zero means the whole texture.
	Src     Rect
	Radii   Corners
	Opacity float32
}

// TextureFormat describes texel layout.
type TextureFormat uint8

const (
	// FormatAlpha8 is one byte of coverage per texel (glyph and icon masks).
	FormatAlpha8 TextureFormat = iota
	// FormatRGBA8 is premultiplied RGBA, 4 bytes per texel.
	FormatRGBA8
	// FormatSubpixel8 holds ClearType glyph masks: per-channel R, G, B coverage
	// in the first three bytes of each 4-byte texel. A glyph run from such a
	// texture is drawn with per-channel blending, so it is only correct on an
	// opaque target at pixel scale (no scaling transform).
	FormatSubpixel8
)

// TextParams tune the alpha correction applied to subpixel glyph masks. They
// mirror IDWriteRenderingParams.
type TextParams struct {
	Gamma    float32 // system text gamma, e.g. 2.2
	Contrast float32 // enhanced contrast, e.g. 0.5
	Level    float32 // ClearType level 0..1 (0 = grayscale)
}

var textureIDs atomic.Uint64

// Texture is CPU-side pixel data that a renderer mirrors onto the GPU.
// Whoever owns a texture bumps Version (and grows Dirty) after changing Pix;
// the renderer re-uploads only the dirty region.
type Texture struct {
	ID            uint64
	Width, Height int
	Format        TextureFormat
	Pix           []byte
	// Text is read for FormatSubpixel8 textures only.
	Text    TextParams
	Version uint64
	// Dirty is the region changed since the renderer last synced, in texels.
	// Renderers reset it after uploading.
	DirtyX0, DirtyY0, DirtyX1, DirtyY1 int
}

// NewTexture allocates a texture with zeroed pixels.
func NewTexture(w, h int, f TextureFormat) *Texture {
	t := &Texture{ID: textureIDs.Add(1), Width: w, Height: h, Format: f}
	t.Pix = make([]byte, w*h*t.BytesPerPixel())
	t.MarkDirty(0, 0, w, h)
	return t
}

// BytesPerPixel returns 1 for masks and 4 for RGBA and subpixel masks.
func (t *Texture) BytesPerPixel() int {
	if t.Format == FormatRGBA8 || t.Format == FormatSubpixel8 {
		return 4
	}
	return 1
}

// MarkDirty records that the given texel region changed.
func (t *Texture) MarkDirty(x0, y0, x1, y1 int) {
	if t.DirtyX1 <= t.DirtyX0 || t.DirtyY1 <= t.DirtyY0 {
		t.DirtyX0, t.DirtyY0, t.DirtyX1, t.DirtyY1 = x0, y0, x1, y1
	} else {
		t.DirtyX0, t.DirtyY0 = min(t.DirtyX0, x0), min(t.DirtyY0, y0)
		t.DirtyX1, t.DirtyY1 = max(t.DirtyX1, x1), max(t.DirtyY1, y1)
	}
	t.Version++
}

// ClearDirty is called by renderers after uploading.
func (t *Texture) ClearDirty() { t.DirtyX0, t.DirtyY0, t.DirtyX1, t.DirtyY1 = 0, 0, 0, 0 }

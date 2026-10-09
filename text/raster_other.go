//go:build !windows

package text

import (
	"image"

	"golang.org/x/image/font/sfnt"
)

// dwFace is the per-font platform rasterizer state; there is none off Windows.
type dwFace struct{}

// dwriteGlyph always reports that the platform rasterizer is unavailable, so
// glyphs use the pure-Go outline rasterizer.
func dwriteGlyph(f *Font, id sfnt.GlyphIndex, size, fx float32, alloc func(w, h int) *image.Alpha) (dx, dy int, m *image.Alpha, ok bool) {
	return 0, 0, nil, false
}

func dwriteGlyphClearType(f *Font, id sfnt.GlyphIndex, size, fx float32, bgr bool, alloc func(w, h int) *image.RGBA) (dx, dy int, m *image.RGBA, ok bool) {
	return 0, 0, nil, false
}

// platformSubpixel reports that there is no subpixel text off Windows.
func platformSubpixel() (s subpixelSettings, ok bool) { return s, false }

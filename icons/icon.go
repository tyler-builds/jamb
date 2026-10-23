// Package icons provides the Lucide icon set (https://lucide.dev, ISC license):
// every Lucide icon as icons.<CamelName>, such as icons.ArrowDown, and a
// rasterizer that draws them as anti-aliased coverage masks.
//
// The icons are generated from a pinned Lucide release (see generate.go and
// third_party/lucide). An icon's SVG is parsed on its first Rasterize, not when
// the package loads, and an icon that a binary does not name is not linked into it.
package icons

import (
	"image"
	"sync"

	"github.com/tyler-builds/jamb/internal/svg"
	"github.com/tyler-builds/jamb/internal/vector"
)

// svgStart is the root element of every icon: a 24 grid, 2-unit strokes with
// round caps and joins, and currentColor for the stroke.
const svgStart = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">`

// Icon is a vector icon on a 24×24 grid.
type Icon struct {
	// Name is the name of the icon in its icon set, such as "arrow-down". It
	// identifies the icon's drawing, so two icons with different drawings have
	// different names.
	Name string
	// src is the inside of the icon's SVG: its elements, without the root.
	src string

	// once parses src on the first Rasterize; doc is the result, nil for an
	// icon with no source.
	once sync.Once
	doc  *svg.Doc
}

// document returns the parsed SVG of the icon, parsing it once.
func (ic *Icon) document() *svg.Doc {
	ic.once.Do(func() {
		if ic.src == "" {
			return
		}
		doc, err := svg.Parse([]byte(svgStart + ic.src + "</svg>"))
		if err == nil {
			ic.doc = doc
		}
	})
	return ic.doc
}

// Rasterize draws the icon into dst, scaled to fill dst's bounds. The alpha of
// dst is the coverage of the icon: 0 outside its strokes and 255 inside them,
// with anti-aliased edges. A zero Icon draws nothing.
func (ic *Icon) Rasterize(dst *image.Alpha) {
	doc := ic.document()
	if doc == nil {
		return
	}
	b := dst.Bounds()
	w, h := b.Dx(), b.Dy()
	c := vector.NewCanvas(w, h)
	doc.Draw(c, float32(w), float32(h), vector.Color{R: 1, G: 1, B: 1, A: 1})
	img := c.Image()
	for y := range h {
		src := img.Pix[y*img.Stride : y*img.Stride+4*w]
		for x := range w {
			// The canvas is premultiplied RGBA drawn in white, so its alpha is the coverage.
			dst.Pix[y*dst.Stride+x] = src[4*x+3]
		}
	}
}

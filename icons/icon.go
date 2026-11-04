// Package icons provides the Lucide icon set (https://lucide.dev, ISC license):
// every Lucide icon as icons.<CamelName>, such as icons.ArrowDown, and a
// rasterizer that draws them as anti-aliased coverage masks.
//
// The icons are generated from a pinned Lucide release (see generate.go and
// third_party/lucide). An icon's SVG is parsed on its first Rasterize, not when
// the package loads, and an icon that a binary does not name is not linked into it.
package icons

import (
	"errors"
	"image"
	"sync"

	"github.com/tyler-builds/jamb/internal/svg"
	"github.com/tyler-builds/jamb/internal/vector"
)

// svgStart is the root element of every icon: a 24 grid, 2-unit strokes with
// round caps and joins, and currentColor for the stroke.
const svgStart = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">`

// Icon is a vector icon: a Lucide icon on a 24×24 grid, or an SVG document of
// its own made with New.
type Icon struct {
	// Name is the name of the icon in its icon set, such as "arrow-down". It
	// identifies the icon's drawing, so two icons with different drawings have
	// different names.
	Name string
	// src is the inside of the icon's SVG: its elements, without the root. For
	// an icon made with New it is the whole document (see whole).
	src string
	// whole is set when src is a whole SVG document, with its own root.
	whole bool
	// colored keeps the icon's own colors (see KeepColors).
	colored bool

	// once parses src on the first Rasterize; doc is the result, nil for an
	// icon with no source.
	once sync.Once
	doc  *svg.Doc
}

// NewOption changes how New draws an icon.
type NewOption uint8

const (
	// KeepColors draws the icon in the colors of its SVG, as a brand logo of
	// several colors needs, instead of in the color of the text around it or
	// the one ui.IconElement.Color sets. currentColor in its SVG is black.
	KeepColors NewOption = iota + 1
)

// New makes an icon from a whole SVG document, with its own root, viewBox,
// fills and strokes, such as a brand mark from Simple Icons:
//
//	github := icons.MustNew("brand-github", `<svg viewBox="0 0 24 24"><path d="M12 .3a12 12 0 0 0-3.8 23.4…"/></svg>`)
//	ui.Icon(github).Size(20).Color(t.TextMuted)
//
// The drawing is scaled to fit the icon's square, keeping its proportions and
// centered, as the viewBox's default preserveAspectRatio asks. Every part it
// paints is drawn in the icon's color, so a one-color logo tints as a Lucide
// icon does; KeepColors keeps the SVG's colors instead. name identifies the
// drawing, as Name does: give each drawing a name that no other icon has, such
// as one with a "brand-" prefix. New fails when the SVG does not parse.
func New(name, svgDoc string, opts ...NewOption) (*Icon, error) {
	if name == "" {
		return nil, errors.New("icons: New needs a name")
	}
	doc, err := svg.Parse([]byte(svgDoc))
	if err != nil {
		return nil, err
	}
	ic := &Icon{Name: name, src: svgDoc, whole: true}
	for _, o := range opts {
		if o == KeepColors {
			ic.colored = true
		}
	}
	ic.once.Do(func() { ic.doc = doc })
	return ic, nil
}

// MustNew is New for an SVG that is part of the program: it panics when the SVG
// does not parse.
func MustNew(name, svgDoc string, opts ...NewOption) *Icon {
	ic, err := New(name, svgDoc, opts...)
	if err != nil {
		panic(err)
	}
	return ic
}

// Colored reports whether the icon keeps its own colors (see KeepColors), so it
// is drawn with RasterizeRGBA and not tinted.
func (ic *Icon) Colored() bool { return ic.colored }

// document returns the parsed SVG of the icon, parsing it once.
func (ic *Icon) document() *svg.Doc {
	ic.once.Do(func() {
		if ic.src == "" {
			return
		}
		src := svgStart + ic.src + "</svg>"
		if ic.whole {
			src = ic.src
		}
		doc, err := svg.Parse([]byte(src))
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

// RasterizeRGBA draws the icon in its own colors into dst, scaled to fill dst's
// bounds, as premultiplied RGBA; currentColor is black. It is how an icon made
// with KeepColors is drawn. A zero Icon draws nothing.
func (ic *Icon) RasterizeRGBA(dst *image.RGBA) {
	doc := ic.document()
	if doc == nil {
		return
	}
	b := dst.Bounds()
	w, h := b.Dx(), b.Dy()
	c := vector.NewCanvas(w, h)
	doc.Draw(c, float32(w), float32(h), vector.Color{A: 1})
	img := c.Image()
	for y := range h {
		copy(dst.Pix[y*dst.Stride:y*dst.Stride+4*w], img.Pix[y*img.Stride:y*img.Stride+4*w])
	}
}

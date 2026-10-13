package ui

import (
	"fmt"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/svg"
	"github.com/tyler-builds/jamb/internal/vector"
)

// SVGSource is a parsed SVG image. Create it once (for example at package
// init, from an embedded file) and reuse it.
type SVGSource struct{ doc *svg.Doc }

// SVGFromBytes parses an SVG image. It draws the static parts of SVG that
// icon sets and design tools write: paths and the basic shapes, groups, use,
// transforms, fills and strokes in colors or gradients, opacity, clip paths,
// the viewBox and preserveAspectRatio, and styles from presentation
// attributes, style attributes and style sheets with type, class and id
// selectors. Text, filters, masks, patterns, markers, embedded images and
// animation draw nothing.
func SVGFromBytes(data []byte) (*SVGSource, error) {
	d, err := svg.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("ui.SVGFromBytes: %v", err)
	}
	return &SVGSource{doc: d}, nil
}

// MustSVG is SVGFromBytes that panics on error; meant for embedded assets.
func MustSVG(data []byte) *SVGSource {
	s, err := SVGFromBytes(data)
	if err != nil {
		panic(err)
	}
	return s
}

// Size returns the image's own size in logical pixels: its width and height,
// else its viewBox's.
func (s *SVGSource) Size() (w, h float32) { return s.doc.W, s.doc.H }

// SVGElement draws an SVGSource.
type SVGElement struct {
	src   *SVGSource
	style Style
	color Color
	alt   string
	key   string
}

// SVG creates an element that draws src, at its own size unless Size sets
// one; the image keeps its aspect ratio inside the box, as its
// preserveAspectRatio says. It is drawn at the display's resolution, so it is
// sharp at any size and zoom, and drawn again only when its size, scale or
// color changes.
//
// currentColor in the image is the inherited text color, as for an Icon, so
// a monochrome icon set (such as Lucide's or Heroicons' SVG files) follows the
// text and the theme; Color sets it instead.
//
//	//go:embed logo.svg
//	var logoSVG []byte
//	var logo = ui.MustSVG(logoSVG)
//
//	ui.SVG(logo).Size(ui.Px(120), ui.Px(40)).Alt("Acme")
func SVG(src *SVGSource) *SVGElement { return &SVGElement{src: src} }

func (*SVGElement) isElement()           {}
func (e *SVGElement) elementKey() string { return e.key }

// Key identifies the image among its siblings (see DivElement.Key).
func (e *SVGElement) Key(k string) *SVGElement { e.key = k; return e }

// Size sets the width and height of the box the image is fitted into.
func (e *SVGElement) Size(w, h Length) *SVGElement {
	e.style.Width, e.style.Height = w, h
	return e
}

// Color sets the color that currentColor in the image stands for.
func (e *SVGElement) Color(c Color) *SVGElement { e.color = c; return e }

// Alt describes the image for assistive technology, which then reads it as an
// image. Without it the image is decorative, as an Icon is, and left out.
func (e *SVGElement) Alt(s string) *SVGElement { e.alt = s; return e }

// Style merges layout fields (margins, alignment, opacity...) of s.
func (e *SVGElement) Style(s Style) *SVGElement { e.style = e.style.Merge(s); return e }

// svgKey is what an SVG's drawing depends on besides its size and the
// inherited text color.
type svgKey struct {
	src   *SVGSource
	color Color
}

func (e *SVGElement) expand(*Context) Element {
	src, color := e.src, e.color
	w, h := src.Size()
	st := Style{Width: Px(w), Height: Px(h), NoShrink: true}.Merge(e.style)
	d := Div().Style(st)
	d.canvas = func(p *Painter, w, h float32) {
		c := color
		if c.IsZero() || p.rt.sys.HighContrast {
			// High contrast draws icons in the text color (see Icon).
			c = p.TextColor()
		}
		src.doc.Draw(p.c, w, h, vector.Color{R: c.R, G: c.G, B: c.B, A: c.A})
	}
	d.canvasKey = svgKey{src, color}
	if e.alt != "" {
		d.Role(access.RoleImage).Label(e.alt)
	}
	return d
}

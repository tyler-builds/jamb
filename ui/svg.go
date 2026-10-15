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
	src    *SVGSource
	style  Style
	color  Color
	alt    string
	key    string
	fit    ImageFit // see Fit
	fitSet bool     // Fit was called; otherwise the image's preserveAspectRatio fits it
}

// SVG creates an element that draws src, at its own size unless Size sets
// one; the image keeps its aspect ratio inside the box, as its
// preserveAspectRatio says, unless Fit places it. It is drawn at the display's resolution, so it is
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

// Radius rounds the image corners.
func (e *SVGElement) Radius(r float32) *SVGElement { e.style.Radius = r; return e }

// Fit chooses cover, contain or fill, as it does for an Image: cover fills the
// box, cropping the image, contain shows the whole image, letterboxed, and fill
// stretches the image to the box. Without Fit the image is fitted as its
// preserveAspectRatio says, which is contain for most files.
func (e *SVGElement) Fit(f ImageFit) *SVGElement { e.fit, e.fitSet = f, true; return e }

// Opacity fades the image.
func (e *SVGElement) Opacity(o float32) *SVGElement { e.style.Opacity = o; return e }

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
	src    *SVGSource
	color  Color
	fit    ImageFit
	fitSet bool
	radii  Corners
}

func (e *SVGElement) expand(*Context) Element {
	src, color := e.src, e.color
	w, h := src.Size()
	st := Style{Width: Px(w), Height: Px(h), NoShrink: true}.Merge(e.style)
	fit, fitSet, radii := e.fit, e.fitSet, st.radii()
	d := Div().Style(st)
	d.canvas = func(p *Painter, w, h float32) {
		c := color
		if c.IsZero() || p.rt.sys.HighContrast {
			// High contrast draws icons in the text color (see Icon).
			c = p.TextColor()
		}
		src.draw(p.c, w, h, fit, fitSet, radii, vector.Color{R: c.R, G: c.G, B: c.B, A: c.A})
	}
	d.canvasKey = svgKey{src, color, fit, fitSet, radii}
	if e.alt != "" {
		d.Role(access.RoleImage).Label(e.alt)
	}
	return d
}

// draw draws the image in a box of w by h. With fitSet the image is placed by fit
// at its own size's aspect, as an Image is (see paintImage), and otherwise its
// preserveAspectRatio places it. The drawing is clipped to the box, or to the
// letterboxed image for FitContain, rounded by radii.
func (s *SVGSource) draw(c *vector.Canvas, w, h float32, fit ImageFit, fitSet bool, radii Corners, current vector.Color) {
	iw, ih := s.Size()
	c.Save()
	defer c.Restore()
	if !fitSet || iw <= 0 || ih <= 0 {
		if !radii.isZero() {
			clipRounded(c, 0, 0, w, h, radii)
		}
		s.doc.Draw(c, w, h, current)
		return
	}
	sx, sy := w/iw, h/ih
	x, y, cw, ch := float32(0), float32(0), w, h // the clip
	switch fit {
	case FitCover:
		sx = max(sx, sy)
		sy = sx
	case FitContain:
		sx = min(sx, sy)
		sy = sx
		cw, ch = iw*sx, ih*sy
		x, y = (w-cw)/2, (h-ch)/2
	}
	clipRounded(c, x, y, cw, ch, radii)
	// The image is drawn at its own size, where its preserveAspectRatio changes
	// nothing, and scaled into place.
	c.Transform(vector.Scale(sx, sy).Then(vector.Translate((w-iw*sx)/2, (h-ih*sy)/2)))
	s.doc.Draw(c, iw, ih, current)
}

// clipRounded clips c to the rectangle at x, y of w by h with the corners r, each
// limited to half the shorter side.
func clipRounded(c *vector.Canvas, x, y, w, h float32, r Corners) {
	lim := func(v float32) float32 { return min(max(v, 0), min(w, h)/2) }
	tl, tr, br, bl := lim(r.TopLeft), lim(r.TopRight), lim(r.BottomRight), lim(r.BottomLeft)
	var p vector.Path
	p.MoveTo(x+tl, y)
	p.LineTo(x+w-tr, y)
	p.ArcTo(tr, tr, 0, false, true, x+w, y+tr)
	p.LineTo(x+w, y+h-br)
	p.ArcTo(br, br, 0, false, true, x+w-br, y+h)
	p.LineTo(x+bl, y+h)
	p.ArcTo(bl, bl, 0, false, true, x, y+h-bl)
	p.LineTo(x, y+tl)
	p.ArcTo(tl, tl, 0, false, true, x+tl, y)
	p.Close()
	c.Clip(&p, vector.NonZero)
}

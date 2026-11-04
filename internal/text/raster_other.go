//go:build !windows

package text

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"math"

	tsfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype/tables"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
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

// hasColor reports whether f has color glyphs that the pure-Go color
// rasterizer draws: COLR (with CPAL), CBDT or sbix glyphs. SVG glyphs are not
// drawn in color.
func (f *Font) hasColor() bool { return f.colorTables }

// dwriteColorGlyph rasterizes glyph id of the color font f in color, in pure
// Go: from its COLR paint (colr.go), or from its CBDT or sbix bitmap, scaled to
// size. It takes the arguments of the Windows version, which uses DirectWrite,
// and ok is false when the glyph has no color data.
func dwriteColorGlyph(f *Font, id sfnt.GlyphIndex, size, fx float32, fg [3]float32, alloc func(w, h int) *image.RGBA) (dx, dy int, m *image.RGBA, ok bool) {
	if !f.colorTables || id > 0xFFFF {
		return 0, 0, nil, false
	}
	gid := tables.GlyphID(id)
	if colr := f.face.COLR; colr != nil && len(f.face.CPAL) > 0 {
		if root, found := colr.Search(gid); found {
			cf := f.colrFont(fg)
			var box *[4]float64
			if b, ok := cf.clipBox(uint16(id)); ok {
				box = &b
			}
			dx, dy, m = drawColr(cf, root, box, float64(f.upem), float64(size), float64(fx), alloc)
			return dx, dy, m, true
		}
	}
	return bitmapGlyph(f, tsfont.GID(id), size, fx, alloc)
}

// colrFont reads the paint graph of f's COLR table, with the text color fg.
func (f *Font) colrFont(fg [3]float32) *colrFont {
	colr := f.face.COLR
	var buf sfnt.Buffer
	ppem := fixed.Int26_6(f.upem * 64) // outlines in font units, as 26.6
	return &colrFont{
		outline: func(gid uint16) []pathSeg {
			segs, err := f.sf.LoadGlyph(&buf, sfnt.GlyphIndex(gid), ppem, nil)
			if err != nil {
				return nil
			}
			out := make([]pathSeg, len(segs))
			for i, s := range segs {
				out[i].op = uint8(s.Op) // move, line, quad, cubic, as pathSeg numbers them
				for j, a := range s.Args {
					// sfnt's y axis points down.
					out[i].pts[j] = [2]float64{float64(a.X) / 64, -float64(a.Y) / 64}
				}
			}
			return out
		},
		base: func(gid uint16) (tables.PaintTable, bool) { return colr.Search(tables.GlyphID(gid)) },
		layers: func(p tables.PaintColrLayers) ([]tables.PaintTable, error) {
			return colr.LayerList.Resolve(p)
		},
		clipBox: func(gid uint16) ([4]float64, bool) {
			switch b, _ := colr.ClipList.Search(tables.GlyphID(gid)); b := b.(type) {
			case tables.ClipBoxFormat1:
				return [4]float64{float64(b.XMin), float64(b.YMin), float64(b.XMax), float64(b.YMax)}, true
			case tables.ClipBoxFormat2:
				return [4]float64{float64(b.XMin), float64(b.YMin), float64(b.XMax), float64(b.YMax)}, true
			}
			return [4]float64{}, false
		},
		palette: f.face.CPAL[0],
		fg:      fg,
	}
}

// bitmapGlyph draws the PNG (or JPEG) bitmap of a CBDT or sbix glyph, from the
// largest strike, scaled to size and placed by the glyph's extents.
func bitmapGlyph(f *Font, gid tsfont.GID, size, fx float32, alloc func(w, h int) *image.RGBA) (dx, dy int, m *image.RGBA, ok bool) {
	bm, found := f.face.GlyphDataBitmap(gid)
	if !found {
		return 0, 0, nil, false
	}
	var src image.Image
	var err error
	switch bm.Format {
	case tsfont.PNG:
		src, err = png.Decode(bytes.NewReader(bm.Data))
	case tsfont.JPG:
		src, err = jpeg.Decode(bytes.NewReader(bm.Data))
	default:
		return 0, 0, nil, false // monochrome bitmaps: draw the outline
	}
	if err != nil {
		return 0, 0, nil, false
	}
	ext, found := f.face.GlyphExtents(gid)
	if !found || ext.Width <= 0 || ext.Height >= 0 {
		return 0, 0, nil, true // an empty glyph
	}
	s := size / f.upem
	left, top := ext.XBearing*s+fx, -ext.YBearing*s
	right, bottom := left+ext.Width*s, top-ext.Height*s
	x0, y0 := int(math.Floor(float64(left))), int(math.Floor(float64(top)))
	x1, y1 := int(math.Ceil(float64(right))), int(math.Ceil(float64(bottom)))
	if x1 <= x0 || y1 <= y0 || x1-x0 > int(4*size)+16 || y1-y0 > int(4*size)+16 {
		return 0, 0, nil, true
	}
	m = alloc(x1-x0, y1-y0)
	r := func(v float32) int { return int(math.Round(float64(v))) }
	dr := image.Rect(r(left)-x0, r(top)-y0, r(right)-x0, r(bottom)-y0)
	xdraw.CatmullRom.Scale(m, dr, src, src.Bounds(), xdraw.Over, nil)
	return x0, y0, m, true
}

// platformSubpixel reports that there is no subpixel text off Windows.
func platformSubpixel() (s subpixelSettings, ok bool) { return s, false }

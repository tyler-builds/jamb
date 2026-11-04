//go:build !windows

package text

import (
	"image"
	"math"
	"testing"

	"github.com/go-text/typesetting/font/opentype/tables"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func nearColor(c [4]float32, want [4]float32) bool {
	for i := range c {
		if math.Abs(float64(c[i]-want[i])) > 0.01 {
			return false
		}
	}
	return true
}

// Transforms compose right to left, rotate and skew counter-clockwise in font
// units (y up), and invert.
func TestAffine(t *testing.T) {
	x, y := rotation(0.5).apply(1, 0) // 90°
	if !near(x, 0) || !near(y, 1) {
		t.Errorf("rotating (1, 0) by 90° gives (%v, %v), want (0, 1)", x, y)
	}
	x, y = around(rotation(1), 10, 0).apply(20, 0) // 180° about (10, 0)
	if !near(x, 0) || !near(y, 0) {
		t.Errorf("rotating (20, 0) by 180° about (10, 0) gives (%v, %v), want (0, 0)", x, y)
	}
	// A skew in x by 45° counter-clockwise tilts the y axis to the left.
	x, y = skew(0.25, 0).apply(0, 1)
	if !near(x, -1) || !near(y, 1) {
		t.Errorf("x-skewing (0, 1) by 45° gives (%v, %v), want (-1, 1)", x, y)
	}
	// m.mul(t) applies t first.
	m := translate(5, 0).mul(affine{xx: 2, yy: 2})
	if x, y = m.apply(1, 1); !near(x, 7) || !near(y, 2) {
		t.Errorf("scale then translate gives (%v, %v), want (7, 2)", x, y)
	}
	inv, ok := m.mul(rotation(0.3)).invert()
	if !ok {
		t.Fatal("not invertible")
	}
	if x, y = inv.apply(m.mul(rotation(0.3)).apply(3, -4)); !near(x, 3) || !near(y, -4) {
		t.Errorf("the inverse maps back to (%v, %v), want (3, -4)", x, y)
	}
	if _, ok := (affine{xx: 1}).invert(); ok {
		t.Error("a singular transform inverts")
	}
}

var (
	red  = [4]float32{1, 0, 0, 1}
	blue = [4]float32{0, 0, 1, 1}
)

func redToBlue(e tables.Extend) colorLine {
	return colorLine{stops: []colorStop{{0, red}, {1, blue}}, extend: e}
}

// A color line interpolates between its stops and extends past them by
// padding, repeating or reflecting.
func TestColorLineExtend(t *testing.T) {
	mid := [4]float32{0.5, 0, 0.5, 1}
	quarter := [4]float32{0.75, 0, 0.25, 1}
	for _, c := range []struct {
		e    tables.Extend
		t    float64
		want [4]float32
	}{
		{tables.ExtendPad, 0.5, mid},
		{tables.ExtendPad, -3, red},
		{tables.ExtendPad, 7, blue},
		{tables.ExtendRepeat, 1.25, quarter},
		{tables.ExtendRepeat, -0.75, quarter},
		{tables.ExtendReflect, 1.75, quarter}, // 1.75 mirrors to 0.25
		{tables.ExtendReflect, -0.25, quarter},
	} {
		if got := redToBlue(c.e).at(c.t); !nearColor(got, c.want) {
			t.Errorf("extend %d at %v: %v, want %v", c.e, c.t, got, c.want)
		}
	}
	// Stops out of order are sorted, and premultiplied colors interpolate.
	r := &colrRender{f: &colrFont{palette: []tables.ColorRecord{{Red: 255, Alpha: 255}, {Blue: 255, Alpha: 0}}}}
	l := r.colorLine(tables.ColorLine{ColorStops: []tables.ColorStop{
		{StopOffset: 16384, PaletteIndex: 1, Alpha: 16384},
		{StopOffset: 0, PaletteIndex: 0, Alpha: 16384},
	}})
	if got := l.at(0.5); !nearColor(got, [4]float32{0.5, 0, 0, 0.5}) {
		t.Errorf("red to transparent at 0.5: %v, want half red at half alpha", got)
	}
}

// A linear gradient runs along p0p1, projected onto the normal of p0p2.
func TestLinearGradient(t *testing.T) {
	g := linearGradient(redToBlue(tables.ExtendPad), 0, 0, 100, 0, 0, 100)
	if got := g(50, 77); !nearColor(got, [4]float32{0.5, 0, 0.5, 1}) {
		t.Errorf("halfway: %v", got)
	}
	// With p2 on the diagonal, lines of one color run along the diagonal,
	// and p1 = (100, 0) projects to (50, -50).
	g = linearGradient(redToBlue(tables.ExtendPad), 0, 0, 100, 0, 100, 100)
	if got := g(30, 30); !nearColor(got, red) {
		t.Errorf("on the diagonal: %v, want the start color", got)
	}
	if got := g(50+40, -50+40); !nearColor(got, blue) {
		t.Errorf("on the parallel through (50, -50): %v, want the end color", got)
	}
	if linearGradient(redToBlue(tables.ExtendPad), 0, 0, 0, 0, 0, 100) != nil {
		t.Error("a gradient of zero length paints")
	}
}

// A radial gradient between concentric circles is a plain radial gradient;
// between two circles that do not nest, points outside the cone they span
// are not painted.
func TestRadialGradient(t *testing.T) {
	g := radialGradient(redToBlue(tables.ExtendPad), 0, 0, 0, 0, 0, 100)
	if got := g(30, 40); !nearColor(got, [4]float32{0.5, 0, 0.5, 1}) {
		t.Errorf("at radius 50 of 100: %v", got)
	}
	if got := g(300, 0); !nearColor(got, blue) {
		t.Errorf("outside the end circle: %v, want the end color", got)
	}
	// Two circles of radius 10 side by side span a band 20 units wide.
	g = radialGradient(redToBlue(tables.ExtendPad), 0, 0, 10, 100, 0, 10)
	// (50, 0) is on the circles of t = 0.4 and t = 0.6; the larger wins.
	if got := g(50, 0); !nearColor(got, [4]float32{0.4, 0, 0.6, 1}) {
		t.Errorf("inside the band: %v, want the color at 0.6", got)
	}
	if got := g(50, 50); got[3] != 0 {
		t.Errorf("outside the band: %v, want nothing", got)
	}
}

// A sweep gradient runs counter-clockwise from its start angle.
func TestSweepGradient(t *testing.T) {
	// Angles are stored as value + 1 in turns of 180°: -1 is 0°, 1 is 360°.
	g := sweepGradient(redToBlue(tables.ExtendPad), 0, 0, -16384, 16384)
	if got := g(0, 10); !nearColor(got, [4]float32{0.75, 0, 0.25, 1}) {
		t.Errorf("at 90°: %v, want a quarter of the way", got)
	}
	if got := g(0, -10); !nearColor(got, [4]float32{0.25, 0, 0.75, 1}) {
		t.Errorf("at 270°: %v, want three quarters of the way", got)
	}
}

// squares is a synthetic font: glyph 1 is the square (0, 0)–(100, 100) and
// glyph 2 the square (50, 50)–(100, 100), in font units; palette entries 0,
// 1 and 2 are red, green and blue.
func squares(base map[uint16]tables.PaintTable, layers []tables.PaintTable) *colrFont {
	sq := func(x0, y0, x1, y1 float64) []pathSeg { return boxPath([4]float64{x0, y0, x1, y1}) }
	return &colrFont{
		outline: func(gid uint16) []pathSeg {
			switch gid {
			case 1:
				return sq(0, 0, 100, 100)
			case 2:
				return sq(50, 50, 100, 100)
			}
			return nil
		},
		base: func(gid uint16) (tables.PaintTable, bool) { p, ok := base[gid]; return p, ok },
		layers: func(p tables.PaintColrLayers) ([]tables.PaintTable, error) {
			return layers[p.FirstLayerIndex : p.FirstLayerIndex+uint32(p.NumLayers)], nil
		},
		clipBox: func(uint16) ([4]float64, bool) { return [4]float64{}, false },
		palette: []tables.ColorRecord{{Red: 255, Alpha: 255}, {Green: 255, Alpha: 255}, {Blue: 255, Alpha: 255}},
		fg:      [3]float32{1, 1, 1},
	}
}

func solid(i uint16) tables.PaintSolid { return tables.PaintSolid{PaletteIndex: i, Alpha: 16384} }

// render draws root at one pixel per font unit and returns the image and the
// pixel at font units (x, y).
func render(t *testing.T, f *colrFont, root tables.PaintTable) (*image.RGBA, func(x, y int) [4]uint8) {
	t.Helper()
	dx, dy, m := drawColr(f, root, nil, 100, 100, 0, func(w, h int) *image.RGBA { return image.NewRGBA(image.Rect(0, 0, w, h)) })
	if m == nil {
		t.Fatal("nothing drawn")
	}
	return m, func(x, y int) [4]uint8 {
		px, py := x-dx, -y-dy-1 // the pixel whose bottom-left corner is (x, y)
		if !(image.Point{px, py}).In(m.Rect) {
			return [4]uint8{}
		}
		return [4]uint8(m.Pix[m.PixOffset(px, py):][:4])
	}
}

// The paint graph draws layers in order, each glyph clipping its paint, and
// transforms apply to what they wrap.
func TestPaintGraphLayersAndTransforms(t *testing.T) {
	layers := []tables.PaintTable{
		tables.PaintGlyph{GlyphID: 1, Paint: solid(0)},
		tables.PaintTranslate{Dx: -50, Dy: -50, Paint: tables.PaintGlyph{GlyphID: 2, Paint: solid(1)}},
	}
	f := squares(nil, layers)
	m, at := render(t, f, tables.PaintColrLayers{NumLayers: 2})
	if m.Rect.Dx() != 100 || m.Rect.Dy() != 100 {
		t.Errorf("image %v, want the 100×100 bounds of the outlines", m.Rect)
	}
	if got := at(75, 75); got != [4]uint8{255, 0, 0, 255} {
		t.Errorf("top right: %v, want red", got)
	}
	if got := at(25, 25); got != [4]uint8{0, 255, 0, 255} {
		t.Errorf("bottom left: %v, want the translated green square", got)
	}

	// A 90° rotation about the center turns glyph 2 from the top right to
	// the top left.
	rot := tables.PaintRotateAroundCenter{Angle: 8192, CenterX: 50, CenterY: 50,
		Paint: tables.PaintGlyph{GlyphID: 2, Paint: solid(2)}}
	f2 := squares(nil, []tables.PaintTable{tables.PaintGlyph{GlyphID: 1, Paint: solid(0)}, rot})
	_, at = render(t, f2, tables.PaintColrLayers{NumLayers: 2})
	if got := at(25, 75); got != [4]uint8{0, 0, 255, 255} {
		t.Errorf("top left after rotating: %v, want blue", got)
	}
	if got := at(75, 75); got != [4]uint8{255, 0, 0, 255} {
		t.Errorf("top right after rotating: %v, want red", got)
	}
}

// A gradient inside a glyph is clipped by the outline and varies across it.
func TestPaintGraphGradientInGlyph(t *testing.T) {
	grad := tables.PaintLinearGradient{
		ColorLine: tables.ColorLine{ColorStops: []tables.ColorStop{
			{StopOffset: 0, PaletteIndex: 0, Alpha: 16384},
			{StopOffset: 16384, PaletteIndex: 2, Alpha: 16384},
		}},
		X0: 0, Y0: 0, X1: 100, Y1: 0, X2: 0, Y2: 100,
	}
	f := squares(nil, []tables.PaintTable{
		tables.PaintGlyph{GlyphID: 1, Paint: solid(1)},
		tables.PaintGlyph{GlyphID: 2, Paint: grad},
	})
	_, at := render(t, f, tables.PaintColrLayers{NumLayers: 2})
	if got := at(25, 25); got != [4]uint8{0, 255, 0, 255} {
		t.Errorf("outside glyph 2: %v, want green", got)
	}
	left, right := at(51, 75), at(98, 75)
	if !(left[0] > 100 && left[2] < 155 && right[2] > 230 && right[0] < 25) {
		t.Errorf("the gradient goes from %v to %v, want from half red to blue", left, right)
	}
}

// Composites: the source is drawn over the backdrop by the mode, and the
// result over what is below.
func TestPaintGraphComposite(t *testing.T) {
	// Destination-out cuts glyph 2 out of glyph 1.
	f := squares(nil, nil)
	_, at := render(t, f, tables.PaintComposite{
		BackdropPaint: tables.PaintGlyph{GlyphID: 1, Paint: solid(0)},
		SourcePaint:   tables.PaintGlyph{GlyphID: 2, Paint: solid(2)},
		CompositeMode: tables.CompositeDestOut,
	})
	if got := at(25, 25); got != [4]uint8{255, 0, 0, 255} {
		t.Errorf("outside the cut: %v, want red", got)
	}
	if got := at(75, 75); got[3] != 0 {
		t.Errorf("inside the cut: %v, want nothing", got)
	}
	// Source-in keeps the source only where the backdrop is.
	_, at = render(t, f, tables.PaintComposite{
		BackdropPaint: tables.PaintGlyph{GlyphID: 2, Paint: solid(0)},
		SourcePaint:   tables.PaintGlyph{GlyphID: 1, Paint: solid(2)},
		CompositeMode: tables.CompositeSrcIn,
	})
	if got := at(25, 25); got[3] != 0 {
		t.Errorf("source-in outside the backdrop: %v, want nothing", got)
	}
	if got := at(75, 75); got != [4]uint8{0, 0, 255, 255} {
		t.Errorf("source-in inside the backdrop: %v, want blue", got)
	}
}

func TestCompositeBlendModes(t *testing.T) {
	gray := func(v float32) [4]float32 { return [4]float32{v, v, v, 1} }
	for _, c := range []struct {
		mode tables.CompositeMode
		d, s [4]float32
		want float32
	}{
		{tables.CompositeMultiply, gray(0.5), gray(0.5), 0.25},
		{tables.CompositeScreen, gray(0.5), gray(0.5), 0.75},
		{tables.CompositeDarken, gray(0.2), gray(0.6), 0.2},
		{tables.CompositeLighten, gray(0.2), gray(0.6), 0.6},
		{tables.CompositeDifference, gray(0.2), gray(0.6), 0.4},
		{tables.CompositePlus, gray(0.7), gray(0.6), 1},
		{tables.CompositeHslHue, gray(0.2), gray(0.6), 0.6}, // approximated by source-over
	} {
		d := c.d
		compositePixel(d[:], c.s, c.mode)
		if math.Abs(float64(d[0]-c.want)) > 1e-5 || d[3] != 1 {
			t.Errorf("mode %d: %v, want %v", c.mode, d, c.want)
		}
	}
}

// A color glyph that refers to itself is drawn once, not forever, and the
// text color is palette index 0xFFFF.
func TestPaintColrGlyphCycleAndForeground(t *testing.T) {
	base := map[uint16]tables.PaintTable{}
	base[7] = tables.PaintColrLayers{NumLayers: 2}
	f := squares(base, []tables.PaintTable{
		tables.PaintGlyph{GlyphID: 1, Paint: tables.PaintSolid{PaletteIndex: 0xFFFF, Alpha: 16384}},
		tables.PaintColrGlyph{GlyphID: 7},
	})
	f.fg = [3]float32{0, 1, 0}
	_, at := render(t, f, tables.PaintColrGlyph{GlyphID: 7})
	if got := at(50, 50); got != [4]uint8{0, 255, 0, 255} {
		t.Errorf("%v, want the text color, green", got)
	}
}

// Version 0 layers are glyphs of one palette color each.
func TestColrV0Layers(t *testing.T) {
	f := squares(nil, nil)
	_, at := render(t, f, tables.PaintColrLayersResolved{{GlyphID: 1, PaletteIndex: 2}, {GlyphID: 2, PaletteIndex: 0}})
	if got := at(25, 25); got != [4]uint8{0, 0, 255, 255} {
		t.Errorf("bottom left: %v, want blue", got)
	}
	if got := at(75, 75); got != [4]uint8{255, 0, 0, 255} {
		t.Errorf("top right: %v, want red", got)
	}
}

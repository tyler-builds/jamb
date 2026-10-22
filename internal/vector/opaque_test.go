package vector

import (
	"bytes"
	"testing"
)

// TestOpaqueSolidMatchesBlend checks that an opaque solid fill, which writes the
// pixels it fully covers without blending, gives the same bytes as the blend. The
// same color as a gradient of two equal stops takes the blend path for every pixel,
// so the two canvases must match byte for byte, edges and translucent layers included.
func TestOpaqueSolidMatchesBlend(t *testing.T) {
	color := Color{R: 0.2, G: 0.5, B: 0.9, A: 1}
	same := &Gradient{Kind: Linear, X0: 0, Y0: 0, X1: 64, Y1: 0,
		Stops: []Stop{{Offset: 0, Color: color}, {Offset: 1, Color: color}}}
	var ellipse, rect Path
	ellipse.Ellipse(30.3, 28.7, 21.6, 17.2)
	rect.Rect(4.25, 6.5, 40.75, 33.3)
	draw := func(paint Paint) []byte {
		c := NewCanvas(64, 64)
		c.Fill(&rect, Paint{Color: Color{R: 1, G: 1, B: 0, A: 0.5}}, NonZero)
		c.Fill(&ellipse, paint, NonZero)
		c.Fill(&rect, Paint{Color: Color{R: 0.1, G: 0.1, B: 0.1, A: 1}}, NonZero)
		c.Fill(&ellipse, Paint{Color: Color{R: 0.3, G: 0.6, B: 1, A: 0.25}}, NonZero)
		return append([]byte(nil), c.Image().Pix...)
	}
	solid := draw(Paint{Color: color})
	blend := draw(Paint{Gradient: same})
	if !bytes.Equal(solid, blend) {
		for i := range solid {
			if solid[i] != blend[i] {
				t.Fatalf("byte %d: solid %d, blend %d", i, solid[i], blend[i])
			}
		}
	}
}

// TestShaderInitReusesTheCanvas checks that a canvas paints each fill with the paint
// of that fill, when a gradient and a solid color follow each other.
func TestShaderInitReusesTheCanvas(t *testing.T) {
	var p Path
	p.Rect(0, 0, 8, 8)
	c := NewCanvas(8, 8)
	c.Fill(&p, Paint{Gradient: &Gradient{Kind: Linear, X0: 0, Y0: 0, X1: 8, Y1: 0,
		Stops: []Stop{{Offset: 0, Color: Color{R: 1, A: 1}}, {Offset: 1, Color: Color{B: 1, A: 1}}}}}, NonZero)
	c.Fill(&p, red, NonZero)
	if got := rgbaAt(c, 2, 2); got != [4]uint8{255, 0, 0, 255} {
		t.Fatalf("a solid fill after a gradient painted %v", got)
	}
}

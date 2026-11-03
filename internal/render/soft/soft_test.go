package soft

import (
	"image"
	"math"
	"testing"

	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/scene"
)

var _ render.Renderer = (*Renderer)(nil)

var (
	black = scene.Color{A: 1}
	white = scene.Color{R: 1, G: 1, B: 1, A: 1}
	red   = scene.Color{R: 1, A: 1}
)

// frame draws one frame of the given size, cleared to black, and returns it.
func frame(t *testing.T, w, h int, draw func(r *Renderer)) *image.RGBA {
	t.Helper()
	r := New(nil)
	if err := r.BeginFrame(w, h, black); err != nil {
		t.Fatal(err)
	}
	draw(r)
	if err := r.EndFrame(); err != nil {
		t.Fatal(err)
	}
	return r.Image()
}

func at(img *image.RGBA, x, y int) [4]byte {
	i := img.PixOffset(x, y)
	return [4]byte(img.Pix[i : i+4])
}

func TestClearAndSolidQuad(t *testing.T) {
	img := frame(t, 8, 8, func(r *Renderer) {
		r.DrawQuads([]scene.Quad{{Bounds: scene.Rect{X: 2, Y: 2, W: 4, H: 4}, Background: red}})
	})
	for y := range 8 {
		for x := range 8 {
			want := [4]byte{0, 0, 0, 255}
			if x >= 2 && x < 6 && y >= 2 && y < 6 {
				want = [4]byte{255, 0, 0, 255}
			}
			if got := at(img, x, y); got != want {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
			}
		}
	}
}

// A quad edge through the middle of a pixel covers it by half.
func TestHalfPixelEdge(t *testing.T) {
	img := frame(t, 4, 1, func(r *Renderer) {
		r.DrawQuads([]scene.Quad{{Bounds: scene.Rect{X: 0, Y: -1, W: 1.5, H: 3}, Background: white}})
	})
	if got := at(img, 0, 0)[0]; got != 255 {
		t.Errorf("covered pixel = %d, want 255", got)
	}
	if got := at(img, 1, 0)[0]; got < 126 || got > 129 {
		t.Errorf("half-covered pixel = %d, want about 128", got)
	}
	if got := at(img, 2, 0)[0]; got != 0 {
		t.Errorf("uncovered pixel = %d, want 0", got)
	}
}

// A translucent fill blends over what is below, premultiplied.
func TestTranslucentBlend(t *testing.T) {
	img := frame(t, 2, 2, func(r *Renderer) {
		r.DrawQuads([]scene.Quad{{Bounds: scene.Rect{W: 2, H: 2}, Background: scene.Color{R: 1, G: 1, B: 1, A: 0.5}}})
	})
	if got := at(img, 0, 0); got != [4]byte{128, 128, 128, 255} {
		t.Errorf("50%% white over black = %v", got)
	}
}

// A rounded corner leaves its corner pixel uncovered and its middle filled,
// and the fast span fill agrees with per-pixel shading.
func TestRoundedQuadMatchesShading(t *testing.T) {
	q := scene.Quad{Bounds: scene.Rect{X: 1.25, Y: 2.5, W: 40, H: 30}, Background: scene.Color{R: 0.2, G: 0.6, B: 0.9, A: 0.8},
		Radii: scene.Corners{TL: 8, TR: 2, BR: 12, BL: 0}, BorderColor: white, BorderSides: scene.Sides{Top: 1, Right: 2, Bottom: 3, Left: 1}}
	img := frame(t, 48, 40, func(r *Renderer) { r.DrawQuads([]scene.Quad{q}) })
	if got := at(img, 1, 2); got != [4]byte{0, 0, 0, 255} {
		t.Errorf("the rounded corner's pixel = %v, want the background", got)
	}
	// Shade every pixel the slow way and compare.
	s := quadShader{hx: 20, hy: 15, cx: 21.25, cy: 17.5, radii: q.Radii, fill: premul(q.Background), border: premul(white),
		sides: [4]float32{1, 2, 3, 1}, hasSides: true}
	for y := range 40 {
		for x := range 48 {
			c := s.shade(float32(x)+0.5, float32(y)+0.5)
			want := [4]byte{to8(c[0]), to8(c[1]), to8(c[2]), 255}
			got := at(img, x, y)
			for k := range 3 {
				if d := int(got[k]) - int(want[k]); d < -1 || d > 1 {
					t.Fatalf("pixel (%d,%d) = %v, shaded %v", x, y, got, want)
				}
			}
		}
	}
}

func TestClipRect(t *testing.T) {
	img := frame(t, 10, 1, func(r *Renderer) {
		r.PushClip(scene.Rect{X: 3, Y: 0, W: 4, H: 1})
		r.DrawQuads([]scene.Quad{{Bounds: scene.Rect{W: 10, H: 1}, Background: white}})
		r.PopClip()
		r.DrawQuads([]scene.Quad{{Bounds: scene.Rect{X: 9, W: 1, H: 1}, Background: red}})
	})
	for x := range 9 {
		want := byte(0)
		if x >= 3 && x < 7 {
			want = 255
		}
		if got := at(img, x, 0)[1]; got != want {
			t.Errorf("pixel %d = %d, want %d", x, got, want)
		}
	}
	if got := at(img, 9, 0); got != [4]byte{255, 0, 0, 255} {
		t.Errorf("after PopClip the quad is clipped: %v", got)
	}
}

// A rounded clip cuts the corner of what it contains.
func TestRoundedClip(t *testing.T) {
	img := frame(t, 20, 20, func(r *Renderer) {
		r.PushRoundedClip(scene.Rect{W: 20, H: 20}, scene.Uniform(10))
		r.DrawQuads([]scene.Quad{{Bounds: scene.Rect{W: 20, H: 20}, Background: white}})
		r.PopClip()
	})
	if got := at(img, 0, 0)[0]; got != 0 {
		t.Errorf("corner = %d, want clipped", got)
	}
	if got := at(img, 10, 10)[0]; got != 255 {
		t.Errorf("center = %d, want filled", got)
	}
}

// A shadow is soft: densest at its center and fading past its edge.
func TestShadow(t *testing.T) {
	img := frame(t, 60, 60, func(r *Renderer) {
		r.DrawQuads([]scene.Quad{{Bounds: scene.Rect{X: 20, Y: 20, W: 20, H: 20}, Background: white, Blur: 4}})
	})
	center, edge, out := at(img, 30, 30)[0], at(img, 20, 30)[0], at(img, 8, 30)[0] // 3 sigma out
	if !(center > edge && edge > out) || center < 200 || out > 5 {
		t.Errorf("shadow center %d, edge %d, outside %d", center, edge, out)
	}
	if edge < 100 || edge > 155 {
		t.Errorf("shadow at its edge = %d, want about half", edge)
	}
}

func TestGradient(t *testing.T) {
	img := frame(t, 101, 1, func(r *Renderer) {
		r.DrawQuads([]scene.Quad{{Bounds: scene.Rect{W: 101, H: 1}, Background: black, Color2: white,
			Gradient: true, Angle: 90, Stop0: 0, Stop1: 1}})
	})
	if a, m, b := at(img, 0, 0)[0], at(img, 50, 0)[0], at(img, 100, 0)[0]; a > 2 || b < 253 || m < 125 || m > 130 {
		t.Errorf("gradient left %d, middle %d, right %d", a, m, b)
	}
}

func TestOklabMixKeepsEnds(t *testing.T) {
	c := oklabMix(red, white, 0)
	if math.Abs(float64(c.R-1)) > 1e-3 || math.Abs(float64(c.G)) > 1e-3 {
		t.Errorf("oklab mix at 0 = %+v, want red", c)
	}
}

// A glyph mask is sampled per pixel with the text color.
func TestGlyphs(t *testing.T) {
	tex := scene.NewTexture(4, 4, scene.FormatAlpha8)
	for i := range tex.Pix {
		tex.Pix[i] = 255
	}
	tex.Pix[0] = 0 // the mask's top-left texel is empty
	img := frame(t, 8, 8, func(r *Renderer) {
		r.DrawGlyphs(scene.GlyphRun{Texture: tex, Glyphs: []scene.Glyph{
			{Bounds: scene.Rect{X: 2, Y: 2, W: 4, H: 4}, Src: scene.Rect{W: 4, H: 4}, Color: white},
		}})
	})
	if got := at(img, 2, 2)[0]; got != 0 {
		t.Errorf("empty texel drew %d", got)
	}
	if got := at(img, 3, 3)[0]; got != 255 {
		t.Errorf("full texel drew %d", got)
	}
	if got := at(img, 6, 6)[0]; got != 0 {
		t.Errorf("outside the glyph drew %d", got)
	}
}

// Mid coverage is boosted, more so for light text, as the GPU shader does.
func TestGlyphContrast(t *testing.T) {
	tex := scene.NewTexture(1, 1, scene.FormatAlpha8)
	tex.Pix[0] = 128
	img := frame(t, 1, 1, func(r *Renderer) {
		r.DrawGlyphs(scene.GlyphRun{Texture: tex, Glyphs: []scene.Glyph{{Bounds: scene.Rect{W: 1, H: 1}, Src: scene.Rect{W: 1, H: 1}, Color: white}}})
	})
	// k = 1 for white: a' = 2a / (a + 1).
	a := float32(128) / 255
	if got, want := at(img, 0, 0)[0], to8(2*a/(a+1)); got != want {
		t.Errorf("white text at half coverage = %d, want %d", got, want)
	}
}

func TestImageBilinear(t *testing.T) {
	tex := scene.NewTexture(2, 1, scene.FormatRGBA8)
	copy(tex.Pix, []byte{0, 0, 0, 255, 255, 255, 255, 255})
	img := frame(t, 4, 1, func(r *Renderer) {
		r.DrawImages([]scene.ImageDraw{{Bounds: scene.Rect{W: 4, H: 1}, Texture: tex, Opacity: 1}})
	})
	// Texel centers fall at x = 1 and 3: the ends clamp, the middle mixes.
	got := [4]byte{at(img, 0, 0)[0], at(img, 1, 0)[0], at(img, 2, 0)[0], at(img, 3, 0)[0]}
	if got[0] != 0 || got[3] != 255 || got[1] < 60 || got[1] > 68 || got[2] < 188 || got[2] > 195 {
		t.Errorf("bilinear row = %v, want 0, ~64, ~191, 255", got)
	}
}

func TestCaptureIsOpaque(t *testing.T) {
	r := New(nil)
	var got *image.RGBA
	r.CaptureNextFrame(func(img *image.RGBA, err error) { got = img })
	if err := r.BeginFrame(2, 2, scene.Color{}); err != nil {
		t.Fatal(err)
	}
	if err := r.EndFrame(); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Pix[3] != 255 {
		t.Fatalf("capture = %v", got)
	}
}

// The target supplies the buffer and is told to present it.
type memTarget struct {
	pix      []byte
	presents int
}

func (m *memTarget) Buffer(w, h int) ([]byte, int, error) {
	stride := w*4 + 16 // padded rows
	m.pix = make([]byte, stride*h)
	return m.pix, stride, nil
}

func (m *memTarget) Present() error { m.presents++; return nil }

func TestTarget(t *testing.T) {
	m := &memTarget{}
	r := New(m)
	if err := r.BeginFrame(3, 2, red); err != nil {
		t.Fatal(err)
	}
	if err := r.EndFrame(); err != nil {
		t.Fatal(err)
	}
	if m.presents != 1 {
		t.Errorf("%d presents", m.presents)
	}
	// BGRA: red is in the third byte; the row padding stays untouched.
	if px := m.pix[(3*4+16)+2*4 : (3*4+16)+2*4+4]; px[2] != 255 || px[0] != 0 {
		t.Errorf("pixel (2,1) = %v", px)
	}
	if m.pix[3*4] != 0 {
		t.Error("the row padding was written")
	}
}

func TestEmptyFrameIsAnError(t *testing.T) {
	if err := New(nil).BeginFrame(0, 10, black); err == nil {
		t.Error("an empty frame was accepted")
	}
}

// The per-row shadow sums give what shadowAlpha gives per pixel.
func TestShadowRowMatchesShadowAlpha(t *testing.T) {
	for _, c := range []struct{ hx, hy, corner, sigma float32 }{{20, 10, 4, 3}, {100, 50, 0, 8}, {8, 8, 8, 1.5}} {
		for qy := -c.hy - 3*c.sigma; qy <= c.hy+3*c.sigma; qy += 1.7 {
			row := newShadowRow(qy, c.hx, c.hy, c.corner, c.sigma)
			for qx := -c.hx - 3*c.sigma; qx <= c.hx+3*c.sigma; qx += 1.3 {
				want := shadowAlpha(qx, qy, c.hx, c.hy, c.corner, c.sigma)
				if got := row.alpha(qx); math.Abs(float64(got-want)) > 1e-4 {
					t.Fatalf("%+v at (%v,%v): row %v, shadowAlpha %v", c, qx, qy, got, want)
				}
			}
		}
	}
}

func TestSRGBTable(t *testing.T) {
	for v := float32(0); v <= 1; v += 0.0007 {
		if d := math.Abs(float64(srgbOf(v) - linearToSRGB(v))); d > 1.5e-3 {
			t.Fatalf("srgbOf(%v) is off by %v", v, d)
		}
	}
}

// A shadow's flat interior, filled once per row, stays within an 8-bit level of
// shading every pixel.
func TestShadowFlatInterior(t *testing.T) {
	q := scene.Quad{Bounds: scene.Rect{X: 30, Y: 20, W: 300, H: 200}, Background: scene.Color{A: 0.6}, Radii: scene.Uniform(12), Blur: 6}
	r := New(nil)
	if err := r.BeginFrame(360, 260, scene.Color{}); err != nil { // transparent: the alpha is the shadow's
		t.Fatal(err)
	}
	r.DrawQuads([]scene.Quad{q})
	if err := r.EndFrame(); err != nil {
		t.Fatal(err)
	}
	s := quadShader{hx: 150, hy: 100, cx: 180, cy: 120, radii: q.Radii, sigma: 6, fill: premul(q.Background)}
	for y := range 260 {
		for x := range 360 {
			want := to8(s.shade(float32(x)+0.5, float32(y)+0.5)[3])
			got := r.pix[y*r.stride+x*4+3]
			if d := int(got) - int(want); d < -1 || d > 1 {
				t.Fatalf("(%d,%d): alpha %d, shaded %d", x, y, got, want)
			}
		}
	}
}

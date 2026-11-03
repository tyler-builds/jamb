package soft

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/tyler-builds/jamb/internal/scene"
)

// busyScene draws many overlapping quads, shadows, gradients, clips, glyphs
// and images across a frame, the mix a UI paints.
func busyScene(r *Renderer, w, h int) {
	rng := rand.New(rand.NewPCG(1, 2))
	f := func(n int) float32 { return rng.Float32() * float32(n) }
	col := func() scene.Color {
		return scene.Color{R: rng.Float32(), G: rng.Float32(), B: rng.Float32(), A: 0.3 + 0.7*rng.Float32()}
	}
	mask := scene.NewTexture(32, 32, scene.FormatAlpha8)
	for i := range mask.Pix {
		mask.Pix[i] = byte(i * 7)
	}
	img := scene.NewTexture(16, 16, scene.FormatRGBA8)
	for i := range img.Pix {
		img.Pix[i] = byte(i * 13)
	}
	for i := range 200 {
		if i%25 == 0 {
			r.PushRoundedClip(scene.Rect{X: f(w / 2), Y: f(h / 2), W: f(w), H: f(h)}, scene.Uniform(f(30)))
		}
		q := scene.Quad{Bounds: scene.Rect{X: f(w), Y: f(h), W: f(400), H: f(300)}, Background: col(),
			BorderColor: col(), BorderWidth: f(3), Radii: scene.Uniform(f(20))}
		switch i % 5 {
		case 1:
			q.Blur = 1 + f(12)
		case 2:
			q.Gradient, q.Color2, q.Angle, q.Stop1, q.Oklab = true, col(), f(360), 1, i%2 == 0
		}
		r.DrawQuads([]scene.Quad{q})
		if i%3 == 0 {
			gs := make([]scene.Glyph, 20)
			for k := range gs {
				gs[k] = scene.Glyph{Bounds: scene.Rect{X: f(w), Y: f(h), W: 10, H: 14}, Src: scene.Rect{X: f(20), Y: f(16), W: 10, H: 14}, Color: col()}
			}
			r.DrawGlyphs(scene.GlyphRun{Texture: mask, Glyphs: gs})
		}
		if i%7 == 0 {
			r.DrawImages([]scene.ImageDraw{{Bounds: scene.Rect{X: f(w), Y: f(h), W: 40 + f(100), H: 40 + f(100)}, Texture: img, Radii: scene.Uniform(8), Opacity: 0.9}})
		}
		if i%25 == 24 {
			r.PopClip()
		}
	}
}

// Rasterizing in bands on many goroutines gives the same pixels as one pass.
func TestBandsMatchOnePass(t *testing.T) {
	const w, h = 700, 500
	par := New(nil)
	if err := par.BeginFrame(w, h, black); err != nil {
		t.Fatal(err)
	}
	busyScene(par, w, h)
	if err := par.EndFrame(); err != nil {
		t.Fatal(err)
	}

	one := New(nil)
	if err := one.BeginFrame(w, h, black); err != nil {
		t.Fatal(err)
	}
	busyScene(one, w, h)
	one.draw(&band{pix: one.pix, stride: one.stride, width: w, y0: 0, y1: h})

	if !bytes.Equal(par.pix, one.pix) {
		for i := range par.pix {
			if par.pix[i] != one.pix[i] {
				t.Fatalf("bands differ from one pass first at pixel (%d,%d)", i/4%w, i/4/w)
			}
		}
	}
}

func BenchmarkBusyFrame1080p(b *testing.B) {
	r := New(nil)
	for b.Loop() {
		r.BeginFrame(1920, 1080, black)
		busyScene(r, 1920, 1080)
		r.EndFrame()
	}
}

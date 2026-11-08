// Package rendertest holds scenes that the renderers' tests and benchmarks
// draw, so the CPU and GPU renderers are measured and compared on the same
// frames.
package rendertest

import (
	"math/rand/v2"

	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/scene"
)

// The textures of BusyScene. They are made once, as a UI's glyph atlas and
// images are, so a GPU renderer uploads them once rather than every frame.
var mask, img = func() (*scene.Texture, *scene.Texture) {
	mask := scene.NewTexture(32, 32, scene.FormatAlpha8)
	for i := range mask.Pix {
		mask.Pix[i] = byte(i * 7)
	}
	img := scene.NewTexture(16, 16, scene.FormatRGBA8)
	for i := range img.Pix {
		img.Pix[i] = byte(i * 13)
	}
	return mask, img
}()

// BusyScene draws many overlapping quads, shadows, gradients, clips, glyphs
// and images across a w×h frame, the mix a UI paints. It draws the same
// primitives on every call.
func BusyScene(r render.Renderer, w, h int) {
	rng := rand.New(rand.NewPCG(1, 2))
	f := func(n int) float32 { return rng.Float32() * float32(n) }
	col := func() scene.Color {
		return scene.Color{R: rng.Float32(), G: rng.Float32(), B: rng.Float32(), A: 0.3 + 0.7*rng.Float32()}
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

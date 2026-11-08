package soft

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/tyler-builds/jamb/internal/render/rendertest"
	"github.com/tyler-builds/jamb/internal/scene"
)

// The spans drawn without shading (the inside of quads, plain or gradient,
// away from their corners and borders and inside the clip) draw exactly what
// shading every pixel draws.
func TestSolidSpansMatchShading(t *testing.T) {
	const w, h = 700, 500
	draw := func() []byte {
		r := New(nil)
		if err := r.BeginFrame(w, h, black); err != nil {
			t.Fatal(err)
		}
		rendertest.BusyScene(r, w, h)
		borderedScene(r, w, h)
		if err := r.EndFrame(); err != nil {
			t.Fatal(err)
		}
		return bytes.Clone(r.own)
	}
	fast := draw()
	shadeEveryPixel = true
	defer func() { shadeEveryPixel = false }()
	slow := draw()
	if n := countDiff(fast, slow); n > 0 {
		t.Fatalf("%d bytes differ from shading every pixel", n)
	}
}

// borderedScene draws quads with uneven borders and corners, plain and
// gradient, inside nested clips, some of them rounded.
func borderedScene(r *Renderer, w, h int) {
	rng := rand.New(rand.NewPCG(3, 4))
	f := func(n int) float32 { return rng.Float32() * float32(n) }
	col := func() scene.Color {
		return scene.Color{R: rng.Float32(), G: rng.Float32(), B: rng.Float32(), A: 0.2 + 0.8*rng.Float32()}
	}
	for i := range 120 {
		if i%20 == 0 {
			r.PushClip(scene.Rect{X: f(w / 3), Y: f(h / 3), W: f(w), H: f(h)})
			r.PushRoundedClip(scene.Rect{X: f(w / 2), Y: f(h / 2), W: 100 + f(w), H: 100 + f(h)},
				scene.Corners{TL: f(40), TR: f(10), BR: f(60), BL: 0})
		}
		q := scene.Quad{
			Bounds:      scene.Rect{X: f(w) - 50, Y: f(h) - 50, W: 1 + f(500), H: 1 + f(300)},
			Background:  col(),
			BorderColor: col(),
			Radii:       scene.Corners{TL: f(50), TR: f(5), BR: f(30), BL: f(80)},
		}
		switch i % 4 {
		case 0:
			q.BorderSides = scene.Sides{Top: f(4), Right: 0, Bottom: f(9), Left: f(2)}
		case 1:
			q.BorderWidth = f(6)
		case 2:
			q.Gradient, q.Color2, q.Angle, q.Stop0, q.Stop1, q.Oklab = true, col(), f(360), 0.3*rng.Float32(), 0.6+rng.Float32(), i%8 == 2
			q.BorderWidth = f(3)
		case 3:
			q.Background.A = 0 // a border alone
			q.BorderWidth = 1 + f(2)
		}
		r.DrawQuads([]scene.Quad{q})
		if i%20 == 19 {
			r.PopClip()
			r.PopClip()
		}
	}
}

func countDiff(a, b []byte) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

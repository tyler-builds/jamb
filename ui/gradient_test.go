package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/scene"
)

// gradientQuads returns the quads in the scene that carry a gradient.
func gradientQuads(rt *Runtime) []scene.Quad {
	var out []scene.Quad
	for _, q := range rt.scene.Quads {
		if q.Gradient {
			out = append(out, q)
		}
	}
	return out
}

// A gradient's angle, stops, color space and opacity reach the scene quad. The
// gradient replaces Background.
func TestGradientReachesSceneQuad(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Size(Px(100), Px(50)).
			Background(Hex("#ff0000")).
			BackgroundGradient(LinearGradient(135, Hex("#0000ff"), Hex("#ffff00")).Stops(0.2, 0.8).InOklab()).
			Opacity(0.5)
	}, 400, 300, 1)
	qs := gradientQuads(rt)
	if len(qs) != 1 {
		t.Fatalf("%d gradient quads, want 1", len(qs))
	}
	q := qs[0]
	if q.Angle != 135 || q.Stop0 != 0.2 || q.Stop1 != 0.8 || !q.Oklab {
		t.Errorf("angle %v stops %v..%v oklab %v, want 135, 0.2..0.8, true", q.Angle, q.Stop0, q.Stop1, q.Oklab)
	}
	if want := (scene.Color{R: 0, G: 0, B: 1, A: 0.5}); q.Background != want {
		t.Errorf("from color %+v, want %+v (the opacity applies)", q.Background, want)
	}
	if want := (scene.Color{R: 1, G: 1, B: 0, A: 0.5}); q.Color2 != want {
		t.Errorf("to color %+v, want %+v", q.Color2, want)
	}
}

// Without stops the gradient spans the whole line, in sRGB.
func TestGradientDefaultStopsAndSRGB(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Size(Px(100), Px(50)).
			BackgroundGradient(LinearGradient(90, Hex("#000000"), Hex("#ffffff")))
	}, 400, 300, 1)
	qs := gradientQuads(rt)
	if len(qs) != 1 {
		t.Fatalf("%d gradient quads, want 1", len(qs))
	}
	if q := qs[0]; q.Stop0 != 0 || q.Stop1 != 1 || q.Oklab || q.Angle != 90 {
		t.Errorf("stops %v..%v oklab %v angle %v, want 0..1, sRGB, 90", q.Stop0, q.Stop1, q.Oklab, q.Angle)
	}
}

// A gradient set by Hover takes effect at once: it does not transition.
func TestGradientOnHoverSnaps(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Size(Px(100), Px(50)).Transition(300 * time.Millisecond).
			Hover(Style{Gradient: LinearGradient(0, Hex("#000000"), Hex("#ffffff"))})
	}, 400, 300, 1)
	rt.PointerMove(10, 10, 0)
	rt.Frame(time.Now())
	qs := gradientQuads(rt)
	if len(qs) != 1 {
		t.Fatalf("%d gradient quads after hover, want 1", len(qs))
	}
	if qs[0].Background != (scene.Color{A: 1}) || qs[0].Color2 != (scene.Color{R: 1, G: 1, B: 1, A: 1}) {
		t.Errorf("hovered gradient colors %+v -> %+v, want black -> white at once", qs[0].Background, qs[0].Color2)
	}
}

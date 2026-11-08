package app

import (
	"image"
	"image/color"
	"testing"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render/soft"
	"github.com/tyler-builds/jamb/internal/scene"
)

type stubFramed struct{ f platform.ClientFrame }

func (w stubFramed) ClientFrame() platform.ClientFrame { return w.f }

var (
	frameBG  = scene.Color{R: 1, G: 1, B: 1, A: 1}
	frameRed = scene.Color{R: 1, A: 1}
)

// drawFramed draws a 100×80 content, a red square at its origin on white,
// through a framedRenderer, and returns the whole frame and the capture.
func drawFramed(t *testing.T, f platform.ClientFrame) (full, captured *image.RGBA) {
	t.Helper()
	cpu := soft.New(nil)
	r := newFramedRenderer(cpu, stubFramed{f}, func() bool { return false })
	r.CaptureNextFrame(func(img *image.RGBA, err error) {
		if err != nil {
			t.Fatal(err)
		}
		captured = img
	})
	if err := r.BeginFrame(100, 80, frameBG); err != nil {
		t.Fatal(err)
	}
	r.PushClip(scene.Rect{W: 100, H: 80})
	r.DrawQuads([]scene.Quad{{Bounds: scene.Rect{W: 10, H: 10}, Background: frameRed}})
	r.PopClip()
	if err := r.EndFrame(); err != nil {
		t.Fatal(err)
	}
	return cpu.Image(), captured
}

// Without a frame, the scene is drawn as it is.
func TestFramedRendererPassThrough(t *testing.T) {
	full, captured := drawFramed(t, platform.ClientFrame{Scale: 1})
	if b := full.Bounds(); b.Dx() != 100 || b.Dy() != 80 {
		t.Fatalf("frame is %v; want 100x80", b)
	}
	if got := full.RGBAAt(2, 2); got != (color.RGBA{R: 255, A: 255}) {
		t.Errorf("pixel (2,2) = %v; want the red square", got)
	}
	if captured.Bounds() != full.Bounds() {
		t.Errorf("capture is %v", captured.Bounds())
	}
}

// With a frame, the content is drawn inside the margin, clipped to its rounded
// corners, over a shadow; the capture is the content alone.
func TestFramedRendererFrame(t *testing.T) {
	f := platform.ClientFrame{Left: 20, Top: 20, Right: 20, Bottom: 20, Radii: [4]float32{12, 12, 12, 12}, Active: true, Scale: 1}
	full, captured := drawFramed(t, f)
	if b := full.Bounds(); b.Dx() != 140 || b.Dy() != 120 {
		t.Fatalf("frame is %v; want the content and margins, 140x120", b)
	}
	if got := full.RGBAAt(26, 26); got.R < 200 || got.G > 40 {
		t.Errorf("pixel (26,26) = %v; want the red square, moved by the margin", got)
	}
	if a := full.RGBAAt(20, 20).A; a > 40 {
		t.Errorf("the content's corner pixel has alpha %d; want it cut by the rounded corner", a)
	}
	if a := full.RGBAAt(1, 1).A; a != 0 {
		t.Errorf("the margin's corner has alpha %d; want it transparent", a)
	}
	if a := full.RGBAAt(70, 108).A; a == 0 || a == 255 {
		t.Errorf("below the content, alpha is %d; want the shadow", a)
	}
	if b := captured.Bounds(); b.Dx() != 100 || b.Dy() != 80 || b.Min != (image.Point{}) {
		t.Fatalf("capture is %v; want the content, 100x80", b)
	}
	if got := captured.RGBAAt(30, 40); got != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Errorf("captured (30,40) = %v; want the white background", got)
	}
}

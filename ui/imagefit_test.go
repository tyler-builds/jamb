package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/scene"
)

// checkerboard is a size×size image of one-pixel black and white squares: the
// hardest image to shrink, since bilinear sampling picks black or white texels
// where the true average is gray.
func checkerboard(size int) *ImageSource {
	m := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			if (x+y)%2 == 0 {
				m.Set(x, y, color.White)
			} else {
				m.Set(x, y, color.Black)
			}
		}
	}
	return ImageFromImage(m)
}

// drawnImage returns the one image the latest frame drew.
func drawnImage(t *testing.T, rt *Runtime) scene.ImageDraw {
	t.Helper()
	if len(rt.scene.Images) != 1 {
		t.Fatalf("%d images drawn, want 1", len(rt.scene.Images))
	}
	return rt.scene.Images[0]
}

// A still image drawn smaller than its pixels is drawn from a copy resampled to
// the device pixels it covers, which averages what each pixel covers.
func TestShrunkImageIsResampledToItsDeviceSize(t *testing.T) {
	src := checkerboard(128)
	for _, scale := range []float32{1, 2} {
		rt := newTestRuntime(func(ctx *Context) Element {
			return Div().Child(Image(src).Size(Px(36), Px(36)))
		}, 300, 200, scale)
		im := drawnImage(t, rt)
		want := int(36 * scale)
		if im.Texture == src.tex || im.Texture.Width != want || im.Texture.Height != want {
			t.Fatalf("at scale %v: drawn from a %dx%d texture, want a %dx%d copy", scale, im.Texture.Width, im.Texture.Height, want, want)
		}
		if im.Src.W != float32(want) || im.Src.H != float32(want) || im.Bounds.W != float32(want) {
			t.Fatalf("at scale %v: src %+v bounds %+v, want the whole copy pixel for pixel", scale, im.Src, im.Bounds)
		}
		for i := 0; i < len(im.Texture.Pix); i += 4 {
			if v := im.Texture.Pix[i]; v < 90 || v > 165 {
				t.Fatalf("at scale %v: texel %d is %d, want the gray average of the checkerboard", scale, i/4, v)
			}
		}
	}
}

// An image drawn at or above its size, or only a little smaller, is drawn from
// its own pixels.
func TestImageNearItsSizeIsNotResampled(t *testing.T) {
	src := checkerboard(40)
	for _, px := range []float32{40, 38, 80} {
		rt := newTestRuntime(func(ctx *Context) Element {
			return Div().Child(Image(src).Size(Px(px), Px(px)))
		}, 300, 200, 1)
		if im := drawnImage(t, rt); im.Texture != src.tex {
			t.Fatalf("drawn at %v px from a %dx%d copy, want its own pixels", px, im.Texture.Width, im.Texture.Height)
		}
	}
}

// An image whose size changes is drawn from its own pixels until the size has
// settled, then resampled once, without needing any other change.
func TestResizedImageIsResampledOnceItSettles(t *testing.T) {
	src := checkerboard(128)
	size := NewSignal[float32](36)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Image(src).Size(Px(size.Get()), Px(size.Get())))
	}, 300, 200, 1)
	now := time.Now()
	rt.Frame(now)
	size.Set(48)
	rt.Frame(now.Add(16 * time.Millisecond))
	if im := drawnImage(t, rt); im.Texture != src.tex {
		t.Fatalf("just resized, drawn from a %dx%d copy, want its own pixels until the size settles", im.Texture.Width, im.Texture.Height)
	}
	if rt.imageDue.IsZero() {
		t.Fatal("no frame is due to resample the image once its size settles")
	}
	rt.Frame(rt.imageDue)
	if im := drawnImage(t, rt); im.Texture.Width != 48 {
		t.Fatalf("after the size settled, drawn from a %dx%d texture, want a 48x48 copy", im.Texture.Width, im.Texture.Height)
	}
	if n := len(src.fits); n > maxFitted {
		t.Fatalf("%d sizes kept, want at most %d", n, maxFitted)
	}
}

// A large image is resampled on another goroutine: it is drawn from its own
// pixels until the copy is ready, and the window repaints with it then.
func TestLargeImageIsResampledInTheBackground(t *testing.T) {
	src := checkerboard(600)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Image(src).Size(Px(100), Px(100)))
	}, 300, 200, 1)
	if im := drawnImage(t, rt); im.Texture != src.tex {
		t.Fatal("a large image was resampled in the paint, want it on another goroutine")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if !rt.NeedsFrame() {
			time.Sleep(time.Millisecond)
		}
		rt.Frame(time.Now())
		if im := drawnImage(t, rt); im.Texture.Width == 100 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the resampled copy never arrived")
		}
	}
}

// FitCover resamples only the part of the image that shows.
func TestCoverImageResamplesTheVisiblePart(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := range 100 {
		for x := range 200 {
			if x < 50 || x >= 150 {
				m.Set(x, y, color.RGBA{255, 0, 0, 255}) // cropped away by FitCover
			} else {
				m.Set(x, y, color.RGBA{0, 0, 255, 255})
			}
		}
	}
	src := ImageFromImage(m)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Image(src).Size(Px(40), Px(40)).Fit(FitCover))
	}, 300, 200, 1)
	im := drawnImage(t, rt)
	if im.Texture == src.tex || im.Texture.Width != 40 {
		t.Fatalf("drawn from a %dx%d texture, want a 40x40 copy", im.Texture.Width, im.Texture.Height)
	}
	for i := 0; i < len(im.Texture.Pix); i += 4 {
		if r, b := im.Texture.Pix[i], im.Texture.Pix[i+2]; r > 8 || b < 247 {
			t.Fatalf("texel %d is r=%d b=%d: the copy includes the part FitCover crops away", i/4, r, b)
		}
	}
}

// Resampling an app icon, as a list of them first draws it.
func BenchmarkResampleIcon(b *testing.B) {
	src := checkerboard(128)
	for b.Loop() {
		resample(src.tex, image.Rect(0, 0, 128, 128), 36, 36)
	}
}

// Resampling a screenshot to a detail page's width.
func BenchmarkResampleScreenshot(b *testing.B) {
	src := checkerboard(1920)
	for b.Loop() {
		resample(src.tex, image.Rect(0, 0, 1920, 1920), 640, 640)
	}
}

package ui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/scene"
)

// testGIF encodes a 4x4 GIF of three frames: red, then a green 2x2 patch over
// it, then blue. Delays are 100 ms, 50 ms and 0 (shown as 100 ms).
func testGIF(t *testing.T, loops int) []byte {
	t.Helper()
	pal := color.Palette{color.Transparent, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}}
	fill := func(r image.Rectangle, idx uint8) *image.Paletted {
		p := image.NewPaletted(r, pal)
		for i := range p.Pix {
			p.Pix[i] = idx
		}
		return p
	}
	g := &gif.GIF{
		Image:     []*image.Paletted{fill(image.Rect(0, 0, 4, 4), 1), fill(image.Rect(0, 0, 2, 2), 2), fill(image.Rect(0, 0, 4, 4), 3)},
		Delay:     []int{10, 5, 0},
		LoopCount: loops,
		Config:    image.Config{Width: 4, Height: 4},
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pixelAt returns the RGBA of a texture's pixel.
func pixelAt(src *ImageSource, frame, x, y int) [4]byte {
	p := src.frames[frame].tex.Pix[(y*src.width+x)*4:]
	return [4]byte{p[0], p[1], p[2], p[3]}
}

// An animated GIF decodes to composited frames with their delays.
func TestAnimatedGIFDecodes(t *testing.T) {
	src, err := ImageFromBytes(testGIF(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !src.Animated() || len(src.frames) != 3 {
		t.Fatalf("animated %v with %d frames, want 3", src.Animated(), len(src.frames))
	}
	want := []time.Duration{100 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond}
	for i, f := range src.frames {
		if f.delay != want[i] {
			t.Errorf("frame %d delay %v, want %v", i, f.delay, want[i])
		}
	}
	// The second frame is the green patch drawn over the red first frame.
	if got := pixelAt(src, 1, 0, 0); got != [4]byte{0, 255, 0, 255} {
		t.Errorf("frame 1 at (0,0) = %v, want green", got)
	}
	if got := pixelAt(src, 1, 3, 3); got != [4]byte{255, 0, 0, 255} {
		t.Errorf("frame 1 at (3,3) = %v, want the red of frame 0 kept", got)
	}
}

// frameAt follows the delays and the loop count.
func TestAnimatedGIFTiming(t *testing.T) {
	src, _ := ImageFromBytes(testGIF(t, -1)) // plays once
	ms := time.Millisecond
	for _, c := range []struct {
		at        time.Duration
		frame     int
		remaining time.Duration
	}{
		{0, 0, 100 * ms}, {120 * ms, 1, 30 * ms}, {160 * ms, 2, 90 * ms}, {300 * ms, 2, 0}, {10 * time.Second, 2, 0},
	} {
		tex, next := src.frameAt(c.at)
		if tex != src.frames[c.frame].tex || next != c.remaining {
			t.Errorf("at %v: frame %v next %v, want frame %d next %v", c.at, tex == src.frames[c.frame].tex, next, c.frame, c.remaining)
		}
	}
	forever, _ := ImageFromBytes(testGIF(t, 0))
	if tex, next := forever.frameAt(260 * ms); tex != forever.frames[0].tex || next != 90*ms {
		t.Errorf("a looping GIF did not start again after 250 ms")
	}
}

// A drawn animated image asks for a frame when the next one is due and shows
// it then; under reduced motion it stays on the first frame.
func TestAnimatedGIFPlays(t *testing.T) {
	src, _ := ImageFromBytes(testGIF(t, 0))
	now := time.Unix(1_000_000, 0)
	rt := NewRuntime(func(ctx *Context) Element { return Div().Child(Image(src).Alt("anim")) }, DarkTheme(), nil)
	rt.clock = func() time.Time { return now }
	rt.SetViewport(200, 100, 1)
	sc := rt.Frame(now)
	if !sceneHasTexture(sc, src.frames[0].tex) || rt.imageDue != now.Add(100*time.Millisecond) {
		t.Fatalf("first frame not drawn or next frame not scheduled (due %v)", rt.imageDue.Sub(now))
	}
	now = now.Add(110 * time.Millisecond)
	sc = rt.Frame(now)
	if !sceneHasTexture(sc, src.frames[1].tex) {
		t.Fatal("the second frame is not drawn when it is due")
	}

	reduced := newTestRuntime(func(ctx *Context) Element { return Div().Child(Image(src).Alt("anim")) }, 200, 100, 1)
	reduced.SetSystemSettings(SystemSettings{ReducedMotion: true})
	sc = reduced.Frame(now)
	if !sceneHasTexture(sc, src.frames[0].tex) || !reduced.imageDue.IsZero() {
		t.Fatal("reduced motion: the GIF is not still on its first frame")
	}
}

// sceneHasTexture reports whether the scene draws tex.
func sceneHasTexture(sc *scene.Scene, tex *scene.Texture) bool {
	for _, im := range sc.Images {
		if im.Texture == tex {
			return true
		}
	}
	return false
}

// WebP still images decode.
func TestWebPDecodes(t *testing.T) {
	// The 1x1 lossless WebP that browsers' feature checks use.
	data, _ := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	src, err := ImageFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := src.Size(); w != 1 || h != 1 {
		t.Fatalf("size %dx%d, want 1x1", w, h)
	}
}

// An animated image that is not drawn (here, not displayed) asks for no frames.
func TestAnimatedGIFHiddenAsksForNothing(t *testing.T) {
	src, _ := ImageFromBytes(testGIF(t, 0))
	now := time.Unix(1_000_000, 0)
	rt := NewRuntime(func(ctx *Context) Element {
		return Div().Child(Div().Display(false).Child(Image(src).Alt("anim")))
	}, DarkTheme(), nil)
	rt.clock = func() time.Time { return now }
	rt.SetViewport(200, 100, 1)
	rt.Frame(now)
	if !rt.imageDue.IsZero() {
		t.Fatal("a GIF that is not displayed scheduled a frame")
	}
}

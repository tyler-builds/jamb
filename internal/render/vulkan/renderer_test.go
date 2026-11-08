//go:build linux && (amd64 || arm64)

package vulkan

import (
	"errors"
	"image"
	"testing"

	"github.com/tyler-builds/jamb/internal/dmabuf"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/rendertest"
	"github.com/tyler-builds/jamb/internal/render/soft"
	"github.com/tyler-builds/jamb/internal/scene"
)

// memSurface stands in for a window: it imports every dma-buf and shows none,
// so the renderer runs without a compositor.
type memSurface struct {
	mods []uint64
	// reject, when set, makes Probe reject the images it returns true for.
	reject   func(dmabuf.Image) bool
	probes   []dmabuf.Image
	main     dmabuf.Device
	hasMain  bool
	imports  int
	presents int
}

func (s *memSurface) Modifiers(uint32) []uint64         { return s.mods }
func (s *memSurface) MainDevice() (dmabuf.Device, bool) { return s.main, s.hasMain }
func (s *memSurface) Present(dmabuf.Buffer) error       { s.presents++; return nil }

func (s *memSurface) Probe(img dmabuf.Image) error {
	s.probes = append(s.probes, img)
	if s.reject != nil && s.reject(img) {
		return errors.New("memSurface: rejected")
	}
	return nil
}

func (s *memSurface) Import(img dmabuf.Image) (dmabuf.Buffer, error) {
	if img.FD < 0 || len(img.Planes) == 0 || img.Planes[0].Stride < uint32(img.Width*4) && img.Modifier == 0 {
		return nil, errors.New("memSurface: bad dma-buf")
	}
	s.imports++
	return memBuffer{}, nil
}

type memBuffer struct{}

func (memBuffer) Busy() bool { return false }
func (memBuffer) Destroy()   {}

// newTestDevice returns a device on the machine's GPU, or skips the test when
// it has none that qualifies (as on CI runners), and a surface that imports
// every modifier the device renders.
func newTestDevice(t testing.TB) (*Device, *memSurface) {
	t.Helper()
	d, err := NewDevice()
	if err != nil {
		t.Skipf("no Vulkan device: %v", err)
	}
	t.Cleanup(d.Close)
	// Compositors import linear images too.
	s := &memSurface{mods: []uint64{dmabuf.ModLinear}}
	for _, m := range d.modifiers {
		if m.mod != dmabuf.ModLinear {
			s.mods = append(s.mods, m.mod)
		}
	}
	return d, s
}

// newTestRenderer returns a renderer on the machine's GPU; see newTestDevice.
func newTestRenderer(t testing.TB) (*Renderer, *memSurface) {
	t.Helper()
	d, s := newTestDevice(t)
	r, err := d.NewRenderer(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r, s
}

// capture draws a frame with draw and returns it.
func capture(t testing.TB, r render.Renderer, w, h int, draw func(render.Renderer)) *image.RGBA {
	t.Helper()
	var img *image.RGBA
	var capErr error
	r.CaptureNextFrame(func(i *image.RGBA, err error) { img, capErr = i, err })
	if err := r.BeginFrame(w, h, scene.Color{R: 0.1, G: 0.1, B: 0.12, A: 1}); err != nil {
		t.Fatal(err)
	}
	draw(r)
	if err := r.EndFrame(); err != nil {
		t.Fatal(err)
	}
	if capErr != nil {
		t.Fatal(capErr)
	}
	return img
}

// compare reports the largest difference of a channel between two frames,
// and how many pixels differ by more than tolerance.
func compare(a, b *image.RGBA, tolerance int) (maxDiff, over int) {
	for i := 0; i+4 <= len(a.Pix); i += 4 {
		worst := 0
		for c := range 3 {
			d := int(a.Pix[i+c]) - int(b.Pix[i+c])
			worst = max(worst, d, -d)
		}
		maxDiff = max(maxDiff, worst)
		if worst > tolerance {
			over++
		}
	}
	return maxDiff, over
}

// The GPU draws what the CPU renderer draws: both follow the same shaders, so
// they differ only by rounding (the CPU renderer approximates exp and pow with
// tables, and the GPU's float math differs in the last bits). The busy scene
// also puts glyphs at fractional positions, which UI text never is: there the
// two pick different texels at a glyph's edge row or column, so a few pixels
// differ by more.
func TestMatchesCPU(t *testing.T) {
	const w, h = 700, 500
	r, s := newTestRenderer(t)
	gpu := capture(t, r, w, h, func(r render.Renderer) { rendertest.BusyScene(r, w, h) })
	cpu := capture(t, soft.New(nil), w, h, func(r render.Renderer) { rendertest.BusyScene(r, w, h) })
	maxDiff, over := compare(gpu, cpu, 2)
	t.Logf("largest difference %d; %d of %d pixels differ by more than 2", maxDiff, over, w*h)
	if over > w*h/1000 {
		t.Errorf("the GPU frame differs from the CPU frame: largest difference %d, %d pixels by more than 2", maxDiff, over)
	}
	if s.presents != 1 || s.imports != 1 {
		t.Errorf("presents, imports = %d, %d; want 1, 1", s.presents, s.imports)
	}
}

// A texture changed between frames is uploaded again: its dirty region on
// the second frame, all of it on the first.
func TestTextureUpdate(t *testing.T) {
	const w, h = 64, 32
	r, _ := newTestRenderer(t)
	mask := scene.NewTexture(16, 16, scene.FormatAlpha8)
	draw := func(r render.Renderer) {
		r.DrawGlyphs(scene.GlyphRun{Texture: mask, Glyphs: []scene.Glyph{{
			Bounds: scene.Rect{X: 8, Y: 8, W: 16, H: 16}, Src: scene.Rect{W: 16, H: 16}, Color: scene.Color{R: 1, G: 1, B: 1, A: 1}}}})
	}
	before := capture(t, r, w, h, draw)
	for i := range mask.Pix {
		mask.Pix[i] = 255
	}
	mask.MarkDirty(4, 4, 12, 12)
	mask.Version++
	after := capture(t, r, w, h, draw)
	if before.RGBAAt(16, 16) == after.RGBAAt(16, 16) {
		t.Errorf("the dirty region was not uploaded: pixel (16,16) is %v on both frames", after.RGBAAt(16, 16))
	}
	if got := after.RGBAAt(9, 9); got != before.RGBAAt(9, 9) {
		t.Errorf("pixel (9,9), outside the dirty region, changed from %v to %v", before.RGBAAt(9, 9), got)
	}
}

// Frames of a new size get images of that size.
func TestResize(t *testing.T) {
	r, s := newTestRenderer(t)
	for _, size := range [][2]int{{100, 80}, {300, 200}, {300, 200}, {50, 40}} {
		img := capture(t, r, size[0], size[1], func(render.Renderer) {})
		if b := img.Bounds(); b.Dx() != size[0] || b.Dy() != size[1] {
			t.Fatalf("frame is %v; want %dx%d", b, size[0], size[1])
		}
	}
	if s.imports != 3 {
		t.Errorf("imports = %d; want 3 (one per size, the repeated size reusing its image)", s.imports)
	}
}

// After the device is released, frames fail with ErrDeviceLost until the
// device is recovered and the window recreated; then they draw as before.
func TestRecover(t *testing.T) {
	const w, h = 200, 150
	r, _ := newTestRenderer(t)
	draw := func(r render.Renderer) { rendertest.BusyScene(r, w, h) }
	want := capture(t, r, w, h, draw)
	r.ReleaseDevice()
	if err := r.BeginFrame(w, h, scene.Color{}); !errors.Is(err, render.ErrDeviceLost) {
		t.Fatalf("BeginFrame after ReleaseDevice: %v; want ErrDeviceLost", err)
	}
	if err := r.Recover(); err != nil {
		t.Fatal(err)
	}
	if err := r.Recreate(); err != nil {
		t.Fatal(err)
	}
	got := capture(t, r, w, h, draw)
	if maxDiff, _ := compare(got, want, 0); maxDiff != 0 {
		t.Errorf("the frame after recovery differs by up to %d", maxDiff)
	}
}

// A kind of image the compositor rejects is not used: the next one is tried,
// and the window is drawn on the CPU (NewRenderer fails) when it rejects all.
func TestProbeFallsBack(t *testing.T) {
	d, s := newTestDevice(t)
	var first uint64
	s.reject = func(img dmabuf.Image) bool {
		if len(s.probes) == 1 {
			first = img.Modifier
			return true
		}
		return false
	}
	r, err := d.NewRenderer(s)
	if err != nil {
		if len(s.mods) == 1 {
			t.Skip("the device renders one modifier only")
		}
		t.Fatal(err)
	}
	defer r.Close()
	if r.place.mod.mod == first && !r.place.shared {
		t.Errorf("the renderer kept the rejected kind of image (modifier %#x)", first)
	}
	capture(t, r, 32, 32, func(render.Renderer) {})

	s.probes, s.reject = nil, func(dmabuf.Image) bool { return true }
	if _, err := d.NewRenderer(s); err == nil {
		t.Error("NewRenderer succeeded although the compositor rejects every image")
	}
}

// A compositor on another GPU gets linear images in system memory first, when
// the device can make them, and they draw correctly.
func TestOtherGPU(t *testing.T) {
	d, s := newTestDevice(t)
	s.main, s.hasMain = dmabuf.Device{Major: 4095, Minor: 4095}, true
	r, err := d.NewRenderer(s)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	t.Logf("placement: %s", r.Describe())
	if !r.place.shared {
		t.Skipf("%s cannot export linear images from system memory", d.name)
	}
	const w, h = 700, 500
	got := capture(t, r, w, h, func(r render.Renderer) { rendertest.BusyScene(r, w, h) })
	want := capture(t, soft.New(nil), w, h, func(r render.Renderer) { rendertest.BusyScene(r, w, h) })
	if _, over := compare(got, want, 2); over > w*h/1000 {
		t.Errorf("%d pixels differ from the CPU frame by more than 2", over)
	}
}

func BenchmarkBusyFrame1080p(b *testing.B) {
	r, _ := newTestRenderer(b)
	for b.Loop() {
		r.BeginFrame(1920, 1080, scene.Color{A: 1})
		rendertest.BusyScene(r, 1920, 1080)
		if err := r.EndFrame(); err != nil {
			b.Fatal(err)
		}
	}
}

package icons

import (
	"image"
	"sync"
	"testing"
)

// The shapes of an icon are built on the first Rasterize, not when the icon is
// declared, so that the set costs an app nothing for the icons it never draws.
func TestIconShapesBuildOnFirstRasterize(t *testing.T) {
	builds := 0
	ic := &Icon{Name: "counted", StrokeWidth: 2, build: func(b *builder) {
		builds++
		b.Line(4, 12, 20, 12)
	}}
	if builds != 0 {
		t.Fatalf("newIcon built the shapes: %d builds before the first Rasterize", builds)
	}
	dst := image.NewAlpha(image.Rect(0, 0, 16, 16))
	ic.Rasterize(dst)
	ic.Rasterize(dst)
	if builds != 1 {
		t.Errorf("builds = %d after two Rasterize calls, want 1", builds)
	}
}

// The first Rasterize may run on several goroutines at once; the shapes are
// still built once and every call draws them.
func TestIconRasterizeConcurrently(t *testing.T) {
	builds := 0
	ic := &Icon{Name: "concurrent", StrokeWidth: 2, build: func(b *builder) {
		builds++
		b.Line(4, 12, 20, 12)
	}}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ic.Rasterize(image.NewAlpha(image.Rect(0, 0, 16, 16)))
		}()
	}
	wg.Wait()
	if builds != 1 {
		t.Errorf("builds = %d, want 1", builds)
	}
}

// A zero Icon has no shapes and draws nothing, without a panic.
func TestZeroIconDrawsNothing(t *testing.T) {
	dst := image.NewAlpha(image.Rect(0, 0, 8, 8))
	var ic Icon
	ic.Rasterize(dst)
	for _, a := range dst.Pix {
		if a != 0 {
			t.Fatal("a zero Icon drew coverage")
		}
	}
}

// A built-in icon draws its shapes on the first Rasterize, and the drawn mask
// has coverage where the shapes are.
func TestBuiltInIconDrawsOnFirstUse(t *testing.T) {
	dst := image.NewAlpha(image.Rect(0, 0, 24, 24))
	Check.Rasterize(dst)
	covered := 0
	for _, a := range dst.Pix {
		if a > 0 {
			covered++
		}
	}
	if covered == 0 {
		t.Error("Check drew no coverage")
	}
}

//go:build windows

package d3d11

import (
	"image"
	"syscall"
	"testing"

	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/win32"
)

// hiddenWindow creates a window that is never shown, so the test does not move
// focus or draw on screen. The swap chain needs a window, and nothing else.
func hiddenWindow(t *testing.T, w, h int32) uintptr {
	t.Helper()
	class, _ := syscall.UTF16PtrFromString("STATIC")
	title, _ := syscall.UTF16PtrFromString("jamb resize test")
	hwnd, err := win32.CreateWindowEx(0, class, title, win32.WS_POPUP, 0, 0, w, h, 0, 0, win32.GetModuleHandle())
	if err != nil {
		t.Fatalf("CreateWindowEx: %v", err)
	}
	return hwnd
}

// A window's swap chain keeps headroom during a live resize, so a drag within it
// takes no ResizeBuffers. The end of the drag fits the buffers to the window, and
// any other change of size (a maximize, a restore) takes exact buffers, so the
// shown part is never smaller than the buffers at rest. A capture copies only the
// frame's own size.
func TestResizeKeepsBuffers(t *testing.T) {
	d, err := NewDevice()
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	defer d.Close()
	hwnd := hiddenWindow(t, 800, 600)
	defer win32.DestroyWindow(hwnd)
	r, err := d.NewRenderer(hwnd, 800, 600)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	defer r.Close()
	if r.swap2 == nil {
		t.Skip("the swap chain has no IDXGISwapChain2, so it has no source size")
	}
	// The frame is only its clear color, so a capture shows whether it holds the frame.
	orange := scene.Color{R: 1, G: 0.5, A: 1}
	draw := func(w, h int) {
		t.Helper()
		if err := r.BeginFrame(w, h, orange); err != nil {
			t.Fatalf("BeginFrame(%d, %d): %v", w, h, err)
		}
		if err := r.EndFrame(); err != nil {
			t.Fatalf("EndFrame at %d×%d: %v", w, h, err)
		}
	}
	exact := func(when string, w, h int) {
		t.Helper()
		if r.bufW != w || r.bufH != h {
			t.Errorf("buffers %s = %d×%d, want exactly %d×%d", when, r.bufW, r.bufH, w, h)
		}
	}

	draw(800, 600)
	r.SetLiveResize(true)
	for w := 810; w <= 1000; w += 10 {
		draw(w, 600)
	}
	if r.resizeBuffersCalls > 1 {
		t.Errorf("a drag from 800 to 1000 called ResizeBuffers %d times, want 1 (the headroom takes the rest)", r.resizeBuffersCalls)
	}
	draw(400, 300)
	r.SetLiveResize(false)
	calls := r.resizeBuffersCalls
	fitted, err := r.FitBuffers()
	if err != nil || !fitted {
		t.Fatalf("FitBuffers after a drag = %t, %v; want true, nil", fitted, err)
	}
	exact("after the drag", 400, 300)
	if again, _ := r.FitBuffers(); again {
		t.Error("a second FitBuffers changed the buffers")
	}
	if r.resizeBuffersCalls != calls+1 {
		t.Errorf("ResizeBuffers calls = %d, want %d", r.resizeBuffersCalls, calls+1)
	}

	// A maximize and a restore are not live: each takes exact buffers.
	draw(1600, 1000)
	exact("after a maximize", 1600, 1000)
	draw(400, 300)
	exact("after a restore", 400, 300)

	var got *image.RGBA
	r.CaptureNextFrame(func(img *image.RGBA, err error) {
		if err != nil {
			t.Errorf("capture: %v", err)
			return
		}
		got = img
	})
	draw(400, 300)
	if got == nil {
		t.Fatal("the capture did not arrive")
	}
	if b := got.Bounds(); b.Dx() != 400 || b.Dy() != 300 {
		t.Fatalf("captured %d×%d, want the shown 400×300", b.Dx(), b.Dy())
	}
	// The last pixel of the shown part is the clear color, not the buffers' padding.
	last := got.Pix[len(got.Pix)-4:]
	if last[0] != 255 || last[1] < 120 || last[1] > 135 || last[2] != 0 {
		t.Errorf("last captured pixel = %v, want the clear color (255, 128, 0)", last[:3])
	}
}

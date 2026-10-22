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

// A window's swap chain keeps headroom for a drag, so a drag within it takes no
// ResizeBuffers. A shrink gives the room back, and a capture copies only the
// frame's own size, not the larger buffers.
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

	draw(800, 600)
	for w := 810; w <= 1000; w += 10 {
		draw(w, 600)
	}
	if r.resizeBuffersCalls > 1 {
		t.Errorf("a drag from 800 to 1000 called ResizeBuffers %d times, want 1 (the headroom takes the rest)", r.resizeBuffersCalls)
	}

	draw(400, 300)
	calls := r.resizeBuffersCalls
	shrunk, err := r.ShrinkBuffers()
	if err != nil || !shrunk {
		t.Fatalf("ShrinkBuffers after a shrink to 400×300 = %t, %v; want true, nil", shrunk, err)
	}
	if r.bufW != headroom(400) || r.bufH != headroom(300) {
		t.Errorf("buffers after the shrink = %d×%d, want %d×%d", r.bufW, r.bufH, headroom(400), headroom(300))
	}
	if again, _ := r.ShrinkBuffers(); again {
		t.Error("a second ShrinkBuffers changed the buffers")
	}
	if r.resizeBuffersCalls != calls+1 {
		t.Errorf("ResizeBuffers calls = %d, want %d", r.resizeBuffersCalls, calls+1)
	}

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

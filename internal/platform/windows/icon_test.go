//go:build windows

package windows

import (
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

// iconSide returns the width of icon in pixels, or 0 for no icon.
func iconSide(icon uintptr) int {
	if icon == 0 {
		return 0
	}
	img, err := win32.IconImage(icon)
	if err != nil {
		return -1
	}
	return img.Bounds().Dx()
}

// windowIconSides opens a window with icon as WindowOptions.Icon, and returns what
// check reports about it, on the UI thread, before the window closes.
func windowIconSides(t *testing.T, icon image.Image, check func(w *window) string) string {
	t.Helper()
	h := &queueHandler{}
	result := "the window did not run its test"
	err := Run(func() error {
		pw, err := Open(platform.WindowOptions{Title: "icons", Width: 200, Height: 120, Icon: icon, NoActivate: true}, h)
		if err != nil {
			return err
		}
		Post(func() {
			result = check(pw.(*window))
			pw.Close()
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result
}

// A window gets a small and a big icon, each made at the size the system wants
// for the window's DPI, and made again at the new sizes when the DPI changes.
func TestWindowIconsAtEachDPI(t *testing.T) {
	if os.Getenv("JAMB_LIVE_TESTS") != "1" {
		t.Skip("opens a real window; set JAMB_LIVE_TESTS=1 to run it")
	}
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 200, 255
	}
	got := windowIconSides(t, img, func(w *window) string {
		for _, dpi := range []uint32{w.dpi, 96, 144, 192} {
			w.dpi = dpi
			w.updateIcons()
			small, big := win32.WindowIcon(w.hwnd, win32.ICON_SMALL), win32.WindowIcon(w.hwnd, win32.ICON_BIG)
			ws, wb := iconSize(dpi, smallIconMetric), bigIconSize(dpi, bigIconMetric)
			if s, b := iconSide(small), iconSide(big); s != ws || b != wb {
				return fmt.Sprintf("at %d DPI, WM_GETICON gives a %d px small and a %d px big icon; want %d and %d", dpi, s, b, ws, wb)
			}
		}
		return ""
	})
	if got != "" {
		t.Error(got)
	}
}

// A program without an icon resource, as this test is, and without an Icon, opens
// its window as before: the window is given no icon, and nothing fails.
func TestWindowWithoutIconResource(t *testing.T) {
	if os.Getenv("JAMB_LIVE_TESTS") != "1" {
		t.Skip("opens a real window; set JAMB_LIVE_TESTS=1 to run it")
	}
	if win32.LoadIcon(win32.GetModuleHandle(), exeIconResource, 32, false) != 0 {
		t.Skip("the test executable has an icon resource")
	}
	got := windowIconSides(t, nil, func(w *window) string {
		if small, big := win32.WindowIcon(w.hwnd, win32.ICON_SMALL), win32.WindowIcon(w.hwnd, win32.ICON_BIG); small != 0 || big != 0 {
			return fmt.Sprintf("WM_GETICON gives %#x and %#x; want no icons", small, big)
		}
		return ""
	})
	if got != "" {
		t.Error(got)
	}
}

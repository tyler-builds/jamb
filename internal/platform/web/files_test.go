//go:build js

package web

import (
	"image"
	"image/color"
	"syscall/js"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
)

func TestAcceptOf(t *testing.T) {
	images := dialog.Filter{Name: "Images", Patterns: []string{"*.png", "*.jpg"}}
	for _, c := range []struct {
		filters []dialog.Filter
		want    string
	}{
		{nil, ""},
		{[]dialog.Filter{images}, ".png,.jpg"},
		{[]dialog.Filter{images, {Name: "Notes", Patterns: []string{"*.md"}}}, ".png,.jpg,.md"},
		{[]dialog.Filter{images, {Name: "All files", Patterns: []string{"*.*"}}}, ""},
		{[]dialog.Filter{{Name: "Reports", Patterns: []string{"report-*.csv"}}}, ""},
	} {
		if got := acceptOf(c.filters); got != c.want {
			t.Errorf("acceptOf(%v) = %q, want %q", c.filters, got, c.want)
		}
	}
}

// An image is decoded at its size, and one too large for the app's memory at
// the largest size that fits, in its proportions.
func TestImageSize(t *testing.T) {
	for _, c := range [][4]int{
		{1179, 2556, 1179, 2556},
		{5120, 2880, 5120, 2880},
		{16384, 1024, 8192, 512},
		{8000, 8000, 5792, 5792},
		{0, 100, 0, 0},
	} {
		if w, h := imageSize(c[0], c[1]); w != c[2] || h != c[3] {
			t.Errorf("imageSize(%d, %d) = %d, %d; want %d, %d", c[0], c[1], w, h, c[2], c[3])
		}
	}
}

// A canvas takes straight colors, so an image whose colors are multiplied by
// their alpha is converted, and a part of an image is copied out of it.
func TestStraightPixels(t *testing.T) {
	pre := image.NewRGBA(image.Rect(0, 0, 2, 1))
	pre.Set(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	pre.Set(1, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	got := straightPixels(pre)
	if len(got) != 8 || got[3] != 128 || got[0] < 198 || got[0] > 202 || got[4] != 10 || got[7] != 255 {
		t.Errorf("the pixels of a premultiplied image are %v", got)
	}
	whole := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	whole.Set(2, 1, color.NRGBA{R: 9, G: 8, B: 7, A: 6})
	part := whole.SubImage(image.Rect(2, 1, 4, 3))
	if got := straightPixels(part); len(got) != 16 || got[0] != 9 || got[3] != 6 {
		t.Errorf("the pixels of a part of an image are %v", got)
	}
}

// setNavigator gives the test a page whose clipboard is clip, or one without a
// clipboard when clip is undefined.
func setNavigator(t *testing.T, clip js.Value) {
	g := js.Global()
	nav := g.Get("Object").New()
	nav.Set("clipboard", clip)
	old := g.Get("navigator")
	define := func(v js.Value) {
		g.Get("Object").Call("defineProperty", g, "navigator",
			map[string]any{"value": v, "configurable": true, "writable": true})
	}
	define(nav)
	t.Cleanup(func() { define(old) })
}

// pastes returns the count of the Ctrl+V presses among the events.
func pastes(events []platform.Event) int {
	n := 0
	for _, e := range events {
		if e.Kind == platform.EventKeyDown && e.Key == input.KeyV && e.Mods == input.ModCtrl {
			n++
		}
	}
	return n
}

// A read of the clipboard while the app handles a click asks the browser for
// the system's clipboard, which then arrives as a paste. Outside a click, and
// in a page without a clipboard, the clipboard is the app's own.
func TestClipboardAsksTheBrowserInAClick(t *testing.T) {
	w, got := eventWindow(t)
	w.clipText = "copied in the app"
	setNavigator(t, js.Undefined())
	inGesture(func() {
		if s, ok := w.Clipboard(); !ok || s != "copied in the app" {
			t.Errorf("in a page without a clipboard, a click reads %q, %v", s, ok)
		}
	})

	asked := 0
	read := js.FuncOf(func(js.Value, []js.Value) any {
		asked++
		return js.Global().Get("Promise").Call("resolve", "from the system")
	})
	t.Cleanup(read.Release)
	clip := js.Global().Get("Object").New()
	clip.Set("readText", read)
	setNavigator(t, clip)
	if s, _ := w.Clipboard(); s != "copied in the app" || asked != 0 {
		t.Errorf("outside a click the clipboard reads %q, and asked the browser %d times", s, asked)
	}
	inGesture(func() {
		if s, ok := w.Clipboard(); ok || s != "" {
			t.Errorf("a click read %q, %v before the browser answered", s, ok)
		}
		w.Clipboard()
		if _, ok := w.ClipboardImage(); ok || w.HasClipboardImage() {
			t.Error("the clipboard has an image before the browser answered")
		}
	})
	if asked != 1 {
		t.Fatalf("a click asked the browser %d times, want once", asked)
	}
	for range 100 {
		if !w.clipPending {
			break
		}
		time.Sleep(time.Millisecond) // lets the promise's callback run
		runPosted()
	}
	if s, ok := w.Clipboard(); !ok || s != "from the system" {
		t.Errorf("after the browser answered, the clipboard reads %q, %v", s, ok)
	}
	if n := pastes(*got); n != 1 {
		t.Errorf("the browser's answer arrived as %d pastes, want 1", n)
	}
}

// When the browser refuses, the paste that arrives is of the app's own clipboard.
func TestClipboardRefused(t *testing.T) {
	w, got := eventWindow(t)
	w.clipText = "copied in the app"
	read := js.FuncOf(func(js.Value, []js.Value) any {
		return js.Global().Get("Promise").Call("reject", "NotAllowedError")
	})
	t.Cleanup(read.Release)
	clip := js.Global().Get("Object").New()
	clip.Set("readText", read)
	setNavigator(t, clip)
	inGesture(func() { w.Clipboard() })
	for range 100 {
		if !w.clipPending {
			break
		}
		time.Sleep(time.Millisecond)
		runPosted()
	}
	if s, _ := w.Clipboard(); s != "copied in the app" || pastes(*got) != 1 {
		t.Errorf("after the browser refused, the clipboard reads %q, after %d pastes", s, pastes(*got))
	}
}

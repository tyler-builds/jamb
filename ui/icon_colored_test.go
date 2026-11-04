package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/icons"
)

// An icon that keeps its colors is drawn as an image of them, at the icon's
// size, and a plain one as a tinted mask.
func TestColoredIconDrawsItsOwnColors(t *testing.T) {
	logo := icons.MustNew("test-red-logo", `<svg viewBox="0 0 1 1"><rect width="1" height="1" fill="#ff0000"/></svg>`, icons.KeepColors)
	rt := newTestRuntime(func(*Context) Element {
		return Div().Row().Child(Icon(logo).Size(20)).Child(Icon(icons.Bell).Size(20))
	}, 200, 100, 1)
	rt.Frame(time.Now())
	if len(rt.scene.Images) != 1 {
		t.Fatalf("%d image draws, want 1 for the colored icon", len(rt.scene.Images))
	}
	im := rt.scene.Images[0]
	if im.Bounds.W != 20 || im.Texture.Width != 20 {
		t.Errorf("colored icon drawn %v wide from a %d px texture, want 20 and 20", im.Bounds.W, im.Texture.Width)
	}
	if p := pixel(im.Texture, 10, 10); p != [4]uint8{255, 0, 0, 255} {
		t.Errorf("colored icon pixel %v, want opaque red", p)
	}
	if len(rt.scene.Glyphs) == 0 {
		t.Error("the plain icon was not drawn as a mask")
	}
}

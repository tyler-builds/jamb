package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

var testSVG = MustSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" width="20" height="20">
	<rect width="10" height="10" fill="currentColor"/></svg>`))

// An SVG takes its own size, draws currentColor in the inherited text color
// or its Color, and is drawn again only when what it depends on changes.
func TestSVGColorSizeAndCache(t *testing.T) {
	n := NewSignal(0)
	color := NewSignal(Color{})
	rt := newTestRuntime(func(ctx *Context) Element {
		n.Get()
		return Div().TextColor(Color{R: 1, A: 1}).Child(SVG(testSVG).Color(color.Get()))
	}, 100, 100, 1)
	im := canvasImage(t, rt)
	if im.Bounds.W != 20 || im.Bounds.H != 20 {
		t.Fatalf("drawn at %+v, want 20x20", im.Bounds)
	}
	if got := pixel(im.Texture, 10, 10); got != [4]uint8{255, 0, 0, 255} {
		t.Errorf("currentColor drew %v, want the text color", got)
	}
	v := im.Texture.Version
	n.Set(1)
	frame(rt)
	if canvasImage(t, rt).Texture.Version != v {
		t.Error("the parent rendering again drew the SVG again")
	}
	color.Set(Color{B: 1, A: 1})
	frame(rt)
	if got := pixel(canvasImage(t, rt).Texture, 10, 10); got != [4]uint8{0, 0, 255, 255} {
		t.Errorf("Color drew %v, want blue", got)
	}
}

// An SVG with Alt is an image to assistive technology; one without is
// decorative and left out.
func TestSVGAccess(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Child(SVG(testSVG).Alt("Logo")).Child(SVG(testSVG).Size(Px(40), Px(10)))
	})
	tree := r.tree()
	accesstest.Find(t, tree, access.RoleImage, "Logo")
	images := 0
	tree.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleImage {
			images++
		}
		return true
	})
	if images != 1 {
		t.Errorf("%d images in the tree, want only the one with Alt", images)
	}
}

func TestSVGFromBytesError(t *testing.T) {
	if _, err := SVGFromBytes([]byte("<html/>")); err == nil {
		t.Error("an HTML document parsed as SVG")
	}
}

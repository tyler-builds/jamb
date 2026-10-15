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

// stripeSVG is a square whose top fifth is red, for telling the fits apart.
var stripeSVG = MustSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" width="20" height="20">
	<rect width="10" height="2" fill="#ff0000"/></svg>`))

// Fit places an SVG in its box as it places an Image: cover crops it, contain
// letterboxes it, and fill stretches it. Without Fit the image's
// preserveAspectRatio places it, which here is contain.
func TestSVGFit(t *testing.T) {
	cases := []struct {
		name       string
		el         *SVGElement
		red, clear [][2]int // pixels in the red stripe, and pixels outside it
	}{
		// The 40 by 20 box shows the square at 20 by 20 in its middle, with the
		// stripe 4 px high.
		{"default", SVG(stripeSVG), [][2]int{{20, 1}}, [][2]int{{5, 1}, {20, 10}}},
		{"contain", SVG(stripeSVG).Fit(FitContain), [][2]int{{20, 1}}, [][2]int{{5, 1}, {20, 10}}},
		// The square is 40 by 40, and the box shows its rows 10 to 30, which crops
		// the stripe (rows 0 to 8) away.
		{"cover", SVG(stripeSVG).Fit(FitCover), nil, [][2]int{{5, 1}, {20, 1}, {20, 10}}},
		// The square is stretched to 40 by 20, and the stripe is 4 px high across it.
		{"fill", SVG(stripeSVG).Fit(FitFill), [][2]int{{5, 1}, {35, 1}}, [][2]int{{5, 10}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			el := c.el.Size(Px(40), Px(20))
			rt := newTestRuntime(func(ctx *Context) Element { return Div().Child(el) }, 100, 100, 1)
			tex := canvasImage(t, rt).Texture
			for _, p := range c.red {
				if got := pixel(tex, p[0], p[1]); got != [4]uint8{255, 0, 0, 255} {
					t.Errorf("pixel %v is %v, want the red stripe", p, got)
				}
			}
			for _, p := range c.clear {
				if got := pixel(tex, p[0], p[1]); got != [4]uint8{} {
					t.Errorf("pixel %v is %v, want clear", p, got)
				}
			}
		})
	}
}

// Radius rounds the corners of an SVG and Opacity fades it, as they do for an
// Image.
func TestSVGRadiusAndOpacity(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().TextColor(Color{R: 1, A: 1}).Child(SVG(testSVG).Radius(10).Opacity(0.5))
	}, 100, 100, 1)
	im := canvasImage(t, rt)
	if got := pixel(im.Texture, 0, 0); got[3] != 0 {
		t.Errorf("the rounded corner drew %v, want nothing", got)
	}
	if got := pixel(im.Texture, 10, 10); got != [4]uint8{255, 0, 0, 255} {
		t.Errorf("the middle drew %v, want red", got)
	}
	if im.Opacity != 0.5 {
		t.Errorf("drawn at opacity %v, want 0.5", im.Opacity)
	}
	// Under contain the corners of the letterboxed image are rounded, as an
	// Image's are.
	rt = newTestRuntime(func(ctx *Context) Element {
		return Div().TextColor(Color{R: 1, A: 1}).Child(SVG(testSVG).Size(Px(40), Px(20)).Fit(FitContain).Radius(10))
	}, 100, 100, 1)
	tex := canvasImage(t, rt).Texture
	if got := pixel(tex, 10, 0); got[3] != 0 {
		t.Errorf("the image's rounded corner drew %v, want nothing", got)
	}
	if got := pixel(tex, 20, 10); got != [4]uint8{255, 0, 0, 255} {
		t.Errorf("the middle drew %v, want red", got)
	}
	rt = newTestRuntime(func(ctx *Context) Element {
		return Div().TextColor(Color{R: 1, A: 1}).Child(SVG(testSVG))
	}, 100, 100, 1)
	if got := pixel(canvasImage(t, rt).Texture, 0, 0); got != [4]uint8{255, 0, 0, 255} {
		t.Errorf("the square corner drew %v, want red", got)
	}
}

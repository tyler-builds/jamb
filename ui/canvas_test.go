package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/internal/scene"
)

// canvasImage returns the drawing of the scene's canvas: the image draw from
// an RGBA texture that no ImageSource made.
func canvasImage(t *testing.T, rt *Runtime) scene.ImageDraw {
	t.Helper()
	for _, im := range rt.scene.Images {
		if im.Texture != nil && im.Texture.Format == scene.FormatRGBA8 {
			return im
		}
	}
	t.Fatal("the scene has no canvas image")
	return scene.ImageDraw{}
}

// pixel returns the premultiplied RGBA of the texture's pixel x, y.
func pixel(tex *scene.Texture, x, y int) [4]uint8 {
	p := tex.Pix[(y*tex.Width+x)*4:]
	return [4]uint8{p[0], p[1], p[2], p[3]}
}

// A canvas draws at the display's resolution, in its content box, and draws
// again only when its component renders or its size changes.
func TestCanvasDrawsAndRedraws(t *testing.T) {
	n := NewSignal(0)
	draws := 0
	var gotW, gotH float32
	rt := newTestRuntime(func(ctx *Context) Element {
		n.Get()
		return Div().Padding(10).Child(Canvas(func(p *Painter, w, h float32) {
			draws++
			gotW, gotH = w, h
			p.FillRect(0, 0, w/2, h, Color{R: 1, A: 1})
		}).Size(Px(100), Px(40)))
	}, 300, 200, 2)
	im := canvasImage(t, rt)
	if gotW != 100 || gotH != 40 || im.Texture.Width != 200 || im.Texture.Height != 80 {
		t.Fatalf("drawn at %vx%v into %dx%d pixels, want 100x40 into 200x80", gotW, gotH, im.Texture.Width, im.Texture.Height)
	}
	if im.Bounds != (scene.Rect{X: 20, Y: 20, W: 200, H: 80}) {
		t.Errorf("drawn at %+v", im.Bounds)
	}
	if l, r := pixel(im.Texture, 50, 40), pixel(im.Texture, 150, 40); l != [4]uint8{255, 0, 0, 255} || r[3] != 0 {
		t.Errorf("left half %v, right half %v", l, r)
	}
	frame(rt)
	frame(rt)
	if draws != 1 {
		t.Fatalf("drew %d times without a change, want 1", draws)
	}
	n.Set(1)
	frame(rt)
	if draws != 2 {
		t.Fatalf("drew %d times after the component rendered, want 2", draws)
	}
	id := canvasImage(t, rt).Texture.ID
	rt.SetViewport(300, 200, 1)
	frame(rt)
	if im := canvasImage(t, rt); draws != 3 || im.Texture.Width != 100 || im.Texture.ID != id {
		t.Fatalf("after a scale change: %d draws, %d pixels wide, texture %d (was %d)", draws, im.Texture.Width, im.Texture.ID, id)
	}
}

// Text is placed at its point with its alignment, over the shapes.
func TestCanvasText(t *testing.T) {
	var tw float32
	rt := newTestRuntime(func(ctx *Context) Element {
		return Canvas(func(p *Painter, w, h float32) {
			p.FillRect(0, 0, w, h, Color{B: 1, A: 1})
			tw, _ = p.MeasureText("Label", TextFormat{})
			p.Translate(10, 0)
			p.Text(90, 10, "Label", TextFormat{Align: End})
		}).Size(Px(200), Px(50))
	}, 300, 200, 1)
	if tw <= 0 || len(rt.scene.Glyphs) == 0 {
		t.Fatalf("text width %v, %d glyphs drawn", tw, len(rt.scene.Glyphs))
	}
	// End alignment puts the text's right edge at 10 + 90.
	var right float32
	for _, g := range rt.scene.Glyphs {
		right = max(right, g.Bounds.X+g.Bounds.W)
	}
	if right < 100-3 || right > 100+1 {
		t.Errorf("the text ends at %v, want about 100", right)
	}
}

// Pointer handlers get the position in the canvas, and assistive technology
// sees a named image.
func TestCanvasEventsAndAccess(t *testing.T) {
	var at [2]float32
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Padding(20).Child(Canvas(func(p *Painter, w, h float32) {}).
			Label("Sales by month").
			OnMouseMove(func(e *MouseEvent) { at = [2]float32{e.LocalX, e.LocalY} }))
	})
	r.rt.PointerMove(50, 30, 0)
	if at != [2]float32{30, 10} {
		t.Errorf("the pointer is at %v in the canvas, want 30, 10", at)
	}
	accesstest.Find(t, r.tree(), access.RoleImage, "Sales by month")
}

// Paths: arcs, even-odd holes, gradients and strokes reach the drawing.
func TestCanvasPaths(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Canvas(func(p *Painter, w, h float32) {
			ring := NewPath().Circle(50, 50, 40).Circle(50, 50, 20).EvenOdd()
			p.Fill(ring, LinearPaint(0, 0, 100, 0, ColorStop{0, Color{R: 1, A: 1}}, ColorStop{1, Color{G: 1, A: 1}}))
			p.Stroke(NewPath().MoveTo(110, 10).LineTo(190, 10), StrokeStyle{Width: 4, Color: Color{B: 1, A: 1}})
			p.Save()
			p.Clip(NewPath().Rect(110, 50, 20, 50))
			p.Fill(NewPath().Rect(110, 50, 80, 50), Color{R: 1, G: 1, B: 1, A: 1})
			p.Restore()
		}).Size(Px(200), Px(100))
	}, 300, 200, 1)
	tex := canvasImage(t, rt).Texture
	if hole := pixel(tex, 50, 50); hole[3] != 0 {
		t.Errorf("the even-odd hole is filled: %v", hole)
	}
	if left, right := pixel(tex, 15, 50), pixel(tex, 85, 50); left[0] < 200 || right[1] < 200 {
		t.Errorf("gradient left %v, right %v", left, right)
	}
	if line := pixel(tex, 150, 10); line != [4]uint8{0, 0, 255, 255} {
		t.Errorf("stroke pixel %v", line)
	}
	if in, out := pixel(tex, 115, 70), pixel(tex, 150, 70); in[3] != 255 || out[3] != 0 {
		t.Errorf("inside the clip %v, outside %v", in, out)
	}
}

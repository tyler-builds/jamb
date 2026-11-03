package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/soft"
)

// A runtime's scene drawn by the CPU renderer shows its elements where layout
// put them, at the window's scale, text included.
func TestSceneDrawsWithSoftRenderer(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(10).Background(Hex("#000000")).
			Child(Div().Key("box").Width(Px(40)).Height(Px(20)).Background(Hex("#ff8000"))).
			Child(Text("Hello").Key("hello").TextColor(Hex("#ffffff")))
	}, DarkTheme(), nil)
	const w, h, scale = 100, 80, 2
	rt.SetViewport(w, h, scale)
	sc := rt.frame(time.Unix(1_000_000, 0))
	r := soft.New(nil)
	if err := render.Draw(r, sc, w*scale, h*scale); err != nil {
		t.Fatal(err)
	}
	img := r.Image()
	box, _ := rt.Find("box")
	cx, cy := int((box.X+box.W/2)*scale), int((box.Y+box.H/2)*scale)
	if got := img.RGBAAt(cx, cy); got.R != 255 || got.G != 128 || got.B != 0 {
		t.Errorf("the box's center (%d,%d) is %v, want #ff8000", cx, cy, got)
	}
	hello, _ := rt.Find("hello")
	lit := 0
	for y := int(hello.Y * scale); y < int((hello.Y+hello.H)*scale); y++ {
		for x := int(hello.X * scale); x < int((hello.X+hello.W)*scale); x++ {
			if img.RGBAAt(x, y).R > 128 {
				lit++
			}
		}
	}
	if lit < 50 {
		t.Errorf("the text lit %d pixels, want its glyphs drawn", lit)
	}
}

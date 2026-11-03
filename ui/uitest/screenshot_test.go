package uitest_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// A screenshot is the window's size times its scale, and shows what the UI draws:
// here a red box on a white window, and an image scaled into a box.
func TestScreenshot(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 222, 222))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i+1], src.Pix[i+3] = 255, 255 // green
	}
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Grow(1).Background(ui.Hex("#ffffff")).
			Child(ui.Div().Key("box").Width(ui.Px(40)).Height(ui.Px(20)).Background(ui.Hex("#ff0000"))).
			Child(ui.Image(ui.ImageFromImage(src)).Size(ui.Px(150), ui.Px(150)).Alt("green"))
	}, uitest.Size(200, 200), uitest.Scale(1.5))
	img := u.Screenshot()
	if b := img.Bounds(); b.Dx() != 300 || b.Dy() != 300 {
		t.Fatalf("screenshot is %v; want 300x300", b)
	}
	at := func(x, y int) color.RGBA { return img.RGBAAt(x, y) }
	if c := at(5, 5); c.R != 255 || c.G != 0 || c.B != 0 {
		t.Errorf("inside the box %v; want red", c)
	}
	if c := at(299, 10); c != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("beside the box %v; want white", c)
	}
	// The image is 150 logical pixels square, under the 20-pixel box: 225 device
	// pixels, from y 30 to 255.
	if c := at(100, 150); c.G != 255 || c.R != 0 {
		t.Errorf("inside the image %v; want green", c)
	}
	if c := at(230, 150); c.G == 255 && c.R == 0 {
		t.Errorf("past the image's right edge %v; want the window", c)
	}
	if c := at(100, 260); c.R != 255 {
		t.Errorf("under the image %v; want white", c)
	}
}

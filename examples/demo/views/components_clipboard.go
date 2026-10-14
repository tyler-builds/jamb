package views

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// clipboardSection copies an image to the clipboard and pastes images from it:
// with a button, and into a text area that takes pasted screenshots.
func clipboardSection(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		pasted := ui.Local(ctx, "clip.pasted", (*ui.ImageSource)(nil))
		info := ui.Local(ctx, "clip.info", "Nothing pasted yet")
		note := ui.Local(ctx, "clip.note", "")
		show := func(img image.Image, how string) {
			b := img.Bounds()
			pasted.Set(ui.ImageFromImage(img))
			info.Set(fmt.Sprintf("%s: %d x %d pixels", how, b.Dx(), b.Dy()))
			ui.Announce("Image pasted, "+fmt.Sprintf("%d by %d pixels", b.Dx(), b.Dy()), access.Polite)
		}

		var preview ui.Element = ui.Div().Size(ui.Px(240), ui.Px(140)).Border(1, t.Border).Radius(8).
			Align(ui.Center).Justify(ui.Center).Child(ui.Text("No image").TextColor(t.TextSubtle))
		if src := pasted.Get(); src != nil {
			// A checkerboard behind the image shows where it is transparent.
			preview = ui.Div().Size(ui.Px(240), ui.Px(140)).Border(1, t.Border).Radius(8).ClipContent().
				Child(ui.Canvas(checkerboard).Size(ui.Full, ui.Full).Absolute().AccessHidden()).
				Child(ui.Image(src).Size(ui.Full, ui.Full).Fit(ui.FitContain).Alt("The pasted image"))
		}

		return section(t, "Clipboard images",
			sample(t, "Copy an image with transparency, then paste it into Paint or a browser",
				ui.Div().Row().Gap(8).
					Child(ui.Button("Copy sample image").Key("clip-copy").OnClick(func() {
						ctx.SetClipboardImage(sampleImage())
						ui.Announce("Sample image copied", access.Polite)
					})).
					Child(ui.Button("Paste image").Key("clip-paste").Secondary().OnClick(func() {
						if !ctx.ClipboardHasImage() {
							info.Set("The clipboard holds no image")
							ui.Announce("The clipboard holds no image", access.Polite)
							return
						}
						if img, ok := ctx.ClipboardImage(); ok {
							show(img, "Pasted with the button")
						}
					}))),
			sample(t, "Paste a screenshot (Ctrl+V) into the text area; text still pastes as text",
				ui.Div().Width(ui.Px(280)).Child(ui.TextArea(note).Label("Message").
					Placeholder("Write a message, or paste an image").
					OnPasteImage(func(img image.Image) { show(img, "Pasted into the message") }))),
			sample(t, info.Get(), preview))
	})
}

// sampleImage is a 200 x 120 gradient with a transparent hole and a half
// transparent band, so a paste shows whether the transparency came through.
func sampleImage() image.Image {
	const w, h = 200, 120
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.NRGBA{R: uint8(60 + 160*x/w), G: uint8(90 + 100*y/h), B: 230, A: 255}
			dx, dy := float64(x-140), float64(y-60)
			switch {
			case math.Hypot(dx, dy) < 28:
				c.A = 0 // a hole
			case y >= 92:
				c.A = 128 // a half transparent band
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// checkerboard draws the gray squares that stand for transparency.
func checkerboard(p *ui.Painter, w, h float32) {
	const s = 10
	light, dark := ui.Hex("#d4d4d8"), ui.Hex("#a1a1aa")
	p.FillRect(0, 0, w, h, light)
	for y := float32(0); y < h; y += s {
		for x := float32(0); x < w; x += s {
			if int(x/s+y/s)%2 == 1 {
				p.FillRect(x, y, s, s, dark)
			}
		}
	}
}

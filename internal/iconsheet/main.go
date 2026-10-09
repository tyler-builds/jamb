// Command iconsheet renders every built-in icon to a PNG contact sheet:
//
//	go run ./internal/iconsheet sheet.png
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"github.com/tyler-builds/jamb/icons"
)

func main() {
	const cell, size, cols = 56, 32, 13
	rows := (len(icons.All) + cols - 1) / cols
	out := image.NewRGBA(image.Rect(0, 0, cols*cell, rows*cell))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.RGBA{24, 24, 27, 255}), image.Point{}, draw.Src)
	fg := image.NewUniform(color.RGBA{235, 235, 240, 255})
	for i, ic := range icons.All {
		m := image.NewAlpha(image.Rect(0, 0, size, size))
		ic.Rasterize(m)
		x, y := (i%cols)*cell+(cell-size)/2, (i/cols)*cell+(cell-size)/2
		draw.DrawMask(out, image.Rect(x, y, x+size, y+size), fg, image.Point{}, m, image.Point{}, draw.Over)
	}
	f, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	png.Encode(f, out)
}

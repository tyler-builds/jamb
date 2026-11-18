package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// A grid cell's clip cuts the text in it as it cuts the shapes: no ink of a
// long cell text shows past the cell, while the text inside it is drawn.
func TestCanvasCellTextIsClipped(t *testing.T) {
	blue := ui.RGB(0, 0, 200)
	u := uitest.New(t, func(*ui.Context) ui.Element {
		return ui.Canvas(func(p *ui.Painter, w, h float32) {
			p.FillRect(0, 0, w, h, ui.RGB(255, 255, 255))
			p.Save()
			p.Clip(ui.NewPath().Rect(12, 10, 90, 26))
			p.FillRect(12, 30, 400, 4, ui.RGB(40, 160, 40))
			p.Text(16, 14, "SELECT * FROM dbo.VeryLongTableName", ui.TextFormat{Color: blue})
			p.Restore()
			p.Text(16, 50, "SELECT * FROM dbo.VeryLongTableName", ui.TextFormat{Color: blue})
		}).Size(ui.Px(400), ui.Px(80))
	}, uitest.Size(400, 80))
	img := u.Screenshot()
	isBlue := func(x, y int) bool {
		r, g, b, _ := img.At(x, y).RGBA()
		return b>>8 > r>>8+40 && b>>8 > g>>8+40
	}
	count := func(x0, y0, x1, y1 int) int {
		n := 0
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if isBlue(x, y) {
					n++
				}
			}
		}
		return n
	}
	if n := count(12, 10, 102, 36); n == 0 {
		t.Errorf("no text drawn inside the cell")
	}
	if n := count(103, 0, 400, 40); n != 0 {
		t.Errorf("%d pixels of the cell's text show past the cell", n)
	}
	if n := count(103, 46, 400, 80); n == 0 {
		t.Errorf("the text after Restore is clipped too")
	}
}

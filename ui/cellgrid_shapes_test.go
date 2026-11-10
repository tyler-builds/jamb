package ui_test

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// TestCellGridShapesJoin checks that box drawing is drawn to the edges of its
// cells: a vertical line down a column has no gaps between rows, whatever the
// font's line height.
func TestCellGridShapesJoin(t *testing.T) {
	lines := []string{
		"╭──┬──╮ ┏━━┓ ╔══╦══╗ ┌┄┄┐",
		"│ab│cd│ ┃  ┃ ║  ║  ║ ┆  ┆",
		"├──┼──┤ ┣━━┫ ╠══╬══╣ └┄┄┘",
		"│  │  │ ┃  ┃ ║  ║  ║ ▁▂▃▄▅▆▇█",
		"╰──┴──╯ ┗━━┛ ╚══╩══╝ ▖▗▘▙▚▛▜▝▞▟",
		"⣿⣿⣀⣤⣶⡇⢸ ░▒▓ ▀▄▌▐ ╒═╕╓─╖",
	}
	cols := 0
	for _, l := range lines {
		cols = max(cols, len([]rune(l)))
	}
	buf := ui.NewCellBuffer(cols, len(lines))
	buf.Lock()
	for y, l := range lines {
		for x, r := range []rune(l) {
			buf.Set(x, y, ui.Cell{Text: string(r)})
		}
	}
	buf.Unlock()
	white := ui.Hex("#ffffff")
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Size(ui.Full, ui.Full).Background(ui.Hex("#000000")).
			Child(ui.CellGrid(buf, nil).Key("grid").FontSize(16).TextColor(white))
	}, uitest.Size(420, 160))
	u.Frame()
	img := u.Screenshot()
	if dir := os.Getenv("JAMB_SHOTS"); dir != "" {
		f, _ := os.Create(filepath.Join(dir, "cellgrid-shapes.png"))
		png.Encode(f, img)
		f.Close()
	}

	// Column 0 of rows 1 to 3 is │ ├ │: one light line down the middle of the cell.
	cw, ch := buf.CellSize()
	if cw <= 0 || ch <= 0 {
		t.Fatal("the grid has no cell size")
	}
	b := u.Get("grid").Bounds()
	lit := func(x, y int) bool {
		r, _, _, _ := img.At(x, y).RGBA()
		return r > 0x8000
	}
	// Find the line's column of pixels in row 1.
	y := int(b.Y + 1.5*ch)
	lineX := -1
	for x := int(b.X); x < int(b.X+cw); x++ {
		if lit(x, y) {
			lineX = x
			break
		}
	}
	if lineX < 0 {
		t.Fatal("no vertical line in the cell at column 0, row 1")
	}
	for y := int(b.Y + ch); y < int(b.Y+4*ch); y++ {
		if !lit(lineX, y) {
			t.Fatalf("the vertical line has a gap at y=%d (rows %.0f px tall)", y, ch)
		}
	}
}

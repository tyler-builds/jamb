package vector

import (
	"fmt"
	"testing"
)

// cellWidths are the canvas widths the cell benchmarks draw at. The cells stay in
// the same 200 by 200 corner, so a fill's cost should not follow the width.
var cellWidths = []int{200, 800, 3200}

var cellPaint = Paint{Color: Color{R: 0.12, G: 0.35, B: 0.78, A: 1}}

// fillCells fills n cells of 8 by 8 pixels on a 10-pixel pitch, 20 to a row, each
// clipped to itself first when clip is set, as a grid of results draws them.
func fillCells(c *Canvas, p *Path, n int, clip bool) {
	for k := range n {
		x, y := float32(k%20*10), float32(k/20*10)
		clearPath(p)
		p.Rect(x, y, 8, 8)
		if clip {
			c.Save()
			c.Clip(p, NonZero)
		}
		c.Fill(p, cellPaint, NonZero)
		if clip {
			c.Restore()
		}
	}
}

// BenchmarkFillCells fills 400 small cells on a canvas 200 pixels high and 200,
// 800 or 3200 wide.
func BenchmarkFillCells(b *testing.B) {
	for _, w := range cellWidths {
		b.Run(fmt.Sprintf("w=%d", w), func(b *testing.B) {
			c := NewCanvas(w, 200)
			p := &Path{}
			b.ReportAllocs()
			for b.Loop() {
				fillCells(c, p, 400, false)
			}
		})
	}
}

// BenchmarkClipCells fills 100 small cells, each in a Save, a Clip to the cell and
// a Restore, at the widths of BenchmarkFillCells.
func BenchmarkClipCells(b *testing.B) {
	for _, w := range cellWidths {
		b.Run(fmt.Sprintf("w=%d", w), func(b *testing.B) {
			c := NewCanvas(w, 200)
			p := &Path{}
			b.ReportAllocs()
			for b.Loop() {
				fillCells(c, p, 100, true)
			}
		})
	}
}

// drawGrid draws a grid of results w by h: rows 20 pixels high in columns 130
// wide, every fifth cell filled as a selection, and a 1-pixel line under each row
// and left of each column.
func drawGrid(c *Canvas, p *Path, w, h float32) {
	const rh, cw = 20, 130
	sel := Paint{Color: Color{R: 0.86, G: 0.9, B: 0.98, A: 1}}
	line := Paint{Color: Color{R: 0.78, G: 0.78, B: 0.78, A: 1}}
	rect := func(x, y, rw, rhh float32, paint Paint) {
		clearPath(p)
		p.Rect(x, y, rw, rhh)
		c.Fill(p, paint, NonZero)
	}
	rows, cols := int(h/rh), int(w/cw)
	for r := range rows {
		for col := range cols {
			if (r+col)%5 == 0 {
				rect(float32(col*cw), float32(r*rh), cw, rh, sel)
			}
		}
		rect(0, float32(r*rh), w, 1, line)
	}
	for col := 0; col <= cols; col++ {
		rect(float32(col*cw), 0, 1, h, line)
	}
}

// BenchmarkGrid1600x900 clears a 1600 by 900 canvas and draws a grid of results
// on it, as a canvas that draws a table does each frame.
func BenchmarkGrid1600x900(b *testing.B) {
	c := NewCanvas(1600, 900)
	p := &Path{}
	b.ReportAllocs()
	for b.Loop() {
		c.Reset(1600, 900)
		drawGrid(c, p, 1600, 900)
	}
}

// clearPath empties p, keeping its buffers.
func clearPath(p *Path) { *p = Path{verbs: p.verbs[:0], pts: p.pts[:0]} }

package ui

import (
	"testing"
	"time"
)

// BenchmarkCanvasGrid redraws a canvas that fills a 1600 by 900 window with a
// grid of results, as a table drawn on a canvas does each time its data
// changes: 45 rows 20 pixels high in 12 columns 130 wide, every fifth cell
// filled as a selection, and a 1-pixel line under each row and left of each
// column. Each frame renders again, so the canvas draws again.
//
//	go test ./ui -run xxx -bench CanvasGrid -benchmem
func BenchmarkCanvasGrid(b *testing.B) {
	tick := NewSignal(0)
	sel, line := RGB(220, 230, 250), RGB(200, 200, 200)
	grid := func(p *Painter, w, h float32) {
		const rh, cw = 20, 130
		for r := range 45 {
			for c := range 12 {
				if (r+c)%5 == 0 {
					p.FillRect(float32(c*cw), float32(r*rh), cw, rh, sel)
				}
			}
			p.FillRect(0, float32(r*rh), w, 1, line)
		}
		for c := 0; c <= 12; c++ {
			p.FillRect(float32(c*cw), 0, 1, h, line)
		}
	}
	rt := NewRuntime(func(ctx *Context) Element {
		_ = tick.Get()
		return Canvas(grid).Size(Full, Full)
	}, DarkTheme(), benchText)
	rt.SetViewport(1600, 900, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	b.ReportAllocs()
	for b.Loop() {
		tick.Set(tick.Peek() + 1)
		now = now.Add(time.Millisecond)
		rt.Frame(now)
	}
}

package text

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/internal/scene"
)

// BenchmarkGlyphRaster draws a paragraph of Latin or CJK text with the glyph
// cache warm, and with it cold: the glyphs are rasterized and uploaded to the
// atlas again on each draw. The layout is shaped before the timer starts, so
// only the glyph path is timed. The glyphs/op metric counts the glyphs drawn
// per draw; CJK glyphs come from the system's fallback fonts.
func BenchmarkGlyphRaster(b *testing.B) {
	texts := []struct {
		name, s string
	}{
		{"latin", strings.Repeat("The quick brown fox jumps over the lazy dog 0123456789. ", 4)},
		{"cjk", strings.Repeat("漢字かなカナ中文字符测试内容显示。", 4)},
	}
	for _, c := range texts {
		sys := NewSystem()
		l := sys.Layout(c.s, Style{Size: 14}, inf)
		glyphs := 0
		for _, ln := range l.Lines {
			for _, run := range ln.Runs {
				for _, g := range run.Glyphs {
					if g.font != nil {
						glyphs++
					}
				}
			}
		}
		white := scene.Color{R: 1, G: 1, B: 1, A: 1}
		b.Run(c.name+"/warm", func(b *testing.B) {
			var sc scene.Scene
			sys.Draw(&sc, l, 0, 0, 0, 1, 0, white) // fill the cache
			b.ReportAllocs()
			for b.Loop() {
				sc.Reset()
				sys.Draw(&sc, l, 0, 0, 0, 1, 0, white)
			}
			b.ReportMetric(float64(glyphs), "glyphs/op")
		})
		b.Run(c.name+"/cold", func(b *testing.B) {
			var sc scene.Scene
			b.ReportAllocs()
			for b.Loop() {
				clear(sys.glyphs) // forget the glyphs, keeping the shaped layout
				sc.Reset()
				sys.Draw(&sc, l, 0, 0, 0, 1, 0, white)
			}
			b.ReportMetric(float64(glyphs), "glyphs/op")
		})
	}
}

// BenchmarkDrawCells draws the short texts of a grid's cells, 200 of them with 70
// texts among them, as a frame of a scroll does. "still" draws each at the same
// left edge every frame, as a vertical scroll does (the top changes, the left edge
// does not), and "moving" one pixel further right every frame, as a horizontal
// scroll does. "cached" gives each cell a DrawCache, as each text node has one;
// "plain" draws without.
func BenchmarkDrawCells(b *testing.B) {
	sys := NewSystem()
	cells := make([]*Layout, 200)
	for i := range cells {
		cells[i] = sys.Layout("Item "+strings.Repeat("x", i%7)+"-"+string(rune('0'+i%10)), Style{Size: 14}, inf)
	}
	white := scene.Color{R: 1, G: 1, B: 1, A: 1}
	for _, c := range []struct {
		name   string
		dx     float32
		cached bool
	}{{"still/plain", 0, false}, {"still/cached", 0, true}, {"moving/plain", 1, false}, {"moving/cached", 1, true}} {
		b.Run(c.name, func(b *testing.B) {
			var sc scene.Scene
			caches := make([]DrawCache, len(cells))
			frame := 0
			draw := func() {
				sc.Reset()
				off := float32(frame) * c.dx
				for i, l := range cells {
					var dc *DrawCache
					if c.cached {
						dc = &caches[i]
					}
					sys.DrawCached(&sc, l, dc, float32(i%8)*100+off, float32(frame+i/8*24), 0, 1, 0, white)
				}
				frame++
			}
			for range 3 {
				draw()
			}
			b.ReportAllocs()
			for b.Loop() {
				draw()
			}
		})
	}
}

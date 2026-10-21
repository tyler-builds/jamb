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

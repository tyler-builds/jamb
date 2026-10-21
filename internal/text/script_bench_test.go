package text

import (
	"strings"
	"testing"
)

// BenchmarkLayoutScripts shapes and wraps a paragraph of each kind of text:
// Latin, Latin and Arabic in one line (bidi), Arabic, Devanagari (a complex
// script that needs a fallback face and forms conjuncts) and CJK. Like
// BenchmarkLayoutParagraph, a different width each time defeats the layout
// cache, so this measures shaping and wrapping.
func BenchmarkLayoutScripts(b *testing.B) {
	scripts := []struct {
		name, s string
	}{
		{"latin", "The quick brown fox jumps over the lazy dog. "},
		{"bidi", "Order 1042 ships to مرحبا بالعالم on Monday, then returns to the depot. "},
		{"arabic", "مرحبا بكم في المدينة، وهذا نص عربي طويل يُلف على عدة أسطر. "},
		{"devanagari", "नमस्ते दुनिया, यह एक लंबा हिंदी वाक्य है जो कई पंक्तियों में लपेटा जाता है। "},
		{"cjk", "这是一段中文文本，用于测量换行和排版的速度。日本語の文章も同じように折り返されます。"},
	}
	for _, c := range scripts {
		s := strings.Repeat(c.s, 20)
		b.Run(c.name, func(b *testing.B) {
			sys := NewSystem()
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				sys.build(s, Style{Size: 15}, nil, 300+float32(i)*0.01)
			}
		})
	}
}

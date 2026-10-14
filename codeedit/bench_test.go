package codeedit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// sourceText returns n lines of Go-like source, with indented blocks, so that folding and the
// text runs have something to do.
func sourceText(n int) string {
	var b strings.Builder
	for i := 0; i < n; i += 6 {
		fmt.Fprintf(&b, "func f%d(x int) int {\n", i)
		fmt.Fprintf(&b, "\tif x > %d {\n", i)
		fmt.Fprintf(&b, "\t\treturn x * %d // line %d\n", i, i)
		b.WriteString("\t}\n\treturn x\n}\n")
	}
	return b.String()
}

// BenchmarkOpen100kLines builds a document of 100,000 lines, an editor on it, and draws the first
// frame, which is what opening a file does.
func BenchmarkOpen100kLines(b *testing.B) {
	text := sourceText(100000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newHarness(b, text)
	}
}

// BenchmarkKeystroke50kLines types a character in the middle of a document of 50,000 lines and
// draws the frame after it: the edit, the change to the view, and the paint of the visible rows.
func BenchmarkKeystroke50kLines(b *testing.B) {
	h := newHarness(b, sourceText(50000))
	h.doc.SetCaret(h.doc.Len() / 2)
	h.frame()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.rt.TextInput("x")
		h.frame()
	}
}

// BenchmarkScrollFrame50kLines scrolls a document of 50,000 lines by a few rows and draws the frame
// after each scroll.
func BenchmarkScrollFrame50kLines(b *testing.B) {
	h := newHarness(b, sourceText(50000))
	r, _ := h.rt.Find("wrap")
	x, y := r.X+100, r.Y+100
	h.frame()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.rt.Scroll(x, y, 0, 40, true, input.Modifiers(0))
		h.frame()
	}
}

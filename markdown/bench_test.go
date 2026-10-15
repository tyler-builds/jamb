package markdown

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// largeDocument returns a README-like text of about lines lines: sections with
// headings, paragraphs with emphasis, code and links, lists, a task list, a table, a
// quote and a code block.
func largeDocument(lines int) string {
	var b strings.Builder
	for s := 0; strings.Count(b.String(), "\n") < lines; s++ {
		fmt.Fprintf(&b, "## Section %d\n\n", s)
		fmt.Fprintf(&b, "This is *section* %d of the guide. It has **strong** words, `inline code`, and a [link](https://example.com/%d)\nthat runs over two lines, as paragraphs in a README do.\n\n", s, s)
		b.WriteString("- the first item\n- the second item, with `code`\n  - a nested item\n- the third item\n\n")
		b.WriteString("1. step one\n2. step two\n3. step three\n\n")
		b.WriteString("- [x] done\n- [ ] not done\n\n")
		b.WriteString("| Option | Default | Meaning |\n|:--|:-:|--:|\n| width | auto | how wide |\n| height | 300 | how high |\n\n")
		b.WriteString("> A quote, with a ~~struck~~ word.\n\n")
		b.WriteString("```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\n")
	}
	return b.String()
}

// BenchmarkParse parses a document of 2,000 lines.
func BenchmarkParse(b *testing.B) {
	src := largeDocument(2000)
	b.SetBytes(int64(len(src)))
	for b.Loop() {
		parse(src)
	}
}

// BenchmarkParseAndBuild renders a view of a new document of 300 and of 2,000 lines
// on each iteration, as the notes preview does after an edit: the parse, the build of
// the elements, their layout and the paint of a 900 by 700 window.
func BenchmarkParseAndBuild(b *testing.B) {
	for _, lines := range []int{300, 2000} {
		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) { parseAndBuild(b, lines) })
	}
}

// parseAndBuild is BenchmarkParseAndBuild for a document of about lines lines.
func parseAndBuild(b *testing.B, lines int) {
	docs := [2]string{largeDocument(lines), largeDocument(lines) + "\nOne more line.\n"}
	src := ui.NewSignal(docs[0])
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		return ui.Div().Size(ui.Full, ui.Full).ScrollY().Child(View(src.Get()))
	}, ui.DarkTheme(), nil)
	rt.SetViewport(900, 700, 1)
	now := time.Now()
	rt.Frame(now)
	i := 0
	for b.Loop() {
		i++
		src.Set(docs[i%2])
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
	}
}

// BenchmarkScroll is one frame of scrolling a 2,000-line document by 120 pixels, down
// through the first 12,000 pixels and back up: nothing is parsed or built, and only
// the blocks in view are painted. It reports the share of the frames that painted.
func BenchmarkScroll(b *testing.B) {
	src := largeDocument(2000)
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		return ui.Div().Size(ui.Full, ui.Full).ScrollY().Child(View(src))
	}, ui.DarkTheme(), nil)
	rt.SetViewport(900, 700, 1)
	now := time.Now()
	rt.Frame(now)
	step, painted, frames := 0, 0, 0
	for b.Loop() {
		dy := float32(120)
		if step++; (step/100)%2 == 1 {
			dy = -dy
		}
		rt.Scroll(450, 350, 0, dy, true, 0)
		now = now.Add(16 * time.Millisecond)
		rt.Frame(now)
		frames++
		if rt.Stats().Painted {
			painted++
		}
	}
	b.ReportMetric(float64(painted)/float64(frames), "painted/frame")
}

package mdcode

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tyler-builds/jamb-grammars/golang"

	"github.com/tyler-builds/jamb/markdown"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

const goCode = "package main\n\nfunc main() {\n\treturn\n}"

// The first request queues the block; once ready is called, the block's ranges are
// there, with "func" a keyword. An alias of the language finds the same grammar.
func TestHighlightAGoFence(t *testing.T) {
	h := Highlight(golang.Grammar)
	done := make(chan struct{}, 1)
	if _, ok := h.HighlightCode(context.Background(), "golang", goCode, func() { done <- struct{}{} }); ok {
		t.Fatal("the first request returned ranges at once")
	}
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("ready was not called")
	}
	spans, ok := h.HighlightCode(context.Background(), "Go", goCode, func() { t.Error("ready called for a block that is done") })
	if !ok {
		t.Fatal("no ranges after ready")
	}
	at := strings.Index(goCode, "func")
	keyword := false
	for _, sp := range spans {
		if sp.Start == at && sp.End == at+4 && strings.HasPrefix(sp.Capture, "keyword") {
			keyword = true
		}
		if sp.Start < 0 || sp.End > len(goCode) || sp.Start > sp.End {
			t.Errorf("span %+v is outside the code", sp)
		}
	}
	if !keyword {
		t.Errorf("no keyword span over func in %+v", spans)
	}
}

// A language without a grammar is plain, and nothing is queued for it.
func TestUnknownLanguageStaysPlain(t *testing.T) {
	h := Highlight(golang.Grammar)
	for _, lang := range []string{"", "rust", "text"} {
		if _, ok := h.HighlightCode(context.Background(), lang, "fn main() {}", func() { t.Errorf("ready called for %q", lang) }); ok {
			t.Errorf("%q has ranges", lang)
		}
	}
	if len(h.cache) != 0 {
		t.Errorf("%d blocks were queued", len(h.cache))
	}
}

// counting wraps a highlighter and counts the requests that returned ranges.
type counting struct {
	h      markdown.CodeHighlighter
	colors atomic.Int32
}

func (c *counting) HighlightCode(ctx context.Context, lang, code string, ready func()) ([]markdown.CodeSpan, bool) {
	spans, ok := c.h.HighlightCode(ctx, lang, code, ready)
	if ok {
		c.colors.Add(1)
	}
	return spans, ok
}

// A view with the highlighter renders again when the block's colors are ready, and
// colors it then.
func TestViewColorsTheBlockWhenReady(t *testing.T) {
	c := &counting{h: Highlight(golang.Grammar)}
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return markdown.View("```go\n" + goCode + "\n```\n").Code(c)
	})
	u.WaitFor("the colored block", func() bool { return c.colors.Load() > 0 })
}

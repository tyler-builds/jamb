package mdcode

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tyler-builds/jamb/markdown"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// TestMain keeps the compiled-module cache of the highlighter in a temporary
// directory, so that the tests do not write to the user's cache.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "jamb-mdcode-test-cache")
	if err != nil {
		panic(err)
	}
	for _, k := range []string{"LOCALAPPDATA", "XDG_CACHE_HOME", "HOME"} {
		os.Setenv(k, dir)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

const goCode = "package main\n\nfunc main() {\n\treturn\n}"

// A Go block has its ranges at once, with "func" a keyword, and an alias of the
// language, in another case, finds the same language.
func TestHighlightAGoFence(t *testing.T) {
	h := Highlight()
	for _, lang := range []string{"go", "golang", "Go"} {
		spans, ok := h.HighlightCode(context.Background(), lang, goCode, func() { t.Error("ready called") })
		if !ok {
			t.Fatalf("%s: no ranges", lang)
		}
		at := strings.Index(goCode, "func")
		keyword := false
		for _, sp := range spans {
			if sp.Start == at && sp.End == at+4 && strings.HasPrefix(sp.Capture, "keyword") {
				keyword = true
			}
			if sp.Start < 0 || sp.End > len(goCode) || sp.Start >= sp.End {
				t.Errorf("span %+v is outside the code", sp)
			}
		}
		if !keyword {
			t.Errorf("%s: no keyword span over func in %+v", lang, spans)
		}
	}
}

// Other languages work without setup.
func TestEveryLanguageWithoutSetup(t *testing.T) {
	h := Highlight()
	for lang, code := range map[string]string{
		"rust": "fn main() { let x = 1; }", "python3": "def f(): pass", "ts": "const x: number = 1",
		"yaml": "a: 1", "sh": "echo hi # c", "zig": "const x = 1;", "c#": "class A {}",
	} {
		if spans, ok := h.HighlightCode(context.Background(), lang, code, nil); !ok || len(spans) == 0 {
			t.Errorf("%s: ok %v, %d ranges", lang, ok, len(spans))
		}
	}
}

// A block without a fence, or in a language that the highlighter does not know, is
// plain, and nothing is kept for it.
func TestUnknownLanguageStaysPlain(t *testing.T) {
	h := Highlight()
	for _, lang := range []string{"", " ", "klingon"} {
		if _, ok := h.HighlightCode(context.Background(), lang, "fn main() {}", nil); ok {
			t.Errorf("%q has ranges", lang)
		}
	}
	if len(h.cache) != 0 {
		t.Errorf("%d blocks were kept", len(h.cache))
	}
}

// A block is lexed once, and the oldest blocks are dropped past maxCached.
func TestCache(t *testing.T) {
	h := Highlight()
	h.HighlightCode(context.Background(), "go", goCode, nil)
	h.HighlightCode(context.Background(), "go", goCode, nil)
	if h.lexed != 1 {
		t.Fatalf("one block was lexed %d times", h.lexed)
	}
	for i := range maxCached + 10 {
		h.HighlightCode(context.Background(), "go", fmt.Sprintf("var x = %d", i), nil)
	}
	if len(h.cache) != maxCached || len(h.order) != maxCached {
		t.Fatalf("the cache holds %d blocks (%d in order), want %d", len(h.cache), len(h.order), maxCached)
	}
	if _, ok := h.cache["go\x00"+goCode]; ok {
		t.Fatal("the oldest block was not dropped")
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

// A view with the highlighter colors its block.
func TestViewColorsTheBlock(t *testing.T) {
	c := &counting{h: Highlight()}
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return markdown.View("```go\n" + goCode + "\n```\n").Code(c)
	})
	u.WaitFor("the colored block", func() bool { return c.colors.Load() > 0 })
}

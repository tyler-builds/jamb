// Package mdcode colors the code blocks of a Markdown view with the highlighter of
// jamb's code editor:
//
//	markdown.View(readme).Code(mdcode.Highlight())
//
// A code block is colored when the first word of its fence names a language that the
// highlighter knows, in any case: one of the 76 languages of @pierre/highlights, by its
// name or a common alias, such as go, golang, python, py, ts, rust, rs, sh or yml (see
// highlight.Languages). Blocks in other languages, and blocks without a fence, stay
// plain.
//
// The package is separate from markdown so that only the apps that import it link the
// highlighter: its WebAssembly module (about 170 KB), the runtime that runs it
// (wazero), and the crypto packages that wazero imports.
//
// A block is lexed the first time a view shows it, while the view renders: a block of
// a few kilobytes takes some microseconds, and a megabyte about 10 ms. Its colors are
// kept for later renders, so a view that renders again, or a preview that shows a new
// text with the same blocks, lexes nothing again. The first block of all also compiles
// the highlighter's module, once per process (the code editor keeps the compiled code
// in a cache directory, so a later run is faster).
package mdcode

import (
	"context"
	"strings"
	"sync"

	"github.com/tyler-builds/jamb/codeedit/highlight"
	"github.com/tyler-builds/jamb/markdown"
)

// maxCached is how many code blocks a highlighter keeps the colors of. Past it the
// oldest are dropped, and lexed again if they are shown again.
const maxCached = 1024

// Highlighter is a markdown.CodeHighlighter that colors code with the code editor's
// highlighter. Make one with Highlight and give it to markdown.ViewElement.Code; one
// highlighter can serve every view of an app. It is safe for concurrent use.
type Highlighter struct {
	mu    sync.Mutex
	cache map[string][]markdown.CodeSpan // by language and code
	order []string                       // the keys of cache, oldest first
	lexed int                            // the blocks lexed, which the tests read
}

// Highlight returns a highlighter for every language that the code editor's
// highlighter knows.
func Highlight() *Highlighter {
	return &Highlighter{cache: map[string][]markdown.CodeSpan{}}
}

// HighlightCode returns the colored ranges of code in the language lang. It lexes the
// code at once, so it never calls ready; it returns false for a language that the
// highlighter does not know. See markdown.CodeHighlighter.
func (h *Highlighter) HighlightCode(_ context.Context, lang, code string, _ func()) ([]markdown.CodeSpan, bool) {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" || !highlight.Supported(lang) {
		return nil, false
	}
	key := lang + "\x00" + code
	h.mu.Lock()
	spans, ok := h.cache[key]
	h.mu.Unlock()
	if ok {
		return spans, spans != nil
	}
	spans = spansOf(lang, code)
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.cache[key]; !ok {
		h.cache[key] = spans
		h.order = append(h.order, key)
		h.lexed++
		for len(h.order) > maxCached {
			delete(h.cache, h.order[0])
			h.order = h.order[1:]
		}
	}
	return spans, spans != nil
}

// spansOf lexes code in the language lang and returns its colored ranges, or nil
// when the highlighter failed, which leaves the block plain.
func spansOf(lang, code string) []markdown.CodeSpan {
	raw, err := highlight.Highlight(lang, []byte(code))
	if err != nil {
		return nil
	}
	names := highlight.CaptureNames()
	spans := make([]markdown.CodeSpan, 0, len(raw))
	for _, sp := range raw {
		if sp.Capture > 0 && sp.Capture < len(names) {
			spans = append(spans, markdown.CodeSpan{Start: sp.Start, End: sp.End, Capture: names[sp.Capture]})
		}
	}
	return spans
}

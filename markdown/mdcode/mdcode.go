// Package mdcode colors the code blocks of a Markdown view with the tree-sitter
// grammars of jamb's code editor:
//
//	h := mdcode.Highlight(golang.Grammar, rust.Grammar, python.Grammar)
//	markdown.View(readme).Code(h)
//
// A code block is colored when the first word of its fence names one of the
// grammars: by the grammar's name (such as "go" or "python"), in any case, or by a
// common alias (see Highlight). Blocks in other languages, and blocks without a
// fence, stay plain.
//
// The package is separate from markdown so that only the apps that import it link
// the code editor's highlighting: the WebAssembly runtime (wazero) that runs the
// grammars, and the crypto packages that wazero imports. The grammars come from the
// packages of github.com/tyler-builds/jamb-grammars, each of which embeds its module
// in the app.
//
// Each block is parsed once for each language and code, the first time a view shows
// it, and its colors are kept for later renders, so a view that renders again, or a
// preview that shows a new text with the same blocks, parses nothing again. A block is
// plain for the moment its parse takes. The parses run on at most two goroutines of
// the highlighter at a time, each with one highlight session, which is closed as soon
// as its block is done; the goroutines end when there is nothing left to parse. A
// block that no view shows any more (the view went away, or its text changed) is
// dropped from the queue, or its parse is stopped. The first block of a language also
// compiles that grammar's module, once per process (the code editor keeps the compiled
// code in a cache directory, so a later run is faster).
package mdcode

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/tyler-builds/jamb/codeedit/highlight"
	"github.com/tyler-builds/jamb/markdown"
)

// aliases are the other names of the languages, as fences write them, by the name of
// the grammar they mean.
var aliases = map[string]string{
	"golang": "go",
	"js":     "javascript", "jsx": "javascript", "mjs": "javascript", "cjs": "javascript", "node": "javascript",
	"ts": "typescript", "mts": "typescript", "cts": "typescript",
	"py": "python", "python3": "python",
	"sh": "bash", "shell": "bash", "zsh": "bash", "console": "bash",
	"rs":  "rust",
	"c++": "cpp", "cc": "cpp", "cxx": "cpp", "hpp": "cpp", "h++": "cpp",
	"h":  "c",
	"cs": "csharp", "c#": "csharp",
	"yml": "yaml",
	"md":  "markdown",
	"htm": "html", "xhtml": "html",
	"jsonc": "json",
}

// maxCached is how many code blocks a highlighter keeps the colors of. Past it the
// oldest are dropped, and parsed again if they are shown again.
const maxCached = 1024

// maxWorkers is how many blocks a highlighter parses at once, each in a highlight
// session of its own (a WebAssembly instance and its goroutine).
const maxWorkers = 2

// timeout is how long a block's parse may take before the block stays plain.
const timeout = 10 * time.Second

// errStopped is the error of a parse that no view wants any more.
var errStopped = errors.New("mdcode: no view shows the block any more")

// Highlighter is a markdown.CodeHighlighter that parses code with tree-sitter
// grammars. Make one with Highlight and give it to markdown.ViewElement.Code; one
// highlighter can serve every view of an app. It is safe for concurrent use.
type Highlighter struct {
	grammars map[string]highlight.Grammar

	mu      sync.Mutex
	cache   map[string]*entry // by language and code: done, queued or being parsed
	order   []string          // the keys of cache, oldest first
	queue   []*entry          // the blocks waiting for a worker, oldest first
	workers int               // the goroutines parsing blocks
	// What the tests read: the sessions opened, those open now, and the most open at once.
	opened, open, peak int
}

// entry is one block: its colors once done, else the views waiting for them.
type entry struct {
	key, code string
	g         highlight.Grammar
	done      bool
	spans     []markdown.CodeSpan
	waiting   []func()
	stops     []func() bool // stop the context.AfterFunc of each request
	live      int           // the requests whose context is not done
	stopped   bool          // no request wants the block any more; stop is closed
	stop      chan struct{}
}

// Highlight returns a highlighter for the languages of the grammars. A fence names a
// grammar by its name, in any case, or by one of these aliases: golang for go; js,
// jsx, mjs, cjs and node for javascript; ts, mts and cts for typescript; py and
// python3 for python; sh, shell, zsh and console for bash; rs for rust; c++, cc, cxx,
// hpp and h++ for cpp; h for c; cs and c# for csharp; yml for yaml; md for markdown;
// htm and xhtml for html; and jsonc for json. An alias works only when its grammar is
// given.
func Highlight(grammars ...highlight.Grammar) *Highlighter {
	h := &Highlighter{grammars: map[string]highlight.Grammar{}, cache: map[string]*entry{}}
	for _, g := range grammars {
		if g != nil {
			h.grammars[strings.ToLower(g.Name())] = g
		}
	}
	return h
}

// grammar returns the grammar that the fence's language lang names, or nil.
func (h *Highlighter) grammar(lang string) highlight.Grammar {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if g := h.grammars[lang]; g != nil {
		return g
	}
	return h.grammars[aliases[lang]]
}

// HighlightCode returns the colored ranges of code in the language lang, once they
// are parsed. The first time it is asked for a block, it queues the block and returns
// false, and it calls ready when the colors are there, unless ctx is done by then:
// when the contexts of all the requests for a block are done, the block is dropped.
// It returns false for good for a language that it has no grammar for. See
// markdown.CodeHighlighter.
func (h *Highlighter) HighlightCode(ctx context.Context, lang, code string, ready func()) ([]markdown.CodeSpan, bool) {
	g := h.grammar(lang)
	if g == nil || ctx.Err() != nil {
		return nil, false
	}
	key := strings.ToLower(g.Name()) + "\x00" + code
	h.mu.Lock()
	defer h.mu.Unlock()
	e := h.cache[key]
	if e != nil && e.done {
		return e.spans, e.spans != nil
	}
	if e == nil {
		e = &entry{key: key, code: code, g: g, stop: make(chan struct{})}
		h.cache[key] = e
		h.order = append(h.order, key)
		h.evict()
		h.queue = append(h.queue, e)
		if h.workers < maxWorkers {
			h.workers++
			go h.work()
		}
	}
	e.waiting = append(e.waiting, ready)
	e.live++
	e.stops = append(e.stops, context.AfterFunc(ctx, func() { h.release(e) }))
	return nil, false
}

// evict drops the oldest blocks that are done while the cache holds more than
// maxCached. h.mu is held.
func (h *Highlighter) evict() {
	for len(h.order) > maxCached {
		if old := h.cache[h.order[0]]; old != nil && old.done {
			delete(h.cache, h.order[0])
		}
		h.order = h.order[1:]
	}
}

// release ends one request for e, whose context is done. When no request wants e any
// more, it is dropped: a worker skips it in the queue, or stops its parse.
func (h *Highlighter) release(e *entry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if e.done || e.stopped {
		return
	}
	if e.live--; e.live > 0 {
		return
	}
	e.stopped = true
	close(e.stop)
	if h.cache[e.key] == e {
		delete(h.cache, e.key)
	}
}

// work parses the queued blocks, one at a time, and ends when the queue is empty.
func (h *Highlighter) work() {
	for {
		h.mu.Lock()
		var e *entry
		for e == nil && len(h.queue) > 0 {
			e, h.queue = h.queue[0], h.queue[1:]
			if e.stopped {
				e = nil
			}
		}
		if e == nil {
			h.workers--
			h.mu.Unlock()
			return
		}
		h.opened++
		h.open++
		h.peak = max(h.peak, h.open)
		h.mu.Unlock()

		spans, err := spansOf(e.g, e.code, e.stop)

		h.mu.Lock()
		h.open--
		if e.stopped {
			h.mu.Unlock()
			continue
		}
		if err != nil {
			spans = nil // the block stays plain
		}
		e.done, e.spans = true, spans
		waiting, stops := e.waiting, e.stops
		e.waiting, e.stops = nil, nil
		h.mu.Unlock()
		for _, stop := range stops {
			stop()
		}
		for _, ready := range waiting {
			ready()
		}
	}
}

// spansOf parses code with the grammar g and returns its colored ranges. It runs a
// highlight session for the code alone, and closes it before it returns: when the
// ranges are there, on an error, on the timeout, or when stop is closed. It sets the
// session's view to the whole code before an empty edit, so the first spans that
// reflect the edit cover all of it.
func spansOf(g highlight.Grammar, code string, stop <-chan struct{}) ([]markdown.CodeSpan, error) {
	notify := make(chan struct{}, 1)
	s, err := highlight.NewSession(g, []byte(code), highlight.Options{Notify: func() {
		select {
		case notify <- struct{}{}:
		default:
		}
	}})
	if err != nil {
		return nil, err
	}
	defer s.Close()
	if err := s.SetViewport(0, len(code)); err != nil {
		return nil, err
	}
	if err := s.Edit(highlight.EditRecord{Start: len(code), OldEnd: len(code)}); err != nil {
		return nil, err
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for s.Synced() < 1 {
		if err := s.Err(); err != nil {
			return nil, err
		}
		select {
		case <-notify:
		case <-stop:
			return nil, errStopped
		case <-deadline.C:
			return nil, errors.New("mdcode: the code took too long to parse")
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	raw, _ := s.Spans()
	names := s.CaptureNames()
	spans := make([]markdown.CodeSpan, 0, len(raw))
	for _, sp := range raw {
		if sp.Capture >= 0 && sp.Capture < len(names) {
			spans = append(spans, markdown.CodeSpan{Start: sp.Start, End: sp.End, Capture: names[sp.Capture]})
		}
	}
	return spans, nil
}

package mdcode

import (
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tyler-builds/jamb-grammars/golang"

	"github.com/tyler-builds/jamb/markdown"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// blocks returns a document of n Go code blocks, each different, numbered from first.
func blocks(first, n int) string {
	var b strings.Builder
	for i := first; i < first+n; i++ {
		fmt.Fprintf(&b, "```go\nfunc f%d() int { return %d }\n```\n\n", i, i)
	}
	return b.String()
}

// stats returns the sessions h opened, those open now, the most open at once, the
// blocks not done yet, and the goroutines parsing.
func (h *Highlighter) stats() (opened, open, peak, pending, workers int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, e := range h.cache {
		if !e.done {
			pending++
		}
	}
	return h.opened, h.open, h.peak, pending, h.workers
}

// sampleGoroutines records the most goroutines seen until stop is called, and returns
// a function that stops it and returns that number.
func sampleGoroutines() func() int {
	var most atomic.Int64
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			if n := int64(runtime.NumGoroutine()); n > most.Load() {
				most.Store(n)
			}
			select {
			case <-done:
				return
			case <-time.After(200 * time.Microsecond):
			}
		}
	}()
	return func() int {
		close(done)
		<-finished
		return int(most.Load())
	}
}

// waitGoroutines waits until there are at most n goroutines, and reports whether
// there are.
func waitGoroutines(n int) bool {
	for range 500 {
		if runtime.NumGoroutine() <= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// A document with 200 code blocks parses them two at a time: at most two sessions
// are open at once, the goroutines stay few, and when the blocks are done every
// session is closed and the highlighter's goroutines have ended.
func TestManyBlocksUseAFewSessions(t *testing.T) {
	h := Highlight(golang.Grammar)
	src := blocks(0, 200)
	base := runtime.NumGoroutine()
	most := sampleGoroutines()
	u := uitest.New(t, func(ctx *ui.Context) ui.Element { return markdown.View(src).Code(h) })
	u.WaitFor("every block to be colored", func() bool {
		opened, _, _, pending, _ := h.stats()
		return opened == 200 && pending == 0
	})
	peakGoroutines := most()
	opened, open, peak, _, _ := h.stats()
	t.Logf("200 blocks: %d sessions opened, at most %d open at once; at most %d goroutines, %d before the view",
		opened, peak, peakGoroutines, base)
	if peak > maxWorkers || open != 0 {
		t.Errorf("%d sessions open at once and %d still open, want at most %d and none", peak, open, maxWorkers)
	}
	// The sampler, the two workers and their two sessions, and the runtime's own.
	if peakGoroutines > base+10 {
		t.Errorf("up to %d goroutines while 200 blocks were parsed, %d before", peakGoroutines, base)
	}
	if !waitGoroutines(base) {
		t.Errorf("%d goroutines after the blocks were done, %d before", runtime.NumGoroutine(), base)
	}
	if _, _, _, _, workers := h.stats(); workers != 0 {
		t.Errorf("%d workers are still running", workers)
	}
}

// A block is parsed once: a render of the same text, and a new text that keeps the
// block, use the colors that are kept.
func TestBlocksAreParsedOnce(t *testing.T) {
	h := Highlight(golang.Grammar)
	src := ui.NewSignal(blocks(0, 3))
	u := uitest.New(t, func(ctx *ui.Context) ui.Element { return markdown.View(src.Get()).Code(h) })
	u.WaitFor("the blocks to be colored", func() bool {
		opened, _, _, pending, _ := h.stats()
		return opened == 3 && pending == 0
	})
	src.Set("A new paragraph.\n\n" + blocks(0, 3))
	u.Frame()
	src.Set(blocks(0, 3) + "One more line.\n")
	u.Frame()
	u.Advance(100 * time.Millisecond)
	if opened, _, _, pending, _ := h.stats(); opened != 3 || pending != 0 {
		t.Errorf("%d sessions opened and %d blocks pending after new texts with the same blocks, want 3 and 0", opened, pending)
	}
}

// When the view goes away, the blocks it was waiting for are dropped: the workers
// skip them, the sessions are closed, and the goroutines end.
func TestBlocksOfAViewThatWentAwayAreDropped(t *testing.T) {
	h := Highlight(golang.Grammar)
	src := blocks(0, 200)
	shown := ui.NewSignal(true)
	base := runtime.NumGoroutine()
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Child(ui.If(shown.Get(), markdown.View(src).Code(h)))
	})
	shown.Set(false)
	u.Frame()
	u.WaitFor("the workers to stop", func() bool {
		_, open, _, pending, workers := h.stats()
		return open == 0 && pending == 0 && workers == 0
	})
	opened, _, _, _, _ := h.stats()
	t.Logf("the view went away: %d of 200 blocks were parsed", opened)
	if opened >= 50 {
		t.Errorf("%d of 200 blocks were parsed after the view went away", opened)
	}
	if !waitGoroutines(base) {
		t.Errorf("%d goroutines after the view went away, %d before", runtime.NumGoroutine(), base)
	}
}

// When the text changes, the blocks only the old text had are dropped, and those of
// the new text are parsed.
func TestBlocksOfAnOldTextAreDropped(t *testing.T) {
	h := Highlight(golang.Grammar)
	src := ui.NewSignal(blocks(0, 100))
	u := uitest.New(t, func(ctx *ui.Context) ui.Element { return markdown.View(src.Get()).Code(h) })
	src.Set(blocks(100, 100))
	u.Frame()
	u.WaitFor("the new blocks to be colored", func() bool {
		_, open, _, pending, workers := h.stats()
		return open == 0 && pending == 0 && workers == 0
	})
	opened, _, _, _, _ := h.stats()
	h.mu.Lock()
	kept := 0
	for _, e := range h.cache {
		if strings.Contains(e.code, "func f1") && e.done {
			kept++
		}
	}
	h.mu.Unlock()
	t.Logf("the text changed: %d sessions for 100 old and 100 new blocks", opened)
	if opened >= 150 {
		t.Errorf("%d sessions opened, want the old blocks dropped", opened)
	}
	if kept < 100 {
		t.Errorf("%d blocks of the new text are colored, want all 100", kept)
	}
}

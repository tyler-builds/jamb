package codeedit

import (
	"os"
	"testing"
	"time"

	"github.com/tyler-builds/jamb-grammars/golang"
)

// TestMain keeps the compiled-module cache of the highlighter in a temporary directory, so that
// the tests do not write to the user's cache.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "jamb-codeedit-test-cache")
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

// TestLanguageHighlightsWithRealSession checks the wiring of a tree-sitter session: the spans
// arrive in the background and are painted, and a keystroke reaches the session at once.
func TestLanguageHighlightsWithRealSession(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles the Go grammar")
	}
	h := newHarness(t, "package main\n\nfunc main() {}\n", func(e *Editor) *Editor {
		return e.Language(golang.Grammar)
	})
	deadline := time.Now().Add(30 * time.Second)
	for {
		h.frame()
		if h.ed.view.hl != nil {
			if spans, _ := h.ed.view.hl.Spans(); len(spans) > 0 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("the session published no spans")
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.doc.SetCaret(h.doc.Len())
	h.typeText("x")
	if err := h.ed.view.session.Err(); err != nil {
		t.Fatalf("session error after an edit: %v", err)
	}
	if got := h.doc.Text(); got != "package main\n\nfunc main() {}\nx" {
		t.Fatalf("text after typing = %q", got)
	}
}

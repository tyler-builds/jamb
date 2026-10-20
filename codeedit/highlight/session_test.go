package highlight_test

import (
	"bytes"
	"errors"
	"math/rand"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/codeedit/highlight"
)

// TestMain keeps the tests' compiled-module cache in a temporary directory, so
// that they do not write to the user's cache.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "jamb-test-cache")
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

// open starts a session on text with the whole text in view, and waits for its
// first spans.
func open(t *testing.T, lang string, text []byte) *highlight.Session {
	t.Helper()
	notify := make(chan struct{}, 1)
	s, err := highlight.NewSession(lang, text, highlight.Options{Notify: func() {
		select {
		case notify <- struct{}{}:
		default:
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.SetViewport(0, len(text)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-notify:
	case <-time.After(30 * time.Second):
		t.Fatal("no spans published")
	}
	return s
}

// wantCapture is a highlighted text and the capture it should carry.
type wantCapture struct{ text, capture string }

var captureTests = []struct {
	lang string
	text string
	want []wantCapture
}{
	{"go",
		"package main\n\n// Hi is a comment.\nfunc main() {\n\tx := \"s\" + 1\n\tprintln(x, true)\n}\n",
		[]wantCapture{
			{"package", "keyword.declaration"},
			{"// Hi is a comment.", "comment"},
			{"func", "keyword.declaration"},
			{"main", "function.definition"},
			{"\"s\"", "string"},
			{"1", "number"},
			{"true", "boolean"},
		}},
	{"typescript",
		"const n: number = 42; // note\nfunction greet(name: string) { return \"hi \" + name; }\n",
		[]wantCapture{
			{"const", "keyword.declaration"},
			{"number", "type.builtin"},
			{"42", "number"},
			{"// note", "comment"},
			{"function", "keyword.declaration"},
			{"greet", "function"},
			{"name", "variable.parameter"},
			{"\"hi \"", "string"},
		}},
	{"py",
		"def add(a, b):\n    # sum\n    return a + 1 if True else None\n",
		[]wantCapture{
			{"def", "keyword.declaration"},
			{"# sum", "comment"},
			{"return", "keyword.control"},
			{"1", "number"},
			{"None", "constant.builtin"},
		}},
	{"CSS",
		".app #id a:hover { color: red; }\n",
		[]wantCapture{
			{".app", "selector.class"},
			{"#id", "selector.id"},
			{"color", "property"},
		}},
}

// hasCapture reports whether a span covers exactly text and carries capture.
func hasCapture(src []byte, spans []highlight.Span, names []string, w wantCapture) bool {
	for _, sp := range spans {
		if string(src[sp.Start:sp.End]) == w.text && names[sp.Capture] == w.capture {
			return true
		}
	}
	return false
}

func TestSpansMatchExpectedCaptures(t *testing.T) {
	for _, tt := range captureTests {
		t.Run(tt.lang, func(t *testing.T) {
			text := []byte(tt.text)
			s := open(t, tt.lang, text)
			spans, _ := s.Spans()
			for _, w := range tt.want {
				if !hasCapture(text, spans, s.CaptureNames(), w) {
					t.Errorf("no %q capture on %q", w.capture, w.text)
				}
			}
		})
	}
}

func TestHighlight(t *testing.T) {
	for _, tt := range captureTests {
		t.Run(tt.lang, func(t *testing.T) {
			text := []byte(tt.text)
			spans, err := highlight.Highlight(tt.lang, text)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !hasCapture(text, spans, highlight.CaptureNames(), w) {
					t.Errorf("no %q capture on %q", w.capture, w.text)
				}
			}
			// A session on the same text publishes the same spans.
			s := open(t, tt.lang, text)
			if got, _ := s.Spans(); !slices.Equal(got, spans) {
				t.Errorf("the session's spans differ from Highlight's:\n session %v\nHighlight %v", got, spans)
			}
		})
	}
}

func TestHighlightConcurrent(t *testing.T) {
	text := bytes.Repeat([]byte("func f() { return \"x\" }\n"), 50)
	want, err := highlight.Highlight("go", text)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				got, err := highlight.Highlight("go", text)
				if err != nil {
					t.Error(err)
					return
				}
				if !slices.Equal(got, want) {
					t.Error("concurrent Highlight calls gave different spans")
					return
				}
			}
		})
	}
	wg.Wait()
}

func TestLanguages(t *testing.T) {
	names := highlight.Languages()
	if !slices.IsSorted(names) || len(names) < 150 {
		t.Fatalf("Languages: %d names, sorted %v", len(names), slices.IsSorted(names))
	}
	for _, n := range []string{"go", "golang", "rust", "python", "python3", "ts", "tsx", "node", "console", "jsonc", "yml", "md", "c#", "plain"} {
		if !slices.Contains(names, n) || !highlight.Supported(n) {
			t.Errorf("%q is not a supported language", n)
		}
	}
	if !highlight.Supported(" Go ") {
		t.Error("language names are not matched in any case")
	}
	for _, n := range names {
		s, err := highlight.NewSession(n, []byte("x = 1 // y\n"), highlight.Options{})
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		s.Close()
	}
}

func TestUnknownLanguage(t *testing.T) {
	if _, err := highlight.NewSession("klingon", []byte("x"), highlight.Options{}); !errors.Is(err, highlight.ErrUnknownLanguage) {
		t.Fatalf("NewSession(klingon): err = %v, want ErrUnknownLanguage", err)
	}
	if _, err := highlight.NewSession("", []byte("x"), highlight.Options{}); !errors.Is(err, highlight.ErrUnknownLanguage) {
		t.Fatalf("NewSession(\"\"): err = %v, want ErrUnknownLanguage", err)
	}
	if _, err := highlight.Highlight("klingon", []byte("x")); !errors.Is(err, highlight.ErrUnknownLanguage) {
		t.Fatalf("Highlight(klingon): err = %v, want ErrUnknownLanguage", err)
	}
	if highlight.Supported("klingon") {
		t.Fatal("klingon is supported")
	}
}

func TestSizeLimit(t *testing.T) {
	text := bytes.Repeat([]byte("a"), 101)
	if _, err := highlight.NewSession("go", text, highlight.Options{MaxBytes: 100}); !errors.Is(err, highlight.ErrTooLarge) {
		t.Fatalf("101 bytes with MaxBytes 100: err = %v, want ErrTooLarge", err)
	}
	big := make([]byte, highlight.DefaultMaxBytes+1)
	if _, err := highlight.NewSession("go", big, highlight.Options{}); !errors.Is(err, highlight.ErrTooLarge) {
		t.Fatalf("over DefaultMaxBytes: err = %v, want ErrTooLarge", err)
	}
	s := open(t, "go", text[:100])
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestEditValidationAndClose(t *testing.T) {
	text := []byte("package p\n")
	s := open(t, "go", text)
	if err := s.Edit(highlight.EditRecord{Start: 5, OldEnd: 50}); err == nil {
		t.Fatal("Edit accepted a range past the end of the document")
	}
	if err := s.Edit(highlight.EditRecord{Start: 3, OldEnd: 1}); err == nil {
		t.Fatal("Edit accepted a reversed range")
	}
	if err := s.Edit(highlight.EditRecord{Start: 0, OldEnd: 0, Text: []byte("// c\n")}); err != nil {
		t.Fatal(err)
	}
	// Only the valid edit counts: the published spans come to reflect one edit.
	for deadline := time.Now().Add(5 * time.Second); s.Synced() < 1 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	if n := s.Synced(); n != 1 {
		t.Fatalf("the spans reflect %d edits, want 1: rejected edits were counted", n)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal("second Close:", err)
	}
	if err := s.Edit(highlight.EditRecord{}); !errors.Is(err, highlight.ErrClosed) {
		t.Fatalf("Edit after Close: err = %v, want ErrClosed", err)
	}
	if err := s.SetViewport(0, 1); !errors.Is(err, highlight.ErrClosed) {
		t.Fatalf("SetViewport after Close: err = %v, want ErrClosed", err)
	}
}

// TestConcurrentUse runs a typing goroutine, a viewport goroutine and readers at
// once. Run it with -race.
func TestConcurrentUse(t *testing.T) {
	text := bytes.Repeat([]byte("func f() { return 1 }\n"), 200)
	var notifies atomic.Int64
	s, err := highlight.NewSession("go", text, highlight.Options{Notify: func() { notifies.Add(1) }})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { // the typing thread
		defer wg.Done()
		rng := rand.New(rand.NewSource(3))
		for range 400 {
			at := rng.Intn(len(text))
			if err := s.Edit(highlight.EditRecord{Start: at, OldEnd: at, Text: []byte("x")}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() { // scrolling
		defer wg.Done()
		for i := range 200 {
			if err := s.SetViewport(i*10, i*10+500); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() { // painting
		defer wg.Done()
		for range 2000 {
			s.Spans()
			s.Err()
		}
	}()
	wg.Wait()
	deadline := time.Now().Add(30 * time.Second)
	for notifies.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	if notifies.Load() == 0 {
		t.Fatal("no notification")
	}
}

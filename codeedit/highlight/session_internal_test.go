package highlight

import (
	"math/rand"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tyler-builds/jamb-grammars/golang"
	"github.com/tyler-builds/jamb-grammars/python"
	"github.com/tyler-builds/jamb-grammars/typescript"
)

// testLanguage returns the grammar of the jamb-grammars package pkg whose module
// is file.
func testLanguage(t *testing.T, pkg, file string) Grammar {
	t.Helper()
	switch pkg + "/" + file {
	case "golang/go":
		return golang.Grammar
	case "typescript/typescript":
		return typescript.TypeScript
	case "python/python":
		return python.Grammar
	}
	t.Fatalf("no test grammar %s/%s", pkg, file)
	return nil
}

// openTest opens a session on text, with a notification channel for each
// publish, and sets the viewport to the whole text.
func openTest(t *testing.T, lang Grammar, text []byte, notify chan struct{}) *Session {
	t.Helper()
	s, err := NewSession(lang, text, Options{Notify: func() {
		select {
		case notify <- struct{}{}:
		default:
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetViewport(0, len(text)); err != nil {
		t.Fatal(err)
	}
	return s
}

// settle waits until the worker has published spans for every change made so
// far: no round is in progress, nothing is queued, the viewport is applied,
// and the published spans are current for the latest version.
func settle(t *testing.T, s *Session, notify <-chan struct{}) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for {
		s.mu.Lock()
		done := !s.busy && len(s.queue) == 0 && !s.viewDirty && !s.first && !s.refresh && s.synced == s.version
		err := s.err
		s.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if done {
			return
		}
		select {
		case <-notify:
		case <-deadline:
			t.Fatal("worker did not catch up")
		}
	}
}

// freshSpans returns the spans of a new session on text, parsed from scratch.
func freshSpans(t *testing.T, lang Grammar, text []byte) []Span {
	t.Helper()
	notify := make(chan struct{}, 1)
	s := openTest(t, lang, text, notify)
	defer s.Close()
	settle(t, s, notify)
	sp, _ := s.Spans()
	return sp
}

// holdWorker makes the worker wait before each round while hold is set. The
// returned release function clears hold and lets a waiting round continue.
func holdWorker(t *testing.T) (hold *atomic.Bool, release func()) {
	t.Helper()
	gate := make(chan struct{})
	hold = new(atomic.Bool)
	workHook = func() {
		if hold.Load() {
			<-gate
		}
	}
	t.Cleanup(func() { workHook = nil })
	var once sync.Once
	return hold, func() {
		once.Do(func() {
			hold.Store(false)
			close(gate)
		})
	}
}

func TestEditShiftsBeforeWorkerAnswers(t *testing.T) {
	lang := testLanguage(t, "golang", "go")
	text := []byte("package main\n\nfunc main() {\n\tprintln(1)\n}\n")
	hold, release := holdWorker(t)
	notify := make(chan struct{}, 100)
	s := openTest(t, lang, text, notify)
	defer s.Close()
	defer release()
	settle(t, s, notify)
	before, v0 := s.Spans()

	hold.Store(true)
	e := EditRecord{Start: 31, OldEnd: 31, Text: []byte("xy")}
	if err := s.Edit(e); err != nil {
		t.Fatal(err)
	}
	after, v1 := s.Spans()
	if v1 != v0+1 {
		t.Fatalf("version = %d, want %d", v1, v0+1)
	}
	if want := shift(before, e); !slices.Equal(after, want) {
		t.Fatalf("spans after edit = %v, want the shifted spans %v", after, want)
	}
	release()
	settle(t, s, notify)

	final := append(slices.Clone(text[:31]), append([]byte("xy"), text[31:]...)...)
	got, _ := s.Spans()
	if want := freshSpans(t, lang, final); !slices.Equal(got, want) {
		t.Fatalf("spans after the worker answered differ from a fresh parse:\n got %v\nwant %v", got, want)
	}
}

func TestBurstIsCoalescedIntoOneParse(t *testing.T) {
	lang := testLanguage(t, "python", "python")
	text := []byte("def f(a, b):\n    return a + b\n")
	hold, release := holdWorker(t)
	notify := make(chan struct{}, 1000)
	s := openTest(t, lang, text, notify)
	defer s.Close()
	defer release()
	settle(t, s, notify)

	before := s.reparses.Load()
	hold.Store(true)
	for range 200 {
		if err := s.Edit(EditRecord{Start: 0, OldEnd: 0, Text: []byte("#")}); err != nil {
			t.Fatal(err)
		}
	}
	// The text is 200 comment markers followed by the function.
	final := append(slices.Repeat([]byte("#"), 200), text...)
	if err := s.SetViewport(0, len(final)); err != nil {
		t.Fatal(err)
	}
	release()
	settle(t, s, notify)
	if n := s.reparses.Load() - before; n != 1 {
		t.Fatalf("200 queued edits caused %d parses, want 1", n)
	}
	got, _ := s.Spans()
	if want := freshSpans(t, lang, final); !slices.Equal(got, want) {
		t.Fatalf("spans after the burst differ from a fresh parse")
	}
}

// editSample is text with characters that the grammars treat differently, for
// random edits.
const editSample = "xyz _(){}[]\n\"'=;:.,+-*/<>!&|1\t#"

// randomEdit returns a random edit of text, inserting or deleting a few bytes.
func randomEdit(rng *rand.Rand, text []byte) EditRecord {
	if len(text) == 0 || rng.Intn(2) == 0 {
		at := rng.Intn(len(text) + 1)
		ins := make([]byte, 1+rng.Intn(6))
		for i := range ins {
			ins[i] = editSample[rng.Intn(len(editSample))]
		}
		return EditRecord{Start: at, OldEnd: at, Text: ins}
	}
	at := rng.Intn(len(text))
	n := min(1+rng.Intn(8), len(text)-at)
	return EditRecord{Start: at, OldEnd: at + n}
}

// apply returns text with e applied.
func apply(text []byte, e EditRecord) []byte {
	out := slices.Clone(text[:e.Start])
	out = append(out, e.Text...)
	return append(out, text[e.OldEnd:]...)
}

func TestSpansMatchFreshParseAfterRandomEdits(t *testing.T) {
	samples := []struct {
		name string
		lang Grammar
		text []byte
	}{
		{"go", testLanguage(t, "golang", "go"), mustRead(t, "session.go")},
		{"typescript", testLanguage(t, "typescript", "typescript"), []byte(tsSample)},
		{"python", testLanguage(t, "python", "python"), []byte(pySample)},
	}
	for _, sm := range samples {
		t.Run(sm.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(7))
			text := slices.Clone(sm.text)
			notify := make(chan struct{}, 1000)
			s := openTest(t, sm.lang, text, notify)
			defer s.Close()
			settle(t, s, notify)
			for batch := range 15 {
				for range 10 {
					e := randomEdit(rng, text)
					if err := s.Edit(e); err != nil {
						t.Fatal(err)
					}
					text = apply(text, e)
				}
				if err := s.SetViewport(0, len(text)); err != nil {
					t.Fatal(err)
				}
				settle(t, s, notify)
				got, _ := s.Spans()
				if want := freshSpans(t, sm.lang, text); !slices.Equal(got, want) {
					t.Fatalf("batch %d: spans differ from a fresh parse of %d bytes", batch, len(text))
				}
			}
		})
	}
}

func TestShiftSpans(t *testing.T) {
	spans := []Span{{Start: 3, End: 6, Capture: 1}}
	tests := []struct {
		name string
		e    EditRecord
		want []Span
	}{
		{"insert before", EditRecord{Start: 0, OldEnd: 0, Text: []byte("ab")}, []Span{{5, 8, 1}}},
		{"insert after", EditRecord{Start: 6, OldEnd: 6, Text: []byte("ab")}, []Span{{3, 6, 1}}},
		{"insert at start", EditRecord{Start: 3, OldEnd: 3, Text: []byte("ab")}, []Span{{3, 8, 1}}},
		{"delete inside", EditRecord{Start: 4, OldEnd: 5}, []Span{{3, 5, 1}}},
		{"delete covering", EditRecord{Start: 2, OldEnd: 8}, nil},
		{"delete across start", EditRecord{Start: 1, OldEnd: 4}, []Span{{1, 3, 1}}},
		{"replace inside", EditRecord{Start: 4, OldEnd: 5, Text: []byte("XYZ")}, []Span{{3, 8, 1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shift(spans, tt.e)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
	if spans[0] != (Span{3, 6, 1}) {
		t.Fatal("shift modified its input")
	}
}

func TestClosest(t *testing.T) {
	styles := map[string]int{"function": 1, "function.method": 2, "keyword": 3}
	tests := []struct {
		capture string
		key     string
		ok      bool
	}{
		{"function.method.builtin", "function.method", true},
		{"function.builtin", "function", true},
		{"function.method", "function.method", true},
		{"keyword.control.import", "keyword", true},
		{"variable", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		key, _, ok := Closest(styles, tt.capture)
		if key != tt.key || ok != tt.ok {
			t.Errorf("Closest(%q) = %q, %v; want %q, %v", tt.capture, key, ok, tt.key, tt.ok)
		}
	}
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// tsSample and pySample are small programs for the random-edit test.
const tsSample = `import { x } from "./x";

export interface Point { x: number; y: number }

export function dist(a: Point, b: Point): number {
  const dx = a.x - b.x; // horizontal
  const dy = a.y - b.y;
  return Math.sqrt(dx * dx + dy * dy) + 1;
}
`

const pySample = `import math


class Shape:
    """A shape with an area."""

    def __init__(self, r: float) -> None:
        self.r = r

    def area(self) -> float:
        return math.pi * self.r ** 2  # circle


if __name__ == "__main__":
    print(Shape(2).area(), None, True)
`

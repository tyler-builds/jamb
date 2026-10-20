package highlight

import (
	"bytes"
	"math/rand"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// openTest opens a session on text, with a notification channel for each
// publish, and sets the viewport to the whole text.
func openTest(t *testing.T, lang string, text []byte, notify chan struct{}) *Session {
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
		done := !s.busy && len(s.queue) == 0 && !s.viewDirty && !s.first && s.synced == s.version
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
		case <-time.After(5 * time.Millisecond): // a round that publishes nothing new does not notify
		case <-deadline:
			t.Fatal("worker did not catch up")
		}
	}
}

// freshSpans returns the spans of text lexed as a whole.
func freshSpans(t *testing.T, lang string, text []byte) []Span {
	t.Helper()
	sp, err := Highlight(lang, text)
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

// inRange returns the spans that intersect [lo, hi).
func inRange(spans []Span, lo, hi int) []Span {
	var out []Span
	for _, sp := range spans {
		if sp.End > lo && sp.Start < hi {
			out = append(out, sp)
		}
	}
	return out
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
	text := []byte("package main\n\nfunc main() {\n\tprintln(1)\n}\n")
	hold, release := holdWorker(t)
	notify := make(chan struct{}, 100)
	s := openTest(t, "go", text, notify)
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
		t.Fatalf("revision = %d, want %d", v1, v0+1)
	}
	if want := shift(before, e); !slices.Equal(after, want) {
		t.Fatalf("spans after edit = %v, want the shifted spans %v", after, want)
	}
	release()
	settle(t, s, notify)

	final := slices.Concat(text[:31], []byte("xy"), text[31:])
	got, v2 := s.Spans()
	if want := freshSpans(t, "go", final); !slices.Equal(got, want) {
		t.Fatalf("spans after the worker answered differ from a fresh lex:\n got %v\nwant %v", got, want)
	}
	// The worker's answer is a new revision with no new edit, so an editor that keeps an
	// index of the spans by revision rebuilds it and paints the new colors at once.
	if v2 == v1 {
		t.Fatalf("the revision stayed %d when the worker published new spans", v1)
	}
}

func TestBurstIsCoalescedIntoOneRound(t *testing.T) {
	text := []byte("def f(a, b):\n    return a + b\n")
	hold, release := holdWorker(t)
	notify := make(chan struct{}, 1000)
	s := openTest(t, "python", text, notify)
	defer s.Close()
	defer release()
	settle(t, s, notify)

	before := s.rounds.Load()
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
	if n := s.rounds.Load() - before; n != 1 {
		t.Fatalf("200 queued edits took %d rounds, want 1", n)
	}
	got, _ := s.Spans()
	if want := freshSpans(t, "python", final); !slices.Equal(got, want) {
		t.Fatalf("spans after the burst differ from a fresh lex")
	}
}

// editSample is text with characters that the lexers treat differently, for
// random edits, with multi-byte characters and a carriage return.
const editSample = "xyz _(){}[]\n\"'`=;:.,+-*/<>!&|1\t#é😀\r"

// randomEdit returns a random edit of text at character boundaries, inserting or
// deleting a few bytes.
func randomEdit(rng *rand.Rand, text []byte) EditRecord {
	sample := []rune(editSample)
	boundary := func(at int) int {
		for at > 0 && at < len(text) && text[at]&0xC0 == 0x80 {
			at--
		}
		return at
	}
	if len(text) == 0 || rng.Intn(2) == 0 {
		at := boundary(rng.Intn(len(text) + 1))
		var ins []byte
		for range 1 + rng.Intn(6) {
			ins = append(ins, string(sample[rng.Intn(len(sample))])...)
		}
		return EditRecord{Start: at, OldEnd: at, Text: ins}
	}
	at := boundary(rng.Intn(len(text)))
	end := min(at+1+rng.Intn(8), len(text))
	for end < len(text) && text[end]&0xC0 == 0x80 {
		end++
	}
	return EditRecord{Start: at, OldEnd: end}
}

// apply returns text with e applied.
func apply(text []byte, e EditRecord) []byte {
	return slices.Concat(text[:e.Start], e.Text, text[e.OldEnd:])
}

func TestSpansMatchFreshLexAfterRandomEdits(t *testing.T) {
	samples := []struct {
		name, lang string
		text       []byte
	}{
		{"go", "go", mustRead(t, "session.go")},
		{"typescript", "typescript", []byte(tsSample)},
		{"python", "python", []byte(pySample)},
		{"markdown", "markdown", mustRead(t, "../../docs/codeedit.md")},
	}
	for _, sm := range samples {
		t.Run(sm.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(7))
			text := slices.Clone(sm.text)
			notify := make(chan struct{}, 1000)
			s := openTest(t, sm.lang, text, notify)
			defer s.Close()
			settle(t, s, notify)
			for batch := range 30 {
				for range 10 {
					e := randomEdit(rng, text)
					if err := s.Edit(e); err != nil {
						t.Fatal(err)
					}
					text = apply(text, e)
				}
				// Alternate between the whole text and a window of it in view.
				lo, hi := 0, len(text)
				if batch%2 == 1 {
					lo = rng.Intn(len(text) + 1)
					hi = min(len(text), lo+rng.Intn(20000))
				}
				if err := s.SetViewport(lo, hi); err != nil {
					t.Fatal(err)
				}
				settle(t, s, notify)
				if want := lineStarts(text); !slices.Equal(s.lines, want) {
					t.Fatalf("batch %d: the worker's line offsets differ from the text's", batch)
				}
				got, _ := s.Spans()
				s.mu.Lock()
				view := s.view
				s.mu.Unlock()
				want := inRange(freshSpans(t, sm.lang, text), view[0], view[1])
				if len(want) == 0 && view[1]-view[0] > 1000 {
					t.Fatalf("batch %d: no spans in [%d, %d)", batch, view[0], view[1])
				}
				if got = inRange(got, view[0], view[1]); !slices.Equal(got, want) {
					t.Fatalf("batch %d: spans of [%d, %d) differ from a fresh lex of %d bytes", batch, view[0], view[1], len(text))
				}
			}
		})
	}
}

// TestOpenerRecolorsTheRest types a comment opener at the top of a long file
// with the end in view: the spans of the end become a comment, and the worker
// goes on to relex the lines outside the view until none wait.
func TestOpenerRecolorsTheRest(t *testing.T) {
	text := bytes.Repeat([]byte("func f() int { return 1 }\n"), 20000)
	notify := make(chan struct{}, 1000)
	s := openTest(t, "go", text, notify)
	defer s.Close()
	end := len(text)
	if err := s.SetViewport(end-2000, end); err != nil {
		t.Fatal(err)
	}
	settle(t, s, notify)
	if err := s.Edit(EditRecord{Start: 0, OldEnd: 0, Text: []byte("/*")}); err != nil {
		t.Fatal(err)
	}
	settle(t, s, notify)
	got, _ := s.Spans()
	names := s.CaptureNames()
	tail := inRange(got, end-1000, end+2)
	if len(tail) == 0 {
		t.Fatal("no spans at the end")
	}
	for _, sp := range tail {
		if names[sp.Capture] != "comment" {
			t.Fatalf("after /* at the top, %q at the end is %s", text[sp.Start-2:sp.End-2], names[sp.Capture])
		}
	}
	for deadline := time.Now().Add(30 * time.Second); ; {
		s.mu.Lock()
		behind := s.behind
		s.mu.Unlock()
		if !behind {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("lines outside the view still wait")
		}
		time.Sleep(time.Millisecond)
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
	styles := map[string]int{"function": 1, "function.method": 2, "keyword": 3, "property": 4}
	tests := []struct {
		capture string
		key     string
		ok      bool
	}{
		{"function.method.builtin", "function.method", true},
		{"function.builtin", "function", true},
		{"keyword.control.import", "keyword", true},
		{"selector.class", "property", true},
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

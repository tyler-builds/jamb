package codeedit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// instantTimers makes the hover's timer fire at once for the test, and returns the delays it was
// asked for.
func instantTimers(t *testing.T) *[]time.Duration {
	var delays []time.Duration
	old := afterFunc
	afterFunc = func(d time.Duration, f func()) { delays = append(delays, d); f() }
	t.Cleanup(func() { afterFunc = old })
	return &delays
}

// waitFor draws frames until cond holds, for the answer of a provider that runs on a goroutine.
func (h *harness) waitFor(what string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
		h.frame()
	}
}

// rest moves the pointer onto the cell col of the display row vi, where it rests.
func (h *harness) rest(vi, col int) {
	x, y := h.lineXY(vi, col)
	h.rt.PointerMove(x, y, 0)
	h.frame()
}

// The hover opens after the delay with the provider's answer, which arrives with a frame, under
// the text it is about.
func TestHoverOpensAfterTheDelay(t *testing.T) {
	delays := instantTimers(t)
	var asked Query
	h := newHarness(t, "alpha beta", func(e *EditorElement) *EditorElement {
		return e.HoverDelay(123 * time.Millisecond).Hover(HoverFunc(func(ctx context.Context, q Query) (Hover, error) {
			asked = q
			return Hover{Text: "a word"}, nil
		}))
	})
	vs := h.ed.view
	h.rest(0, 7)
	if len(*delays) != 1 || (*delays)[0] != 123*time.Millisecond {
		t.Fatalf("the hover asked for the delays %v, want one of 123ms", *delays)
	}
	h.waitFor("the hover", func() bool { return vs.hover.open })
	if asked.Text != "alpha beta" || asked.Offset < 6 || asked.Version != h.doc.TextVersion() {
		t.Errorf("the provider was asked %+v", asked)
	}
	if vs.hover.from != 6 || vs.hover.to != 10 {
		t.Errorf("the hover is about [%d, %d), want the word [6, 10)", vs.hover.from, vs.hover.to)
	}
	h.frame()
	pop, ok := h.rt.Find("codeedit.hover")
	if !ok {
		t.Fatal("the hover popup is not in the tree")
	}
	if _, y := h.lineXY(0, 0); pop.Y < y {
		t.Errorf("the popup is at y %v, above the row at %v", pop.Y, y)
	}
	// A move off the word closes it.
	h.rest(0, 1)
	if vs.hover.open {
		t.Error("a move off the range left the hover open")
	}
}

// An edit cancels the request on its way, and its answer is dropped.
func TestHoverEditCancelsThePendingRequest(t *testing.T) {
	instantTimers(t)
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	h := newHarness(t, "alpha beta", func(e *EditorElement) *EditorElement {
		return e.Hover(HoverFunc(func(ctx context.Context, q Query) (Hover, error) {
			started <- ctx
			<-release
			return Hover{Text: "late"}, nil
		}))
	})
	vs := h.ed.view
	h.rest(0, 2)
	var ctx context.Context
	h.waitFor("the request", func() bool {
		select {
		case ctx = <-started:
			return true
		default:
			return false
		}
	})
	h.typeText("x")
	if ctx.Err() == nil {
		t.Fatal("an edit did not cancel the pending request")
	}
	close(release)
	for i := 0; i < 20; i++ {
		time.Sleep(time.Millisecond)
		h.frame()
	}
	if vs.hover.open || vs.hover.answer.Text != "" {
		t.Errorf("the canceled answer was shown: %+v", vs.hover.answer)
	}
}

// Ctrl+K Ctrl+I opens the hover at the caret and announces it, with the diagnostics there; Escape
// closes it and leaves the selection.
func TestHoverChordOpensAndAnnounces(t *testing.T) {
	h := newHarness(t, "alpha beta", func(e *EditorElement) *EditorElement {
		return e.Hover(HoverFunc(func(ctx context.Context, q Query) (Hover, error) {
			return Hover{Text: "beta is a word"}, nil
		}))
	})
	updates := activateAccess(h)
	vs := h.ed.view
	h.doc.SetDiagnostics([]Diagnostic{{From: 6, To: 10, Severity: SeverityHint, Message: "unused"}})
	h.doc.SetSelections(Selection{Anchor: 6, Head: 8})
	h.frame()
	h.key(input.KeyK, input.ModCtrl)
	h.key(input.KeyI, input.ModCtrl)
	h.waitFor("the hover", func() bool { return vs.hover.open && vs.hover.answer.Text != "" })
	h.frame()
	got := strings.Join(announcements(*updates), "|")
	if !strings.Contains(got, "Hint on line 1: unused\nbeta is a word") {
		t.Errorf("the announcements are %q, want the diagnostic and the hover", got)
	}
	h.key(input.KeyEscape, 0)
	if vs.hover.open {
		t.Error("Escape left the hover open")
	}
	if p := h.doc.Primary(); p.Anchor != 6 || p.Head != 8 {
		t.Errorf("Escape over the hover changed the selection to %+v", p)
	}
}

// Without a provider the hover shows the diagnostics under the pointer, at once.
func TestHoverShowsDiagnosticsWithoutAProvider(t *testing.T) {
	instantTimers(t)
	h := newHarness(t, "alpha beta")
	vs := h.ed.view
	h.doc.SetDiagnostics([]Diagnostic{{From: 0, To: 5, Message: "undefined: alpha", Source: "vet"}})
	h.frame()
	h.rest(0, 2)
	h.frame()
	if !vs.hover.open || len(vs.hover.diags) != 1 {
		t.Fatalf("the hover is open %v with %d diagnostics, want the one under the pointer", vs.hover.open, len(vs.hover.diags))
	}
	if _, ok := h.rt.Find("codeedit.hover"); !ok {
		t.Error("the hover popup is not in the tree")
	}
}

// The hover and the list can be open together, and their anchors are siblings: each has its own
// key, so the runtime warns of no duplicate key.
func TestHoverAndListOpenTogetherWarnOfNoDuplicateKey(t *testing.T) {
	instantTimers(t)
	h := newHarness(t, "al alpha", func(e *EditorElement) *EditorElement {
		return e.HoverDelay(time.Millisecond).
			Hover(HoverFunc(func(ctx context.Context, q Query) (Hover, error) { return Hover{Text: "a word"}, nil })).
			Completions(words("alpha", "alfa"))
	})
	var warns []string
	h.rt.SetWarnFunc(func(s string) { warns = append(warns, s) })
	vs := h.ed.view
	h.doc.SetCaret(2)
	h.openList()
	h.rest(0, 1)
	h.waitFor("the hover", func() bool { return vs.hover.open })
	h.frame()
	if !vs.completion.open {
		t.Fatal("the list closed when the hover opened")
	}
	if _, ok := h.rt.Find("codeedit.hover"); !ok {
		t.Error("the hover is not in the tree")
	}
	if _, ok := h.rt.Find("codeedit.completions"); !ok {
		t.Error("the list is not in the tree")
	}
	for _, w := range warns {
		if strings.Contains(w, "duplicate key") {
			t.Errorf("warning: %s", w)
		}
	}
}

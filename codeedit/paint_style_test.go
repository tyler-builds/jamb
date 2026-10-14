package codeedit

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/codeedit/highlight"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// A run takes the slant of the capture of the span that covers it: an italic capture slants its
// run, and a run covered by another capture keeps the upright style.
func TestSegmentStyleTakesItalicFromTheCapture(t *testing.T) {
	vs := &viewState{
		spans: spanIndex{list: []highlight.Span{{Start: 0, End: 4, Capture: 0}, {Start: 5, End: 9, Capture: 1}}},
		capStyle: []capStyle{
			{st: ui.SyntaxStyle{Italic: true}, ok: true},
			{st: ui.SyntaxStyle{Bold: true}, ok: true},
		},
	}
	fg := ui.Hex("#ffffff")
	base := ui.TextStyle{Mono: true}
	if seg := vs.segmentStyle(base, fg, []int{0, 1}, 0, 0, 3); !seg.Italic || !seg.Mono {
		t.Errorf("the run under the italic capture = %+v, want italic and mono", seg)
	}
	if seg := vs.segmentStyle(base, fg, []int{0, 1}, 0, 5, 8); seg.Italic || seg.Weight != ui.FontSemibold {
		t.Errorf("the run under the bold capture = %+v, want upright and semibold", seg)
	}
}

// The built-in themes slant comments, so a comment capture is drawn in italic.
func TestBuiltInThemesSlantComments(t *testing.T) {
	for _, th := range []*ui.Theme{ui.DarkTheme(), ui.LightTheme()} {
		st, ok := captureStyles([]string{"comment.doc.go"}, th.Syntax)
		if !ok[0] || !st[0].Italic {
			t.Errorf("the comment capture = %+v (mapped %v), want italic", st[0], ok[0])
		}
	}
}

// An edit, a move and a click restart the caret's blink at the time of the frame that draws them,
// so the caret is on at once.
func TestEditsMovesAndClicksRestartTheCaretBlink(t *testing.T) {
	h := newHarness(t, "abc")
	h.doc.SetCaret(0)
	h.frame()

	// Typing and a key press draw their own frame, on the runtime's clock, so the blink starts at
	// the time of that frame.
	from := time.Now()
	h.rt.Type("x")
	if vs := h.ed.view; vs.restart || vs.activity.Before(from) || vs.activity.After(time.Now()) {
		t.Fatalf("after typing the blink starts at %v, want the time of its frame after %v", vs.activity, from)
	}
	from = time.Now()
	h.rt.Press(input.KeyRight, 0)
	if vs := h.ed.view; vs.restart || vs.activity.Before(from) || vs.activity.After(time.Now()) {
		t.Fatalf("after a move the blink starts at %v, want the time of its frame after %v", vs.activity, from)
	}

	// A click draws on the harness's frame time.
	x, y := h.lineXY(0, 1)
	h.click(x, y, 0, 1)
	if vs := h.ed.view; !vs.activity.Equal(h.now) {
		t.Fatalf("after a click the blink starts at %v, want the frame time %v", vs.activity, h.now)
	}
}

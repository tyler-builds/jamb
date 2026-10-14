package codeedit

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/uihost"
)

// imeHost is the window host's side of the runtime, which the platform calls for an input method
// (see ui.runtimeHost).
type imeHost interface {
	Compose(text string, cursor, clauseStart, clauseEnd int)
	SetCancelComposition(fn func())
	TextInputRect() (scene.Rect, bool)
}

// compose shows what an input method composes in the focused editor, as the platform reports it,
// and draws the frame.
func (h *harness) compose(text string, cursor, clauseStart, clauseEnd int) {
	uihost.Of(h.rt).(imeHost).Compose(text, cursor, clauseStart, clauseEnd)
	h.frame()
}

// countCancels returns a counter of the times the input method was told to drop its composition.
func (h *harness) countCancels() *int {
	n := new(int)
	uihost.Of(h.rt).(imeHost).SetCancelComposition(func() { *n++ })
	return n
}

// shownRow returns what the primary caret's row shows, and whether a composition is in it.
func (h *harness) shownRow() (string, bool) {
	row := h.doc.RowOf(h.doc.Caret())
	shown, _, ok := h.ed.view.composedRow(row, h.doc.Line(row))
	return shown, ok
}

// A composition shows inline at the caret, and does not change the document.
func TestCompositionShowsInlineWithoutChangingDocument(t *testing.T) {
	h := newHarness(t, "ab")
	h.doc.SetCaret(1)
	h.frame()
	h.compose("にほん", 6, 3, 9)
	if got := h.doc.Text(); got != "ab" {
		t.Fatalf("composing changed the document to %q", got)
	}
	if shown, ok := h.shownRow(); !ok || shown != "aにほんb" {
		t.Fatalf("the row shows %q (%v), want the composition at the caret", shown, ok)
	}
}

// A second update replaces the composition.
func TestCompositionUpdateReplacesIt(t *testing.T) {
	h := newHarness(t, "ab")
	h.doc.SetCaret(1)
	h.frame()
	h.compose("に", 3, 0, 3)
	h.compose("にほん", 6, 3, 9)
	if got := h.ed.view.comp.Text; got != "にほん" {
		t.Fatalf("the composition is %q, want the second update", got)
	}
	if shown, _ := h.shownRow(); shown != "aにほんb" {
		t.Fatalf("the row shows %q after the second update", shown)
	}
}

// Empty text cancels the composition, and types nothing.
func TestEmptyCompositionCancels(t *testing.T) {
	h := newHarness(t, "ab")
	h.doc.SetCaret(1)
	h.frame()
	cancels := h.countCancels()
	h.compose("か", 3, 0, 0)
	h.compose("", 0, 0, 0)
	if _, ok := h.shownRow(); ok || h.doc.Text() != "ab" {
		t.Fatalf("document %q, composition still shown %v after an empty update", h.doc.Text(), ok)
	}
	if *cancels != 0 {
		t.Errorf("the input method was told to cancel %d times after it ended the composition itself", *cancels)
	}
}

// A key the editor handles types the composition once, before its action.
func TestCompositionCommittedByKey(t *testing.T) {
	h := newHarness(t, "ab")
	h.doc.SetCaret(2)
	h.frame()
	cancels := h.countCancels()
	h.compose("中文", 6, 0, 0)
	h.key(input.KeyHome, 0)
	if got := h.doc.Text(); got != "ab中文" {
		t.Fatalf("document %q after Home, want the composition typed once", got)
	}
	if h.doc.Caret() != 0 || h.ed.view.comp.Text != "" || *cancels != 1 {
		t.Fatalf("caret %d, composition %q, %d cancels after Home", h.doc.Caret(), h.ed.view.comp.Text, *cancels)
	}
}

// A click in the editor types the composition, then places the caret.
func TestCompositionCommittedByClick(t *testing.T) {
	h := newHarness(t, "ab")
	h.doc.SetCaret(2)
	h.frame()
	cancels := h.countCancels()
	h.compose("中文", 6, 0, 0)
	x, y := h.lineXY(0, 0)
	h.click(x-h.ed.view.geo.charW/2+1, y, 0, 1) // just inside the left edge of the first cell
	if got := h.doc.Text(); got != "ab中文" {
		t.Fatalf("document %q after the click, want the composition typed once", got)
	}
	if h.doc.Caret() != 0 || *cancels != 1 {
		t.Fatalf("caret %d, %d cancels after the click", h.doc.Caret(), *cancels)
	}
}

// A read-only editor shows no composition.
func TestReadOnlyEditorShowsNoComposition(t *testing.T) {
	h := newHarness(t, "ab", func(e *Editor) *Editor { return e.ReadOnly(true) })
	h.doc.SetCaret(1)
	h.frame()
	h.compose("か", 3, 0, 0)
	if _, ok := h.shownRow(); ok || h.doc.Text() != "ab" {
		t.Fatalf("document %q, composition shown %v in a read-only editor", h.doc.Text(), ok)
	}
}

// The candidate window opens at the input method's caret inside the composition, and at the
// caret when there is none.
func TestCompositionCaretRectangle(t *testing.T) {
	h := newHarness(t, "ab")
	h.doc.SetCaret(1)
	h.frame()
	g := h.ed.view.geo
	x0, y0 := h.lineXY(0, 0) // the centre of the first cell of the row
	near := func(a, b float32) bool { return a-b < 0.5 && b-a < 0.5 }

	// After "a" and the first character of the composition, which is 2 cells wide.
	h.compose("中文", 3, 0, 3)
	rect, ok := uihost.Of(h.rt).(imeHost).TextInputRect()
	if want := x0 - g.charW/2 + 3*g.charW; !ok || !near(rect.X, want) {
		t.Errorf("the input rectangle starts at %v (%v), want the input method's caret at %v", rect.X, ok, want)
	}
	if want := y0 - g.lineH/2; !near(rect.Y, want) {
		t.Errorf("the input rectangle is at y %v, want the row at %v", rect.Y, want)
	}

	// Without a composition, it is at the caret, after "a".
	h.compose("", 0, 0, 0)
	rect, _ = uihost.Of(h.rt).(imeHost).TextInputRect()
	if want := x0 - g.charW/2 + g.charW; !near(rect.X, want) {
		t.Errorf("the input rectangle starts at %v without a composition, want the caret at %v", rect.X, want)
	}
}

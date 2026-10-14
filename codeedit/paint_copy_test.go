package codeedit

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// TestPaintAfterEditCopiesNoText checks that the keystroke and paint path reads the document
// through the rope and never copies the whole text: a paint after each typed character on a
// document of 100,000 lines, with the caret next to a bracket, makes no copy of the text.
func TestPaintAfterEditCopiesNoText(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 100,000 line document")
	}
	h := newHarness(t, sourceText(100000))
	h.doc.SetCaret(h.doc.Len() / 2)
	h.frame()
	before := textCopies
	for i := 0; i < 5; i++ {
		h.typeText("x")
		h.key(input.KeyLeft, 0)
		h.key(input.KeyRight, 0)
	}
	// A bracket at the caret makes the paint look for its pair, which must stay within a window.
	h.doc.SetCaret(indexOf(h.doc.Text(), "(") + 1)
	before = textCopies
	h.frame()
	h.typeText("y")
	if textCopies != before {
		t.Fatalf("paint and keystrokes copied the whole text %d times", textCopies-before)
	}
}

// indexOf returns the byte offset of the first occurrence of s in text, or 0 when there is none.
func indexOf(text, s string) int {
	for i := 0; i+len(s) <= len(text); i++ {
		if text[i:i+len(s)] == s {
			return i
		}
	}
	return 0
}

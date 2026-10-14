package codeedit

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// TestAssistiveSelectionSetsPrimarySelection checks that the Text pattern of the editor is writable:
// a SetTextSelection through the access layer selects the range of byte offsets in the document,
// and ScrollIntoView asks the editor to reveal its caret.
func TestAssistiveSelectionSetsPrimarySelection(t *testing.T) {
	h := newHarness(t, "one two three\nfour")
	activateAccess(h)
	h.frame()
	node := accessField(t, h)
	if !node.Has(access.ActionSetTextSelection) {
		t.Fatalf("the editor's actions = %v, want set text selection", node.Actions)
	}
	bridge := h.rt.AccessBridge()
	bridge.Do(node.ID, access.Request{Action: access.ActionSetTextSelection, Anchor: 4, Caret: 7})
	h.frame()
	if got := h.doc.Primary(); got != (Selection{Anchor: 4, Head: 7}) {
		t.Fatalf("selection after SetTextSelection = %+v, want {4 7}", got)
	}
	if h.doc.Text()[4:7] != "two" {
		t.Fatalf("the selected text is %q", h.doc.Text()[4:7])
	}

}

// TestAssistiveScrollIntoViewRevealsCaret checks that ScrollIntoView through the Text pattern scrolls
// the editor to its caret: the caret is far down a long document, the view is back at the top, and
// the request moves the view. The request runs on the UI thread at the start of the next frame.
func TestAssistiveScrollIntoViewRevealsCaret(t *testing.T) {
	text := strings.Repeat("a line of text\n", 300)
	h := newHarness(t, text)
	activateAccess(h)
	h.doc.SetCaret(h.doc.Len())
	h.frame()
	h.ed.view.scrollY = 0
	node := accessField(t, h)
	if !node.Has(access.ActionScrollIntoView) {
		t.Fatalf("the editor's actions = %v, want scroll into view", node.Actions)
	}
	h.rt.AccessBridge().Do(node.ID, access.Request{Action: access.ActionScrollIntoView})
	h.frame()
	if h.ed.view.scrollY <= 0 {
		t.Fatalf("ScrollIntoView left the view at the top: scroll %v", h.ed.view.scrollY)
	}
}

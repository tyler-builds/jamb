package codeedit

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// accessField returns the access node of the editor (the harness labels it "Source") from the
// published tree, after a frame. The tree is published when a frame changed the UI.
func accessField(t *testing.T, h *harness) *access.Node {
	t.Helper()
	h.frame()
	return accesstest.Find(t, h.rt.AccessBridge().Tree(), access.RoleTextArea, "Source")
}

// activateAccess starts the access tree of the harness's runtime, and returns the list of the
// updates it publishes from then on.
func activateAccess(h *harness) *[]access.Update {
	var updates []access.Update
	bridge := h.rt.AccessBridge()
	bridge.Listen(func(u access.Update) { updates = append(updates, u) })
	bridge.Activate()
	return &updates
}

// hasID reports whether the update lists have id.
func hasID(lists [][]access.ID, id access.ID) bool {
	for _, l := range lists {
		for _, x := range l {
			if x == id {
				return true
			}
		}
	}
	return false
}

// textChanges returns the text changes of the updates, one list for each update.
func textChanges(us []access.Update) [][]access.ID {
	var out [][]access.ID
	for _, u := range us {
		out = append(out, u.TextChanged)
	}
	return out
}

// selectionChanges returns the selection changes of the updates, one list for each update.
func selectionChanges(us []access.Update) [][]access.ID {
	var out [][]access.ID
	for _, u := range us {
		out = append(out, u.TextSelection)
	}
	return out
}

func TestEditorExposesTheTextPattern(t *testing.T) {
	const value = "one two\nthree\n"
	h := newHarness(t, value)
	activateAccess(h)
	n := accessField(t, h)
	if n.Text == nil {
		t.Fatal("the editor has no Text pattern")
	}
	c := n.Text.Content
	if got := c.Read(0, c.Len(), -1); got != value {
		t.Errorf("text = %q, want the document's text %q", got, value)
	}
	if c.Len() != len(value) {
		t.Errorf("end = %d, want the byte length %d", c.Len(), len(value))
	}
	if n.Text.Caret < 0 || n.Text.Caret > c.Len() {
		t.Errorf("caret %d is outside the text", n.Text.Caret)
	}
	if n.Text.ReadOnly {
		t.Error("an editor that is not read-only reports ReadOnly")
	}
	if rs := n.Text.Whole().Rects(); len(rs) == 0 {
		t.Error("the whole text of a painted editor has no rectangles")
	}
}

func TestEditorReportsTextAndSelectionChanges(t *testing.T) {
	h := newHarness(t, "abc")
	updates := activateAccess(h)
	id := accessField(t, h).ID

	*updates = nil
	h.doc.SetCaret(len("abc"))
	h.typeText("d")
	h.frame()
	if !hasID(textChanges(*updates), id) {
		t.Errorf("typing did not report a text change for the editor: %+v", *updates)
	}

	*updates = nil
	h.doc.SetCaret(0)
	h.frame()
	if !hasID(selectionChanges(*updates), id) {
		t.Errorf("a caret move did not report a selection change: %+v", *updates)
	}
	if hasID(textChanges(*updates), id) {
		t.Error("a caret move reported a text change")
	}
}

// The text model of a large document is queried without a copy of its text. The test counts the
// whole-text copies that Document.Text makes (textCopies), which is the copy that a query of the
// whole text would need: a range read near the caret, unit moves and expands near the caret, and the
// rectangles of the whole text must not add to the count. The rectangles of the whole text are the
// visible rows only, since the rows scrolled out of the editor are clipped.
func TestLargeDocumentQueriesDoNotCopyTheText(t *testing.T) {
	const line = "line of text\n"
	text := strings.Repeat(line, 100000)
	h := newHarness(t, text)
	activateAccess(h)
	h.doc.SetCaret(len(text)/2 + 3)
	info := accessField(t, h).Text
	c := info.Content
	if c.Len() != len(text) {
		t.Fatalf("end = %d, want %d", c.Len(), len(text))
	}

	// The document keeps a copy of its text until the text changes, and the frames above made one
	// (the bracket match reads the whole text when it paints). The copy is dropped here, so that a
	// whole-text read in the queries below is a cache miss, which the counter counts.
	h.doc.cacheOK = false
	before := textCopies
	caret := access.TextRange{Info: info, Start: info.Caret, End: info.Caret}
	small := caret
	for range 6 {
		small.End = c.Next(access.UnitCharacter, small.End)
	}
	if got, want := small.GetText(-1), text[c.ByteOf(small.Start):c.ByteOf(small.End)]; got != want {
		t.Errorf("range read near the caret = %q, want %q", got, want)
	}
	if got := small.GetText(3); got != small.GetText(-1)[:3] {
		t.Errorf("range read with a limit of 3 = %q", got)
	}
	if moved, k := caret.Move(access.UnitWord, 2); k != 2 || moved.Start <= caret.Start {
		t.Errorf("word move near the caret = %d units to %d, want 2 forward", k, moved.Start)
	}
	if e := caret.Expand(access.UnitLine); e.Text() != line {
		t.Errorf("line near the caret = %q, want %q", e.Text(), line)
	}
	if e := caret.Expand(access.UnitParagraph); e.End-e.Start != len(line) {
		t.Errorf("paragraph near the caret spans %d points, want %d", e.End-e.Start, len(line))
	}
	if got := textCopies - before; got != 0 {
		t.Errorf("the queries copied the whole text %d times", got)
	}

	rs := info.Whole().Rects()
	if len(rs) == 0 || len(rs) > 100 {
		t.Errorf("rectangles of the whole text = %d, want the visible rows only", len(rs))
	}
	if got := textCopies - before; got != 0 {
		t.Errorf("the rectangles copied the whole text %d times", got)
	}
	// The counter does move for a whole-text copy, so the zero counts above mean something.
	_ = h.doc.Text()
	if textCopies == before {
		t.Error("the copy counter did not count a whole-text copy")
	}
}

// The model of a folded document places its rows as the document does: a hidden row has no
// height, and the rows after it move up.
func TestModelFollowsFoldedRows(t *testing.T) {
	h := newHarness(t, "func a() {\n\tx\n\ty\n}\nfunc b() {}\n")
	activateAccess(h)
	h.doc.Fold(0)
	c := accessField(t, h).Text.Content
	lineAt := func(row int) access.TextLine {
		ln, _ := c.Line(h.doc.LineStart(row))
		return ln
	}
	first, after := lineAt(0), lineAt(4)
	for _, row := range []int{1, 2} {
		if got := lineAt(row).Height; got != 0 {
			t.Errorf("hidden row %d has height %v, want 0", row, got)
		}
	}
	if first.Height <= 0 {
		t.Fatalf("the first row has height %v", first.Height)
	}
	if want := float32(h.doc.VisibleIndex(4)) * first.Height; after.Top-first.Top != want {
		t.Errorf("row 4 is %v below row 0, want %d shown rows of %v", after.Top-first.Top, h.doc.VisibleIndex(4), first.Height)
	}
	if got := c.Nearest(0, after.Top+first.Height/2); got != h.doc.LineStart(4) {
		t.Errorf("the point nearest to row 4 is %d, want its start %d", got, h.doc.LineStart(4))
	}
}

// A model holds the text it was made from: a change made after it was made does not show in it.
func TestModelIsImmutable(t *testing.T) {
	h := newHarness(t, "abc")
	activateAccess(h)
	old := accessField(t, h).Text.Content
	h.doc.SetCaret(3)
	h.typeText("def")
	h.frame()
	if got := old.Read(0, old.Len(), -1); got != "abc" {
		t.Errorf("the old model reads %q after an edit, want %q", got, "abc")
	}
	now := accessField(t, h).Text.Content
	if now == old {
		t.Error("an edit did not make a new model")
	}
	if got := now.Read(0, now.Len(), -1); got != "abcdef" {
		t.Errorf("the new model reads %q, want %q", got, "abcdef")
	}
}

// The model is kept while the text and the layout stay the same, so a range of the tree keeps
// comparing equal with the content of its snapshot (see access.TextRange.Same).
func TestModelIsKeptWhileUnchanged(t *testing.T) {
	h := newHarness(t, "abc\ndef\n")
	activateAccess(h)
	first := accessField(t, h).Text.Content
	second := accessField(t, h).Text.Content
	if first != second {
		t.Error("the model changed in frames that changed no text and no layout")
	}
}

// The model finds the point under a window position as the pointer does.
func TestModelNearestMatchesThePointer(t *testing.T) {
	h := newHarness(t, "alpha\nbeta\ngamma\n")
	activateAccess(h)
	info := accessField(t, h).Text
	x, y := h.lineXY(1, 2)
	r, _ := h.rt.Find("wrap")
	_, want, ok := h.ed.view.hit(x-(r.X+1), y-(r.Y+1))
	if !ok {
		t.Fatal("the pointer finds no point")
	}
	if got := info.PointAt(x, y); got != want {
		t.Errorf("the model's point at (%v, %v) = %d, the pointer's %d", x, y, got, want)
	}
}

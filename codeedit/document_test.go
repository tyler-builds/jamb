package codeedit

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// clock is a fake time source for Document, advanced by the tests.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestDoc(text string) (*Document, *clock) {
	d := NewDocument(text)
	c := &clock{t: time.Unix(1_000_000, 0)}
	d.now = c.now
	return d, c
}

// randText returns a random text of n runes, with line feeds and some multi-byte runes.
func randText(rng *rand.Rand, n int) string {
	alphabet := []rune("abc xyz\n\n\t()é世_")
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(alphabet[rng.Intn(len(alphabet))])
	}
	return b.String()
}

// randChanges returns sorted, non-overlapping changes that are valid for text.
func randChanges(rng *rand.Rand, text string) []Change {
	var out []Change
	pos := 0
	for pos <= len(text) && rng.Intn(3) != 0 {
		from := pos + rng.Intn(4)
		if from > len(text) {
			break
		}
		for from < len(text) && !utf8.RuneStart(text[from]) {
			from++
		}
		to := from
		if from < len(text) && rng.Intn(2) == 0 {
			to = from + rng.Intn(len(text)-from+1)
			for to < len(text) && !utf8.RuneStart(text[to]) {
				to++
			}
		}
		ins := randText(rng, rng.Intn(3))
		out = append(out, Change{From: from, To: to, Text: ins})
		pos = to + 1
	}
	return out
}

// applyModel applies sorted changes to a plain string, the model of the Document.
func applyModel(text string, changes []Change) string {
	var b strings.Builder
	prev := 0
	for _, c := range changes {
		b.WriteString(text[prev:c.From])
		b.WriteString(c.Text)
		prev = c.To
	}
	b.WriteString(text[prev:])
	return b.String()
}

// TestDocumentMatchesModel applies random multi-part edits, undo and redo to a Document and
// to a string model, and checks the text and the line index after each step.
func TestDocumentMatchesModel(t *testing.T) {
	for seed := int64(1); seed <= 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		start := randText(rng, 60)
		d, c := newTestDoc(start)
		text := start
		var undo, redo []string
		for step := 0; step < 150; step++ {
			c.t = c.t.Add(3 * time.Second) // keep every step its own undo step
			switch rng.Intn(5) {
			case 0, 1, 2:
				changes := randChanges(rng, text)
				if err := d.Edit(changes...); err != nil {
					t.Fatalf("seed %d step %d: Edit: %v", seed, step, err)
				}
				if len(changes) > 0 {
					// An edit with no changes is no step, as in the document.
					undo = append(undo, text)
					redo = nil
				}
				text = applyModel(text, changes)
			case 3:
				if d.Undo() {
					redo = append(redo, text)
					text = undo[len(undo)-1]
					undo = undo[:len(undo)-1]
				}
			case 4:
				if d.Redo() {
					undo = append(undo, text)
					text = redo[len(redo)-1]
					redo = redo[:len(redo)-1]
				}
			}
			if got := d.Text(); got != text {
				t.Fatalf("seed %d step %d: text\n got %q\nwant %q", seed, step, got, text)
			}
			checkLineIndex(t, d, text)
		}
	}
}

// checkLineIndex checks that the line index of d agrees with text.
func checkLineIndex(t *testing.T, d *Document, text string) {
	t.Helper()
	lines := strings.Split(text, "\n")
	if d.LineCount() != len(lines) {
		t.Fatalf("LineCount = %d, want %d", d.LineCount(), len(lines))
	}
	off := 0
	for row, line := range lines {
		if got := d.LineStart(row); got != off {
			t.Fatalf("LineStart(%d) = %d, want %d", row, got, off)
		}
		if got := d.Line(row); got != line {
			t.Fatalf("Line(%d) = %q, want %q", row, got, line)
		}
		if got := d.RowOf(off); got != row {
			t.Fatalf("RowOf(%d) = %d, want %d", off, got, row)
		}
		off += len(line) + 1
	}
}

// TestSelectionsMapThroughEdits checks where carets go when the text around them changes.
func TestSelectionsMapThroughEdits(t *testing.T) {
	d, _ := newTestDoc("hello world")
	d.SetSelections(Selection{Anchor: 6, Head: 6})
	// An insertion before the caret moves it; one at the caret stays before the text.
	if err := d.Edit(Change{From: 0, To: 0, Text: ">> "}); err != nil {
		t.Fatal(err)
	}
	if got := d.Primary(); got != (Selection{9, 9}) {
		t.Fatalf("caret after insert before it = %+v, want {9 9}", got)
	}
	d.SetSelections(Selection{Anchor: 0, Head: 3})
	if err := d.Edit(Change{From: 1, To: 2, Text: "XYZ"}); err != nil {
		t.Fatal(err)
	}
	// The head is at 3, after the replaced byte, so it moves with the text. The anchor
	// stays at the start.
	if got := d.Primary(); got.Anchor != 0 || got.Head != 5 {
		t.Fatalf("selection after replacing inside = %+v, want {0 5}", got)
	}
}

// TestEditRejectsBadRanges checks that Edit refuses ranges that are out of the text, reversed,
// overlapping, or not on character boundaries, and makes no change.
func TestEditRejectsBadRanges(t *testing.T) {
	d, _ := newTestDoc("héllo")
	before := d.Text()
	bad := [][]Change{
		{{From: 0, To: 99}},
		{{From: 3, To: 1}},
		{{From: 0, To: 3}, {From: 2, To: 4}},
		{{From: 2, To: 3}}, // inside the two bytes of é
	}
	for _, cs := range bad {
		if err := d.Edit(cs...); !errors.Is(err, ErrRange) {
			t.Fatalf("Edit(%v) = %v, want ErrRange", cs, err)
		}
	}
	if d.Text() != before || d.CanUndo() {
		t.Fatal("a rejected edit changed the document or the undo history")
	}
}

// TestUndoGroupsTyping checks that typing within the group time is one undo step, and that
// a change of kind or a pause starts another.
func TestUndoGroupsTyping(t *testing.T) {
	d, c := newTestDoc("")
	for _, s := range []string{"a", "b", "c"} {
		if err := d.TypeText(s); err != nil {
			t.Fatal(err)
		}
		c.t = c.t.Add(100 * time.Millisecond)
	}
	if d.Text() != "abc" {
		t.Fatalf("text = %q, want abc", d.Text())
	}
	d.Undo()
	if d.Text() != "" {
		t.Fatalf("one undo of typing = %q, want the empty text", d.Text())
	}
	d.Redo()
	c.t = c.t.Add(5 * time.Second)
	if err := d.TypeText("d"); err != nil {
		t.Fatal(err)
	}
	d.Undo()
	if d.Text() != "abc" {
		t.Fatalf("a pause should start a new step: text = %q, want abc", d.Text())
	}
}

// TestUndoRestoresSelections checks that undo puts back the selections with the text.
func TestUndoRestoresSelections(t *testing.T) {
	d, c := newTestDoc("one two")
	d.SetSelections(Selection{Anchor: 0, Head: 3})
	c.t = c.t.Add(time.Minute)
	if err := d.Replace(0, 3, "1"); err != nil {
		t.Fatal(err)
	}
	d.Undo()
	if d.Text() != "one two" || d.Primary() != (Selection{0, 3}) {
		t.Fatalf("undo gave %q with %+v, want the text and the selection back", d.Text(), d.Primary())
	}
}

// TestMultiCursorTypingAndDeleting checks that typing and deleting at several carets edit
// each of them, and leave a caret after each piece of text.
func TestMultiCursorTypingAndDeleting(t *testing.T) {
	d, _ := newTestDoc("ab\ncd\nef")
	d.SetSelections(Selection{0, 0}, Selection{3, 3}, Selection{6, 6})
	if err := d.TypeText("X"); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "Xab\nXcd\nXef" {
		t.Fatalf("typed = %q", d.Text())
	}
	if got := d.Selections(); len(got) != 3 || got[1] != (Selection{5, 5}) || got[2] != (Selection{9, 9}) {
		t.Fatalf("carets after typing = %+v", got)
	}
	if err := d.DeleteBackward(); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "ab\ncd\nef" {
		t.Fatalf("deleted = %q", d.Text())
	}
}

// TestMultiCursorPasteOneLinePerCaret checks that a paste of as many lines as there are
// carets goes one line to each caret.
func TestMultiCursorPasteOneLinePerCaret(t *testing.T) {
	d, _ := newTestDoc("a\nb\n")
	d.SetSelections(Selection{0, 0}, Selection{2, 2})
	if err := d.Paste("1\n2"); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "1a\n2b\n" {
		t.Fatalf("pasted = %q, want one line at each caret", d.Text())
	}
}

// TestSetTextClearsHistory checks that SetText is not an undo step and clears the history.
func TestSetTextClearsHistory(t *testing.T) {
	d, c := newTestDoc("x")
	c.t = c.t.Add(time.Minute)
	if err := d.TypeText("y"); err != nil {
		t.Fatal(err)
	}
	d.SetText("loaded")
	if d.CanUndo() || d.Text() != "loaded" || d.Caret() != 0 {
		t.Fatalf("after SetText: undo %v, text %q, caret %d", d.CanUndo(), d.Text(), d.Caret())
	}
}

// TestOnEditReportsChangesInOrder checks that OnEdit reports each change with offsets that
// hold for the text before it, so that the same changes applied to a copy give the text.
func TestOnEditReportsChangesInOrder(t *testing.T) {
	d, _ := newTestDoc("abcdef")
	mirror := "abcdef"
	cancel := d.OnEdit(func(c Change) {
		mirror = mirror[:c.From] + c.Text + mirror[c.To:]
	})
	defer cancel()
	if err := d.Edit(Change{From: 0, To: 1, Text: "X"}, Change{From: 3, To: 5, Text: "YY"}); err != nil {
		t.Fatal(err)
	}
	if mirror != d.Text() || d.Text() != "XbcYYf" {
		t.Fatalf("mirror %q, text %q, want XbcYYf", mirror, d.Text())
	}
}

// TestDocumentVersionAndListeners checks that listeners run on every change and that a
// cancelled listener does not.
func TestDocumentVersionAndListeners(t *testing.T) {
	d, _ := newTestDoc("a")
	calls := 0
	cancel := d.OnChange(func() { calls++ })
	v := d.Version()
	d.SetCaret(1)
	if d.Version() == v || calls != 1 {
		t.Fatalf("selection change: version %d -> %d, calls %d", v, d.Version(), calls)
	}
	cancel()
	if err := d.TypeText("b"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("a cancelled listener ran: calls %d", calls)
	}
}

// TestNewDocumentNormalizesText checks the load rules: CR LF becomes LF, and invalid UTF-8
// becomes U+FFFD.
func TestNewDocumentNormalizesText(t *testing.T) {
	d := NewDocument("a\r\nb\xffc")
	if got := d.Text(); got != "a\nb�c" {
		t.Fatalf("Text = %q, want a LF b U+FFFD c", got)
	}
}

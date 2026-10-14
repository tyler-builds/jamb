package codeedit

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// typeAll types each text as one command, with the clock advanced so that each is its own step.
func typeAll(t *testing.T, d *Document, c *clock, texts ...string) {
	t.Helper()
	for _, s := range texts {
		c.t = c.t.Add(time.Minute)
		if err := d.TypeText(s); err != nil {
			t.Fatalf("TypeText(%q): %v", s, err)
		}
	}
}

func TestAutoCloseAndTypeOver(t *testing.T) {
	d, c := newTestDoc("")
	typeAll(t, d, c, "(")
	if d.Text() != "()" || d.Caret() != 1 {
		t.Fatalf("typed ( gave %q caret %d, want () caret 1", d.Text(), d.Caret())
	}
	typeAll(t, d, c, "x", ")")
	if d.Text() != "(x)" || d.Caret() != 3 {
		t.Fatalf("typed x then ) gave %q caret %d, want (x) caret 3", d.Text(), d.Caret())
	}
}

func TestQuoteInsideWordDoesNotPair(t *testing.T) {
	d, c := newTestDoc("don")
	d.SetCaret(3)
	typeAll(t, d, c, "'")
	if d.Text() != "don'" {
		t.Fatalf("an apostrophe after a letter gave %q, want don'", d.Text())
	}
}

func TestWrapSelectionInPair(t *testing.T) {
	d, c := newTestDoc("word")
	d.SetSelections(Selection{Anchor: 0, Head: 4})
	typeAll(t, d, c, "[")
	if d.Text() != "[word]" {
		t.Fatalf("wrap gave %q, want [word]", d.Text())
	}
	if got := d.Primary(); got != (Selection{1, 5}) {
		t.Fatalf("the selection after wrapping = %+v, want the word, {1 5}", got)
	}
}

func TestBackspaceDeletesEmptyPair(t *testing.T) {
	d, c := newTestDoc("")
	typeAll(t, d, c, "{")
	if err := d.DeleteBackward(); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "" {
		t.Fatalf("backspace inside {} gave %q, want the empty text", d.Text())
	}
}

func TestNewlineKeepsIndentAndSplitsBrackets(t *testing.T) {
	d, _ := newTestDoc("func f() {")
	d.SetCaret(len(d.Text()))
	if err := d.Newline(4); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "func f() {\n    " {
		t.Fatalf("newline after { gave %q, want one more indent", d.Text())
	}
	d2, _ := newTestDoc("\tx := 1")
	d2.SetCaret(len("\tx := 1"))
	// The new line keeps the indent of its line.
	if err := d2.Newline(4); err != nil {
		t.Fatal(err)
	}
	if d2.Text() != "\tx := 1\n\t" {
		t.Fatalf("newline keeps the indent: got %q", d2.Text())
	}
	d3, _ := newTestDoc("{}")
	d3.SetCaret(1)
	if err := d3.Newline(4); err != nil {
		t.Fatal(err)
	}
	if d3.Text() != "{\n    \n}" || d3.Caret() != len("{\n    ") {
		t.Fatalf("newline between brackets gave %q caret %d", d3.Text(), d3.Caret())
	}
}

func TestMoveKeepsColumnAcrossShortLines(t *testing.T) {
	d, _ := newTestDoc("abcdef\nab\nabcdef")
	d.SetCaret(5) // column 5 of the first line
	d.Move(MoveDown, false, 1)
	if d.Caret() != 7+2 {
		t.Fatalf("down onto a short line goes to its end: caret %d, want 9", d.Caret())
	}
	d.Move(MoveDown, false, 1)
	if d.Caret() != 10+5 {
		t.Fatalf("the column is kept: caret %d, want 15", d.Caret())
	}
}

func TestSmartHome(t *testing.T) {
	d, _ := newTestDoc("    code")
	d.SetCaret(8)
	d.Move(MoveLineStart, false, 1)
	if d.Caret() != 4 {
		t.Fatalf("first press goes to the first non-blank: caret %d, want 4", d.Caret())
	}
	d.Move(MoveLineStart, false, 1)
	if d.Caret() != 0 {
		t.Fatalf("second press goes to column 0: caret %d", d.Caret())
	}
}

func TestWordMovementAndSelection(t *testing.T) {
	d, _ := newTestDoc("one two_3 + four")
	d.SetCaret(0)
	d.Move(MoveWordRight, false, 1)
	if d.Caret() != 3 {
		t.Fatalf("word right from 0 = %d, want 3", d.Caret())
	}
	d.Move(MoveWordRight, false, 1)
	if d.Caret() != 9 {
		t.Fatalf("word right over the word with a digit and underscore = %d, want 9", d.Caret())
	}
	d.Move(MoveWordRight, true, 1)
	if got := d.Primary(); got.Anchor != 9 || got.Head != 11 {
		t.Fatalf("select word right over punctuation: %+v", got)
	}
}

func TestSelectWord(t *testing.T) {
	d, _ := newTestDoc("alpha beta")
	d.SelectWord(7)
	if from, to := d.Primary().Range(); d.Text()[from:to] != "beta" {
		t.Fatalf("SelectWord(7) selected %q", d.Text()[from:to])
	}
	d.SelectWord(5) // the space after alpha
	if from, to := d.Primary().Range(); d.Text()[from:to] != " " {
		t.Fatalf("SelectWord on a space selected %q", d.Text()[from:to])
	}
}

func TestIndentAndOutdentRows(t *testing.T) {
	d, _ := newTestDoc("a\nb\nc")
	d.SetSelections(Selection{Anchor: 0, Head: 3})
	if err := d.Indent(4); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "    a\n    b\nc" {
		t.Fatalf("indent of two rows = %q", d.Text())
	}
	if err := d.Outdent(4); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "a\nb\nc" {
		t.Fatalf("outdent = %q", d.Text())
	}
}

func TestCaretIndentInsertsToTabStop(t *testing.T) {
	d, _ := newTestDoc("ab")
	d.SetCaret(1)
	if err := d.Indent(4); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "a   b" {
		t.Fatalf("indent at column 1 with tab stop 4 = %q, want three spaces", d.Text())
	}
}

func TestToggleComment(t *testing.T) {
	d, _ := newTestDoc("  x := 1\n  y := 2")
	d.SetSelections(Selection{Anchor: 0, Head: len(d.Text())})
	if err := d.ToggleComment("// "); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "  // x := 1\n  // y := 2" {
		t.Fatalf("comment = %q", d.Text())
	}
	if err := d.ToggleComment("// "); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "  x := 1\n  y := 2" {
		t.Fatalf("uncomment = %q", d.Text())
	}
	// A prefix without its space is removed too.
	d2, _ := newTestDoc("//x")
	d2.SetCaret(0)
	if err := d2.ToggleComment("// "); err != nil {
		t.Fatal(err)
	}
	if d2.Text() != "x" {
		t.Fatalf("uncomment of //x = %q", d2.Text())
	}
}

func TestAddNextOccurrenceAndVerticalCarets(t *testing.T) {
	d, _ := newTestDoc("foo bar foo\nfoo")
	d.SetCaret(1)
	d.AddNextOccurrence() // selects the word at the caret
	if from, to := d.Primary().Range(); d.Text()[from:to] != "foo" {
		t.Fatalf("first press selects the word, got %q", d.Text()[from:to])
	}
	d.AddNextOccurrence()
	d.AddNextOccurrence()
	if got := len(d.Selections()); got != 3 {
		t.Fatalf("three occurrences of foo selected, have %d", got)
	}
	d.Collapse()
	if got := len(d.Selections()); got != 1 {
		t.Fatalf("Collapse keeps one selection, have %d", got)
	}
	d.SetCaret(1)
	d.AddCaretVertical(1)
	if got := d.Selections(); len(got) != 2 || got[1].Head != 12+1 {
		t.Fatalf("a caret below at the same column: %+v", got)
	}
}

func TestMultiCursorDeleteAndPaste(t *testing.T) {
	d, _ := newTestDoc("ab\ncd")
	d.SetSelections(Selection{0, 1}, Selection{3, 4})
	if err := d.DeleteBackward(); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "b\nd" {
		t.Fatalf("deleting two selections = %q", d.Text())
	}
	d.SetSelections(Selection{0, 1}, Selection{2, 3})
	if got := d.Copy(); got != "b\nd" {
		t.Fatalf("copy of two selections = %q, want one line each", got)
	}
}

func TestMatchingBracket(t *testing.T) {
	d, _ := newTestDoc("f(a, (b))")
	q, ok := d.MatchingBracket(1)
	if !ok || q != 8 {
		t.Fatalf("bracket at 1 matches %d (%v), want 8", q, ok)
	}
	q, ok = d.MatchingBracket(9) // just after the last closer
	if !ok || q != 1 {
		t.Fatalf("bracket before the caret matches %d (%v), want 1", q, ok)
	}
}

// TestFindCountsAndOptions checks the matches of the find options, and an invalid pattern.
func TestFindCountsAndOptions(t *testing.T) {
	d, _ := newTestDoc("Foo foo foobar FOO")
	f, err := NewFinder("foo", FindOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(f.Matches(d)); got != 4 {
		t.Fatalf("case-insensitive matches = %d, want 4", got)
	}
	f, _ = NewFinder("foo", FindOptions{MatchCase: true})
	if got := len(f.Matches(d)); got != 2 {
		t.Fatalf("case-sensitive matches = %d, want 2", got)
	}
	f, _ = NewFinder("foo", FindOptions{WholeWord: true})
	if got := len(f.Matches(d)); got != 3 {
		t.Fatalf("whole word matches = %d, want 3 (not foobar)", got)
	}
	f, _ = NewFinder(`f[o]+`, FindOptions{Regex: true})
	if got := len(f.Matches(d)); got != 4 {
		t.Fatalf("regex matches = %d, want 4", got)
	}
	if _, err := NewFinder("(", FindOptions{Regex: true}); err == nil {
		t.Fatal("an invalid pattern gave no error")
	}
}

// TestReplaceAllIsOneStep checks that replacing every match is one undo step, with the
// groups of a regular expression expanded.
func TestReplaceAllIsOneStep(t *testing.T) {
	d, c := newTestDoc("a1 b2 c3")
	f, _ := NewFinder(`([a-z])(\d)`, FindOptions{Regex: true})
	c.t = c.t.Add(time.Minute)
	n, err := d.ReplaceMatches(f, f.Matches(d), "$2$1")
	if err != nil || n != 3 {
		t.Fatalf("ReplaceMatches = %d, %v", n, err)
	}
	if d.Text() != "1a 2b 3c" {
		t.Fatalf("replaced = %q", d.Text())
	}
	d.Undo()
	if d.Text() != "a1 b2 c3" {
		t.Fatalf("one undo of replace all = %q", d.Text())
	}
}

// TestFoldHidesRowsAndSkipsThem checks that a fold hides the indented lines, that the visible
// rows skip them, that the caret does not stop in them, and that unfolding brings them back.
func TestFoldHidesRowsAndSkipsThem(t *testing.T) {
	d, _ := newTestDoc("func a() {\n\tone\n\ttwo\n}\nnext")
	if !d.FoldableRow(0, 4) || d.FoldableRow(4, 4) {
		t.Fatal("row 0 should be foldable and row 4 should not")
	}
	if !d.Fold(0, 4) {
		t.Fatal("Fold(0) did nothing")
	}
	if got := d.VisibleRows(); got != 3 {
		t.Fatalf("visible rows after the fold = %d, want 3", got)
	}
	if r, _ := d.RowAtVisible(1); r != 3 {
		t.Fatalf("the second shown row is %d, want the closing brace at 3", r)
	}
	if d.VisibleIndex(2) != 0 {
		t.Fatalf("a hidden row has the index of the row above it: got %d", d.VisibleIndex(2))
	}
	d.SetCaret(d.LineStart(2)) // inside the fold
	if d.Caret() != d.LineEnd(0) {
		t.Fatalf("a caret inside a fold moves to the header's end: caret %d", d.Caret())
	}
	d.Move(MoveDown, false, 1)
	if d.RowOf(d.Caret()) != 3 {
		t.Fatalf("down from the header goes to the next shown row, got row %d", d.RowOf(d.Caret()))
	}
	if !d.Unfold(0) || d.VisibleRows() != 5 {
		t.Fatalf("Unfold did not restore the rows: visible %d", d.VisibleRows())
	}
}

// TestFoldFollowsEdits checks that a fold moves with the text above it, as typing a line above
// it does.
func TestFoldFollowsEdits(t *testing.T) {
	d, _ := newTestDoc("x\nfunc a() {\n\tone\n}")
	d.Fold(1, 4)
	if err := d.Replace(0, 0, "new line\n"); err != nil {
		t.Fatal(err)
	}
	// Five rows, one of them hidden, so four are shown.
	if !d.Folded(2) || d.VisibleRows() != 4 {
		t.Fatalf("after inserting a line above, the fold is on row 2 with %d visible rows", d.VisibleRows())
	}
}

// TestCellsOfTabsAndWideCharacters checks the cell grid that the pointer and the caret use.
func TestCellsOfTabsAndWideCharacters(t *testing.T) {
	line := "a\tb世c"
	if got := cellOfCol(line, 2, 4); got != 4 {
		t.Fatalf("cell after a tab = %d, want 4", got)
	}
	if got := cellOfCol(line, len("a\tb世"), 4); got != 7 {
		t.Fatalf("cell after a wide character = %d, want 7", got)
	}
	if got := colOfCell(line, 4, 4); got != 2 {
		t.Fatalf("byte offset of cell 4 = %d, want 2", got)
	}
	if got := colOfCell(line, 1, 4); got != 1 {
		t.Fatalf("cell 1 is inside the tab and goes to its start: %d", got)
	}
	if got := expandRun("a\tb", 0, 4); got != "a   b" {
		t.Fatalf("expandRun = %q", got)
	}
}

// TestCaptureStylesUseClosestName checks that the capture names of a highlighter take the theme's
// style of the closest name they extend.
func TestCaptureStylesUseClosestName(t *testing.T) {
	styles := map[string]ui.SyntaxStyle{"string": {Bold: true}, "string.escape": {Italic: true}}
	st, ok := captureStyles([]string{"string.escape.go", "string.raw", "keyword"}, styles)
	if !ok[0] || !st[0].Italic || !ok[1] || !st[1].Bold || ok[2] {
		t.Fatalf("capture styles = %+v %v", st, ok)
	}
}

// TestSpanIndexLaterSpansWin checks which span paints where spans overlap.
func TestSpanIndexLaterSpansWin(t *testing.T) {
	spans := []Span{{Start: 0, End: 10, Capture: 0}, {Start: 4, End: 6, Capture: 1}}
	var x spanIndex
	x.build(spans, 1)
	got := x.around(0, 10)
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("around = %v, want both in list order", got)
	}
	if got := x.around(20, 30); len(got) != 0 {
		t.Fatalf("around past the spans = %v", got)
	}
}

// TestCommandsOnEmptyDocument checks that the commands do nothing harmful with no text.
func TestCommandsOnEmptyDocument(t *testing.T) {
	d, _ := newTestDoc("")
	d.Move(MoveDown, false, 1)
	d.Move(MoveWordLeft, false, 1)
	if err := d.DeleteBackward(); err != nil {
		t.Fatal(err)
	}
	if err := d.ToggleComment("// "); err != nil {
		t.Fatal(err)
	}
	d.SelectWord(0)
	if d.Text() != "" {
		t.Fatal("an empty document changed")
	}
	// Tab on an empty line inserts the indent, as it does anywhere a caret has no selection.
	if err := d.Indent(4); err != nil {
		t.Fatal(err)
	}
	if d.Text() != "    " {
		t.Fatalf("Indent on an empty line = %q, want four spaces", d.Text())
	}
}

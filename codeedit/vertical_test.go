package codeedit

import "testing"

// A vertical move keeps the screen cell of the caret, so a tab before the caret on the row it
// starts from does not move the caret off its cell on the row below.
func TestVerticalMoveKeepsTheScreenCellAcrossATab(t *testing.T) {
	d := NewDocument("ab\tx\nabcdefgh")
	d.SetCaret(3) // before x, at cell 4 after the tab stop
	d.Move(MoveDown, false, 1)
	if got := d.Caret(); got != len("ab\tx\n")+4 {
		t.Fatalf("down from cell 4 went to offset %d, want the cell 4 of the next row", got)
	}

	// With tab stops of 8, the same caret is at cell 8, which is the end of the shorter row.
	d = NewDocument("ab\tx\nabcdefgh")
	d.SetTabSize(8)
	d.SetCaret(3)
	d.Move(MoveDown, false, 1)
	if got := d.Caret(); got != len("ab\tx\nabcdefgh") {
		t.Fatalf("down from cell 8 with tab width 8 went to offset %d, want the end of the row", got)
	}
}

// A character of two cells takes the cell of its first half, so a line of wide characters above an
// ASCII line keeps the screen column, and the other way back.
func TestVerticalMoveKeepsTheScreenColumnOfWideCharacters(t *testing.T) {
	d := NewDocument("abcdef\n世界x")
	d.SetCaret(4) // cell 4 of the ASCII row
	d.Move(MoveDown, false, 1)
	if got := d.Caret(); got != len("abcdef\n世界") {
		t.Fatalf("down from cell 4 went to offset %d, want the boundary after 界 at cell 4", got)
	}
	d.Move(MoveUp, false, 1)
	if got := d.Caret(); got != 4 {
		t.Fatalf("up from the boundary after 界 went to offset %d, want 4", got)
	}
}

// A run of vertical moves keeps the goal cell across a row that is too short for it.
func TestVerticalRunKeepsTheGoalCellAcrossAShortLine(t *testing.T) {
	d := NewDocument("abcdefgh\nab\nabcdefgh")
	d.SetCaret(6)
	d.Move(MoveDown, false, 1)
	if got := d.Caret(); got != len("abcdefgh\nab") {
		t.Fatalf("down onto the short row went to offset %d, want its end", got)
	}
	d.Move(MoveDown, false, 1)
	if got := d.Caret(); got != len("abcdefgh\nab\n")+6 {
		t.Fatalf("down past the short row went to offset %d, want cell 6 of the last row", got)
	}
}

// AddCaretVertical puts the new caret on the same screen cell, past a tab.
func TestAddCaretVerticalUsesScreenCells(t *testing.T) {
	d := NewDocument("ab\tx\nabcdefgh")
	d.SetCaret(3)
	d.AddCaretVertical(1)
	if got := d.Caret(); got != len("ab\tx\n")+4 {
		t.Fatalf("the added caret is at offset %d, want cell 4 of the next row", got)
	}
}

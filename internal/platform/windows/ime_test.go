//go:build windows

package windows

import (
	"testing"
	"unicode/utf16"
)

// A composition's UTF-16 caret and clause become byte offsets in its text,
// including past characters that take two units (an emoji) or three bytes.
func TestCompositionEvent(t *testing.T) {
	text := utf16.Encode([]rune("a😀にほん"))
	// a, the two units of the emoji, に, ほ (target), ん (target).
	attrs := []byte{0, 0, 0, 2, attrTargetConverted, attrTargetConverted}
	e := compositionEvent(text, attrs, 3) // before に
	if e.Text != "a😀にほん" {
		t.Fatalf("text %q", e.Text)
	}
	if want := len("a😀"); e.Cursor != want {
		t.Errorf("cursor %d, want %d", e.Cursor, want)
	}
	if s, end := len("a😀に"), len("a😀にほん"); e.ClauseStart != s || e.ClauseEnd != end {
		t.Errorf("clause %d..%d, want %d..%d", e.ClauseStart, e.ClauseEnd, s, end)
	}

	// A caret inside a surrogate pair, at the end, or past it.
	for _, c := range []struct{ cursor, want int }{{2, 1}, {6, len("a😀にほん")}, {99, len("a😀にほん")}} {
		if e := compositionEvent(text, nil, c.cursor); e.Cursor != c.want {
			t.Errorf("cursor %d became %d, want %d", c.cursor, e.Cursor, c.want)
		}
	}
	// No target: no clause. A lone surrogate becomes U+FFFD.
	e = compositionEvent([]uint16{'x', 0xD800}, []byte{0, 0}, 0)
	if e.ClauseStart != e.ClauseEnd || e.Text != "x�" {
		t.Errorf("clause %d..%d, text %q", e.ClauseStart, e.ClauseEnd, e.Text)
	}
	// Only the first target run is the clause.
	e = compositionEvent(utf16.Encode([]rune("abc")), []byte{attrTargetNotConverted, 0, attrTargetConverted}, 0)
	if e.ClauseStart != 0 || e.ClauseEnd != 1 {
		t.Errorf("clause %d..%d, want 0..1", e.ClauseStart, e.ClauseEnd)
	}
	// Empty: the composition ended.
	if e := compositionEvent(nil, nil, 0); e.Text != "" || e.Cursor != 0 {
		t.Errorf("empty composition gave %+v", e)
	}
}

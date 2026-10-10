package text

import (
	"strings"
	"testing"
)

// notepadUnits is the word units of a text in Notepad, read through UI Automation
// and the keyboard (Ctrl+Right stops and Move(Word, 1) landed on the same starts).
// The text is "A small note-taking app. Next line  \rSecond line, ok.", where \r is
// the line break Notepad stores.
var notepadUnits = []string{
	"A ", "small ", "note", "-", "taking ", "app", ". ", "Next ",
	"line  ", "\r", "Second ", "line", ", ", "ok", ".",
}

// unitsOf splits s at the starts of its word units.
func unitsOf(s string) []string {
	starts := WordStarts(s)
	var out []string
	for i, a := range starts {
		b := len(s)
		if i+1 < len(starts) {
			b = starts[i+1]
		}
		out = append(out, s[a:b])
	}
	return out
}

func TestWordUnitsMatchNotepad(t *testing.T) {
	text := strings.Join(notepadUnits, "")
	got := unitsOf(text)
	if strings.Join(got, "|") != strings.Join(notepadUnits, "|") {
		t.Errorf("units of %q\n got %q\nwant %q", text, got, notepadUnits)
	}
}

func TestWordUnitRules(t *testing.T) {
	cases := []struct {
		s    string
		want []string
	}{
		{"", nil},
		{"one two", []string{"one ", "two"}},
		{"   x", []string{"   ", "x"}},
		{"foo   bar", []string{"foo   ", "bar"}},
		{"a\r\nb", []string{"a", "\r\n", "b"}},
		{"x\n\ny", []string{"x", "\n", "\n", "y"}},
		{"end.", []string{"end", "."}},
		{"a_1 b-2", []string{"a_1 ", "b", "-", "2"}},
	}
	for _, tc := range cases {
		if got := unitsOf(tc.s); strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("units of %q = %q, want %q", tc.s, got, tc.want)
		}
	}
}

// TestWordStopsAreUnitBoundaries checks Ctrl+Right and Ctrl+Left at every position.
// From inside a unit, or at its start, Ctrl+Right goes to the unit's end; Ctrl+Left
// goes to the start of the unit holding the character before the caret.
func TestWordStopsAreUnitBoundaries(t *testing.T) {
	for _, s := range []string{strings.Join(notepadUnits, ""), "one two", "", "x\r\ny"} {
		starts := WordStarts(s)
		unitStart := make([]int, len(s)+1)
		unitEnd := make([]int, len(s)+1)
		for i, a := range starts {
			b := len(s)
			if i+1 < len(starts) {
				b = starts[i+1]
			}
			for p := a; p < b; p++ {
				unitStart[p], unitEnd[p] = a, b
			}
		}
		unitStart[len(s)], unitEnd[len(s)] = len(s), len(s)
		for pos := 0; pos <= len(s); pos++ {
			wantNext := len(s)
			if pos < len(s) {
				wantNext = unitEnd[pos]
			}
			if got := NextWordStart(s, pos); got != wantNext {
				t.Errorf("NextWordStart(%q, %d) = %d, want %d", s, pos, got, wantNext)
			}
			wantPrev := 0
			if pos > 0 {
				wantPrev = unitStart[pos-1]
			}
			if got := PrevWordStart(s, pos); got != wantPrev {
				t.Errorf("PrevWordStart(%q, %d) = %d, want %d", s, pos, got, wantPrev)
			}
		}
	}
}

// TestNotepadKeyStops is the Ctrl+Right sequence Notepad showed from the start of
// its text, as byte offsets into notepadUnits joined.
func TestNotepadKeyStops(t *testing.T) {
	s := strings.Join(notepadUnits, "")
	want := []int{2, 8, 12, 13, 20, 23, 25, 30, 36, 37, 44, 48, 50, 52, 53}
	pos := 0
	for i, w := range want {
		pos = NextWordStart(s, pos)
		if pos != w {
			t.Fatalf("Ctrl+Right stop %d = %d, want %d", i, pos, w)
		}
	}
	// The end-of-word convention, which macOS and Linux use, stops at the end of
	// each word instead.
	if got := NextWord("A small", 0); got != 1 {
		t.Errorf("NextWord from 0 = %d, want 1 (the end of A)", got)
	}
}

package codeedit

import (
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// Diagnostics follow the text: an insertion before one moves it, typing at either end does not
// grow it, and a deletion over it shrinks it or leaves it empty at the start of the deletion.
func TestDiagnosticsMapThroughEdits(t *testing.T) {
	d := NewDocument("alpha beta gamma")
	d.SetDiagnostics([]Diagnostic{{From: 6, To: 10, Message: "beta"}, {From: 11, To: 11, Message: "empty"}})
	check := func(when string, want ...[2]int) {
		t.Helper()
		got := d.Diagnostics()
		if len(got) != len(want) {
			t.Fatalf("%s: %d diagnostics, want %d", when, len(got), len(want))
		}
		for i, w := range want {
			if got[i].From != w[0] || got[i].To != w[1] {
				t.Errorf("%s: diagnostic %d is [%d, %d), want [%d, %d)", when, i, got[i].From, got[i].To, w[0], w[1])
			}
		}
	}
	_ = d.Replace(0, 0, "xx")
	check("after an insertion before", [2]int{8, 12}, [2]int{13, 13})
	_ = d.Replace(8, 8, "y")
	_ = d.Replace(13, 13, "z")
	check("after typing at both ends", [2]int{9, 13}, [2]int{15, 15})
	_ = d.Replace(7, 11, "")
	check("after a deletion over the start", [2]int{7, 9}, [2]int{11, 11})
	_ = d.Replace(6, 10, "Q")
	check("after a replacement over the whole range", [2]int{6, 6}, [2]int{8, 8})
	d.Undo()
	if got := d.Diagnostics(); len(got) != 2 || got[1].To > d.Len() {
		t.Errorf("after undo the diagnostics are %+v, want both, in the text", got)
	}
	d.SetText("new")
	check("after SetText")
}

// SetDiagnosticsFor drops a result computed for an older text, but not one computed before a
// move of the caret.
func TestSetDiagnosticsForDropsAStaleSet(t *testing.T) {
	d := NewDocument("abc")
	v := d.TextVersion()
	d.SetCaret(2)
	if !d.SetDiagnosticsFor(v, []Diagnostic{{From: 0, To: 1}}) || len(d.Diagnostics()) != 1 {
		t.Fatal("a set for the current text was dropped after a caret move")
	}
	_ = d.Replace(0, 0, "x")
	if d.SetDiagnosticsFor(v, []Diagnostic{{From: 0, To: 2}, {From: 1, To: 2}}) {
		t.Fatal("a set for an older text was taken")
	}
	if got := d.Diagnostics(); len(got) != 1 || got[0].From != 1 {
		t.Errorf("the diagnostics are %+v, want the old one moved by the edit", got)
	}
	if got := d.Diagnostics()[0].Severity; got != SeverityError {
		t.Errorf("a zero severity is %d, want SeverityError", got)
	}
}

// DiagnosticsAt finds the diagnostics around an offset, ends included.
func TestDiagnosticsAt(t *testing.T) {
	d := NewDocument("one two three")
	d.SetDiagnostics([]Diagnostic{{From: 4, To: 7, Message: "a"}, {From: 0, To: 13, Message: "b"}})
	if got := d.DiagnosticsAt(7); len(got) != 2 {
		t.Errorf("at the end of a range: %d diagnostics, want 2", len(got))
	}
	if got := d.DiagnosticsAt(9); len(got) != 1 || got[0].Message != "b" {
		t.Errorf("outside the first range: %+v, want the second", got)
	}
}

// The UTF-16 positions count the code units of the row, with a character outside the BMP as two.
func TestUTF16Positions(t *testing.T) {
	d := NewDocument("ab\n😀é x\n")
	x := strings.Index(d.Text(), "x")
	if row, col := d.UTF16Pos(x); row != 1 || col != 4 {
		t.Errorf("UTF16Pos(x) = %d, %d, want 1, 4", row, col)
	}
	if got := d.OffsetUTF16(1, 4); got != x {
		t.Errorf("OffsetUTF16(1, 4) = %d, want %d", got, x)
	}
	if got := d.OffsetUTF16(1, 1); got != d.LineStart(1) {
		t.Errorf("a column inside a surrogate pair = %d, want the start of its character %d", got, d.LineStart(1))
	}
	if got := d.OffsetUTF16(0, 99); got != 2 {
		t.Errorf("a column past the row = %d, want its end 2", got)
	}
	if got := d.OffsetUTF16(99, 0); got != d.Len() {
		t.Errorf("a row past the text = %d, want the last row's start %d", got, d.Len())
	}
}

// announcements returns the texts announced in the updates.
func announcements(us []access.Update) []string {
	var out []string
	for _, u := range us {
		for _, a := range u.Announcements {
			out = append(out, a.Text)
		}
	}
	return out
}

// F8 and Shift+F8 select the next and the previous diagnostic, wrapping around, and announce it.
func TestNextDiagnosticWrapsAndAnnounces(t *testing.T) {
	h := newHarness(t, "one\ntwo\nthree\n")
	updates := activateAccess(h)
	h.doc.SetDiagnostics([]Diagnostic{
		{From: 4, To: 7, Severity: SeverityWarning, Message: "unused"},
		{From: 8, To: 13, Message: "undefined: three"},
	})
	h.doc.SetCaret(0)
	h.frame()
	h.key(input.KeyF8, 0)
	if p := h.doc.Primary(); p.Anchor != 4 || p.Head != 7 {
		t.Fatalf("F8 selected %+v, want the first diagnostic", p)
	}
	h.key(input.KeyF8, 0)
	h.key(input.KeyF8, 0)
	if p := h.doc.Primary(); p.Anchor != 4 {
		t.Fatalf("F8 past the last diagnostic selected %+v, want the first again", p)
	}
	h.key(input.KeyF8, input.ModShift)
	if p := h.doc.Primary(); p.Anchor != 8 || p.Head != 13 {
		t.Fatalf("Shift+F8 selected %+v, want the last diagnostic", p)
	}
	h.frame()
	got := announcements(*updates) // the queue is the process's, so earlier tests may have left some
	if !slices.Contains(got, "Warning on line 2: unused") || got[len(got)-1] != "Error on line 3: undefined: three" {
		t.Errorf("the announcements are %q", got)
	}
}

// The marks of the diagnostics take the theme's colors, and the system colors under high contrast.
func TestDiagnosticColorsFollowTheTheme(t *testing.T) {
	for _, th := range []*ui.Theme{ui.DarkTheme(), ui.LightTheme()} {
		st, used := resolvedIn(Style{}, th, false)
		if st.DiagnosticError.TextColor != used.Danger || st.DiagnosticWarning.TextColor != used.Warning ||
			st.DiagnosticInfo.TextColor != used.Accent || st.DiagnosticHint.TextColor != used.TextMuted {
			t.Errorf("%s: the diagnostic colors are %v %v %v %v", used.Name, st.DiagnosticError.TextColor,
				st.DiagnosticWarning.TextColor, st.DiagnosticInfo.TextColor, st.DiagnosticHint.TextColor)
		}
	}
	red := ui.Hex("#ff0000")
	st, used := resolvedIn(Style{DiagnosticError: ui.Style{TextColor: red}}, ui.DarkTheme(), true)
	if st.DiagnosticError.TextColor == red || st.DiagnosticError.TextColor != used.Danger || st.DiagnosticInfo.TextColor != used.Accent {
		t.Errorf("under high contrast the error mark is %v and the information mark %v, want the system colors %v and %v",
			st.DiagnosticError.TextColor, st.DiagnosticInfo.TextColor, used.Danger, used.Accent)
	}
}

// A diagnostic shows on each display row it covers, and an empty one on the row of its character.
func TestDiagnosticOnRows(t *testing.T) {
	h := newHarness(t, "aaaa bbbb cccc\ndd", wrapAt(5))
	vs := h.ed.view
	rowsWith := func(g Diagnostic) int {
		n := 0
		for i := 0; i < vs.displayRows(); i++ {
			dr, _ := vs.displayAt(i)
			if g.onRow(dr, h.doc.LineEnd(dr.row)) {
				n++
			}
		}
		return n
	}
	for _, c := range []struct {
		g    Diagnostic
		want int
	}{
		{Diagnostic{From: 2, To: 7}, 2},
		{Diagnostic{From: 7, To: 7}, 1},
		{Diagnostic{From: 14, To: 14}, 1}, // at the end of the line
		{Diagnostic{From: 0, To: 17}, 4},
	} {
		if got := rowsWith(c.g); got != c.want {
			t.Errorf("[%d, %d) is on %d display rows, want %d", c.g.From, c.g.To, got, c.want)
		}
	}
}

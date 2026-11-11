package markdown

import (
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// describe lists the marks of v as kind:text, in order of where they start: hide
// for syntax that hides, syn for syntax that stays, and b, i, s, code, link and h
// for the looks.
func describe(v string, marks []ui.TextMark, st EditStyle) []string {
	ms := slices.Clone(marks)
	slices.SortStableFunc(ms, func(a, b ui.TextMark) int { return a.Start - b.Start })
	var out []string
	for _, m := range ms {
		kind := "?"
		switch {
		case m.Syntax:
			kind = "hide"
		case m.Style == st.Syntax:
			kind = "syn"
		case m.Style == st.Code:
			kind = "code"
		case m.Style == st.Link:
			kind = "link"
		case m.Style == st.Heading:
			kind = "h"
		case m.Style.Weight == ui.FontBold:
			kind = "b"
		case m.Style.Italic:
			kind = "i"
		case m.Style.Strikethrough:
			kind = "s"
		}
		out = append(out, kind+":"+v[m.Start:m.End])
	}
	return out
}

func TestEditMarks(t *testing.T) {
	st := EditStyle{
		Syntax:  ui.Highlight{Color: ui.Hex("#888888")},
		Code:    ui.Highlight{Color: ui.Hex("#ff0000"), Mono: true},
		Link:    ui.Highlight{Color: ui.Hex("#0000ff")},
		Heading: ui.Highlight{Weight: ui.FontSemibold},
	}
	marks := EditMarks(st)
	cases := []struct {
		in   string
		want string
	}{
		{"plain text", ""},
		{"a **bold** b", "hide:** b:bold hide:**"},
		{"a __bold__ b", "hide:__ b:bold hide:__"},
		{"*it* and _it_", "hide:* i:it hide:* hide:_ i:it hide:_"},
		{"***both***", "hide:* hide:** i:**both** b:both hide:** hide:*"},
		{"~~gone~~", "hide:~~ s:gone hide:~~"},
		{"use `a**b**c` here", "hide:` code:a**b**c hide:`"},
		{"``a ` b``", "hide:`` code:a ` b hide:``"},
		{"see [docs](https://x.y/z) now", "hide:[ link:docs hide:](https://x.y/z)"},
		{"![img](a.png)", ""},
		{"[**bold link**](u)", "hide:[ link:**bold link** hide:** b:bold link hide:** hide:](u)"},
		{`\*not\*`, ""},
		{"snake_case_name", ""},
		{"2 * 3 * 4", ""},
		{"**open only", ""},
		{"**a\nb**", ""},
		{"# Title *x*", "syn:#  h:Title *x* hide:* i:x hide:*"},
		{"#hashtag", ""},
		{"```go\nx := `a`\n```\n**b**", "syn:```go code:x := `a` syn:``` hide:** b:b hide:**"},
		{"**Skip writes.** Right now `Signal.Set` re-renders",
			"hide:** b:Skip writes. hide:** hide:` code:Signal.Set hide:`"},
	}
	for _, c := range cases {
		got := strings.Join(describe(c.in, marks(c.in), st), " ")
		if got != c.want {
			t.Errorf("%q:\n got %s\nwant %s", c.in, got, c.want)
		}
	}
}

// The syntax of a construct shows with the whole construct: the scope of each
// run of stars is all of it, and the inner runs of ***both*** show with the outer.
func TestEditMarksScopes(t *testing.T) {
	v := "x ***both*** y"
	for _, m := range EditMarks(EditStyle{})(v) {
		if m.Syntax && (m.ScopeStart != 2 || m.ScopeEnd != 12) {
			t.Errorf("syntax %q has scope %d..%d, want 2..12", v[m.Start:m.End], m.ScopeStart, m.ScopeEnd)
		}
	}
	v = "[a](b) `c`"
	for _, m := range EditMarks(EditStyle{})(v) {
		if !m.Syntax {
			continue
		}
		scope := v[m.ScopeStart:m.ScopeEnd]
		if scope != "[a](b)" && scope != "`c`" {
			t.Errorf("syntax %q has scope %q", v[m.Start:m.End], scope)
		}
	}
}

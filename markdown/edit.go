package markdown

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/ui"
)

// EditStyle is how EditMarks styles Markdown as it is typed. A zero field takes
// its default.
type EditStyle struct {
	// Syntax is the look of the syntax while it shows, such as the ** of bold
	// text, a heading's # and a code fence. By default it looks as the text does.
	Syntax ui.Highlight
	// Code is the look of inline code and of code blocks, which are always in the
	// monospace font.
	Code ui.Highlight
	// Link is the look of a link's text. By default it is underlined.
	Link ui.Highlight
	// Heading is the look of a heading's text. By default it is bold.
	Heading ui.Highlight
}

// EditMarks returns marks for ui.TextAreaElement.Marks that show Markdown
// formatted as it is typed: bold, italic, strikethrough, inline code and links,
// whose syntax hides until the caret comes to it, and headings and code blocks,
// whose syntax stays in view.
//
//	ui.TextArea(draft).Marks(markdown.EditMarks(markdown.EditStyle{Syntax: ui.Highlight{Color: muted}}))
//
// It reads each line on its own, as an editor must for text that is still being
// written: emphasis and inline code do not go on past the end of a line.
func EditMarks(st EditStyle) func(string) []ui.TextMark {
	if st.Link == (ui.Highlight{}) {
		st.Link = ui.Highlight{Underline: true}
	}
	if st.Heading == (ui.Highlight{}) {
		st.Heading = ui.Highlight{Weight: ui.FontBold}
	}
	st.Code.Mono = true
	return func(v string) []ui.TextMark { return editMarks(v, st) }
}

func editMarks(v string, st EditStyle) []ui.TextMark {
	s := &markScan{st: st}
	fence := ""
	for start := 0; start <= len(v); {
		end := strings.IndexByte(v[start:], '\n')
		if end < 0 {
			end = len(v)
		} else {
			end += start
		}
		line := v[start:end]
		body := strings.TrimLeft(line, " ")
		indent := len(line) - len(body)
		switch {
		case fence != "":
			if indent <= 3 && strings.HasPrefix(body, fence) && strings.Trim(body, fence[:1]+" \t") == "" {
				s.style(start, end, st.Syntax)
				fence = ""
			} else {
				s.style(start, end, st.Code)
			}
		case indent <= 3 && (strings.HasPrefix(body, "```") || strings.HasPrefix(body, "~~~")):
			fence = body[:len(body)-len(strings.TrimLeft(body, body[:1]))]
			s.style(start, end, st.Syntax)
		default:
			if n := headingMarker(body); indent <= 3 && n > 0 {
				s.style(start, start+indent+n, st.Syntax)
				s.style(start+indent+n, end, st.Heading)
				s.inline(v, start+indent+n, end)
			} else {
				s.inline(v, start, end)
			}
		}
		if end == len(v) {
			break
		}
		start = end + 1
	}
	return s.marks
}

// headingMarker returns the length of the ATX heading marker that s starts with,
// its #s and the space after them, or 0 if it starts with none.
func headingMarker(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	switch {
	case n == 0 || n > 6:
		return 0
	case n == len(s):
		return n
	case s[n] == ' ' || s[n] == '\t':
		return n + 1
	}
	return 0
}

type markScan struct {
	st    EditStyle
	marks []ui.TextMark
}

// style marks the text from a to b with h, if h is a look at all.
func (s *markScan) style(a, b int, h ui.Highlight) {
	if b > a && h != (ui.Highlight{}) {
		s.marks = append(s.marks, ui.TextMark{Start: a, End: b, Style: h})
	}
}

// syntax marks the text from a to b as the syntax of the construct from scopeA to scopeB.
func (s *markScan) syntax(a, b, scopeA, scopeB int) {
	if b > a {
		s.marks = append(s.marks, ui.TextMark{Start: a, End: b, Style: s.st.Syntax, Syntax: true, ScopeStart: scopeA, ScopeEnd: scopeB})
	}
}

// delim is a run of delimiters of emphasis: * or _ (emphasis and strong
// emphasis) or ~ (strikethrough), from start to end.
type delim struct {
	c                  byte
	start, end         int
	canOpen, canClose  bool
	pairStart, pairEnd int // the marks of the last match of this run as an opener (see inline)
}

// inline marks the inline syntax of the line of v from a to b: code spans first,
// then links, then emphasis in what is left.
func (s *markScan) inline(v string, a, b int) {
	taken := make([]bool, b-a) // bytes that are syntax, code or escaped
	for i := a; i < b; {
		switch c := v[i]; {
		case c == '\\' && i+1 < b && isASCIIPunct(v[i+1]):
			taken[i-a], taken[i+1-a] = true, true
			i += 2
		case c == '`':
			n := run(v, i, b, '`')
			if close := closingTicks(v, i+n, b, n); close >= 0 {
				s.syntax(i, i+n, i, close+n)
				s.style(i+n, close, s.st.Code)
				s.syntax(close, close+n, i, close+n)
				for j := i; j < close+n; j++ {
					taken[j-a] = true
				}
				i = close + n
			} else {
				i += n
			}
		default:
			i++
		}
	}
	s.links(v, a, b, taken)
	s.emphasis(v, a, b, taken)
}

// run returns the length of the run of c at i, up to b.
func run(v string, i, b int, c byte) int {
	n := 0
	for i+n < b && v[i+n] == c {
		n++
	}
	return n
}

// closingTicks returns where the run of exactly n backticks that closes a code
// span starts, from i up to b, or -1.
func closingTicks(v string, i, b, n int) int {
	for i < b {
		if v[i] != '`' {
			i++
			continue
		}
		m := run(v, i, b, '`')
		if m == n {
			return i
		}
		i += m
	}
	return -1
}

// links marks the links of the line from a to b: "[text](url)", whose text is
// styled and whose brackets and URL are syntax. Images, "![alt](src)", stay as
// they are.
func (s *markScan) links(v string, a, b int, taken []bool) {
	free := func(i int) bool { return !taken[i-a] }
	for i := a; i < b; i++ {
		if v[i] != '[' || !free(i) || (i > a && v[i-1] == '!' && free(i-1)) {
			continue
		}
		close := -1
		for j := i + 1; j < b; j++ {
			if v[j] == ']' && free(j) {
				close = j
				break
			}
			if v[j] == '[' && free(j) {
				break
			}
		}
		if close < 0 || close == i+1 || close+1 >= b || v[close+1] != '(' {
			continue
		}
		end := -1
		for j := close + 2; j < b; j++ {
			if v[j] == ')' && free(j) {
				end = j + 1
				break
			}
			if v[j] == ' ' || !free(j) {
				break
			}
		}
		if end < 0 {
			continue
		}
		s.syntax(i, i+1, i, end)
		s.style(i+1, close, s.st.Link)
		s.syntax(close, end, i, end)
		taken[i-a] = true
		for j := close; j < end; j++ {
			taken[j-a] = true
		}
		i = end - 1
	}
}

// emphasis marks the emphasis, strong emphasis and strikethrough of the line
// from a to b, matching its runs of delimiters as CommonMark does, without its
// rule of three.
func (s *markScan) emphasis(v string, a, b int, taken []bool) {
	var runs []*delim
	for i := a; i < b; {
		c := v[i]
		if (c != '*' && c != '_' && c != '~') || taken[i-a] {
			i++
			continue
		}
		j := i
		for j < b && v[j] == c && !taken[j-a] {
			j++
		}
		d := &delim{c: c, start: i, end: j, pairStart: -1}
		before, after := ' ', ' '
		if i > a {
			before, _ = utf8.DecodeLastRuneInString(v[a:i])
		}
		if j < b {
			after, _ = utf8.DecodeRuneInString(v[j:b])
		}
		left := !unicode.IsSpace(after) && (!isPunct(after) || unicode.IsSpace(before) || isPunct(before))
		right := !unicode.IsSpace(before) && (!isPunct(before) || unicode.IsSpace(after) || isPunct(after))
		switch c {
		case '_':
			d.canOpen = left && (!right || isPunct(before))
			d.canClose = right && (!left || isPunct(after))
		case '~':
			d.canOpen, d.canClose = left && j-i == 2, right && j-i == 2
		default:
			d.canOpen, d.canClose = left, right
		}
		runs = append(runs, d)
		i = j
	}
	var stack []*delim
	for _, d := range runs {
		if d.canClose {
			for d.end > d.start {
				k := len(stack) - 1
				for k >= 0 && stack[k].c != d.c {
					k--
				}
				if k < 0 {
					break
				}
				o := stack[k]
				use := 1
				if o.end-o.start >= 2 && d.end-d.start >= 2 {
					use = 2
				}
				h := ui.Highlight{Italic: true}
				switch {
				case d.c == '~':
					h = ui.Highlight{Strikethrough: true}
				case use == 2:
					h = ui.Highlight{Weight: ui.FontBold}
				}
				scopeA, scopeB := o.end-use, d.start+use
				if o.pairStart >= 0 && o.pairEnd == d.start {
					// The rest of the same two runs, as in ***both***: the
					// syntax inside shows and hides with this.
					for m := range s.marks[o.pairStart:] {
						mk := &s.marks[o.pairStart+m]
						if mk.Syntax {
							mk.ScopeStart, mk.ScopeEnd = scopeA, scopeB
						}
					}
				}
				first := len(s.marks)
				s.syntax(scopeA, o.end, scopeA, scopeB)
				s.style(o.end, d.start, h)
				s.syntax(d.start, scopeB, scopeA, scopeB)
				o.end -= use
				d.start += use
				o.pairStart, o.pairEnd = first, d.start
				stack = stack[:k+1]
				if o.end == o.start {
					stack = stack[:k]
				}
			}
		}
		if d.canOpen && d.end > d.start {
			stack = append(stack, d)
		}
	}
}

func isASCIIPunct(c byte) bool {
	return c < utf8.RuneSelf && isPunct(rune(c))
}

// isPunct reports whether r is punctuation as CommonMark has it: a Unicode
// punctuation or symbol character.
func isPunct(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }

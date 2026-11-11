package ui

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/internal/text"
)

// TextMark styles a range of a text area's value as it is edited (see
// TextAreaElement.Marks), such as the bold of "**bold**" in markdown.
type TextMark struct {
	// Start and End are the byte offsets of the range in the value. A range not
	// on rune boundaries is moved back to them.
	Start, End int
	// Style is how the range looks. Marks over the same text combine, later
	// ones over earlier ones.
	Style Highlight
	// Syntax makes the range syntax rather than text, such as the two ** on
	// either side of bold text. Syntax shows only while the field has focus and
	// its caret or selection touches the range from ScopeStart to ScopeEnd
	// (either end included), and is hidden otherwise, so text reads as
	// formatted until the caret comes to edit it.
	Syntax bool
	// ScopeStart and ScopeEnd are the construct a Syntax mark belongs to, such
	// as all of "**bold**".
	ScopeStart, ScopeEnd int
}

// cleanMarks returns marks with their ranges inside v and on rune boundaries,
// and leaves out empty ones.
func cleanMarks(marks []TextMark, v string) []TextMark {
	out := marks[:0:0]
	for _, m := range marks {
		m.Start, m.End = clampToRune(v, m.Start), clampToRune(v, m.End)
		if m.End <= m.Start {
			continue
		}
		if m.Syntax {
			m.ScopeStart = min(clampToRune(v, m.ScopeStart), m.Start)
			m.ScopeEnd = max(clampToRune(v, m.ScopeEnd), m.End)
		}
		out = append(out, m)
	}
	return out
}

// marksOf returns the marks of value v, asking the field once per value it is
// built with.
func (es *editState) marksOf(e *editorElement, v string) []TextMark {
	if !es.marksSet || es.marksV != v {
		es.marks = cleanMarks(e.marks(v), v)
		es.marksV, es.marksSet = v, true
	}
	return es.marks
}

// hiddenSyntax returns the ranges of the Syntax marks that are hidden while the
// field has focus (focused) or not, with its selection from a to b: sorted and
// merged.
func hiddenSyntax(marks []TextMark, focused bool, a, b int) [][2]int {
	var hide [][2]int
	for _, m := range marks {
		if m.Syntax && !(focused && a <= m.ScopeEnd && b >= m.ScopeStart) {
			hide = append(hide, [2]int{m.Start, m.End})
		}
	}
	if len(hide) < 2 {
		return hide
	}
	slices.SortFunc(hide, func(x, y [2]int) int { return x[0] - y[0] })
	out := hide[:1]
	for _, r := range hide[1:] {
		if last := &out[len(out)-1]; r[0] <= last[1] {
			last[1] = max(last[1], r[1])
		} else {
			out = append(out, r)
		}
	}
	return out
}

// presentDrawn returns what the editor of n draws for its value v: the text as
// present gives it, or, for a field with marks, v without its hidden syntax and
// with the spans of its marks.
func (rt *Runtime) presentDrawn(n *node, v string) presented {
	e := n.editor
	if e.marks == nil || e.hidden() || e.format != nil || n.st == nil || n.st.edit == nil {
		return rt.present(e, v)
	}
	es := n.st.edit
	a, b := es.selection()
	hide := hiddenSyntax(es.marksOf(e, v), rt.focusID == n.id, a, b)
	es.hid = hide
	return presentMarks(v, es.marks, hide)
}

// marksChanged notes a change of what a field with marks hides, after its caret,
// selection or focus moved: the field lays out again for it.
func (rt *Runtime) marksChanged(n *node) {
	e := n.editor
	if e == nil || e.marks == nil || n.st == nil || n.st.edit == nil {
		return
	}
	es := n.st.edit
	v := e.value.Peek()
	a, b := es.selection()
	if hide := hiddenSyntax(es.marksOf(e, v), rt.focusID == n.id, a, b); !slices.Equal(hide, es.hid) {
		rt.invalidateLayout(n)
	}
}

// presentMarks returns v as drawn with the ranges hide left out, and the spans of
// marks in the text drawn.
func presentMarks(v string, marks []TextMark, hide [][2]int) presented {
	p := presented{shown: v}
	if len(hide) > 0 {
		var sb strings.Builder
		sb.Grow(len(v))
		p.raw = make([]int, 0, len(v)+1)
		p.at = make([]int, 0, len(v)+1)
		p.hides = true
		h := 0
		for i := 0; ; {
			p.raw = append(p.raw, i)
			p.at = append(p.at, sb.Len())
			if i == len(v) {
				break
			}
			for h < len(hide) && hide[h][1] <= i {
				h++
			}
			_, size := utf8.DecodeRuneInString(v[i:])
			if h == len(hide) || i < hide[h][0] {
				sb.WriteString(v[i : i+size])
			}
			i += size
		}
		p.shown = sb.String()
	}
	if len(marks) == 0 {
		return p
	}
	bounds := []int{0, len(v)}
	for _, m := range marks {
		bounds = append(bounds, m.Start, m.End)
	}
	for _, r := range hide {
		bounds = append(bounds, r[0], r[1])
	}
	slices.Sort(bounds)
	bounds = slices.Compact(bounds)
	removed, h := 0, 0
	for i := 0; i+1 < len(bounds); i++ {
		x, y := bounds[i], bounds[i+1]
		for h < len(hide) && hide[h][1] <= x {
			h++
		}
		if h < len(hide) && hide[h][0] <= x {
			removed += y - x // a piece is all hidden or not at all
			continue
		}
		var hl Highlight
		for _, m := range marks {
			if m.Start <= x && y <= m.End {
				hl = hl.over(m.Style)
			}
		}
		if hl == (Highlight{}) {
			continue
		}
		s, t := x-removed, y-removed
		if k := len(p.spans) - 1; k >= 0 && p.paint[k] == hl && p.spans[k].End == s {
			p.spans[k].End = t
			continue
		}
		sp := text.Span{Start: s, End: t, Weight: hl.Weight, Italic: hl.Italic}
		if hl.Mono {
			sp.Family = text.FamilyMono
		}
		p.spans = append(p.spans, sp)
		p.paint = append(p.paint, hl)
	}
	return p
}

// shiftSpans returns spans with n bytes inserted at offset at, as an input
// method's composition is: a span that goes on past at takes them in.
func shiftSpans(spans []text.Span, at, n int) []text.Span {
	if len(spans) == 0 || n == 0 {
		return spans
	}
	out := slices.Clone(spans)
	for i := range out {
		if out[i].Start >= at {
			out[i].Start += n
		}
		if out[i].End > at {
			out[i].End += n
		}
	}
	return out
}

// runPaint returns how the editor paints span i of the text it draws: in its
// mark's style, over the text color fg.
func (p *presented) runPaint(i int, fg Color, opacity float32) text.RunPaint {
	if i < 0 || i >= len(p.paint) {
		return text.RunPaint{Color: fg.Alpha(opacity).scene()}
	}
	hl := p.paint[i]
	c := hl.Color
	if c.IsZero() {
		c = fg
	}
	return text.RunPaint{
		Color:      c.Alpha(opacity).scene(),
		Background: hl.Background.Alpha(opacity).scene(),
		Underline:  hl.Underline,
		Strike:     hl.Strikethrough,
	}
}

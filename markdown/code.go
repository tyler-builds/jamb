package markdown

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/text"

	"github.com/tyler-builds/jamb/ui"
)

// CodeHighlighter colors the code blocks of a view (see ViewElement.Code). The mdcode
// package provides one built on the tree-sitter grammars of jamb's code editor; an app
// can write its own.
type CodeHighlighter interface {
	// HighlightCode returns the colored ranges of code, which is written in the
	// language lang: the first word of the code block's fence, as it is written, such
	// as "go" or "Python", or "" for a block without one. It returns false when it has
	// no ranges for the code: when it does not know the language, or while it is still
	// working on them. In the second case it calls ready once they are done, from any
	// goroutine, and the view asks again. The view calls it while it renders, on the UI
	// thread, so it must return at once.
	//
	// ctx is done when the view no longer shows the block: when the view goes away, or
	// when its text changes and the new text does not ask for the same code again. The
	// highlighter may then stop the work for the block and not call ready.
	HighlightCode(ctx context.Context, lang, code string, ready func()) ([]CodeSpan, bool)
}

// CodeSpan is a range of code, [Start, End) in bytes, and its syntax capture: a
// tree-sitter capture name, such as "keyword" or "string.escape". The view colors it
// with the theme's Syntax style for the name, or for the name without its last dotted
// parts when the theme has no style for it, as the code editor does. A span later in
// the list wins over an earlier one where they overlap.
type CodeSpan struct {
	// Start and End are the byte offsets of the range in the code.
	Start, End int
	// Capture is the name of the range's syntax capture.
	Capture string
}

// codeBlock returns a code block: its text, selectable and on lines that do not wrap,
// in a box that scrolls sideways when a line is too long for it.
func (b *builder) codeBlock(lines *text.Segments, lang string) ui.Element {
	var sb strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		sb.Write(seg.Value(b.doc.src))
	}
	code := strings.TrimSuffix(strings.ReplaceAll(sb.String(), "\r\n", "\n"), "\n")
	return ui.Div().Style(b.st.CodeBlock).ScrollX().Label(b.s.MarkdownCode(lang)).
		Child(ui.Rich(b.codeSegments(lang, code)...).NoWrap().Selectable())
}

// codeSegments returns code as segments of rich text, colored by the view's
// highlighter when it has one and it has the ranges, and plain otherwise.
func (b *builder) codeSegments(lang, code string) []ui.Segment {
	if b.v.code == nil || code == "" {
		return []ui.Segment{ui.Seg(code)}
	}
	spans, ok := b.v.code.HighlightCode(b.code, lang, code, func() { b.update(func() {}) })
	if !ok || len(spans) == 0 {
		return []ui.Segment{ui.Seg(code)}
	}
	// Each byte takes the capture of the last span over it, so a later span wins.
	captures := make([]int32, len(code))
	for i := range captures {
		captures[i] = -1
	}
	for i, sp := range spans {
		for j := max(sp.Start, 0); j < min(sp.End, len(code)); j++ {
			captures[j] = int32(i)
		}
	}
	var segs []ui.Segment
	for start := 0; start < len(code); {
		end := start + 1
		// A run ends where the capture changes, at the start of a character, so that a
		// range that ends inside one (from a highlighter's mistake) splits no character.
		for end < len(code) && (captures[end] == captures[start] || !utf8.RuneStart(code[end])) {
			end++
		}
		g := ui.Seg(code[start:end])
		if c := captures[start]; c >= 0 {
			if st, ok := closestSyntax(b.t.Syntax, spans[c].Capture); ok {
				g = g.Color(st.Color)
				if st.Bold {
					g = g.Bold()
				}
				if st.Italic {
					g = g.Italic()
				}
			}
		}
		segs = append(segs, g)
		start = end
	}
	return segs
}

// codeScope is the context of a view's requests to its CodeHighlighter for one
// text. A new text gets a new context, and the old one is cancelled after the build
// of the new text has asked again for the blocks it still has, so the highlighter
// keeps working on those and drops the rest. The view's disposal cancels it too.
type codeScope struct {
	src       string
	ctx       context.Context
	cancel    context.CancelFunc
	old       context.CancelFunc // the previous text's, cancelled by done
	disposing bool               // the view's OnDispose cancels the context
}

// begin returns the context for the build of src.
func (s *codeScope) begin(uctx *ui.Context, src string) context.Context {
	if !s.disposing {
		s.disposing = true
		uctx.OnDispose(func() {
			if s.cancel != nil {
				s.cancel()
			}
		})
	}
	if s.ctx == nil || s.src != src {
		s.old = s.cancel
		s.src = src
		s.ctx, s.cancel = context.WithCancel(context.Background())
	}
	return s.ctx
}

// done ends a build: the previous text's requests are cancelled.
func (s *codeScope) done() {
	if s.old != nil {
		s.old()
		s.old = nil
	}
}

// closestSyntax returns the theme's style for a capture name: the name's own, or else
// that of the name without its last dotted part, and so on, so "function.method"
// falls back to "function".
func closestSyntax(syntax map[string]ui.SyntaxStyle, capture string) (ui.SyntaxStyle, bool) {
	for name := capture; name != ""; {
		if st, ok := syntax[name]; ok {
			return st, true
		}
		i := strings.LastIndexByte(name, '.')
		if i < 0 {
			break
		}
		name = name[:i]
	}
	return ui.SyntaxStyle{}, false
}

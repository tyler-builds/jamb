package markdown

import (
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// mdParser is the parser of every view: CommonMark with the GitHub extensions
// (tables, strikethrough, task lists and autolinks) and footnotes. A goldmark
// parser is safe for concurrent use, so one is shared.
var mdParser = sync.OnceValue(func() parser.Parser {
	return goldmark.New(goldmark.WithExtensions(extension.GFM, extension.Footnote)).Parser()
})

// document is a parsed text: its syntax tree, which refers to the bytes of src, and
// what the build needs to know before it walks the tree.
type document struct {
	src  []byte
	root ast.Node
	// targets are the anchors that the text links to ("#name" links), lowercased,
	// so that only the headings they name take focus handles.
	targets map[string]bool
}

// parse parses s.
func parse(s string) *document {
	src := []byte(s)
	d := &document{src: src, root: mdParser().Parse(text.NewReader(src))}
	_ = ast.Walk(d.root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if l, ok := n.(*ast.Link); entering && ok && len(l.Destination) > 1 && l.Destination[0] == '#' {
			if d.targets == nil {
				d.targets = map[string]bool{}
			}
			d.targets[strings.ToLower(string(l.Destination[1:]))] = true
		}
		return ast.WalkContinue, nil
	})
	return d
}

// docCache keeps the parsed text of a view between its renders, so a text is parsed
// once: again only when it changes.
type docCache struct {
	src string
	doc *document
}

// get returns the parsed s, parsing it if it is not the text parsed last.
func (c *docCache) get(s string) *document {
	if c.doc == nil || c.src != s {
		c.src, c.doc = s, parse(s)
	}
	return c.doc
}

// slugger makes the anchors of the headings of a document as GitHub does: the text in
// lower case, without punctuation, with a hyphen for each space, and "-1", "-2" and so
// on after the second and later headings with the same anchor.
type slugger map[string]int

// slug returns the anchor of a heading whose text is s.
func (sl slugger) slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r):
			b.WriteRune(r)
		}
	}
	base := b.String()
	n := sl[base]
	sl[base] = n + 1
	if n == 0 {
		return base
	}
	return base + "-" + strconv.Itoa(n)
}

// plainText returns the text of the inline nodes under n, as a reader sees it: soft
// line breaks are spaces, and code, links and emphasis are their text.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			b.Write(c.Value(src))
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.AutoLink:
			b.Write(c.Label(src))
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}

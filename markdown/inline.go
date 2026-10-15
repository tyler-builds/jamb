package markdown

import (
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"

	"github.com/tyler-builds/jamb/ui"
)

// look is the style of a run of inline text: its emphasis, and the link it belongs
// to (0 for none, else 1 + its index in inlines.links).
type look struct {
	bold, italic, strike, code bool
	link                       int
}

// inlines is the content of a block of text as it is collected: runs of text, each
// with one look, and the images between them.
type inlines struct {
	b     *builder
	parts []part
	links []func() // the handlers of the links, by look.link - 1
	cur   look
	text  strings.Builder // the text of the run in progress, in the look cur
}

// part is a run of text, or an image when img is set.
type part struct {
	text string
	look look
	img  *ast.Image
	link func() // the link around an image
}

// collect walks the inline children of n into parts.
func (b *builder) collect(n ast.Node) *inlines {
	in := &inlines{b: b}
	in.children(n, look{})
	in.flush()
	return in
}

// add appends s to the run in progress, starting a new run if its look differs.
func (in *inlines) add(s string, lk look) {
	if s == "" {
		return
	}
	if lk != in.cur {
		in.flush()
		in.cur = lk
	}
	in.text.WriteString(s)
}

// flush ends the run in progress.
func (in *inlines) flush() {
	if in.text.Len() > 0 {
		in.parts = append(in.parts, part{text: in.text.String(), look: in.cur})
		in.text.Reset()
	}
}

// children walks the children of n in the look lk.
func (in *inlines) children(n ast.Node, lk look) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		in.node(c, lk)
	}
}

// node walks one inline node in the look lk.
func (in *inlines) node(n ast.Node, lk look) {
	src := in.b.doc.src
	switch n := n.(type) {
	case *ast.Text:
		in.add(string(n.Value(src)), lk)
		switch {
		case n.HardLineBreak():
			in.add("\n", lk)
		case n.SoftLineBreak():
			in.add(" ", lk)
		}
	case *ast.String:
		in.add(string(n.Value), lk)
	case *ast.CodeSpan:
		lk.code = true
		in.children(n, lk)
	case *ast.Emphasis:
		if n.Level >= 2 {
			lk.bold = true
		} else {
			lk.italic = true
		}
		in.children(n, lk)
	case *east.Strikethrough:
		lk.strike = true
		in.children(n, lk)
	case *ast.Link:
		lk.link = in.link(in.b.follow(string(n.Destination)))
		in.children(n, lk)
	case *ast.AutoLink:
		url := string(n.URL(src))
		if n.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(url), "mailto:") {
			url = "mailto:" + url
		}
		lk.link = in.link(in.b.follow(url))
		in.add(string(n.Label(src)), lk)
	case *ast.Image:
		in.flush()
		var link func()
		if lk.link > 0 {
			link = in.links[lk.link-1]
		}
		in.parts = append(in.parts, part{img: n, link: link})
	case *ast.RawHTML:
		// Inline HTML is shown as it is written, and a comment is left out, as a
		// browser leaves it out.
		var b strings.Builder
		for i := 0; i < n.Segments.Len(); i++ {
			seg := n.Segments.At(i)
			b.Write(seg.Value(src))
		}
		if s := b.String(); !strings.HasPrefix(s, "<!--") {
			in.add(s, lk)
		}
	case *east.FootnoteLink:
		lk.link = in.link(in.b.toFootnote(n.Index))
		in.add("["+strconv.Itoa(n.Index)+"]", lk)
	case *east.TaskCheckBox, *east.FootnoteBacklink:
		// The list item draws the check box (see listItem); the way back from a
		// footnote is the link that led there.
	default:
		in.children(n, lk)
	}
}

// link records a link's handler and returns its number for a look.
func (in *inlines) link(fn func()) int {
	in.links = append(in.links, fn)
	return len(in.links)
}

// blank reports whether the parts hold no image and no text but white space.
func (in *inlines) blank() bool {
	for _, p := range in.parts {
		if p.img != nil || strings.TrimSpace(p.text) != "" {
			return false
		}
	}
	return true
}

// segments returns the runs of text of parts as the segments of a rich text, styled
// with the view's parts: links in the Link color, with LinkHover and an underline
// under the pointer, and code in the monospace font with the InlineCode colors. The
// runs of one link, such as a bold word in it, are one link (see ui.LinkSegs): one
// Tab stop, and one link for assistive technology.
func (in *inlines) segments(parts []part) []ui.Segment {
	st := &in.b.st
	segs := make([]ui.Segment, 0, len(parts))
	var link []ui.Segment // the runs of the link in progress
	linkNum := 0
	endLink := func() {
		if len(link) > 0 {
			segs = append(segs, ui.LinkSegs(in.links[linkNum-1], link...)...)
			link = nil
		}
	}
	for _, p := range parts {
		g := ui.Seg(p.text)
		if p.look.bold {
			g = g.Bold()
		}
		if p.look.italic {
			g = g.Italic()
		}
		if p.look.strike {
			g = g.Strikethrough()
		}
		if p.look.code {
			g = g.Mono().Color(st.InlineCode.TextColor).Background(st.InlineCode.Background)
		}
		if p.look.link != linkNum {
			endLink()
			linkNum = p.look.link
		}
		if p.look.link > 0 {
			link = append(link, g.Color(st.Link.TextColor).Background(st.Link.Background).
				Hover(ui.Highlight{Color: st.LinkHover.TextColor, Background: st.LinkHover.Background, Underline: true}))
			continue
		}
		segs = append(segs, g)
	}
	endLink()
	return segs
}

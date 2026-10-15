package markdown

import (
	"context"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// builder builds the elements of one render of a view from its parsed text.
type builder struct {
	v      *ViewElement
	ctx    *ui.Context
	t      *ui.Theme
	s      *ui.Strings
	st     Style
	doc    *document
	images *imageCache
	update func(func()) // runs a function on the UI thread and renders the view again
	// code is done when the view no longer shows this text: it is the context of the
	// requests to the view's CodeHighlighter (see codeScope).
	code  context.Context
	slugs slugger
	// anchors are the focus handles of the headings that "#name" links lead to, by
	// anchor, and notes those of the footnotes, by number. The links look them up
	// when they are followed, after the build has made them all.
	anchors map[string]*ui.FocusHandle
	notes   map[int]*ui.FocusHandle
	tasks   int // the task list items built so far
	depth   int // how many lists the block being built is inside
}

// newBuilder returns the builder of one render of v.
func newBuilder(v *ViewElement, ctx *ui.Context, doc *document, images *imageCache, update func(func()), code context.Context) *builder {
	return &builder{v: v, ctx: ctx, t: ctx.Theme(), s: ctx.Strings(), st: v.resolveStyle(ctx), doc: doc,
		images: images, update: update, code: code, slugs: slugger{}, anchors: map[string]*ui.FocusHandle{}, notes: map[int]*ui.FocusHandle{}}
}

// document returns the column of the document's blocks, with the view's sizes.
func (b *builder) document() ui.Element {
	root := applySizes(b.v.size, ui.Div().Style(b.st.Document))
	if b.v.label != "" {
		root.Label(b.v.label)
	}
	return root.Children(b.blocks(b.doc.root)...)
}

// blocks returns the elements of the block children of n.
func (b *builder) blocks(n ast.Node) []ui.Element {
	var out []ui.Element
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if el := b.block(c); el != nil {
			out = append(out, el)
		}
	}
	return out
}

// block returns the element of the block n, or nil for a block that shows nothing.
func (b *builder) block(n ast.Node) ui.Element {
	src := b.doc.src
	switch n := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return b.paragraph(n)
	case *ast.Heading:
		return b.heading(n)
	case *ast.ThematicBreak:
		return ui.Div().Role(access.RoleSeparator).Style(b.st.Rule)
	case *ast.FencedCodeBlock:
		return b.codeBlock(n.Lines(), string(n.Language(src)))
	case *ast.CodeBlock:
		return b.codeBlock(n.Lines(), "")
	case *ast.Blockquote:
		return ui.Div().Style(b.st.Quote).Children(b.blocks(n)...)
	case *ast.List:
		return b.list(n)
	case *ast.HTMLBlock:
		return b.html(n)
	case *east.Table:
		return b.table(n)
	case *east.FootnoteList:
		return b.footnotes(n)
	}
	if n.HasChildren() {
		return ui.Div().Gap(b.st.Document.Gap).Children(b.blocks(n)...)
	}
	return nil
}

// paragraph returns the text of a paragraph, or nil when it has none.
func (b *builder) paragraph(n ast.Node) ui.Element {
	in := b.collect(n)
	if in.blank() {
		return nil
	}
	return b.flow(in, b.st.Paragraph, ui.Start)
}

// flow returns the inline content in as elements: each run of text between images
// one selectable rich text in the style st, aligned by align, and each image a block
// of its own, with images that follow one another in a row that wraps.
func (b *builder) flow(in *inlines, st ui.Style, align ui.Alignment) ui.Element {
	var out []ui.Element
	var run []part
	var imgs []ui.Element
	endText := func() {
		blank := true
		for _, p := range run {
			blank = blank && strings.TrimSpace(p.text) == ""
		}
		if !blank {
			t := ui.Rich(in.segments(run)...).Style(st).Selectable()
			if align != ui.Start {
				t.Align(align)
			}
			out = append(out, t)
		}
		run = nil
	}
	endImages := func() {
		switch len(imgs) {
		case 0:
		case 1:
			out = append(out, imgs[0])
		default:
			out = append(out, ui.Div().Row().Wrap().Gap(8).Children(imgs...))
		}
		imgs = nil
	}
	for _, p := range in.parts {
		switch {
		case p.img != nil:
			endText()
			imgs = append(imgs, b.image(p.img, p.link))
		case len(imgs) > 0 && strings.TrimSpace(p.text) == "":
			// The space between two images, such as a row of badges.
		default:
			endImages()
			run = append(run, p)
		}
	}
	endText()
	endImages()
	if len(out) == 1 {
		return out[0]
	}
	return ui.Div().Gap(8).Children(out...)
}

// heading returns a heading: a box that assistive technology reads as a heading of its
// level. A heading that a "#name" link leads to takes focus when the link is followed,
// which scrolls it into view; Tab does not stop on it.
func (b *builder) heading(n *ast.Heading) ui.Element {
	box := ui.Div().Heading(min(max(n.Level, 1), 6)).Style(b.st.heading(n.Level))
	if slug := b.slugs.slug(plainText(n, b.doc.src)); b.doc.targets[slug] {
		h := ui.NewFocusHandle()
		box.Focusable().TabStop(false).FocusHandle(h)
		b.anchors[slug] = h
	}
	return box.Child(b.flow(b.collect(n), ui.Style{}, ui.Start))
}

// follow returns what following a link to url does: the view's OnLink, or else for
// "#name" a move to the heading with that anchor, and for the other URLs
// ui.Context.OpenURL, which opens http, https and mailto URLs and refuses the rest.
func (b *builder) follow(url string) func() {
	onLink, ctx, anchors := b.v.onLink, b.ctx, b.anchors
	return func() {
		switch {
		case onLink != nil:
			onLink(url)
		case strings.HasPrefix(url, "#"):
			if h := anchors[strings.ToLower(url[1:])]; h != nil {
				h.Focus()
			}
		default:
			_ = ctx.OpenURL(url) // a relative path or another scheme is refused, and does nothing
		}
	}
}

// toFootnote returns what following the reference to footnote i does: a move to it.
func (b *builder) toFootnote(i int) func() {
	notes := b.notes
	return func() {
		if h := notes[i]; h != nil {
			h.Focus()
		}
	}
}

// list returns a list: a column of items, each with its bullet, its number or its
// task's check box before its blocks. Assistive technology reads each item with its
// position in the list ("2 of 5") and its level, 1 for a list that is not nested.
func (b *builder) list(n *ast.List) ui.Element {
	gap := b.st.List.Gap
	if !n.IsTight {
		gap = b.st.Document.Gap
	}
	box := ui.Div().Role(access.RoleList).Style(b.st.List).Gap(gap)
	b.depth++
	num, pos, total := n.Start, 0, n.ChildCount()
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		pos++
		var marker ui.Element
		if checked, ok := taskOf(c); ok {
			marker = b.taskBox(c, checked)
		} else if n.IsOrdered() {
			marker = ui.Text(strconv.Itoa(num) + string(n.Marker)).Style(b.st.ListMarker)
		} else {
			// The bullet is not read: assistive technology says that the item is one of a list.
			marker = ui.Div().AccessHidden().Child(ui.Text(bullet(b.depth)).Style(b.st.ListMarker))
		}
		num++
		box.Child(ui.Div().Role(access.RoleListItem).AccessPosition(pos, total).AccessLevel(b.depth).
			Row().Align(ui.Start).Gap(6).
			Child(marker).
			Child(ui.Div().Grow(1).MinWidth(ui.Px(0)).Gap(gap).Children(b.blocks(c)...)))
	}
	b.depth--
	return box
}

// bullet is the bullet of a list nested depth deep: a disc, a circle, then a square.
func bullet(depth int) string {
	return [...]string{"•", "◦", "▪"}[(depth-1)%3]
}

// taskOf reports whether a list item is a task, and whether it is done.
func taskOf(item ast.Node) (checked, ok bool) {
	if first := item.FirstChild(); first != nil {
		if box, ok := first.FirstChild().(*east.TaskCheckBox); ok {
			return box.IsChecked, true
		}
	}
	return false, false
}

// taskBox returns the check box of a task list item, named by the item's text. It is
// operable only when the view has OnTaskToggle.
func (b *builder) taskBox(item ast.Node, checked bool) ui.Element {
	name := plainText(item.FirstChild(), b.doc.src)
	if name == "" {
		name = b.s.MarkdownTask
	}
	st := b.st.Task
	if checked {
		st = st.Merge(b.st.TaskChecked)
	}
	box := ui.Div().Role(access.RoleCheckBox).AccessChecked(checked).Label(name).Style(st).Shrink(0)
	if checked {
		box.Child(ui.Canvas(func(p *ui.Painter, w, h float32) {
			mark := ui.NewPath().MoveTo(w*0.22, h*0.52).LineTo(w*0.42, h*0.72).LineTo(w*0.78, h*0.3)
			p.Stroke(mark, ui.StrokeStyle{Width: 1.6, Color: p.TextColor(), Cap: ui.CapRound, Join: ui.JoinRound})
		}).Size(ui.Full, ui.Full).AccessHidden())
	}
	index := b.tasks
	b.tasks++
	if fn := b.v.onTask; fn != nil {
		box.Focusable().Cursor(ui.CursorPointer).
			FocusVisible(ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: b.t.FocusRing}}}).
			OnClick(func() { fn(index, !checked) })
	}
	return box
}

// html returns an HTML block as the text it is written in, which is not interpreted.
// A comment shows nothing.
func (b *builder) html(n *ast.HTMLBlock) ui.Element {
	if n.HTMLBlockType == ast.HTMLBlockType2 {
		return nil
	}
	var sb strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		sb.Write(seg.Value(b.doc.src))
	}
	if n.HasClosure() {
		sb.Write(n.ClosureLine.Value(b.doc.src))
	}
	s := strings.TrimRight(sb.String(), "\r\n")
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return ui.Text(s).Style(b.st.Paragraph).Selectable()
}

// footnotes returns the footnotes at the end of the document: a rule, then a list of
// the notes by number, each with its position in the list. A reference to a note
// moves focus to it, which scrolls it into view.
func (b *builder) footnotes(n *east.FootnoteList) ui.Element {
	list := ui.Div().Role(access.RoleList).Label(b.s.MarkdownFootnotes).Style(b.st.List)
	var notes []*east.Footnote
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if note, ok := c.(*east.Footnote); ok {
			notes = append(notes, note)
		}
	}
	for i, note := range notes {
		h := ui.NewFocusHandle()
		b.notes[note.Index] = h
		list.Child(ui.Div().Role(access.RoleListItem).AccessPosition(i+1, len(notes)).Row().Align(ui.Start).Gap(6).
			Focusable().TabStop(false).FocusHandle(h).
			Child(ui.Text(strconv.Itoa(note.Index) + ".").Style(b.st.ListMarker)).
			Child(ui.Div().Grow(1).MinWidth(ui.Px(0)).Gap(b.st.List.Gap).Children(b.blocks(note)...)))
	}
	return ui.Div().Gap(b.st.Document.Gap).
		Child(ui.Div().Role(access.RoleSeparator).Style(b.st.Rule)).
		Child(list)
}

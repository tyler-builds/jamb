package ui

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/internal/text"
)

// Highlight styles a range of text. Zero fields are unset (keep the text's own style).
type Highlight struct {
	// Color and Background are the text color and the background of the highlighted text.
	Color, Background Color
	// Weight is the font weight of the text.
	Weight FontWeight
	// Italic, Underline and Strikethrough turn on that look for the text.
	Italic, Underline, Strikethrough bool
	Mono                             bool // monospace family (inline code)
}

// over returns h with the set fields of o applied on top.
func (h Highlight) over(o Highlight) Highlight {
	if !o.Color.IsZero() {
		h.Color = o.Color
	}
	if !o.Background.IsZero() {
		h.Background = o.Background
	}
	if o.Weight != 0 {
		h.Weight = o.Weight
	}
	h.Italic = h.Italic || o.Italic
	h.Underline = h.Underline || o.Underline
	h.Strikethrough = h.Strikethrough || o.Strikethrough
	h.Mono = h.Mono || o.Mono
	return h
}

// paintOnly keeps the fields of a hover highlight that do not change the
// shaping, so hovering never changes the layout.
func paintOnly(h Highlight) Highlight {
	return Highlight{Color: h.Color, Background: h.Background, Underline: h.Underline, Strikethrough: h.Strikethrough}
}

// Segment is one piece of rich text built with Rich.
type Segment struct {
	text    string
	hl      Highlight
	accent  bool // a Link: the theme's Link color unless a Color is set
	onClick func()
	hover   *Highlight
	// link is shared by the segments of one link made by LinkSegs, so that
	// consecutive segments with the same link are one link (see linkEnd).
	link *linkGroup
}

// linkGroup is the identity of a link made of several segments (see LinkSegs).
// It has a field so that two groups are never the same pointer.
type linkGroup struct{ _ int }

// Seg is a plain piece of rich text.
func Seg(s string) Segment { return Segment{text: s} }

// Bold, Italic, Mono, Underline and Strikethrough style the segment.
func (g Segment) Bold() Segment { g.hl.Weight = FontBold; return g }

// Italic sets the segment's text in italic.
func (g Segment) Italic() Segment { g.hl.Italic = true; return g }

// Mono sets the segment's text in the monospace font.
func (g Segment) Mono() Segment { g.hl.Mono = true; return g }

// Underline draws a line under the segment's text.
func (g Segment) Underline() Segment { g.hl.Underline = true; return g }

// Strikethrough draws a line through the segment's text.
func (g Segment) Strikethrough() Segment { g.hl.Strikethrough = true; return g }

// Color and Background set the segment's text and background colors.
func (g Segment) Color(c Color) Segment { g.hl.Color = c; return g }

// Background sets the color behind the segment's text.
func (g Segment) Background(c Color) Segment { g.hl.Background = c; return g }

// Style merges h into the segment.
func (g Segment) Style(h Highlight) Segment { g.hl = g.hl.over(h); return g }

// OnClick makes the segment clickable: the pointer is a hand over it, and a
// click runs fn instead of the clickable elements around it. The segment is a
// link to assistive technology and a stop of its own in the Tab order, and
// Enter runs fn while it has focus.
func (g Segment) OnClick(fn func()) Segment { g.onClick = fn; return g }

// Hover merges h into the segment while the pointer is over it. Only the
// fields that do not change the shaping apply (Color, Background, Underline,
// Strikethrough), so hovering never re-lays out the text.
func (g Segment) Hover(h Highlight) Segment { g.hover = &h; return g }

// Link is a clickable piece of text. It takes the theme's Link color (when
// no Color is set) and underlines while the pointer is over it. Tab stops on
// it, a ring shows its keyboard focus, and Enter follows it (see OnClick).
func Link(s string, fn func()) Segment {
	g := Seg(s).OnClick(fn).Hover(Highlight{Underline: true})
	g.accent = true
	return g
}

// LinkSegs makes one link of the segments segs, as Link does for one: each takes
// the theme's Link color unless it sets a Color, underlines while the pointer is
// over the link (unless it has a Hover of its own), and a click on any of them
// runs fn. Use it for a link whose text mixes styles, such as a bold word in it:
//
//	ui.Rich(append([]ui.Segment{ui.Seg("See ")},
//		ui.LinkSegs(open, ui.Seg("the "), ui.Seg("guide").Bold())...)...)
//
// Placed one after another in a Rich text, the segments are one link: one stop in
// the Tab order, one focus ring around all of their words, and one link for
// assistive technology, named by their joined text. Enter runs fn once.
func LinkSegs(fn func(), segs ...Segment) []Segment {
	g := &linkGroup{}
	out := make([]Segment, len(segs))
	for i, s := range segs {
		s.onClick, s.accent, s.link = fn, true, g
		if s.hover == nil {
			s.hover = &Highlight{Underline: true}
		}
		out[i] = s
	}
	return out
}

// linkEnd returns the index after the last segment of the link that starts at
// segs[i]: the segments after it that LinkSegs made with it, or i+1.
func linkEnd(segs []Segment, i int) int {
	j := i + 1
	if g := segs[i].link; g != nil {
		for j < len(segs) && segs[j].link == g {
			j++
		}
	}
	return j
}

// linkStart reports whether segs[i] begins a link: it is clickable, and is not
// the continuation of the link of the segment before it.
func linkStart(segs []Segment, i int) bool {
	return segs[i].onClick != nil && (i == 0 || segs[i].link == nil || segs[i-1].link != segs[i].link)
}

// Rich creates one text element from segments. It is laid out, wrapped,
// truncated and aligned like Text.
func Rich(segs ...Segment) *TextElement {
	var b strings.Builder
	for _, g := range segs {
		b.WriteString(g.text)
	}
	return &TextElement{text: b.String(), segs: segs}
}

// richText is a text element's segments and highlights, flattened once when
// the node is built: spans for the text system and a paint entry per span.
type richText struct {
	spans []text.Span
	paint []richPaint // parallel to spans
	segs  []Segment   // the owners named by richPaint.owner
	// interactive is set when some segment is clickable or hoverable.
	interactive bool
	// lay is the layout last painted, which hit testing reads.
	lay *text.Layout
}

// richPaint is the look of one span.
type richPaint struct {
	hl     Highlight
	accent bool
	owner  int // index of the segment the span comes from, or -1
}

// segRef names a segment of a rich text node.
type segRef struct {
	node uint64
	seg  int
}

// buildRich flattens the segments and highlights of t. Every boundary of
// either becomes a span, and adjacent spans with the same look are merged.
func buildRich(t *TextElement) *richText {
	segs := t.segs
	if len(segs) == 0 {
		segs = []Segment{Seg(t.text)}
	}
	starts := make([]int, len(segs))
	bounds := []int{0, len(t.text)}
	off := 0
	rt := &richText{segs: segs}
	for i, g := range segs {
		starts[i] = off
		off += len(g.text)
		bounds = append(bounds, off)
		if g.onClick != nil || g.hover != nil {
			rt.interactive = true
		}
	}
	for _, h := range t.highlights {
		bounds = append(bounds, h.start, h.end)
	}
	slices.Sort(bounds)
	bounds = slices.Compact(bounds)
	for i := 0; i+1 < len(bounds); i++ {
		a, b := bounds[i], bounds[i+1]
		p := richPaint{owner: -1}
		for j, g := range segs {
			if a >= starts[j] && a < starts[j]+len(g.text) {
				p.hl, p.accent, p.owner = g.hl, g.accent, j
				break
			}
		}
		for _, h := range t.highlights {
			if h.start <= a && b <= h.end {
				p.hl = p.hl.over(h.hl)
			}
		}
		if k := len(rt.paint) - 1; k >= 0 && rt.paint[k] == p && rt.spans[k].End == a {
			rt.spans[k].End = b
			continue
		}
		sp := text.Span{Start: a, End: b, Weight: p.hl.Weight, Italic: p.hl.Italic}
		if p.hl.Mono {
			sp.Family = text.FamilyMono
		}
		rt.spans = append(rt.spans, sp)
		rt.paint = append(rt.paint, p)
	}
	return rt
}

// textLayout lays out the text of text node n, with its spans when it is rich.
func (n *node) textLayout(maxWidth float32) *text.Layout {
	if n.rich != nil {
		return n.rt.text.LayoutSpans(n.textEl.text, n.textStyle(), n.rich.spans, maxWidth)
	}
	return n.rt.text.Layout(n.textEl.text, n.textStyle(), maxWidth)
}

// richSpanAt returns the span under (x, y) in logical window coordinates, or
// -1. It uses the layout last painted, so it matches what is on screen.
func (n *node) richSpanAt(x, y float32) int {
	l := n.rich.lay
	if l == nil || l.LineHeight <= 0 || len(l.Lines) == 0 {
		return -1
	}
	s := &n.style
	x0 := n.abs.X + s.Padding.Left
	y0 := n.abs.Y + s.Padding.Top
	ly := y - y0
	li := int(math.Floor(float64(ly / l.LineHeight)))
	if li < 0 || li >= len(l.Lines) {
		return -1
	}
	contentW := max(n.abs.W-s.Padding.Left-s.Padding.Right, 0)
	var off float32
	switch n.ts.align {
	case Center:
		off = (contentW - l.Lines[li].Width) / 2
	case End:
		off = contentW - l.Lines[li].Width
	}
	return l.SpanAt(x-x0-off, ly)
}

// segmentAt returns the segment under (x, y) when the deepest node of path is
// a rich text node with a clickable or hoverable segment there.
func (rt *Runtime) segmentAt(path []*node, x, y float32) segRef {
	if len(path) == 0 || path[0].flags&flagRich == 0 {
		return segRef{}
	}
	n := path[0]
	span := n.richSpanAt(x, y)
	if span < 0 || span >= len(n.rich.paint) {
		return segRef{}
	}
	owner := n.rich.paint[span].owner
	if owner < 0 {
		return segRef{}
	}
	if seg := &n.rich.segs[owner]; seg.onClick == nil && seg.hover == nil {
		return segRef{}
	}
	return segRef{n.id, owner}
}

// clickable reports whether r names a segment with an OnClick.
func (rt *Runtime) clickable(r segRef) bool {
	n := rt.byID[r.node]
	return n != nil && n.rich != nil && r.seg < len(n.rich.segs) && n.rich.segs[r.seg].onClick != nil
}

// runPaint returns how a span of rich text n is painted: its merged look,
// with the hover highlight of its segment merged on top while hovered.
// base is the inherited text color.
func (rt *Runtime) runPaint(n *node, span int, base Color, opacity float32) text.RunPaint {
	if span < 0 || span >= len(n.rich.paint) {
		return text.RunPaint{Color: base.Alpha(opacity).scene()}
	}
	p := n.rich.paint[span]
	hl := p.hl
	if p.owner >= 0 && rt.hovered(n, p.owner) {
		if h := n.rich.segs[p.owner].hover; h != nil {
			hl = hl.over(paintOnly(*h))
		}
	}
	color := hl.Color
	switch {
	case color.IsZero() && p.accent:
		color = rt.theme.Link
	case color.IsZero():
		color = base
	}
	return text.RunPaint{
		Color:      color.Alpha(opacity).scene(),
		Background: hl.Background.Alpha(opacity).scene(),
		Underline:  hl.Underline,
		Strike:     hl.Strikethrough,
	}
}

// hovered reports whether the pointer is over segment i of rich text n, or over
// another segment of the same link.
func (rt *Runtime) hovered(n *node, i int) bool {
	h := rt.hoverSeg
	if h.node != n.id || h.seg < 0 || h.seg >= len(n.rich.segs) {
		return false
	}
	g := n.rich.segs[i].link
	return h.seg == i || (g != nil && n.rich.segs[h.seg].link == g)
}

// checkRange panics, naming the text and how to fix it, when [start, end) is
// not a range of byte offsets into s.
func checkRange(s string, start, end int) {
	if start > end {
		panic(fmt.Sprintf("ui: Highlight(%d, %d) has start after end; pass the start first", start, end))
	}
	for _, off := range []int{start, end} {
		switch {
		case off < 0 || off > len(s):
			panic(fmt.Sprintf("ui: Highlight(%d, %d) on %q: offset %d is outside the text (%d bytes); use byte offsets from strings.Index on the same string", start, end, clipText(s), off, len(s)))
		case off < len(s) && !utf8.RuneStart(s[off]):
			b := off
			for !utf8.RuneStart(s[b]) {
				b--
			}
			r, _ := utf8.DecodeRuneInString(s[b:])
			panic(fmt.Sprintf("ui: Highlight(%d, %d) on %q: offset %d is inside the UTF-8 encoding of '%c'; use byte offsets from strings.Index on the same string", start, end, clipText(s), off, r))
		}
	}
}

// clipText shortens s for a panic message.
func clipText(s string) string {
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}

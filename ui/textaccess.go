package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/text"
)

// The Text pattern of a text field or text area reads the editor's text and the
// layout that the last paint made of it. The access content (access.TextContent)
// is built from them on the UI thread, once for each change of the text or the
// layout, and kept on the editor's state. Each snapshot adds its own selection
// and where the content is drawn (access.TextInfo), so a selection change does
// not rebuild the content.

// textContent returns the access content of the editor's text v, from the layout
// es.layout. It is built again only when the text or the layout changed.
func (rt *Runtime) textContent(es *editState, v string) *access.TextContent {
	if es.content != nil && es.content.Text == v && es.contentLayout == es.layout {
		return es.content
	}
	es.content = rt.buildTextContent(v, es.layout)
	es.contentLayout = es.layout
	return es.content
}

// buildTextContent makes the access content of the text v laid out as l. A layout
// that is missing or was made from other text gives the text without geometry:
// one line, with no height, so that it has no rectangles until the next paint.
func (rt *Runtime) buildTextContent(v string, l *text.Layout) *access.TextContent {
	c := &access.TextContent{Text: v, Points: rt.text.GraphemeStops(v)}
	n := c.Len()
	c.X = make([]float32, n+1)
	c.Lines = []access.TextLine{{Start: 0}}
	if l != nil && l.Text == v && len(l.Lines) > 0 {
		c.Lines = c.Lines[:0]
		for i, ln := range l.Lines {
			start, _ := l.LineBounds(i)
			c.Lines = append(c.Lines, access.TextLine{Start: c.PointOf(start), Top: ln.Top, Height: l.LineHeight, Right: ln.Width})
		}
		// Each line owns the points from its start up to the next line's start, so
		// the x of every point is found in one pass over the lines.
		for i := range c.Lines {
			end := n + 1
			if i+1 < len(c.Lines) {
				end = c.Lines[i+1].Start
			}
			for p := c.Lines[i].Start; p < end; p++ {
				c.X[p] = l.LineCaretX(i, c.ByteOf(p))
			}
		}
	}
	// A paragraph ends with its line break. A break is its own grapheme cluster,
	// or the last byte of a CR LF pair.
	c.Paras = []int{0}
	for p := 0; p < n; p++ {
		if c.Text[c.ByteOf(p+1)-1] == '\n' {
			c.Paras = append(c.Paras, p+1)
		}
	}
	// Word units are the Windows units (text.WordStarts), which the UI Automation
	// Text pattern reports on every platform, as the Windows adapter is the one
	// that exposes it.
	for _, b := range text.WordStarts(v) {
		c.Words = append(c.Words, c.PointOf(b))
	}
	return c
}

// textInfo returns the Text pattern state of the editor node n, with the text v
// and the editor state es, as the snapshot being built shows them.
func (b *accessBuilder) textInfo(n *node, es *editState, v string) *access.TextInfo {
	rt := b.rt
	// The content is the text as the field shows it: a masked field's bullets, or the
	// text in its format, without the prompts of the slots not filled.
	pr := rt.present(n.editor, v)
	anchor, caret := pr.toShown(es.anchor), pr.toShown(es.caret)
	c := rt.textContent(es, pr.shown)
	return &access.TextInfo{
		Content:    c,
		Anchor:     c.PointOf(anchor),
		Caret:      c.PointOf(caret),
		OriginX:    (n.abs.X + es.originX) * rt.scale,
		OriginY:    (n.abs.Y + es.originY) * rt.scale,
		Scale:      rt.scale,
		Clip:       b.bounds(n),
		FontName:   n.ts.family,
		FontSize:   float64(n.ts.size*n.rt.textFactor) * 0.75, // logical pixels to points, at 96 per inch
		FontWeight: int(n.ts.weight),
	}
}

// editorSelect sets the selection of an editor to the byte offsets anchor and
// caret, clamped to the text on a rune boundary, and focuses the editor, as a
// selection made by an assistive technology should. It is the Select of a text
// range. The request is made from a snapshot, so the offsets may be out of date.
// The offsets are of the text shown: the bullets of a masked field, or the text in
// its format (see textInfo).
func (rt *Runtime) editorSelect(id uint64, anchor, caret int) {
	n, es, v := rt.editorOf(id)
	if n == nil {
		return
	}
	pr := rt.present(n.editor, v)
	anchor, caret = pr.fromShown(anchor), pr.fromShown(caret)
	if rt.focusID != id {
		rt.setFocus(id)
	}
	es.anchor, es.caret = clampToRune(v, anchor), clampToRune(v, caret)
	es.havePrefX = false
	rt.editorTouched(id)
}

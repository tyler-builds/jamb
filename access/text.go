package access

import (
	"slices"
	"sort"
	"unicode"
	"unicode/utf8"
)

// The Text pattern of a text field or text area (see TextInfo and TextRange).
//
// A text is measured in points. A point is a position between two grapheme
// clusters: point p is the byte offset Points[p], and the last point is the end
// of the text. Points are what a text range's endpoints are made of, so a range
// never splits a grapheme cluster, a surrogate pair or a combining sequence.
// UI Automation does not expose offsets, so the point is an internal unit; the
// UTF-16 length of a text is counted only where UI Automation counts it, in
// GetText's maxLength (see UTF16Prefix).

// TextUnit is a unit of text that a range is expanded or moved by. The values
// follow the TextUnit enumeration of UI Automation.
type TextUnit uint8

const (
	UnitCharacter TextUnit = iota // a grapheme cluster
	UnitFormat                    // a run of text with the same formatting; a plain text has one
	UnitWord                      // a word unit: a word, or punctuation, with the spaces after it, by the Windows word rule
	UnitLine                      // a visual line, with the line break that ends it
	UnitParagraph                 // a paragraph, with the line break that ends it
	UnitPage                      // a page; a text field's one page is its whole text
	UnitDocument                  // the whole text
)

// TextLine is one visual line of a laid-out text. The line covers the points
// from Start up to the Start of the next line, so a line break belongs to the
// line it ends. Top, Height and Right are in layout units, from the top-left of
// the layout: Right is the end of the line's text, without its trailing spaces.
type TextLine struct {
	// Start is the first point of the line.
	Start int
	// Top and Height are the vertical position and the height of the line, in layout units.
	Top, Height float32
	// Right is the end of the line's text, without its trailing spaces, in layout units.
	Right float32
}

// TextModel is the text of a text field or text area, as the Text pattern reads
// it. The pattern works in points (see TextContent). A model is never changed
// after it is published, so any goroutine may read it. Implementations are
// pointers, since TextInfo.Equal compares the models by identity.
//
// The pattern asks the model for the units around a point, and for a range's
// text with a limit, rather than for the whole text, so that a large text need
// not be copied for a query. TextContent is the model of a text that is held in
// memory; the codeedit package has one over a rope.
// Methods may be added to TextModel in later versions.
type TextModel interface {
	// Len returns the last point, the end of the text.
	Len() int
	// PointOf returns the point at byte offset b, rounded down to a point.
	PointOf(b int) int
	// ByteOf returns the byte offset of point p, clamped to the text.
	ByteOf(p int) int
	// Span returns the points [a, b) of the unit u that holds point p, with
	// 0 <= p < Len().
	Span(u TextUnit, p int) (int, int)
	// Next returns the point one unit after p, or Len() when there is none.
	Next(u TextUnit, p int) int
	// Prev returns the point one unit before p, or 0 when there is none.
	Prev(u TextUnit, p int) int
	// Line returns the visual line that holds point p, and the point where the
	// line ends: the first point of the next line, or Len() on the last line.
	Line(p int) (TextLine, int)
	// CaretX returns the x of point p in layout units, on the line it belongs to.
	CaretX(p int) float32
	// Nearest returns the point nearest to the position (x, y) in layout units.
	Nearest(x, y float32) int
	// Read returns the text of the points [from, to), cut to at most max UTF-16
	// code units (see UTF16Prefix). A negative max means no limit.
	Read(from, to, max int) string
	// SameText reports whether o holds the same text as the model. A change of
	// text is reported separately from a change of selection (see Diff).
	SameText(o TextModel) bool
}

// TextContent is the text of one editor, with the geometry of its layout. The
// ui runtime builds it on the UI thread when the text or the layout changes, and
// shares it between the snapshots that show that text. It is never changed after
// it is built, so any goroutine may read it. It implements TextModel.
type TextContent struct {
	Text   string     // the text, UTF-8, as the editor holds it
	Points []int      // the byte offset of each point, from 0 to len(Text)
	X      []float32  // for each point, the x of the caret in layout units, on the line the point belongs to
	Lines  []TextLine // the visual lines, in order; there is always one
	Paras  []int      // the first point of each paragraph, in order
	Words  []int      // the first point of each word unit, in order, by the Windows word rule
}

// Len returns the number of grapheme clusters in the text. The last point is
// Len(), the end of the text, so there are Len()+1 points.
func (c *TextContent) Len() int { return len(c.Points) - 1 }

// PointOf returns the point at byte offset b. An offset inside a grapheme
// cluster rounds down to the start of the cluster.
func (c *TextContent) PointOf(b int) int {
	i, found := slices.BinarySearch(c.Points, b)
	if found {
		return i
	}
	return max(i-1, 0)
}

// ByteOf returns the byte offset of point p, clamped to the text.
func (c *TextContent) ByteOf(p int) int {
	return c.Points[min(max(p, 0), c.Len())]
}

// TextInfo is the state of a text field or text area in one snapshot: its
// content, its selection, and where the content is drawn in the window.
type TextInfo struct {
	// Content is the text of the field, as the Text pattern reads it.
	Content TextModel
	Anchor  int // the selection is the points between Anchor and Caret; a degenerate one is a caret
	Caret   int // the end of the selection that moves
	// OriginX and OriginY are the window position of the layout's top-left
	// corner, in physical pixels, and Scale is the number of physical pixels per
	// layout unit. Clip is the visible part of the field, in physical pixels.
	OriginX, OriginY float32
	// Scale is the number of physical pixels per layout unit.
	Scale float32
	// Clip is the visible part of the field, in physical pixels.
	Clip Rect
	// FontName, FontSize and FontWeight describe the text's font. FontSize is in
	// points, and FontWeight is the CSS-style weight (400 is normal).
	FontName string
	// FontSize is the font size, in points.
	FontSize float64
	// FontWeight is the CSS-style weight of the font.
	FontWeight int
	// Italic reports whether the text is set in italic.
	Italic   bool
	ReadOnly bool // the text can be read but not edited
}

// Equal reports whether two infos describe the same text state. Their contents
// are compared by identity, since a content is shared by the snapshots that show
// it; Diff reports a text change separately (see Update.TextChanged).
func (t *TextInfo) Equal(o *TextInfo) bool {
	if t == o {
		return true
	}
	if t == nil || o == nil {
		return false
	}
	return t.Content == o.Content && t.Anchor == o.Anchor && t.Caret == o.Caret &&
		t.OriginX == o.OriginX && t.OriginY == o.OriginY && t.Scale == o.Scale &&
		t.Clip == o.Clip && t.FontName == o.FontName && t.FontSize == o.FontSize &&
		t.FontWeight == o.FontWeight && t.Italic == o.Italic && t.ReadOnly == o.ReadOnly
}

// lastStart returns the index of the last start in starts that is at or before
// grapheme g. The starts are in order, and the first one is 0.
func lastStart(starts []int, g int) int {
	i := sort.Search(len(starts), func(i int) bool { return starts[i] > g })
	return max(i-1, 0)
}

// Span returns the points [a, b) of the unit u that holds point p, with
// 0 <= p < Len().
func (c *TextContent) Span(u TextUnit, p int) (int, int) {
	switch u {
	case UnitCharacter:
		return p, p + 1
	case UnitLine:
		ln, end := c.Line(p)
		return ln.Start, end
	case UnitParagraph:
		i := lastStart(c.Paras, p)
		return c.Paras[i], c.endOf(c.Paras, i)
	case UnitWord:
		i := lastStart(c.Words, p)
		return c.Words[i], c.endOf(c.Words, i)
	}
	return 0, c.Len() // format, page and document
}

// Line returns the visual line that holds point p, and the point where it ends.
func (c *TextContent) Line(p int) (TextLine, int) {
	i := sort.Search(len(c.Lines), func(i int) bool { return c.Lines[i].Start > p })
	i = max(i-1, 0)
	end := c.Len()
	if i+1 < len(c.Lines) {
		end = c.Lines[i+1].Start
	}
	return c.Lines[i], end
}

// endOf returns the end of the unit i of starts: the next start, or the end of
// the text.
func (c *TextContent) endOf(starts []int, i int) int {
	if i+1 < len(starts) {
		return starts[i+1]
	}
	return c.Len()
}

// Next returns the point one unit after p, or the end of the text when there is
// none. A word unit moves to the start of the next word unit, as Ctrl+Right does on
// Windows.
func (c *TextContent) Next(u TextUnit, p int) int {
	n := c.Len()
	if p >= n {
		return n
	}
	switch u {
	case UnitCharacter:
		return p + 1
	case UnitLine:
		i := sort.Search(len(c.Lines), func(i int) bool { return c.Lines[i].Start > p })
		if i < len(c.Lines) {
			return c.Lines[i].Start
		}
		return n
	case UnitParagraph:
		return firstAfter(c.Paras, p, n)
	case UnitWord:
		return firstAfter(c.Words, p, n)
	}
	return n
}

// Prev returns the point one unit before p, or 0 when there is none. A word unit
// follows the editor's Ctrl+Left: it moves to the start of the word before p,
// past any separator.
func (c *TextContent) Prev(u TextUnit, p int) int {
	if p <= 0 {
		return 0
	}
	switch u {
	case UnitCharacter:
		return p - 1
	case UnitLine:
		i := sort.Search(len(c.Lines), func(i int) bool { return c.Lines[i].Start >= p }) - 1
		if i >= 0 {
			return c.Lines[i].Start
		}
		return 0
	case UnitParagraph:
		return lastBefore(c.Paras, p)
	case UnitWord:
		return lastBefore(c.Words, p)
	}
	return 0
}

// CaretX returns the x of point p in layout units.
func (c *TextContent) CaretX(p int) float32 { return c.X[p] }

// Nearest returns the point nearest to the position (x, y) in layout units. Above
// the text it is the first point of the first line; below it, the end of the last
// line.
func (c *TextContent) Nearest(x, y float32) int {
	li := sort.Search(len(c.Lines), func(i int) bool { return c.Lines[i].Top+c.Lines[i].Height > y })
	li = min(max(li, 0), len(c.Lines)-1)
	ln := c.Lines[li]
	// The points of the line run up to the next line's first point, which
	// belongs to the next line. The last line also has the end of the text.
	end := c.Len() + 1
	if li+1 < len(c.Lines) {
		end = c.Lines[li+1].Start
	}
	best, bestD := ln.Start, float32(-1)
	for p := ln.Start; p < end; p++ {
		d := c.X[p] - x
		if d < 0 {
			d = -d
		}
		if bestD < 0 || d < bestD {
			best, bestD = p, d
		}
	}
	return best
}

// Read returns the text of the points [from, to), cut to at most max UTF-16 code
// units. A negative max means no limit.
func (c *TextContent) Read(from, to, max int) string {
	return UTF16Prefix(c.Text[c.ByteOf(from):c.ByteOf(to)], max)
}

// SameText reports whether o holds the same text as c. Another model is compared by
// reading its whole text, which only a model of another kind needs.
func (c *TextContent) SameText(o TextModel) bool {
	if oc, ok := o.(*TextContent); ok {
		return oc == c || oc.Text == c.Text
	}
	return o != nil && o.Read(0, o.Len(), -1) == c.Text
}

// firstAfter returns the first start after p, or fallback when there is none.
func firstAfter(starts []int, p, fallback int) int {
	i := sort.Search(len(starts), func(i int) bool { return starts[i] > p })
	if i < len(starts) {
		return starts[i]
	}
	return fallback
}

// lastBefore returns the last start before p, or 0 when there is none.
func lastBefore(starts []int, p int) int {
	i := sort.Search(len(starts), func(i int) bool { return starts[i] >= p }) - 1
	if i >= 0 {
		return starts[i]
	}
	return 0
}

// lastGrapheme returns the point of the last grapheme cluster of a text that is
// not empty.
func lastGrapheme(c TextModel) int { return c.Prev(UnitCharacter, c.Len()) }

// step moves point p by count units, one unit at a time, and returns the point
// reached and the number of units moved, which is negative for a backward move.
// It stops at the ends of the text.
func step(c TextModel, u TextUnit, p, count int) (int, int) {
	moved := 0
	for ; count > 0 && p < c.Len(); count-- {
		p = c.Next(u, p)
		moved++
	}
	for ; count < 0 && p > 0; count++ {
		p = c.Prev(u, p)
		moved--
	}
	return p, moved
}

// TextRange is a span of a text: the points from Start to End, with Start <= End.
// A range with Start == End is degenerate, a position, which is where the
// insertion point of a text field is. A range keeps the TextInfo of the snapshot it
// was made from, so its text and geometry are the ones the client was told about,
// even after the field has changed. Actions on a range (Select, ScrollIntoView)
// act on the current text, and their points are clamped to it.
type TextRange struct {
	// Info is the text that the range lies in.
	Info *TextInfo
	// Start and End are the points where the range begins and ends.
	Start, End int
}

// Content returns the content of the range's text.
func (r TextRange) Content() TextModel { return r.Info.Content }

// Degenerate reports whether the range is a position.
func (r TextRange) Degenerate() bool { return r.Start == r.End }

// Text returns the text between the endpoints.
func (r TextRange) Text() string { return r.Content().Read(r.Start, r.End, -1) }

// GetText returns the text between the endpoints, cut to at most max UTF-16 code
// units, as UI Automation's GetText asks for it. A negative max means no limit.
// A model reads only the requested range.
func (r TextRange) GetText(max int) string { return r.Content().Read(r.Start, r.End, max) }

// Whole returns the range of the whole text of t.
func (t *TextInfo) Whole() TextRange {
	return TextRange{Info: t, Start: 0, End: t.Content.Len()}
}

// CaretRange returns the degenerate range at the caret of t.
func (t *TextInfo) CaretRange() TextRange {
	return TextRange{Info: t, Start: t.Caret, End: t.Caret}
}

// Selection returns the selected range of t: the points between Anchor and Caret.
func (t *TextInfo) Selection() TextRange {
	return TextRange{Info: t, Start: min(t.Anchor, t.Caret), End: max(t.Anchor, t.Caret)}
}

// Expand returns the range expanded to whole units of u, as ExpandToEnclosingUnit
// does: the start moves back to the start of the unit it is in, and the end moves
// forward to the end of the unit that holds its last grapheme. A range that is
// already whole units is unchanged. A degenerate range expands to the unit at its
// position is the unit that holds the grapheme there, or the last grapheme of the text
// at its end. An empty text has no units and expands to nothing.
func (r TextRange) Expand(u TextUnit) TextRange {
	c := r.Content()
	if c.Len() == 0 {
		return TextRange{Info: r.Info}
	}
	last := lastGrapheme(c)
	if r.Degenerate() {
		a, b := c.Span(u, min(r.Start, last))
		return TextRange{Info: r.Info, Start: a, End: b}
	}
	a, _ := c.Span(u, min(r.Start, last))
	_, b := c.Span(u, c.Prev(UnitCharacter, r.End))
	return TextRange{Info: r.Info, Start: a, End: b}
}

// Move moves the range by count units of u, and returns the moved range and the
// number of units moved. A positive count moves forward, a negative one back, and
// zero does nothing. Following Microsoft's rules for ITextRangeProvider::Move:
//
//   - A degenerate range moves its position, and stays degenerate.
//   - Any other range first moves its start back to the start of its unit, then
//     moves that position by count units, and ends one unit after it, so the
//     result is a whole unit.
//   - A move that cannot happen at all leaves the range unchanged.
//
// A word unit moves like a line: a degenerate range moves to the
// start of the nth unit after it, which is where Ctrl+Right stops; a backward move goes
// to the start of the unit before it, which is where Ctrl+Left stops. A non-degenerate
// range starts at the start of its unit and moves from there.
// The count is the number of units actually moved, which is less than requested
// at the ends of the text. A backward move returns a negative count.
func (r TextRange) Move(u TextUnit, count int) (TextRange, int) {
	c := r.Content()
	if count == 0 || c.Len() == 0 {
		return r, 0
	}
	if r.Degenerate() {
		p, moved := step(c, u, r.Start, count)
		if moved == 0 {
			return r, 0
		}
		return TextRange{Info: r.Info, Start: p, End: p}, moved
	}
	a, _ := c.Span(u, min(r.Start, lastGrapheme(c)))
	p, moved := step(c, u, a, count)
	if moved == 0 {
		return r, 0
	}
	return TextRange{Info: r.Info, Start: p, End: c.Next(u, p)}, moved
}

// MoveEndpoint moves one endpoint of the range by count units of u, and returns
// the range and the number of units moved, with the sign convention of Move. The
// endpoint moves to the next unit boundary in that direction. If it passes the
// other endpoint, the other endpoint moves with it, so the range stays ordered
// and becomes degenerate, as ITextRangeProvider::MoveEndpointByUnit describes.
// start selects the start endpoint; otherwise the end endpoint moves.
func (r TextRange) MoveEndpoint(start bool, u TextUnit, count int) (TextRange, int) {
	c := r.Content()
	p := r.End
	if start {
		p = r.Start
	}
	q, moved := step(c, u, p, count)
	if moved == 0 {
		return r, 0
	}
	return r.withEndpoint(start, q), moved
}

// MoveEndpointByRange sets one endpoint of the range to an endpoint of target,
// and keeps the range ordered as MoveEndpoint does.
func (r TextRange) MoveEndpointByRange(start bool, target TextRange, targetStart bool) TextRange {
	p := target.End
	if targetStart {
		p = target.Start
	}
	return r.withEndpoint(start, p)
}

// withEndpoint returns the range with one endpoint set to p. If the other
// endpoint is on the far side of p, it is moved to p too.
func (r TextRange) withEndpoint(start bool, p int) TextRange {
	if start {
		r.Start = p
		r.End = max(r.End, p)
	} else {
		r.End = p
		r.Start = min(r.Start, p)
	}
	return r
}

// CompareEndpoints compares an endpoint of r with an endpoint of target. It
// returns -1, 0 or 1 as the first point is before, at, or after the second. The
// ranges must be of the same text; the magnitude is not defined.
func (r TextRange) CompareEndpoints(start bool, target TextRange, targetStart bool) int {
	p, q := r.End, target.End
	if start {
		p = r.Start
	}
	if targetStart {
		q = target.Start
	}
	switch {
	case p < q:
		return -1
	case p > q:
		return 1
	}
	return 0
}

// Same reports whether two ranges have the same text and the same endpoints.
// The text is compared by its TextInfo, since a snapshot's text is one object.
func (r TextRange) Same(o TextRange) bool {
	return r.Info.Content == o.Info.Content && r.Start == o.Start && r.End == o.End
}

// Find returns the first range (or the last, when backward is set) within r
// whose text is needle. Matches start on grapheme boundaries, and end on one.
// Case is ignored when ignoreCase is set, by simple Unicode case folding. An
// empty needle matches nothing.
func (r TextRange) Find(needle string, backward, ignoreCase bool) (TextRange, bool) {
	c := r.Content()
	if needle == "" {
		return TextRange{}, false
	}
	nr := []rune(needle)
	// The search works on the text of the range, with offsets from its start.
	s := c.Read(r.Start, r.End, -1)
	base := c.ByteOf(r.Start)
	match := func(p int) (TextRange, bool) {
		end, ok := matchFold(s, c.ByteOf(p)-base, len(s), nr, ignoreCase)
		if !ok {
			return TextRange{}, false
		}
		q := c.PointOf(end + base)
		if c.ByteOf(q) != end+base {
			q = c.Next(UnitCharacter, q) // the match ends inside a grapheme cluster; take the whole cluster
		}
		return TextRange{Info: r.Info, Start: p, End: min(q, r.End)}, true
	}
	if backward {
		p := r.End
		for p > r.Start {
			p = c.Prev(UnitCharacter, p)
			if m, ok := match(p); ok {
				return m, true
			}
		}
		return TextRange{}, false
	}
	for p := r.Start; p < r.End; p = c.Next(UnitCharacter, p) {
		if m, ok := match(p); ok {
			return m, true
		}
	}
	return TextRange{}, false
}

// matchFold reports whether needle matches s starting at byte from, not reading
// past byte limit, and returns the byte offset after the match.
func matchFold(s string, from, limit int, needle []rune, ignoreCase bool) (int, bool) {
	i := from
	for _, want := range needle {
		if i >= limit {
			return 0, false
		}
		got, n := utf8.DecodeRuneInString(s[i:limit])
		if got != want && !(ignoreCase && foldEqual(got, want)) {
			return 0, false
		}
		i += n
	}
	return i, true
}

// foldEqual reports whether two runes are equal under simple case folding.
func foldEqual(a, b rune) bool {
	for f := unicode.SimpleFold(a); f != a; f = unicode.SimpleFold(f) {
		if f == b {
			return true
		}
	}
	return false
}

// Rects returns the bounding rectangles of the range, in physical pixels in the
// window: one per visual line it touches, clipped to the visible part of the
// field. A degenerate range, or one that is not visible, has none.
func (r TextRange) Rects() []Rect {
	if r.Degenerate() {
		return nil
	}
	t, c := r.Info, r.Content()
	var out []Rect
	for s := r.Start; s < r.End; {
		ln, end := c.Line(s)
		if end <= s {
			break
		}
		e := min(r.End, end)
		x0 := c.CaretX(s)
		x1 := ln.Right
		if e < end {
			x1 = c.CaretX(e)
		}
		if x1 < x0 {
			x0, x1 = x1, x0
		}
		rect := t.layoutRect(x0, ln.Top, x1-x0, ln.Height)
		rect = intersect(rect, t.Clip)
		if !rect.Empty() {
			out = append(out, rect)
		}
		s = end
	}
	return out
}

// layoutRect converts a rectangle in layout units to physical pixels in the window.
func (t *TextInfo) layoutRect(x, y, w, h float32) Rect {
	return Rect{X: t.OriginX + x*t.Scale, Y: t.OriginY + y*t.Scale, W: w * t.Scale, H: h * t.Scale}
}

// intersect returns the part of r that lies inside c.
func intersect(r, c Rect) Rect {
	x0, y0 := max(r.X, c.X), max(r.Y, c.Y)
	x1, y1 := min(r.X+r.W, c.X+c.W), min(r.Y+r.H, c.Y+c.H)
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// PointAt returns the point nearest to the position (x, y), in physical pixels in
// the window, as a click or hover would find it. Above the text it is the first
// point of the first line; below it, the end of the last line.
func (t *TextInfo) PointAt(x, y float32) int {
	if t.Scale <= 0 {
		return t.Caret
	}
	return t.Content.Nearest((x-t.OriginX)/t.Scale, (y-t.OriginY)/t.Scale)
}

// UTF16Prefix returns the longest prefix of s that has at most max UTF-16 code
// units, and never splits a surrogate pair. A negative max means no limit.
func UTF16Prefix(s string, max int) string {
	if max < 0 {
		return s
	}
	n := 0
	for i, r := range s {
		units := 1
		if r >= 0x10000 {
			units = 2
		}
		if n+units > max {
			return s[:i]
		}
		n += units
	}
	return s
}

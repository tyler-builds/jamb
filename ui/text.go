package ui

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TextElement is a run of text. Unset properties (color, size, weight...)
// are inherited from the nearest ancestor that sets them, then the theme.
type TextElement struct {
	text     string
	style    Style
	maxLines int
	noWrap   bool
	italic   bool
	key      string
	// selectable lets the user select and copy the text (see Selectable).
	selectable bool
	// segs and highlights make the text rich (see Rich and Highlight).
	segs       []Segment
	highlights []highlightRange
}

// highlightRange is a Highlight call: byte offsets into the text.
type highlightRange struct {
	start, end int
	hl         Highlight
}

// isRich reports whether the text has segments or highlights.
func (t *TextElement) isRich() bool { return t.segs != nil || len(t.highlights) > 0 }

// Text creates a text element.
func Text(s string) *TextElement { return &TextElement{text: s} }

// Textf creates a text element with fmt.Sprintf formatting.
func Textf(format string, args ...any) *TextElement {
	return &TextElement{text: fmt.Sprintf(format, args...)}
}

func (*TextElement) isElement()           {}
func (t *TextElement) elementKey() string { return t.key }

// Key gives the element a stable identity among its siblings.
func (t *TextElement) Key(k string) *TextElement { t.key = k; return t }

// FontSize sets the font size in logical pixels.
func (t *TextElement) FontSize(px float32) *TextElement { t.style.FontSize = px; return t }

// Weight sets the font weight.
func (t *TextElement) Weight(w FontWeight) *TextElement { t.style.FontWeight = w; return t }

// Medium, Semibold and Bold are shorthands for Weight.
func (t *TextElement) Medium() *TextElement   { return t.Weight(FontMedium) }
func (t *TextElement) Semibold() *TextElement { return t.Weight(FontSemibold) }
func (t *TextElement) Bold() *TextElement     { return t.Weight(FontBold) }

// TextColor sets the text color.
func (t *TextElement) TextColor(c Color) *TextElement { t.style.TextColor = c; return t }

// Italic sets the whole text in the italic face (upright when the family has none).
func (t *TextElement) Italic() *TextElement { t.italic = true; return t }

// Highlight styles the bytes [start, end) of the text, with offsets from
// strings.Index on the same string. Later highlights win field by field over
// earlier ones and over the segments of a Rich text. It panics on offsets
// outside the text or inside a UTF-8 sequence.
func (t *TextElement) Highlight(start, end int, h Highlight) *TextElement {
	checkRange(t.text, start, end)
	t.highlights = append(t.highlights, highlightRange{start, end, h})
	return t
}

// HighlightMatches styles every case-insensitive occurrence of query in the
// text with h, as a search does. A blank query leaves the text as it is. For a
// Rich text the joined text is searched. Offsets are into the text itself, so
// text whose lowercase form has another length (İ, the Kelvin sign) still
// matches correctly.
func (t *TextElement) HighlightMatches(query string, h Highlight) *TextElement {
	for _, r := range matchRanges(t.text, query) {
		t.Highlight(r[0], r[1], h)
	}
	return t
}

// matchRanges returns the byte ranges [start, end) of s where the query q
// occurs, ignoring case. The offsets index s itself: lowering a string can
// change its length (İ and the Kelvin sign K do), so the comparison is rune by
// rune on s rather than on a lowered copy.
func matchRanges(s, q string) [][2]int {
	qr := []rune(strings.TrimSpace(q))
	if len(qr) == 0 {
		return nil
	}
	var out [][2]int
	for i := 0; i < len(s); {
		end, ok := matchAt(s, i, qr)
		if !ok {
			_, n := utf8.DecodeRuneInString(s[i:])
			i += n
			continue
		}
		out = append(out, [2]int{i, end})
		i = end
	}
	return out
}

// matchAt reports whether the runes q occur in s starting at byte i, and the
// byte offset just after them.
func matchAt(s string, i int, q []rune) (int, bool) {
	for _, want := range q {
		if i >= len(s) {
			return 0, false
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		if unicode.ToLower(r) != unicode.ToLower(want) {
			return 0, false
		}
		i += n
	}
	return i, true
}

// Mono uses the monospace font.
func (t *TextElement) Mono() *TextElement { t.style.FontFamily = "mono"; return t }

// Font selects a font family registered with the text system.
func (t *TextElement) Font(family string) *TextElement { t.style.FontFamily = family; return t }

// LineHeight sets the line height as a multiple of the font size.
func (t *TextElement) LineHeight(m float32) *TextElement { t.style.LineHeight = m; return t }

// Align aligns lines within the element: Start, Center or End.
func (t *TextElement) Align(a Alignment) *TextElement { t.style.TextAlign = a; return t }

// MaxLines limits the number of lines, ending with an ellipsis.
func (t *TextElement) MaxLines(n int) *TextElement { t.maxLines = n; return t }

// Truncate keeps the text on one line, ending with an ellipsis if it does not fit.
func (t *TextElement) Truncate() *TextElement {
	t.noWrap, t.maxLines = true, 1
	t.style.MinWidth = Px(0)
	return t
}

// NoWrap keeps the text on one line.
func (t *TextElement) NoWrap() *TextElement { t.noWrap = true; return t }

// Grow lets the text take free space on its parent's main axis.
func (t *TextElement) Grow(f float32) *TextElement { t.style.Grow = f; return t }

// Width sets a fixed width, or a Percent or Full width of the parent. Text has no
// defaults of its own, so a size set here is the one the text has (see Style).
func (t *TextElement) Width(l Length) *TextElement { t.style.Width = l; return t }

// MinWidth sets the narrowest the text may shrink to in a shrinking row. The
// automatic minimum is the widest word (see Shrinking and minimum sizes).
func (t *TextElement) MinWidth(l Length) *TextElement { t.style.MinWidth = l; return t }

// MaxWidth sets the widest the text may grow to.
func (t *TextElement) MaxWidth(l Length) *TextElement { t.style.MaxWidth = l; return t }

// Style merges layout and text fields of s.
func (t *TextElement) Style(s Style) *TextElement { t.style = t.style.Merge(s); return t }

package rope

import (
	"bytes"
	"unicode/utf8"
)

// newlines returns the number of line feeds in the text.
func (t Text) newlines() int {
	if t.root == nil {
		return 0
	}
	return t.root.lines
}

// LineCount returns the number of lines. A text with n line feeds has n+1
// lines, and the last line may be empty.
func (t Text) LineCount() int {
	return t.newlines() + 1
}

// LineStart returns the byte offset of the first byte of the zero-based line.
// It panics if line is out of range.
func (t Text) LineStart(line int) int {
	if line < 0 || line > t.newlines() {
		panic("rope: line out of range")
	}
	if line == 0 {
		return 0
	}
	// Find the line-th line feed, counting from one. The byte after it starts
	// line.
	n, base, k := t.root, 0, line
	for !n.isLeaf() {
		if k <= n.left.lines {
			n = n.left
		} else {
			k -= n.left.lines
			base += n.left.bytes
			n = n.right
		}
	}
	pos := 0
	for ; k > 0; k-- {
		pos += bytes.IndexByte(n.data[pos:], '\n') + 1
	}
	return base + pos
}

// PointAt returns the row and the byte column of byte offset off. It panics if
// off is out of range. The column is the distance in bytes from the start of
// the row, and it includes the '\r' of a CRLF break.
func (t Text) PointAt(off int) Point {
	t.checkOffset(off)
	row := t.lineOf(off)
	return Point{Row: row, Column: off - t.LineStart(row)}
}

// OffsetOf returns the byte offset of point p. It is the inverse of PointAt and
// costs two line lookups, which is O(log n). Clamping is not applied: a row
// outside [0, LineCount()-1], a negative column, or a column past the line feed
// that ends the row returns ErrRange. The valid columns of a row run from 0 to
// the byte offset of its line feed, inclusive, so a point may address the '\r'
// or '\n' of a break, as PointAt returns it. The last row has no line feed, so
// its valid columns run to Len(). A column that falls inside a UTF-8 sequence
// returns ErrBoundary.
func (t Text) OffsetOf(p Point) (int, error) {
	if p.Row < 0 || p.Row > t.newlines() || p.Column < 0 {
		return 0, ErrRange
	}
	start := t.LineStart(p.Row)
	end := t.Len()
	if p.Row < t.newlines() {
		end = t.LineStart(p.Row+1) - 1
	}
	if p.Column > end-start {
		return 0, ErrRange
	}
	off := start + p.Column
	if !t.IsBoundary(off) {
		return 0, ErrBoundary
	}
	return off, nil
}

// lineOf returns the number of line feeds before byte offset off.
func (t Text) lineOf(off int) int {
	if t.root == nil {
		return 0
	}
	n, line := t.root, 0
	for !n.isLeaf() {
		if off < n.left.bytes {
			n = n.left
		} else {
			off -= n.left.bytes
			line += n.left.lines
			n = n.right
		}
	}
	return line + bytes.Count(n.data[:off], lineFeed)
}

// IsBoundary reports whether off is a valid position for an edit. That is, off
// is in [0, t.Len()] and does not fall inside a UTF-8 sequence.
func (t Text) IsBoundary(off int) bool {
	if off < 0 || off > t.Len() {
		return false
	}
	if off == t.Len() {
		return true
	}
	leaf, start := t.locate(off)
	return !isContinuation(leaf.data[off-start])
}

// UTF16Len returns the length of the text in UTF-16 code units.
func (t Text) UTF16Len() int {
	if t.root == nil {
		return 0
	}
	return t.root.units
}

// ByteToUTF16 returns the UTF-16 code unit offset of byte offset off. It panics
// if off is out of range or is not on a character boundary.
func (t Text) ByteToUTF16(off int) int {
	t.checkOffset(off)
	if !t.IsBoundary(off) {
		panic("rope: offset is not on a character boundary")
	}
	if t.root == nil {
		return 0
	}
	n, units := t.root, 0
	for !n.isLeaf() {
		if off < n.left.bytes {
			n = n.left
		} else {
			off -= n.left.bytes
			units += n.left.units
			n = n.right
		}
	}
	return units + utf16Units(n.data[:off])
}

// UTF16ToByte returns the byte offset of UTF-16 code unit offset u. It returns
// ErrRange if u is outside [0, UTF16Len()], and ErrBoundary if u falls between
// the two code units of a surrogate pair.
func (t Text) UTF16ToByte(u int) (int, error) {
	if u < 0 || u > t.UTF16Len() {
		return 0, ErrRange
	}
	if t.root == nil {
		return 0, nil
	}
	n, off := t.root, 0
	for !n.isLeaf() {
		if u < n.left.units {
			n = n.left
		} else {
			u -= n.left.units
			off += n.left.bytes
			n = n.right
		}
	}
	i := 0
	for u > 0 {
		_, size := utf8.DecodeRune(n.data[i:])
		w := 1
		if size == utf8.UTFMax {
			w = 2
		}
		if u < w {
			return 0, ErrBoundary
		}
		u -= w
		i += size
	}
	return off + i, nil
}

// utf16Units returns the number of UTF-16 code units in the valid UTF-8
// sequence b. A four-byte sequence encodes a surrogate pair and counts as two
// units. Continuation bytes add nothing.
func utf16Units(b []byte) int {
	units := 0
	for _, c := range b {
		switch {
		case c < utf8.RuneSelf:
			units++
		case c >= 0xF0:
			units += 2
		case c >= 0xC0:
			units++
		}
	}
	return units
}

// advance returns the point reached by moving from start across text. It is
// used to compute the new end point of an edit without rescanning the rope.
func advance(start Point, text []byte) Point {
	lines := bytes.Count(text, lineFeed)
	if lines == 0 {
		return Point{Row: start.Row, Column: start.Column + len(text)}
	}
	return Point{Row: start.Row + lines, Column: len(text) - bytes.LastIndexByte(text, '\n') - 1}
}

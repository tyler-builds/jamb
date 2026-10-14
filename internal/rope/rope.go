// Package rope implements a persistent rope for UTF-8 text, the text buffer
// behind a code editor.
//
// Structure. A rope is a height-balanced (AVL) binary tree whose leaves hold
// immutable byte chunks of 512 to 2048 bytes. Each node caches the byte
// length, the number of line feeds, and the number of UTF-16 code units of its
// subtree. A descent from the root therefore converts a byte offset to a line,
// a line to its start offset, and a byte offset to a UTF-16 offset in O(log n).
// The UTF-16 and column conversions then scan at most one leaf.
//
// Persistence. An edit copies only the nodes on the path to the leaves it
// touches; every other node is shared with earlier versions. Nodes are never
// mutated after construction, so Text, the immutable value returned by
// Rope.Snapshot, is a single pointer. A background parser can read a snapshot
// while the editor keeps applying edits to the Rope that produced it.
//
// Alternatives. A gap buffer or a piece table makes local edits cheap, but a
// snapshot would have to copy the buffer or pin the edit history. A persistent
// B-tree with wide nodes has the same asymptotic bounds with fewer levels, but
// each edit must copy whole child arrays at every level. The binary tree is
// deeper, but each copy is a single small node, and the code is considerably
// smaller.
//
// Line breaks. A line ends at '\n'. The byte pair "\r\n" is one line break, and
// the '\r' belongs to that break, so the content of a line excludes it. A '\r'
// that is not followed by '\n' is ordinary text. Columns count bytes from the
// start of the line and include the '\r' of a CRLF break, which matches how
// tree-sitter counts columns. The last line runs from the final '\n' to the end
// of the text and may be empty, so a text with n line feeds has n+1 lines.
//
// Offsets. Offsets are byte offsets. Insert and Delete return ErrRange,
// ErrBoundary, or ErrInvalidUTF8 and leave the rope unchanged when an offset is
// out of range, falls inside a UTF-8 sequence, or the inserted text is not valid
// UTF-8. Read methods panic on an out-of-range offset, as slice indexing does.
// The conversion from UTF-16 offsets returns an error instead, because such
// offsets usually come from an external client. Grapheme clusters are not
// tracked; callers that must not split a cluster should snap offsets with
// IsBoundary and their own segmentation.
//
// Concurrency. Text is safe for concurrent use by any number of goroutines. A
// Rope is not safe for concurrent use, so the caller must serialize its
// mutations and take snapshots from the same goroutine.
package rope

import (
	"errors"
	"unicode/utf8"
)

var (
	// ErrRange reports an offset or a range that lies outside the text.
	ErrRange = errors.New("rope: offset out of range")

	// ErrBoundary reports an offset inside a UTF-8 sequence, or a UTF-16 offset
	// inside a surrogate pair.
	ErrBoundary = errors.New("rope: offset is not on a character boundary")

	// ErrInvalidUTF8 reports text that is not valid UTF-8.
	ErrInvalidUTF8 = errors.New("rope: text is not valid UTF-8")
)

// Point is a zero-based line number and a byte column within that line.
type Point struct {
	Row    int
	Column int
}

// Edit describes a replacement applied to a Rope. Its fields match the
// arguments of tree-sitter's TSInputEdit, so the value can be passed to
// ts_tree_edit as it is. The start point is the same in the old and the new
// text. The old end point is measured in the old text and the new end point in
// the new text.
type Edit struct {
	StartByte   int
	OldEndByte  int
	NewEndByte  int
	StartPoint  Point
	OldEndPoint Point
	NewEndPoint Point
}

// Rope is a mutable text buffer. It embeds Text, so every read method of Text
// is also available on a Rope. The zero value is an empty rope.
type Rope struct {
	Text
}

// New returns a rope that holds a copy of text. It returns ErrInvalidUTF8 if
// text is not valid UTF-8.
func New(text []byte) (*Rope, error) {
	if !utf8.Valid(text) {
		return nil, ErrInvalidUTF8
	}
	return &Rope{Text{root: balanced(chunk(text))}}, nil
}

// NewString returns a rope that holds a copy of text. It returns
// ErrInvalidUTF8 if text is not valid UTF-8.
func NewString(text string) (*Rope, error) {
	if !utf8.ValidString(text) {
		return nil, ErrInvalidUTF8
	}
	return &Rope{Text{root: balanced(chunk(text))}}, nil
}

// Snapshot returns the current text as an immutable value. The snapshot is not
// affected by later edits to r, and it may be read from other goroutines while
// r continues to be edited. Taking a snapshot costs O(1).
func (r *Rope) Snapshot() Text {
	return r.Text
}

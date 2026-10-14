package rope

import (
	"io"
	"iter"
	"strings"
)

// Text is an immutable snapshot of the text of a Rope. The zero value is the
// empty text. Text is safe for concurrent use.
type Text struct {
	root *node
}

// Len returns the length of the text in bytes.
func (t Text) Len() int {
	if t.root == nil {
		return 0
	}
	return t.root.bytes
}

// checkOffset panics if off is outside [0, t.Len()].
func (t Text) checkOffset(off int) {
	if off < 0 || off > t.Len() {
		panic("rope: offset out of range")
	}
}

// locate returns the leaf that holds byte off and the byte offset of the leaf's
// first byte. An offset equal to t.Len() resolves to the last leaf. The text
// must not be empty.
func (t Text) locate(off int) (*node, int) {
	n, start := t.root, 0
	for !n.isLeaf() {
		if off < n.left.bytes {
			n = n.left
		} else {
			off -= n.left.bytes
			start += n.left.bytes
			n = n.right
		}
	}
	return n, start
}

// String returns the text as a string. It copies the text.
func (t Text) String() string {
	var b strings.Builder
	b.Grow(t.Len())
	for c := range t.Chunks(0) {
		b.Write(c)
	}
	return b.String()
}

// Bytes returns a copy of the text.
func (t Text) Bytes() []byte {
	out := make([]byte, 0, t.Len())
	for c := range t.Chunks(0) {
		out = append(out, c...)
	}
	return out
}

// Slice returns a copy of the bytes in [from, to). It panics if the range is
// out of bounds.
func (t Text) Slice(from, to int) []byte {
	t.checkOffset(from)
	t.checkOffset(to)
	if from > to {
		panic("rope: invalid range")
	}
	out := make([]byte, 0, to-from)
	for c := range t.Chunks(from) {
		if rest := to - from - len(out); len(c) >= rest {
			return append(out, c[:rest]...)
		}
		out = append(out, c...)
	}
	return out
}

// Chunks returns an iterator over the text from byte offset from to the end.
// Each yielded slice is a view of internal storage, so the caller must not
// modify it. The storage is never modified after construction, so the caller
// may keep a slice. Concatenating the chunks yields the text from from onward.
// Chunks panics if from is out of range.
func (t Text) Chunks(from int) iter.Seq[[]byte] {
	t.checkOffset(from)
	return func(yield func([]byte) bool) {
		walk(t.root, from, yield)
	}
}

// walk yields the leaf bytes of n from offset from onward, in order. It stops
// early and returns false when yield returns false.
func walk(n *node, from int, yield func([]byte) bool) bool {
	if n == nil {
		return true
	}
	if n.isLeaf() {
		if from >= len(n.data) {
			return true
		}
		return yield(n.data[from:])
	}
	if from < n.left.bytes {
		if !walk(n.left, from, yield) {
			return false
		}
		from = 0
	} else {
		from -= n.left.bytes
	}
	return walk(n.right, from, yield)
}

// Reader returns an io.Reader over the text from byte offset from to the end.
// Reader panics if from is out of range.
func (t Text) Reader(from int) io.Reader {
	t.checkOffset(from)
	return &reader{text: t, off: from}
}

// reader implements io.Reader over a Text. Each call to Read locates the leaf
// at the current offset, so the reader holds no tree state.
type reader struct {
	text Text
	off  int
}

// Read copies bytes from the text into p. It returns io.EOF at the end of the
// text.
func (r *reader) Read(p []byte) (int, error) {
	if r.off >= r.text.Len() {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	leaf, start := r.text.locate(r.off)
	n := copy(p, leaf.data[r.off-start:])
	r.off += n
	return n, nil
}

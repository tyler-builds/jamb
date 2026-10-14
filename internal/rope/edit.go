package rope

import (
	"bytes"
	"unicode/utf8"
)

// Insert inserts text before byte offset off and returns the edit record. It
// returns ErrRange, ErrBoundary, or ErrInvalidUTF8 and leaves the rope unchanged
// if the arguments are invalid.
func (r *Rope) Insert(off int, text []byte) (Edit, error) {
	return r.Replace(off, off, text)
}

// Delete removes the bytes in [from, to) and returns the edit record. It
// returns ErrRange or ErrBoundary and leaves the rope unchanged if the range is
// invalid.
func (r *Rope) Delete(from, to int) (Edit, error) {
	return r.Replace(from, to, nil)
}

// Replace substitutes b for the bytes in [from, to) as a single edit and returns
// one edit record. It has the same validation and atomicity as Insert and
// Delete: if any argument is invalid, it returns ErrRange, ErrBoundary, or
// ErrInvalidUTF8 and leaves the rope unchanged. Edits that stay inside one leaf
// take a fast path that copies only the path to that leaf. Other edits split the
// tree at the two offsets and rejoin the pieces.
func (r *Rope) Replace(from, to int, b []byte) (Edit, error) {
	if from < 0 || from > to || to > r.Len() {
		return Edit{}, ErrRange
	}
	if !r.IsBoundary(from) || !r.IsBoundary(to) {
		return Edit{}, ErrBoundary
	}
	if !utf8.Valid(b) {
		return Edit{}, ErrInvalidUTF8
	}
	start := r.PointAt(from)
	e := Edit{
		StartByte:   from,
		OldEndByte:  to,
		NewEndByte:  from + len(b),
		StartPoint:  start,
		OldEndPoint: r.PointAt(to),
		NewEndPoint: advance(start, b),
	}
	if from == to && len(b) == 0 {
		return e, nil
	}
	if !r.fastReplace(from, to, b) {
		r.slowReplace(from, to, b)
	}
	return e, nil
}

// fastReplace applies an edit that lies inside one leaf and leaves that leaf
// within the leaf size limits. It reports false when the edit needs the general
// path. The line and UTF-16 aggregates of the new leaf are derived from those
// of the old leaf and the changed bytes, so the cost does not depend on the
// leaf size.
func (r *Rope) fastReplace(from, to int, text []byte) bool {
	if r.root == nil {
		return false
	}
	leaf, start := r.locate(from)
	local, end := from-start, to-start
	if end > len(leaf.data) {
		return false
	}
	n := len(leaf.data) - (to - from) + len(text)
	if n == 0 || n > maxLeaf || (n < minLeaf && !r.root.isLeaf()) {
		return false
	}
	removed := leaf.data[local:end]
	data := make([]byte, n)
	copy(data, leaf.data[:local])
	copy(data[local:], text)
	copy(data[local+len(text):], leaf.data[end:])
	r.root = replaceLeaf(r.root, from, &node{
		data:   data,
		bytes:  n,
		lines:  leaf.lines - bytes.Count(removed, lineFeed) + bytes.Count(text, lineFeed),
		units:  leaf.units - utf16Units(removed) + utf16Units(text),
		height: 1,
	})
	return true
}

// slowReplace applies an edit by splitting the tree at from and to and joining
// the pieces around the new text. The split creates two pieces that may be
// shorter than minLeaf, at the left and right edges of the edit. Those pieces
// are merged into the new text and re-chunked, so every leaf keeps its size
// bounds. If the merged text is still shorter than minLeaf, one more whole leaf
// is taken from a neighbor, because a short leaf between full leaves would
// break the invariant.
func (r *Rope) slowReplace(from, to int, text []byte) {
	left, rest := split(r.root, from)
	_, right := split(rest, to-from)
	var head, tail []byte
	if left != nil && (lastLeaf(left).bytes < minLeaf || len(text) < minLeaf) {
		last := lastLeaf(left)
		left, _ = split(left, left.bytes-last.bytes)
		head = last.data
	}
	if right != nil && (firstLeaf(right).bytes < minLeaf || len(head)+len(text) < minLeaf) {
		first := firstLeaf(right)
		_, right = split(right, first.bytes)
		tail = first.data
	}
	if len(head)+len(text)+len(tail) < minLeaf {
		if left != nil {
			last := lastLeaf(left)
			left, _ = split(left, left.bytes-last.bytes)
			head = concat(last.data, head)
		} else if right != nil {
			first := firstLeaf(right)
			_, right = split(right, first.bytes)
			tail = concat(tail, first.data)
		}
	}
	r.root = join(join(left, balanced(chunk(concat(head, text, tail)))), right)
}

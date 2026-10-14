package rope

import "bytes"

// Leaf sizes in bytes. Every leaf holds between minLeaf and maxLeaf bytes,
// except that the only leaf of a text may hold fewer. A leaf made by splitting
// a larger text holds at least targetLeaf bytes.
const (
	minLeaf    = 512
	targetLeaf = 1024
	maxLeaf    = 2048
)

var lineFeed = []byte{'\n'}

// node is an immutable tree node. A leaf has no children and holds data. An
// internal node has two children and no data. The aggregate fields describe the
// whole subtree, so they are valid for every node without further work.
type node struct {
	left, right *node
	data        []byte
	bytes       int // length in bytes
	lines       int // number of '\n' bytes
	units       int // length in UTF-16 code units
	height      int // a leaf has height 1, and a nil node has height 0
}

func (n *node) isLeaf() bool {
	return n.left == nil
}

func height(n *node) int {
	if n == nil {
		return 0
	}
	return n.height
}

// newLeaf returns a leaf that takes ownership of data. The data must be
// non-empty valid UTF-8, and it must not be modified afterwards.
func newLeaf(data []byte) *node {
	return &node{
		data:   data,
		bytes:  len(data),
		lines:  bytes.Count(data, lineFeed),
		units:  utf16Units(data),
		height: 1,
	}
}

// mk returns the internal node with children l and r, which must not be nil.
func mk(l, r *node) *node {
	return &node{
		left:   l,
		right:  r,
		bytes:  l.bytes + r.bytes,
		lines:  l.lines + r.lines,
		units:  l.units + r.units,
		height: max(l.height, r.height) + 1,
	}
}

// rotateLeft turns (a, (b1, b2)) into ((a, b1), b2). The right child must be
// internal.
func rotateLeft(n *node) *node {
	r := n.right
	return mk(mk(n.left, r.left), r.right)
}

// rotateRight turns ((a1, a2), b) into (a1, (a2, b)). The left child must be
// internal.
func rotateRight(n *node) *node {
	l := n.left
	return mk(l.left, mk(l.right, n.right))
}

// join returns the concatenation of l and r, rebalanced. Both may be nil. The
// cost is proportional to the difference in height, so join and split stay
// O(log n).
func join(l, r *node) *node {
	switch {
	case l == nil:
		return r
	case r == nil:
		return l
	case height(l) > height(r)+1:
		return joinRight(l, r)
	case height(r) > height(l)+1:
		return joinLeft(l, r)
	}
	return mk(l, r)
}

// joinRight concatenates l and r when l is at least two levels taller than r.
func joinRight(l, r *node) *node {
	ll, c := l.left, l.right
	if height(c) <= height(r)+1 {
		t := mk(c, r)
		if height(t) <= height(ll)+1 {
			return mk(ll, t)
		}
		return rotateLeft(mk(ll, rotateRight(t)))
	}
	t := joinRight(c, r)
	if height(t) <= height(ll)+1 {
		return mk(ll, t)
	}
	return rotateLeft(mk(ll, t))
}

// joinLeft concatenates l and r when r is at least two levels taller than l.
func joinLeft(l, r *node) *node {
	c, rr := r.left, r.right
	if height(c) <= height(l)+1 {
		t := mk(l, c)
		if height(t) <= height(rr)+1 {
			return mk(t, rr)
		}
		return rotateRight(mk(rotateLeft(t), rr))
	}
	t := joinLeft(l, c)
	if height(t) <= height(rr)+1 {
		return mk(t, rr)
	}
	return rotateRight(mk(t, rr))
}

// split returns the first off bytes of n and the remaining bytes, where
// 0 <= off <= n.bytes. The only new leaves are the two pieces at the cut, and
// they are copies, so the pieces do not keep the bytes of the original leaf
// alive.
func split(n *node, off int) (*node, *node) {
	switch {
	case n == nil:
		return nil, nil
	case off == 0:
		return nil, n
	case off == n.bytes:
		return n, nil
	case n.isLeaf():
		return newLeaf(clone(n.data[:off])), newLeaf(clone(n.data[off:]))
	case off < n.left.bytes:
		l, r := split(n.left, off)
		return l, join(r, n.right)
	}
	l, r := split(n.right, off-n.left.bytes)
	return join(n.left, l), r
}

// balanced builds a balanced tree from leaves given in text order.
func balanced(leaves []*node) *node {
	switch len(leaves) {
	case 0:
		return nil
	case 1:
		return leaves[0]
	}
	m := len(leaves) / 2
	return mk(balanced(leaves[:m]), balanced(leaves[m:]))
}

// firstLeaf returns the leftmost leaf of n.
func firstLeaf(n *node) *node {
	for !n.isLeaf() {
		n = n.left
	}
	return n
}

// lastLeaf returns the rightmost leaf of n.
func lastLeaf(n *node) *node {
	for !n.isLeaf() {
		n = n.right
	}
	return n
}

// replaceLeaf returns a copy of n in which the leaf that holds byte off is
// replaced by leaf. The replacement must have the same height as the leaf it
// replaces, so the tree stays balanced without rebalancing.
func replaceLeaf(n *node, off int, leaf *node) *node {
	if n.isLeaf() {
		return leaf
	}
	if off < n.left.bytes {
		return mk(replaceLeaf(n.left, off, leaf), n.right)
	}
	return mk(n.left, replaceLeaf(n.right, off-n.left.bytes, leaf))
}

// chunk copies s into leaves. A text of n bytes is split into max(1, n/targetLeaf)
// pieces of nearly equal size, so each piece holds at least targetLeaf bytes
// before the cut adjustment. Each cut point is moved forward past any
// continuation bytes, which moves it by at most three bytes, so every leaf starts
// and ends on a character boundary. The input must be valid UTF-8. Every leaf
// holds between minLeaf and maxLeaf bytes, except that a text shorter than
// minLeaf forms a lone leaf.
func chunk[S string | []byte](s S) []*node {
	n := len(s)
	if n == 0 {
		return nil
	}
	k := max(1, n/targetLeaf)
	leaves := make([]*node, 0, k)
	start := 0
	for i := 1; i <= k; i++ {
		end := n
		if i < k {
			end = n * i / k
			for end < n && isContinuation(s[end]) {
				end++
			}
		}
		data := make([]byte, end-start)
		copy(data, s[start:end])
		leaves = append(leaves, newLeaf(data))
		start = end
	}
	return leaves
}

// isContinuation reports whether c is a UTF-8 continuation byte.
func isContinuation(c byte) bool {
	return c&0xC0 == 0x80
}

// clone returns a copy of b with its own backing array.
func clone(b []byte) []byte {
	return concat(b)
}

// concat returns a new slice that holds the bytes of parts in order. The result
// never shares a backing array with any part, so leaf data is never appended to.
func concat(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

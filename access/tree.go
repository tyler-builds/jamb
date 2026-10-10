package access

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// Tree is an immutable snapshot of the access tree. The ui runtime publishes a
// new Tree when the UI changed; readers keep the one they loaded, so no lock is
// needed to read a snapshot from another thread.
type Tree struct {
	Root  ID // the window's node, RootID
	Focus ID // the node with keyboard focus, or zero
	nodes map[ID]*Node
	order []ID // every node, depth first from the root, in reading order
}

// NewTree builds a snapshot from nodes. The root is the node whose ID is
// RootID; the nodes it lists as children, directly or through other nodes,
// are the tree. Nodes that are not reachable from the root are dropped.
// Ownership of the nodes passes to the tree.
func NewTree(focus ID, nodes []*Node) *Tree {
	all := make(map[ID]*Node, len(nodes))
	for _, n := range nodes {
		all[n.ID] = n
	}
	t := &Tree{Root: RootID, Focus: focus, nodes: make(map[ID]*Node, len(nodes))}
	t.walk(all, RootID)
	return t
}

// walk adds the node with the ID from all, and then its children, depth first.
// A node already added is skipped, so a cycle cannot loop.
func (t *Tree) walk(all map[ID]*Node, id ID) {
	n := all[id]
	if n == nil || t.nodes[id] != nil {
		return
	}
	t.nodes[id] = n
	t.order = append(t.order, id)
	for _, c := range n.Children {
		t.walk(all, c)
	}
}

// Node returns the node with the given ID, or nil if the tree has none. A nil
// tree has no nodes.
func (t *Tree) Node(id ID) *Node {
	if t == nil {
		return nil
	}
	return t.nodes[id]
}

// Len returns the number of nodes in the tree.
func (t *Tree) Len() int {
	if t == nil {
		return 0
	}
	return len(t.order)
}

// Walk calls fn for each node in reading order, depth first, until fn returns
// false.
func (t *Tree) Walk(fn func(*Node) bool) {
	if t == nil {
		return
	}
	for _, id := range t.order {
		if !fn(t.nodes[id]) {
			return
		}
	}
}

// Find returns the first node, in reading order, that has the role and the
// name, or nil.
func (t *Tree) Find(role Role, name string) *Node {
	var found *Node
	t.Walk(func(n *Node) bool {
		if n.Role == role && n.Name == name {
			found = n
			return false
		}
		return true
	})
	return found
}

// Dump returns the tree as indented text, one node per line, for tests and
// the development tools:
//
//	window "Demo" (1)
//	  button "New project" [focusable] @12,40 120x34 (4096)
//
// Bounds are physical pixels. The ID is in parentheses.
func (t *Tree) Dump() string {
	var b strings.Builder
	var rec func(id ID, depth int)
	rec = func(id ID, depth int) {
		n := t.Node(id)
		if n == nil {
			return
		}
		fmt.Fprintf(&b, "%s%s", strings.Repeat("  ", depth), n.Role)
		if n.Name != "" {
			fmt.Fprintf(&b, " %q", n.Name)
		}
		if s := n.States.String(); s != "" {
			fmt.Fprintf(&b, " [%s]", s)
		}
		if n.PosInSet > 0 {
			fmt.Fprintf(&b, " %d of %d", n.PosInSet, n.SetSize)
		}
		if !n.Bounds.Empty() {
			fmt.Fprintf(&b, " @%.0f,%.0f %.0fx%.0f", n.Bounds.X, n.Bounds.Y, n.Bounds.W, n.Bounds.H)
		}
		fmt.Fprintf(&b, " (%d)\n", n.ID)
		for _, c := range n.Children {
			rec(c, depth+1)
		}
	}
	rec(t.Root, 0)
	return b.String()
}

// Update describes what changed between two snapshots. Adapters use it to
// raise events and to decide which of their cached objects to refresh.
type Update struct {
	Focus        ID   // the focused node in the new snapshot
	FocusChanged bool // the focused node differs from the previous snapshot
	Added        []ID // nodes in the new snapshot that were not in the previous one
	Removed      []ID // nodes in the previous snapshot that are gone
	Changed      []ID // nodes present in both whose content differs
	// Structure is set when nodes were added, removed or re-parented.
	Structure bool
	// Announcements are the status messages queued since the last snapshot.
	Announcements []Announcement
}

// Diff compares two snapshots. prev may be nil, in which case every node is
// Added. The ID lists are sorted, so the result is deterministic.
func Diff(prev, next *Tree) Update {
	var u Update
	if next != nil {
		u.Focus = next.Focus
	}
	var prevFocus ID
	if prev != nil {
		prevFocus = prev.Focus
	}
	u.FocusChanged = u.Focus != prevFocus
	next.Walk(func(n *Node) bool {
		old := prev.Node(n.ID)
		switch {
		case old == nil:
			u.Added = append(u.Added, n.ID)
			u.Structure = true
		case old.Parent != n.Parent:
			u.Changed = append(u.Changed, n.ID)
			u.Structure = true
		case !old.Equal(n):
			u.Changed = append(u.Changed, n.ID)
		}
		return true
	})
	prev.Walk(func(n *Node) bool {
		if next.Node(n.ID) == nil {
			u.Removed = append(u.Removed, n.ID)
			u.Structure = true
		}
		return true
	})
	slices.Sort(u.Added)
	slices.Sort(u.Removed)
	slices.Sort(u.Changed)
	return u
}

// Bridge connects the ui runtime that owns a window's tree with the platform
// adapter that exposes it. The runtime publishes snapshots on the UI thread;
// the adapter reads the latest one from any thread, asks for activation when
// an assistive technology first connects, and sends actions back, which the
// runtime runs on the UI thread.
//
// The zero value is not usable; create one with NewBridge.
type Bridge struct {
	tree atomic.Pointer[Tree]

	mu       sync.Mutex
	active   atomic.Bool
	activate func()
	perform  func(ID, Action)
	listener func(Update)
}

// NewBridge creates a bridge with no runtime or adapter attached.
func NewBridge() *Bridge { return &Bridge{} }

// Tree returns the latest published snapshot, or nil before the first one.
// Safe from any goroutine.
func (b *Bridge) Tree() *Tree { return b.tree.Load() }

// Active reports whether an assistive technology has asked for the tree. Until
// it has, the runtime builds nothing. Safe from any goroutine.
func (b *Bridge) Active() bool { return b.active.Load() }

// Activate is called by the adapter, on the UI thread, when an assistive
// technology first asks for the tree. It starts the runtime building the tree,
// once; later calls do nothing.
func (b *Bridge) Activate() {
	b.mu.Lock()
	if b.active.Load() {
		b.mu.Unlock()
		return
	}
	b.active.Store(true)
	fn := b.activate
	b.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Perform asks the runtime to run action a on node id. It returns at once and
// is safe from any goroutine; the runtime runs the action on the UI thread, and
// ignores it if the node no longer exists.
func (b *Bridge) Perform(id ID, a Action) {
	b.mu.Lock()
	fn := b.perform
	b.mu.Unlock()
	if fn != nil {
		fn(id, a)
	}
}

// Publish stores a new snapshot and reports the change to the adapter. The
// runtime calls it on the UI thread.
func (b *Bridge) Publish(t *Tree, u Update) {
	b.tree.Store(t)
	b.mu.Lock()
	fn := b.listener
	b.mu.Unlock()
	if fn != nil {
		fn(u)
	}
}

// SetHooks connects the runtime. activate runs on the UI thread once, when the
// tree is first asked for. perform receives actions from any goroutine.
func (b *Bridge) SetHooks(activate func(), perform func(ID, Action)) {
	b.mu.Lock()
	b.activate, b.perform = activate, perform
	b.mu.Unlock()
}

// Listen registers the adapter's callback, which Publish calls on the UI
// thread with each Update. Only one listener is kept.
func (b *Bridge) Listen(fn func(Update)) {
	b.mu.Lock()
	b.listener = fn
	b.mu.Unlock()
}

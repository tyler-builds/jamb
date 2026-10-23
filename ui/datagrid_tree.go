package ui

import (
	"slices"
	"unsafe"
)

// dgTree is the tree of the rows of a DataGrid with Children (see
// DataGridElement.Children), flattened depth first: each row comes after its parent,
// and the children of a row follow it in the order they were given. The rows are
// indexed by their place in this list, and the grid shows some of them, in the order
// that visible makes.
type dgTree[T any] struct {
	rows   []T            // every row, depth first
	keys   []string       // the key of each row
	level  []int          // the depth of each row, 1 at the top
	parent []int          // the row of the parent of each row, or -1 at the top
	kids   [][]int        // the children of each row, in the order they were given
	roots  []int          // the top rows, in the order they were given
	at     map[string]int // the row with each key
	dup    string         // a key that two rows share, for the warning, or ""

	// kidsBy and rootsBy are kids and roots in the order of the sort (see sortLevels).
	// pos and size are the position of each shown row among its siblings, and the
	// number of its siblings (see visible).
	kidsBy  [][]int
	rootsBy []int
	pos     []int
	size    []int
}

// flattenRows returns the rows of a tree depth first, with their keys, depths, parents
// and children. children is called once for each row.
func flattenRows[T any](rows []T, key func(T) string, children func(T) []T) *dgTree[T] {
	t := &dgTree[T]{at: map[string]int{}}
	var add func(ns []T, level, parent int) []int
	add = func(ns []T, level, parent int) []int {
		idx := make([]int, 0, len(ns))
		for _, r := range ns {
			i := len(t.rows)
			k := key(r)
			if _, ok := t.at[k]; ok && t.dup == "" {
				t.dup = k
			}
			t.at[k] = i
			t.rows = append(t.rows, r)
			t.keys = append(t.keys, k)
			t.level = append(t.level, level)
			t.parent = append(t.parent, parent)
			t.kids = append(t.kids, nil)
			idx = append(idx, i)
			kids := add(children(r), level+1, i)
			t.kids[i] = kids
		}
		return idx
	}
	t.roots = add(rows, 1, -1)
	t.pos, t.size = make([]int, len(t.rows)), make([]int, len(t.rows))
	return t
}

// reread copies the rows into the tree again, depth first, and reports whether they have
// the shape the tree has: the same keys under the same parents, and the same number of
// children for each row. Then the order of the rows stays as it is, as a flat grid keeps
// its order when a row changes in place. For any other shape it reports false, and the
// tree is flattened again (see ensureTree).
func (t *dgTree[T]) reread(rows []T, key func(T) string, children func(T) []T) bool {
	n := 0
	var walk func(ns []T, parent int) bool
	walk = func(ns []T, parent int) bool {
		ids := t.roots
		if parent >= 0 {
			ids = t.kids[parent]
		}
		if len(ns) != len(ids) {
			return false
		}
		for _, r := range ns {
			i := n
			n++
			if i >= len(t.rows) || t.parent[i] != parent || t.keys[i] != key(r) {
				return false
			}
			t.rows[i] = r
			if !walk(children(r), i) {
				return false
			}
		}
		return true
	}
	return walk(rows, -1) && n == len(t.rows)
}

// sortLevels orders the top rows, and the children of each row, by cmp (descending
// when desc is set), stably, so that a group sorts among the groups and a child among
// its siblings. A nil cmp keeps the order the rows were given in, and so does keepRoots
// for the top rows (see DataGridElement.SortChildrenOnly).
func (t *dgTree[T]) sortLevels(cmp func(a, b T) int, desc, keepRoots bool) {
	if cmp == nil {
		t.kidsBy, t.rootsBy = t.kids, t.roots
		return
	}
	less := func(a, b int) int {
		r := cmp(t.rows[a], t.rows[b])
		if desc {
			return -r
		}
		return r
	}
	t.rootsBy = t.roots
	if !keepRoots {
		t.rootsBy = slices.Clone(t.roots)
		slices.SortStableFunc(t.rootsBy, less)
	}
	t.kidsBy = make([][]int, len(t.kids))
	for i, ks := range t.kids {
		t.kidsBy[i] = ks
		if len(ks) > 1 {
			t.kidsBy[i] = slices.Clone(ks)
			slices.SortStableFunc(t.kidsBy[i], less)
		}
	}
}

// visible returns the rows that are shown, depth first: the top rows, and under each
// open row its children. open is the set of the keys of the open rows. It sets the
// position of each shown row among its siblings, and the number of its siblings.
func (t *dgTree[T]) visible(open map[string]bool) []int {
	out := make([]int, 0, len(t.roots))
	var walk func(ids []int)
	walk = func(ids []int) {
		for p, i := range ids {
			t.pos[i], t.size[i] = p+1, len(ids)
			out = append(out, i)
			if len(t.kids[i]) > 0 && open[t.keys[i]] {
				walk(t.kidsBy[i])
			}
		}
	}
	walk(t.rootsBy)
	return out
}

// ensureTree reads the rows into the tree on each build, as a flat grid reads its rows:
// the rows are copied again, and the children are read for each row, so that a change in
// place shows. The tree is flattened again when the slice is another one or its shape
// has changed. The identity of the rows is their first element, as for the keys of a
// flat grid.
func (sh *dgShared[T]) ensureTree(rows []T, key func(T) string, children func(T) []T) {
	ident := unsafe.Pointer(unsafe.SliceData(rows))
	if sh.tree != nil && sh.treeIdent == ident && sh.tree.reread(rows, key, children) {
		return
	}
	sh.treeIdent = ident
	sh.tree = flattenRows(rows, key, children)
	sh.keys, sh.keyed = sh.tree.keys, false
	sh.n = len(sh.tree.rows)
	sh.ordered = false
}

// updateTree is update for a tree (see DataGridElement.Children), for the tree that
// ensureTree read: the children are sorted within each level when the sort changes or
// the shape does, and the order is the rows shown under the open keys, when either
// changes.
func (sh *dgShared[T]) updateTree(cols []DataColumn[T], by SortState, open []string, keepRoots bool) {
	if sh.ordered && sh.by == by && sh.keepRoots == keepRoots && slices.Equal(sh.open, open) {
		return
	}
	if !sh.ordered || sh.by != by || sh.keepRoots != keepRoots {
		var cmp func(a, b T) int
		for _, c := range cols {
			if by.Key != "" && c.key() == by.Key && c.Compare != nil {
				cmp = c.Compare
				break
			}
		}
		sh.tree.sortLevels(cmp, by.Desc, keepRoots)
	}
	sh.ordered, sh.by, sh.keepRoots = true, by, keepRoots
	sh.showOpen(open)
	sh.dup = sh.tree.dup
}

// showOpen makes the shown rows those under the open keys, and keeps the keys. setOpen
// calls it after each change, so that a key pressed before the next frame reads the rows
// as they will be shown, not as the last frame showed them.
func (sh *dgShared[T]) showOpen(open []string) {
	sh.open = slices.Clone(open)
	sh.openSet = keySetOf(open)
	sh.order = sh.tree.visible(sh.openSet)
	sh.pos = nil
}

// closedAbove returns the keys of the rows above the row with key that are closed in
// open, which must open for the row to be shown. It is nil when the row is not in the
// tree, or is shown already.
func (sh *dgShared[T]) closedAbove(key string, open []string) []string {
	i, ok := sh.tree.at[key]
	if !ok {
		return nil
	}
	set := keySetOf(open)
	var out []string
	for p := sh.tree.parent[i]; p >= 0; p = sh.tree.parent[p] {
		if k := sh.tree.keys[p]; !set[k] {
			out = append(out, k)
		}
	}
	return out
}

// shownKey returns the key of the row with key when it is shown, or else of the nearest
// row above it that is shown, with its position: a collapse that hides the focused row
// moves the focus there. It reports false when the key is not in the tree.
func (sh *dgShared[T]) shownKey(key string) (string, int, bool) {
	i, ok := sh.tree.at[key]
	for ok {
		if q, shown := sh.posOf(sh.tree.keys[i]); shown {
			return sh.tree.keys[i], q, true
		}
		i = sh.tree.parent[i]
		ok = i >= 0
	}
	return "", 0, false
}

// openStore returns the signal that keeps the open rows of a tree: the one of Expanded,
// or the grid's own state. It is nil for a grid without Children.
func (g *DataGridElement[T]) openStore(ctx *Context) *Signal[[]string] {
	switch {
	case g.children == nil:
		return nil
	case g.expanded != nil:
		return g.expanded
	}
	return Local(ctx, "expanded", []string(nil))
}

// refresh updates the keys and the order of the rows (see dgShared.update and
// updateTree), and returns the rows that the order indexes: those given, or for a tree
// every row of it, flattened. A ScrollToRow or FocusCell on a row that a collapse hides
// opens the rows above it first, through store, so that the row is shown.
func (g *DataGridElement[T]) refresh(sh *dgShared[T], store *Signal[[]string], cols []DataColumn[T], by SortState) []T {
	if store == nil {
		sh.update(g.rows, g.key, cols, by)
		return g.rows
	}
	sh.ensureTree(g.rows, g.key, g.children)
	open := store.Get()
	if h := g.gridHandle; h != nil && h.hasRow {
		if add := sh.closedAbove(h.row, open); len(add) > 0 {
			open = append(slices.Clone(open), add...)
			store.Set(open)
			if g.onExpand != nil {
				for _, k := range add {
					g.onExpand(k)
				}
			}
		}
	}
	sh.updateTree(cols, by, open, g.childrenOnly)
	return sh.tree.rows
}

// setOpen opens or closes the row with key in store, the open rows of a tree, and shows
// the rows that the change opens or hides (see showOpen). A change calls OnExpand or
// OnCollapse, once for the row.
func (g *DataGridElement[T]) setOpen(store *Signal[[]string], sh *dgShared[T], key string, v bool) {
	keys := store.Peek()
	if slices.Contains(keys, key) == v {
		return
	}
	if v {
		keys = append(slices.Clone(keys), key)
	} else {
		keys = slices.DeleteFunc(slices.Clone(keys), func(k string) bool { return k == key })
	}
	store.Set(keys)
	sh.showOpen(keys)
	if v && g.onExpand != nil {
		g.onExpand(key)
	}
	if !v && g.onCollapse != nil {
		g.onCollapse(key)
	}
}

// treeKey does what Right (dir 1) or Left (dir -1) does on the row at the displayed
// position p, in the tree column. Right opens a closed row, and on an open row moves to
// its first child; Left closes an open row, and on a row that is closed or a leaf moves
// to its parent. It reports whether the key was used, and whether the focus moves, to the
// displayed position to. The open rows are those of sh.openSet, which setOpen keeps as
// the signal has them, so that a second key before a frame sees the first one's change.
func (g *DataGridElement[T]) treeKey(store *Signal[[]string], sh *dgShared[T], p, dir int) (to int, moves, used bool) {
	i := sh.order[p]
	key := sh.keys[i]
	kids := len(sh.tree.kids[i])
	open := sh.openSet[key]
	switch {
	case dir > 0 && kids > 0 && !open:
		g.setOpen(store, sh, key, true)
		return p, false, true
	case dir > 0 && kids > 0 && p+1 < len(sh.order):
		return p + 1, true, true // the first child is the row below
	case dir < 0 && kids > 0 && open:
		g.setOpen(store, sh, key, false)
		return p, false, true
	case dir < 0 && sh.tree.parent[i] >= 0:
		q, ok := sh.posOf(sh.tree.keys[sh.tree.parent[i]])
		return q, ok, ok
	}
	return p, false, false
}

// openSiblings opens the closed rows among the siblings of the row at the displayed
// position p that have children, as the * key does in a Tree.
func (g *DataGridElement[T]) openSiblings(store *Signal[[]string], sh *dgShared[T], p int) {
	i := sh.order[p]
	sibs := sh.tree.rootsBy
	if par := sh.tree.parent[i]; par >= 0 {
		sibs = sh.tree.kidsBy[par]
	}
	// The siblings open in one change, so that the rows are shown once for all of them.
	keys := slices.Clone(store.Peek())
	var added []string
	for _, s := range sibs {
		if k := sh.tree.keys[s]; len(sh.tree.kids[s]) > 0 && !slices.Contains(keys, k) {
			keys = append(keys, k)
			added = append(added, k)
		}
	}
	if len(added) == 0 {
		return
	}
	store.Set(keys)
	sh.showOpen(keys)
	if g.onExpand != nil {
		for _, k := range added {
			g.onExpand(k)
		}
	}
}

package ui

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// TreeNode is one node of a Tree. Key identifies the node in the whole tree:
// the keyboard focus, the selection and the open state are kept by it, so it must
// be unique and stable. Children is nil for a node that cannot have children. For
// any other node it returns the node's children, and it is called only while the
// node is open, so a large tree can load each part when it is shown.
type TreeNode struct {
	// Key identifies the node among its siblings.
	Key string
	// Label is the text of the node.
	Label string
	// Description is muted text after the label, such as a count or a path. Assistive
	// technology reads it as the node's description.
	Description string
	// Icon is an icon shown before the label.
	Icon *icons.Icon
	// Children are the children of the node, shown when it is open.
	Children func() []TreeNode
}

// TreeStyle is the look of each part of a tree. A part's style merges over the
// built-in defaults, then over the theme's Tree, then over the Styles of the
// instance; see docs/styling.md. Row is each visible row, indented by its level
// by Indent pixels per level (a row's own left padding, when its Row style sets
// one, replaces the indent). Chevron is the click target that opens and closes a
// node, Icon the node's icon and Label its text. The states are parts of their
// own: RowHover while the pointer is over a row that is not selected,
// RowSelected for the selected row, and TreeFocusVisible for the tree while it
// has keyboard focus (the ring is drawn on the tree, as the rows are not focused
// themselves). Description is the muted text after a node's label, and a node's
// RenderTrailing content sits after it, at the end of the row.
type TreeStyle struct {
	Row              Style   // each visible row
	RowHover         Style   // a row the pointer is over, if it is not selected
	RowSelected      Style   // the selected row
	Chevron          Style   // the click target of a node's chevron
	Icon             Style   // the icon of a node
	Label            Style   // the text of a node
	Description      Style   // the text after a node's label (see TreeNode.Description)
	TreeFocusVisible Style   // the tree while it has keyboard focus
	Indent           float32 // the indent of each level below the top, in logical pixels
}

// merge returns s with the set fields of o applied on top, part by part.
func (s TreeStyle) merge(o TreeStyle) TreeStyle {
	indent := s.Indent
	if o.Indent != 0 {
		indent = o.Indent
	}
	return TreeStyle{
		Row:              s.Row.Merge(o.Row),
		RowHover:         s.RowHover.Merge(o.RowHover),
		RowSelected:      s.RowSelected.Merge(o.RowSelected),
		Chevron:          s.Chevron.Merge(o.Chevron),
		Icon:             s.Icon.Merge(o.Icon),
		Label:            s.Label.Merge(o.Label),
		Description:      s.Description.Merge(o.Description),
		TreeFocusVisible: s.TreeFocusVisible.Merge(o.TreeFocusVisible),
		Indent:           indent,
	}
}

// plain returns s as high contrast allows it (see plainStyle). The indent is a
// size, so it is kept.
func (s TreeStyle) plain() TreeStyle {
	return TreeStyle{
		Row:              plainStyle(s.Row),
		RowHover:         plainStyle(s.RowHover),
		RowSelected:      plainStyle(s.RowSelected),
		Chevron:          plainStyle(s.Chevron),
		Icon:             plainStyle(s.Icon),
		Label:            plainStyle(s.Label),
		Description:      plainStyle(s.Description),
		TreeFocusVisible: plainStyle(s.TreeFocusVisible),
		Indent:           s.Indent,
	}
}

// treeDefaults returns the built-in look of a tree, from the colors of t. Under
// high contrast the selected row is the Highlight color with HighlightText on it.
func treeDefaults(t *Theme) TreeStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := TreeStyle{
		Row: Style{Direction: Row, Align: Center, Gap: 6, MinHeight: Px(26), NoShrink: true,
			Radius: t.Radius / 2, Padding: Edges{Top: 2, Right: 8, Bottom: 2}, TextColor: t.Text},
		RowHover:    Style{Background: t.SurfaceHover},
		RowSelected: Style{Background: t.AccentSoft},
		Chevron: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(16), Height: Px(16),
			NoShrink: true, TextColor: t.TextMuted},
		Icon:             Style{TextColor: t.TextMuted},
		Description:      Style{TextColor: t.TextMuted, Grow: 1, MinWidth: Px(0)},
		TreeFocusVisible: Style{Shadows: []Shadow{ring}},
		Indent:           18,
	}
	if t.HighContrast {
		d.RowSelected = Style{Background: t.Accent, TextColor: t.AccentText}
	}
	return d
}

// resolveTree returns the look of a tree: the defaults, merged with the theme's
// Tree and then the instance's Styles. Under high contrast the two overrides
// are plain (see plainStyle).
func resolveTree(ctx *Context, inst TreeStyle) TreeStyle {
	t := ctx.Theme()
	return layered(ctx, treeDefaults(t), TreeStyle.merge, TreeStyle.plain,
		themeLayer(t.Tree), instanceLayer(inst))
}

// TreeElement is a hierarchy of nodes that open and close, shown as a list of the
// rows that are visible. Only the rows near the viewport are built, so a tree of
// tens of thousands of nodes costs what its visible part costs. Size it with Style
// (Height, Grow or MaxHeight): a tree with no bounded height shows every row.
//
// The tree is one focusable element, as a Windows tree view is, and its keys act
// on the selected node. Selection follows the keyboard: Up and Down select the
// previous and next visible node and scroll it into view. The keys follow the
// Windows TreeView conventions, and the WAI-ARIA tree view pattern
// (https://www.w3.org/WAI/ARIA/apg/patterns/treeview/):
//
//   - Down and Up move to the next and previous visible node, without wrapping.
//   - Home and End move to the first and last visible node.
//   - Right opens a closed node, and on an open node moves to its first child.
//   - Left closes an open node, and otherwise moves to the parent node.
//   - Enter and Space open and close a node that has children.
//   - * (asterisk) opens all the closed siblings of the selected node.
//   - Typing letters selects the next node whose label starts with them. A pause
//     of a second starts a new search, as in a select.
//
// A click selects a node, and a click on its chevron opens or closes it without
// selecting it. A double-click opens or closes it, as it does on Windows.
// Selection is single.
//
// The open nodes are the tree's own state, unless Expanded gives the tree the keys
// of the open nodes in a signal, which the app can open and close.
type TreeElement struct {
	nodes       []TreeNode
	selected    *Signal[string]
	name        string // the accessible name, see Label
	key         string
	style       Style
	onSelect    func(key string)
	onExpand    func(key string)
	onCollapse  func(key string)
	expanded    *Signal[[]string] // the open keys, when the app keeps them (see Expanded)
	handle      *TreeHandle       // see Handle
	styles      TreeStyle
	renderRow   func(node TreeNode, state PartState) Element
	renderLabel func(node TreeNode, state PartState) Element
	trailing    func(node TreeNode, state PartState) Element // see RenderTrailing
	menu        func(key string) []MenuItem                  // see ContextMenu
	focusHandle *FocusHandle                                 // see FocusHandle
	size        sizing                                       // see the sizing methods
}

// The sizing methods give the tree's outer box a size, over its Style. See Sizing
// in docs/styling.md.
func (tv *TreeElement) Width(l Length) *TreeElement { tv.size.setWidth(l); return tv }

// Height sets the height of the tree.
func (tv *TreeElement) Height(l Length) *TreeElement { tv.size.setHeight(l); return tv }

// MinWidth sets the smallest width of the tree.
func (tv *TreeElement) MinWidth(l Length) *TreeElement { tv.size.setMinWidth(l); return tv }

// MinHeight sets the smallest height of the tree.
func (tv *TreeElement) MinHeight(l Length) *TreeElement { tv.size.setMinHeight(l); return tv }

// MaxWidth sets the largest width of the tree.
func (tv *TreeElement) MaxWidth(l Length) *TreeElement { tv.size.setMaxWidth(l); return tv }

// MaxHeight sets the largest height of the tree.
func (tv *TreeElement) MaxHeight(l Length) *TreeElement { tv.size.setMaxHeight(l); return tv }

// Grow makes the tree take a share of the free space on its parent's main axis.
func (tv *TreeElement) Grow(f float32) *TreeElement { tv.size.setGrow(f); return tv }

// GridSpan sets how many columns and rows the tree covers in its parent's grid.
func (tv *TreeElement) GridSpan(cols, rows int) *TreeElement { tv.size.setSpan(cols, rows); return tv }

// FocusHandle lets code focus the tree (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (tv *TreeElement) FocusHandle(h *FocusHandle) *TreeElement {
	tv.focusHandle = h
	return tv
}

// Tree creates a tree of nodes, with the node whose key equals selected's
// value selected. The nodes shown first are the top level, and every node is
// closed until the user opens it.
//
// With no size, the tree takes the height its parent leaves it, and shows the
// rows that fit in it, however many rows are open. A tree in a column that is
// not bounded shows every row.
func Tree(nodes []TreeNode, selected *Signal[string]) *TreeElement {
	return &TreeElement{nodes: nodes, selected: selected}
}

func (*TreeElement) isElement()            {}
func (tv *TreeElement) elementKey() string { return tv.key }

// Key gives the tree a stable identity among its siblings.
func (tv *TreeElement) Key(k string) *TreeElement { tv.key = k; return tv }

// Label names the tree for assistive technology, such as "Project files".
func (tv *TreeElement) Label(name string) *TreeElement { tv.name = name; return tv }

// Style is merged into the scroll container: size it with Height, Grow or MaxHeight.
func (tv *TreeElement) Style(s Style) *TreeElement { tv.style = tv.style.Merge(s); return tv }

// Styles sets the look of the parts of the tree, for this instance only. Set
// fields win over the theme's Tree and the defaults; see TreeStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (tv *TreeElement) Styles(st TreeStyle) *TreeElement { tv.styles = tv.styles.merge(st); return tv }

// OnChange runs fn with the key of the node after the selection changes, by a
// click, the keys or a typed letter. The signal is set first.
func (tv *TreeElement) OnChange(fn func(key string)) *TreeElement { tv.onSelect = fn; return tv }

// Expanded keeps the keys of the open nodes in keys: the tree sets it when a node
// is opened or closed, and a value the app sets in keys opens those nodes and
// closes the others. A nil keys keeps the open nodes inside the tree. The keys are
// kept in the order the nodes were opened; closing a node keeps the keys of the
// nodes below it, so opening it again shows them open.
func (tv *TreeElement) Expanded(keys *Signal[[]string]) *TreeElement { tv.expanded = keys; return tv }

// OnExpand runs fn with the key of a node after it is opened, by a click, the keys,
// the accessibility actions, a TreeHandle or Reveal. It does not run for the keys
// the app sets in Expanded.
func (tv *TreeElement) OnExpand(fn func(key string)) *TreeElement { tv.onExpand = fn; return tv }

// OnCollapse runs fn with the key of a node after it is closed, from the same
// sources as OnExpand. It does not run for the keys the app sets in Expanded.
func (tv *TreeElement) OnCollapse(fn func(key string)) *TreeElement { tv.onCollapse = fn; return tv }

// Handle connects h, so that code can open, close, reveal and select the nodes of
// the tree. See TreeHandle.
func (tv *TreeElement) Handle(h *TreeHandle) *TreeElement { tv.handle = h; return tv }

// TreeHandle opens, closes, reveals and selects the nodes of a Tree from code, by
// their keys. Create one with NewTreeHandle and pass it to Tree.Handle. A request
// takes effect when the tree is next built, on the next frame, in the order the
// requests were made. A key that names no visible node does nothing, except for
// Reveal and Select, which open the nodes above the key. Call it on the UI thread.
//
// ExpandAll and Reveal call the children of the nodes they open, and so load every
// node below them: a tree that loads its children lazily loads them all.
type TreeHandle struct {
	inst *componentInstance
	ops  []treeOp
}

// treeOp is a request of a TreeHandle.
type treeOp struct {
	kind treeOpKind
	key  string
}

// treeOpKind is the kind of a request of a TreeHandle.
type treeOpKind uint8

const (
	treeOpExpand treeOpKind = iota
	treeOpCollapse
	treeOpExpandAll
	treeOpCollapseAll
	treeOpReveal
	treeOpSelect
)

// NewTreeHandle creates a handle to pass to Tree.Handle.
func NewTreeHandle() *TreeHandle { return &TreeHandle{} }

// Expand opens the node with the key, if it is shown and can have children.
func (h *TreeHandle) Expand(key string) { h.request(treeOp{treeOpExpand, key}) }

// Collapse closes the node with the key, if it is shown and can have children. If
// the selected node was shown only below it, the selection moves to it, and OnChange
// runs.
func (h *TreeHandle) Collapse(key string) { h.request(treeOp{treeOpCollapse, key}) }

// ExpandAll opens every node that can have children.
func (h *TreeHandle) ExpandAll() { h.request(treeOp{kind: treeOpExpandAll}) }

// CollapseAll closes every node. The selection moves to the top-level node that
// holds it, and OnChange runs.
func (h *TreeHandle) CollapseAll() { h.request(treeOp{kind: treeOpCollapseAll}) }

// Reveal opens the nodes above the node with the key, and scrolls it into view. It
// does nothing when the tree has no such node.
func (h *TreeHandle) Reveal(key string) { h.request(treeOp{treeOpReveal, key}) }

// Select reveals the node with the key and selects it. It does not run OnChange.
func (h *TreeHandle) Select(key string) { h.request(treeOp{treeOpSelect, key}) }

func (h *TreeHandle) request(op treeOp) {
	h.ops = append(h.ops, op)
	if h.inst != nil {
		h.inst.invalidate()
	}
}

// RenderRow replaces the icon and the label of each row with what fn returns. The
// row keeps its chevron, its level, key, selection, keys and accessible name, which
// is still the node's label. fn is called again when the row's pointer state
// changes, with the state in the PartState (Selected for the selected row). The
// content is hidden from assistive technology.
func (tv *TreeElement) RenderRow(fn func(node TreeNode, state PartState) Element) *TreeElement {
	tv.renderRow = fn
	return tv
}

// RenderLabel replaces the text of each row with what fn returns, keeping the
// row's icon, level, key, selection and accessible name. It is not called when
// RenderRow is set, which replaces the label too. The content is hidden from
// assistive technology.
func (tv *TreeElement) RenderLabel(fn func(node TreeNode, state PartState) Element) *TreeElement {
	tv.renderLabel = fn
	return tv
}

// RenderTrailing puts what fn returns at the end of each row, after the label and the
// description, such as a count or a badge. The content is hidden from assistive
// technology, so the row's name and description still say what the node is. fn is
// called again when the row's pointer state changes, with the state in the PartState.
func (tv *TreeElement) RenderTrailing(fn func(node TreeNode, state PartState) Element) *TreeElement {
	tv.trailing = fn
	return tv
}

// ContextMenu opens a menu on a row, built by build with the row's key. A right click
// selects the row first, as File Explorer does, and Shift+F10 or the Menu key opens the
// menu of the selected row while the tree has focus. An empty result opens nothing.
func (tv *TreeElement) ContextMenu(build func(key string) []MenuItem) *TreeElement {
	tv.menu = build
	return tv
}

// treeRow is one visible row of a tree: a node, its place in the tree and its state.
type treeRow struct {
	key, label, description string
	icon                    *icons.Icon
	level                   int // 1 at the top
	parent                  int // the index of the parent row, or -1 at the top
	pos, size               int // the position of the node among its siblings, and their number
	expandable              bool
	open                    bool
}

// flattenTree returns the visible rows of nodes, depth first. The children of a
// node are asked for only when the node is open. capHint is the number of rows
// the last flatten made, if known, so that the rows of a large tree are not
// copied as they grow; the top-level nodes are always room enough.
func flattenTree(nodes []TreeNode, open map[string]bool, capHint int) []treeRow {
	rows := make([]treeRow, 0, max(capHint, len(nodes)))
	var add func(ns []TreeNode, level, parent int)
	add = func(ns []TreeNode, level, parent int) {
		for i, n := range ns {
			idx := len(rows)
			r := treeRow{key: n.Key, label: n.Label, description: n.Description, icon: n.Icon, level: level, parent: parent,
				pos: i + 1, size: len(ns), expandable: n.Children != nil}
			r.open = r.expandable && open[n.Key]
			rows = append(rows, r)
			if r.open {
				add(n.Children(), level+1, idx)
			}
		}
	}
	add(nodes, 1, -1)
	return rows
}

// rowOf returns the index of the row with key, or -1.
func rowOf(rows []treeRow, key string) int {
	for i, r := range rows {
		if r.key == key {
			return i
		}
	}
	return -1
}

// treeTypeahead is the row that a typed text selects, searching from the row
// after from and wrapping around. A single letter moves past from, so that a
// repeated letter cycles through the rows that start with it.
func treeTypeahead(rows []treeRow, from int, text string) int {
	n := len(rows)
	if n == 0 {
		return -1
	}
	text = strings.ToLower(text)
	start := max(from, 0)
	if utf8.RuneCountInString(text) == 1 {
		start = from + 1
	}
	for k := range n {
		i := ((start+k)%n + n) % n
		if strings.HasPrefix(strings.ToLower(rows[i].label), text) {
			return i
		}
	}
	return -1
}

// setOpen opens or closes the node with key in store, the open keys of the tree. A
// change calls OnExpand or OnCollapse, once for the node.
func (tv *TreeElement) setOpen(store *Signal[[]string], key string, v bool) {
	keys := store.Peek()
	if slices.Contains(keys, key) == v {
		return
	}
	if v {
		store.Set(append(slices.Clone(keys), key))
		if tv.onExpand != nil {
			tv.onExpand(key)
		}
		return
	}
	store.Set(slices.DeleteFunc(slices.Clone(keys), func(k string) bool { return k == key }))
	if tv.onCollapse != nil {
		tv.onCollapse(key)
	}
}

// applyTreeOps runs the requests of h in order, against the open keys in store, and
// rebuilds the visible rows after each. It returns the key of the last node to
// reveal, if there was one, and whether a request closed a node, which may hide the
// selection (see moveSelection).
func (tv *TreeElement) applyTreeOps(h *TreeHandle, store *Signal[[]string]) (reveal string, collapsed bool) {
	ops := h.ops
	h.ops = nil
	for _, op := range ops {
		rows := flattenTree(tv.nodes, keySetOf(store.Peek()), 0)
		switch op.kind {
		case treeOpExpand:
			if i := rowOf(rows, op.key); i >= 0 && rows[i].expandable {
				tv.setOpen(store, op.key, true)
			}
		case treeOpCollapse:
			if i := rowOf(rows, op.key); i >= 0 && rows[i].expandable {
				tv.setOpen(store, op.key, false)
				collapsed = true
			}
		case treeOpExpandAll:
			for _, k := range allExpandable(tv.nodes) {
				tv.setOpen(store, k, true)
			}
		case treeOpCollapseAll:
			for _, k := range store.Peek() {
				tv.setOpen(store, k, false)
			}
			collapsed = true
		case treeOpReveal, treeOpSelect:
			path := treePath(tv.nodes, op.key)
			if len(path) == 0 {
				break
			}
			for _, k := range path[:len(path)-1] {
				tv.setOpen(store, k, true)
			}
			reveal = op.key
			if op.kind == treeOpSelect {
				tv.selected.Set(op.key)
			}
		}
	}
	return reveal, collapsed
}

// moveSelection selects the nearest node above the selection that is shown, when
// the selection is hidden: a collapse code requested hid it. It runs OnChange.
func (tv *TreeElement) moveSelection(rows []treeRow) {
	sel := tv.selected.Peek()
	if sel == "" || rowOf(rows, sel) >= 0 {
		return
	}
	path := treePath(tv.nodes, sel)
	for i := len(path) - 2; i >= 0; i-- {
		if rowOf(rows, path[i]) >= 0 {
			tv.selected.Set(path[i])
			if tv.onSelect != nil {
				tv.onSelect(path[i])
			}
			return
		}
	}
}

// treePath returns the keys of the nodes from the top of nodes down to the node
// with key, or nil when there is no such node. It calls the children of the nodes
// on the way.
func treePath(nodes []TreeNode, key string) []string {
	for _, n := range nodes {
		if n.Key == key {
			return []string{n.Key}
		}
		if n.Children != nil {
			if p := treePath(n.Children(), key); p != nil {
				return append([]string{n.Key}, p...)
			}
		}
	}
	return nil
}

// allExpandable returns the keys of every node that can have children, at any
// depth. It calls the children of every such node.
func allExpandable(nodes []TreeNode) []string {
	var out []string
	for _, n := range nodes {
		if n.Children != nil {
			out = append(out, n.Key)
			out = append(out, allExpandable(n.Children())...)
		}
	}
	return out
}

// keySetOf returns the keys as a set.
func keySetOf(keys []string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func (tv *TreeElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		rt := ctx.rt
		st := resolveTree(ctx, tv.styles)
		sel := tv.selected.Get()
		// The open keys are the app's signal when Expanded set one, else the tree's state.
		store := Local(ctx, "open", []string(nil))
		if tv.expanded != nil {
			store = tv.expanded
		}
		handle := Local(ctx, "handle", NewListHandle()).Peek()
		typed := Local(ctx, "typed", selectType{})
		hint := Local(ctx, "rowsHint", new(int)).Peek()
		rows := flattenTree(tv.nodes, keySetOf(store.Get()), *hint)
		if h := tv.handle; h != nil {
			h.inst = ctx.inst
			if len(h.ops) > 0 {
				reveal, collapsed := tv.applyTreeOps(h, store)
				rows = flattenTree(tv.nodes, keySetOf(store.Peek()), len(rows))
				if collapsed {
					tv.moveSelection(rows)
				}
				if i := rowOf(rows, reveal); reveal != "" && i >= 0 {
					handle.ScrollToItem(i)
				}
				sel = tv.selected.Peek()
			}
		}
		cur := rowOf(rows, sel)

		setOpen := func(key string, v bool) { tv.setOpen(store, key, v) }
		// choose selects the node at row i, and scrolls it into view.
		choose := func(i int) {
			if i < 0 || i >= len(rows) {
				return
			}
			handle.ScrollToItem(i)
			if rows[i].key == sel {
				return
			}
			tv.selected.Set(rows[i].key)
			if tv.onSelect != nil {
				tv.onSelect(rows[i].key)
			}
		}
		// openSiblings opens the closed siblings of row i that have children.
		openSiblings := func(i int) {
			for _, r := range rows {
				if r.parent == rows[i].parent && r.expandable && !r.open {
					tv.setOpen(store, r.key, true)
				}
			}
		}

		keys := func(e *KeyEvent) {
			if e.Mods != 0 {
				return
			}
			n := len(rows)
			if n == 0 {
				return
			}
			i := cur
			switch e.Key {
			case input.KeyDown:
				choose(min(i+1, n-1)) // from no row, the first
			case input.KeyUp:
				choose(max(i-1, 0))
			case input.KeyHome:
				choose(0)
			case input.KeyEnd:
				choose(n - 1)
			case input.KeyRight:
				if i < 0 {
					break
				}
				switch {
				case rows[i].expandable && !rows[i].open:
					setOpen(rows[i].key, true)
				case rows[i].open && i+1 < n && rows[i+1].parent == i:
					choose(i + 1)
				}
			case input.KeyLeft:
				if i < 0 {
					break
				}
				switch {
				case rows[i].open:
					setOpen(rows[i].key, false)
				case rows[i].parent >= 0:
					choose(rows[i].parent)
				}
			case input.KeyEnter, input.KeySpace:
				if e.Repeat || i < 0 || !rows[i].expandable {
					return
				}
				setOpen(rows[i].key, !rows[i].open)
			default:
				return
			}
			e.StopPropagation()
		}
		typeahead := func(text string) {
			if text == "*" {
				if cur >= 0 {
					openSiblings(cur)
				}
				return
			}
			r, _ := utf8.DecodeRuneInString(text)
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return
			}
			now := rt.now()
			prev := typed.Peek()
			s := text
			if now.Sub(prev.at) < selectTypeahead {
				s = prev.text + text
			}
			typed.Set(selectType{text: s, at: now})
			if i := treeTypeahead(rows, cur, s); i >= 0 {
				choose(i)
			}
		}

		*hint = len(rows)
		list := UniformList(len(rows), func(i int) Element {
			return tv.row(&st, t, rows[i], i, sel, setOpen, choose)
		})
		list.Key(tv.key).SelectionContainer().Style(Style{Grow: 1}).Handle(handle)
		list.ownPos = true
		list.decorate = func(c *DivElement) {
			// The ring is the tree's own, drawn on the list while it has focus.
			focus := Style{Shadows: []Shadow{{Spread: 2, Color: t.FocusRing}}}.Merge(st.TreeFocusVisible)
			c.asPart("Tree.Tree").Role(access.RoleTree).Focusable().FocusVisible(focus).
				OnKeyDown(keys).OnTextInput(typeahead)
			if tv.focusHandle != nil {
				c.FocusHandle(tv.focusHandle)
			}
			if tv.name != "" {
				c.Label(tv.name)
			}
		}
		// The list sits in a column that takes the tree's style, so that the list
		// fills the space the style gives it. The column may shrink below its rows,
		// so that the list windows them to the space instead of building them all.
		root := tv.size.apply(Div().Column().MinHeight(Px(0)).Style(tv.style)).Child(list)
		if tv.menu != nil {
			// Shift+F10 and the Menu key open the menu of the selected row, from the tree's
			// own box. A pointer press that is not on a row opens nothing.
			root.ContextMenu(func() []MenuItem {
				if !rt.keyboardModality || cur < 0 {
					return nil
				}
				return tv.menu(rows[cur].key)
			})
		}
		return root
	})
}

// row is the element of one visible row. Its position is among its siblings, and
// its level is its depth. The row that is selected is the highlighted one, so it
// is the row that has accessibility focus while the tree has keyboard focus.
func (tv *TreeElement) row(st *TreeStyle, t *Theme, r treeRow, i int, sel string,
	setOpen func(string, bool), choose func(int)) Element {
	on := r.key == sel
	node := TreeNode{Key: r.key, Label: r.label, Description: r.description, Icon: r.icon}
	// draw returns the row in the state ps describes. The level sets the left
	// padding, which the Row style replaces when it sets one.
	draw := func(ps PartState) *DivElement {
		row := Div().asPart("Tree.Row").Key(r.key).
			Style(Style{Padding: Edges{Left: 4 + float32(r.level-1)*st.Indent}}).Style(st.Row)
		if on {
			row.Style(st.RowSelected)
		} else {
			row.Hover(st.RowHover)
		}

		// The chevron is a click target of its own: a click on it opens or closes the
		// node and does not select it, as the Windows glyph does.
		chev := Div().asPart("Tree.Chevron").Style(st.Chevron)
		chev.access().inert = true // the row is the item; assistive technology uses its Expand action
		if r.expandable {
			glyph := icons.ChevronRight
			if r.open {
				glyph = icons.ChevronDown
			}
			key := r.key
			open := r.open
			chev.Child(Icon(glyph).Size(12)).Cursor(CursorPointer).
				OnClick(func() { setOpen(key, !open) })
		}
		row.Child(chev)
		if tv.renderRow != nil {
			row.Child(Div().AccessHidden().Child(tv.renderRow(node, ps)))
		} else {
			if r.icon != nil {
				row.Child(Div().asPart("Tree.Icon").Style(st.Icon).Child(Icon(r.icon).Size(16)))
			}
			label := Div().asPart("Tree.Label").Style(st.Label)
			if tv.renderLabel != nil {
				label.Child(Div().AccessHidden().Child(tv.renderLabel(node, ps)))
			} else {
				label.Child(Text(r.label).NoWrap())
			}
			row.Child(label)
		}
		// The description and the trailing content are hidden from assistive technology:
		// the row reports the description, and the trailing content is for sighted users.
		if r.description != "" {
			row.Child(Div().asPart("Tree.Description").AccessHidden().Style(st.Description).
				Child(Text(r.description).NoWrap().Truncate()))
		}
		if tv.trailing != nil {
			if r.description == "" {
				row.Child(Div().Grow(1)) // pushes the trailing content to the row's end
			}
			row.Child(Div().asPart("Tree.Trailing").AccessHidden().Row().Align(Center).
				Style(Style{NoShrink: true}).Child(tv.trailing(node, ps)))
		}

		a := row.access()
		row.Role(access.RoleTreeItem).Label(r.label).AccessSelected(on)
		if r.description != "" {
			row.Description(r.description)
		}
		if tv.menu != nil {
			key := r.key
			row.ContextMenu(func() []MenuItem {
				choose(i) // a right click selects the row first, as File Explorer does
				return tv.menu(key)
			})
		}
		a.setPos, a.setSize, a.level = r.pos, r.size, r.level
		a.highlighted = on
		if r.expandable {
			row.AccessExpanded(r.open)
			key := r.key
			a.setExpanded = func(v bool) { setOpen(key, v) }
		}
		key, open := r.key, r.open
		row.Cursor(CursorPointer).OnClick(func() { choose(i) }).
			OnDoubleClick(func() {
				if r.expandable {
					setOpen(key, !open)
				}
			})
		return row
	}
	if tv.renderRow == nil && tv.renderLabel == nil && tv.trailing == nil {
		return draw(PartState{Selected: on, HighContrast: t.HighContrast})
	}
	// With a render slot the row is its own component, so that it re-renders when
	// its pointer state changes, and the slot gets the new state.
	return Component(func(ctx *Context) Element {
		return draw(PartState{Hovered: ctx.hovered(), Pressed: ctx.pressed(), Focused: on,
			Selected: on, HighContrast: t.HighContrast})
	}).Key(r.key)
}

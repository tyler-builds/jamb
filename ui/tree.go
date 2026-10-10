package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// TreeNode is one node of a TreeView. Key identifies the node in the whole tree:
// the keyboard focus, the selection and the open state are kept by it, so it must
// be unique and stable. Children is nil for a node that cannot have children. For
// any other node it returns the node's children, and it is called only while the
// node is open, so a large tree can load each part when it is shown.
type TreeNode struct {
	Key      string
	Label    string
	Icon     *icons.Icon
	Children func() []TreeNode
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
type TreeElement struct {
	nodes    []TreeNode
	selected *Signal[string]
	name     string // the accessible name, see Label
	key      string
	style    Style
	onSelect func(key string)
}

// TreeView creates a tree of nodes, with the node whose key equals selected's
// value selected. The nodes shown first are the top level, and every node is
// closed until the user opens it.
func TreeView(nodes []TreeNode, selected *Signal[string]) *TreeElement {
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

// OnSelect runs fn with the key of the node after the selection changes, by a
// click, the keys or a typed letter. The signal is set first.
func (tv *TreeElement) OnSelect(fn func(key string)) *TreeElement { tv.onSelect = fn; return tv }

// treeRow is one visible row of a tree: a node, its place in the tree and its state.
type treeRow struct {
	key, label string
	icon       *icons.Icon
	level      int // 1 at the top
	parent     int // the index of the parent row, or -1 at the top
	pos, size  int // the position of the node among its siblings, and their number
	expandable bool
	open       bool
}

// flattenTree returns the visible rows of nodes, depth first. The children of a
// node are asked for only when the node is open.
func flattenTree(nodes []TreeNode, open map[string]bool) []treeRow {
	var rows []treeRow
	var add func(ns []TreeNode, level, parent int)
	add = func(ns []TreeNode, level, parent int) {
		for i, n := range ns {
			idx := len(rows)
			r := treeRow{key: n.Key, label: n.Label, icon: n.Icon, level: level, parent: parent,
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

func (tv *TreeElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		rt := ctx.rt
		sel := tv.selected.Get()
		open := Local(ctx, "open", map[string]bool{})
		handle := Local(ctx, "handle", NewListHandle()).Peek()
		typed := Local(ctx, "typed", selectType{})
		rows := flattenTree(tv.nodes, open.Get())
		cur := rowOf(rows, sel)
		ring := Shadow{Spread: 2, Color: t.FocusRing}

		// setOpen opens or closes the node with key, keeping the other nodes' state.
		setOpen := func(key string, v bool) {
			m := open.Peek()
			if m[key] == v {
				return
			}
			next := make(map[string]bool, len(m)+1)
			for k, x := range m {
				next[k] = x
			}
			next[key] = v
			open.Set(next)
		}
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
			m := open.Peek()
			next := make(map[string]bool, len(m)+len(rows))
			for k, x := range m {
				next[k] = x
			}
			changed := false
			for _, r := range rows {
				if r.parent == rows[i].parent && r.expandable && !r.open {
					next[r.key] = true
					changed = true
				}
			}
			if changed {
				open.Set(next)
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

		list := UniformList(len(rows), func(i int) Element {
			return tv.row(t, rows[i], i, sel, setOpen, choose)
		})
		list.Key(tv.key).SelectionContainer().Style(Style{Grow: 1}).Handle(handle)
		list.ownPos = true
		list.decorate = func(c *DivElement) {
			c.Role(access.RoleTree).Focusable().FocusVisible(Style{Shadows: []Shadow{ring}}).
				OnKeyDown(keys).OnTextInput(typeahead)
			if tv.name != "" {
				c.Label(tv.name)
			}
		}
		// The list sits in a column that takes the tree's style, so that the list
		// fills the space the style gives it.
		return Div().Column().Style(tv.style).Child(list)
	})
}

// row is the element of one visible row. Its position is among its siblings, and
// its level is its depth. The row that is selected is the highlighted one, so it
// is the row that has accessibility focus while the tree has keyboard focus.
func (tv *TreeElement) row(t *Theme, r treeRow, i int, sel string,
	setOpen func(string, bool), choose func(int)) Element {
	on := r.key == sel
	text := Text(r.label).NoWrap().Color(t.Text)
	if on {
		text.Semibold()
	}
	row := Div().Key(r.key).Row().Align(Center).Gap(6).MinHeight(Px(26)).Shrink(0).Radius(t.Radius / 2).
		PaddingEdges(Edges{Top: 2, Right: 8, Bottom: 2, Left: 4 + float32(r.level-1)*18})
	if on {
		if t.HighContrast {
			row.Background(t.Accent)
			text.Color(t.AccentText)
		} else {
			row.Background(t.AccentSoft)
		}
	} else {
		row.Hover(Style{Background: t.SurfaceHover})
	}

	// The chevron is a click target of its own: a click on it opens or closes the
	// node and does not select it, as the Windows glyph does.
	chev := Div().Size(Px(16), Px(16)).Center().Shrink(0)
	chev.access().inert = true // the row is the item; assistive technology uses its Expand action
	if r.expandable {
		glyph := icons.ChevronRight
		if r.open {
			glyph = icons.ChevronDown
		}
		key := r.key
		open := r.open
		chev.Child(Icon(glyph).Size(12).Color(t.TextMuted)).Cursor(CursorPointer).
			OnClick(func() { setOpen(key, !open) })
	}
	row.Child(chev)
	if r.icon != nil {
		row.Child(Icon(r.icon).Size(16).Color(t.TextMuted))
	}
	row.Child(text)

	a := row.access()
	row.Role(access.RoleTreeItem).Label(r.label).Selected(on)
	a.setPos, a.setSize, a.level = r.pos, r.size, r.level
	a.highlighted = on
	if r.expandable {
		row.Expanded(r.open)
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

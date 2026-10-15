package ui

import (
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
// themselves).
type TreeStyle struct {
	Row              Style   // each visible row
	RowHover         Style   // a row the pointer is over, if it is not selected
	RowSelected      Style   // the selected row
	Chevron          Style   // the click target of a node's chevron
	Icon             Style   // the icon of a node
	Label            Style   // the text of a node
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
type TreeElement struct {
	nodes       []TreeNode
	selected    *Signal[string]
	name        string // the accessible name, see Label
	key         string
	style       Style
	onSelect    func(key string)
	styles      TreeStyle
	renderRow   func(node TreeNode, state PartState) Element
	renderLabel func(node TreeNode, state PartState) Element
	focusHandle *FocusHandle // see FocusHandle
	size        sizing       // see the sizing methods
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
		st := resolveTree(ctx, tv.styles)
		sel := tv.selected.Get()
		open := Local(ctx, "open", map[string]bool{})
		handle := Local(ctx, "handle", NewListHandle()).Peek()
		typed := Local(ctx, "typed", selectType{})
		rows := flattenTree(tv.nodes, open.Get())
		cur := rowOf(rows, sel)

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
			return tv.row(st, t, rows[i], i, sel, setOpen, choose)
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
		// fills the space the style gives it.
		return tv.size.apply(Div().Column().Style(tv.style)).Child(list)
	})
}

// row is the element of one visible row. Its position is among its siblings, and
// its level is its depth. The row that is selected is the highlighted one, so it
// is the row that has accessibility focus while the tree has keyboard focus.
func (tv *TreeElement) row(st TreeStyle, t *Theme, r treeRow, i int, sel string,
	setOpen func(string, bool), choose func(int)) Element {
	on := r.key == sel
	node := TreeNode{Key: r.key, Label: r.label, Icon: r.icon}
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

		a := row.access()
		row.Role(access.RoleTreeItem).Label(r.label).AccessSelected(on)
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
	if tv.renderRow == nil && tv.renderLabel == nil {
		return draw(PartState{Selected: on, HighContrast: t.HighContrast})
	}
	// With a render slot the row is its own component, so that it re-renders when
	// its pointer state changes, and the slot gets the new state.
	return Component(func(ctx *Context) Element {
		return draw(PartState{Hovered: ctx.hovered(), Pressed: ctx.pressed(), Focused: on,
			Selected: on, HighContrast: t.HighContrast})
	}).Key(r.key)
}

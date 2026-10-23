package ui

import (
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// listBoxPage is how many items PageUp and PageDown move the focus by. A list box
// builds only the rows in view and does not know how many of them fit, so the
// page is a fixed number of items.
const listBoxPage = 10

// ListBoxStyle is the look of each part of a list box. A part's style merges over
// the built-in defaults, then over the theme's ListBox, then over the Styles of the
// instance; see docs/styling.md. List is the scrolling list that holds the items.
// The states are parts of their own: ItemHover applies to an item the pointer is
// over, if it is not selected, ItemSelected to a selected item, and ItemFocusVisible
// to the item that has keyboard focus while the list has focus. The ring of
// ItemFocusVisible is drawn over the fill of ItemSelected, so an item that is both
// shows both.
type ListBoxStyle struct {
	List             Style // the scrolling list that holds the items
	Item             Style // each item
	ItemHover        Style // an item the pointer is over, if it is not selected
	ItemSelected     Style // a selected item
	ItemFocusVisible Style // the item that has keyboard focus, while the list has focus
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ListBoxStyle) merge(o ListBoxStyle) ListBoxStyle {
	return ListBoxStyle{
		List:             s.List.Merge(o.List),
		Item:             s.Item.Merge(o.Item),
		ItemHover:        s.ItemHover.Merge(o.ItemHover),
		ItemSelected:     s.ItemSelected.Merge(o.ItemSelected),
		ItemFocusVisible: s.ItemFocusVisible.Merge(o.ItemFocusVisible),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ListBoxStyle) plain() ListBoxStyle {
	return ListBoxStyle{
		List:             plainStyle(s.List),
		Item:             plainStyle(s.Item),
		ItemHover:        plainStyle(s.ItemHover),
		ItemSelected:     plainStyle(s.ItemSelected),
		ItemFocusVisible: plainStyle(s.ItemFocusVisible),
	}
}

// listBoxDefaults returns the built-in look of a list box, from the colors of t.
// Under high contrast the selected item is the Accent color with AccentText on it.
// No state changes the font weight: a selected item is shown by its fill.
func listBoxDefaults(t *Theme) ListBoxStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := ListBoxStyle{
		List: Style{Grow: 1, Padding: All(2)},
		Item: Style{Direction: Row, Align: Center, Gap: 8, MinHeight: Px(32), NoShrink: true,
			Radius: 6, Padding: XY(10, 4), TextColor: t.Text},
		ItemHover:        Style{Background: t.SurfaceHover},
		ItemSelected:     Style{Background: t.SurfaceActive},
		ItemFocusVisible: Style{Shadows: []Shadow{ring}},
	}
	if t.HighContrast {
		d.ItemSelected = Style{Background: t.Accent, TextColor: t.AccentText}
	}
	return d
}

// resolveListBox returns the look of a list box: the defaults, merged with the
// theme's ListBox and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveListBox(ctx *Context, inst ListBoxStyle) ListBoxStyle {
	t := ctx.Theme()
	return layered(ctx, listBoxDefaults(t), ListBoxStyle.merge, ListBoxStyle.plain,
		themeLayer(t.ListBox), instanceLayer(inst))
}

// ListBoxElement is a list of items that the user selects from, with the keyboard
// model of a WAI-ARIA listbox (https://www.w3.org/WAI/ARIA/apg/patterns/listbox/).
// Its items are built only in view, as ListOf builds its rows, so it can hold many
// of them. Create one with ListBox.
//
// The list is one Tab stop. Tab reaches the selected item, or the first item when
// none is selected, and the focus then moves with the keys:
//
//   - Up and Down move the focus to the previous and next item, without wrapping.
//   - Home and End move it to the first and last item, and PageUp and PageDown by
//     a page of ten items.
//   - In a single selection the selection follows the focus. In a multiple one a
//     plain move selects only the item the focus reaches.
//   - Ctrl with an arrow or a page key moves the focus and keeps the selection.
//   - Shift with an arrow or a page key, in a multiple selection, selects the items
//     from the anchor to the focus. Shift+Home and Shift+End do the same.
//   - Space and Enter select the focused item in a single selection, and toggle it
//     in a multiple one, keeping the others. Enter also activates it (OnActivate).
//   - Ctrl+A selects every item in a multiple selection.
//   - Typing letters moves the focus to the next item whose name starts with them,
//     when ItemName is set. A pause of a second starts a new search, as in a select.
//
// A click selects the item alone. Ctrl+click toggles an item and Shift+click selects
// from the anchor to it, in a multiple selection. A double-click activates the item.
// A right-press on an item that is not selected selects it first, as File Explorer
// does, and then opens its context menu. Shift+F10 and the Menu key open the menu of
// the focused item.
//
// The selection is a signal of the keys of the chosen items. The focus, and the
// anchor of a Shift selection, are the list's own state.
//
// Assistive technology reads the list as a list named by its Label, each item as a
// list item with its position, and the selected items as selected. An item is named
// by its ItemName; without one, its own text is its name. The focused item is the
// one a screen reader follows while the list has focus.
type ListBoxElement[T any] struct {
	items      []T
	itemKey    func(T) string
	selected   keySelection
	row        func(item T, s PartState) Element
	mode       SelectionMode
	name       string // the accessible name of the list, see Label
	key        string
	itemName   func(T) string
	menu       func(item T) []MenuItem
	onChange   func(keys []string)
	onActivate func(item T)
	styles     ListBoxStyle
	style      Style
	size       sizing // see the sizing methods
}

// ListBox creates a list of items, whose keys are the values of selected. Each
// item is drawn by row, which gets the item and its PartState (Selected for a
// selected item, Focused for the item that has keyboard focus while the list has
// focus). The selection is single unless Selection says otherwise. A nil selected
// keeps the selection in the list, or in the signal SelectedKey gives.
//
//	ui.ListBox(repos, func(r Repo) string { return r.Path }, selected,
//		func(r Repo, s ui.PartState) ui.Element { return ui.Text(r.Name) }).
//		Label("Repositories").ItemName(func(r Repo) string { return r.Name })
func ListBox[T any](items []T, key func(T) string, selected *Signal[[]string],
	row func(item T, s PartState) Element) *ListBoxElement[T] {
	l := &ListBoxElement[T]{items: items, itemKey: key, row: row, mode: SelectionSingle}
	if selected != nil {
		l.selected = selected
	}
	return l
}

// keySelection is where a list box keeps its selection: a *Signal[[]string] of
// keys, or a single key (see SelectedKey).
type keySelection interface {
	Get() []string
	Peek() []string
	Set(keys []string)
}

// singleKey keeps a single selection in a string signal, "" for none.
type singleKey struct{ s *Signal[string] }

func (k singleKey) Get() []string  { return keyList(k.s.Get()) }
func (k singleKey) Peek() []string { return keyList(k.s.Peek()) }
func (k singleKey) Set(keys []string) {
	if len(keys) == 0 {
		k.s.Set("")
		return
	}
	k.s.Set(keys[0])
}

func keyList(k string) []string {
	if k == "" {
		return nil
	}
	return []string{k}
}

// SelectedKey keeps a single selection in key, the selected item's key or "" for
// none, as Tree and Sidebar keep theirs, in place of the keys signal ListBox takes
// (which may then be nil). It sets the selection to SelectionSingle.
//
//	ui.ListBox(repos, func(r Repo) string { return r.Path }, nil, row).SelectedKey(current)
func (l *ListBoxElement[T]) SelectedKey(key *Signal[string]) *ListBoxElement[T] {
	l.selected, l.mode = singleKey{key}, SelectionSingle
	return l
}

func (*ListBoxElement[T]) isElement()           {}
func (l *ListBoxElement[T]) elementKey() string { return l.key }

// Key gives the list box a stable identity among its siblings.
func (l *ListBoxElement[T]) Key(k string) *ListBoxElement[T] { l.key = k; return l }

// Label names the list for assistive technology, such as "Repositories".
func (l *ListBoxElement[T]) Label(name string) *ListBoxElement[T] { l.name = name; return l }

// Selection sets how many items the user can select at once: SelectionSingle (the
// default), SelectionMultiple, or SelectionNone for a list the user only moves
// through. See the keys in ListBoxElement.
func (l *ListBoxElement[T]) Selection(mode SelectionMode) *ListBoxElement[T] {
	l.mode = mode
	return l
}

// ItemName names each item for assistive technology and for type-ahead. The item's
// content is then hidden from assistive technology, so the name is what it reads.
func (l *ListBoxElement[T]) ItemName(name func(item T) string) *ListBoxElement[T] {
	l.itemName = name
	return l
}

// ContextMenu opens a menu on an item, built by build with the item: on a right-press
// on the item, and with Shift+F10 or the Menu key for the focused item. An empty
// result opens nothing.
func (l *ListBoxElement[T]) ContextMenu(build func(item T) []MenuItem) *ListBoxElement[T] {
	l.menu = build
	return l
}

// OnChange runs fn with the keys of the selected items after the user changes the
// selection, by a click, the keys or a typed letter. The selection signal is set
// first. It does not run for a change the app makes to the signal.
func (l *ListBoxElement[T]) OnChange(fn func(keys []string)) *ListBoxElement[T] {
	l.onChange = fn
	return l
}

// OnActivate runs fn with an item after the user activates it: with Enter, or by a
// double-click.
func (l *ListBoxElement[T]) OnActivate(fn func(item T)) *ListBoxElement[T] {
	l.onActivate = fn
	return l
}

// Styles sets the look of the parts of the list box, for this instance only. Set
// fields win over the theme's ListBox and the defaults; see ListBoxStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (l *ListBoxElement[T]) Styles(st ListBoxStyle) *ListBoxElement[T] {
	l.styles = l.styles.merge(st)
	return l
}

// Style is merged into the outer box of the list box: size it with Height, Grow or
// MaxHeight. The list fills that box.
func (l *ListBoxElement[T]) Style(s Style) *ListBoxElement[T] { l.style = l.style.Merge(s); return l }

// The sizing methods give the list box's outer box a size, over its Style. See Sizing
// in docs/styling.md.
func (l *ListBoxElement[T]) Width(v Length) *ListBoxElement[T] { l.size.setWidth(v); return l }

// Height sets the height of the list box.
func (l *ListBoxElement[T]) Height(v Length) *ListBoxElement[T] { l.size.setHeight(v); return l }

// MinWidth sets the smallest width of the list box.
func (l *ListBoxElement[T]) MinWidth(v Length) *ListBoxElement[T] { l.size.setMinWidth(v); return l }

// MinHeight sets the smallest height of the list box.
func (l *ListBoxElement[T]) MinHeight(v Length) *ListBoxElement[T] { l.size.setMinHeight(v); return l }

// MaxWidth sets the largest width of the list box.
func (l *ListBoxElement[T]) MaxWidth(v Length) *ListBoxElement[T] { l.size.setMaxWidth(v); return l }

// MaxHeight sets the largest height of the list box.
func (l *ListBoxElement[T]) MaxHeight(v Length) *ListBoxElement[T] { l.size.setMaxHeight(v); return l }

// Grow makes the list box take a share of the free space on its parent's main axis.
func (l *ListBoxElement[T]) Grow(f float32) *ListBoxElement[T] { l.size.setGrow(f); return l }

// GridSpan sets how many columns and rows the list box covers in its parent's grid.
func (l *ListBoxElement[T]) GridSpan(cols, rows int) *ListBoxElement[T] {
	l.size.setSpan(cols, rows)
	return l
}

// toggleKey returns keys with k removed when it is there, and added when it is not.
func toggleKey(keys []string, k string) []string {
	if slices.Contains(keys, k) {
		return slices.DeleteFunc(slices.Clone(keys), func(x string) bool { return x == k })
	}
	return append(slices.Clone(keys), k)
}

func (l *ListBoxElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		rt := ctx.rt
		st := resolveListBox(ctx, l.styles)
		items := l.items
		n := len(items)
		if l.selected == nil {
			// Neither a keys signal nor SelectedKey: the list keeps its selection itself.
			l.selected = Local(ctx, "selected", []string(nil))
		}
		focus := Local(ctx, "focus", "")
		focused := Local(ctx, "focused", false)
		anchor := Local(ctx, "anchor", "")
		typed := Local(ctx, "typed", selectType{})
		pressed := Local(ctx, "pressed", new(bool)).Peek()
		handle := Local(ctx, "handle", NewListHandle()).Peek()

		keyOf := func(i int) string { return l.itemKey(items[i]) }
		// find returns the index of the item with key k, or -1.
		find := func(k string) int {
			for i := range items {
				if keyOf(i) == k {
					return i
				}
			}
			return -1
		}
		// stop is the item that Tab reaches: the first selected item, else the first.
		stop := func() int {
			sel := l.selected.Peek()
			for i := range items {
				if slices.Contains(sel, keyOf(i)) {
					return i
				}
			}
			if n > 0 {
				return 0
			}
			return -1
		}
		// cur is the item the keys act on: the focused item, else the item Tab reaches.
		cur := func() int {
			if i := find(focus.Peek()); i >= 0 {
				return i
			}
			return stop()
		}
		// anchorAt returns the index of the anchor of a Shift selection, or i when
		// there is none.
		anchorAt := func(i int) int {
			if a := find(anchor.Peek()); a >= 0 {
				return a
			}
			return i
		}
		// set changes the selection to keys, and runs OnChange when that changes it.
		set := func(keys []string) {
			if l.mode == SelectionNone || slices.Equal(keys, l.selected.Peek()) {
				return
			}
			l.selected.Set(keys)
			if l.onChange != nil {
				l.onChange(keys)
			}
		}
		// span returns the keys of the items from index a to index b, in either order.
		span := func(a, b int) []string {
			a, b = min(a, b), max(a, b)
			out := make([]string, 0, b-a+1)
			for i := a; i <= b; i++ {
				out = append(out, keyOf(i))
			}
			return out
		}
		// pick handles a press on item i with the modifier keys mods, as a click does.
		// Shift selects from the anchor to i and Ctrl toggles i, in a multiple selection;
		// otherwise i alone is selected. The item gets the focus either way.
		pick := func(i int, mods input.Modifiers) {
			k := keyOf(i)
			focus.Set(k)
			switch {
			case l.mode == SelectionMultiple && mods&input.ModShift != 0:
				set(span(anchorAt(i), i))
			case l.mode == SelectionMultiple && mods&input.ModCtrl != 0:
				anchor.Set(k)
				set(toggleKey(l.selected.Peek(), k))
			default:
				anchor.Set(k)
				set([]string{k})
			}
		}
		// moveTo moves the focus to item i and scrolls it into view, for the keys. Ctrl
		// moves the focus alone; Shift in a multiple selection selects from the anchor to
		// i; otherwise i alone is selected, as the selection follows the focus.
		moveTo := func(i int, mods input.Modifiers) {
			k := keyOf(i)
			focus.Set(k)
			handle.ScrollToItem(i)
			switch {
			case mods&input.ModCtrl != 0:
			case l.mode == SelectionMultiple && mods&input.ModShift != 0:
				set(span(anchorAt(i), i))
			default:
				anchor.Set(k)
				set([]string{k})
			}
		}
		// press chooses item i by the keys: Space and Enter select it in a single
		// selection, and toggle it in a multiple one.
		press := func(i int) {
			k := keyOf(i)
			anchor.Set(k)
			if l.mode == SelectionMultiple {
				set(toggleKey(l.selected.Peek(), k))
				return
			}
			set([]string{k})
		}

		keys := func(e *KeyEvent) {
			if e.Mods&^(input.ModCtrl|input.ModShift) != 0 || n == 0 {
				return
			}
			i := cur()
			j := -1
			switch e.Key {
			case input.KeyUp:
				j = max(i-1, 0)
			case input.KeyDown:
				j = min(i+1, n-1)
			case input.KeyHome:
				j = 0
			case input.KeyEnd:
				j = n - 1
			case input.KeyPageUp:
				j = max(i-listBoxPage, 0)
			case input.KeyPageDown:
				j = min(i+listBoxPage, n-1)
			case input.KeySpace, input.KeyEnter:
				if e.Repeat {
					return
				}
				press(i)
				if e.Key == input.KeyEnter && l.onActivate != nil {
					l.onActivate(items[i])
				}
				e.StopPropagation()
				return
			case input.KeyA:
				if e.Mods != input.ModCtrl || l.mode != SelectionMultiple {
					return
				}
				all := make([]string, n)
				for x := range items {
					all[x] = keyOf(x)
				}
				set(all)
				e.StopPropagation()
				return
			default:
				return
			}
			e.StopPropagation()
			moveTo(j, e.Mods)
		}
		// typeahead moves the focus to the next item whose name starts with the letters
		// typed in quick succession. A single letter moves past the focused item, so a
		// repeated letter cycles through the items that start with it.
		typeahead := func(text string) {
			r, _ := utf8.DecodeRuneInString(text)
			if l.itemName == nil || n == 0 || (!unicode.IsLetter(r) && !unicode.IsDigit(r)) {
				return
			}
			now := rt.now()
			prev := typed.Peek()
			s := text
			if now.Sub(prev.at) < selectTypeahead {
				s = prev.text + text
			}
			typed.Set(selectType{text: s, at: now})
			i := cur()
			start := max(i, 0)
			if utf8.RuneCountInString(text) == 1 {
				start = i + 1
			}
			opts := make([]Option[string], n)
			for x := range items {
				opts[x] = Option[string]{Value: keyOf(x), Label: l.itemName(items[x])}
			}
			if j := matchLabel(opts, start, s); j >= 0 {
				moveTo(j, 0)
			}
		}

		// drawRow returns item i, which is the item with key k.
		drawRow := func(i int, it T) Element {
			k := l.itemKey(it)
			r := Div().asPart("ListBox.Item").Key(k).Style(st.Item).Role(access.RoleListItem).Cursor(CursorPointer)
			if slices.Contains(l.selected.Get(), k) {
				r.Style(st.ItemSelected)
			} else {
				r.Hover(st.ItemHover)
			}
			isFocus := focus.Get() == k
			if isFocus && focused.Get() {
				r.Style(st.ItemFocusVisible)
			}
			if l.mode != SelectionNone {
				r.AccessSelected(slices.Contains(l.selected.Get(), k))
			}
			r.access().highlighted = isFocus
			if l.itemName != nil {
				r.Label(l.itemName(it))
			}
			r.OnMouseDown(func(e *MouseEvent) {
				if e.Button == input.ButtonLeft {
					*pressed = true
					pick(i, e.Mods)
				}
			})
			r.OnClick(func() {
				// A press has already picked the item; a click from assistive technology picks it here.
				if *pressed {
					*pressed = false
					return
				}
				pick(i, 0)
			})
			r.OnDoubleClick(func() {
				if l.onActivate != nil {
					l.onActivate(it)
				}
			})
			if l.menu != nil {
				r.ContextMenu(func() []MenuItem {
					// A right-press on an item that is not selected selects it first, as File
					// Explorer does, and a selected item keeps the selection.
					if !slices.Contains(l.selected.Peek(), k) {
						pick(i, 0)
					}
					return l.menu(it)
				})
			}
			content := Component(func(ctx *Context) Element {
				return l.row(it, PartState{Hovered: ctx.hovered(), Pressed: ctx.pressed(),
					Focused:  focus.Get() == k && focused.Get(),
					Selected: slices.Contains(l.selected.Get(), k), HighContrast: ctx.Theme().HighContrast})
			})
			if l.itemName != nil {
				r.Child(Div().AccessHidden().Child(content))
			} else {
				r.Child(content)
			}
			return r
		}

		list := ListOf(items, l.itemKey, drawRow).Handle(handle).Style(st.List)
		if l.key != "" {
			list.Key(l.key)
		}
		if l.mode != SelectionNone {
			list.SelectionContainer()
		}
		list.decorate = func(c *DivElement) {
			c.asPart("ListBox.List").OnKeyDown(keys).OnTextInput(typeahead).
				OnFocus(func() {
					focused.Set(true)
					if i := cur(); i >= 0 {
						focus.Set(keyOf(i))
					}
				}).
				OnBlur(func() { focused.Set(false) })
			if l.name != "" {
				c.Label(l.name)
			}
			if l.menu != nil {
				// The keyboard opens the menu of the focused item. A press with the pointer
				// is for the item under it, which has its own menu, so nothing opens here.
				c.ContextMenu(func() []MenuItem {
					if !rt.keyboardModality {
						return nil
					}
					if i := cur(); i >= 0 {
						return l.menu(items[i])
					}
					return nil
				})
			}
		}
		// The list sits in a column that takes the list box's style and size, so that the
		// list fills the space the style gives it and windows its rows to that space (see Tree).
		return l.size.apply(Div().Column().MinHeight(Px(0)).Style(l.style)).Child(list)
	})
}

package ui

import (
	"fmt"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// sidebarWidth is the width of an expanded sidebar at the Text size of 100
// percent, and sidebarCollapsedWidth the width of a collapsed one. The expanded
// width scales with the Text size (see textScale); the collapsed one holds icons,
// which do not scale.
const (
	sidebarWidth          = 240
	sidebarCollapsedWidth = 48
)

// SidebarItem is one entry of a Sidebar: a page or a command, with its key, its
// label, an optional icon and an optional count. Selecting an item sets the
// selected signal to its Key. A disabled item is shown but cannot be selected.
// A heading is made with SidebarHeading instead.
type SidebarItem struct {
	// Key identifies the item among the items of the sidebar.
	Key string
	// Label is the text of the item.
	Label string
	// Icon is an icon shown before the label.
	Icon  *icons.Icon
	Badge int // a count shown at the end of the item when it is above zero
	// Disabled makes the item unable to be chosen.
	Disabled bool
	heading  bool // set by SidebarHeading
}

// SidebarHeading returns a heading of a Sidebar: a line of text that names the
// group of items below it. A heading is not selectable, and in a collapsed
// sidebar it is drawn as a thin rule.
func SidebarHeading(label string) SidebarItem { return SidebarItem{Label: label, heading: true} }

// SidebarStyle is the look of each part of a sidebar. A part's style merges over
// the built-in defaults, then over the theme's Sidebar, then over the Styles
// of the instance. The states are parts of their own: ItemHover applies to an
// enabled item the pointer is over, if it is not selected, ItemSelected to the
// selected item, ItemDisabled to a disabled one, and ItemFocusVisible to an item
// that has keyboard focus. Indicator is the bar at the left of the selected item,
// so the selection is not carried by color alone. See docs/styling.md.
type SidebarStyle struct {
	Pane             Style // the column that holds the sidebar
	Item             Style // each item
	ItemHover        Style // an item the pointer is over, if it is enabled and not selected
	ItemSelected     Style // the selected item
	ItemDisabled     Style // a disabled item
	ItemFocusVisible Style // an item that has keyboard focus
	Indicator        Style // the bar at the left of the selected item
	Heading          Style // each heading of an expanded sidebar
	Toggle           Style // the button that collapses and expands the sidebar
}

// merge returns s with the set fields of o applied on top, part by part.
func (s SidebarStyle) merge(o SidebarStyle) SidebarStyle {
	return SidebarStyle{
		Pane:             s.Pane.Merge(o.Pane),
		Item:             s.Item.Merge(o.Item),
		ItemHover:        s.ItemHover.Merge(o.ItemHover),
		ItemSelected:     s.ItemSelected.Merge(o.ItemSelected),
		ItemDisabled:     s.ItemDisabled.Merge(o.ItemDisabled),
		ItemFocusVisible: s.ItemFocusVisible.Merge(o.ItemFocusVisible),
		Indicator:        s.Indicator.Merge(o.Indicator),
		Heading:          s.Heading.Merge(o.Heading),
		Toggle:           s.Toggle.Merge(o.Toggle),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s SidebarStyle) plain() SidebarStyle {
	return SidebarStyle{
		Pane:             plainStyle(s.Pane),
		Item:             plainStyle(s.Item),
		ItemHover:        plainStyle(s.ItemHover),
		ItemSelected:     plainStyle(s.ItemSelected),
		ItemDisabled:     plainStyle(s.ItemDisabled),
		ItemFocusVisible: plainStyle(s.ItemFocusVisible),
		Indicator:        plainStyle(s.Indicator),
		Heading:          plainStyle(s.Heading),
		Toggle:           plainStyle(s.Toggle),
	}
}

// sidebarDefaults returns the built-in look of a sidebar, from the colors of t.
// Under high contrast the selected item is the Accent color with AccentText on
// it, and the borders are in Text.
func sidebarDefaults(t *Theme) SidebarStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := SidebarStyle{
		Pane: Style{Gap: 2, Padding: All(6), Background: t.Panel, BorderEdges: Edges{Right: 1}, BorderColor: t.Border},
		Item: Style{Direction: Row, Align: Center, Gap: 10, MinHeight: Px(36), NoShrink: true,
			Radius: t.Radius / 2, Padding: XY(10, 0), TextColor: t.Text},
		ItemHover:        Style{Background: t.SurfaceHover},
		ItemSelected:     Style{Background: t.AccentSoft},
		ItemFocusVisible: Style{Shadows: []Shadow{ring}},
		Indicator:        Style{Width: Px(3), Background: t.Accent, Radius: 2},
		Heading: Style{FontSize: 12, FontWeight: FontSemibold, TextColor: t.TextMuted,
			Padding: Edges{Top: 12, Right: 10, Bottom: 4, Left: 10}},
		Toggle: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(36), Height: Px(36),
			NoShrink: true, Radius: t.Radius / 2, TextColor: t.Text},
	}
	if t.HighContrast {
		// Under high contrast a disabled item is GrayText, not dimmed, the selected
		// item is the system highlight, and the borders are the text color.
		d.Pane.BorderColor = t.Text
		d.ItemDisabled = Style{TextColor: t.Disabled}
		d.ItemSelected = Style{Background: t.Accent, TextColor: t.AccentText}
		d.Indicator.Background = t.AccentText
	} else {
		d.ItemDisabled = Style{Opacity: 0.5}
	}
	return d
}

// resolveSidebar returns the look of a sidebar: the defaults, merged with the
// theme's Sidebar and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveSidebar(ctx *Context, inst SidebarStyle) SidebarStyle {
	t := ctx.Theme()
	return layered(ctx, sidebarDefaults(t), SidebarStyle.merge, SidebarStyle.plain,
		themeLayer(t.Sidebar), instanceLayer(inst))
}

// SidebarElement is a vertical navigation pane, as the left pane of WinUI's
// NavigationView. The item whose key is the selected signal's value is selected.
//
// The pane is one Tab stop, at the selected item, or at the first enabled item
// when the selected one is not among them. Up and Down move the keyboard focus to
// the previous and next enabled item, without wrapping, and skip the headings and
// the disabled items. Home and End go to the first and last enabled item. Focus
// follows the keys, but the selection does not change until the user presses
// Enter or Space on the focused item, or clicks it, as NavigationView does. A
// click selects an item and gives it the focus.
//
// Assistive technology reads the pane as a group named by its Label (by default
// the Navigation text of the window's Strings), the items as the items of a list,
// and the headings as headings.
// The selected item reports the selected state and a disabled one the disabled
// state. An item with a badge is named with its count, as in "Inbox, 4".
//
// The pane takes the height of its row, so put it in a Row beside the content.
// Its items scroll when they do not fit, and a Header and a Footer stay in place.
type SidebarElement struct {
	selected  *Signal[string]
	items     []SidebarItem
	name      string // the accessible name of the pane, see Label
	key       string
	onChange  func(key string)
	styles    SidebarStyle
	collapsed *Signal[bool] // see Collapsed
	toggle    bool          // see Toggle
	header    Element       // see Header
	footer    Element       // see Footer
	size      sizing        // the expanded width, from the sizing methods (see Sizing in docs/styling.md)
}

// Sidebar creates a navigation pane of items, with the item whose key equals
// selected's value selected. The items are shown in order.
func Sidebar(selected *Signal[string], items ...SidebarItem) *SidebarElement {
	return &SidebarElement{selected: selected, items: items}
}

func (*SidebarElement) isElement()           {}
func (s *SidebarElement) elementKey() string { return s.key }

// Key gives the sidebar a stable identity. It also scopes the keys of its items.
// Without a key the signal's identity scopes them.
func (s *SidebarElement) Key(k string) *SidebarElement { s.key = k; return s }

// Label names the pane for assistive technology, such as "Main navigation". The
// default is the Navigation text of the window's Strings.
func (s *SidebarElement) Label(name string) *SidebarElement { s.name = name; return s }

// OnChange runs fn with the key of the item after the user selects it, by a click,
// Enter or Space. The signal is set first.
func (s *SidebarElement) OnChange(fn func(key string)) *SidebarElement { s.onChange = fn; return s }

// Styles sets the look of the parts of the sidebar, for this instance only. Set
// fields win over the theme's Sidebar and the defaults; see SidebarStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (s *SidebarElement) Styles(st SidebarStyle) *SidebarElement {
	s.styles = s.styles.merge(st)
	return s
}

// Collapsed makes the sidebar collapsible by c: when c is true the sidebar is
// about 48 px wide and shows only the icons of its items. Each item then shows its
// label as a tooltip, and each heading is a thin rule. An item with no icon shows
// nothing in a collapsed sidebar. The header and the footer are drawn as they are.
func (s *SidebarElement) Collapsed(c *Signal[bool]) *SidebarElement { s.collapsed = c; return s }

// Toggle adds a menu button at the top of the sidebar that flips the signal given
// to Collapsed. It is named with the CollapseNavigation text of the window's Strings
// while the sidebar is expanded, and with the ExpandNavigation text while it is
// collapsed, and it reports the expanded state. It
// has no effect without Collapsed.
func (s *SidebarElement) Toggle() *SidebarElement { s.toggle = true; return s }

// Header puts el above the items. It does not scroll.
func (s *SidebarElement) Header(el Element) *SidebarElement { s.header = el; return s }

// Footer puts el below the items, pinned at the bottom of the sidebar, such as a
// Settings item or an account Avatar.
func (s *SidebarElement) Footer(el Element) *SidebarElement { s.footer = el; return s }

// The sizing methods give the expanded sidebar a width, which the Text size
// scales by default. A collapsed sidebar is always about 48 px wide, whatever they
// are set to. See Sizing in docs/styling.md.
func (s *SidebarElement) Width(l Length) *SidebarElement { s.size.setWidth(l); return s }

// MinWidth sets the smallest width of the sidebar.
func (s *SidebarElement) MinWidth(l Length) *SidebarElement { s.size.setMinWidth(l); return s }

// MaxWidth sets the largest width of the sidebar.
func (s *SidebarElement) MaxWidth(l Length) *SidebarElement { s.size.setMaxWidth(l); return s }

// GridSpan sets how many columns and rows the sidebar covers in its parent's grid.
func (s *SidebarElement) GridSpan(cols, rows int) *SidebarElement {
	s.size.setSpan(cols, rows)
	return s
}

// scope is the prefix of the keys of this sidebar.
func (s *SidebarElement) scope() string {
	if s.key != "" {
		return s.key
	}
	return fmt.Sprintf("sidebar:%p", s.selected)
}

func (s *SidebarElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		st := resolveSidebar(ctx, s.styles)
		cur := s.selected.Get()
		collapsed := s.collapsed != nil && s.collapsed.Get()
		strs := ctx.Strings()
		name := s.name
		if name == "" {
			name = strs.Navigation
		}
		scope := s.scope()
		// opts are the items as options. A heading is an option that cannot be
		// chosen, so the steps and the edges skip it as they skip a disabled item.
		opts := make([]Option[string], len(s.items))
		size := 0
		for i, it := range s.items {
			opts[i] = Option[string]{Value: it.Key, Label: it.Label, Disabled: it.Disabled || it.heading}
			if !it.heading {
				size++
			}
		}
		sel := indexOf(opts, cur)
		stop := tabStop(opts, cur, false) // the one item Tab reaches
		// handles holds the focus handle of each enabled item, by key. A handle lets
		// the keys move the focus to an item without selecting it.
		handles := Local(ctx, "handles", map[string]*FocusHandle{}).Peek()
		for i, it := range s.items {
			if !opts[i].Disabled && handles[it.Key] == nil {
				handles[it.Key] = NewFocusHandle()
			}
		}
		// choose selects the item at index i and gives it the focus, as a click does.
		choose := func(i int) {
			if i < 0 || opts[i].Disabled || i == sel {
				return
			}
			s.selected.Set(s.items[i].Key)
			if s.onChange != nil {
				s.onChange(s.items[i].Key)
			}
			handles[s.items[i].Key].Focus()
		}
		// focusItem gives the keyboard focus to the item at index i, without selecting it.
		focusItem := func(i int) {
			if i >= 0 && !opts[i].Disabled {
				handles[s.items[i].Key].Focus()
			}
		}
		// drawItem returns the item at index i, the pos-th of size items.
		drawItem := func(i, pos int) *DivElement {
			it := s.items[i]
			on := i == sel
			row := Div().asPart("Sidebar.Item").Key(scope+"/item:"+it.Key).Style(st.Item).
				listItem(pos, size).Label(it.Label).AccessSelected(on)
			if on {
				row.Style(st.ItemSelected)
				// The bar is the selection's mark that is not a color, in the expanded and
				// the collapsed sidebar alike.
				row.Child(Div().asPart("Sidebar.Indicator").Absolute().Left(0).Top(8).Bottom(8).Style(st.Indicator))
			}
			if collapsed {
				row.Justify(Center).Tooltip(it.Label)
			}
			if it.Icon != nil {
				row.Child(Icon(it.Icon).Size(16))
			}
			if !collapsed {
				row.Child(Text(it.Label).NoWrap()).Child(Spacer())
				if it.Badge > 0 {
					row.Badged(Badge(it.Badge))
				}
			} else if it.Badge > 0 {
				// The count is not drawn when collapsed, but the name keeps it.
				row.access().badge = Badge(it.Badge)
			}
			if it.Disabled {
				row.access().disabled = true
				row.Style(st.ItemDisabled).Cursor(CursorNotAllowed)
				return row
			}
			if !on {
				row.Hover(st.ItemHover)
			}
			row.Cursor(CursorPointer).Focusable().FocusHandle(handles[it.Key]).FocusVisible(st.ItemFocusVisible)
			if i != stop {
				row.TabStop(false) // reached with the arrow keys, not Tab
			}
			row.OnClick(func() { choose(i) }).
				OnKeyDown(func(e *KeyEvent) {
					if e.Mods != 0 || e.Repeat {
						return
					}
					j := -1
					switch e.Key {
					case input.KeyUp:
						j = listStep(opts, i, -1)
					case input.KeyDown:
						j = listStep(opts, i, 1)
					case input.KeyHome:
						j = listEdge(opts, false)
					case input.KeyEnd:
						j = listEdge(opts, true)
					default:
						return
					}
					e.StopPropagation()
					focusItem(j)
				})
			return row
		}

		// The items are a list of list items, with the headings among them. The list
		// scrolls, so the header and the footer stay in place.
		list := Div().Column().Gap(2).Grow(1).MinHeight(Px(0)).ScrollY().Role(access.RoleList).SelectionContainer()
		pos := 0
		for i, it := range s.items {
			if it.heading {
				h := Div().asPart("Sidebar.Heading").Keyf("%s/heading:%d", scope, i).Heading(2).Label(it.Label)
				if collapsed {
					// A heading is a thin rule when the sidebar is collapsed, and its label is
					// not drawn.
					h.Style(Style{Height: Px(1), Margin: Edges{Top: 6, Bottom: 6}, Background: t.Border})
				} else {
					h.Style(st.Heading).Child(Text(it.Label).NoWrap())
				}
				list.Child(h)
				continue
			}
			pos++
			list.Child(drawItem(i, pos))
		}

		pane := Div().asPart("Sidebar.Pane").Key(scope).Column().Role(access.RoleGroup).Label(name).
			Style(Style{Width: Px(sidebarWidth * textScale(ctx))}).Style(st.Pane)
		if collapsed {
			w := Px(sidebarCollapsedWidth)
			pane.Style(Style{Width: w, MinWidth: w, MaxWidth: w})
		} else {
			s.size.apply(pane)
		}
		if s.toggle && s.collapsed != nil {
			label := strs.CollapseNavigation
			if collapsed {
				label = strs.ExpandNavigation
			}
			c := s.collapsed
			pane.Child(Div().asPart("Sidebar.Toggle").Key(scope + "/toggle").Style(st.Toggle).
				Role(access.RoleButton).Label(label).AccessExpanded(!collapsed).
				Cursor(CursorPointer).Focusable().FocusVisible(Style{Shadows: []Shadow{{Spread: 2, Color: t.FocusRing}}}).
				OnClick(func() { c.Set(!c.Peek()) }).
				Child(Icon(icons.Menu).Size(16)))
		}
		return pane.Child(s.header).Child(list).Child(s.footer)
	})
}

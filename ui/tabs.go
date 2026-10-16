package ui

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// Tab is one tab of a Tabs: the value it selects, its label, an optional icon
// and the panel it shows. Panel is called only while the tab is selected, so the
// panels of the other tabs are not built, unless the Tabs keeps its panels (see
// KeepMounted). A disabled tab is shown but cannot be selected.
type Tab[T comparable] struct {
	// Value identifies the tab; the selected signal holds it.
	Value T
	// Label is the text of the tab.
	Label string
	// Icon is an icon shown before the label.
	Icon *icons.Icon
	// Disabled makes the tab unable to be chosen.
	Disabled bool
	// Panel builds the content of the tab, which is shown while the tab is selected.
	Panel func() Element
	// Popup builds what a floating panel under the tab shows, such as a menu of
	// actions or a small form. It opens when the selected tab is clicked again,
	// and with Down, Shift+F10 or the Menu key while the tab has focus. Escape, a
	// click outside or a call to close closes it. Its focus behaves as a Popover's:
	// it moves into the panel when the panel opens, and back to the tab when
	// Escape or close is used. The panel is styled as a Popover. Assistive
	// technology hears the tab as expanded or collapsed.
	Popup func(close func()) Element
}

// TabsVariant selects the look of a tab list.
type TabsVariant uint8

const (
	// TabsUnderline draws a line under the selected tab. It is the default.
	TabsUnderline TabsVariant = iota
	// TabsSegmented draws the tabs as one row of segments, the selected one
	// filled with Accent. It is the look of a segmented control.
	TabsSegmented
)

// TabsStyle is the look of each part of a tab list. A part's style merges over
// the built-in defaults of its variant, then over the theme's Tabs, then over
// the Styles of the instance. The states are parts of their own: TabSelected
// applies to the selected tab, TabDisabled to a disabled one, TabHover while
// the pointer is over an enabled tab that is not selected, and TabFocusVisible
// while the tab has keyboard focus. See docs/styling.md.
type TabsStyle struct {
	List            Style // the strip that holds the tabs
	Tab             Style // each tab
	TabHover        Style // a tab the pointer is over, if it is enabled and not selected
	TabSelected     Style // the selected tab
	TabDisabled     Style // a disabled tab
	TabFocusVisible Style // a tab that has keyboard focus
	Indicator       Style // the line under a tab in TabsUnderline; only the selected one shows it
	Panel           Style // the panel of the selected tab
}

// merge returns s with the set fields of o applied on top, part by part.
func (s TabsStyle) merge(o TabsStyle) TabsStyle {
	return TabsStyle{
		List:            s.List.Merge(o.List),
		Tab:             s.Tab.Merge(o.Tab),
		TabHover:        s.TabHover.Merge(o.TabHover),
		TabSelected:     s.TabSelected.Merge(o.TabSelected),
		TabDisabled:     s.TabDisabled.Merge(o.TabDisabled),
		TabFocusVisible: s.TabFocusVisible.Merge(o.TabFocusVisible),
		Indicator:       s.Indicator.Merge(o.Indicator),
		Panel:           s.Panel.Merge(o.Panel),
	}
}

// plain returns s as high contrast allows it (see plainStyle). The indicator's height
// is the system's too, so a size cannot hide the selection either.
func (s TabsStyle) plain() TabsStyle {
	ind := plainStyle(s.Indicator)
	ind.Height = Auto
	return TabsStyle{
		List:            plainStyle(s.List),
		Tab:             plainStyle(s.Tab),
		TabHover:        plainStyle(s.TabHover),
		TabSelected:     plainStyle(s.TabSelected),
		TabDisabled:     plainStyle(s.TabDisabled),
		TabFocusVisible: plainStyle(s.TabFocusVisible),
		Indicator:       ind,
		Panel:           plainStyle(s.Panel),
	}
}

// tabsDefaults returns the built-in look of a tab list of variant v, from the
// colors of th.
func tabsDefaults(th *Theme, v TabsVariant) TabsStyle {
	ring := Shadow{Spread: 2, Color: th.FocusRing}
	d := TabsStyle{
		List: Style{Direction: Row, Gap: 4, BorderEdges: Edges{Bottom: 1}, BorderColor: th.Border},
		Tab: Style{Direction: Row, Align: Center, Gap: 6, MinHeight: Px(36), Padding: XY(12, 0),
			NoShrink: true, Radius: th.Radius / 2, TextColor: th.TextMuted, Transition: 110 * time.Millisecond},
		TabHover:        Style{Background: th.SurfaceHover},
		TabSelected:     Style{TextColor: th.Text},
		TabFocusVisible: Style{Shadows: []Shadow{ring}},
		Indicator:       Style{Height: Px(2), Background: th.Accent, Transition: 120 * time.Millisecond},
	}
	if th.HighContrast {
		// Under high contrast a disabled tab is GrayText, not dimmed.
		d.TabDisabled = Style{TextColor: th.Disabled}
	} else {
		d.TabDisabled = Style{Opacity: 0.5}
	}
	if v == TabsSegmented {
		d.List = Style{Direction: Row, Gap: 2, Padding: All(3), Radius: th.Radius,
			Background: th.Panel, BorderWidth: 1, BorderColor: th.Border, fitWidth: true}
		d.Tab.MinHeight = Px(30)
		d.Tab.Radius = max(th.Radius-3, 0)
		d.TabSelected = Style{Background: th.Accent, TextColor: th.AccentText}
		d.Indicator = Style{}
	}
	return d
}

// resolveTabs returns the look of a tab list: the defaults of v, merged with
// the theme's Tabs and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveTabs(ctx *Context, v TabsVariant, inst TabsStyle) TabsStyle {
	th := ctx.Theme()
	return layered(ctx, tabsDefaults(th, v), TabsStyle.merge, TabsStyle.plain,
		themeLayer(th.Tabs), instanceLayer(inst))
}

// TabsElement is a tab list and the panel of the selected tab. The selected tab
// is the one whose value equals the signal's.
//
// The tab list is one Tab stop, at the selected tab, as Microsoft's tab guidance
// describes for TabView: Tab enters the strip at the selected tab, and Tab again
// leaves it for the panel. Left and Right move to the previous and next enabled
// tab and select it at once (automatic activation), without wrapping; Home and
// End go to the first and last. Space and Enter select the focused tab. Ctrl+Tab
// and Ctrl+Shift+Tab select the next and previous enabled tab, and wrap around.
// They work from the tab list and from the panel, as in TabView, and focus moves
// to the tab that was selected. Ctrl+Tab is not otherwise bound in the framework,
// so it moves focus as Tab does everywhere else.
//
// The tabs take the space their parent gives them, as a column does, so the panel
// can fill a window: a list in a panel grows to the height left under the tabs.
type TabsElement[T comparable] struct {
	selected    *Signal[T]
	tabs        []Tab[T]
	name        string // the accessible name of the tab list, see Label
	key         string
	onChange    func(T)
	onReselect  func(T)
	keep        bool // see KeepMounted
	variant     TabsVariant
	styles      TabsStyle
	renderTab   func(Tab[T], PartState) Element
	size        sizing       // the tab list's sizes, from the sizing methods (see Sizing in docs/styling.md)
	focusHandle *FocusHandle // see FocusHandle
}

// FocusHandle lets code focus the selected tab (the tab Tab reaches) (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (t *TabsElement[T]) FocusHandle(h *FocusHandle) *TabsElement[T] {
	t.focusHandle = h
	return t
}

// Tabs creates a tab list over tabs, showing the panel of the tab that selected
// reads and writes. The tabs are shown in order.
func Tabs[T comparable](selected *Signal[T], tabs ...Tab[T]) *TabsElement[T] {
	return &TabsElement[T]{selected: selected, tabs: tabs}
}

func (*TabsElement[T]) isElement()           {}
func (t *TabsElement[T]) elementKey() string { return t.key }

// Key gives the tabs a stable identity. It also scopes the keys that name each
// panel by its tab, so give two tab lists that show the same values different
// keys. Without a key the signal's identity scopes them.
func (t *TabsElement[T]) Key(k string) *TabsElement[T] { t.key = k; return t }

// Label names the tab list for assistive technology, such as "Settings sections".
func (t *TabsElement[T]) Label(name string) *TabsElement[T] { t.name = name; return t }

// OnChange runs fn with the new value after the user selects a tab, by a click,
// by the keys or by Ctrl+Tab. The signal is set first.
func (t *TabsElement[T]) OnChange(fn func(T)) *TabsElement[T] { t.onChange = fn; return t }

// OnReselect runs fn with the selected tab's value when the user activates
// that tab again, by a click, Enter or Space. A tab with a Popup also opens or
// closes it then.
func (t *TabsElement[T]) OnReselect(fn func(T)) *TabsElement[T] { t.onReselect = fn; return t }

// KeepMounted keeps each panel once it has been built. A panel is built the first
// time its tab is selected, as it is by default, and then stays in the tree while
// other tabs are selected, hidden with Display. Its component instances, Local
// state, text, selection, scroll positions and lists survive switching tabs.
// Tabs that were never selected are not built. Without KeepMounted only the
// selected panel exists, and the state of the others is discarded.
//
// A kept panel renders again when the code that makes the tabs runs again, and
// when its own state changes. Switching tabs does not render it.
func (t *TabsElement[T]) KeepMounted() *TabsElement[T] { t.keep = true; return t }

// Variant sets the look of the tab list: TabsUnderline (the default) or
// TabsSegmented. The behavior is the same in both.
func (t *TabsElement[T]) Variant(v TabsVariant) *TabsElement[T] { t.variant = v; return t }

// Styles sets the look of the parts of the tab list, for this instance only.
// Set fields win over the theme's Tabs and the defaults; see TabsStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
//
//	ui.Tabs(view, tabs...).Styles(ui.TabsStyle{
//		Indicator: ui.Style{Height: ui.Px(3)},
//	})
func (t *TabsElement[T]) Styles(s TabsStyle) *TabsElement[T] { t.styles = t.styles.merge(s); return t }

// The sizing methods give the tab list a width; the panel always fills the width of
// the Tabs. The segmented tab list is as wide as its tabs by default, and the
// underline tab list fills its container, as WinUI's TabView does. See Sizing in
// docs/styling.md.
func (t *TabsElement[T]) Width(l Length) *TabsElement[T] { t.size.setWidth(l); return t }

// MinWidth sets the smallest width of the tabs.
func (t *TabsElement[T]) MinWidth(l Length) *TabsElement[T] { t.size.setMinWidth(l); return t }

// MaxWidth sets the largest width of the tabs.
func (t *TabsElement[T]) MaxWidth(l Length) *TabsElement[T] { t.size.setMaxWidth(l); return t }

// RenderTab replaces the content of each tab (its icon and label) with what fn
// returns. The tab keeps its behavior, its focus, selection and state styles,
// and its accessible name is still its Label, so put in the label anything a
// user must hear, such as a count. The content is hidden from assistive
// technology. fn is called again when the tab's state changes, with the state
// in the PartState.
//
//	ui.Tabs(view, tabs...).RenderTab(func(tab ui.Tab[string], s ui.PartState) ui.Element {
//		return ui.Div().Row().Align(ui.Center).Gap(6).
//			Child(ui.Text(names[tab.Value])).
//			Child(ui.Badge(counts[tab.Value]))
//	})
func (t *TabsElement[T]) RenderTab(fn func(tab Tab[T], state PartState) Element) *TabsElement[T] {
	t.renderTab = fn
	return t
}

// scope is the prefix of the keys of this tab list.
func (t *TabsElement[T]) scope() string {
	if t.key != "" {
		return t.key
	}
	return fmt.Sprintf("tabs:%p", t.selected)
}

// tabKey is the key of the tab with value v. The tab and panel keys are
// disjoint: a tab's key has the "tab:" prefix and a panel's has "panel", so
// a value that formats as "panel" cannot collide with the default panel.
func (t *TabsElement[T]) tabKey(v T) string { return t.scope() + "/tab:" + fmt.Sprint(v) }

func (t *TabsElement[T]) expand(ctx *Context) Element {
	// panels holds the component of each kept panel, made by this render of the
	// tab list. Switching tabs reuses them, so the panels are not rendered again.
	panels := map[T]*ComponentElement{}
	return Component(func(ctx *Context) Element {
		th := ctx.Theme()
		st := resolveTabs(ctx, t.variant, t.styles)
		cur := t.selected.Get()
		opts := make([]Option[T], len(t.tabs))
		for i, tb := range t.tabs {
			opts[i] = Option[T]{Value: tb.Value, Label: tb.Label, Disabled: tb.Disabled}
		}
		sel := indexOf(opts, cur)
		stop := tabStop(opts, cur, false) // the one tab Tab reaches
		// focus takes keyboard focus to the selected tab, on the frame after a
		// change. The handle is attached to that tab alone.
		focus := Local(ctx, "focus", NewFocusHandle()).Peek()
		if t.focusHandle != nil {
			focus = t.focusHandle
		}
		// popupFor is the key of the tab whose Popup is open, or "". Only the
		// selected tab's can be open, so a change of tab, even by code, closes it.
		popupFor := Local(ctx, "popup", "")
		popupH := Local(ctx, "popupFocus", NewFocusHandle()).Peek()
		entering := Local(ctx, "popupEntering", false)
		selKey := ""
		if sel >= 0 && t.tabs[sel].Popup != nil {
			selKey = t.tabKey(cur)
		}
		if k := popupFor.Peek(); k != "" && k != selKey {
			popupFor.Set("") // the selection moved on, by code, while it was open
		}
		rt := ctx.rt
		isOpen := func() bool { return selKey != "" && popupFor.Peek() == selKey }
		// inPopup reports whether focus is in the open popup, whose keys belong to
		// its content and not to the tab list it hangs from.
		inPopup := func() bool { return isOpen() && popupH.id != 0 && rt.focusInside(popupH.id) }
		setPopup := func(on bool) {
			if isOpen() == on || selKey == "" {
				return
			}
			if !on && inPopup() {
				focus.FocusNow() // focus inside it returns to the tab
			}
			if on {
				popupFor.Set(selKey)
			} else {
				popupFor.Set("")
			}
			entering.Set(on)
		}
		// choose selects the tab at index i and moves focus to it. Choosing the
		// selected tab again is a reselect.
		choose := func(i int) {
			if i < 0 || opts[i].Disabled {
				return
			}
			if i == sel {
				if t.onReselect != nil {
					t.onReselect(opts[i].Value)
				}
				setPopup(!isOpen())
				return
			}
			setPopup(false)
			t.choose(opts[i].Value)
			focus.Focus()
		}
		// drawTab returns the tab at index i, in the state ps describes.
		drawTab := func(i int, ps PartState) *DivElement {
			tb := t.tabs[i]
			on := i == sel
			style := st.Tab
			if on {
				style = style.Merge(st.TabSelected)
			}
			if tb.Disabled {
				style = style.Merge(st.TabDisabled)
			}
			tab := Div().asPart("Tabs.Tab").Key(t.tabKey(tb.Value)).Style(style).
				Role(access.RoleTab).Label(tb.Label).AccessSelected(on)
			if t.renderTab != nil {
				tab.Child(Div().AccessHidden().Child(t.renderTab(tb, ps)))
			} else {
				if tb.Icon != nil {
					tab.Child(Icon(tb.Icon).Size(16))
				}
				tab.Child(Text(tb.Label).NoWrap())
			}
			if t.variant == TabsUnderline {
				// The line is drawn under the selected tab only. The others have a
				// transparent one, so that it eases in when the tab is selected.
				ind := Div().asPart("Tabs.Indicator").Absolute().Left(0).Right(0).Bottom(0).Style(st.Indicator)
				if !on {
					ind.Background(Transparent)
				} else if tb.Disabled && th.HighContrast {
					ind.Background(th.Disabled) // a disabled tab's line is GrayText under high contrast
				}
				tab.Child(ind)
			}
			if on && tb.Popup != nil {
				open := popupFor.Get() == selKey
				tab.AccessExpanded(open)
				// Expand and Collapse from assistive technology open and close it
				// without counting as a reselect.
				tab.access().setExpanded = setPopup
				if open {
					tab.Child(t.popupPanel(ctx, tb, popupH, entering, func() { setPopup(false) }))
				}
			}
			if tb.Disabled {
				tab.access().disabled = true
				tab.Cursor(CursorNotAllowed)
				return tab
			}
			if on {
				tab.FocusHandle(focus) // a handle makes the tab focusable, so a disabled one has none
			}
			if i != stop {
				tab.TabStop(false) // reached with the arrow keys, not Tab
			}
			if !on {
				tab.Hover(st.TabHover)
			}
			tab.Cursor(CursorPointer).Focusable().FocusVisible(st.TabFocusVisible).
				OnClick(func() { choose(i) }).
				OnKeyDown(func(e *KeyEvent) {
					if e.Repeat || inPopup() {
						return
					}
					if on && tb.Popup != nil && (e.Key == input.KeyF10 && e.Mods == input.ModShift ||
						e.Mods == 0 && (e.Key == input.KeyDown || e.Key == input.KeyMenu)) {
						e.StopPropagation()
						setPopup(true)
						return
					}
					if e.Mods != 0 {
						return
					}
					j := -1
					switch e.Key {
					case input.KeyLeft:
						j = listStep(opts, i, -1)
					case input.KeyRight:
						j = listStep(opts, i, 1)
					case input.KeyHome:
						j = listEdge(opts, false)
					case input.KeyEnd:
						j = listEdge(opts, true)
					default:
						return
					}
					e.StopPropagation()
					choose(j)
				})
			return tab
		}

		strip := t.size.apply(Div().asPart("Tabs.List").Role(access.RoleTabList).Style(st.List))
		if t.name != "" {
			strip.Label(t.name)
		}
		for i, tb := range t.tabs {
			if t.renderTab == nil {
				strip.Child(drawTab(i, PartState{Selected: i == sel, Disabled: tb.Disabled, HighContrast: th.HighContrast}))
				continue
			}
			// With a render slot the tab is its own component, so that it re-renders
			// when its pointer or focus state changes, and the slot gets the new state.
			i := i
			strip.Child(Component(func(ctx *Context) Element {
				return drawTab(i, PartState{Hovered: ctx.hovered(), Pressed: ctx.pressed(), Focused: ctx.isFocused(),
					Selected: i == sel, Disabled: t.tabs[i].Disabled, HighContrast: th.HighContrast})
			}).Key(t.tabKey(tb.Value)))
		}

		group := Div().Column().Gap(12).Grow(1).MinHeight(Px(0)).Key(t.scope()).
			// Ctrl+Tab and Ctrl+Shift+Tab reach here from the tab list and from the
			// panel, since both are inside the group.
			OnKeyDown(func(e *KeyEvent) {
				if e.Key != input.KeyTab || e.Repeat || (e.Mods != input.ModCtrl && e.Mods != input.ModCtrl|input.ModShift) || inPopup() {
					return
				}
				dir := 1
				if e.Mods.Has(input.ModShift) {
					dir = -1
				}
				e.StopPropagation()
				// nextEnabled wraps around, as TabView does for these keys.
				choose(nextEnabled(opts, sel, dir, false))
			}).
			Child(strip)
		if t.keep {
			group.Children(t.keptPanels(ctx, sel, panels, st.Panel)...)
		} else if sel >= 0 && t.tabs[sel].Panel != nil {
			if p := t.tabs[sel].Panel(); p != nil {
				// The panel is named by its tab: the label is its name, and LabelledBy
				// links it to the tab for the relation.
				group.Child(Div().asPart("Tabs.Panel").Key(t.scope() + "/panel").Column().Grow(1).MinHeight(Px(0)).
					Role(access.RoleTabPanel).Label(t.tabs[sel].Label).Style(st.Panel).
					LabelledBy(t.tabKey(t.tabs[sel].Value)).Child(p))
			}
		}
		return group
	})
}

// popupPanel returns the floating panel of tb's Popup, which hangs under the
// tab. Like a Popover's panel, it takes focus when it opens and hands it to its
// first field or button, and it closes on a click outside it and on Escape.
func (t *TabsElement[T]) popupPanel(ctx *Context, tb Tab[T], h *FocusHandle, entering *Signal[bool], closeFn func()) Element {
	rt := ctx.rt
	st := resolvePopover(ctx, PopoverStyle{})
	return Div().asPart("Popover.Panel").Overlay(BelowStart).OverlayGap(6).Style(st.Panel).
		Role(access.RoleDialog).Label(tb.Label).
		Focusable().TabStop(false).AutoFocus().FocusHandle(h).
		OnFocus(func() {
			if entering.Peek() {
				entering.Set(false)
				rt.focusFirstIn(h.id)
			}
		}).
		OnClickOutside(closeFn).
		Enter(Fade().Slide(0, -4), Animation{Duration: 120 * time.Millisecond}).
		Exit(Fade(), Animation{Duration: 80 * time.Millisecond}).
		Child(tb.Popup(closeFn))
}

// keptPanels returns the panel of each tab that has been selected, with the
// selected one shown and the others hidden (see KeepMounted). The first time a
// tab is selected its panel is recorded in the Local set; its component is made
// once per render of the tab list, so a tab switch reuses it.
func (t *TabsElement[T]) keptPanels(ctx *Context, sel int, panels map[T]*ComponentElement, panel Style) []Element {
	built := Local(ctx, "built", map[T]bool{}).Peek()
	if sel >= 0 && t.tabs[sel].Panel != nil {
		built[t.tabs[sel].Value] = true
	}
	var out []Element
	for i, tb := range t.tabs {
		if !built[tb.Value] {
			continue
		}
		pc := panels[tb.Value]
		if pc == nil {
			fn := tb.Panel
			pc = Component(func(*Context) Element { return fn() })
			panels[tb.Value] = pc
		}
		// Each panel has its own key, and is named by its tab as the panel is by default.
		out = append(out, Div().asPart("Tabs.Panel").Key(t.scope()+"/panel:"+fmt.Sprint(tb.Value)).Column().Grow(1).MinHeight(Px(0)).
			Role(access.RoleTabPanel).Label(tb.Label).Display(i == sel).Style(panel).
			LabelledBy(t.tabKey(tb.Value)).Child(pc))
	}
	return out
}

// choose sets the selected value to v and runs OnChange.
func (t *TabsElement[T]) choose(v T) {
	t.selected.Set(v)
	if t.onChange != nil {
		t.onChange(v)
	}
}

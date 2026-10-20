package ui

import (
	"fmt"
	"slices"
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
// while the tab has keyboard focus. Bar is the row that holds the strip with the
// Prefix, the Suffix, the scroll buttons and the overflow menu's button; it is
// built only when one of them is shown. ScrollButton and MenuButton are the Root
// of those buttons. See docs/styling.md.
type TabsStyle struct {
	Bar             Style // the row of the prefix, the strip, its buttons and the suffix
	List            Style // the strip that holds the tabs
	Tab             Style // each tab
	TabHover        Style // a tab the pointer is over, if it is enabled and not selected
	TabSelected     Style // the selected tab
	TabDisabled     Style // a disabled tab
	TabFocusVisible Style // a tab that has keyboard focus
	Indicator       Style // the line under a tab in TabsUnderline; only the selected one shows it
	Panel           Style // the panel of the selected tab
	DropLine        Style // the line that shows where a dragged tab lands (see Reorderable)
	ScrollButton    Style // each button that scrolls the strip while the tabs overflow it
	MenuButton      Style // the button that opens the menu of all the tabs (see OverflowMenu)
}

// merge returns s with the set fields of o applied on top, part by part.
func (s TabsStyle) merge(o TabsStyle) TabsStyle {
	return TabsStyle{
		Bar:             s.Bar.Merge(o.Bar),
		List:            s.List.Merge(o.List),
		Tab:             s.Tab.Merge(o.Tab),
		TabHover:        s.TabHover.Merge(o.TabHover),
		TabSelected:     s.TabSelected.Merge(o.TabSelected),
		TabDisabled:     s.TabDisabled.Merge(o.TabDisabled),
		TabFocusVisible: s.TabFocusVisible.Merge(o.TabFocusVisible),
		Indicator:       s.Indicator.Merge(o.Indicator),
		Panel:           s.Panel.Merge(o.Panel),
		DropLine:        s.DropLine.Merge(o.DropLine),
		ScrollButton:    s.ScrollButton.Merge(o.ScrollButton),
		MenuButton:      s.MenuButton.Merge(o.MenuButton),
	}
}

// plain returns s as high contrast allows it (see plainStyle). The indicator's height
// is the system's too, so a size cannot hide the selection either.
func (s TabsStyle) plain() TabsStyle {
	ind := plainStyle(s.Indicator)
	ind.Height = Auto
	return TabsStyle{
		Bar:             plainStyle(s.Bar),
		List:            plainStyle(s.List),
		Tab:             plainStyle(s.Tab),
		TabHover:        plainStyle(s.TabHover),
		TabSelected:     plainStyle(s.TabSelected),
		TabDisabled:     plainStyle(s.TabDisabled),
		TabFocusVisible: plainStyle(s.TabFocusVisible),
		Indicator:       ind,
		Panel:           plainStyle(s.Panel),
		DropLine:        plainStyle(s.DropLine),
		ScrollButton:    plainStyle(s.ScrollButton),
		MenuButton:      plainStyle(s.MenuButton),
	}
}

// tabsDefaults returns the built-in look of a tab list of variant v, from the
// colors of th.
func tabsDefaults(th *Theme, v TabsVariant) TabsStyle {
	ring := Shadow{Spread: 2, Color: th.FocusRing}
	d := TabsStyle{
		Bar:  Style{Direction: Row, Align: Center, Gap: 4},
		List: Style{Direction: Row, Gap: 4, BorderEdges: Edges{Bottom: 1}, BorderColor: th.Border},
		Tab: Style{Direction: Row, Align: Center, Gap: 6, MinHeight: Px(36), Padding: XY(12, 0),
			NoShrink: true, Radius: th.Radius / 2, TextColor: th.TextMuted, Transition: 110 * time.Millisecond},
		TabHover:        Style{Background: th.SurfaceHover},
		TabSelected:     Style{TextColor: th.Text},
		TabFocusVisible: Style{Shadows: []Shadow{ring}},
		Indicator:       Style{Height: Px(2), Background: th.Accent, Transition: 120 * time.Millisecond},
		DropLine:        Style{Width: Px(2), Background: th.Accent},
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
//
// When the tabs are wider than the strip, the strip scrolls sideways, as WinUI's
// TabView does: two buttons after it scroll it back and forward (see
// ScrollButtons), a vertical wheel over it scrolls it sideways, and the selected
// tab is scrolled into view whenever the selection changes, with a little of the
// next tab showing. The buttons are not Tab stops: the keys that move between the
// tabs move the strip too. Tabs that fit are drawn as they are without overflow.
type TabsElement[T comparable] struct {
	selected    *Signal[T]
	tabs        []Tab[T]
	name        string // the accessible name of the tab list, see Label
	key         string
	onChange    func(T)
	onReselect  func(T)
	reorder     bool // see Reorderable
	onReorder   func([]T)
	keep        bool // see KeepMounted
	variant     TabsVariant
	styles      TabsStyle
	renderTab   func(Tab[T], PartState) Element
	size        sizing       // the tab list's sizes, from the sizing methods (see Sizing in docs/styling.md)
	focusHandle *FocusHandle // see FocusHandle
	prefix      Element      // see Prefix
	suffix      Element      // see Suffix
	menu        bool         // see OverflowMenu
	noScroll    bool         // see ScrollButtons
	tabMax      Length       // see TabMaxWidth; Auto for none
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

// Reorderable lets the user move the tabs: drag a tab and drop it on the half
// of another tab it should go beside, where a line shows it will land, or press
// Ctrl+Shift+Left or Ctrl+Shift+Right on the focused tab. Assistive technology
// hears the new position. The Tabs keeps the user's order; tabs it has not seen
// before go after the others, in the order given. A disabled tab cannot be
// dragged, but a tab can be put beside it.
func (t *TabsElement[T]) Reorderable() *TabsElement[T] { t.reorder = true; return t }

// OnReorder runs fn with the values of the tabs in their new order after the
// user moves a tab (see Reorderable), for example to save it. Passing the tabs
// in that order next time restores it.
func (t *TabsElement[T]) OnReorder(fn func(order []T)) *TabsElement[T] { t.onReorder = fn; return t }

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

// Prefix puts e before the strip, in the same row: outside the tab list, and
// outside what scrolls when the tabs overflow.
func (t *TabsElement[T]) Prefix(e Element) *TabsElement[T] { t.prefix = e; return t }

// Suffix puts e after the strip and its buttons, in the same row, such as a
// button that adds a tab. It does not scroll with the tabs.
func (t *TabsElement[T]) Suffix(e Element) *TabsElement[T] { t.suffix = e; return t }

// OverflowMenu adds a button after the strip that opens a menu of every tab, in
// order and with its full label: the selected tab is checked and a disabled one
// cannot be chosen. Choosing a tab selects it as a click does, then scrolls it
// into view and focuses it. The button is one Tab stop after the tab list.
func (t *TabsElement[T]) OverflowMenu() *TabsElement[T] { t.menu = true; return t }

// ScrollButtons sets whether buttons that scroll the strip are shown while the
// tabs overflow it; they are on by default. They are hidden while the tabs fit,
// and each is disabled at its end of the strip. Without them the strip still
// scrolls, with the wheel, the keys and the selection.
func (t *TabsElement[T]) ScrollButtons(on bool) *TabsElement[T] { t.noScroll = !on; return t }

// TabMaxWidth caps the width of each tab at l. A longer label is cut short with
// an ellipsis, and its tooltip shows it whole; its accessible name is still the
// whole label. With RenderTab the cap applies to the tab, and the content is the
// app's to fit.
func (t *TabsElement[T]) TabMaxWidth(l Length) *TabsElement[T] { t.tabMax = l; return t }

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
		// order is the order of the values once the user has moved a tab (see
		// Reorderable); until then the tabs are shown in the order given.
		order := Local(ctx, "order", []T(nil))
		tabs := t.tabs
		if t.reorder {
			tabs = arrangeTabs(t.tabs, order.Get())
		}
		opts := make([]Option[T], len(tabs))
		for i, tb := range tabs {
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
		if sel >= 0 && tabs[sel].Popup != nil {
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
		// ext is what the strip's paint reports (see extentWatch): whether the tabs
		// overflow it, and whether it can scroll back and forward from where it is.
		// watch reports the strip, and slot the scroll buttons, whose room the strip
		// must be able to give back before they are hidden.
		ext := Local(ctx, "extent", tabsExtent{})
		watch := Local(ctx, "extentWatch", &extentWatch{}).Peek()
		slot := Local(ctx, "slotWatch", &extentWatch{}).Peek()
		buttons := !t.noScroll
		watch.fn = func(x, maxX, _ float32) {
			e := ext.Peek()
			n := tabsExtent{over: maxX > 0.5, back: x > 0.5, forward: x < maxX-0.5}
			if e.over && buttons && slot.seen {
				// Shown buttons take room from the strip. They go only once the tabs
				// fit in that room too, so that the strip does not flicker between the
				// two at one width.
				n.over = maxX > slot.last[2]+st.Bar.Gap+0.5
			}
			if n != e {
				ext.Set(n)
			}
		}
		e := ext.Get()
		over := e.over
		inBar := t.prefix != nil || t.suffix != nil || t.menu || over && buttons
		// view is what the strip showed last: whether it was in the bar, and the
		// selected tab it scrolled to. The bar comes and goes with the scroll buttons
		// when nothing else is in it, and the strip is built anew then: focus in it
		// goes back to the selected tab, which is scrolled to again.
		view := Local(ctx, "view", &tabsView[T]{}).Peek()
		rebuilt := view.built && view.inBar != inBar
		if rebuilt && (focus.Focused() || inPopup()) {
			focus.Focus()
		}
		reveal := over && (rebuilt || !view.over || view.shown != cur || view.again)
		view.built, view.inBar, view.over, view.shown, view.again = true, inBar, over, cur, false
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
		// pick chooses the tab at index i from the overflow menu, scrolls it into view
		// and focuses it, unless its popup opened and took the focus.
		pick := func(i int) {
			choose(i)
			if i == sel && over {
				// A reselect does not change the selection, which the next render scrolls to.
				if n := rt.byID[focus.id]; n != nil {
					rt.scrollIntoView(n)
				}
			}
			if !isOpen() {
				focus.Focus()
			}
		}
		// move puts the tab at index from at index to, keeps the order, reports it,
		// and announces the tab's new position.
		move := func(from, to int) {
			if from == to || from < 0 || to < 0 || to >= len(tabs) {
				return
			}
			vals := make([]T, 0, len(tabs))
			for _, tb := range tabs {
				vals = append(vals, tb.Value)
			}
			v := vals[from]
			vals = slices.Insert(slices.Delete(vals, from, from+1), to, v)
			order.Set(vals)
			view.again = true // the moved tab, the selected one, is scrolled into view
			if t.onReorder != nil {
				t.onReorder(slices.Clone(vals))
			}
			Announce(ctx.Strings().TabMoved(tabs[from].Label, to+1, len(vals)), access.Polite)
		}
		// mark is where a dragged tab would land: before or after the tab with
		// the key, which draws the drop line on that side.
		mark := Local(ctx, "dropMark", tabDropMark{})
		setMark := func(m tabDropMark) {
			if mark.Peek() != m {
				mark.Set(m)
			}
		}
		// The dragged tab is dimmed, except under high contrast, where it keeps its colors.
		source := Style{Opacity: 0.5}
		if th.HighContrast {
			source = Style{}
		}
		// drawTab returns the tab at index i, in the state ps describes.
		drawTab := func(i int, ps PartState) *DivElement {
			tb := tabs[i]
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
			tab.revealRow = on && reveal // the next paint scrolls the strip to it
			capped := !t.tabMax.IsAuto()
			if capped {
				tab.MaxWidth(t.tabMax)
			}
			if t.renderTab != nil {
				tab.Child(Div().AccessHidden().Child(t.renderTab(tb, ps)))
			} else {
				if tb.Icon != nil {
					tab.Child(Icon(tb.Icon).Size(16))
				}
				if capped {
					tab.Child(Text(tb.Label).Truncate()).Tooltip(tb.Label)
				} else {
					tab.Child(Text(tb.Label).NoWrap())
				}
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
			if t.reorder {
				// Every tab is a drop target, a disabled one too, so a tab can be put
				// beside it; the half of the tab under the pointer picks the side.
				key := t.tabKey(tb.Value)
				if m := mark.Get(); m.key == key {
					line := Div().asPart("Tabs.DropLine").Absolute().Top(0).Bottom(0).Style(st.DropLine)
					if m.after {
						line.Right(-3)
					} else {
						line.Left(-3)
					}
					tab.Child(line)
				}
				tab.OnDrop(Drop(func(p tabDrag, e *DropEvent) {
					setMark(tabDropMark{})
					from := slices.IndexFunc(tabs, func(x Tab[T]) bool { return any(x.Value) == p.value })
					to := i
					if e.LocalX > e.Width/2 {
						to++
					}
					if from < to {
						to-- // the tab leaves its place before it is put back
					}
					move(from, to)
				}).
					Accept(func(p tabDrag) bool { return p.scope == t.scope() && p.value != any(tb.Value) }).
					Move(func(p tabDrag, e *DropEvent) { setMark(tabDropMark{key: key, after: e.LocalX > e.Width/2}) }).
					Leave(func() { setMark(tabDropMark{}) }))
			}
			if tb.Disabled {
				tab.access().disabled = true
				tab.Cursor(CursorNotAllowed)
				return tab
			}
			if t.reorder {
				label := tb.Label
				tab.Dragging(source).OnDrag(Drag(tabDrag{scope: t.scope(), value: tb.Value}).Preview(func() Element {
					return Div().Padding(6).Background(th.Surface).Border(1, th.Border).Child(Text(label))
				}).End(func(DragEnd) { setMark(tabDropMark{}) }))
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
					if t.reorder && e.Mods == input.ModCtrl|input.ModShift && (e.Key == input.KeyLeft || e.Key == input.KeyRight) {
						e.StopPropagation()
						if e.Key == input.KeyLeft {
							move(i, i-1)
						} else {
							move(i, i+1)
						}
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

		// The strip is never wider than the room it has: a segmented strip, as wide
		// as its tabs, stops there, and the tabs overflow it.
		strip := Div().asPart("Tabs.List").Role(access.RoleTabList).MaxWidth(Full)
		if inBar {
			// In the bar's row the strip takes the room the other parts leave, and can
			// give it all up, so that the tabs overflow the strip and not the row.
			strip.MinWidth(Px(0))
			if t.variant == TabsUnderline {
				strip.Grow(1)
			}
		}
		strip = t.size.apply(strip.Style(st.List))
		if t.name != "" {
			strip.Label(t.name)
		}
		strip.extent, strip.noScrollbar = watch, true
		if over {
			strip.ScrollX().OnScroll(func(e *ScrollEvent) {
				// A vertical wheel over the strip scrolls it sideways while it can move
				// that way; at an end the page under it scrolls on. A wheel over an open
				// popup, which hangs below the strip, is the popup's.
				n := rt.byID[watch.id]
				if e.DeltaX != 0 || e.DeltaY == 0 || n == nil || !n.abs.Contains(rt.mouseX, rt.mouseY) {
					return
				}
				if watch.scrollBy(e.DeltaY) {
					e.StopPropagation()
					setPopup(false)
				}
			})
		}
		for i, tb := range tabs {
			if t.renderTab == nil {
				strip.Child(drawTab(i, PartState{Selected: i == sel, Disabled: tb.Disabled, HighContrast: th.HighContrast}))
				continue
			}
			// With a render slot the tab is its own component, so that it re-renders
			// when its pointer or focus state changes, and the slot gets the new state.
			i := i
			strip.Child(Component(func(ctx *Context) Element {
				return drawTab(i, PartState{Hovered: ctx.hovered(), Pressed: ctx.pressed(), Focused: ctx.isFocused(),
					Selected: i == sel, Disabled: tabs[i].Disabled, HighContrast: th.HighContrast})
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
			Child(t.bar(ctx, st, strip, inBar, e, watch, slot, tabs, sel, pick))
		if t.keep {
			group.Children(t.keptPanels(ctx, tabs, sel, panels, st.Panel)...)
		} else if sel >= 0 && tabs[sel].Panel != nil {
			if p := tabs[sel].Panel(); p != nil {
				// The panel is named by its tab: the label is its name, and LabelledBy
				// links it to the tab for the relation.
				group.Child(Div().asPart("Tabs.Panel").Key(t.scope() + "/panel").Column().Grow(1).MinHeight(Px(0)).
					Role(access.RoleTabPanel).Label(tabs[sel].Label).Style(st.Panel).
					LabelledBy(t.tabKey(tabs[sel].Value)).Child(p))
			}
		}
		return group
	})
}

// tabsExtent is what the paint of a tab strip reports: whether the tabs overflow
// it, and whether it can scroll back and forward from where it is.
type tabsExtent struct {
	over, back, forward bool
}

// tabsView is what a tab strip showed at its last render: whether it was in the
// bar, whether it overflowed, and the selected tab it scrolled to. again asks the
// next render to scroll to the selected tab even though it is the same.
type tabsView[T comparable] struct {
	built, inBar, over, again bool
	shown                     T
}

// bar returns what the tab list's group holds above the panel: the strip alone, or,
// when inBar is set, the bar's row of the prefix, the strip, the scroll buttons,
// the overflow menu's button and the suffix. The scroll buttons stay in the bar
// while the tabs fit, hidden by Display, so that nothing after them is built anew
// when they appear.
func (t *TabsElement[T]) bar(ctx *Context, st TabsStyle, strip *DivElement, inBar bool, e tabsExtent,
	watch, slot *extentWatch, tabs []Tab[T], sel int, pick func(int)) Element {
	if !inBar {
		return strip
	}
	strs := ctx.Strings()
	bar := Div().asPart("Tabs.Bar").Key(t.scope() + "/bar").Style(st.Bar)
	if t.prefix != nil {
		bar.Child(t.prefix)
	}
	bar.Child(strip)
	if !t.noScroll {
		// Each button scrolls the strip by most of its width, so that a tab cut at
		// one edge is shown whole at the other. They take no focus, so Tab does not
		// reach them and a press on them leaves the focus where it is.
		step := func(dir float32) func() {
			return func() { watch.scrollBy(dir * 0.75 * watch.last[2]) }
		}
		back := IconButton(icons.ChevronLeft).Small().Label(strs.ScrollTabsBack).Key("back").
			Disabled(!e.back).Style(st.ScrollButton).OnClick(step(-1))
		fwd := IconButton(icons.ChevronRight).Small().Label(strs.ScrollTabsForward).Key("forward").
			Disabled(!e.forward).Style(st.ScrollButton).OnClick(step(1))
		back.noFocus, fwd.noFocus = true, true
		buttons := Div().Key(t.scope() + "/scroll").Row().Align(Center).Gap(2).KeepFocus().
			Display(e.over).Child(back).Child(fwd)
		buttons.extent = slot
		bar.Child(buttons)
	}
	if t.menu {
		trigger := IconButton(icons.MoreHorizontal).Small().Label(strs.AllTabs).Key("all").Style(st.MenuButton)
		bar.Child(DropdownMenu(trigger, func() []MenuItem {
			items := make([]MenuItem, len(tabs))
			for i, tb := range tabs {
				items[i] = MenuItem{Label: tb.Label, Icon: tb.Icon, Disabled: tb.Disabled,
					Check: MenuCheckRadio, Checked: i == sel, OnClick: func() { pick(i) }}
			}
			return items
		}).Key(t.scope() + "/menu"))
	}
	if t.suffix != nil {
		bar.Child(t.suffix)
	}
	return bar
}

// tabDrag is the payload of a dragged tab: the value of the tab, and the scope
// of its tab list, so that a tab is dropped only on its own list.
type tabDrag struct {
	scope string
	value any
}

// tabDropMark is where a dragged tab would land: beside the tab whose key it
// holds, after it when after is set. The zero value marks nothing.
type tabDropMark struct {
	key   string
	after bool
}

// arrangeTabs returns tabs in order: the tabs whose values are in order first,
// in that order, then the others as given. Values in order with no tab are
// skipped.
func arrangeTabs[T comparable](tabs []Tab[T], order []T) []Tab[T] {
	if len(order) == 0 {
		return tabs
	}
	out := make([]Tab[T], 0, len(tabs))
	placed := make(map[T]bool, len(tabs))
	for _, v := range order {
		if i := slices.IndexFunc(tabs, func(x Tab[T]) bool { return x.Value == v }); i >= 0 && !placed[v] {
			out = append(out, tabs[i])
			placed[v] = true
		}
	}
	for _, tb := range tabs {
		if !placed[tb.Value] {
			out = append(out, tb)
		}
	}
	return out
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
func (t *TabsElement[T]) keptPanels(ctx *Context, tabs []Tab[T], sel int, panels map[T]*ComponentElement, panel Style) []Element {
	built := Local(ctx, "built", map[T]bool{}).Peek()
	if sel >= 0 && tabs[sel].Panel != nil {
		built[tabs[sel].Value] = true
	}
	var out []Element
	for i, tb := range tabs {
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

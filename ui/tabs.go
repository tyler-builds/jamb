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
// panels of the other tabs are not built. A disabled tab is shown but cannot be
// selected.
type Tab[T comparable] struct {
	Value    T
	Label    string
	Icon     *icons.Icon
	Disabled bool
	Panel    func() Element
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
	selected *Signal[T]
	tabs     []Tab[T]
	name     string // the accessible name of the tab list, see Label
	key      string
	onChange func(T)
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

// scope is the prefix of the keys of this tab list.
func (t *TabsElement[T]) scope() string {
	if t.key != "" {
		return t.key
	}
	return fmt.Sprintf("tabs:%p", t.selected)
}

// tabKey is the key of the tab with value v. A panel names itself by it.
func (t *TabsElement[T]) tabKey(v T) string { return t.scope() + "/" + fmt.Sprint(v) }

func (t *TabsElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		th := ctx.Theme()
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
		// choose selects the tab at index i and moves focus to it.
		choose := func(i int) {
			if i < 0 || opts[i].Disabled || i == sel {
				return
			}
			t.choose(opts[i].Value)
			focus.Focus()
		}

		strip := Div().Row().Gap(4).BorderEdges(Edges{Bottom: 1}, th.Border).Role(access.RoleTabList)
		if t.name != "" {
			strip.Label(t.name)
		}
		ring := Shadow{Spread: 2, Color: th.FocusRing}
		for i, tb := range t.tabs {
			on := i == sel
			tab := t.tabView(th, tb, on, tb.Disabled)
			tab.Key(t.tabKey(tb.Value))
			tab.Role(access.RoleTab).Label(tb.Label).Selected(on)
			if on && !tb.Disabled {
				tab.FocusHandle(focus) // a handle makes the tab focusable, so a disabled one has none
			}
			if i != stop {
				tab.TabStop(false) // reached with the arrow keys, not Tab
			}
			if tb.Disabled {
				tab.access().disabled = true
				tab.Cursor(CursorNotAllowed)
				if !th.HighContrast {
					tab.Opacity(0.5) // under high contrast the label is drawn in GrayText instead
				}
				strip.Child(tab)
				continue
			}
			i := i
			tab.Cursor(CursorPointer).Focusable().FocusVisible(Style{Shadows: []Shadow{ring}}).
				Hover(Style{Background: th.SurfaceHover}).
				OnClick(func() { choose(i) }).
				OnKeyDown(func(e *KeyEvent) {
					if e.Mods != 0 || e.Repeat {
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
			strip.Child(tab)
		}

		group := Div().Column().Gap(12).Grow(1).MinHeight(Px(0)).Key(t.scope()).
			// Ctrl+Tab and Ctrl+Shift+Tab reach here from the tab list and from the
			// panel, since both are inside the group.
			OnKeyDown(func(e *KeyEvent) {
				if e.Key != input.KeyTab || e.Repeat || (e.Mods != input.ModCtrl && e.Mods != input.ModCtrl|input.ModShift) {
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
		if sel >= 0 && t.tabs[sel].Panel != nil {
			if p := t.tabs[sel].Panel(); p != nil {
				// The panel is named by its tab: the label is its name, and LabelledBy
				// links it to the tab for the relation.
				group.Child(Div().Key(t.scope() + "/panel").Column().Grow(1).MinHeight(Px(0)).
					Role(access.RoleTabPanel).Label(t.tabs[sel].Label).
					LabelledBy(t.tabKey(t.tabs[sel].Value)).Child(p))
			}
		}
		return group
	})
}

// tabView is the drawn tab: its icon and label above a line that marks the selected
// tab. The selected tab is also bold and the line is solid Accent, so the selection
// is not carried by color alone.
func (t *TabsElement[T]) tabView(th *Theme, tb Tab[T], on, disabled bool) *DivElement {
	// A disabled tab under high contrast is GrayText throughout, label, icon and line.
	// Elsewhere it is dimmed as a whole, by the opacity its caller sets.
	c := th.TextMuted
	if on {
		c = th.Text
	}
	line := Transparent
	if on {
		line = th.Accent
	}
	if disabled && th.HighContrast {
		c, line = th.Disabled, th.Disabled
	}
	text := Text(tb.Label).NoWrap().Color(c)
	if on {
		text.Semibold()
	}
	label := Div().Row().Align(Center).Gap(6).MinHeight(Px(36)).PaddingX(12).Shrink(0)
	if tb.Icon != nil {
		label.Child(Icon(tb.Icon).Size(16).Color(c))
	}
	label.Child(text)
	// The line eases in and out of its color, which is instant under reduced motion.
	bar := Div().Height(Px(2)).Background(line).Transition(120 * time.Millisecond)
	return Div().Column().Shrink(0).Radius(th.Radius / 2).Child(label).Child(bar)
}

// choose sets the selected value to v and runs OnChange.
func (t *TabsElement[T]) choose(v T) {
	t.selected.Set(v)
	if t.onChange != nil {
		t.onChange(v)
	}
}

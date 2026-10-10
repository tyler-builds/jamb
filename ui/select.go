package ui

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// selectTypeahead is the pause after which typing starts a new search.
const selectTypeahead = time.Second

// SelectElement is a drop-down list: a field that shows the chosen option, and
// opens a list of the options below it (above it when there is no room).
//
// Click, Enter, Space, Alt+Down or F4 open the list. While it is closed, Up and
// Down choose the previous and next option, Home and End choose the first and
// last, and typing letters chooses the first option that starts with them. In
// the list, Up and Down move the highlight (without wrapping), Home and End go
// to the ends, and typing moves to a matching option. Enter or Space chooses the
// highlighted option and closes the list. Escape, Tab and a press outside the
// list close it without a change. Focus returns to the field when the list
// closes, as it does for a menu.
type SelectElement[T comparable] struct {
	value       *Signal[T]
	options     []Option[T]
	placeholder string
	name        string // the accessible name, see Label
	labelledBy  []string
	key         string
	disabled    bool
	width       float32
	onChange    func(T)
}

// Select creates a drop-down list that reads and writes value. The options are
// shown in order. The default width is 220 px.
func Select[T comparable](value *Signal[T], options ...Option[T]) *SelectElement[T] {
	return &SelectElement[T]{value: value, options: options, width: 220}
}

func (*SelectElement[T]) isElement()           {}
func (s *SelectElement[T]) elementKey() string { return s.key }

func (s *SelectElement[T]) Key(k string) *SelectElement[T] { s.key = k; return s }

// Label names the field for assistive technology, such as "Channel".
func (s *SelectElement[T]) Label(name string) *SelectElement[T] { s.name = name; return s }

// LabelledBy names the field by the text of the elements with the given keys;
// see DivElement.LabelledBy.
func (s *SelectElement[T]) LabelledBy(keys ...string) *SelectElement[T] {
	s.labelledBy = keys
	return s
}

// Placeholder is the text shown while no option is chosen.
func (s *SelectElement[T]) Placeholder(text string) *SelectElement[T] {
	s.placeholder = text
	return s
}

// Disabled shows the field but does not let it change.
func (s *SelectElement[T]) Disabled(d bool) *SelectElement[T] { s.disabled = d; return s }

// OnChange runs fn with the chosen value after the user chooses an option, by the
// list or by the keys. The signal is set first.
func (s *SelectElement[T]) OnChange(fn func(T)) *SelectElement[T] { s.onChange = fn; return s }

// Width sets the width of the field, in logical pixels. The list is at least as wide.
func (s *SelectElement[T]) Width(px float32) *SelectElement[T] { s.width = px; return s }

// selectType is the type-ahead search: the letters typed so far and when the last
// letter was typed.
type selectType struct {
	text string
	at   time.Time
}

func (s *SelectElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		rt := ctx.rt
		cur := s.value.Get()
		sel := indexOf(s.options, cur)
		open := Local(ctx, "open", false)
		hl := Local(ctx, "hl", -1) // the highlighted option while the list is open
		typed := Local(ctx, "typed", selectType{})
		// back is the field's focus handle. Focus returns to the field when the list
		// closes, whichever way it was opened.
		back := Local(ctx, "back", NewFocusHandle())
		ring := Shadow{Spread: 2, Color: t.FocusRing}

		// shut closes the list. Focus returns to the field when restore is set.
		shut := func(restore bool) {
			if !open.Peek() {
				return
			}
			open.Set(false)
			rt.menuClosed(restore)
		}
		// openList opens the list with the chosen option highlighted, or the first
		// option that can be chosen.
		openList := func() {
			if s.disabled || open.Peek() {
				return
			}
			rt.menuOpened()
			rt.menuReturn = back.Peek()
			start := sel
			if start < 0 || s.options[start].Disabled {
				start = nextEnabled(s.options, -1, 1, false)
			}
			hl.Set(start)
			open.Set(true)
		}
		choose := func(i int) {
			if i < 0 || s.options[i].Disabled {
				return
			}
			s.choose(s.options[i].Value)
			shut(true)
		}
		// pick chooses option i while the list is closed, for the keys that move
		// the choice.
		pick := func(i int) {
			if i >= 0 && i != sel && !s.options[i].Disabled {
				s.choose(s.options[i].Value)
			}
		}
		// search returns the option that the typed letter, with the letters typed
		// before it, selects, starting at from. A pause resets the search. A single
		// letter moves past from, so that repeated letters cycle through the options.
		search := func(r rune, from int) int {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return -1
			}
			now := rt.now()
			prev := typed.Peek()
			text := string(r)
			if now.Sub(prev.at) < selectTypeahead {
				text = prev.text + text
			}
			typed.Set(selectType{text: text, at: now})
			start := max(from, 0)
			if utf8.RuneCountInString(text) == 1 {
				start = (from + 1 + len(s.options)) % max(len(s.options), 1)
			}
			return matchLabel(s.options, start, text)
		}

		// The field is the combo box. Its text is hidden from assistive technology,
		// which reads the Value of the box instead.
		label := ""
		if sel >= 0 {
			label = s.options[sel].Label
		}
		chevron := icons.ChevronDown
		if open.Get() {
			chevron = icons.ChevronUp
		}
		// Under high contrast a disabled select is all GrayText, with no dimming.
		placeholderColor, chevronColor := t.TextSubtle, t.TextMuted
		if s.disabled && t.HighContrast {
			placeholderColor, chevronColor = t.Disabled, t.Disabled
		}
		shown := Text(label).NoWrap()
		if sel < 0 {
			shown = Text(s.placeholder).NoWrap().Color(placeholderColor)
		}
		field := Div().Row().Align(Center).Gap(8).Width(Px(s.width)).MinHeight(Px(34)).PaddingX(10).
			Radius(t.Radius).Background(t.Surface).Border(1, t.BorderStrong).Transition(110 * time.Millisecond).
			Child(Div().Grow(1).Hidden().Child(shown)).
			Child(Icon(chevron).Size(16).Color(chevronColor))
		a := field.access()
		field.Role(access.RoleComboBox).Expanded(open.Get())
		a.value, a.readOnly = label, true
		if s.name != "" {
			field.Label(s.name)
		}
		if len(s.labelledBy) > 0 {
			field.LabelledBy(s.labelledBy...)
		}
		field.FocusHandle(back.Peek())
		if s.disabled {
			field.Cursor(CursorNotAllowed)
			a.disabled = true
			if t.HighContrast {
				field.TextColor(t.Disabled).Border(1, t.Disabled)
			} else {
				field.Opacity(0.5)
			}
			return field
		}
		field.Cursor(CursorPointer).Hover(Style{BorderColor: t.TextMuted}).
			FocusVisible(Style{Shadows: []Shadow{ring}}).
			OnClick(func() {
				if open.Peek() {
					shut(true)
				} else {
					openList()
				}
			}).
			OnKeyDown(func(e *KeyEvent) {
				if open.Peek() || e.Repeat {
					return // with the list open, the list has the keys
				}
				switch {
				case e.Key == input.KeyDown && e.Mods == input.ModAlt, e.Key == input.KeyF4 && e.Mods == 0:
					openList()
				case e.Mods != 0:
					return
				case e.Key == input.KeyDown:
					pick(listStep(s.options, sel, 1))
				case e.Key == input.KeyUp:
					pick(listStep(s.options, sel, -1))
				case e.Key == input.KeyHome:
					pick(listEdge(s.options, false))
				case e.Key == input.KeyEnd:
					pick(listEdge(s.options, true))
				default:
					return
				}
				e.StopPropagation()
			}).
			OnTextInput(func(text string) {
				if open.Peek() {
					return
				}
				r, _ := utf8.DecodeRuneInString(text)
				if i := search(r, sel); i >= 0 {
					pick(i)
				}
			})

		if open.Get() {
			field.Child(s.list(t, sel, hl.Get(), shut, choose, func(r rune) int {
				return search(r, hl.Peek())
			}, func(i int) { hl.Set(i) }))
		}
		return field
	})
}

// list is the open list of the field: an overlay below it, which takes focus.
// Its rows are list items, the chosen one is selected, and the highlighted one
// is the row that has focus.
func (s *SelectElement[T]) list(t *Theme, sel, hl int, shut func(bool), choose func(int),
	search func(rune) int, highlight func(int)) Element {
	name := s.name
	if name == "" {
		name = "Options"
	}
	keys := func(e *KeyEvent) {
		if e.Mods&(input.ModCtrl|input.ModAlt|input.ModSuper) != 0 {
			return
		}
		switch e.Key {
		case input.KeyDown:
			highlight(listStep(s.options, hl, 1))
		case input.KeyUp:
			highlight(listStep(s.options, hl, -1))
		case input.KeyHome:
			highlight(listEdge(s.options, false))
		case input.KeyEnd:
			highlight(listEdge(s.options, true))
		case input.KeyEnter, input.KeySpace:
			if !e.Repeat && hl >= 0 {
				choose(hl)
			}
		case input.KeyEscape:
			if e.Repeat {
				return
			}
			shut(true)
		case input.KeyTab:
			shut(true)
			return // not stopped: focus moves on from the field
		default:
			return
		}
		e.StopPropagation()
	}
	panel := Div().Key("jamb.listbox").Column().Padding(4).MinWidth(Px(s.width)).
		Background(t.Surface).Border(1, t.Border).Radius(t.Radius).Shadow(ShadowMd).
		Enter(Fade().Slide(0, -6).Grow(0.98), Animation{Duration: 140 * time.Millisecond}).
		Exit(Fade().Grow(0.98), Animation{Duration: 90 * time.Millisecond}).
		Overlay(BelowStart).OverlayGap(4).
		Role(access.RoleList).SelectionContainer().Label(name).
		AutoFocus().TabStop(false).OnKeyDown(keys).
		OnTextInput(func(text string) {
			r, _ := utf8.DecodeRuneInString(text)
			if i := search(r); i >= 0 {
				highlight(i)
			}
		}).
		OnClickOutside(func() { shut(false) }).
		OnClick(func() {}) // a press on the list's padding must not reach the field
	for i, o := range s.options {
		on := i == sel
		hi := i == hl && !o.Disabled
		mark := Div().Size(Px(16), Px(16)).Center().Shrink(0)
		if on {
			mark.Child(Icon(icons.Check).Size(14).Color(t.Text))
		}
		row := Div().Row().Align(Center).Gap(8).MinHeight(Px(28)).PaddingX(8).Radius(4).Shrink(0).
			FontSize(13).TextColor(t.Text).
			Role(access.RoleListItem).Selected(on).Label(o.Label).
			Child(mark).Child(Text(o.Label).NoWrap())
		row.access().highlighted = hi
		if hi {
			row.Background(t.AccentSoft)
		}
		if o.Disabled {
			row.TextColor(t.Disabled)
			row.access().disabled = true
		} else {
			i := i
			row.OnMouseEnter(func() { highlight(i) }).OnClick(func() { choose(i) })
		}
		panel.Child(row)
	}
	return panel
}

// indexOf returns the index of the first option whose value is v, or -1.
func indexOf[T comparable](options []Option[T], v T) int {
	for i, o := range options {
		if o.Value == v {
			return i
		}
	}
	return -1
}

// listStep returns the next option that can be chosen after from, moving dir (+1
// or -1) without wrapping. From -1 it starts at the first option going forward,
// or the last going back. When there is no such option it returns from.
func listStep[T comparable](options []Option[T], from, dir int) int {
	n := len(options)
	i := from + dir
	if from < 0 || from >= n {
		i = 0
		if dir < 0 {
			i = n - 1
		}
	}
	for ; i >= 0 && i < n; i += dir {
		if !options[i].Disabled {
			return i
		}
	}
	return from
}

// listEdge returns the first option that can be chosen, or the last when last is
// set, or -1 when none can.
func listEdge[T comparable](options []Option[T], last bool) int {
	n := len(options)
	for k := range n {
		i := k
		if last {
			i = n - 1 - k
		}
		if !options[i].Disabled {
			return i
		}
	}
	return -1
}

// matchLabel returns the first option, from start and wrapping around, that
// can be chosen and whose label starts with prefix, ignoring case. It returns -1
// when none does.
func matchLabel[T comparable](options []Option[T], start int, prefix string) int {
	n := len(options)
	if n == 0 {
		return -1
	}
	prefix = strings.ToLower(prefix)
	for k := range n {
		i := ((start+k)%n + n) % n
		if !options[i].Disabled && strings.HasPrefix(strings.ToLower(options[i].Label), prefix) {
			return i
		}
	}
	return -1
}

// choose sets the value to v and runs OnChange.
func (s *SelectElement[T]) choose(v T) {
	s.value.Set(v)
	if s.onChange != nil {
		s.onChange(v)
	}
}

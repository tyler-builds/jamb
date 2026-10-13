package ui

import (
	"fmt"
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

// SelectStyle is the look of each part of a select. A part's style merges over
// the built-in defaults, then over the theme's Select, then over the Styles of
// the instance; see docs/styling.md. The states are parts of their own:
// TriggerHover while the pointer is over the field, TriggerFocusVisible while
// it has keyboard focus, TriggerDisabled when it is disabled, OptionHighlighted
// for the option that has keyboard focus in the open list, OptionSelected for
// the chosen option and OptionDisabled for an option that cannot be chosen.
type SelectStyle struct {
	Trigger             Style // the field that shows the chosen option
	TriggerHover        Style // the field while the pointer is over it
	TriggerFocusVisible Style // the field while it has keyboard focus
	TriggerDisabled     Style // the field when it is disabled
	TriggerInvalid      Style // the field when its value is not valid (see Field)
	Value               Style // the label of the chosen option in the field
	Placeholder         Style // the placeholder, while no option is chosen
	Chevron             Style // the arrow at the right of the field
	Popup               Style // the open list of options
	Option              Style // each option in the list
	OptionHighlighted   Style // the option that has keyboard focus in the list
	OptionSelected      Style // the chosen option
	OptionDisabled      Style // an option that cannot be chosen
}

// merge returns s with the set fields of o applied on top, part by part.
func (s SelectStyle) merge(o SelectStyle) SelectStyle {
	return SelectStyle{
		Trigger:             s.Trigger.Merge(o.Trigger),
		TriggerHover:        s.TriggerHover.Merge(o.TriggerHover),
		TriggerFocusVisible: s.TriggerFocusVisible.Merge(o.TriggerFocusVisible),
		TriggerDisabled:     s.TriggerDisabled.Merge(o.TriggerDisabled),
		TriggerInvalid:      s.TriggerInvalid.Merge(o.TriggerInvalid),
		Value:               s.Value.Merge(o.Value),
		Placeholder:         s.Placeholder.Merge(o.Placeholder),
		Chevron:             s.Chevron.Merge(o.Chevron),
		Popup:               s.Popup.Merge(o.Popup),
		Option:              s.Option.Merge(o.Option),
		OptionHighlighted:   s.OptionHighlighted.Merge(o.OptionHighlighted),
		OptionSelected:      s.OptionSelected.Merge(o.OptionSelected),
		OptionDisabled:      s.OptionDisabled.Merge(o.OptionDisabled),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s SelectStyle) plain() SelectStyle {
	return SelectStyle{
		Trigger:             plainStyle(s.Trigger),
		TriggerHover:        plainStyle(s.TriggerHover),
		TriggerFocusVisible: plainStyle(s.TriggerFocusVisible),
		TriggerDisabled:     plainStyle(s.TriggerDisabled),
		TriggerInvalid:      plainStyle(s.TriggerInvalid),
		Value:               plainStyle(s.Value),
		Placeholder:         plainStyle(s.Placeholder),
		Chevron:             plainStyle(s.Chevron),
		Popup:               plainStyle(s.Popup),
		Option:              plainStyle(s.Option),
		OptionHighlighted:   plainStyle(s.OptionHighlighted),
		OptionSelected:      plainStyle(s.OptionSelected),
		OptionDisabled:      plainStyle(s.OptionDisabled),
	}
}

// selectWidth is the default width of a select at the Text size of 100 percent, in
// logical pixels. It scales with the Text size.
const selectWidth = 220

// selectDefaults returns the built-in look of a select of the given width, from
// the colors of t. Under high contrast a disabled select is all GrayText.
func selectDefaults(t *Theme, width float32, disabled bool) SelectStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	placeholder, chevron := t.TextSubtle, t.TextMuted
	if disabled && t.HighContrast {
		placeholder, chevron = t.Disabled, t.Disabled
	}
	d := SelectStyle{
		Trigger: Style{Direction: Row, Align: Center, Gap: 8, Width: Px(width), MinHeight: Px(34),
			Padding: XY(10, 0), Radius: t.Radius, Background: t.Surface, BorderWidth: 1, BorderColor: t.BorderStrong,
			Transition: 110 * time.Millisecond, TextColor: t.Text},
		TriggerHover:        Style{BorderColor: t.TextMuted},
		TriggerFocusVisible: Style{Shadows: []Shadow{ring}},
		// The invalid trigger is drawn with a thicker border in the danger color, as an
		// invalid text field is (the text color under high contrast).
		TriggerInvalid: Style{BorderWidth: 2, BorderColor: messageIconColor(t, Style{})},
		Placeholder:    Style{TextColor: placeholder},
		Chevron:        Style{TextColor: chevron},
		Popup: Style{Padding: All(4), Background: t.Surface, BorderWidth: 1,
			BorderColor: t.Border, Radius: t.Radius, Shadows: []Shadow{ShadowMd}},
		Option: Style{Direction: Row, Align: Center, Gap: 8, MinHeight: Px(28), Padding: XY(8, 0),
			Radius: 4, NoShrink: true, FontSize: 13, TextColor: t.Text},
		OptionDisabled: Style{TextColor: t.Disabled},
	}
	if t.HighContrast {
		// The highlighted option is the Highlight color, as a selected item is.
		d.TriggerDisabled = Style{TextColor: t.Disabled, BorderColor: t.Disabled}
		d.OptionHighlighted = Style{Background: t.Accent, TextColor: t.AccentText}
	} else {
		d.TriggerDisabled = Style{Opacity: 0.5}
		d.OptionHighlighted = Style{Background: t.AccentSoft}
	}
	return d
}

// resolveSelect returns the look of a select: the defaults, merged with the
// theme's Select and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveSelect(t *Theme, width float32, disabled bool, inst SelectStyle) SelectStyle {
	st := selectDefaults(t, width, disabled)
	for _, o := range []SelectStyle{t.Select, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

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
	value        *Signal[T]
	options      []Option[T]
	placeholder  string
	name         string // the accessible name, see Label
	labelledBy   []string
	key          string
	disabled     bool
	size         sizing
	onChange     func(T)
	styles       SelectStyle
	renderValue  func(Option[T], PartState) Element
	renderOption func(Option[T], PartState) Element
	rules        []Validator[T] // the validators of the value (see Validate)
	wire         fieldWire      // what a Field gives the field (see Field)
}

// Select creates a drop-down list that reads and writes value. The options are
// shown in order. The default width is 220 px at the Text size of 100 percent.
func Select[T comparable](value *Signal[T], options ...Option[T]) *SelectElement[T] {
	return &SelectElement[T]{value: value, options: options}
}

// Validate sets the validators of the chosen value. The zero value is the
// placeholder, so Required[T] rejects it: a field must show a choice (see Field).
func (s *SelectElement[T]) Validate(rules ...Validator[T]) *SelectElement[T] {
	s.rules = rules
	return s
}

// formCheck and formWire make a select a formControl (see Field).
func (s *SelectElement[T]) formCheck() error { return firstError(s.value.Get(), s.rules) }

func (s *SelectElement[T]) formWire(w fieldWire) Element {
	c := *s
	c.wire = w
	return &c
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

// The sizing methods give the field a size. The field is 220 px wide at the Text
// size of 100 percent by default; Width(Full) fills its container. The list is at
// least as wide as the field. See Sizing in docs/styling.md.
func (s *SelectElement[T]) Width(l Length) *SelectElement[T]     { s.size.setWidth(l); return s }
func (s *SelectElement[T]) Height(l Length) *SelectElement[T]    { s.size.setHeight(l); return s }
func (s *SelectElement[T]) MinWidth(l Length) *SelectElement[T]  { s.size.setMinWidth(l); return s }
func (s *SelectElement[T]) MinHeight(l Length) *SelectElement[T] { s.size.setMinHeight(l); return s }
func (s *SelectElement[T]) MaxWidth(l Length) *SelectElement[T]  { s.size.setMaxWidth(l); return s }
func (s *SelectElement[T]) MaxHeight(l Length) *SelectElement[T] { s.size.setMaxHeight(l); return s }
func (s *SelectElement[T]) Grow(f float32) *SelectElement[T]     { s.size.setGrow(f); return s }

// Styles sets the look of the parts of the select, for this instance only. Set
// fields win over the theme's Select and the defaults; see SelectStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (s *SelectElement[T]) Styles(st SelectStyle) *SelectElement[T] {
	s.styles = s.styles.merge(st)
	return s
}

// RenderValue replaces the label of the chosen option in the field with what fn
// returns, for example a swatch and the label. fn is called only while an option
// is chosen; the placeholder is shown otherwise. The field keeps its behavior,
// and its accessible name and value stay the option's label. The content is
// hidden from assistive technology.
func (s *SelectElement[T]) RenderValue(fn func(opt Option[T], state PartState) Element) *SelectElement[T] {
	s.renderValue = fn
	return s
}

// RenderOption replaces the label of each option in the open list with what fn
// returns. The option keeps its check mark, role, selection and highlight, and
// its accessible name stays its label. The content is hidden from assistive
// technology. The state's Focused means the option is highlighted.
//
//	ui.Select(color, options...).RenderOption(func(o ui.Option[string], s ui.PartState) ui.Element {
//		return ui.Div().Size(ui.Px(12), ui.Px(12)).Radius(6).Background(swatch[o.Value])
//	})
func (s *SelectElement[T]) RenderOption(fn func(opt Option[T], state PartState) Element) *SelectElement[T] {
	s.renderOption = fn
	return s
}

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
		st := resolveSelect(t, selectWidth*textScale(ctx), s.disabled, s.styles)
		cur := s.value.Get()
		sel := indexOf(s.options, cur)
		open := Local(ctx, "open", false)
		hl := Local(ctx, "hl", -1) // the highlighted option while the list is open
		typed := Local(ctx, "typed", selectType{})
		// back is the field's focus handle. Focus returns to the field when the list
		// closes, whichever way it was opened.
		back := Local(ctx, "back", NewFocusHandle())
		// ret is the field's handle: the one a Field gave it, else back.
		ret := back.Peek()
		w := s.wire
		if w.handle != nil {
			ret = w.handle
		}

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
			rt.menuReturn = ret
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
		// The state of the field, for a render slot. A slot's content follows the
		// field's pointer and focus state, so the field re-renders when they change.
		var ps PartState
		if s.renderValue != nil {
			ps = PartState{Hovered: ctx.hovered(), Pressed: ctx.pressed(), Focused: ctx.isFocused(),
				Disabled: s.disabled, HighContrast: t.HighContrast}
		}
		var shown Element
		switch {
		case sel >= 0 && s.renderValue != nil:
			ps.Selected = true
			shown = Div().asPart("Select.Value").Grow(1).AccessHidden().Style(st.Value).
				Child(s.renderValue(s.options[sel], ps))
		case sel >= 0:
			shown = Div().asPart("Select.Value").Grow(1).AccessHidden().Style(st.Value).Child(Text(label).NoWrap())
		default:
			shown = Div().asPart("Select.Placeholder").Grow(1).AccessHidden().Style(st.Placeholder).
				Child(Text(s.placeholder).NoWrap())
		}
		field := Div().asPart("Select.Trigger").Style(st.Trigger).
			Child(shown).
			Child(Div().asPart("Select.Chevron").Shrink(0).Style(st.Chevron).Child(Icon(chevron).Size(16)))
		a := field.access()
		field.Role(access.RoleComboBox).AccessExpanded(open.Get())
		a.value, a.readOnly = label, true
		name, labelledBy := s.name, s.labelledBy
		if w.label != "" {
			name, labelledBy = w.label, w.labelledBy
		}
		if name != "" {
			field.Label(name)
		}
		if len(labelledBy) > 0 {
			field.LabelledBy(labelledBy...)
		}
		if w.description != "" {
			field.Description(w.description)
		}
		a.invalid, a.required = w.invalid, w.required
		hover := st.TriggerHover
		if w.invalid {
			field.Style(st.TriggerInvalid)
			hover = st.TriggerInvalid
		}
		if w.onBlur != nil {
			field.OnBlur(w.onBlur)
		}
		field.FocusHandle(ret)
		if s.disabled {
			field.Cursor(CursorNotAllowed)
			a.disabled = true
			field.Style(st.TriggerDisabled)
			return s.size.apply(field)
		}
		field.Cursor(CursorPointer).Hover(hover).
			FocusVisible(st.TriggerFocusVisible).
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
			field.Child(s.list(st, t, sel, hl.Get(), shut, choose, func(r rune) int {
				return search(r, hl.Peek())
			}, func(i int) { hl.Set(i) }))
		}
		return s.size.apply(field)
	})
}

// list is the open list of the field: an overlay below it, which takes focus.
// Its rows are list items, the chosen one is selected, and the highlighted one
// is the row that has focus.
func (s *SelectElement[T]) list(st SelectStyle, t *Theme, sel, hl int, shut func(bool), choose func(int),
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
	panel := Div().asPart("Select.Popup").Key("jamb.listbox").Column().Style(st.Popup).
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
	panel.matchAnchor = true // the list is at least as wide as the field, whatever its width
	for i, o := range s.options {
		i := i
		on := i == sel
		hi := i == hl && !o.Disabled
		draw := func(ps PartState) *DivElement {
			style := st.Option
			if hi {
				style = style.Merge(st.OptionHighlighted)
			}
			if on {
				style = style.Merge(st.OptionSelected)
			}
			if o.Disabled {
				style = style.Merge(st.OptionDisabled)
			}
			mark := Div().Size(Px(16), Px(16)).Center().Shrink(0)
			if on {
				mark.Child(Icon(icons.Check).Size(14))
			}
			row := Div().asPart("Select.Option").Style(style).
				Role(access.RoleListItem).AccessSelected(on).Label(o.Label).Child(mark)
			if s.renderOption != nil {
				row.Child(Div().AccessHidden().Child(s.renderOption(o, ps)))
			} else {
				row.Child(Text(o.Label).NoWrap())
			}
			row.access().highlighted = hi
			if o.Disabled {
				row.access().disabled = true
			} else {
				row.OnMouseEnter(func() { highlight(i) }).OnClick(func() { choose(i) })
			}
			return row
		}
		if s.renderOption == nil {
			panel.Child(draw(PartState{}))
			continue
		}
		// With a render slot the row is its own component, so that it re-renders
		// when its pointer state changes, and the slot gets the new state.
		panel.Child(Component(func(ctx *Context) Element {
			return draw(PartState{Hovered: ctx.hovered(), Pressed: ctx.pressed(), Focused: hi,
				Selected: on, Disabled: o.Disabled, HighContrast: t.HighContrast})
		}).Key(fmt.Sprintf("jamb.listbox/%d", i)))
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
	if s.wire.onEdit != nil {
		s.wire.onEdit()
	}
	if s.onChange != nil {
		s.onChange(v)
	}
}

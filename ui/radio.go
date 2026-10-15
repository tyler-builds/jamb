package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// Option is one choice of a RadioGroup or a Select: the value it sets, the text
// it shows, and whether it can be chosen.
type Option[T comparable] struct {
	// Value is the value that the option sets.
	Value T
	// Label is the text of the option.
	Label string
	// Disabled makes the option unable to be chosen.
	Disabled bool
}

// RadioGroupStyle is the look of each part of a radio group. A part's style
// merges over the built-in defaults, then over the theme's RadioGroup, then over
// the Styles of the instance; see docs/styling.md. Group is the column of
// options. Each option is a Radio row, with a Ring (the circle) and a Dot in the
// ring when the option is selected, and a Label. The states are parts of their
// own: RingChecked for the selected option's ring, RingDisabled and
// RadioDisabled for an option that cannot be chosen, RadioFocusVisible while
// the option has keyboard focus, and LabelDisabled for its text.
type RadioGroupStyle struct {
	Group             Style // the column of options
	Radio             Style // each option: its ring and its label
	RadioFocusVisible Style // an option that has keyboard focus
	RadioDisabled     Style // an option that cannot be chosen
	Ring              Style // the circle of each option
	RingChecked       Style // the circle of the selected option
	RingDisabled      Style // the circle of an option that cannot be chosen
	RingInvalid       Style // the circle of each option when the group's value is not valid (see Field)
	Dot               Style // the dot in the circle of the selected option
	DotDisabled       Style // the dot of a selected option that cannot be chosen
	Label             Style // the text of each option
	LabelDisabled     Style // the text of an option that cannot be chosen
}

// merge returns s with the set fields of o applied on top, part by part.
func (s RadioGroupStyle) merge(o RadioGroupStyle) RadioGroupStyle {
	return RadioGroupStyle{
		Group:             s.Group.Merge(o.Group),
		Radio:             s.Radio.Merge(o.Radio),
		RadioFocusVisible: s.RadioFocusVisible.Merge(o.RadioFocusVisible),
		RadioDisabled:     s.RadioDisabled.Merge(o.RadioDisabled),
		Ring:              s.Ring.Merge(o.Ring),
		RingChecked:       s.RingChecked.Merge(o.RingChecked),
		RingDisabled:      s.RingDisabled.Merge(o.RingDisabled),
		RingInvalid:       s.RingInvalid.Merge(o.RingInvalid),
		Dot:               s.Dot.Merge(o.Dot),
		DotDisabled:       s.DotDisabled.Merge(o.DotDisabled),
		Label:             s.Label.Merge(o.Label),
		LabelDisabled:     s.LabelDisabled.Merge(o.LabelDisabled),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s RadioGroupStyle) plain() RadioGroupStyle {
	return RadioGroupStyle{
		Group:             plainStyle(s.Group),
		Radio:             plainStyle(s.Radio),
		RadioFocusVisible: plainStyle(s.RadioFocusVisible),
		RadioDisabled:     plainStyle(s.RadioDisabled),
		Ring:              plainStyle(s.Ring),
		RingChecked:       plainStyle(s.RingChecked),
		RingDisabled:      plainStyle(s.RingDisabled),
		RingInvalid:       plainStyle(s.RingInvalid),
		Dot:               plainStyle(s.Dot),
		DotDisabled:       plainStyle(s.DotDisabled),
		Label:             plainStyle(s.Label),
		LabelDisabled:     plainStyle(s.LabelDisabled),
	}
}

// radioGroupDefaults returns the built-in look of a radio group, from the colors
// of t. Under high contrast the ring is a solid 2 px line in the text color, or
// in Highlight when selected, and a disabled option is GrayText and not dimmed.
func radioGroupDefaults(t *Theme) RadioGroupStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := RadioGroupStyle{
		Group:             Style{Direction: Column, Gap: 8, fitWidth: true},
		Radio:             Style{Direction: Row, Align: Center, Gap: 10, Radius: 4, NoShrink: true},
		RadioFocusVisible: Style{Shadows: []Shadow{ring}},
		RadioDisabled:     Style{Opacity: 0.5},
		Ring: Style{Width: Px(18), Height: Px(18), NoShrink: true, Radius: 9,
			Direction: Row, Align: Center, Justify: Center, BorderWidth: 1, BorderColor: t.BorderStrong, Background: t.Surface},
		RingChecked: Style{BorderColor: t.Accent},
		RingInvalid: Style{BorderWidth: 2, BorderColor: messageIconColor(t, Style{})},
		Dot:         Style{Width: Px(8), Height: Px(8), NoShrink: true, Radius: 4, Background: t.Accent},
	}
	if t.HighContrast {
		d.Ring.BorderWidth, d.Ring.BorderColor = 2, t.Text
		d.RadioDisabled = Style{Opacity: 1, TextColor: t.Disabled}
		d.RingDisabled = Style{BorderColor: t.Disabled}
		d.DotDisabled = Style{Background: t.Disabled}
		d.LabelDisabled = Style{TextColor: t.Disabled}
	}
	return d
}

// resolveRadioGroup returns the look of a radio group: the defaults, merged with
// the theme's RadioGroup and then the instance's Styles. Under high contrast the
// two overrides are plain (see plainStyle).
func resolveRadioGroup(ctx *Context, inst RadioGroupStyle) RadioGroupStyle {
	t := ctx.Theme()
	return layered(ctx, radioGroupDefaults(t), RadioGroupStyle.merge, RadioGroupStyle.plain,
		themeLayer(t.RadioGroup), instanceLayer(inst))
}

// RadioGroupElement is a set of radio buttons, one per option, bound to a value.
// The selected option is the one whose value equals the signal's. The group is
// one Tab stop: Tab enters it at the selected option, and the arrow keys move
// to the next option and select it. Up and Left go back; Down and Right go on.
// They wrap around and skip disabled options, as Windows radio buttons do.
type RadioGroupElement[T comparable] struct {
	value       *Signal[T]
	options     []Option[T]
	name        string // the group's accessible name, see Label
	labelledBy  []string
	key         string
	disabled    bool
	onChange    func(T)
	styles      RadioGroupStyle
	rules       []Validator[T] // the validators of the value (see Validate)
	wire        fieldWire      // what a Field gives the group (see Field)
	size        sizing         // the group's sizes, from the sizing methods (see Sizing in docs/styling.md)
	focusHandle *FocusHandle   // see FocusHandle
	tooltip     string         // see Tooltip
}

// Tooltip shows s below the group after the pointer rests on it. Assistive
// technology reads it as the group's description, unless it has another.
func (r *RadioGroupElement[T]) Tooltip(s string) *RadioGroupElement[T] {
	r.tooltip = s
	return r
}

// FocusHandle lets code focus the group's tab stop: the checked option, or the first enabled one (see NewFocusHandle). A Field that wraps
// the control gives it its own handle, which takes precedence.
func (r *RadioGroupElement[T]) FocusHandle(h *FocusHandle) *RadioGroupElement[T] {
	r.focusHandle = h
	return r
}

// RadioGroup creates a group of radio buttons that reads and writes value.
func RadioGroup[T comparable](value *Signal[T], options ...Option[T]) *RadioGroupElement[T] {
	return &RadioGroupElement[T]{value: value, options: options}
}

// Validate sets the validators of the chosen value. The zero value is no choice, so
// Required[T] rejects it (see Field).
func (r *RadioGroupElement[T]) Validate(rules ...Validator[T]) *RadioGroupElement[T] {
	r.rules = rules
	return r
}

// formCheck and formWire make a radio group a formControl (see Field).
func (r *RadioGroupElement[T]) formCheck() error { return firstError(r.value.Get(), r.rules) }

func (r *RadioGroupElement[T]) formWire(w fieldWire) Element {
	c := *r
	c.wire = w
	return &c
}

func (*RadioGroupElement[T]) isElement()           {}
func (r *RadioGroupElement[T]) elementKey() string { return r.key }

// Key identifies the radio group among its siblings (see DivElement.Key).
func (r *RadioGroupElement[T]) Key(k string) *RadioGroupElement[T] { r.key = k; return r }

// Label names the group for assistive technology, such as "Frequency". The
// group has no visible name of its own, so give it one.
func (r *RadioGroupElement[T]) Label(s string) *RadioGroupElement[T] { r.name = s; return r }

// LabelledBy names the group by the text of the elements with the given keys,
// such as a visible label above it (see DivElement.LabelledBy).
func (r *RadioGroupElement[T]) LabelledBy(keys ...string) *RadioGroupElement[T] {
	r.labelledBy = keys
	return r
}

// OnChange runs fn with the chosen value after the user selects an option, by a
// click or by the arrow keys. The signal is set first.
func (r *RadioGroupElement[T]) OnChange(fn func(T)) *RadioGroupElement[T] { r.onChange = fn; return r }

// Disabled shows every option but lets none of them be chosen.
func (r *RadioGroupElement[T]) Disabled(d bool) *RadioGroupElement[T] { r.disabled = d; return r }

// The sizing methods give the group's outer box a size. A group is as wide as its
// widest option by default; see Sizing in docs/styling.md.
func (r *RadioGroupElement[T]) Width(l Length) *RadioGroupElement[T] { r.size.setWidth(l); return r }

// Height sets the height of the radio group.
func (r *RadioGroupElement[T]) Height(l Length) *RadioGroupElement[T] { r.size.setHeight(l); return r }

// MinWidth sets the smallest width of the radio group.
func (r *RadioGroupElement[T]) MinWidth(l Length) *RadioGroupElement[T] {
	r.size.setMinWidth(l)
	return r
}

// MaxWidth sets the largest width of the radio group.
func (r *RadioGroupElement[T]) MaxWidth(l Length) *RadioGroupElement[T] {
	r.size.setMaxWidth(l)
	return r
}

// Grow makes the radio group take a share of the free space on its parent's main axis.
func (r *RadioGroupElement[T]) Grow(f float32) *RadioGroupElement[T] { r.size.setGrow(f); return r }

// GridSpan sets how many columns and rows the radio group covers in its parent's grid.
func (r *RadioGroupElement[T]) GridSpan(cols, rows int) *RadioGroupElement[T] {
	r.size.setSpan(cols, rows)
	return r
}

// Styles sets the look of the parts of the radio group, for this instance only.
// Set fields win over the theme's RadioGroup and the defaults; see
// RadioGroupStyle and docs/styling.md. Calling Styles again merges with the
// earlier values.
func (r *RadioGroupElement[T]) Styles(st RadioGroupStyle) *RadioGroupElement[T] {
	r.styles = r.styles.merge(st)
	return r
}

func (r *RadioGroupElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		cur := r.value.Get()
		// One focus handle per option, kept across renders, so the arrow keys can
		// move focus to the next option.
		handles := Local(ctx, "handles", newHandles(len(r.options))).Peek()
		stop := tabStop(r.options, cur, r.disabled)
		st := resolveRadioGroup(ctx, r.styles)
		// The tab stop is the option a Field gave its handle to, when there is one, and the
		// arrow keys reach every option by the handle it has.
		w := r.wire
		handleOf := func(j int) *FocusHandle {
			if j == stop && w.handle != nil {
				return w.handle
			}
			if j == stop && r.focusHandle != nil {
				return r.focusHandle
			}
			return handles[j]
		}

		g := Div().asPart("RadioGroup.Group").Style(st.Group).Role(access.RoleGroup).SelectionContainer().Tooltip(r.tooltip)
		name, labelledBy := r.name, r.labelledBy
		if w.label != "" {
			name, labelledBy = w.label, w.labelledBy
		}
		if name != "" {
			g.Label(name)
		}
		if len(labelledBy) > 0 {
			g.LabelledBy(labelledBy...)
		}
		if w.description != "" {
			g.Description(w.description)
		}
		g.access().invalid, g.access().required = w.invalid, w.required
		for i, o := range r.options {
			on := o.Value == cur
			dis := r.disabled || o.Disabled
			// The circle shows the selected option as a dot, so the state is not
			// carried by color alone.
			ring := Div().asPart("RadioGroup.Ring").Style(st.Ring)
			if on {
				ring.Style(st.RingChecked)
			}
			if dis {
				ring.Style(st.RingDisabled)
			}
			if w.invalid {
				ring.Style(st.RingInvalid)
			}
			if on {
				dot := Div().asPart("RadioGroup.Dot").Style(st.Dot)
				if dis {
					dot.Style(st.DotDisabled)
				}
				ring.Child(dot)
			}
			label := Div().asPart("RadioGroup.Label").Style(st.Label)
			if dis {
				label.Style(st.LabelDisabled)
			}
			label.Child(Text(o.Label))
			item := Div().asPart("RadioGroup.Radio").Style(st.Radio).Child(ring).Child(label)
			if r.key != "" {
				item.Keyf("%s/%v", r.key, o.Value)
			}
			item.Role(access.RoleRadioButton).AccessChecked(on)
			if dis {
				item.access().disabled = true
				item.Cursor(CursorNotAllowed).Style(st.RadioDisabled)
			} else {
				item.Cursor(CursorPointer).Focusable().FocusHandle(handleOf(i)).FocusVisible(st.RadioFocusVisible).
					OnClick(func() { r.choose(r.options[i].Value) }).
					OnKeyDown(func(e *KeyEvent) {
						if e.Mods != 0 {
							return
						}
						dir := 0
						switch e.Key {
						case input.KeyUp, input.KeyLeft:
							dir = -1
						case input.KeyDown, input.KeyRight:
							dir = 1
						default:
							return
						}
						e.StopPropagation()
						if j := nextEnabled(r.options, i, dir, r.disabled); j >= 0 {
							r.choose(r.options[j].Value)
							handleOf(j).Focus()
						}
					})
				if w.onBlur != nil {
					item.OnBlur(w.onBlur)
				}
				item.spaceOnly = true // Enter goes on to the modal default button, as on Windows
				if i != stop {
					item.TabStop(false) // reached with the arrow keys, not Tab
				}
			}
			g.Child(item)
		}
		return r.size.apply(g)
	})
}

// newHandles returns n new focus handles.
func newHandles(n int) []*FocusHandle {
	hs := make([]*FocusHandle, n)
	for i := range hs {
		hs[i] = NewFocusHandle()
	}
	return hs
}

// tabStop returns the index of the option that Tab reaches: the selected one,
// else the first that can be chosen. It returns -1 when none can.
func tabStop[T comparable](options []Option[T], cur T, disabled bool) int {
	if disabled {
		return -1
	}
	for i, o := range options {
		if o.Value == cur && !o.Disabled {
			return i
		}
	}
	return nextEnabled(options, -1, 1, disabled)
}

// nextEnabled returns the index of the next option that can be chosen after
// from, moving dir (+1 or -1) and wrapping around. From -1 it starts at the
// first option going forward and at the last going back. It returns -1 when
// no option can be chosen.
func nextEnabled[T comparable](options []Option[T], from, dir int, disabled bool) int {
	n := len(options)
	if n == 0 || disabled {
		return -1
	}
	i := from
	if i < 0 || i >= n {
		i = n - 1
		if dir < 0 {
			i = 0
		}
	}
	for range n {
		i = (i + dir + n) % n
		if !options[i].Disabled {
			return i
		}
	}
	return -1
}

// choose sets the value to v and runs OnChange.
func (r *RadioGroupElement[T]) choose(v T) {
	r.value.Set(v)
	if r.wire.onEdit != nil {
		r.wire.onEdit()
	}
	if r.onChange != nil {
		r.onChange(v)
	}
}

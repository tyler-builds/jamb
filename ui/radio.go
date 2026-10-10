package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// Option is one choice of a RadioGroup or a Select: the value it sets, the text
// it shows, and whether it can be chosen.
type Option[T comparable] struct {
	Value    T
	Label    string
	Disabled bool
}

// RadioGroupElement is a set of radio buttons, one per option, bound to a value.
// The selected option is the one whose value equals the signal's. The group is
// one Tab stop: Tab enters it at the selected option, and the arrow keys move
// to the next option and select it. Up and Left go back; Down and Right go on.
// They wrap around and skip disabled options, as Windows radio buttons do.
type RadioGroupElement[T comparable] struct {
	value      *Signal[T]
	options    []Option[T]
	name       string // the group's accessible name, see Label
	labelledBy []string
	key        string
	disabled   bool
	onChange   func(T)
}

// RadioGroup creates a group of radio buttons that reads and writes value.
func RadioGroup[T comparable](value *Signal[T], options ...Option[T]) *RadioGroupElement[T] {
	return &RadioGroupElement[T]{value: value, options: options}
}

func (*RadioGroupElement[T]) isElement()           {}
func (r *RadioGroupElement[T]) elementKey() string { return r.key }

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

func (r *RadioGroupElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		cur := r.value.Get()
		// One focus handle per option, kept across renders, so the arrow keys can
		// move focus to the next option.
		handles := Local(ctx, "handles", newHandles(len(r.options))).Peek()
		stop := tabStop(r.options, cur, r.disabled)
		ring := Shadow{Spread: 2, Color: t.FocusRing}

		g := Div().Column().Gap(8).Role(access.RoleGroup).SelectionContainer()
		if r.name != "" {
			g.Label(r.name)
		}
		if len(r.labelledBy) > 0 {
			g.LabelledBy(r.labelledBy...)
		}
		for i, o := range r.options {
			on := o.Value == cur
			dis := r.disabled || o.Disabled
			// The circle shows the selected option as a dot, so the state is not
			// carried by color alone.
			// Under high contrast the ring is a solid 2 px line in the text color, or in
			// Highlight when selected, and the dot is Highlight in the same way, so the
			// selected option is one crisp ring with one solid dot.
			edge, width, dot := t.BorderStrong, float32(1), t.Accent
			if t.HighContrast {
				edge, width, dot = t.Text, 2, t.Accent
			}
			if on {
				edge = t.Accent
			}
			if dis && t.HighContrast {
				edge, dot = t.Disabled, t.Disabled
			}
			circle := Div().Size(Px(18), Px(18)).Shrink(0).Radius(9).Center().
				Border(width, edge).Background(t.Surface)
			if on {
				circle.Child(Div().Size(Px(8), Px(8)).Radius(4).Background(dot))
			}
			item := Div().Row().Align(Center).Gap(10).Radius(4).Shrink(0).
				Child(circle).Child(Text(o.Label))
			if r.key != "" {
				item.Keyf("%s/%v", r.key, o.Value)
			}
			item.Role(access.RoleRadioButton).Checked(on)
			if dis {
				item.access().disabled = true
				item.Cursor(CursorNotAllowed).Opacity(0.5)
				if t.HighContrast {
					item.Opacity(1).TextColor(t.Disabled)
				}
			} else {
				item.Cursor(CursorPointer).Focusable().FocusHandle(handles[i]).FocusVisible(Style{Shadows: []Shadow{ring}}).
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
							handles[j].Focus()
						}
					})
				item.spaceOnly = true // Enter goes on to the modal default button, as on Windows
				if i != stop {
					item.TabStop(false) // reached with the arrow keys, not Tab
				}
			}
			g.Child(item)
		}
		return g
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
	if r.onChange != nil {
		r.onChange(v)
	}
}

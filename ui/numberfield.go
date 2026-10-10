package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// NumberFieldElement is a text field for a number, bound to an int or float64
// signal, from lo to hi in steps of step. The user types a number, or steps it
// with Up and Down (and with the optional buttons).
//
// The text is checked when it is committed, by Enter or by leaving the field:
//   - a number outside the range is clamped to the nearest bound;
//   - text that is not a number (or, for an int, not a whole number) is not
//     applied. Enter shows the field as invalid, with a message that gives the
//     range, and keeps the text so it can be corrected. Leaving the field
//     reverts the text to the value that is set.
//
// Assistive technology reports it as a spin box, which holds the text field.
type NumberFieldElement[T int | float64] struct {
	value      *Signal[T]
	lo, hi     T
	step       T
	name       string // the accessible name, see Label
	labelledBy []string
	key        string
	disabled   bool
	buttons    bool
	precision  int // the decimals of a float, or -1 for the decimals of the step
	width      float32
}

// NumberField creates a number field that reads and writes value, from lo to hi
// in steps of step. Its default width is 120 px and it shows the decimals of
// step.
func NumberField[T int | float64](value *Signal[T], lo, hi, step T) *NumberFieldElement[T] {
	return &NumberFieldElement[T]{value: value, lo: lo, hi: hi, step: step, precision: -1, width: 120}
}

func (*NumberFieldElement[T]) isElement()           {}
func (n *NumberFieldElement[T]) elementKey() string { return n.key }

func (n *NumberFieldElement[T]) Key(k string) *NumberFieldElement[T] { n.key = k; return n }

// Label names the field for assistive technology, such as "Quiet hours from".
func (n *NumberFieldElement[T]) Label(s string) *NumberFieldElement[T] { n.name = s; return n }

// LabelledBy names the field by the text of the elements with the given keys;
// see DivElement.LabelledBy.
func (n *NumberFieldElement[T]) LabelledBy(keys ...string) *NumberFieldElement[T] {
	n.labelledBy = keys
	return n
}

// Disabled shows the number but does not let it change.
func (n *NumberFieldElement[T]) Disabled(d bool) *NumberFieldElement[T] { n.disabled = d; return n }

// Buttons adds a decrease and an increase button after the field.
func (n *NumberFieldElement[T]) Buttons() *NumberFieldElement[T] { n.buttons = true; return n }

// Precision sets the number of decimals a float is shown with, so 2 shows 1.50.
// The default is the number of decimals of the step.
func (n *NumberFieldElement[T]) Precision(d int) *NumberFieldElement[T] { n.precision = d; return n }

// Width sets the width of the field, in logical pixels.
func (n *NumberFieldElement[T]) Width(px float32) *NumberFieldElement[T] { n.width = px; return n }

func (n *NumberFieldElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		v := n.value.Get()
		// The field keeps its own text while the user types. draft is that text,
		// and seen is the value it was last made from, so that a value set from
		// outside the field replaces the text.
		draft := Local(ctx, "draft", n.format(v))
		seen := Local(ctx, "seen", v)
		invalid := Local(ctx, "invalid", false)
		if v != seen.Peek() {
			draft.Set(n.format(v))
			seen.Set(v)
			invalid.Set(false)
		}

		apply := func(x T) {
			x = n.clamp(x)
			n.value.Set(x)
			draft.Set(n.format(x))
			seen.Set(x)
			invalid.Set(false)
		}
		submit := func(text string) {
			x, ok := n.parse(text)
			if !ok {
				invalid.Set(true)
				return
			}
			apply(x)
		}
		blur := func() {
			if x, ok := n.parse(draft.Peek()); ok {
				apply(x)
				return
			}
			draft.Set(n.format(n.value.Peek()))
			invalid.Set(false)
		}
		step := func(dir int) {
			base, ok := n.parse(draft.Peek())
			if !ok {
				base = n.value.Peek()
			}
			apply(n.stepped(base, dir))
		}
		isInvalid := invalid.Get()
		errKey := "number-error"
		if n.key != "" {
			errKey = n.key + "/error"
		}

		root := Div().Column().Gap(6).Role(access.RoleSpinner)
		a := root.access()
		if n.name != "" {
			root.Label(n.name)
		}
		if len(n.labelledBy) > 0 {
			root.LabelledBy(n.labelledBy...)
		}
		a.rng = &access.Range{Value: float64(v), Min: float64(n.lo), Max: float64(n.hi),
			SmallChange: float64(n.step), LargeChange: 10 * float64(n.step)}
		a.setRange = func(x float64) { apply(numberFrom[T](x)) }
		a.invalid = isInvalid

		row := Div().Row().Align(Center).Gap(6)
		if n.disabled {
			a.disabled = true
			// The disabled number is drawn as text in a frame, and under high contrast
			// the frame and the text are GrayText, with no dimming.
			frame := Div().Row().Align(Center).Width(Px(n.width)).MinHeight(Px(34)).PaddingX(10).
				Radius(t.Radius).Background(t.Surface).Border(1, t.Border)
			if t.HighContrast {
				frame.Border(1, t.Disabled).TextColor(t.Disabled)
			} else {
				frame.Opacity(0.5)
			}
			root.Child(row.Child(frame.Child(Text(draft.Get()))))
			return root
		}

		fo := fieldOptions{value: draft, name: n.name, labelledBy: n.labelledBy,
			onSubmit: submit, onChange: func(string) { invalid.Set(false) }, onBlur: blur, onStep: step}
		fo.style = Style{Width: Px(n.width)}
		if isInvalid {
			fo.describedBy = []string{errKey}
			// The invalid frame is drawn thicker and in the danger color (the text
			// color under high contrast), and the message says what is wrong.
			fo.style.BorderWidth, fo.style.BorderColor = 2, t.Danger
			if t.HighContrast {
				fo.style.BorderColor = t.Text
			}
		}
		row.Child(fo.expand(ctx, false, 0))
		if n.buttons {
			row.Child(IconButton(icons.Minus).Label("Decrease").Small().Disabled(v <= n.lo).
				OnClick(func() { step(-1) }))
			row.Child(IconButton(icons.Plus).Label("Increase").Small().Disabled(v >= n.hi).
				OnClick(func() { step(1) }))
		}
		root.Child(row)
		if isInvalid {
			msg := fmt.Sprintf("Enter a number from %s to %s.", n.format(n.lo), n.format(n.hi))
			iconColor := t.Danger
			if t.HighContrast {
				iconColor = t.Text
			}
			root.Child(Div().Row().Align(Center).Gap(6).
				Child(Icon(icons.Alert).Size(14).Color(iconColor)).
				Child(Text(msg).Size(12).Key(errKey)))
		}
		return root
	})
}

// clamp returns x within the range.
func (n *NumberFieldElement[T]) clamp(x T) T { return min(max(x, n.lo), n.hi) }

// stepped returns the next grid value from x in direction dir (+1 or -1), where
// the grid starts at lo and is spaced by step. A value between two grid values
// moves to the nearer one in that direction, so 5.5 steps up to 6.
func (n *NumberFieldElement[T]) stepped(x T, dir int) T {
	step := float64(n.step)
	if step <= 0 {
		return n.clamp(x)
	}
	k := (float64(x) - float64(n.lo)) / step
	if dir > 0 {
		k = math.Floor(k+1e-9) + 1
	} else {
		k = math.Ceil(k-1e-9) - 1
	}
	return n.clamp(numberFrom[T](float64(n.lo) + k*step))
}

// parse returns the number that text holds, and whether it is a valid value.
// For an int the number must be a whole number.
func (n *NumberFieldElement[T]) parse(text string) (T, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if _, isInt := any(n.lo).(int); isInt && f != math.Trunc(f) {
		return 0, false
	}
	return numberFrom[T](f), true
}

// format returns the text of x: a whole number for an int, and for a float the
// number of decimals set by Precision, or that of the step.
func (n *NumberFieldElement[T]) format(x T) string {
	switch v := any(x).(type) {
	case int:
		return strconv.Itoa(v)
	case float64:
		d := n.precision
		if d < 0 {
			d = decimalsOf(float64(n.step))
		}
		return strconv.FormatFloat(v, 'f', d, 64)
	}
	return ""
}

// numberFrom converts f to T. A float is rounded to the nearest int for an int.
func numberFrom[T int | float64](f float64) T {
	var zero T
	if _, isInt := any(zero).(int); isInt {
		return T(math.Round(f))
	}
	return T(f)
}
